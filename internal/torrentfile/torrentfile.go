package torrentfile

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"dumb-tor-client/internal/client"
	"dumb-tor-client/internal/message"
	"dumb-tor-client/internal/peers"

	bencode "github.com/jackpal/bencode-go"
)

const (
	Port         uint16 = 6881
	MaxBacklog          = 5
	MaxBlockSize        = 16384
)

var counter int = 0

type pieceWork struct {
	index       int
	pieceHash   [20]byte
	pieceLength int
}

type pieceResult struct {
	index int
	buf   []byte
}

type TorrentFile struct {
	Announce  string
	InfoHash  [20]byte
	PieceHash [][20]byte
	PieceLen  int
	Length    int
	Name      string
	PeerID    [20]byte
}

type bencodeInfo struct {
	Pieces   string `bencode:"pieces"`
	PieceLen int    `bencode:"piece length"`
	Len      int    `bencode:"length"`
	Name     string `bencode:"name"`
}

type bencodeTorrent struct {
	Announce string      `bencode:"announce"`
	Info     bencodeInfo `bencode:"info"`
}

type pieceProgress struct {
	index      int
	client     *client.Client
	buf        []byte
	downloaded int
	requested  int
	backlog    int
}

func (state *pieceProgress) readMessage() error {
	msg, err := state.client.Read()
	if err != nil {
		return err
	}

	switch msg.ID {
	case message.MsgHave:
		index, err := message.ParseHave(*msg)
		if err != nil {
			return err
		}
		state.client.Bitfield.SetPiece(index)
	case message.MsgPiece:
		n, err := message.ParsePiece(state.index, state.buf, *msg)
		if err != nil {
			return err
		}
		state.downloaded += n
		state.backlog--
	}

	return nil
}

func Open(r io.Reader) (*TorrentFile, error) {
	bto := bencodeTorrent{}
	err := bencode.Unmarshal(r, &bto)
	if err != nil {
		return nil, err
	}

	torFile, err := bto.toTorrentFile()
	if err != nil {
		return nil, err
	}

	fmt.Printf("Starting work on downloading file \"%s\"\n", torFile.Name)

	return &torFile, nil
}

// Converter from bencoded Torrent file to specific structure that contains more useful info
func (b *bencodeTorrent) toTorrentFile() (TorrentFile, error) {
	newTF := TorrentFile{}
	var err error

	if len(b.Announce) == 0 {
		err := errors.New("Announce link not found. Torrent not supported.")
		return TorrentFile{}, err
	}
	newTF.Announce = b.Announce

	newTF.InfoHash, err = b.Info.getHashInfo()
	if err != nil {
		return TorrentFile{}, err
	}
	newTF.PieceHash, err = b.Info.getHashPieces()
	if err != nil {
		return TorrentFile{}, err
	}

	var peerId [20]byte
	_, err = rand.Read(peerId[:])
	if err != nil {
		return TorrentFile{}, err
	}
	newTF.PeerID = peerId

	newTF.PieceLen = b.Info.PieceLen
	newTF.Length = b.Info.Len
	newTF.Name = b.Info.Name

	return newTF, nil
}

// Function to get hash of specific file
func (i *bencodeInfo) getHashInfo() ([20]byte, error) {
	var buf bytes.Buffer
	err := bencode.Marshal(&buf, *i)
	if err != nil {
		return [20]byte{}, err
	}
	h := sha1.Sum(buf.Bytes())
	return h, nil
}

// Pieces element from decoded bencode contains SHA-1 hashes of every piece to download
func (i *bencodeInfo) getHashPieces() ([][20]byte, error) {
	hashLen := 20
	buf := []byte(i.Pieces)

	if len(buf)%hashLen != 0 {
		err := fmt.Errorf("Received malformed pieces lenght of %s", len(buf))
		return nil, err
	}

	numPieces := len(buf) / hashLen
	hashes := make([][20]byte, numPieces)

	for i := 0; i < numPieces; i += 1 {
		copy(hashes[i][:], buf[i*hashLen:(i+1)*hashLen])
	}

	return hashes, nil
}

// Function build's tracker url with specific params, that used for GET request
// peerID - a unique peername for created peer
// port - by standart 6881
func (t *TorrentFile) buildTrackerURL(peerID [20]byte, port uint16) (string, error) {
	base, err := url.Parse(t.Announce)
	if err != nil {
		return "", err
	}

	params := url.Values{
		"info_hash":  []string{string(t.InfoHash[:])},
		"peer_id":    []string{string(peerID[:])},
		"port":       []string{strconv.Itoa(int(port))},
		"uploaded":   []string{"0"},
		"downloaded": []string{"0"},
		"compact":    []string{"1"},
		"left":       []string{strconv.Itoa(t.Length)},
	}

	base.RawQuery = params.Encode()
	return base.String(), nil
}

// Function to get peers IP's and ports for onward downloading
func (t *TorrentFile) getPeers() ([]peers.Peer, error) {
	// Generate random peerId
	url, err := t.buildTrackerURL(t.PeerID, Port)

	// GET request
	responce, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	// Read body
	body, err := io.ReadAll(responce.Body)
	if err != nil {
		return nil, err
	}

	// Got bencoded responce with
	// interval - time in seconds, within which we're supposed to reconnect
	// Peers - long binary blob, that contains IP adress of each peer
	bencoded_resp := string(body)

	presp := peers.PeerResponce{}

	err = bencode.Unmarshal(strings.NewReader(bencoded_resp), &presp)
	if err != nil {
		return nil, err
	}

	// Transform array of bytes to array of structures
	p_arr, err := peers.Unmarshal([]byte(presp.PeersBin))
	if err != nil {
		return nil, err
	}

	return p_arr, nil
}

func (t TorrentFile) calcuateBound(index int) (int, int) {
	left := index * t.PieceLen
	right := left + t.PieceLen

	if right > t.Length {
		right = t.Length
	}

	return left, right
}

func (t TorrentFile) calculatePieceSize(index int) int {
	left := index * t.PieceLen
	right := left + t.PieceLen

	if right > t.Length {
		right = t.Length
	}

	return right - left
}

func (t TorrentFile) checkIntegrity(pw *pieceWork, buf []byte) error {
	hash := sha1.Sum(buf)

	if !bytes.Equal(hash[:], pw.pieceHash[:]) {
		return fmt.Errorf("Index %d failed integrity check", pw.index)
	}
	return nil
}

func (t *TorrentFile) attemptDownloadPiece(c *client.Client, pw *pieceWork) ([]byte, error) {
	state := pieceProgress{
		index:  pw.index,
		client: c,
		buf:    make([]byte, pw.pieceLength),
	}

	c.Conn.SetDeadline(time.Now().Add(30 * time.Second))
	defer c.Conn.SetDeadline(time.Time{})

	for state.downloaded < pw.pieceLength {
		if !state.client.Chocked {
			for state.backlog < MaxBacklog && state.requested < pw.pieceLength {
				blockSize := MaxBlockSize

				if pw.pieceLength-state.requested < blockSize {
					blockSize = pw.pieceLength - state.requested
				}

				err := c.SendRequest(pw.index, state.requested, blockSize)
				if err != nil {
					return nil, err
				}
				state.backlog++
				state.requested += blockSize
			}
		}

		err := state.readMessage()
		if err != nil {
			return nil, err
		}
	}

	return state.buf, nil
}

func (t *TorrentFile) startDownloadWork(peer peers.Peer, workQueue chan *pieceWork, results chan *pieceResult) {
	c, err := client.New(peer, t.PeerID, t.InfoHash)
	if err != nil {
		log.Println(err)
		return
	}
	defer c.CloseClient()

	log.Printf("Completed handshake with %s", peer.String())

	c.SendUnchoke()
	c.SendInterested()

	for pw := range workQueue {
		if !c.Bitfield.HasPiece(pw.index) {
			workQueue <- pw
			continue
		}

		// Download the piece
		buf, err := t.attemptDownloadPiece(c, pw)
		if err != nil {
			log.Println("Exiting", err)
			workQueue <- pw
			return
		}

		// checkIntegrity
		err = t.checkIntegrity(pw, buf)
		if err != nil {
			log.Println("Exiting", err)
			workQueue <- pw
			continue
		}
		// SendHave
		c.SendHave(pw.index)
		// Place in results
		results <- &pieceResult{pw.index, buf}
	}
}

func (t *TorrentFile) VerifyHash(buf []byte) error {
	hash := sha1.Sum(buf)

	if !bytes.Equal(hash[:], t.InfoHash[:]) {
		return fmt.Errorf("Failed integrity check")
	}

	return nil
}

func (t *TorrentFile) StartDownload(filename string) error {
	if filename == "" {
		filename = t.Name
	}

	peers, err := t.getPeers()
	if err != nil {
		return err
	}
	log.Println("Sized: ", t.Length)
	log.Println("Piece size: ", t.PieceLen)
	log.Println("Number of pieces: ", len(t.PieceHash))
	log.Println("For comparison: ", float32(t.Length)/float32(t.PieceLen))

	workQueue := make(chan *pieceWork, len(t.PieceHash))
	results := make(chan *pieceResult)

	for index, hash := range t.PieceHash {
		length := t.calculatePieceSize(index)
		workQueue <- &pieceWork{index, hash, length}
	}

	for _, peer := range peers {
		go t.startDownloadWork(peer, workQueue, results)
	}

	buf := make([]byte, t.Length)
	donePieces := 0
	for donePieces < len(t.PieceHash) {
		res := <-results
		begin, end := t.calcuateBound(res.index)
		copy(buf[begin:end], res.buf)
		donePieces++

		percent := float64(donePieces) / float64(len(t.PieceHash)) * 100
		numWorkers := runtime.NumGoroutine() - 1 // subtract 1 for main thread
		log.Printf("[%0.2f%%] Downloaded piece #%d from %d peers\n", percent, res.index, numWorkers)
	}
	close(workQueue)

	// Dump buffer to file
	err = t.VerifyHash(buf)
	if err != nil {
		return err
	}

	err = os.WriteFile("results/"+filename, buf, 0o644)
	if err != nil {
		return err
	}

	return nil
}

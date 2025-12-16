package torrentfile

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"dumb-tor-client/internal/handshake"
	"dumb-tor-client/internal/peers"

	bencode "github.com/jackpal/bencode-go"
	litter "github.com/sanity-io/litter"
)

const (
	Port uint16 = 6881
)

var pid [20]byte

type TorrentFile struct {
	Announce  string
	InfoHash  [20]byte
	PieceHash [][20]byte
	PieceLen  int
	Length    int
	Name      string
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

func Open(r io.Reader) (*TorrentFile, error) {
	bto := bencodeTorrent{}
	err := bencode.Unmarshal(r, &bto)
	if err != nil {
		return nil, err
	}

	var peerId [20]byte
	_, err = rand.Read(peerId[:])
	if err != nil {
		return nil, err
	}
	pid = peerId

	torFile, err := bto.toTorrentFile()
	if err != nil {
		return nil, err
	}

	fmt.Printf("Startring work on downloading file \"%s\"\n", torFile.Name)

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

// TODO:
// Calculate piece length

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
	url, err := t.buildTrackerURL(pid, Port)

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

func (t *TorrentFile) completeHandshake(connection net.Conn) (bool, error) {
	litter.Config.Compact = true
	hshake := handshake.Handshake{
		InfoHash: t.InfoHash,
		PeerId:   pid,
		Pstr:     "BitTorrent protocol",
	}

	buf := hshake.Serialize()

	log.Println("Handshake:\n ", litter.Sdump(hshake))
	log.Println("Sending: ", buf)

	n, err := connection.Write(buf)
	if err != nil {
		return false, err
	}
	log.Printf("Written %d bytes to peer.", n)

	responce, err := handshake.Read(connection)
	if err != nil {
		return false, err
	}

	log.Println("Got responce hshake:\n ", litter.Sdump(responce))

	if responce.InfoHash != hshake.InfoHash {
		return false, nil
	}

	return true, nil
}

func (t *TorrentFile) StartDownload() error {
	// Start TCP connection with peer
	// conn, err := net.DialTimeout("tcp", peer.String(), 3 * time.Second)
	// Complete Two-way BitTorrent handshake
	// Exchange messages to download pieces
	peers, err := t.getPeers()
	if err != nil {
		return err
	}

	rand_peer := peers[0]
	log.Println("Trying peer ", rand_peer.String())

	conn, err := net.DialTimeout("tcp", rand_peer.String(), 3*time.Second)
	if err != nil {
		return fmt.Errorf("dial %s: %w", conn, err)
	}
	defer conn.Close()

	log.Println("Connected to: ", rand_peer.String())

	if completed, err := t.completeHandshake(conn); err != nil || !completed {
		return fmt.Errorf("handshake. completed: %t, err: %w", completed, err)
	}
	log.Printf("Two-way handshake completed for %s", rand_peer)

	return nil
}

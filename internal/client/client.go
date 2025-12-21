package client

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"dumb-tor-client/internal/handshake"
	"dumb-tor-client/internal/message"
	"dumb-tor-client/internal/peers"

	"github.com/sanity-io/litter"
)

// A Bitfield represents the pieces that a peer has
type Bitfield []byte

// HasPiece tells if a bitfield has a particular index set
func (bf Bitfield) HasPiece(index int) bool {
	if bf == nil || len(bf) == 0 {
		return false
	}
	byteIndex := index / 8
	if byteIndex >= len(bf) {
		return false
	}
	offset := index % 8
	return bf[byteIndex]>>(7-offset)&1 != 0
}

// SetPiece sets a bit in the bitfield
func (bf Bitfield) SetPiece(index int) {
	byteIndex := index / 8
	offset := index % 8
	bf[byteIndex] |= 1 << (7 - offset)
}

type Client struct {
	Peer     peers.Peer
	PeerID   [20]byte
	Conn     net.Conn
	Bitfield Bitfield
	Chocked  bool
}

func (c Client) completeHandshake(infoHash [20]byte) (Bitfield, error) {
	litter.Config.Compact = true
	hshake := handshake.Handshake{
		InfoHash: infoHash,
		PeerId:   c.PeerID,
		Pstr:     "BitTorrent protocol",
	}

	buf := hshake.Serialize()

	// log.Println("Handshake:\n ", litter.Sdump(hshake))
	// log.Println("Sending: ", buf)

	_, err := c.Conn.Write(buf)
	if err != nil {
		return Bitfield{}, err
	}
	// log.Printf("Written %d bytes to peer.", n)

	responce, err := handshake.Read(c.Conn)
	if err != nil {
		return Bitfield{}, err
	}

	// log.Println("Got responce hshake:\n ", litter.Sdump(responce))

	if responce.InfoHash != hshake.InfoHash {
		return Bitfield{}, nil
	}

	bf_message, err := message.Read(c.Conn)
	if err != nil || bf_message.ID != 5 {
		return Bitfield{}, err
	}

	return bf_message.Payload, nil
}

func New(peer peers.Peer, peerID [20]byte, infoHash [20]byte) (*Client, error) {
	log.Println("Trying peer ", peer.String())

	// Start TCP connection with peer
	conn, err := net.DialTimeout("tcp", peer.String(), 3*time.Second)
	if err != nil {
		return &Client{}, fmt.Errorf("dial %s: %w", conn, err)
	}

	log.Println("Connected to: ", peer.String())

	client := Client{
		Peer:     peer,
		PeerID:   peerID,
		Conn:     conn,
		Bitfield: Bitfield{},
		Chocked:  true,
	}

	if client.Bitfield, err = client.completeHandshake(infoHash); err != nil {
		client.CloseClient()
		return nil, err
	}

	return &client, nil
}

func (c *Client) Read() (*message.Message, error) {
	if c == nil || c.Conn == nil {
		return nil, errors.New("client not initialised")
	}

	msg, err := message.Read(c.Conn)
	if err != nil {
		return &message.Message{}, err
	}
	if msg == nil || msg.ID < 0 {
		log.Println("MESSAGE IS NIL")
	}
	switch msg.ID {
	case message.MsgChoke:
		c.Chocked = true
	case message.MsgUnchoke:
		c.Chocked = false
	default:
	}

	return msg, err
}

// Memory leak maybe?
func (c *Client) CloseClient() error {
	return c.Conn.Close()
}

func (c Client) SendChoke() {
	msg := message.Message{
		ID:      message.MsgChoke,
		Payload: []byte{},
	}
	_, err := c.Conn.Write(msg.Serialize())
	// Not a good thing probably to sever connection
	if err != nil {
		// c.Conn.Close()
		log.Println("Error while sending choke message: ", err)
	}
	// log.Println("Sent choke message")
}

func (c Client) SendUnchoke() {
	msg := message.Message{
		ID:      message.MsgUnchoke,
		Payload: []byte{},
	}
	_, err := c.Conn.Write(msg.Serialize())
	// Not a good thing probably to sever connection
	if err != nil {
		// c.conn.Close()
		log.Println("Error while sending unchoke message: ", err)
	}
	// log.Println("Sent unchoke message")
}

func (c Client) SendInterested() {
	msg := message.Message{
		ID:      message.MsgInterested,
		Payload: []byte{},
	}
	_, err := c.Conn.Write(msg.Serialize())
	// Not a good thing probably to sever connection
	if err != nil {
		// c.conn.Close()
		log.Println("Error while sending interested message: ", err)
	}
	// log.Println("Sent interested message")
}

func (c Client) SendRequest(index, requested, blockSize int) error {
	buf := make([]byte, 4+4+4)

	binary.BigEndian.PutUint32(buf[0:4], uint32(index))
	binary.BigEndian.PutUint32(buf[4:8], uint32(requested))
	binary.BigEndian.PutUint32(buf[8:12], uint32(blockSize))

	msg := message.Message{
		ID:      message.MsgRequest,
		Payload: buf,
	}

	_, err := c.Conn.Write(msg.Serialize())
	if err != nil {
		// c.conn.Close()
		return fmt.Errorf("Error while sending request message: ", err)
	}
	// log.Println("Sent request message")
	return nil
}

func (c Client) SendHave(index int) error {
	idx := make([]byte, 4)
	binary.BigEndian.PutUint32(idx, uint32(index))

	msg := message.Message{
		ID:      message.MsgHave,
		Payload: idx,
	}

	_, err := c.Conn.Write(msg.Serialize())
	if err != nil {
		// c.conn.Close()
		return fmt.Errorf("Error while sending have message: ", err)
	}
	// log.Println("Sent have message")
	return nil
}

package message

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
)

type messageID uint8

const (
	MsgChoke         messageID = 0 // Means the peer isn't ready to accept messages
	MsgUnchoke       messageID = 1 // Peer is unchoked
	MsgInterested    messageID = 2
	MsgNotInterested messageID = 3
	MsgHave          messageID = 4
	MsgBitfield      messageID = 5
	MsgRequest       messageID = 6
	MsgPiece         messageID = 7
	MsgCancel        messageID = 8
)

// A message starts with a length indicator which tells us how many bytes long the message will be.
// It’s a 32-bit integer, meaning it’s made out of four bytes smooshed together in big-endian order.
// Message strores ID and payload of a message
type Message struct {
	ID      messageID
	Payload []byte
}

// Function to serialize a message struct to form
// <length_of_a_message><id><optional_payload>
func (m *Message) Serialize() []byte {
	// nil is keep-alive message
	if m == nil {
		return make([]byte, 4)
	}

	length := uint32(len(m.Payload) + 1) // + 1 for ID
	buf := make([]byte, 4+length)

	binary.BigEndian.PutUint32(buf[0:4], length)
	buf[4] = byte(m.ID)
	copy(buf[5:], m.Payload)

	return buf
}

// TODO: Error handling here
func ParseHave(msg Message) (int, error) {
	index := binary.BigEndian.Uint32(msg.Payload)
	return int(index), nil
}

func ParsePiece(expectedIndex int, buf []byte, msg Message) (int, error) {
	if msg.ID != MsgPiece {
		return 0, fmt.Errorf("expected PIECE (id 7), got %d", msg.ID)
	}
	if len(msg.Payload) < 8 {
		return 0, errors.New("piece message too short")
	}

	index := int(binary.BigEndian.Uint32(msg.Payload[0:4]))
	begin := int(binary.BigEndian.Uint32(msg.Payload[4:8]))
	block := msg.Payload[8:]

	if index != expectedIndex {
		return 0, fmt.Errorf("unexpected piece index %d (want %d)", index, expectedIndex)
	}
	if begin+len(block) > len(buf) {
		return 0, fmt.Errorf("block [%d:%d] overflows piece buffer", begin, begin+len(block))
	}

	copy(buf[begin:], block)
	return len(block), nil
}

func Read(r io.Reader) (*Message, error) {
	lengthBuf := make([]byte, 4)
	_, err := io.ReadFull(r, lengthBuf)
	if err != nil {
		log.Println("Error reading buffer")
		return nil, err
	}
	length := binary.BigEndian.Uint32(lengthBuf)

	// keep-alive message
	if length == 0 {
		return nil, nil
	}

	messageBuf := make([]byte, length)
	_, err = io.ReadFull(r, messageBuf)
	if err != nil {
		return nil, err
	}

	m := Message{
		ID:      messageID(messageBuf[0]),
		Payload: messageBuf[1:],
	}

	return &m, nil
}

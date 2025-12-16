package handshake

import (
	"fmt"
	"io"
)

type Handshake struct {
	Pstr     string // Actually always "BitTorrent protocol"
	InfoHash [20]byte
	PeerId   [20]byte
}

// Handshake string might look like this:
// \x13BitTorrent protocol\x00\x00\x00\x00\x00\x00\x00\x00\x86\xd4\xc8\x00\x24\xa4\x69\xbe\x4c\x50\xbc\x5a\x10\x2c\xf7\x17\x80\x31\x00\x74-TR2940-k8hj0wgej6ch
// Function to serialize hanshake info struct to this format
func (h *Handshake) Serialize() []byte {
	buf := make([]byte, len(h.Pstr)+49)

	buf[0] = byte(len(h.Pstr))
	buf_ptr := 1
	buf_ptr += copy(buf[buf_ptr:], h.Pstr)
	buf_ptr += copy(buf[buf_ptr:], make([]byte, 8))
	buf_ptr += copy(buf[buf_ptr:], h.InfoHash[:])
	buf_ptr += copy(buf[buf_ptr:], h.PeerId[:])

	return buf
}

// TODO: Make error handling here
// Catch broken stream
func Read(r io.Reader) (*Handshake, error) {
	newHandshakeInfo := Handshake{}

	buf := make([]byte, 68)
	r.Read(buf)

	lenPstr := uint16(buf[0])

	if lenPstr > 19 {
		err := fmt.Errorf("broken handshake. got pstr size: %d", lenPstr)
		return nil, err
	}

	buf_ptr := 1
	newHandshakeInfo.Pstr = string(buf[buf_ptr : buf_ptr+int(lenPstr)])
	if newHandshakeInfo.Pstr != "BitTorrent protocol" {
		err := fmt.Errorf("broken handshake. protocol differs from \"BitTorrent protocol\", got: %s", newHandshakeInfo.Pstr)
		return nil, err
	}

	buf_ptr += int(lenPstr)
	buf_ptr += 8 // 8 system bytes
	buf_ptr += copy(newHandshakeInfo.InfoHash[:], buf[buf_ptr:buf_ptr+20])
	buf_ptr += copy(newHandshakeInfo.PeerId[:], buf[buf_ptr:buf_ptr+20])

	return &newHandshakeInfo, nil
}

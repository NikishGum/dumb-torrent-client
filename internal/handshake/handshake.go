package handshake

import "io"

type Handshake struct {
	Pstr string 	// Actually always "BitTorrent protocol"
	InfoHash [20]byte
	PeerId [20]byte
}

func (h *Handshake) Serialize() []byte {
	buf := make([]byte, len(h.Pstr) + 49)
	buf[0] = byte(len(h.Pstr))
	buf_ptr := 1
	buf_ptr += copy(buf[buf_ptr:], h.Pstr)
	buf_ptr += copy(buf[buf_ptr:], make([]byte, 8))
	buf_ptr += copy(buf[buf_ptr:], h.InfoHash[:])
	buf_ptr += copy(buf[buf_ptr:], h.PeerId[:])
	return buf
}

func Read(r io.Reader) (*Handshake, error) {
	newHandshakeInfo := Handshake{}

	buf := make([]byte, 50)
	r.Read(buf)
	
	
	lenPstr := uint16(buf[0])
	buf_ptr := 1
	newHandshakeInfo.Pstr = string(buf[buf_ptr:buf_ptr + lenPstr +1])
	buf_ptr += int(lenPstr) + 1
	newHandshakeInfo.PeerId = 

}
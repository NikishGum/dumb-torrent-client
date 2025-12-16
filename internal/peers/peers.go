package peers

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
)

type Peer struct {
	IP   net.IP
	Port uint16
}

type PeerResponce struct {
	Interval uint16 `bencode:"interval"`
	PeersBin string `bencode:"peers"`
}

func Unmarshal(peersBin []byte) ([]Peer, error) {
	// peersBin = (example) [192, 0. 2, 123, 0x1A, 0xE1, 192, ...]
	//                        0   1  2   3    4     5

	// IP and port len

	log.Printf("got peerBin sized %d", len(peersBin))

	lenPeer := 6

	peers := []Peer{}

	if len(peersBin)%lenPeer != 0 {
		err := fmt.Errorf("Bad peersBin array, got size %s bytes", len(peersBin))
		return []Peer{}, err
	}
	numPeers := len(peersBin) / lenPeer

	log.Printf("Number of peers: %d", numPeers)

	for i := 0; i < numPeers; i += 1 {
		offset := i * lenPeer

		log.Printf("Iteration #%d", i)
		peer_ip := net.IPv4(peersBin[offset], peersBin[offset+1], peersBin[offset+2], peersBin[offset+3])
		log.Println("Got ip: ", peer_ip)

		peer_port := binary.BigEndian.Uint16(peersBin[offset+4 : offset+6])
		log.Println("Got port: ", peer_port)

		peers = append(peers, Peer{
			IP:   peer_ip,
			Port: peer_port,
		})
	}

	return peers, nil
}

func (p Peer) String() string {
	return p.IP.String() + ":" + fmt.Sprintf("%d", p.Port)
}

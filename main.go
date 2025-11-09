package main

import (
	"fmt"
	"os"
	//"encoding/binary"
	//"fmt"
	"dumb-tor-client/internal/torrentfile"
)

func main() {
	fp, err := os.Open("debian-13.1.0-amd64-netinst.iso.torrent")
	if err != nil {
		panic(err)
	}
	defer func() {
		fp.Close()
	}()

	tfile, err := torrentfile.Open(fp)
	if err != nil {
		panic(err)
	}

	fmt.Println(tfile.GetPeers())
}

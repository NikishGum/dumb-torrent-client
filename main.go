package main

import (
	"fmt"
	"os"

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
		// Probably not the best error handling practice
		fmt.Println("Got error: ", err)
		return
	}

	err = tfile.StartDownload()
	if err != nil {
		fmt.Println("Got error: ", err)
		return
	}
}

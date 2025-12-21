package main

import (
	"fmt"
	"os"

	"dumb-tor-client/internal/torrentfile"
)

func main() {
	args := os.Args[1:]

	fp, err := os.Open(args[0])
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

	err = tfile.StartDownload(args[1])
	if err != nil {
		fmt.Println("Got error: ", err)
		return
	}
}

package main

import (
	"fmt"
	"os"

	"remote-pull/internal/transfer"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Printf("Usage: %s <image> <user@host>\n", os.Args[0])
		os.Exit(1)
	}

	imageName := os.Args[1]
	remoteServer := os.Args[2]

	if err := transfer.TransferImage(imageName, remoteServer); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

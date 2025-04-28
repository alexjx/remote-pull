package main

import (
	"flag"
	"fmt"
	"os"

	"remote-pull/internal/transfer"
)

func main() {
	// Define flags
	skipPull := flag.Bool("skip-pull", false, "Skip pulling the image locally before transfer")

	// Parse flags but keep positional args
	flag.Parse()
	args := flag.Args()

	if len(args) != 2 {
		fmt.Printf("Usage: %s [OPTIONS] <image> <user@host>\n\n", os.Args[0])
		fmt.Println("Options:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	imageName := args[0]
	remoteServer := args[1]

	if err := transfer.TransferImage(imageName, remoteServer, *skipPull); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

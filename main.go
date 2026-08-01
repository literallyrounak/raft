package main

import (
	"fmt"
	"os"

	"raft/internal/transfer"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "share":
		runShare(os.Args[2:])
	case "receive":
		runReceive(os.Args[2:])
	default:
		printUsage()
		os.Exit(1)
	}
}

func runShare(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: raft share <file> [listen-addr]")
		os.Exit(1)
	}

	filePath := args[0]
	addr := ":9876"
	if len(args) >= 2 {
		addr = args[1]
	}

	if err := transfer.Share(filePath, addr); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runReceive(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: raft receive <addr> [out-dir]")
		os.Exit(1)
	}

	addr := args[0]
	outDir := "."
	if len(args) >= 2 {
		outDir = args[1]
	}

	if err := transfer.Receive(addr, outDir); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("raft - direct device-to-device file transfer")
	fmt.Println()
	fmt.Println("usage:")
	fmt.Println("  raft share <file> [listen-addr]     (default listen-addr :9876)")
	fmt.Println("  raft receive <addr> [out-dir]       (default out-dir .)")
}

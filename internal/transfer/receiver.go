package transfer

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"raft/internal/protocol"
)

func Receive(addr string, outDir string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("connecting to %s: %w", addr, err)
	}
	defer conn.Close()

	msgType, payload, err := protocol.ReadFrame(conn)
	if err != nil {
		return fmt.Errorf("reading handshake: %w", err)
	}
	if msgType != protocol.MsgHandshake {
		return fmt.Errorf("expected handshake, got message type %d", msgType)
	}

	manifest, err := protocol.DecodeManifest(payload)
	if err != nil {
		return fmt.Errorf("decoding manifest: %w", err)
	}

	fmt.Printf("receiving %s (%d bytes, %d chunks)\n", manifest.FileName, manifest.FileSize, len(manifest.ChunkHashes))

	outPath := filepath.Join(outDir, manifest.FileName)
	transferID := manifestTransferID(manifest)

	startIndex := uint32(0)
	if st, err := loadTransferState(outPath); err == nil && st.TransferID == transferID {
		startIndex = st.NextIndex
		fmt.Printf("resuming transfer from chunk %d\n", startIndex)
	}

	openFlags := os.O_CREATE | os.O_RDWR
	if startIndex == 0 {
		openFlags |= os.O_TRUNC
	}
	outFile, err := os.OpenFile(outPath, openFlags, 0644)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer outFile.Close()

	if err := protocol.WriteFrame(conn, protocol.MsgStartFrom, protocol.EncodeIndex(startIndex)); err != nil {
		return fmt.Errorf("sending start-from: %w", err)
	}

	fmt.Println("type 'p' + enter to pause, 'r' + enter to resume")
	go listenForPauseInput(conn)

	for {
		msgType, payload, err := protocol.ReadFrame(conn)
		if err != nil {
			return fmt.Errorf("reading frame: %w", err)
		}

		if msgType == protocol.MsgComplete {
			break
		}

		if msgType != protocol.MsgChunk {
			return fmt.Errorf("unexpected message type %d", msgType)
		}

		index, data, err := protocol.DecodeChunk(payload)
		if err != nil {
			return fmt.Errorf("decoding chunk: %w", err)
		}

		if int(index) >= len(manifest.ChunkHashes) {
			return fmt.Errorf("chunk index %d out of range", index)
		}

		sum := sha256.Sum256(data)
		gotHash := hex.EncodeToString(sum[:])
		wantHash := manifest.ChunkHashes[index]
		if gotHash != wantHash {
			return fmt.Errorf("chunk %d failed integrity check", index)
		}

		offset := int64(index) * manifest.ChunkSize
		if _, err := outFile.WriteAt(data, offset); err != nil {
			return fmt.Errorf("writing chunk %d: %w", index, err)
		}

		if err := saveTransferState(outPath, transferState{TransferID: transferID, NextIndex: index + 1}); err != nil {
			return fmt.Errorf("saving transfer state: %w", err)
		}

		fmt.Printf("\rreceived chunk %d/%d", index+1, len(manifest.ChunkHashes))
	}

	deleteTransferState(outPath)
	fmt.Println()
	fmt.Printf("saved to %s\n", outPath)
	return nil
}

func listenForPauseInput(conn net.Conn) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch line {
		case "p":
			protocol.WriteFrame(conn, protocol.MsgPause, nil)
		case "r":
			protocol.WriteFrame(conn, protocol.MsgResume, nil)
		}
	}
}

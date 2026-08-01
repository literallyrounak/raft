package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"

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
	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer outFile.Close()

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

		fmt.Printf("\rreceived chunk %d/%d", index+1, len(manifest.ChunkHashes))
	}

	fmt.Println()
	fmt.Printf("saved to %s\n", outPath)
	return nil
}

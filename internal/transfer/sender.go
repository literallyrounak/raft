package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"

	"raft/internal/protocol"
)

func Share(filePath string, addr string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}

	hashes, err := computeChunkHashes(file, info.Size())
	if err != nil {
		return fmt.Errorf("hashing file: %w", err)
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", addr, err)
	}
	defer listener.Close()

	fmt.Printf("waiting for a peer on %s\n", listener.Addr().String())

	conn, err := listener.Accept()
	if err != nil {
		return fmt.Errorf("accepting connection: %w", err)
	}
	defer conn.Close()

	fmt.Printf("peer connected: %s\n", conn.RemoteAddr().String())

	manifest := protocol.Manifest{
		FileName:    filepath.Base(filePath),
		FileSize:    info.Size(),
		ChunkSize:   protocol.ChunkSize,
		ChunkHashes: hashes,
	}

	manifestBytes, err := protocol.EncodeManifest(manifest)
	if err != nil {
		return fmt.Errorf("encoding manifest: %w", err)
	}

	if err := protocol.WriteFrame(conn, protocol.MsgHandshake, manifestBytes); err != nil {
		return fmt.Errorf("sending handshake: %w", err)
	}

	startIndex, err := readStartFrom(conn)
	if err != nil {
		return fmt.Errorf("reading start-from: %w", err)
	}

	startOffset := int64(startIndex) * protocol.ChunkSize
	if _, err := file.Seek(startOffset, io.SeekStart); err != nil {
		return fmt.Errorf("seeking file: %w", err)
	}

	gate := newPauseGate()
	controlErrCh := make(chan error, 1)
	go listenForControlMessages(conn, gate, controlErrCh)

	if err := sendChunks(conn, file, len(hashes), startIndex, gate); err != nil {
		return fmt.Errorf("sending chunks: %w", err)
	}

	if err := protocol.WriteFrame(conn, protocol.MsgComplete, nil); err != nil {
		return fmt.Errorf("sending complete: %w", err)
	}

	fmt.Println("transfer complete")
	return nil
}

func readStartFrom(conn net.Conn) (uint32, error) {
	msgType, payload, err := protocol.ReadFrame(conn)
	if err != nil {
		return 0, err
	}
	if msgType != protocol.MsgStartFrom {
		return 0, fmt.Errorf("expected start-from, got message type %d", msgType)
	}
	return protocol.DecodeIndex(payload)
}

func listenForControlMessages(conn net.Conn, gate *pauseGate, errCh chan<- error) {
	for {
		msgType, _, err := protocol.ReadFrame(conn)
		if err != nil {
			errCh <- err
			return
		}
		switch msgType {
		case protocol.MsgPause:
			fmt.Println("\npeer requested pause")
			gate.Pause()
		case protocol.MsgResume:
			fmt.Println("\npeer requested resume")
			gate.Resume()
		}
	}
}

func computeChunkHashes(file *os.File, size int64) ([]string, error) {
	var hashes []string
	buf := make([]byte, protocol.ChunkSize)

	for {
		n, err := file.Read(buf)
		if n > 0 {
			sum := sha256.Sum256(buf[:n])
			hashes = append(hashes, hex.EncodeToString(sum[:]))
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	return hashes, nil
}

func sendChunks(conn net.Conn, file *os.File, totalChunks int, startIndex uint32, gate *pauseGate) error {
	buf := make([]byte, protocol.ChunkSize)
	index := startIndex

	for {
		n, err := file.Read(buf)
		if n > 0 {
			gate.WaitIfPaused()
			payload := protocol.EncodeChunk(index, buf[:n])
			if writeErr := protocol.WriteFrame(conn, protocol.MsgChunk, payload); writeErr != nil {
				return writeErr
			}
			fmt.Printf("\rsent chunk %d/%d", index+1, totalChunks)
			index++
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	fmt.Println()
	return nil
}

package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net"
	"os"
	"path/filepath"

	"raft/internal/protocol"
	"raft/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func Share(filePath string, addr string, msgs chan<- tea.Msg) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	msgs <- ui.StatusMsg{Status: ui.StatusHashing}
	hashes, err := computeChunkHashes(file, info.Size())
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	msgs <- ui.StatusMsg{Status: ui.StatusWaiting}

	conn, err := listener.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()

	msgs <- ui.PeerConnectedMsg{Addr: conn.RemoteAddr().String()}

	manifest := protocol.Manifest{
		FileName:    filepath.Base(filePath),
		FileSize:    info.Size(),
		ChunkSize:   protocol.ChunkSize,
		ChunkHashes: hashes,
	}

	manifestBytes, err := protocol.EncodeManifest(manifest)
	if err != nil {
		return err
	}

	if err := protocol.WriteFrame(conn, protocol.MsgHandshake, manifestBytes); err != nil {
		return err
	}

	startIndex, err := readStartFrom(conn)
	if err != nil {
		return err
	}

	msgs <- ui.FileInfoMsg{
		Name:  manifest.FileName,
		Size:  info.Size(),
		Total: len(hashes),
	}

	startOffset := int64(startIndex) * protocol.ChunkSize
	if _, err := file.Seek(startOffset, io.SeekStart); err != nil {
		return err
	}

	gate := newPauseGate(msgs)
	go listenForControlMessages(conn, gate)

	if err := sendChunks(conn, file, info.Size(), startOffset, startIndex, gate, msgs); err != nil {
		return err
	}

	if err := protocol.WriteFrame(conn, protocol.MsgComplete, nil); err != nil {
		return err
	}

	msgs <- ui.DoneMsg{}
	return nil
}

func readStartFrom(conn net.Conn) (uint32, error) {
	msgType, payload, err := protocol.ReadFrame(conn)
	if err != nil {
		return 0, err
	}
	if msgType != protocol.MsgStartFrom {
		return 0, nil
	}
	return protocol.DecodeIndex(payload)
}

func listenForControlMessages(conn net.Conn, gate *pauseGate) {
	for {
		msgType, _, err := protocol.ReadFrame(conn)
		if err != nil {
			return
		}
		switch msgType {
		case protocol.MsgPause:
			gate.Pause()
		case protocol.MsgResume:
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

func sendChunks(conn net.Conn, file *os.File, fileSize int64, startOffset int64, startIndex uint32, gate *pauseGate, msgs chan<- tea.Msg) error {
	buf := make([]byte, protocol.ChunkSize)
	index := startIndex
	transferred := startOffset

	for {
		n, err := file.Read(buf)
		if n > 0 {
			gate.WaitIfPaused()
			payload := protocol.EncodeChunk(index, buf[:n])
			if writeErr := protocol.WriteFrame(conn, protocol.MsgChunk, payload); writeErr != nil {
				return writeErr
			}
			transferred += int64(n)
			msgs <- ui.ProgressMsg{Transferred: transferred}
			index++
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

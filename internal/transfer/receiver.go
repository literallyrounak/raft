package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"os"
	"path/filepath"

	"raft/internal/protocol"
	"raft/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func Receive(addr string, outDir string, msgs chan<- tea.Msg, ctrl <-chan ui.ControlCmd) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	go forwardControlCmds(conn, ctrl)

	msgs <- ui.PeerConnectedMsg{Addr: addr}

	msgType, payload, err := protocol.ReadFrame(conn)
	if err != nil {
		return err
	}
	if msgType != protocol.MsgHandshake {
		return nil
	}

	manifest, err := protocol.DecodeManifest(payload)
	if err != nil {
		return err
	}

	outPath := filepath.Join(outDir, manifest.FileName)
	transferID := manifestTransferID(manifest)

	startIndex := uint32(0)
	if st, err := loadTransferState(outPath); err == nil && st.TransferID == transferID {
		startIndex = st.NextIndex
	}

	openFlags := os.O_CREATE | os.O_RDWR
	if startIndex == 0 {
		openFlags |= os.O_TRUNC
	}
	outFile, err := os.OpenFile(outPath, openFlags, 0644)
	if err != nil {
		return err
	}
	defer outFile.Close()

	if err := protocol.WriteFrame(conn, protocol.MsgStartFrom, protocol.EncodeIndex(startIndex)); err != nil {
		return err
	}

	msgs <- ui.FileInfoMsg{
		Name:  manifest.FileName,
		Size:  manifest.FileSize,
		Total: len(manifest.ChunkHashes),
	}

	transferred := int64(startIndex) * manifest.ChunkSize
	msgs <- ui.ProgressMsg{Transferred: transferred}

	for {
		msgType, payload, err := protocol.ReadFrame(conn)
		if err != nil {
			return err
		}

		if msgType == protocol.MsgComplete {
			break
		}

		if msgType != protocol.MsgChunk {
			continue
		}

		index, data, err := protocol.DecodeChunk(payload)
		if err != nil {
			return err
		}

		if int(index) >= len(manifest.ChunkHashes) {
			continue
		}

		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != manifest.ChunkHashes[index] {
			return nil
		}

		offset := int64(index) * manifest.ChunkSize
		if _, err := outFile.WriteAt(data, offset); err != nil {
			return err
		}

		if err := saveTransferState(outPath, transferState{TransferID: transferID, NextIndex: index + 1}); err != nil {
			return err
		}

		transferred += int64(len(data))
		msgs <- ui.ProgressMsg{Transferred: transferred}
	}

	deleteTransferState(outPath)
	msgs <- ui.DoneMsg{OutPath: outPath}
	return nil
}

func forwardControlCmds(conn net.Conn, ctrl <-chan ui.ControlCmd) {
	for cmd := range ctrl {
		switch cmd {
		case ui.CmdPause:
			protocol.WriteFrame(conn, protocol.MsgPause, nil)
		case ui.CmdResume:
			protocol.WriteFrame(conn, protocol.MsgResume, nil)
		}
	}
}

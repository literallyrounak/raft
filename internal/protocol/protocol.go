package protocol

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
)

const (
	MsgHandshake byte = iota
	MsgChunk
	MsgComplete
	MsgError
	MsgStartFrom
	MsgPause
	MsgResume
)

const ChunkSize = 4 * 1024 * 1024

const MaxFrameSize = ChunkSize + 1024

type Manifest struct {
	FileName    string   `json:"file_name"`
	FileSize    int64    `json:"file_size"`
	ChunkSize   int64    `json:"chunk_size"`
	ChunkHashes []string `json:"chunk_hashes"`
}

func WriteFrame(w io.Writer, msgType byte, payload []byte) error {
	header := make([]byte, 5)
	binary.BigEndian.PutUint32(header[0:4], uint32(len(payload)))
	header[4] = msgType
	if _, err := w.Write(header); err != nil {
		return err
	}
	if len(payload) == 0 {
		return nil
	}
	_, err := w.Write(payload)
	return err
}

func ReadFrame(r io.Reader) (byte, []byte, error) {
	header := make([]byte, 5)
	if _, err := io.ReadFull(r, header); err != nil {
		return 0, nil, err
	}
	length := binary.BigEndian.Uint32(header[0:4])
	msgType := header[4]
	if length > MaxFrameSize {
		return 0, nil, errors.New("protocol: frame exceeds max size")
	}
	if length == 0 {
		return msgType, nil, nil
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return msgType, payload, nil
}

func EncodeManifest(m Manifest) ([]byte, error) {
	return json.Marshal(m)
}

func DecodeManifest(data []byte) (Manifest, error) {
	var m Manifest
	err := json.Unmarshal(data, &m)
	return m, err
}

func EncodeChunk(index uint32, data []byte) []byte {
	buf := make([]byte, 4+len(data))
	binary.BigEndian.PutUint32(buf[0:4], index)
	copy(buf[4:], data)
	return buf
}

func DecodeChunk(payload []byte) (uint32, []byte, error) {
	if len(payload) < 4 {
		return 0, nil, errors.New("protocol: chunk payload too short")
	}
	index := binary.BigEndian.Uint32(payload[0:4])
	return index, payload[4:], nil
}

func EncodeIndex(index uint32) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, index)
	return buf
}

func DecodeIndex(payload []byte) (uint32, error) {
	if len(payload) != 4 {
		return 0, errors.New("protocol: index payload must be 4 bytes")
	}
	return binary.BigEndian.Uint32(payload), nil
}

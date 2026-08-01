package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"

	"raft/internal/protocol"
)

type transferState struct {
	TransferID string `json:"transfer_id"`
	NextIndex  uint32 `json:"next_index"`
}

func stateFilePath(outPath string) string {
	return outPath + ".p2pstate"
}

func manifestTransferID(m protocol.Manifest) string {
	h := sha256.New()
	h.Write([]byte(m.FileName))
	for _, chunkHash := range m.ChunkHashes {
		h.Write([]byte(chunkHash))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func loadTransferState(outPath string) (*transferState, error) {
	data, err := os.ReadFile(stateFilePath(outPath))
	if err != nil {
		return nil, err
	}
	var st transferState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

func saveTransferState(outPath string, st transferState) error {
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return os.WriteFile(stateFilePath(outPath), data, 0644)
}

func deleteTransferState(outPath string) {
	os.Remove(stateFilePath(outPath))
}

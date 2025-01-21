package brunch

import (
	"encoding/json"
	"fmt"
	"os"
)

type Snapshot struct {
	ActiveBranch string `json:"active_branch"`
	Contents     []byte `json:"contents"`
}

func (s *Snapshot) Save(filepath string) error {
	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open snapshot file: %w", err)
	}
	defer file.Close()

	b, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	_, err = file.Write(b)
	if err != nil {
		return fmt.Errorf("failed to write snapshot: %w", err)
	}

	return nil
}

func LoadSnapshot(filepath string) (*Snapshot, error) {
	file, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot file: %w", err)
	}

	var snapshot Snapshot
	if err := json.Unmarshal(file, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}

	return &snapshot, nil
}

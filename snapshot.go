package brunch

import (
	"encoding/json"
	"fmt"
)

type Snapshot struct {
	ProviderName string   `json:"provider_name"`
	ActiveBranch string   `json:"active_branch"`
	Contents     []byte   `json:"contents"`
	Contexts     []string `json:"contexts"`
}

func (s *Snapshot) Marshal() ([]byte, error) {
	return json.Marshal(s)
}

func SnapshotFromJSON(data []byte) (*Snapshot, error) {
	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}
	return &snapshot, nil
}

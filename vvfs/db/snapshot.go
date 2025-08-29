package db

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/trees"

	"github.com/google/uuid"
)

type Snapshot struct {
	ID             uuid.UUID
	TakenAt        time.Time
	DirectoryState []byte
}

type snapshotJSON struct {
	ID             string `json:"id"`
	TakenAt        string `json:"taken_at"`
	DirectoryState []byte `json:"directory_state"`
}

func (cd *CentralDBProvider) TakeSnapshot(tree *trees.DirectoryTree) error {
	state, err := tree.MarshalJSON()
	if err != nil {
		return fmt.Errorf("error marshalling directory tree: %w", err)
	}

	_, err = cd.db.Exec("INSERT INTO snapshots (id, taken_at, directory_state) VALUES ($1, $2, $3)", uuid.New(), time.Now(), state)
	if err != nil {
		return fmt.Errorf("error inserting snapshot into database: %w", err)
	}

	return nil
}

func (cd *CentralDBProvider) RestoreSnapshot(snapshotID uuid.UUID) (*trees.DirectoryTree, error) {
	snapshot, err := cd.GetSnapshot(snapshotID)
	if err != nil {
		return nil, fmt.Errorf("error getting snapshot: %w", err)
	}

	tree := &trees.DirectoryTree{}
	err = tree.UnMarshalJSON(snapshot.DirectoryState)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling directory tree: %w", err)
	}

	// cd.DirectoryTree = tree // Removed: Caller will manage the tree state.
	return tree, nil
}

// GetSnapshot has been moved to centraldbprovider.go for better implementation

func (cd *CentralDBProvider) GetSnapshots() ([]Snapshot, error) {
	rows, err := cd.db.Query("SELECT id, taken_at, directory_state FROM snapshots")
	if err != nil {
		return nil, fmt.Errorf("error querying snapshots: %w", err)
	}
	defer rows.Close()

	var snapshots []Snapshot
	for rows.Next() {
		var snap Snapshot
		err := rows.Scan(&snap.ID, &snap.TakenAt, &snap.DirectoryState)
		if err != nil {
			return nil, fmt.Errorf("error scanning snapshot: %w", err)
		}

		snapshots = append(snapshots, snap)
	}

	return snapshots, nil
}

func (sn *Snapshot) MarshalJSON() ([]byte, error) {
	return json.Marshal(snapshotJSON{
		ID:             sn.ID.String(),
		TakenAt:        sn.TakenAt.Format(time.RFC3339),
		DirectoryState: sn.DirectoryState,
	})
}

func (sn *Snapshot) UnMarshalJSON(data []byte) error {
	var snap snapshotJSON

	if err := json.Unmarshal(data, &snap); err != nil {
		return fmt.Errorf("error unmarshalling snapshot: %w", err)
	}

	takenAt, err := time.Parse(time.RFC3339, snap.TakenAt)
	if err != nil {
		return fmt.Errorf("error parsing time: %w", err)
	}

	sn.ID, err = uuid.Parse(snap.ID)
	sn.TakenAt = takenAt
	sn.DirectoryState = snap.DirectoryState

	return nil
}

package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"

	statetypes "github.com/runfabric/runfabric/extensions/types"
	_ "modernc.org/sqlite"
)

const createTableSQL = `CREATE TABLE IF NOT EXISTS runfabric_receipts (
	workspace_id TEXT NOT NULL,
	stage TEXT NOT NULL,
	data TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (workspace_id, stage)
);`

// ReceiptBackend stores deploy receipts in SQLite (one DB file per workspace or shared path).
type ReceiptBackend struct {
	db          *sql.DB
	workspaceID string
}

// NewReceiptBackend opens the SQLite DB at path (use filepath.Join(root, sqlitePath) when path is relative).
// workspaceID is the root path or a stable id for the workspace.
func NewReceiptBackend(path, workspaceID string) (*ReceiptBackend, error) {
	if workspaceID == "" {
		workspaceID = path
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := db.Exec(createTableSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create table: %w", err)
	}
	return &ReceiptBackend{db: db, workspaceID: workspaceID}, nil
}

func (b *ReceiptBackend) Load(stage string) (*statetypes.Receipt, error) {
	var data string
	err := b.db.QueryRow(
		`SELECT data FROM runfabric_receipts WHERE workspace_id = ? AND stage = ?`,
		b.workspaceID, stage,
	).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var r statetypes.Receipt
	if err := json.Unmarshal([]byte(data), &r); err != nil {
		return nil, fmt.Errorf("unmarshal receipt: %w", err)
	}
	return &r, nil
}

func (b *ReceiptBackend) Save(receipt *statetypes.Receipt) error {
	if receipt == nil {
		return fmt.Errorf("nil receipt")
	}
	data, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	_, err = b.db.Exec(
		`INSERT INTO runfabric_receipts (workspace_id, stage, data, updated_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(workspace_id, stage) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at`,
		b.workspaceID, receipt.Stage, string(data), receipt.UpdatedAt,
	)
	return err
}

func (b *ReceiptBackend) Delete(stage string) error {
	_, err := b.db.Exec(`DELETE FROM runfabric_receipts WHERE workspace_id = ? AND stage = ?`, b.workspaceID, stage)
	return err
}

func (b *ReceiptBackend) ListReleases() ([]statetypes.ReleaseEntry, error) {
	rows, err := b.db.Query(
		`SELECT stage, updated_at FROM runfabric_receipts WHERE workspace_id = ? ORDER BY updated_at DESC`,
		b.workspaceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []statetypes.ReleaseEntry
	for rows.Next() {
		var e statetypes.ReleaseEntry
		if err := rows.Scan(&e.Stage, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (b *ReceiptBackend) Kind() string {
	return "sqlite"
}

func (b *ReceiptBackend) Close() error {
	if b.db != nil {
		return b.db.Close()
	}
	return nil
}

// ResolvePath returns an absolute path for the SQLite file (joining root and path if path is relative).
func ResolvePath(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

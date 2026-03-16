package postgres

import (
	"database/sql"
	"encoding/json"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/runfabric/runfabric/engine/internal/state"
)

const createTableSQL = `CREATE TABLE IF NOT EXISTS runfabric_receipts (
	workspace_id TEXT NOT NULL,
	stage TEXT NOT NULL,
	data JSONB NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (workspace_id, stage)
);`

// ReceiptBackend stores deploy receipts in Postgres.
type ReceiptBackend struct {
	db          *sql.DB
	table       string
	workspaceID string
}

// NewReceiptBackend opens a Postgres connection and ensures the receipts table exists.
// tableName defaults to "runfabric_receipts" if empty.
func NewReceiptBackend(dsn, tableName, workspaceID string) (*ReceiptBackend, error) {
	if tableName == "" {
		tableName = "runfabric_receipts"
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	createSQL := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		workspace_id TEXT NOT NULL,
		stage TEXT NOT NULL,
		data JSONB NOT NULL,
		updated_at TEXT NOT NULL,
		PRIMARY KEY (workspace_id, stage)
	);`, tableName)
	if _, err := db.Exec(createSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create table: %w", err)
	}
	return &ReceiptBackend{db: db, table: tableName, workspaceID: workspaceID}, nil
}

func (b *ReceiptBackend) Load(stage string) (*state.Receipt, error) {
	var data []byte
	err := b.db.QueryRow(
		fmt.Sprintf(`SELECT data FROM %s WHERE workspace_id = $1 AND stage = $2`, b.table),
		b.workspaceID, stage,
	).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var r state.Receipt
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("unmarshal receipt: %w", err)
	}
	return &r, nil
}

func (b *ReceiptBackend) Save(receipt *state.Receipt) error {
	if receipt == nil {
		return fmt.Errorf("nil receipt")
	}
	data, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	_, err = b.db.Exec(
		fmt.Sprintf(`INSERT INTO %s (workspace_id, stage, data, updated_at) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (workspace_id, stage) DO UPDATE SET data = EXCLUDED.data, updated_at = EXCLUDED.updated_at`, b.table),
		b.workspaceID, receipt.Stage, data, receipt.UpdatedAt,
	)
	return err
}

func (b *ReceiptBackend) Delete(stage string) error {
	_, err := b.db.Exec(
		fmt.Sprintf(`DELETE FROM %s WHERE workspace_id = $1 AND stage = $2`, b.table),
		b.workspaceID, stage,
	)
	return err
}

func (b *ReceiptBackend) ListReleases() ([]state.ReleaseEntry, error) {
	rows, err := b.db.Query(
		fmt.Sprintf(`SELECT stage, updated_at FROM %s WHERE workspace_id = $1 ORDER BY updated_at DESC`, b.table),
		b.workspaceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []state.ReleaseEntry
	for rows.Next() {
		var e state.ReleaseEntry
		if err := rows.Scan(&e.Stage, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (b *ReceiptBackend) Kind() string {
	return "postgres"
}

func (b *ReceiptBackend) Close() error {
	if b.db != nil {
		return b.db.Close()
	}
	return nil
}

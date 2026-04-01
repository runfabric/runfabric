package states

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	dynbackend "github.com/runfabric/runfabric/extensions/states/dynamodb"
	localbackend "github.com/runfabric/runfabric/extensions/states/local"
	pgbackend "github.com/runfabric/runfabric/extensions/states/postgres"
	s3backend "github.com/runfabric/runfabric/extensions/states/s3"
	sqlitebackend "github.com/runfabric/runfabric/extensions/states/sqlite"
	extstate "github.com/runfabric/runfabric/extensions/types"
	sdkrouter "github.com/runfabric/runfabric/plugin-sdk/go/router"
	sdkstate "github.com/runfabric/runfabric/plugin-sdk/go/state"
)

const (
	defaultSQLitePath = ".runfabric/state.db"
)

type statePlugin struct {
	meta    sdkrouter.PluginMeta
	lock    lockBackend
	journal journalBackend
	receipt receiptBackend
}

type lockBackend interface {
	Acquire(service, stage, operation string, staleAfter time.Duration) (*extstate.Handle, error)
	Read(service, stage string) (*extstate.LockRecord, error)
	Release(service, stage string) error
}

type journalBackend interface {
	Load(service, stage string) (*extstate.JournalFile, error)
	Save(journal *extstate.JournalFile) error
	Delete(service, stage string) error
}

type receiptBackend interface {
	Load(stage string) (*extstate.Receipt, error)
	Save(receipt *extstate.Receipt) error
	Delete(stage string) error
	ListReleases() ([]extstate.ReleaseEntry, error)
}

func (p *statePlugin) Meta() sdkrouter.PluginMeta {
	return p.meta
}

func (p *statePlugin) LockAcquire(_ context.Context, service, stage, operation string, staleAfterMillis int64) (*sdkstate.LockRecord, error) {
	h, err := p.lock.Acquire(service, stage, operation, sdkstate.MillisToDuration(staleAfterMillis))
	if err != nil {
		return nil, err
	}
	if h == nil {
		return nil, nil
	}
	r, err := p.lock.Read(service, stage)
	if err != nil {
		return &sdkstate.LockRecord{Service: service, Stage: stage, Operation: operation, OwnerToken: strings.TrimSpace(h.OwnerToken)}, nil
	}
	return toSDKLockRecord(r), nil
}

func (p *statePlugin) LockRead(_ context.Context, service, stage string) (*sdkstate.LockRecord, error) {
	r, err := p.lock.Read(service, stage)
	if err != nil {
		return nil, err
	}
	return toSDKLockRecord(r), nil
}

func (p *statePlugin) LockRelease(_ context.Context, service, stage string) error {
	return p.lock.Release(service, stage)
}

func (p *statePlugin) JournalLoad(_ context.Context, service, stage string) (*sdkstate.JournalFile, error) {
	j, err := p.journal.Load(service, stage)
	if err != nil {
		return nil, err
	}
	if j == nil {
		return nil, nil
	}
	return toSDKJournalFile(j)
}

func (p *statePlugin) JournalSave(_ context.Context, journal *sdkstate.JournalFile) error {
	j, err := fromSDKJournalFile(journal)
	if err != nil {
		return err
	}
	return p.journal.Save(j)
}

func (p *statePlugin) JournalDelete(_ context.Context, service, stage string) error {
	return p.journal.Delete(service, stage)
}

func (p *statePlugin) ReceiptLoad(_ context.Context, stage string) (*sdkstate.Receipt, error) {
	r, err := p.receipt.Load(stage)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, nil
	}
	return toSDKReceipt(r)
}

func (p *statePlugin) ReceiptSave(_ context.Context, receipt *sdkstate.Receipt) error {
	r, err := fromSDKReceipt(receipt)
	if err != nil {
		return err
	}
	return p.receipt.Save(r)
}

func (p *statePlugin) ReceiptDelete(_ context.Context, stage string) error {
	return p.receipt.Delete(stage)
}

func (p *statePlugin) ReceiptListReleases(_ context.Context) ([]sdkstate.ReleaseEntry, error) {
	list, err := p.receipt.ListReleases()
	if err != nil {
		return nil, err
	}
	out := make([]sdkstate.ReleaseEntry, 0, len(list))
	for _, entry := range list {
		out = append(out, sdkstate.ReleaseEntry{Stage: entry.Stage, UpdatedAt: entry.UpdatedAt})
	}
	return out, nil
}

// NewLocalTransportPlugin creates the external state plugin for the local backend.
func NewLocalTransportPlugin() sdkstate.Plugin {
	root := stateRoot()
	return &statePlugin{
		meta: sdkrouter.PluginMeta{ID: "local", Name: "Local State Backend", Description: "Local state backend plugin"},
		lock: localbackend.NewLockBackend(root), journal: localbackend.NewJournalBackend(root), receipt: localbackend.NewReceiptBackend(root),
	}
}

// NewSQLiteTransportPlugin creates the external state plugin for the sqlite backend.
func NewSQLiteTransportPlugin() (sdkstate.Plugin, error) {
	root := stateRoot()
	path := sqlitebackend.ResolvePath(root, firstNonEmpty(strings.TrimSpace(os.Getenv("RUNFABRIC_STATE_SQLITE_PATH")), defaultSQLitePath))
	receipts, err := sqlitebackend.NewReceiptBackend(path, root)
	if err != nil {
		return nil, err
	}
	return &statePlugin{
		meta: sdkrouter.PluginMeta{ID: "sqlite", Name: "SQLite State Backend", Description: "SQLite receipts with local locks/journals"},
		lock: localbackend.NewLockBackend(root), journal: localbackend.NewJournalBackend(root), receipt: receipts,
	}, nil
}

// NewPostgresTransportPlugin creates the external state plugin for the postgres backend.
func NewPostgresTransportPlugin() (sdkstate.Plugin, error) {
	root := stateRoot()
	dsn := strings.TrimSpace(os.Getenv("RUNFABRIC_STATE_POSTGRES_DSN"))
	if dsn == "" {
		return nil, fmt.Errorf("RUNFABRIC_STATE_POSTGRES_DSN is required")
	}
	table := strings.TrimSpace(os.Getenv("RUNFABRIC_STATE_POSTGRES_TABLE"))
	receipts, err := pgbackend.NewReceiptBackend(dsn, table, root)
	if err != nil {
		return nil, err
	}
	return &statePlugin{
		meta: sdkrouter.PluginMeta{ID: "postgres", Name: "Postgres State Backend", Description: "Postgres receipts with local locks/journals"},
		lock: localbackend.NewLockBackend(root), journal: localbackend.NewJournalBackend(root), receipt: receipts,
	}, nil
}

// NewDynamoDBTransportPlugin creates the external state plugin for the dynamodb backend.
func NewDynamoDBTransportPlugin() (sdkstate.Plugin, error) {
	root := stateRoot()
	region := firstNonEmpty(strings.TrimSpace(os.Getenv("RUNFABRIC_AWS_REGION")), strings.TrimSpace(os.Getenv("AWS_REGION")), "us-east-1")
	table := firstNonEmpty(strings.TrimSpace(os.Getenv("RUNFABRIC_STATE_DYNAMODB_TABLE")), strings.TrimSpace(os.Getenv("RUNFABRIC_STATE_RECEIPT_TABLE")))
	if table == "" {
		return nil, fmt.Errorf("RUNFABRIC_STATE_DYNAMODB_TABLE is required")
	}
	client, err := dynbackend.New(context.Background(), region, table)
	if err != nil {
		return nil, err
	}
	receipts := dynbackend.NewReceiptBackend(client, root)
	return &statePlugin{
		meta: sdkrouter.PluginMeta{ID: "dynamodb", Name: "DynamoDB State Backend", Description: "DynamoDB receipts with local locks/journals"},
		lock: localbackend.NewLockBackend(root), journal: localbackend.NewJournalBackend(root), receipt: receipts,
	}, nil
}

// NewS3TransportPlugin creates the external state plugin for the s3 backend.
func NewS3TransportPlugin() (sdkstate.Plugin, error) {
	root := stateRoot()
	region := firstNonEmpty(strings.TrimSpace(os.Getenv("RUNFABRIC_AWS_REGION")), strings.TrimSpace(os.Getenv("AWS_REGION")), "us-east-1")
	bucket := strings.TrimSpace(os.Getenv("RUNFABRIC_STATE_S3_BUCKET"))
	if bucket == "" {
		return nil, fmt.Errorf("RUNFABRIC_STATE_S3_BUCKET is required")
	}
	prefix := strings.TrimSpace(os.Getenv("RUNFABRIC_STATE_S3_PREFIX"))
	client, err := s3backend.New(context.Background(), region, bucket, prefix)
	if err != nil {
		return nil, err
	}
	receipts := s3backend.NewReceiptBackend(context.Background(), client)
	return &statePlugin{
		meta: sdkrouter.PluginMeta{ID: "s3", Name: "S3 State Backend", Description: "S3 receipts with local locks/journals"},
		lock: localbackend.NewLockBackend(root), journal: localbackend.NewJournalBackend(root), receipt: receipts,
	}, nil
}

func stateRoot() string {
	root := strings.TrimSpace(os.Getenv("RUNFABRIC_STATE_ROOT"))
	if root == "" {
		root = "."
	}
	if abs, err := filepath.Abs(root); err == nil {
		return abs
	}
	return root
}

func toSDKLockRecord(in *extstate.LockRecord) *sdkstate.LockRecord {
	if in == nil {
		return nil
	}
	return &sdkstate.LockRecord{
		Service:         in.Service,
		Stage:           in.Stage,
		Operation:       in.Operation,
		OwnerToken:      in.OwnerToken,
		PID:             in.PID,
		CreatedAt:       in.CreatedAt,
		ExpiresAt:       in.ExpiresAt,
		LastHeartbeatAt: in.LastHeartbeatAt,
	}
}

func toSDKJournalFile(in *extstate.JournalFile) (*sdkstate.JournalFile, error) {
	if in == nil {
		return nil, nil
	}
	return remarshal[*sdkstate.JournalFile](in)
}

func fromSDKJournalFile(in *sdkstate.JournalFile) (*extstate.JournalFile, error) {
	if in == nil {
		return nil, fmt.Errorf("journal is required")
	}
	return remarshal[*extstate.JournalFile](in)
}

func toSDKReceipt(in *extstate.Receipt) (*sdkstate.Receipt, error) {
	if in == nil {
		return nil, nil
	}
	return remarshal[*sdkstate.Receipt](in)
}

func fromSDKReceipt(in *sdkstate.Receipt) (*extstate.Receipt, error) {
	if in == nil {
		return nil, fmt.Errorf("receipt is required")
	}
	return remarshal[*extstate.Receipt](in)
}

func remarshal[T any](v any) (T, error) {
	var out T
	data, err := json.Marshal(v)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, err
	}
	return out, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

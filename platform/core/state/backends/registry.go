package backends

import (
	"context"
	"fmt"

	dynbackend "github.com/runfabric/runfabric/platform/core/state/backends/dynamodb"
	localbackend "github.com/runfabric/runfabric/platform/core/state/backends/local"
	pgbackend "github.com/runfabric/runfabric/platform/core/state/backends/postgres"
	sqlitebackend "github.com/runfabric/runfabric/platform/core/state/backends/sqlite"
)

type Bundle struct {
	Locks    LockBackend
	Journals JournalBackend
	Receipts ReceiptBackend
}

func NewBundle(ctx context.Context, opts Options) (*Bundle, error) {
	switch opts.Kind {
	case "", "local":
		return &Bundle{
			Locks:    localbackend.NewLockBackend(opts.Root),
			Journals: localbackend.NewJournalBackend(opts.Root),
			Receipts: localbackend.NewReceiptBackend(opts.Root),
		}, nil

	case "postgres":
		receipts, err := pgbackend.NewReceiptBackend(opts.PostgresDSN, opts.PostgresTable, opts.Root)
		if err != nil {
			return nil, fmt.Errorf("init postgres receipts: %w", err)
		}
		return &Bundle{
			Locks:    localbackend.NewLockBackend(opts.Root),
			Journals: localbackend.NewJournalBackend(opts.Root),
			Receipts: receipts,
		}, nil

	case "sqlite":
		path := sqlitebackend.ResolvePath(opts.Root, opts.SqlitePath)
		receipts, err := sqlitebackend.NewReceiptBackend(path, opts.Root)
		if err != nil {
			return nil, fmt.Errorf("init sqlite receipts: %w", err)
		}
		return &Bundle{
			Locks:    localbackend.NewLockBackend(opts.Root),
			Journals: localbackend.NewJournalBackend(opts.Root),
			Receipts: receipts,
		}, nil

	case "dynamodb":
		table := opts.ReceiptTable
		if table == "" {
			table = opts.DynamoTableName
		}
		if table == "" {
			return nil, fmt.Errorf("backend.receiptTable or backend.lockTable required for kind dynamodb")
		}
		dynamoClient, err := dynbackend.New(ctx, opts.AWSRegion, table)
		if err != nil {
			return nil, fmt.Errorf("init dynamodb receipts: %w", err)
		}
		receipts := dynbackend.NewReceiptBackend(dynamoClient, opts.Root)
		return &Bundle{
			Locks:    localbackend.NewLockBackend(opts.Root),
			Journals: localbackend.NewJournalBackend(opts.Root),
			Receipts: receipts,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported backend kind: %s", opts.Kind)
	}
}

package backends

import (
	"context"
	"fmt"

	dynbackend "github.com/runfabric/runfabric/internal/backends/dynamodb"
	localbackend "github.com/runfabric/runfabric/internal/backends/local"
	s3backend "github.com/runfabric/runfabric/internal/backends/s3"
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

	case "aws-remote":
		s3Client, err := s3backend.New(ctx, opts.AWSRegion, opts.S3Bucket, opts.S3Prefix)
		if err != nil {
			return nil, fmt.Errorf("init s3 backend: %w", err)
		}

		dynamoClient, err := dynbackend.New(ctx, opts.AWSRegion, opts.DynamoTableName)
		if err != nil {
			return nil, fmt.Errorf("init dynamodb backend: %w", err)
		}

		return &Bundle{
			Locks:    dynbackend.NewLockBackend(dynamoClient),
			Journals: s3backend.NewJournalBackend(ctx, s3Client),
			Receipts: s3backend.NewReceiptBackend(ctx, s3Client),
		}, nil

	default:
		return nil, fmt.Errorf("unsupported backend kind: %s", opts.Kind)
	}
}

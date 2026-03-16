package app

import (
	"context"
	"fmt"

	"github.com/runfabric/runfabric/engine/internal/backends"
)

func BackendMigrate(configPath, stage, target string) (any, error) {
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}

	sourceReceipt, receiptErr := ctx.Backends.Receipts.Load(ctx.Stage)
	sourceJournal, journalErr := ctx.Backends.Journals.Load(ctx.Config.Service, ctx.Stage)

	var s3Bucket, s3Prefix, lockTable string
	if ctx.Config.Backend != nil {
		s3Bucket = ctx.Config.Backend.S3Bucket
		s3Prefix = ctx.Config.Backend.S3Prefix
		lockTable = ctx.Config.Backend.LockTable
	}

	targetBundle, err := backends.NewBundle(context.Background(), backends.Options{
		Kind:            target,
		Root:            ctx.RootDir,
		AWSRegion:       ctx.Config.Provider.Region,
		S3Bucket:        s3Bucket,
		S3Prefix:        s3Prefix,
		DynamoTableName: lockTable,
	})
	if err != nil {
		return nil, err
	}

	migrated := map[string]any{
		"target": target,
	}

	if receiptErr == nil && sourceReceipt != nil {
		if err := targetBundle.Receipts.Save(sourceReceipt); err != nil {
			return nil, fmt.Errorf("migrate receipt: %w", err)
		}
		migrated["receipt"] = "migrated"
	}

	if journalErr == nil && sourceJournal != nil {
		if err := targetBundle.Journals.Save(sourceJournal); err != nil {
			return nil, fmt.Errorf("migrate journal: %w", err)
		}
		migrated["journal"] = "migrated"
	}

	return migrated, nil
}

package backends

import (
	"context"
	"fmt"

	_ "github.com/runfabric/runfabric/platform/extensions/providerpolicy"

	"github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"
)

func init() {
	RegisterBundleFactory("local", newLocalBundle)
	RegisterBundleFactory("postgres", newPostgresBundle)
	RegisterBundleFactory("sqlite", newSQLiteBundle)
	RegisterBundleFactory("dynamodb", newDynamoDBBundle)
	RegisterBundleFactory("s3", newS3Bundle)
}

func stateCatalogOptions(opts Options) catalog.StateBackendOptions {
	return catalog.StateBackendOptions{
		Root:            opts.Root,
		AWSRegion:       opts.AWSRegion,
		S3Bucket:        opts.S3Bucket,
		S3Prefix:        opts.S3Prefix,
		DynamoTableName: opts.DynamoTableName,
		PostgresDSN:     opts.PostgresDSN,
		PostgresTable:   opts.PostgresTable,
		SqlitePath:      opts.SqlitePath,
		ReceiptTable:    opts.ReceiptTable,
	}
}

func buildBundleFromCatalogFactory(kind string, ctx context.Context, opts Options) (*Bundle, error) {
	factory, ok := catalog.StateBackendFactory(kind)
	if !ok {
		return nil, fmt.Errorf("unsupported backend kind: %s", kind)
	}
	components, err := factory(ctx, stateCatalogOptions(opts))
	if err != nil {
		return nil, err
	}
	return &Bundle{Locks: components.Locks, Journals: components.Journals, Receipts: components.Receipts}, nil
}

func newLocalBundle(ctx context.Context, opts Options) (*Bundle, error) {
	return buildBundleFromCatalogFactory("local", ctx, opts)
}

func newPostgresBundle(ctx context.Context, opts Options) (*Bundle, error) {
	return buildBundleFromCatalogFactory("postgres", ctx, opts)
}

func newSQLiteBundle(ctx context.Context, opts Options) (*Bundle, error) {
	return buildBundleFromCatalogFactory("sqlite", ctx, opts)
}

func newDynamoDBBundle(ctx context.Context, opts Options) (*Bundle, error) {
	return buildBundleFromCatalogFactory("dynamodb", ctx, opts)
}

func newS3Bundle(ctx context.Context, opts Options) (*Bundle, error) {
	return buildBundleFromCatalogFactory("s3", ctx, opts)
}

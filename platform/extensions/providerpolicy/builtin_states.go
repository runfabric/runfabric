package providerpolicy

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	dynbackend "github.com/runfabric/runfabric/extensions/states/dynamodb"
	localbackend "github.com/runfabric/runfabric/extensions/states/local"
	pgbackend "github.com/runfabric/runfabric/extensions/states/postgres"
	s3backend "github.com/runfabric/runfabric/extensions/states/s3"
	sqlitebackend "github.com/runfabric/runfabric/extensions/states/sqlite"
	statetypes "github.com/runfabric/runfabric/internal/state/types"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"
)

func init() {
	catalog.RegisterStateBackendFactory("local", localStateComponents)
	catalog.RegisterStateBackendFactory("postgres", postgresStateComponents)
	catalog.RegisterStateBackendFactory("sqlite", sqliteStateComponents)
	catalog.RegisterStateBackendFactory("dynamodb", dynamoDBStateComponents)
	catalog.RegisterStateBackendFactory("s3", s3StateComponents)
}

func localStateComponents(_ context.Context, opts catalog.StateBackendOptions) (*catalog.StateBundleComponents, error) {
	lockBackend := localbackend.NewLockBackend(opts.Root)
	journalBackend := localbackend.NewJournalBackend(opts.Root)
	receiptBackend := localbackend.NewReceiptBackend(opts.Root)
	return &catalog.StateBundleComponents{
		Locks:    &lockBackendAdapter{inner: lockBackend},
		Journals: &journalBackendAdapter{inner: journalBackend},
		Receipts: &receiptBackendAdapter{inner: receiptBackend},
	}, nil
}

func postgresStateComponents(_ context.Context, opts catalog.StateBackendOptions) (*catalog.StateBundleComponents, error) {
	receipts, err := pgbackend.NewReceiptBackend(opts.PostgresDSN, opts.PostgresTable, opts.Root)
	if err != nil {
		return nil, fmt.Errorf("init postgres receipts: %w", err)
	}
	lockBackend := localbackend.NewLockBackend(opts.Root)
	journalBackend := localbackend.NewJournalBackend(opts.Root)
	return &catalog.StateBundleComponents{
		Locks:    &lockBackendAdapter{inner: lockBackend},
		Journals: &journalBackendAdapter{inner: journalBackend},
		Receipts: &receiptBackendAdapter{inner: receipts},
	}, nil
}

func sqliteStateComponents(_ context.Context, opts catalog.StateBackendOptions) (*catalog.StateBundleComponents, error) {
	path := sqlitebackend.ResolvePath(opts.Root, opts.SqlitePath)
	receipts, err := sqlitebackend.NewReceiptBackend(path, opts.Root)
	if err != nil {
		return nil, fmt.Errorf("init sqlite receipts: %w", err)
	}
	lockBackend := localbackend.NewLockBackend(opts.Root)
	journalBackend := localbackend.NewJournalBackend(opts.Root)
	return &catalog.StateBundleComponents{
		Locks:    &lockBackendAdapter{inner: lockBackend},
		Journals: &journalBackendAdapter{inner: journalBackend},
		Receipts: &receiptBackendAdapter{inner: receipts},
	}, nil
}

func dynamoDBStateComponents(ctx context.Context, opts catalog.StateBackendOptions) (*catalog.StateBundleComponents, error) {
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
	lockBackend := localbackend.NewLockBackend(opts.Root)
	journalBackend := localbackend.NewJournalBackend(opts.Root)
	return &catalog.StateBundleComponents{
		Locks:    &lockBackendAdapter{inner: lockBackend},
		Journals: &journalBackendAdapter{inner: journalBackend},
		Receipts: &receiptBackendAdapter{inner: receipts},
	}, nil
}

func s3StateComponents(ctx context.Context, opts catalog.StateBackendOptions) (*catalog.StateBundleComponents, error) {
	if opts.S3Bucket == "" {
		return nil, fmt.Errorf("backend.s3Bucket required for kind s3")
	}
	client, err := s3backend.New(ctx, opts.AWSRegion, opts.S3Bucket, opts.S3Prefix)
	if err != nil {
		return nil, fmt.Errorf("init s3 receipts: %w", err)
	}
	lockBackend := localbackend.NewLockBackend(opts.Root)
	journalBackend := localbackend.NewJournalBackend(opts.Root)
	receiptBackend := s3backend.NewReceiptBackend(ctx, client)
	return &catalog.StateBundleComponents{
		Locks:    &lockBackendAdapter{inner: lockBackend},
		Journals: &journalBackendAdapter{inner: journalBackend},
		Receipts: &receiptBackendAdapter{inner: receiptBackend},
	}, nil
}

type lockBackendAdapter struct {
	inner any
}

func (a *lockBackendAdapter) Acquire(service, stage, operation string, staleAfter time.Duration) (*statetypes.Handle, error) {
	raw, err := callMethodResult(a.inner, "Acquire", service, stage, operation, staleAfter)
	if err != nil {
		return nil, err
	}
	record := statetypes.LockRecord{Service: service, Stage: stage, Operation: operation}
	if err := decodeStatePayload(raw, &record); err != nil {
		return nil, fmt.Errorf("decode lock acquire result: %w", err)
	}
	if strings.TrimSpace(record.OwnerToken) == "" {
		record.OwnerToken = "state-lock"
	}
	h := statetypes.NewHandle(service, stage, record.OwnerToken, a, nil, nil)
	h.Held = true
	return h, nil
}

func (a *lockBackendAdapter) Read(service, stage string) (*statetypes.LockRecord, error) {
	raw, err := callMethodResult(a.inner, "Read", service, stage)
	if err != nil {
		return nil, err
	}
	record := &statetypes.LockRecord{}
	if err := decodeStatePayload(raw, record); err != nil {
		return nil, fmt.Errorf("decode lock read result: %w", err)
	}
	return record, nil
}

func (a *lockBackendAdapter) Release(service, stage string) error {
	_, err := callMethodResult(a.inner, "Release", service, stage)
	return err
}

func (a *lockBackendAdapter) Kind() string {
	raw, err := callMethodResult(a.inner, "Kind")
	if err != nil {
		return ""
	}
	if v, ok := raw.(string); ok {
		return v
	}
	return ""
}

type journalBackendAdapter struct {
	inner any
}

func (a *journalBackendAdapter) Load(service, stage string) (*statetypes.JournalFile, error) {
	raw, err := callMethodResult(a.inner, "Load", service, stage)
	if err != nil {
		return nil, err
	}
	j := &statetypes.JournalFile{}
	if err := decodeStatePayload(raw, j); err != nil {
		return nil, fmt.Errorf("decode journal load result: %w", err)
	}
	return j, nil
}

func (a *journalBackendAdapter) Save(j *statetypes.JournalFile) error {
	if j == nil {
		return fmt.Errorf("journal is required")
	}
	_, err := callMethodResult(a.inner, "Save", j)
	return err
}

func (a *journalBackendAdapter) Delete(service, stage string) error {
	_, err := callMethodResult(a.inner, "Delete", service, stage)
	return err
}

func (a *journalBackendAdapter) Kind() string {
	raw, err := callMethodResult(a.inner, "Kind")
	if err != nil {
		return ""
	}
	if v, ok := raw.(string); ok {
		return v
	}
	return ""
}

type receiptBackendAdapter struct {
	inner any
}

func (a *receiptBackendAdapter) Load(stage string) (*statetypes.Receipt, error) {
	raw, err := callMethodResult(a.inner, "Load", stage)
	if err != nil {
		return nil, err
	}
	receipt := &statetypes.Receipt{}
	if err := decodeStatePayload(raw, receipt); err != nil {
		return nil, fmt.Errorf("decode receipt load result: %w", err)
	}
	return receipt, nil
}

func (a *receiptBackendAdapter) Save(receipt *statetypes.Receipt) error {
	if receipt == nil {
		return fmt.Errorf("receipt is required")
	}
	_, err := callMethodResult(a.inner, "Save", receipt)
	return err
}

func (a *receiptBackendAdapter) Delete(stage string) error {
	_, err := callMethodResult(a.inner, "Delete", stage)
	return err
}

func (a *receiptBackendAdapter) ListReleases() ([]statetypes.ReleaseEntry, error) {
	raw, err := callMethodResult(a.inner, "ListReleases")
	if err != nil {
		return nil, err
	}
	entries := []statetypes.ReleaseEntry{}
	if err := decodeStatePayload(raw, &entries); err != nil {
		return nil, fmt.Errorf("decode release list result: %w", err)
	}
	return entries, nil
}

func (a *receiptBackendAdapter) Kind() string {
	raw, err := callMethodResult(a.inner, "Kind")
	if err != nil {
		return ""
	}
	if v, ok := raw.(string); ok {
		return v
	}
	return ""
}

func callMethodResult(target any, methodName string, args ...any) (any, error) {
	v := reflect.ValueOf(target)
	m := v.MethodByName(methodName)
	if !m.IsValid() {
		return nil, fmt.Errorf("method %s not found", methodName)
	}
	mt := m.Type()
	if mt.NumIn() != len(args) {
		return nil, fmt.Errorf("method %s expects %d args, got %d", methodName, mt.NumIn(), len(args))
	}
	in := make([]reflect.Value, mt.NumIn())
	for i := 0; i < mt.NumIn(); i++ {
		arg, err := convertArg(args[i], mt.In(i))
		if err != nil {
			return nil, err
		}
		in[i] = arg
	}
	out := m.Call(in)
	if len(out) == 0 {
		return nil, nil
	}
	if len(out) == 1 {
		if err, ok := out[0].Interface().(error); ok {
			return nil, err
		}
		return out[0].Interface(), nil
	}
	if len(out) == 2 {
		if !out[1].IsNil() {
			if err, ok := out[1].Interface().(error); ok {
				return nil, err
			}
			return nil, fmt.Errorf("method %s returned non-error second value", methodName)
		}
		return out[0].Interface(), nil
	}
	return nil, fmt.Errorf("method %s returned unsupported arity %d", methodName, len(out))
}

func remarshal(src any, dst any) error {
	blob, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(blob, dst)
}

func decodeStatePayload(raw any, out any) error {
	return remarshal(raw, out)
}

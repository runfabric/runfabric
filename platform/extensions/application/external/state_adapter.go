package external

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	statetypes "github.com/runfabric/runfabric/internal/state/types"
	providers "github.com/runfabric/runfabric/platform/core/contracts/provider"
	"github.com/runfabric/runfabric/platform/state/backends"
)

// NewExternalStateBundleFactory creates a state backend bundle factory backed by
// an external plugin executable.
func NewExternalStateBundleFactory(pluginID, backendKind, executable string) backends.BundleFactory {
	adapter := newExternalStateBundleAdapter(pluginID, backendKind, executable)
	return func(_ context.Context, _ backends.Options) (*backends.Bundle, error) {
		return &backends.Bundle{
			Locks:    adapter.lock,
			Journals: adapter.journal,
			Receipts: adapter.receipt,
		}, nil
	}
}

type externalStateBundleAdapter struct {
	lock    *externalStateLockBackend
	journal *externalStateJournalBackend
	receipt *externalStateReceiptBackend
}

type externalStateClient struct {
	id   string
	kind string
	raw  *ExternalProviderAdapter
}

func newExternalStateBundleAdapter(pluginID, backendKind, executable string) *externalStateBundleAdapter {
	id := strings.TrimSpace(pluginID)
	kind := strings.ToLower(strings.TrimSpace(backendKind))
	meta := providers.ProviderMeta{Name: id}
	client := &externalStateClient{
		id:   id,
		kind: kind,
		raw:  NewExternalProviderAdapter(id, executable, meta),
	}
	return &externalStateBundleAdapter{
		lock:    &externalStateLockBackend{client: client},
		journal: &externalStateJournalBackend{client: client},
		receipt: &externalStateReceiptBackend{client: client},
	}
}

func (c *externalStateClient) callAny(methods []string, params any, out any) error {
	var lastErr error
	for _, method := range methods {
		if err := c.raw.call(method, params, out); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("state plugin %q call failed", c.id)
}

type externalStateLockBackend struct {
	client *externalStateClient
}

func (b *externalStateLockBackend) Acquire(service, stage, operation string, staleAfter time.Duration) (*statetypes.Handle, error) {
	params := map[string]any{
		"service":          service,
		"stage":            stage,
		"operation":        operation,
		"staleAfterMillis": staleAfter.Milliseconds(),
	}
	var raw any
	if err := b.client.callAny([]string{"LockAcquire", "AcquireLock", "Acquire"}, params, &raw); err != nil {
		return nil, err
	}

	record := statetypes.LockRecord{Service: service, Stage: stage, Operation: operation}
	if err := decodeStatePayload(raw, &record); err != nil {
		return nil, fmt.Errorf("decode lock acquire result: %w", err)
	}
	if strings.TrimSpace(record.OwnerToken) == "" {
		record.OwnerToken = "external-state"
	}
	handle := statetypes.NewHandle(service, stage, record.OwnerToken, b, nil, nil)
	handle.Held = true
	return handle, nil
}

func (b *externalStateLockBackend) Read(service, stage string) (*statetypes.LockRecord, error) {
	params := map[string]any{"service": service, "stage": stage}
	var raw any
	if err := b.client.callAny([]string{"LockRead", "ReadLock", "Read"}, params, &raw); err != nil {
		return nil, err
	}
	record := &statetypes.LockRecord{Service: service, Stage: stage}
	if err := decodeStatePayload(raw, record); err != nil {
		return nil, fmt.Errorf("decode lock read result: %w", err)
	}
	return record, nil
}

func (b *externalStateLockBackend) Release(service, stage string) error {
	params := map[string]any{"service": service, "stage": stage}
	var raw any
	if err := b.client.callAny([]string{"LockRelease", "ReleaseLock", "Release"}, params, &raw); err != nil {
		return err
	}
	return nil
}

func (b *externalStateLockBackend) Kind() string {
	return b.client.kind
}

type externalStateJournalBackend struct {
	client *externalStateClient
}

func (b *externalStateJournalBackend) Load(service, stage string) (*statetypes.JournalFile, error) {
	params := map[string]any{"service": service, "stage": stage}
	var raw any
	if err := b.client.callAny([]string{"JournalLoad", "LoadJournal"}, params, &raw); err != nil {
		return nil, err
	}
	journal := &statetypes.JournalFile{Service: service, Stage: stage}
	if err := decodeStatePayload(raw, journal); err != nil {
		return nil, fmt.Errorf("decode journal load result: %w", err)
	}
	return journal, nil
}

func (b *externalStateJournalBackend) Save(j *statetypes.JournalFile) error {
	if j == nil {
		return fmt.Errorf("journal is required")
	}
	var raw any
	if err := b.client.callAny([]string{"JournalSave", "SaveJournal"}, map[string]any{"journal": j}, &raw); err != nil {
		return err
	}
	return nil
}

func (b *externalStateJournalBackend) Delete(service, stage string) error {
	params := map[string]any{"service": service, "stage": stage}
	var raw any
	if err := b.client.callAny([]string{"JournalDelete", "DeleteJournal"}, params, &raw); err != nil {
		return err
	}
	return nil
}

func (b *externalStateJournalBackend) Kind() string {
	return b.client.kind
}

type externalStateReceiptBackend struct {
	client *externalStateClient
}

func (b *externalStateReceiptBackend) Load(stage string) (*statetypes.Receipt, error) {
	var raw any
	if err := b.client.callAny([]string{"ReceiptLoad", "LoadReceipt"}, map[string]any{"stage": stage}, &raw); err != nil {
		return nil, err
	}
	receipt := &statetypes.Receipt{Stage: stage}
	if err := decodeStatePayload(raw, receipt); err != nil {
		return nil, fmt.Errorf("decode receipt load result: %w", err)
	}
	return receipt, nil
}

func (b *externalStateReceiptBackend) Save(receipt *statetypes.Receipt) error {
	if receipt == nil {
		return fmt.Errorf("receipt is required")
	}
	var raw any
	if err := b.client.callAny([]string{"ReceiptSave", "SaveReceipt"}, map[string]any{"receipt": receipt}, &raw); err != nil {
		return err
	}
	return nil
}

func (b *externalStateReceiptBackend) Delete(stage string) error {
	var raw any
	if err := b.client.callAny([]string{"ReceiptDelete", "DeleteReceipt"}, map[string]any{"stage": stage}, &raw); err != nil {
		return err
	}
	return nil
}

func (b *externalStateReceiptBackend) ListReleases() ([]statetypes.ReleaseEntry, error) {
	var raw any
	if err := b.client.callAny([]string{"ReceiptListReleases", "ListReleases"}, map[string]any{}, &raw); err != nil {
		return nil, err
	}
	entries := []statetypes.ReleaseEntry{}
	if err := decodeStatePayload(raw, &entries); err == nil {
		return entries, nil
	}
	if payloadMap, ok := raw.(map[string]any); ok {
		if nested, ok := payloadMap["releases"]; ok {
			if err := decodeStatePayload(nested, &entries); err == nil {
				return entries, nil
			}
		}
	}
	return nil, fmt.Errorf("decode receipt list result: unsupported payload")
}

func (b *externalStateReceiptBackend) Kind() string {
	return b.client.kind
}

func decodeStatePayload(raw any, out any) error {
	blob, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(blob, out)
}

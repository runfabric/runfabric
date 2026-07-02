package runstore

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	dynamodbv2 "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	core "github.com/runfabric/runfabric/platform/core/state/core"
)

// fakeDynamo is an in-memory dynamoAPI that evaluates exactly the condition
// expressions this backend issues (CAS on version, absent-or-expired lease,
// owner match). It is not a general DynamoDB emulator.
type fakeDynamo struct {
	mu    sync.Mutex
	items map[string]map[string]ddbtypes.AttributeValue // "table|pk|sk" -> item
}

func newFakeDynamo() *fakeDynamo {
	return &fakeDynamo{items: map[string]map[string]ddbtypes.AttributeValue{}}
}

func itemKey(table string, key map[string]ddbtypes.AttributeValue) string {
	pk := key[attrPK].(*ddbtypes.AttributeValueMemberS).Value
	sk := key[attrSK].(*ddbtypes.AttributeValueMemberS).Value
	return table + "|" + pk + "|" + sk
}

func numAttr(item map[string]ddbtypes.AttributeValue, name string) int64 {
	if v, ok := item[name].(*ddbtypes.AttributeValueMemberN); ok {
		n, _ := strconv.ParseInt(v.Value, 10, 64)
		return n
	}
	return 0
}

func conditionFailed() error {
	msg := "conditional check failed"
	return &ddbtypes.ConditionalCheckFailedException{Message: &msg}
}

// checkCondition evaluates the three expressions the backend uses against the
// currently stored item (nil when absent).
func checkCondition(cond string, stored, values map[string]ddbtypes.AttributeValue) error {
	switch cond {
	case "attribute_not_exists(" + attrVersion + ") OR " + attrVersion + " = :expected":
		if stored == nil || stored[attrVersion] == nil {
			return nil
		}
		expected := values[":expected"].(*ddbtypes.AttributeValueMemberS).Value
		if stringAttr(stored, attrVersion) != expected {
			return conditionFailed()
		}
	case "attribute_not_exists(" + attrPK + ") OR " + attrExpiresAt + " < :now":
		if stored == nil {
			return nil
		}
		now, _ := strconv.ParseInt(values[":now"].(*ddbtypes.AttributeValueMemberN).Value, 10, 64)
		if numAttr(stored, attrExpiresAt) >= now {
			return conditionFailed()
		}
	case "#o = :owner":
		owner := values[":owner"].(*ddbtypes.AttributeValueMemberS).Value
		if stored == nil || stringAttr(stored, attrOwner) != owner {
			return conditionFailed()
		}
	default:
		return errors.New("fakeDynamo: unknown condition expression: " + cond)
	}
	return nil
}

func (f *fakeDynamo) GetItem(_ context.Context, in *dynamodbv2.GetItemInput, _ ...func(*dynamodbv2.Options)) (*dynamodbv2.GetItemOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	item := f.items[itemKey(*in.TableName, in.Key)]
	return &dynamodbv2.GetItemOutput{Item: item}, nil
}

func (f *fakeDynamo) PutItem(_ context.Context, in *dynamodbv2.PutItemInput, _ ...func(*dynamodbv2.Options)) (*dynamodbv2.PutItemOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := itemKey(*in.TableName, in.Item)
	if in.ConditionExpression != nil {
		if err := checkCondition(*in.ConditionExpression, f.items[key], in.ExpressionAttributeValues); err != nil {
			return nil, err
		}
	}
	f.items[key] = in.Item
	return &dynamodbv2.PutItemOutput{}, nil
}

func (f *fakeDynamo) Query(_ context.Context, in *dynamodbv2.QueryInput, _ ...func(*dynamodbv2.Options)) (*dynamodbv2.QueryOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	pk := in.ExpressionAttributeValues[":pk"].(*ddbtypes.AttributeValueMemberS).Value
	var out []map[string]ddbtypes.AttributeValue
	for _, item := range f.items {
		if stringAttr(item, attrPK) == pk {
			out = append(out, item)
		}
	}
	return &dynamodbv2.QueryOutput{Items: out}, nil
}

func (f *fakeDynamo) UpdateItem(_ context.Context, in *dynamodbv2.UpdateItemInput, _ ...func(*dynamodbv2.Options)) (*dynamodbv2.UpdateItemOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := itemKey(*in.TableName, in.Key)
	stored := f.items[key]
	if in.ConditionExpression != nil {
		if err := checkCondition(*in.ConditionExpression, stored, in.ExpressionAttributeValues); err != nil {
			return nil, err
		}
	}
	// Only the heartbeat's "SET expiresAt = :exp" is supported.
	stored[attrExpiresAt] = in.ExpressionAttributeValues[":exp"]
	return &dynamodbv2.UpdateItemOutput{}, nil
}

func (f *fakeDynamo) DeleteItem(_ context.Context, in *dynamodbv2.DeleteItemInput, _ ...func(*dynamodbv2.Options)) (*dynamodbv2.DeleteItemOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := itemKey(*in.TableName, in.Key)
	if in.ConditionExpression != nil {
		if err := checkCondition(*in.ConditionExpression, f.items[key], in.ExpressionAttributeValues); err != nil {
			return nil, err
		}
	}
	delete(f.items, key)
	return &dynamodbv2.DeleteItemOutput{}, nil
}

// fakeClock is a manually advanced clock for lease-expiry tests.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func newTestDynamoStore() (*dynamoDBRunStore, *fakeClock) {
	clock := &fakeClock{t: time.Now()}
	return &dynamoDBRunStore{
		client:    newFakeDynamo(),
		table:     "runs",
		lockTable: "runs",
		now:       clock.Now,
		lockPoll:  5 * time.Millisecond,
	}, clock
}

func TestDynamoSaveLoadRoundTrip(t *testing.T) {
	s, _ := newTestDynamoStore()
	ctx := context.Background()

	if _, _, err := s.Load(ctx, "dev", "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load(missing) = %v, want ErrNotFound", err)
	}

	v1, err := s.Save(ctx, newRun("r1"), "")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if v1 == "" {
		t.Fatal("Save returned empty version")
	}

	got, v2, err := s.Load(ctx, "dev", "r1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.RunID != "r1" || got.Service != "svc" {
		t.Fatalf("round-trip run = %+v", got)
	}
	if v2 != v1 {
		t.Fatalf("Load version %s != Save version %s", v2, v1)
	}
}

func TestDynamoCASConflict(t *testing.T) {
	s, _ := newTestDynamoStore()
	ctx := context.Background()

	v1, err := s.Save(ctx, newRun("r1"), "")
	if err != nil {
		t.Fatalf("initial Save: %v", err)
	}

	runA, _, _ := s.Load(ctx, "dev", "r1")
	runB, _, _ := s.Load(ctx, "dev", "r1")

	runA.Status = core.RunStatusOK
	v2, err := s.Save(ctx, runA, v1)
	if err != nil {
		t.Fatalf("writer A Save: %v", err)
	}
	if v2 == v1 {
		t.Fatal("expected a new version after write")
	}

	runB.Status = core.RunStatusFailed
	if _, err := s.Save(ctx, runB, v1); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("writer B Save = %v, want ErrVersionConflict", err)
	}

	runB, cur, _ := s.Load(ctx, "dev", "r1")
	runB.Status = core.RunStatusFailed
	if _, err := s.Save(ctx, runB, cur); err != nil {
		t.Fatalf("writer B retry: %v", err)
	}
}

func TestDynamoListNewestFirst(t *testing.T) {
	s, _ := newTestDynamoStore()
	ctx := context.Background()

	base := time.Now().UTC()
	for i, id := range []string{"old", "mid", "new"} {
		r := newRun(id)
		r.StartedAt = base.Add(time.Duration(i) * time.Minute).Format(time.RFC3339)
		if _, err := s.Save(ctx, r, ""); err != nil {
			t.Fatalf("Save(%s): %v", id, err)
		}
	}

	runs, err := s.List(ctx, "dev", 2)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(runs) != 2 || runs[0].RunID != "new" || runs[1].RunID != "mid" {
		ids := make([]string, len(runs))
		for i, r := range runs {
			ids[i] = r.RunID
		}
		t.Fatalf("List = %v, want [new mid]", ids)
	}
}

func TestDynamoLockContentionAndRelease(t *testing.T) {
	s, _ := newTestDynamoStore()
	ctx := context.Background()

	release, err := s.Lock(ctx, "dev", "r1", time.Minute)
	if err != nil {
		t.Fatalf("first Lock: %v", err)
	}

	// A second contender times out while the lease is held.
	shortCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if _, err := s.Lock(shortCtx, "dev", "r1", time.Minute); !errors.Is(err, ErrLockHeld) {
		t.Fatalf("contended Lock = %v, want ErrLockHeld", err)
	}

	if err := release(); err != nil {
		t.Fatalf("release: %v", err)
	}
	if err := release(); err != nil {
		t.Fatalf("second release should be a no-op, got %v", err)
	}

	// After release the lock is immediately acquirable.
	release2, err := s.Lock(ctx, "dev", "r1", time.Minute)
	if err != nil {
		t.Fatalf("re-Lock after release: %v", err)
	}
	if err := release2(); err != nil {
		t.Fatalf("release2: %v", err)
	}
}

func TestDynamoLockExpiryTakeover(t *testing.T) {
	s, clock := newTestDynamoStore()
	ctx := context.Background()

	// Holder A acquires and "crashes" (never releases; heartbeat period for a
	// 1s ttl is clamped to 1s, so it does not renew before the takeover below).
	if _, err := s.Lock(ctx, "dev", "r1", time.Second); err != nil {
		t.Fatalf("holder A Lock: %v", err)
	}

	// Before expiry, B cannot take it.
	shortCtx, cancel := context.WithTimeout(ctx, 30*time.Millisecond)
	defer cancel()
	if _, err := s.Lock(shortCtx, "dev", "r1", time.Second); !errors.Is(err, ErrLockHeld) {
		t.Fatalf("pre-expiry Lock = %v, want ErrLockHeld", err)
	}

	// After the lease expires, B takes over without a release.
	clock.Advance(2 * time.Second)
	release, err := s.Lock(ctx, "dev", "r1", time.Second)
	if err != nil {
		t.Fatalf("post-expiry Lock: %v", err)
	}
	if err := release(); err != nil {
		t.Fatalf("release: %v", err)
	}
}

func TestDynamoReleaseAfterStealIsNoop(t *testing.T) {
	s, clock := newTestDynamoStore()
	ctx := context.Background()

	releaseA, err := s.Lock(ctx, "dev", "r1", time.Second)
	if err != nil {
		t.Fatalf("holder A Lock: %v", err)
	}

	clock.Advance(2 * time.Second)
	releaseB, err := s.Lock(ctx, "dev", "r1", time.Minute)
	if err != nil {
		t.Fatalf("holder B takeover: %v", err)
	}

	// A's stale release must not clobber B's lease.
	if err := releaseA(); err != nil {
		t.Fatalf("stale release: %v", err)
	}
	shortCtx, cancel := context.WithTimeout(ctx, 30*time.Millisecond)
	defer cancel()
	if _, err := s.Lock(shortCtx, "dev", "r1", time.Minute); !errors.Is(err, ErrLockHeld) {
		t.Fatalf("Lock after stale release = %v, want ErrLockHeld (B still holds)", err)
	}
	if err := releaseB(); err != nil {
		t.Fatalf("releaseB: %v", err)
	}
}

package runstore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	dynamodbv2 "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/runfabric/runfabric/internal/lease"
	core "github.com/runfabric/runfabric/platform/core/state/core"
)

// DynamoDB multi-instance backend.
//
// Table schema (create before use):
//
//	runs table:  partition key "pk" (S), sort key "sk" (S)
//	lock table:  partition key "pk" (S), sort key "sk" (S) — defaults to the
//	             runs table; override with ?lockTable=<name>
//
// Item layout:
//
//	run item:  pk = "run#<stage>",  sk = "run#<runID>",  version, data (run JSON), startedAt
//	lock item: pk = "lock#<stage>", sk = "lock#<runID>", owner, expiresAt (unix ms)
//
// URI: dynamodb://<table>/runs?region=us-east-1&lockTable=my-locks&endpoint=http://localhost:8000
// (endpoint is for DynamoDB Local / testing; region falls back to the SDK
// default chain, e.g. AWS_REGION.)
//
// Save is a compare-and-swap: a conditional PutItem whose
// ConditionalCheckFailedException maps to ErrVersionConflict. Lock is a
// conditional-write lease with a heartbeat goroutine that extends expiresAt
// while the holder is alive, so a crashed holder's lease expires after ttl.

func init() {
	Register("dynamodb", newDynamoDBRunStore)
}

// dynamoAPI is the subset of the DynamoDB client the backend uses. It exists so
// tests can substitute an in-memory fake for the real client.
type dynamoAPI interface {
	GetItem(ctx context.Context, in *dynamodbv2.GetItemInput, opts ...func(*dynamodbv2.Options)) (*dynamodbv2.GetItemOutput, error)
	PutItem(ctx context.Context, in *dynamodbv2.PutItemInput, opts ...func(*dynamodbv2.Options)) (*dynamodbv2.PutItemOutput, error)
	Query(ctx context.Context, in *dynamodbv2.QueryInput, opts ...func(*dynamodbv2.Options)) (*dynamodbv2.QueryOutput, error)
	UpdateItem(ctx context.Context, in *dynamodbv2.UpdateItemInput, opts ...func(*dynamodbv2.Options)) (*dynamodbv2.UpdateItemOutput, error)
	DeleteItem(ctx context.Context, in *dynamodbv2.DeleteItemInput, opts ...func(*dynamodbv2.Options)) (*dynamodbv2.DeleteItemOutput, error)
}

type dynamoDBRunStore struct {
	client    dynamoAPI
	table     string
	lockTable string

	// now and lockPoll are swappable for tests.
	now      func() time.Time
	lockPoll time.Duration
}

var (
	_ RunStore  = (*dynamoDBRunStore)(nil)
	_ RunLocker = (*dynamoDBRunStore)(nil)
)

const (
	defaultLockTTL  = 30 * time.Second
	defaultLockPoll = 500 * time.Millisecond
	listPageCap     = 1000 // safety bound on items scanned per List
	attrPK          = "pk"
	attrSK          = "sk"
	attrVersion     = "version"
	attrData        = "data"
	attrStartedAt   = "startedAt"
	attrOwner       = "owner"
	attrExpiresAt   = "expiresAt"
)

func newDynamoDBRunStore(cfg Config) (RunStore, error) {
	u, err := url.Parse(cfg.URI)
	if err != nil {
		return nil, fmt.Errorf("runstore: parse dynamodb uri %q: %w", cfg.URI, err)
	}
	table := u.Host
	if table == "" {
		return nil, fmt.Errorf("runstore: dynamodb uri %q is missing a table name (dynamodb://<table>/...)", cfg.URI)
	}

	var loadOpts []func(*awsconfig.LoadOptions) error
	if region := cfg.Params.Get("region"); region != "" {
		loadOpts = append(loadOpts, awsconfig.WithRegion(region))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("runstore: load aws config: %w", err)
	}

	var clientOpts []func(*dynamodbv2.Options)
	if endpoint := cfg.Params.Get("endpoint"); endpoint != "" {
		clientOpts = append(clientOpts, func(o *dynamodbv2.Options) {
			o.BaseEndpoint = &endpoint
		})
	}

	s := &dynamoDBRunStore{
		client:    dynamodbv2.NewFromConfig(awsCfg, clientOpts...),
		table:     table,
		lockTable: table,
		now:       time.Now,
		lockPoll:  defaultLockPoll,
	}
	if lt := cfg.Params.Get("lockTable"); lt != "" {
		s.lockTable = lt
	}
	return s, nil
}

func (d *dynamoDBRunStore) Kind() string { return "dynamodb" }

func runKey(stage, runID string) map[string]ddbtypes.AttributeValue {
	return map[string]ddbtypes.AttributeValue{
		attrPK: &ddbtypes.AttributeValueMemberS{Value: "run#" + stage},
		attrSK: &ddbtypes.AttributeValueMemberS{Value: "run#" + runID},
	}
}

func lockKey(stage, runID string) map[string]ddbtypes.AttributeValue {
	return map[string]ddbtypes.AttributeValue{
		attrPK: &ddbtypes.AttributeValueMemberS{Value: "lock#" + stage},
		attrSK: &ddbtypes.AttributeValueMemberS{Value: "lock#" + runID},
	}
}

// newToken returns a fresh random version/owner token.
func newToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("runstore: generate token: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

func isConditionalCheckFailed(err error) bool {
	var ccf *ddbtypes.ConditionalCheckFailedException
	return errors.As(err, &ccf)
}

func stringAttr(item map[string]ddbtypes.AttributeValue, name string) string {
	if v, ok := item[name].(*ddbtypes.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}

// Load reads a run with a strongly consistent read and returns its stored
// version token.
func (d *dynamoDBRunStore) Load(ctx context.Context, stage, runID string) (*core.WorkflowRun, Version, error) {
	if stage == "" || runID == "" {
		return nil, "", fmt.Errorf("runstore: stage and runID are required")
	}
	consistent := true
	out, err := d.client.GetItem(ctx, &dynamodbv2.GetItemInput{
		TableName:      &d.table,
		Key:            runKey(stage, runID),
		ConsistentRead: &consistent,
	})
	if err != nil {
		return nil, "", fmt.Errorf("runstore: dynamodb get: %w", err)
	}
	if len(out.Item) == 0 {
		return nil, "", ErrNotFound
	}
	var run core.WorkflowRun
	if err := json.Unmarshal([]byte(stringAttr(out.Item, attrData)), &run); err != nil {
		return nil, "", fmt.Errorf("runstore: unmarshal run: %w", err)
	}
	return &run, Version(stringAttr(out.Item, attrVersion)), nil
}

// Save writes the run with a fresh version token. When expected != "", the
// PutItem is conditioned on the stored version still matching (create-if-absent
// is allowed, since runs are never deleted mid-flight); a conditional-check
// failure maps to ErrVersionConflict.
func (d *dynamoDBRunStore) Save(ctx context.Context, run *core.WorkflowRun, expected Version) (Version, error) {
	if run == nil {
		return "", fmt.Errorf("runstore: nil run")
	}
	if run.RunID == "" || run.Stage == "" {
		return "", fmt.Errorf("runstore: run stage and runID are required")
	}

	run.UpdatedAt = d.now().UTC().Format(time.RFC3339)
	data, err := json.Marshal(run)
	if err != nil {
		return "", fmt.Errorf("runstore: marshal run: %w", err)
	}
	next, err := newToken()
	if err != nil {
		return "", err
	}

	item := runKey(run.Stage, run.RunID)
	item[attrVersion] = &ddbtypes.AttributeValueMemberS{Value: next}
	item[attrData] = &ddbtypes.AttributeValueMemberS{Value: string(data)}
	item[attrStartedAt] = &ddbtypes.AttributeValueMemberS{Value: run.StartedAt}

	in := &dynamodbv2.PutItemInput{TableName: &d.table, Item: item}
	if expected != "" {
		cond := "attribute_not_exists(" + attrVersion + ") OR " + attrVersion + " = :expected"
		in.ConditionExpression = &cond
		in.ExpressionAttributeValues = map[string]ddbtypes.AttributeValue{
			":expected": &ddbtypes.AttributeValueMemberS{Value: string(expected)},
		}
	}
	if _, err := d.client.PutItem(ctx, in); err != nil {
		if isConditionalCheckFailed(err) {
			return "", fmt.Errorf("%w: expected %s", ErrVersionConflict, short(expected))
		}
		return "", fmt.Errorf("runstore: dynamodb put: %w", err)
	}
	return Version(next), nil
}

// List queries the stage partition and returns up to limit runs, newest-first
// by StartedAt. Ordering is best-effort (sorted client-side after the query).
func (d *dynamoDBRunStore) List(ctx context.Context, stage string, limit int) ([]*core.WorkflowRun, error) {
	if limit <= 0 {
		limit = 20
	}
	keyExpr := attrPK + " = :pk"
	values := map[string]ddbtypes.AttributeValue{
		":pk": &ddbtypes.AttributeValueMemberS{Value: "run#" + stage},
	}

	type item struct {
		run *core.WorkflowRun
		t   time.Time
	}
	var items []item
	var startKey map[string]ddbtypes.AttributeValue
	for {
		out, err := d.client.Query(ctx, &dynamodbv2.QueryInput{
			TableName:                 &d.table,
			KeyConditionExpression:    &keyExpr,
			ExpressionAttributeValues: values,
			ExclusiveStartKey:         startKey,
		})
		if err != nil {
			return nil, fmt.Errorf("runstore: dynamodb query: %w", err)
		}
		for _, it := range out.Items {
			var r core.WorkflowRun
			if err := json.Unmarshal([]byte(stringAttr(it, attrData)), &r); err != nil {
				continue
			}
			t, _ := time.Parse(time.RFC3339, r.StartedAt)
			items = append(items, item{run: &r, t: t})
		}
		startKey = out.LastEvaluatedKey
		if len(startKey) == 0 || len(items) >= listPageCap {
			break
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].t.After(items[j].t) })
	if len(items) > limit {
		items = items[:limit]
	}
	out := make([]*core.WorkflowRun, 0, len(items))
	for _, it := range items {
		out = append(out, it.run)
	}
	return out, nil
}

// Lock acquires a distributed lease on the run. It blocks (polling, subject to
// ctx) until the lock item can be written with the condition "absent or
// expired". A heartbeat goroutine extends expiresAt while the holder is alive;
// if the holder crashes, the lease expires after ttl and another instance can
// take it. The returned release stops the heartbeat and deletes the lock item
// (conditioned on ownership, so a stolen lease is not clobbered).
func (d *dynamoDBRunStore) Lock(ctx context.Context, stage, runID string, ttl time.Duration) (func() error, error) {
	if stage == "" || runID == "" {
		return nil, fmt.Errorf("runstore: stage and runID are required")
	}
	if ttl <= 0 {
		ttl = defaultLockTTL
	}
	owner, err := newToken()
	if err != nil {
		return nil, err
	}

	for {
		acquired, err := d.tryAcquire(ctx, stage, runID, owner, ttl)
		if err != nil {
			return nil, err
		}
		if acquired {
			break
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("%w: %s/%s (%v)", ErrLockHeld, stage, runID, ctx.Err())
		case <-time.After(d.lockPoll):
		}
	}

	// The lease outlives the acquiring ctx on purpose: it is held until
	// release, not until the caller's deadline.
	managed := lease.Manage(context.Background(), d.renewFunc(stage, runID, owner), ttl, lease.DefaultInterval(ttl), func() error {
		return d.releaseLock(stage, runID, owner)
	})
	return managed.Release, nil
}

// tryAcquire attempts one conditional write of the lock item. It returns false
// (no error) when the lock is currently held by a live owner.
func (d *dynamoDBRunStore) tryAcquire(ctx context.Context, stage, runID, owner string, ttl time.Duration) (bool, error) {
	nowMs := d.now().UnixMilli()
	item := lockKey(stage, runID)
	item[attrOwner] = &ddbtypes.AttributeValueMemberS{Value: owner}
	item[attrExpiresAt] = &ddbtypes.AttributeValueMemberN{Value: strconv.FormatInt(nowMs+ttl.Milliseconds(), 10)}

	cond := "attribute_not_exists(" + attrPK + ") OR " + attrExpiresAt + " < :now"
	_, err := d.client.PutItem(ctx, &dynamodbv2.PutItemInput{
		TableName:           &d.lockTable,
		Item:                item,
		ConditionExpression: &cond,
		ExpressionAttributeValues: map[string]ddbtypes.AttributeValue{
			":now": &ddbtypes.AttributeValueMemberN{Value: strconv.FormatInt(nowMs, 10)},
		},
	})
	if err != nil {
		if isConditionalCheckFailed(err) {
			return false, nil
		}
		return false, fmt.Errorf("runstore: dynamodb lock put: %w", err)
	}
	return true, nil
}

// renewFunc returns the lease renewal callback: it extends expiresAt while we
// still own the lock item. A conditional-check failure means the lease was
// stolen after an expiry — terminal, so the heartbeat stops; any other error
// is transient and worth retrying on the next tick.
func (d *dynamoDBRunStore) renewFunc(stage, runID, owner string) lease.RenewFunc {
	return func(ttl time.Duration) error {
		// "owner" is a DynamoDB reserved keyword; expressions must refer to
		// it through an ExpressionAttributeNames placeholder.
		update := "SET " + attrExpiresAt + " = :exp"
		cond := "#o = :owner"
		exp := strconv.FormatInt(d.now().UnixMilli()+ttl.Milliseconds(), 10)
		_, err := d.client.UpdateItem(context.Background(), &dynamodbv2.UpdateItemInput{
			TableName:                &d.lockTable,
			Key:                      lockKey(stage, runID),
			UpdateExpression:         &update,
			ConditionExpression:      &cond,
			ExpressionAttributeNames: map[string]string{"#o": attrOwner},
			ExpressionAttributeValues: map[string]ddbtypes.AttributeValue{
				":exp":   &ddbtypes.AttributeValueMemberN{Value: exp},
				":owner": &ddbtypes.AttributeValueMemberS{Value: owner},
			},
		})
		if isConditionalCheckFailed(err) {
			return fmt.Errorf("runstore: lease for %s/%s lost to another owner: %w", stage, runID, err)
		}
		return nil
	}
}

// releaseLock deletes the lock item if we still own it. A conditional-check
// failure means the lease already expired and was taken over — the lock is no
// longer ours to delete, which release treats as done.
func (d *dynamoDBRunStore) releaseLock(stage, runID, owner string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// "owner" is a DynamoDB reserved keyword; refer to it through an
	// ExpressionAttributeNames placeholder.
	cond := "#o = :owner"
	_, err := d.client.DeleteItem(ctx, &dynamodbv2.DeleteItemInput{
		TableName:                &d.lockTable,
		Key:                      lockKey(stage, runID),
		ConditionExpression:      &cond,
		ExpressionAttributeNames: map[string]string{"#o": attrOwner},
		ExpressionAttributeValues: map[string]ddbtypes.AttributeValue{
			":owner": &ddbtypes.AttributeValueMemberS{Value: owner},
		},
	})
	if err != nil && !isConditionalCheckFailed(err) {
		return fmt.Errorf("runstore: dynamodb lock release: %w", err)
	}
	return nil
}

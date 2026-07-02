package runstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	dynamodbv2 "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	core "github.com/runfabric/runfabric/platform/core/state/core"
)

// EnvTestDynamoEndpoint opts the integration test in against a real DynamoDB
// endpoint (DynamoDB Local), e.g.:
//
//	docker run -p 8000:8000 amazon/dynamodb-local
//	RUNFABRIC_TEST_DYNAMODB_ENDPOINT=http://localhost:8000 go test ./platform/core/state/runstore/ -run Integration -v
//
// The test creates (and deletes) its own uniquely named table.
const EnvTestDynamoEndpoint = "RUNFABRIC_TEST_DYNAMODB_ENDPOINT"

func TestDynamoIntegration(t *testing.T) {
	endpoint := os.Getenv(EnvTestDynamoEndpoint)
	if endpoint == "" {
		t.Skipf("set %s (e.g. http://localhost:8000 for DynamoDB Local) to run", EnvTestDynamoEndpoint)
	}
	// DynamoDB Local accepts any credentials, but the SDK requires some.
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		t.Setenv("AWS_ACCESS_KEY_ID", "test")
		t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	table := fmt.Sprintf("runstore-it-%d", time.Now().UnixNano())
	raw := mustRawClient(t, ctx, endpoint)
	createTestTable(t, ctx, raw, table)
	t.Cleanup(func() {
		_, _ = raw.DeleteTable(context.Background(), &dynamodbv2.DeleteTableInput{TableName: &table})
	})

	// Go through Open so the URI wiring (table, region, endpoint) is exercised.
	uri := fmt.Sprintf("dynamodb://%s/runs?region=us-east-1&endpoint=%s", table, endpoint)
	store, err := Open(uri, t.TempDir())
	if err != nil {
		t.Fatalf("Open(%s): %v", uri, err)
	}
	d := store.(*dynamoDBRunStore)
	d.lockPoll = 25 * time.Millisecond

	t.Run("SaveLoadRoundTrip", func(t *testing.T) {
		if _, _, err := d.Load(ctx, "dev", "missing"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("Load(missing) = %v, want ErrNotFound", err)
		}
		v1, err := d.Save(ctx, newRun("it-r1"), "")
		if err != nil {
			t.Fatalf("Save: %v", err)
		}
		got, v2, err := d.Load(ctx, "dev", "it-r1")
		if err != nil || got.RunID != "it-r1" || v2 != v1 {
			t.Fatalf("Load = %+v, %s, %v; want it-r1 at version %s", got, v2, err, v1)
		}
	})

	t.Run("CASConflict", func(t *testing.T) {
		v1, err := d.Save(ctx, newRun("it-cas"), "")
		if err != nil {
			t.Fatalf("initial Save: %v", err)
		}
		runA, _, _ := d.Load(ctx, "dev", "it-cas")
		runB, _, _ := d.Load(ctx, "dev", "it-cas")

		runA.Status = core.RunStatusOK
		if _, err := d.Save(ctx, runA, v1); err != nil {
			t.Fatalf("writer A Save: %v", err)
		}
		runB.Status = core.RunStatusFailed
		if _, err := d.Save(ctx, runB, v1); !errors.Is(err, ErrVersionConflict) {
			t.Fatalf("writer B Save = %v, want ErrVersionConflict", err)
		}
	})

	t.Run("ListNewestFirst", func(t *testing.T) {
		base := time.Now().UTC()
		for i, id := range []string{"it-old", "it-new"} {
			r := newRun(id)
			r.Stage = "it-list"
			r.StartedAt = base.Add(time.Duration(i) * time.Minute).Format(time.RFC3339)
			if _, err := d.Save(ctx, r, ""); err != nil {
				t.Fatalf("Save(%s): %v", id, err)
			}
		}
		runs, err := d.List(ctx, "it-list", 10)
		if err != nil || len(runs) != 2 || runs[0].RunID != "it-new" {
			t.Fatalf("List = %v, %v; want [it-new it-old]", runs, err)
		}
	})

	t.Run("LockContentionAndRelease", func(t *testing.T) {
		release, err := d.Lock(ctx, "dev", "it-lock", time.Minute)
		if err != nil {
			t.Fatalf("first Lock: %v", err)
		}
		shortCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
		defer cancel()
		if _, err := d.Lock(shortCtx, "dev", "it-lock", time.Minute); !errors.Is(err, ErrLockHeld) {
			t.Fatalf("contended Lock = %v, want ErrLockHeld", err)
		}
		if err := release(); err != nil {
			t.Fatalf("release: %v", err)
		}
		release2, err := d.Lock(ctx, "dev", "it-lock", time.Minute)
		if err != nil {
			t.Fatalf("re-Lock after release: %v", err)
		}
		if err := release2(); err != nil {
			t.Fatalf("release2: %v", err)
		}
	})

	t.Run("CrashedHolderExpiry", func(t *testing.T) {
		// Simulate a crashed holder: write the lease directly (no heartbeat, no
		// release), as if the owning process died right after acquiring.
		ttl := 500 * time.Millisecond
		acquired, err := d.tryAcquire(ctx, "dev", "it-crash", "dead-owner", ttl)
		if err != nil || !acquired {
			t.Fatalf("seed crashed lease: acquired=%v err=%v", acquired, err)
		}

		// While the lease is live, takeover must fail.
		shortCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()
		if _, err := d.Lock(shortCtx, "dev", "it-crash", time.Minute); !errors.Is(err, ErrLockHeld) {
			t.Fatalf("pre-expiry Lock = %v, want ErrLockHeld", err)
		}

		// After ttl the lease is stale and a new holder takes over.
		time.Sleep(ttl + 100*time.Millisecond)
		release, err := d.Lock(ctx, "dev", "it-crash", time.Minute)
		if err != nil {
			t.Fatalf("post-expiry Lock: %v", err)
		}
		if err := release(); err != nil {
			t.Fatalf("release: %v", err)
		}
	})
}

func mustRawClient(t *testing.T, ctx context.Context, endpoint string) *dynamodbv2.Client {
	t.Helper()
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion("us-east-1"))
	if err != nil {
		t.Fatalf("load aws config: %v", err)
	}
	return dynamodbv2.NewFromConfig(cfg, func(o *dynamodbv2.Options) {
		o.BaseEndpoint = &endpoint
	})
}

func createTestTable(t *testing.T, ctx context.Context, c *dynamodbv2.Client, table string) {
	t.Helper()
	_, err := c.CreateTable(ctx, &dynamodbv2.CreateTableInput{
		TableName:   &table,
		BillingMode: ddbtypes.BillingModePayPerRequest,
		AttributeDefinitions: []ddbtypes.AttributeDefinition{
			{AttributeName: strPtr(attrPK), AttributeType: ddbtypes.ScalarAttributeTypeS},
			{AttributeName: strPtr(attrSK), AttributeType: ddbtypes.ScalarAttributeTypeS},
		},
		KeySchema: []ddbtypes.KeySchemaElement{
			{AttributeName: strPtr(attrPK), KeyType: ddbtypes.KeyTypeHash},
			{AttributeName: strPtr(attrSK), KeyType: ddbtypes.KeyTypeRange},
		},
	})
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	for i := 0; i < 100; i++ {
		out, err := c.DescribeTable(ctx, &dynamodbv2.DescribeTableInput{TableName: &table})
		if err == nil && out.Table.TableStatus == ddbtypes.TableStatusActive {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("table %s did not become ACTIVE", table)
}

func strPtr(s string) *string { return &s }

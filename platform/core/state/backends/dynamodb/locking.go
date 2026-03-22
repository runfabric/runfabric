package dynamodb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/runfabric/runfabric/platform/core/state/locking"

	dynamodbv2 "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type LockBackend struct {
	client *Client
}

func NewLockBackend(client *Client) *LockBackend {
	return &LockBackend{client: client}
}

func (b *LockBackend) Kind() string {
	return "dynamodb"
}

func (b *LockBackend) lockKey(service, stage string) string {
	return service + ":" + stage
}

func (b *LockBackend) Acquire(service, stage, operation string, staleAfter time.Duration) (*locking.Handle, error) {
	token, err := randomToken()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	expiresAt := now.Add(staleAfter)
	key := b.lockKey(service, stage)

	_, err = b.client.DB.PutItem(
		context.Background(),
		&dynamodbv2.PutItemInput{
			TableName: &b.client.TableName,
			Item: map[string]types.AttributeValue{
				"lockKey":         &types.AttributeValueMemberS{Value: key},
				"service":         &types.AttributeValueMemberS{Value: service},
				"stage":           &types.AttributeValueMemberS{Value: stage},
				"operation":       &types.AttributeValueMemberS{Value: operation},
				"ownerToken":      &types.AttributeValueMemberS{Value: token},
				"createdAt":       &types.AttributeValueMemberS{Value: now.Format(time.RFC3339)},
				"expiresAt":       &types.AttributeValueMemberS{Value: expiresAt.Format(time.RFC3339)},
				"lastHeartbeatAt": &types.AttributeValueMemberS{Value: now.Format(time.RFC3339)},
				"ttl":             &types.AttributeValueMemberN{Value: strconv.FormatInt(expiresAt.Unix(), 10)},
			},
			ConditionExpression: str("attribute_not_exists(lockKey)"),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("acquire remote lock: %w", err)
	}

	return locking.NewHandle(service, stage, token, b, b, b), nil
}

func (b *LockBackend) Read(service, stage string) (*locking.LockRecord, error) {
	key := b.lockKey(service, stage)

	out, err := b.client.DB.GetItem(
		context.Background(),
		&dynamodbv2.GetItemInput{
			TableName: &b.client.TableName,
			Key: map[string]types.AttributeValue{
				"lockKey": &types.AttributeValueMemberS{Value: key},
			},
		},
	)
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, fmt.Errorf("lock not found")
	}

	rec := &locking.LockRecord{}
	if v, ok := out.Item["service"].(*types.AttributeValueMemberS); ok {
		rec.Service = v.Value
	}
	if v, ok := out.Item["stage"].(*types.AttributeValueMemberS); ok {
		rec.Stage = v.Value
	}
	if v, ok := out.Item["operation"].(*types.AttributeValueMemberS); ok {
		rec.Operation = v.Value
	}
	if v, ok := out.Item["ownerToken"].(*types.AttributeValueMemberS); ok {
		rec.OwnerToken = v.Value
	}
	if v, ok := out.Item["createdAt"].(*types.AttributeValueMemberS); ok {
		rec.CreatedAt = v.Value
	}
	if v, ok := out.Item["expiresAt"].(*types.AttributeValueMemberS); ok {
		rec.ExpiresAt = v.Value
	}
	if v, ok := out.Item["lastHeartbeatAt"].(*types.AttributeValueMemberS); ok {
		rec.LastHeartbeatAt = v.Value
	}

	return rec, nil
}

func (b *LockBackend) Release(service, stage string) error {
	key := b.lockKey(service, stage)

	_, err := b.client.DB.DeleteItem(
		context.Background(),
		&dynamodbv2.DeleteItemInput{
			TableName: &b.client.TableName,
			Key: map[string]types.AttributeValue{
				"lockKey": &types.AttributeValueMemberS{Value: key},
			},
		},
	)
	return err
}

func (b *LockBackend) ReleaseOwned(service, stage, ownerToken string) error {
	key := b.lockKey(service, stage)

	_, err := b.client.DB.DeleteItem(
		context.Background(),
		&dynamodbv2.DeleteItemInput{
			TableName: &b.client.TableName,
			Key: map[string]types.AttributeValue{
				"lockKey": &types.AttributeValueMemberS{Value: key},
			},
			ConditionExpression: str("ownerToken = :owner"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":owner": &types.AttributeValueMemberS{Value: ownerToken},
			},
		},
	)
	if err != nil {
		return fmt.Errorf("release owned remote lock: %w", err)
	}
	return nil
}

func (b *LockBackend) Renew(service, stage, ownerToken string, leaseFor time.Duration) error {
	now := time.Now().UTC()
	expiresAt := now.Add(leaseFor)
	key := b.lockKey(service, stage)

	_, err := b.client.DB.UpdateItem(
		context.Background(),
		&dynamodbv2.UpdateItemInput{
			TableName: &b.client.TableName,
			Key: map[string]types.AttributeValue{
				"lockKey": &types.AttributeValueMemberS{Value: key},
			},
			UpdateExpression:    str("SET expiresAt = :exp, lastHeartbeatAt = :hb, ttl = :ttl"),
			ConditionExpression: str("ownerToken = :owner"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":exp":   &types.AttributeValueMemberS{Value: expiresAt.Format(time.RFC3339)},
				":hb":    &types.AttributeValueMemberS{Value: now.Format(time.RFC3339)},
				":ttl":   &types.AttributeValueMemberN{Value: strconv.FormatInt(expiresAt.Unix(), 10)},
				":owner": &types.AttributeValueMemberS{Value: ownerToken},
			},
		},
	)
	return err
}

func (b *LockBackend) Steal(service, stage, newOperation string, staleAfter time.Duration) (*locking.Handle, error) {
	existing, err := b.Read(service, stage)
	if err != nil {
		return b.Acquire(service, stage, newOperation, staleAfter)
	}

	exp, err := time.Parse(time.RFC3339, existing.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("parse existing lock expiry: %w", err)
	}
	if time.Now().UTC().Before(exp) {
		return nil, fmt.Errorf("cannot steal active lock owned by %s", existing.OwnerToken)
	}

	if err := b.ReleaseOwned(service, stage, existing.OwnerToken); err != nil {
		return nil, err
	}
	return b.Acquire(service, stage, newOperation, staleAfter)
}

func randomToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

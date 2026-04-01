package dynamodb

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	statetypes "github.com/runfabric/runfabric/extensions/types"
)

// ReceiptBackend stores deploy receipts in DynamoDB.
// Table must have partition key "pk" (S) and sort key "sk" (S).
// Items: pk = workspaceID, sk = "STAGE#" + stage, data = receipt JSON in attribute "data" (S).
type ReceiptBackend struct {
	client      *Client
	workspaceID string
}

// NewReceiptBackend returns a receipt backend for DynamoDB. Use a Client with the receipt table name.
func NewReceiptBackend(client *Client, workspaceID string) *ReceiptBackend {
	return &ReceiptBackend{client: client, workspaceID: workspaceID}
}

func (b *ReceiptBackend) sk(stage string) string {
	return "STAGE#" + stage
}

func (b *ReceiptBackend) Load(stage string) (*statetypes.Receipt, error) {
	out, err := b.client.DB.GetItem(context.Background(), &dynamodb.GetItemInput{
		TableName: &b.client.TableName,
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: b.workspaceID},
			"sk": &types.AttributeValueMemberS{Value: b.sk(stage)},
		},
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, nil
	}
	dataAttr, ok := out.Item["data"]
	if !ok {
		return nil, fmt.Errorf("missing data attribute")
	}
	dataVal, ok := dataAttr.(*types.AttributeValueMemberS)
	if !ok {
		return nil, fmt.Errorf("data is not string")
	}
	var r statetypes.Receipt
	if err := json.Unmarshal([]byte(dataVal.Value), &r); err != nil {
		return nil, fmt.Errorf("unmarshal receipt: %w", err)
	}
	return &r, nil
}

func (b *ReceiptBackend) Save(receipt *statetypes.Receipt) error {
	if receipt == nil {
		return fmt.Errorf("nil receipt")
	}
	data, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	_, err = b.client.DB.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName: &b.client.TableName,
		Item: map[string]types.AttributeValue{
			"pk":        &types.AttributeValueMemberS{Value: b.workspaceID},
			"sk":        &types.AttributeValueMemberS{Value: b.sk(receipt.Stage)},
			"data":      &types.AttributeValueMemberS{Value: string(data)},
			"updatedAt": &types.AttributeValueMemberS{Value: receipt.UpdatedAt},
		},
	})
	return err
}

func (b *ReceiptBackend) Delete(stage string) error {
	_, err := b.client.DB.DeleteItem(context.Background(), &dynamodb.DeleteItemInput{
		TableName: &b.client.TableName,
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: b.workspaceID},
			"sk": &types.AttributeValueMemberS{Value: b.sk(stage)},
		},
	})
	return err
}

func (b *ReceiptBackend) ListReleases() ([]statetypes.ReleaseEntry, error) {
	out, err := b.client.DB.Query(context.Background(), &dynamodb.QueryInput{
		TableName:              &b.client.TableName,
		KeyConditionExpression: aws.String("pk = :pk AND begins_with(sk, :prefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":     &types.AttributeValueMemberS{Value: b.workspaceID},
			":prefix": &types.AttributeValueMemberS{Value: "STAGE#"},
		},
	})
	if err != nil {
		return nil, err
	}
	var entries []statetypes.ReleaseEntry
	for _, item := range out.Items {
		skAttr, ok := item["sk"]
		if !ok {
			continue
		}
		skVal, ok := skAttr.(*types.AttributeValueMemberS)
		if !ok {
			continue
		}
		stage := skVal.Value
		if len(stage) > 6 {
			stage = stage[6:] // strip "STAGE#"
		}
		updatedAt := ""
		if u, ok := item["updatedAt"]; ok {
			if uv, ok := u.(*types.AttributeValueMemberS); ok {
				updatedAt = uv.Value
			}
		}
		entries = append(entries, statetypes.ReleaseEntry{Stage: stage, UpdatedAt: updatedAt})
	}
	return entries, nil
}

func (b *ReceiptBackend) Kind() string {
	return "dynamodb"
}

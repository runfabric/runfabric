package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	s3v2 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/runfabric/runfabric/internal/state"
)

type ReceiptBackend struct {
	Client *Client
	Ctx    context.Context
}

func NewReceiptBackend(ctx context.Context, client *Client) *ReceiptBackend {
	return &ReceiptBackend{
		Client: client,
		Ctx:    ctx,
	}
}

func (b *ReceiptBackend) key(stage string) string {
	return fmt.Sprintf("%s/receipts/%s.receipt.json", b.Client.Prefix, stage)
}

func (b *ReceiptBackend) Load(stage string) (*state.Receipt, error) {
	out, err := b.Client.S3.GetObject(b.Ctx, &s3v2.GetObjectInput{
		Bucket: &b.Client.Bucket,
		Key:    aws.String(b.key(stage)),
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, err
	}

	var r state.Receipt
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func (b *ReceiptBackend) Save(receipt *state.Receipt) error {
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}

	_, err = b.Client.S3.PutObject(b.Ctx, &s3v2.PutObjectInput{
		Bucket:      &b.Client.Bucket,
		Key:         aws.String(b.key(receipt.Stage)),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("application/json"),
	})
	return err
}

func (b *ReceiptBackend) Delete(stage string) error {
	_, err := b.Client.S3.DeleteObject(b.Ctx, &s3v2.DeleteObjectInput{
		Bucket: &b.Client.Bucket,
		Key:    aws.String(b.key(stage)),
	})
	return err
}

func (b *ReceiptBackend) Kind() string {
	return "dynamodb"
}

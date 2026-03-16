package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	s3v2 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/runfabric/runfabric/engine/internal/state"
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
	if b.Client == nil || b.Client.S3 == nil {
		return nil, fmt.Errorf("s3 receipt backend: client not initialized")
	}
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
	if b.Client == nil || b.Client.S3 == nil {
		return fmt.Errorf("s3 receipt backend: client not initialized")
	}
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
	if b.Client == nil || b.Client.S3 == nil {
		return fmt.Errorf("s3 receipt backend: client not initialized")
	}
	_, err := b.Client.S3.DeleteObject(b.Ctx, &s3v2.DeleteObjectInput{
		Bucket: &b.Client.Bucket,
		Key:    aws.String(b.key(stage)),
	})
	return err
}

func (b *ReceiptBackend) ListReleases() ([]state.ReleaseEntry, error) {
	if b.Client == nil || b.Client.S3 == nil || b.Client.Bucket == "" {
		return nil, nil
	}
	prefix := b.Client.Prefix + "/receipts/"
	input := &s3v2.ListObjectsV2Input{
		Bucket: aws.String(b.Client.Bucket),
		Prefix: aws.String(prefix),
	}
	paginator := s3v2.NewListObjectsV2Paginator(b.Client.S3, input)
	var out []state.ReleaseEntry
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(b.Ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}
			// key is prefix/receipts/{stage}.receipt.json
			key := *obj.Key
			if len(key) <= len(prefix)+14 {
				continue
			}
			stage := key[len(prefix) : len(key)-14] // strip .receipt.json
			r, err := b.Load(stage)
			if err != nil {
				continue
			}
			out = append(out, state.ReleaseEntry{Stage: stage, UpdatedAt: r.UpdatedAt})
		}
	}
	return out, nil
}

func (b *ReceiptBackend) Kind() string {
	return "s3"
}

package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	s3v2 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/runfabric/runfabric/engine/internal/errors"
	"github.com/runfabric/runfabric/engine/internal/transactions"
)

type JournalBackend struct {
	Client *Client
	Ctx    context.Context
}

func NewJournalBackend(ctx context.Context, client *Client) *JournalBackend {
	return &JournalBackend{
		Client: client,
		Ctx:    ctx,
	}
}

func (b *JournalBackend) key(service, stage string) string {
	return fmt.Sprintf("%s/journals/%s-%s.journal.json", b.Client.Prefix, service, stage)
}

func (b *JournalBackend) Load(service, stage string) (*transactions.JournalFile, error) {
	out, err := b.Client.S3.GetObject(b.Ctx, &s3v2.GetObjectInput{
		Bucket: &b.Client.Bucket,
		Key:    aws.String(b.key(service, stage)),
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, err
	}

	var jf transactions.JournalFile
	if err := json.Unmarshal(data, &jf); err != nil {
		return nil, err
	}
	return &jf, nil
}

func (b *JournalBackend) Save(j *transactions.JournalFile) error {
	current, err := b.Load(j.Service, j.Stage)

	if err == nil && current != nil {
		if j.Version < current.Version {
			return &errors.ConflictError{
				Backend:         "s3",
				Service:         j.Service,
				Stage:           j.Stage,
				Resource:        "journal",
				CurrentVersion:  current.Version,
				IncomingVersion: j.Version,
				Action:          "inspect journal and retry with latest state",
			}
		}
	}

	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}

	_, err = b.Client.S3.PutObject(b.Ctx, &s3v2.PutObjectInput{
		Bucket:      &b.Client.Bucket,
		Key:         aws.String(b.key(j.Service, j.Stage)),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("application/json"),
	})
	return err
}

func (b *JournalBackend) Delete(service, stage string) error {

	_, err := b.Client.S3.DeleteObject(b.Ctx, &s3v2.DeleteObjectInput{
		Bucket: &b.Client.Bucket,
		Key:    aws.String(b.key(service, stage)),
	})
	return err
}

func (b *JournalBackend) Kind() string {
	return "local"
}

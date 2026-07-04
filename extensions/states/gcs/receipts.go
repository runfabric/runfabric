package gcs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	statetypes "github.com/runfabric/runfabric/extensions/types"
)

// ReceiptBackend stores receipts as JSON objects at
// <prefix>/receipts/<stage>.receipt.json — the same layout as the s3 backend.
type ReceiptBackend struct {
	Client *Client
}

func NewReceiptBackend(client *Client) *ReceiptBackend {
	return &ReceiptBackend{Client: client}
}

const receiptSuffix = ".receipt.json"

func (b *ReceiptBackend) key(stage string) string {
	if b.Client.Prefix == "" {
		return fmt.Sprintf("receipts/%s%s", stage, receiptSuffix)
	}
	return fmt.Sprintf("%s/receipts/%s%s", b.Client.Prefix, stage, receiptSuffix)
}

func (b *ReceiptBackend) Load(stage string) (*statetypes.Receipt, error) {
	if b.Client == nil {
		return nil, fmt.Errorf("gcs receipt backend: client not initialized")
	}
	u := fmt.Sprintf("%s/b/%s/o/%s?alt=media", b.Client.BaseURL, url.PathEscape(b.Client.Bucket), url.PathEscape(b.key(stage)))
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := b.Client.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gcs get %s: %s: %s", b.key(stage), resp.Status, string(body))
	}
	var r statetypes.Receipt
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func (b *ReceiptBackend) Save(receipt *statetypes.Receipt) error {
	if b.Client == nil {
		return fmt.Errorf("gcs receipt backend: client not initialized")
	}
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	u := fmt.Sprintf("%s/b/%s/o?uploadType=media&name=%s",
		b.Client.UploadBaseURL, url.PathEscape(b.Client.Bucket), url.QueryEscape(b.key(receipt.Stage)))
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.Client.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gcs upload %s: %s: %s", b.key(receipt.Stage), resp.Status, string(body))
	}
	return nil
}

func (b *ReceiptBackend) Delete(stage string) error {
	if b.Client == nil {
		return fmt.Errorf("gcs receipt backend: client not initialized")
	}
	u := fmt.Sprintf("%s/b/%s/o/%s", b.Client.BaseURL, url.PathEscape(b.Client.Bucket), url.PathEscape(b.key(stage)))
	req, err := http.NewRequest(http.MethodDelete, u, nil)
	if err != nil {
		return err
	}
	resp, err := b.Client.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// 404 tolerated: deleting an absent receipt is a no-op, matching local semantics.
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gcs delete %s: %s: %s", b.key(stage), resp.Status, string(body))
	}
	return nil
}

func (b *ReceiptBackend) ListReleases() ([]statetypes.ReleaseEntry, error) {
	if b.Client == nil {
		return nil, nil
	}
	prefix := "receipts/"
	if b.Client.Prefix != "" {
		prefix = b.Client.Prefix + "/receipts/"
	}
	var out []statetypes.ReleaseEntry
	pageToken := ""
	for {
		u := fmt.Sprintf("%s/b/%s/o?prefix=%s", b.Client.BaseURL, url.PathEscape(b.Client.Bucket), url.QueryEscape(prefix))
		if pageToken != "" {
			u += "&pageToken=" + url.QueryEscape(pageToken)
		}
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		resp, err := b.Client.do(req)
		if err != nil {
			return nil, err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("gcs list %s: %s: %s", prefix, resp.Status, string(body))
		}
		var page struct {
			Items []struct {
				Name string `json:"name"`
			} `json:"items"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, err
		}
		for _, item := range page.Items {
			if !strings.HasPrefix(item.Name, prefix) || !strings.HasSuffix(item.Name, receiptSuffix) {
				continue
			}
			stage := strings.TrimSuffix(strings.TrimPrefix(item.Name, prefix), receiptSuffix)
			r, err := b.Load(stage)
			if err != nil {
				continue
			}
			out = append(out, statetypes.ReleaseEntry{Stage: stage, UpdatedAt: r.UpdatedAt})
		}
		if page.NextPageToken == "" {
			return out, nil
		}
		pageToken = page.NextPageToken
	}
}

func (b *ReceiptBackend) Kind() string {
	return "gcs"
}

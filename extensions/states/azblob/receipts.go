package azblob

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	statetypes "github.com/runfabric/runfabric/extensions/types"
)

// ReceiptBackend stores receipts as JSON block blobs at
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
		return nil, fmt.Errorf("azblob receipt backend: client not initialized")
	}
	req, err := http.NewRequest(http.MethodGet, b.Client.blobURL(b.key(stage)), nil)
	if err != nil {
		return nil, err
	}
	resp, err := b.Client.do(req, 0)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("azblob get %s: %s: %s", b.key(stage), resp.Status, string(body))
	}
	var r statetypes.Receipt
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func (b *ReceiptBackend) Save(receipt *statetypes.Receipt) error {
	if b.Client == nil {
		return fmt.Errorf("azblob receipt backend: client not initialized")
	}
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, b.Client.blobURL(b.key(receipt.Stage)), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("x-ms-blob-type", "BlockBlob")
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.Client.do(req, len(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("azblob put %s: %s: %s", b.key(receipt.Stage), resp.Status, string(body))
	}
	return nil
}

func (b *ReceiptBackend) Delete(stage string) error {
	if b.Client == nil {
		return fmt.Errorf("azblob receipt backend: client not initialized")
	}
	req, err := http.NewRequest(http.MethodDelete, b.Client.blobURL(b.key(stage)), nil)
	if err != nil {
		return err
	}
	resp, err := b.Client.do(req, 0)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// 404 tolerated: deleting an absent receipt is a no-op, matching local semantics.
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("azblob delete %s: %s: %s", b.key(stage), resp.Status, string(body))
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
	marker := ""
	for {
		u := fmt.Sprintf("%s/%s?restype=container&comp=list&prefix=%s",
			b.Client.Endpoint, url.PathEscape(b.Client.Container), url.QueryEscape(prefix))
		if marker != "" {
			u += "&marker=" + url.QueryEscape(marker)
		}
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		resp, err := b.Client.do(req, 0)
		if err != nil {
			return nil, err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("azblob list %s: %s: %s", prefix, resp.Status, string(body))
		}
		var page struct {
			Blobs struct {
				Blob []struct {
					Name string `xml:"Name"`
				} `xml:"Blob"`
			} `xml:"Blobs"`
			NextMarker string `xml:"NextMarker"`
		}
		if err := xml.Unmarshal(body, &page); err != nil {
			return nil, err
		}
		for _, blob := range page.Blobs.Blob {
			if !strings.HasPrefix(blob.Name, prefix) || !strings.HasSuffix(blob.Name, receiptSuffix) {
				continue
			}
			stage := strings.TrimSuffix(strings.TrimPrefix(blob.Name, prefix), receiptSuffix)
			r, err := b.Load(stage)
			if err != nil {
				continue
			}
			out = append(out, statetypes.ReleaseEntry{Stage: stage, UpdatedAt: r.UpdatedAt})
		}
		if page.NextMarker == "" {
			return out, nil
		}
		marker = page.NextMarker
	}
}

func (b *ReceiptBackend) Kind() string {
	return "azblob"
}

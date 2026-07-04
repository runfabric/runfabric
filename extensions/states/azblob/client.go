// Package azblob implements an Azure Blob Storage receipt state backend over
// the Blob REST API with SharedKey auth. Credentials come from
// AZURE_STORAGE_CONNECTION_STRING, or AZURE_STORAGE_ACCOUNT + AZURE_STORAGE_KEY.
package azblob

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

const apiVersion = "2021-08-06"

// Client is a minimal Blob REST client scoped to one container/prefix.
type Client struct {
	// Endpoint is the blob service base URL including the account path for
	// emulator-style endpoints, e.g. https://acct.blob.core.windows.net or
	// http://127.0.0.1:10000/devstoreaccount1.
	Endpoint  string
	Account   string
	Key       string // base64 account key
	Container string
	Prefix    string
	HTTP      *http.Client
	// now is injectable for tests.
	now func() time.Time
}

// New builds a client for container/prefix from the environment:
// AZURE_STORAGE_CONNECTION_STRING wins, else AZURE_STORAGE_ACCOUNT + AZURE_STORAGE_KEY.
func New(container, prefix string) (*Client, error) {
	if strings.TrimSpace(container) == "" {
		return nil, fmt.Errorf("azblob state backend: container is required")
	}
	c := &Client{
		Container: container,
		Prefix:    strings.Trim(strings.TrimSpace(prefix), "/"),
		HTTP:      &http.Client{Timeout: 30 * time.Second},
		now:       time.Now,
	}
	if cs := strings.TrimSpace(os.Getenv("AZURE_STORAGE_CONNECTION_STRING")); cs != "" {
		if err := c.applyConnectionString(cs); err != nil {
			return nil, err
		}
	} else {
		c.Account = strings.TrimSpace(os.Getenv("AZURE_STORAGE_ACCOUNT"))
		c.Key = strings.TrimSpace(os.Getenv("AZURE_STORAGE_KEY"))
		if c.Account != "" {
			c.Endpoint = fmt.Sprintf("https://%s.blob.core.windows.net", c.Account)
		}
	}
	if c.Account == "" || c.Key == "" {
		return nil, fmt.Errorf("azblob state backend: set AZURE_STORAGE_CONNECTION_STRING, or AZURE_STORAGE_ACCOUNT + AZURE_STORAGE_KEY")
	}
	if _, err := base64.StdEncoding.DecodeString(c.Key); err != nil {
		return nil, fmt.Errorf("azblob state backend: account key must be base64: %w", err)
	}
	return c, nil
}

func (c *Client) applyConnectionString(cs string) error {
	suffix := "core.windows.net"
	protocol := "https"
	var blobEndpoint string
	for _, part := range strings.Split(cs, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch strings.ToLower(kv[0]) {
		case "accountname":
			c.Account = kv[1]
		case "accountkey":
			c.Key = kv[1]
		case "endpointsuffix":
			suffix = kv[1]
		case "defaultendpointsprotocol":
			protocol = kv[1]
		case "blobendpoint":
			blobEndpoint = kv[1]
		}
	}
	if blobEndpoint != "" {
		c.Endpoint = strings.TrimRight(blobEndpoint, "/")
	} else if c.Account != "" {
		c.Endpoint = fmt.Sprintf("%s://%s.blob.%s", protocol, c.Account, suffix)
	}
	return nil
}

// blobURL returns the full URL for a blob path within the container.
func (c *Client) blobURL(blobPath string) string {
	segments := strings.Split(blobPath, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return fmt.Sprintf("%s/%s/%s", c.Endpoint, url.PathEscape(c.Container), strings.Join(segments, "/"))
}

// do signs the request with SharedKey and executes it.
func (c *Client) do(req *http.Request, contentLength int) (*http.Response, error) {
	req.Header.Set("x-ms-date", c.now().UTC().Format(http.TimeFormat))
	req.Header.Set("x-ms-version", apiVersion)
	auth, err := c.sharedKeyAuth(req, contentLength)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", auth)
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return client.Do(req)
}

// sharedKeyAuth computes the SharedKey Authorization header
// (api-version >= 2015-02-21: empty Content-Length when zero).
func (c *Client) sharedKeyAuth(req *http.Request, contentLength int) (string, error) {
	lengthStr := ""
	if contentLength > 0 {
		lengthStr = fmt.Sprintf("%d", contentLength)
	}
	var canonHeaders strings.Builder
	var msHeaders []string
	for name := range req.Header {
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "x-ms-") {
			msHeaders = append(msHeaders, lower)
		}
	}
	sort.Strings(msHeaders)
	for _, name := range msHeaders {
		canonHeaders.WriteString(name + ":" + strings.TrimSpace(req.Header.Get(name)) + "\n")
	}

	// CanonicalizedResource: /account/path plus sorted query params as name:value lines.
	var canonResource strings.Builder
	path := req.URL.EscapedPath()
	// Emulator endpoints already carry the account as the first path segment;
	// strip it so it is not doubled after the canonical /account prefix.
	if strings.HasPrefix(path, "/"+c.Account+"/") {
		path = strings.TrimPrefix(path, "/"+c.Account)
	}
	canonResource.WriteString("/" + c.Account + path)
	if len(req.URL.Query()) > 0 {
		params := req.URL.Query()
		names := make([]string, 0, len(params))
		for name := range params {
			names = append(names, strings.ToLower(name))
		}
		sort.Strings(names)
		for _, name := range names {
			values := params[name]
			sort.Strings(values)
			canonResource.WriteString("\n" + name + ":" + strings.Join(values, ","))
		}
	}

	stringToSign := strings.Join([]string{
		req.Method,
		req.Header.Get("Content-Encoding"),
		req.Header.Get("Content-Language"),
		lengthStr,
		req.Header.Get("Content-MD5"),
		req.Header.Get("Content-Type"),
		"", // Date — x-ms-date is used instead
		req.Header.Get("If-Modified-Since"),
		req.Header.Get("If-Match"),
		req.Header.Get("If-None-Match"),
		req.Header.Get("If-Unmodified-Since"),
		req.Header.Get("Range"),
	}, "\n") + "\n" + canonHeaders.String() + canonResource.String()

	key, err := base64.StdEncoding.DecodeString(c.Key)
	if err != nil {
		return "", fmt.Errorf("azblob: decode account key: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(stringToSign))
	return "SharedKey " + c.Account + ":" + base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

type s3Presigner struct {
	bucket       string
	region       string
	endpoint     string
	accessKeyID  string
	secretKey    string
	sessionToken string
}

func newS3Presigner(opts Options) *s3Presigner {
	bucket := strings.TrimSpace(opts.S3Bucket)
	region := strings.TrimSpace(opts.S3Region)
	ak := strings.TrimSpace(opts.S3AccessKeyID)
	sk := strings.TrimSpace(opts.S3SecretAccessKey)
	if bucket == "" || region == "" || ak == "" || sk == "" {
		return nil
	}
	endpoint := strings.TrimSpace(opts.S3Endpoint)
	if endpoint == "" {
		endpoint = "https://s3." + region + ".amazonaws.com"
	}
	endpoint = strings.TrimRight(endpoint, "/")
	return &s3Presigner{
		bucket:       bucket,
		region:       region,
		endpoint:     endpoint,
		accessKeyID:  ak,
		secretKey:    sk,
		sessionToken: strings.TrimSpace(opts.S3SessionToken),
	}
}

func (s *Server) presignS3(key, method string, ttl time.Duration) (string, bool) {
	if s == nil || s.s3Presigner == nil {
		return "", false
	}
	u, err := s.s3Presigner.presign(strings.TrimSpace(key), strings.ToUpper(strings.TrimSpace(method)), ttl)
	if err != nil {
		return "", false
	}
	return u, true
}

func (p *s3Presigner) presign(key, method string, ttl time.Duration) (string, error) {
	if p == nil {
		return "", fmt.Errorf("s3 presigner unavailable")
	}
	if key == "" {
		return "", fmt.Errorf("artifact key is required")
	}
	if method != "GET" && method != "PUT" {
		return "", fmt.Errorf("unsupported method")
	}
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	secs := int(ttl.Seconds())
	if secs > 7*24*3600 {
		secs = 7 * 24 * 3600
	}

	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	scope := dateStamp + "/" + p.region + "/s3/aws4_request"

	base, err := url.Parse(p.endpoint)
	if err != nil {
		return "", err
	}
	host := base.Host
	canonicalURI := "/" + p.bucket + "/" + uriEncodePath(key)

	params := map[string]string{
		"X-Amz-Algorithm":     "AWS4-HMAC-SHA256",
		"X-Amz-Credential":    p.accessKeyID + "/" + scope,
		"X-Amz-Date":          amzDate,
		"X-Amz-Expires":       fmt.Sprintf("%d", secs),
		"X-Amz-SignedHeaders": "host",
	}
	if p.sessionToken != "" {
		params["X-Amz-Security-Token"] = p.sessionToken
	}
	canonicalQuery := canonicalizeQuery(params)
	canonicalHeaders := "host:" + host + "\n"
	payloadHash := "UNSIGNED-PAYLOAD"
	canonicalRequest := strings.Join([]string{
		method,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders,
		"host",
		payloadHash,
	}, "\n")
	crHash := sha256Hex([]byte(canonicalRequest))
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		crHash,
	}, "\n")
	signingKey := awsSigningKey(p.secretKey, dateStamp, p.region, "s3")
	sig := hmacSHA256Hex(signingKey, stringToSign)
	params["X-Amz-Signature"] = sig

	out := *base
	out.Path = canonicalURI
	out.RawQuery = canonicalizeQuery(params)
	return out.String(), nil
}

func uriEncodePath(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i := range parts {
		parts[i] = queryEscapeRFC3986(parts[i])
	}
	return strings.Join(parts, "/")
}

func canonicalizeQuery(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, queryEscapeRFC3986(k)+"="+queryEscapeRFC3986(params[k]))
	}
	return strings.Join(pairs, "&")
}

func queryEscapeRFC3986(s string) string {
	escaped := url.QueryEscape(s)
	escaped = strings.ReplaceAll(escaped, "+", "%20")
	escaped = strings.ReplaceAll(escaped, "*", "%2A")
	escaped = strings.ReplaceAll(escaped, "%7E", "~")
	return escaped
}

func awsSigningKey(secret, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), dateStamp)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	return hmacSHA256(kService, "aws4_request")
}

func hmacSHA256(key []byte, data string) []byte {
	m := hmac.New(sha256.New, key)
	_, _ = m.Write([]byte(data))
	return m.Sum(nil)
}

func hmacSHA256Hex(key []byte, data string) string {
	return hex.EncodeToString(hmacSHA256(key, data))
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

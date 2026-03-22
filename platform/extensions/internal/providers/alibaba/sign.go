// Package alibaba implements Alibaba FC deploy/remove/invoke/logs and trigger resources per capability matrix.
package alibaba

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"sort"
	"strings"
	"time"
)

const fcAPIVersion = "2021-04-06"
const fcDateLayout = "Mon, 02 Jan 2006 15:04:05 GMT"

// SignRequest signs an FC OpenAPI request per Alibaba signature authentication.
// Authorization = "FC " + accessKeyID + ":" + base64(hmac-sha256(StringToSign))
// StringToSign = METHOD + "\n" + Content-MD5 + "\n" + Content-Type + "\n" + Date + "\n" + CanonicalizedFCHeaders + CanonicalizedResource
func SignRequest(method, canonicalResource, body string, fcHeaders map[string]string, accessKeyID, accessKeySecret string) (date, auth string, err error) {
	now := time.Now().UTC()
	date = now.Format(fcDateLayout)
	contentMD5 := ""
	if body != "" {
		contentMD5 = base64.StdEncoding.EncodeToString(md5Sum([]byte(body)))
	}
	contentType := "application/json"
	var canonicalHeaders strings.Builder
	lowerToVal := make(map[string]string)
	for k, v := range fcHeaders {
		lower := strings.ToLower(strings.TrimSpace(k))
		if strings.HasPrefix(lower, "x-fc-") {
			lowerToVal[lower] = v
		}
	}
	keys := make([]string, 0, len(lowerToVal))
	for k := range lowerToVal {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		canonicalHeaders.WriteString(k)
		canonicalHeaders.WriteString(":")
		canonicalHeaders.WriteString(lowerToVal[k])
		canonicalHeaders.WriteString("\n")
	}
	stringToSign := method + "\n" + contentMD5 + "\n" + contentType + "\n" + date + "\n" + canonicalHeaders.String() + canonicalResource
	h := hmac.New(sha256.New, []byte(accessKeySecret))
	h.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))
	auth = "FC " + accessKeyID + ":" + signature
	return date, auth, nil
}

func md5Sum(data []byte) []byte {
	h := md5.New()
	h.Write(data)
	return h.Sum(nil)
}

package alibaba

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/runfabric/runfabric/internal/apiutil"
)

const fcHostFmt = "https://%s.%s.fc.aliyuncs.com"

// fcClient performs signed requests to Alibaba FC OpenAPI.
type fcClient struct {
	accountID string
	region    string
	accessKey string
	secretKey string
	client    *http.Client
}

func newFCClient(accountID, region, accessKey, secretKey string) *fcClient {
	if region == "" {
		region = "cn-hangzhou"
	}
	return &fcClient{
		accountID: accountID,
		region:    region,
		accessKey: accessKey,
		secretKey: secretKey,
		client:    apiutil.DefaultClient,
	}
}

func (c *fcClient) baseURL() string {
	return fmt.Sprintf(fcHostFmt, c.accountID, c.region)
}

// doSigned executes a signed request. path is the canonical resource e.g. /2021-04-06/services.
func (c *fcClient) doSigned(ctx context.Context, method, path string, body []byte, fcHeaders map[string]string) ([]byte, int, error) {
	url := c.baseURL() + path
	bodyStr := ""
	if len(body) > 0 {
		bodyStr = string(body)
	}
	if fcHeaders == nil {
		fcHeaders = make(map[string]string)
	}
	fcHeaders["X-Fc-Account-Id"] = c.accountID
	date, auth, err := SignRequest(method, path, bodyStr, fcHeaders, c.accessKey, c.secretKey)
	if err != nil {
		return nil, 0, err
	}
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Date", date)
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range fcHeaders {
		req.Header.Set(k, v)
	}
	if len(body) > 0 {
		req.Header.Set("Content-MD5", base64MD5(body))
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return b, resp.StatusCode, nil
}

func base64MD5(data []byte) string {
	sum := md5.Sum(data)
	return base64.StdEncoding.EncodeToString(sum[:])
}

// CreateService creates an FC service.
func (c *fcClient) CreateService(ctx context.Context, serviceName string) (int, error) {
	path := "/" + fcAPIVersion + "/services"
	body, _ := json.Marshal(map[string]any{"serviceName": serviceName})
	b, code, err := c.doSigned(ctx, http.MethodPost, path, body, nil)
	if err != nil {
		return 0, err
	}
	if code != http.StatusOK && code != http.StatusCreated && code != 201 {
		return code, fmt.Errorf("CreateService: %s", string(b))
	}
	return code, nil
}

// CreateFunction creates a function with optional inline zip (base64).
func (c *fcClient) CreateFunction(ctx context.Context, serviceName, functionName, handler, runtime string, memory, timeout int, zipBase64 string) (int, error) {
	path := "/" + fcAPIVersion + "/services/" + serviceName + "/functions"
	payload := map[string]any{
		"functionName": functionName,
		"handler":      handler,
		"runtime":      runtime,
		"memorySize":   memory,
		"timeout":      timeout,
	}
	if zipBase64 != "" {
		payload["code"] = map[string]string{"zipFile": zipBase64}
	}
	body, _ := json.Marshal(payload)
	b, code, err := c.doSigned(ctx, http.MethodPost, path, body, nil)
	if err != nil {
		return 0, err
	}
	if code != http.StatusOK && code != http.StatusCreated && code != 201 {
		return code, fmt.Errorf("CreateFunction: %s", string(b))
	}
	return code, nil
}

// CreateTrigger creates a trigger (e.g. http, timer, mns_topic, oss).
func (c *fcClient) CreateTrigger(ctx context.Context, serviceName, functionName, triggerName, triggerType string, triggerConfig map[string]any) (int, error) {
	path := "/" + fcAPIVersion + "/services/" + serviceName + "/functions/" + functionName + "/triggers"
	payload := map[string]any{
		"triggerName":   triggerName,
		"triggerType":   triggerType,
		"triggerConfig": triggerConfig,
	}
	body, _ := json.Marshal(payload)
	b, code, err := c.doSigned(ctx, http.MethodPost, path, body, nil)
	if err != nil {
		return 0, err
	}
	if code != http.StatusOK && code != http.StatusCreated && code != 201 {
		return code, fmt.Errorf("CreateTrigger: %s", string(b))
	}
	return code, nil
}

// DeleteFunction deletes a function.
func (c *fcClient) DeleteFunction(ctx context.Context, serviceName, functionName string) (int, error) {
	path := "/" + fcAPIVersion + "/services/" + serviceName + "/functions/" + functionName
	b, code, err := c.doSigned(ctx, http.MethodDelete, path, nil, nil)
	if err != nil {
		return 0, err
	}
	if code != http.StatusOK && code != http.StatusNoContent && code != 204 && code != 200 {
		return code, fmt.Errorf("DeleteFunction: %s", string(b))
	}
	return code, nil
}

// DeleteService deletes a service.
func (c *fcClient) DeleteService(ctx context.Context, serviceName string) (int, error) {
	path := "/" + fcAPIVersion + "/services/" + serviceName
	b, code, err := c.doSigned(ctx, http.MethodDelete, path, nil, nil)
	if err != nil {
		return 0, err
	}
	if code != http.StatusOK && code != http.StatusNoContent && code != 204 && code != 200 {
		return code, fmt.Errorf("DeleteService: %s", string(b))
	}
	return code, nil
}

// GetFunctionURL returns the HTTP trigger URL for a function (if configured).
func (c *fcClient) GetFunctionURL(ctx context.Context, serviceName, functionName string) (string, error) {
	path := "/" + fcAPIVersion + "/services/" + serviceName + "/functions/" + functionName + "/triggers"
	b, code, err := c.doSigned(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return "", err
	}
	if code != http.StatusOK {
		return "", nil
	}
	var out struct {
		Triggers []struct {
			TriggerConfig struct {
				Methods []string `json:"methods"`
				AuthType string `json:"authType"`
			} `json:"triggerConfig"`
		} `json:"triggers"`
	}
	if json.Unmarshal(b, &out) != nil {
		return "", nil
	}
	// FC HTTP trigger URL format: https://{account}.{region}.fc.aliyuncs.com/2021-04-06/proxy/{service}/{function}/
	return fmt.Sprintf("%s/%s/proxy/%s/%s/", c.baseURL(), fcAPIVersion, serviceName, functionName), nil
}

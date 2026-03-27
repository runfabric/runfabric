package routers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const cloudflareBaseURL = "https://api.cloudflare.com/client/v4"

type cloudflareClient struct {
	token      string
	zoneID     string
	accountID  string
	baseURL    string
	httpClient *http.Client
}

func (c *cloudflareClient) apiBase() string {
	if c.baseURL != "" {
		return c.baseURL
	}
	return cloudflareBaseURL
}

func (c *cloudflareClient) do(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.apiBase()+path, bodyReader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	cl := c.httpClient
	if cl == nil {
		cl = http.DefaultClient
	}
	resp, err := cl.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return data, resp.StatusCode, nil
}

func (c *cloudflareClient) listDNSRecords(ctx context.Context, name, recordType string) ([]cloudflareDNSRecord, error) {
	q := url.Values{}
	q.Set("name", name)
	if recordType != "" {
		q.Set("type", recordType)
	}
	data, status, err := c.do(ctx, http.MethodGet, "/zones/"+c.zoneID+"/dns_records?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var resp cloudflareResponse[[]cloudflareDNSRecord]
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse dns records (status %d): %w", status, err)
	}
	if !resp.Success {
		return nil, cloudflareErrors(resp.Errors)
	}
	return resp.Result, nil
}

func (c *cloudflareClient) createDNSRecord(ctx context.Context, rec cloudflareDNSRecord) (*cloudflareDNSRecord, error) {
	data, status, err := c.do(ctx, http.MethodPost, "/zones/"+c.zoneID+"/dns_records", rec)
	if err != nil {
		return nil, err
	}
	var resp cloudflareResponse[cloudflareDNSRecord]
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse create dns record (status %d): %w", status, err)
	}
	if !resp.Success {
		return nil, cloudflareErrors(resp.Errors)
	}
	return &resp.Result, nil
}

func (c *cloudflareClient) updateDNSRecord(ctx context.Context, id string, rec cloudflareDNSRecord) (*cloudflareDNSRecord, error) {
	data, status, err := c.do(ctx, http.MethodPatch, "/zones/"+c.zoneID+"/dns_records/"+id, rec)
	if err != nil {
		return nil, err
	}
	var resp cloudflareResponse[cloudflareDNSRecord]
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse update dns record (status %d): %w", status, err)
	}
	if !resp.Success {
		return nil, cloudflareErrors(resp.Errors)
	}
	return &resp.Result, nil
}

func (c *cloudflareClient) listMonitors(ctx context.Context) ([]cloudflareLBMonitor, error) {
	data, status, err := c.do(ctx, http.MethodGet, "/accounts/"+c.accountID+"/load_balancing/monitors", nil)
	if err != nil {
		return nil, err
	}
	var resp cloudflareResponse[[]cloudflareLBMonitor]
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse monitors (status %d): %w", status, err)
	}
	if !resp.Success {
		return nil, cloudflareErrors(resp.Errors)
	}
	return resp.Result, nil
}

func (c *cloudflareClient) createMonitor(ctx context.Context, mon cloudflareLBMonitor) (*cloudflareLBMonitor, error) {
	data, status, err := c.do(ctx, http.MethodPost, "/accounts/"+c.accountID+"/load_balancing/monitors", mon)
	if err != nil {
		return nil, err
	}
	var resp cloudflareResponse[cloudflareLBMonitor]
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse create monitor (status %d): %w", status, err)
	}
	if !resp.Success {
		return nil, cloudflareErrors(resp.Errors)
	}
	return &resp.Result, nil
}

func (c *cloudflareClient) updateMonitor(ctx context.Context, id string, mon cloudflareLBMonitor) (*cloudflareLBMonitor, error) {
	data, status, err := c.do(ctx, http.MethodPut, "/accounts/"+c.accountID+"/load_balancing/monitors/"+id, mon)
	if err != nil {
		return nil, err
	}
	var resp cloudflareResponse[cloudflareLBMonitor]
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse update monitor (status %d): %w", status, err)
	}
	if !resp.Success {
		return nil, cloudflareErrors(resp.Errors)
	}
	return &resp.Result, nil
}

func (c *cloudflareClient) listPools(ctx context.Context) ([]cloudflareLBPool, error) {
	data, status, err := c.do(ctx, http.MethodGet, "/accounts/"+c.accountID+"/load_balancing/pools", nil)
	if err != nil {
		return nil, err
	}
	var resp cloudflareResponse[[]cloudflareLBPool]
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse pools (status %d): %w", status, err)
	}
	if !resp.Success {
		return nil, cloudflareErrors(resp.Errors)
	}
	return resp.Result, nil
}

func (c *cloudflareClient) createPool(ctx context.Context, pool cloudflareLBPool) (*cloudflareLBPool, error) {
	data, status, err := c.do(ctx, http.MethodPost, "/accounts/"+c.accountID+"/load_balancing/pools", pool)
	if err != nil {
		return nil, err
	}
	var resp cloudflareResponse[cloudflareLBPool]
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse create pool (status %d): %w", status, err)
	}
	if !resp.Success {
		return nil, cloudflareErrors(resp.Errors)
	}
	return &resp.Result, nil
}

func (c *cloudflareClient) updatePool(ctx context.Context, id string, pool cloudflareLBPool) (*cloudflareLBPool, error) {
	data, status, err := c.do(ctx, http.MethodPut, "/accounts/"+c.accountID+"/load_balancing/pools/"+id, pool)
	if err != nil {
		return nil, err
	}
	var resp cloudflareResponse[cloudflareLBPool]
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse update pool (status %d): %w", status, err)
	}
	if !resp.Success {
		return nil, cloudflareErrors(resp.Errors)
	}
	return &resp.Result, nil
}

func (c *cloudflareClient) listLoadBalancers(ctx context.Context) ([]cloudflareLoadBalancer, error) {
	data, status, err := c.do(ctx, http.MethodGet, "/zones/"+c.zoneID+"/load_balancers", nil)
	if err != nil {
		return nil, err
	}
	var resp cloudflareResponse[[]cloudflareLoadBalancer]
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse load balancers (status %d): %w", status, err)
	}
	if !resp.Success {
		return nil, cloudflareErrors(resp.Errors)
	}
	return resp.Result, nil
}

func (c *cloudflareClient) createLoadBalancer(ctx context.Context, lb cloudflareLoadBalancer) (*cloudflareLoadBalancer, error) {
	data, status, err := c.do(ctx, http.MethodPost, "/zones/"+c.zoneID+"/load_balancers", lb)
	if err != nil {
		return nil, err
	}
	var resp cloudflareResponse[cloudflareLoadBalancer]
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse create lb (status %d): %w", status, err)
	}
	if !resp.Success {
		return nil, cloudflareErrors(resp.Errors)
	}
	return &resp.Result, nil
}

func (c *cloudflareClient) updateLoadBalancer(ctx context.Context, id string, lb cloudflareLoadBalancer) (*cloudflareLoadBalancer, error) {
	data, status, err := c.do(ctx, http.MethodPut, "/zones/"+c.zoneID+"/load_balancers/"+id, lb)
	if err != nil {
		return nil, err
	}
	var resp cloudflareResponse[cloudflareLoadBalancer]
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse update lb (status %d): %w", status, err)
	}
	if !resp.Success {
		return nil, cloudflareErrors(resp.Errors)
	}
	return &resp.Result, nil
}

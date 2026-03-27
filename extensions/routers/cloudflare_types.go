package routers

import (
	"fmt"
	"strings"
)

// cloudflareResponse is the Cloudflare API response envelope.
type cloudflareResponse[T any] struct {
	Result  T               `json:"result"`
	Success bool            `json:"success"`
	Errors  []cloudflareErr `json:"errors"`
}

type cloudflareErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func cloudflareErrors(errs []cloudflareErr) error {
	if len(errs) == 0 {
		return fmt.Errorf("cloudflare API error (no details)")
	}
	msgs := make([]string, len(errs))
	for i, e := range errs {
		msgs[i] = fmt.Sprintf("[%d] %s", e.Code, e.Message)
	}
	return fmt.Errorf("cloudflare API errors: %s", strings.Join(msgs, "; "))
}

type cloudflareDNSRecord struct {
	ID      string `json:"id,omitempty"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
	Comment string `json:"comment,omitempty"`
}

type cloudflareLBOrigin struct {
	Name    string  `json:"name"`
	Address string  `json:"address"`
	Enabled bool    `json:"enabled"`
	Weight  float64 `json:"weight"`
}

type cloudflareLBPool struct {
	ID          string               `json:"id,omitempty"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Origins     []cloudflareLBOrigin `json:"origins"`
	Monitor     string               `json:"monitor,omitempty"`
	Enabled     bool                 `json:"enabled"`
}

type cloudflareLBMonitor struct {
	ID          string `json:"id,omitempty"`
	Type        string `json:"type"`
	Path        string `json:"path"`
	Description string `json:"description"`
	Interval    int    `json:"interval"`
	Timeout     int    `json:"timeout"`
	Retries     int    `json:"retries"`
}

type cloudflareLoadBalancer struct {
	ID             string   `json:"id,omitempty"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	FallbackPool   string   `json:"fallback_pool"`
	DefaultPools   []string `json:"default_pools"`
	SteeringPolicy string   `json:"steering_policy"`
	TTL            int      `json:"ttl"`
	Proxied        bool     `json:"proxied"`
	Enabled        bool     `json:"enabled"`
}

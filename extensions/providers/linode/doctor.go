package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

type linodeProfile struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}

func (p *plugin) Doctor(ctx context.Context, req sdkprovider.DoctorRequest) (*sdkprovider.DoctorResult, error) {
	service, functions, warnings, err := p.inspectConfig(req.Config)
	if err != nil {
		return nil, err
	}
	token, tokenSource := p.resolveToken(req.Config)
	if token == "" {
		return nil, fmt.Errorf("missing Linode token: set %s or config.token/config.tokenEnv", defaultTokenEnv)
	}
	profile, err := p.fetchProfile(ctx, token)
	if err != nil {
		return nil, err
	}
	checks := []string{
		fmt.Sprintf("authenticated to Linode API as %s", firstNonEmpty(profile.Username, profile.Email)),
		fmt.Sprintf("service: %s", service),
		fmt.Sprintf("functions discovered: %d", len(functions)),
		fmt.Sprintf("token source: %s", tokenSource),
	}
	for _, op := range []string{"deploy", "remove", "invoke", "logs"} {
		cmd := strings.TrimSpace(p.resolveCommand(req.Config, op))
		if cmd == "" {
			checks = append(checks, fmt.Sprintf("%s command not configured", op))
			continue
		}
		checks = append(checks, fmt.Sprintf("%s command configured", op))
	}
	checks = append(checks, warnings...)
	return &sdkprovider.DoctorResult{Provider: p.provider, Checks: checks}, nil
}

func (p *plugin) fetchProfile(ctx context.Context, token string) (*linodeProfile, error) {
	url := strings.TrimSuffix(p.apiBaseURL, "/") + "/profile"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call Linode profile API: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Linode profile API returned %s: %s", resp.Status, parseAPIError(body))
	}
	var profile linodeProfile
	if err := json.Unmarshal(body, &profile); err != nil {
		return nil, fmt.Errorf("decode Linode profile response: %w", err)
	}
	return &profile, nil
}

func parseAPIError(body []byte) string {
	var payload struct {
		Errors []struct {
			Reason string `json:"reason"`
		} `json:"errors"`
	}
	if json.Unmarshal(body, &payload) == nil && len(payload.Errors) > 0 {
		parts := make([]string, 0, len(payload.Errors))
		for _, err := range payload.Errors {
			if strings.TrimSpace(err.Reason) != "" {
				parts = append(parts, strings.TrimSpace(err.Reason))
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "; ")
		}
	}
	return strings.TrimSpace(string(body))
}

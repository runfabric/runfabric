package gcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/runfabric/runfabric/platform/deploy/apiutil"
	coredevstream "github.com/runfabric/runfabric/platform/model/devstream"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

var gcpFunctionsAPI = "https://cloudfunctions.googleapis.com/v2"

// DevStreamState holds state for redirecting Cloud Functions to a tunnel and restoring on exit.
type DevStreamState struct {
	FunctionResource  string
	FunctionName      string
	OriginalUpdate    map[string]string // Stores original function environment for restore
	GatewaySetURL     string
	GatewayRestoreURL string
	GatewayApplied    bool
	Applied           bool
	EffectiveMode     coredevstream.Mode
	MissingPrereqs    []string
	StatusMessage     string
}

// RedirectToTunnel finds the Cloud Function for the service/stage and redirects it to the tunnel.
// For GCP Cloud Functions v2, direct routing override like AWS API Gateway is not available.
// This stub returns a no-op state for now. Future implementation would require one of:
// 1. Cloud Load Balancer with custom routing rules
// 2. Cloud Run Traffic Manager
// 3. API Gateway integration
func RedirectToTunnel(ctx context.Context, cfg sdkprovider.Config, stage, tunnelURL string) (*DevStreamState, error) {
	if cfg == nil || stage == "" || tunnelURL == "" {
		return nil, fmt.Errorf("config, stage, and tunnel URL required")
	}
	token := apiutil.Env("GCP_ACCESS_TOKEN")
	project := apiutil.Env("GCP_PROJECT")
	if project == "" {
		project = apiutil.Env("GCP_PROJECT_ID")
	}
	region := providerRegion(cfg)
	if region == "" {
		region = "us-central1"
	}

	funcName := primaryFunctionName(cfg, stage)
	resource := fmt.Sprintf("projects/%s/locations/%s/functions/%s", project, region, funcName)

	state := &DevStreamState{
		FunctionResource:  resource,
		FunctionName:      funcName,
		OriginalUpdate:    make(map[string]string),
		GatewaySetURL:     strings.TrimSpace(apiutil.Env("GCP_DEV_STREAM_GATEWAY_SET_URL")),
		GatewayRestoreURL: strings.TrimSpace(apiutil.Env("GCP_DEV_STREAM_GATEWAY_RESTORE_URL")),
	}
	status := coredevstream.EvaluateProvider("gcp-functions")
	state.EffectiveMode = status.EffectiveMode
	state.MissingPrereqs = append([]string(nil), status.MissingPrereqs...)
	state.StatusMessage = status.Message

	if state.GatewaySetURL != "" && state.GatewayRestoreURL != "" {
		payload := map[string]string{
			"service":          configService(cfg),
			"stage":            stage,
			"project":          project,
			"region":           region,
			"function":         funcName,
			"functionResource": resource,
			"tunnelUrl":        tunnelURL,
		}
		if err := apiutil.APIPost(ctx, state.GatewaySetURL, "GCP_ACCESS_TOKEN", payload, nil); err != nil {
			state.EffectiveMode = coredevstream.ModeLifecycleOnly
			state.StatusMessage = fmt.Sprintf("provider-side mutation skipped: gateway rewrite hook failed: %v", err)
			return state, nil
		}
		state.GatewayApplied = true
		state.Applied = true
		state.EffectiveMode = coredevstream.ModeRouteRewrite
		state.StatusMessage = "gateway-owned route rewrite applied via GCP dev-stream gateway hook; gateway route state will be restored on exit"
		return state, nil
	}

	// Missing prerequisites => lifecycle hook only (no cloud mutation).
	if len(state.MissingPrereqs) > 0 || token == "" || project == "" {
		return state, nil
	}

	getURL := gcpFunctionsAPI + "/" + resource
	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, getURL, nil)
	if err != nil {
		state.EffectiveMode = coredevstream.ModeLifecycleOnly
		state.StatusMessage = fmt.Sprintf("provider-side mutation skipped: could not build function lookup request: %v", err)
		return state, nil
	}
	getReq.Header.Set("Authorization", "Bearer "+token)
	getResp, err := apiutil.DefaultClient.Do(getReq)
	if err != nil {
		state.EffectiveMode = coredevstream.ModeLifecycleOnly
		state.StatusMessage = fmt.Sprintf("provider-side mutation skipped: function lookup failed: %v", err)
		return state, nil
	}
	defer getResp.Body.Close()
	if getResp.StatusCode < 200 || getResp.StatusCode >= 300 {
		state.EffectiveMode = coredevstream.ModeLifecycleOnly
		state.StatusMessage = fmt.Sprintf("provider-side mutation skipped: function lookup returned %s", getResp.Status)
		return state, nil
	}
	body, _ := io.ReadAll(getResp.Body)
	var functionPayload struct {
		ServiceConfig struct {
			EnvironmentVariables map[string]string `json:"environmentVariables"`
		} `json:"serviceConfig"`
	}
	if err := json.Unmarshal(body, &functionPayload); err != nil {
		state.EffectiveMode = coredevstream.ModeLifecycleOnly
		state.StatusMessage = fmt.Sprintf("provider-side mutation skipped: could not decode function metadata: %v", err)
		return state, nil
	}
	for k, v := range functionPayload.ServiceConfig.EnvironmentVariables {
		state.OriginalUpdate[k] = v
	}

	updatedEnv := map[string]string{}
	for k, v := range functionPayload.ServiceConfig.EnvironmentVariables {
		updatedEnv[k] = v
	}
	updatedEnv["RUNFABRIC_TUNNEL_URL"] = tunnelURL
	updatedEnv["RUNFABRIC_DEV_STREAM_STAGE"] = stage

	patchBody := map[string]any{
		"serviceConfig": map[string]any{
			"environmentVariables": updatedEnv,
		},
	}
	patchBytes, err := json.Marshal(patchBody)
	if err != nil {
		state.EffectiveMode = coredevstream.ModeLifecycleOnly
		state.StatusMessage = fmt.Sprintf("provider-side mutation skipped: could not encode patch request: %v", err)
		return state, nil
	}
	patchURL := gcpFunctionsAPI + "/" + resource + "?updateMask=serviceConfig.environmentVariables"
	patchReq, err := http.NewRequestWithContext(ctx, http.MethodPatch, patchURL, strings.NewReader(string(patchBytes)))
	if err != nil {
		state.EffectiveMode = coredevstream.ModeLifecycleOnly
		state.StatusMessage = fmt.Sprintf("provider-side mutation skipped: could not build patch request: %v", err)
		return state, nil
	}
	patchReq.Header.Set("Authorization", "Bearer "+token)
	patchReq.Header.Set("Content-Type", "application/json")
	patchResp, err := apiutil.DefaultClient.Do(patchReq)
	if err != nil {
		state.EffectiveMode = coredevstream.ModeLifecycleOnly
		state.StatusMessage = fmt.Sprintf("provider-side mutation skipped: patch request failed: %v", err)
		return state, nil
	}
	_, _ = io.ReadAll(patchResp.Body)
	patchResp.Body.Close()
	if patchResp.StatusCode < 200 || patchResp.StatusCode >= 300 {
		state.EffectiveMode = coredevstream.ModeLifecycleOnly
		state.StatusMessage = fmt.Sprintf("provider-side mutation skipped: patch request returned %s", patchResp.Status)
		return state, nil
	}
	state.Applied = true
	state.EffectiveMode = coredevstream.ModeConditionalMutation
	state.StatusMessage = "provider-side mutation applied by patching Cloud Functions environment variables for the selected function; this is not a full route rewrite, so provider routing may still require gateway-level configuration"

	return state, nil
}

// Restore reverts the function to its original configuration.
func (s *DevStreamState) Restore(ctx context.Context, region string) error {
	_ = region
	if s == nil || s.FunctionName == "" {
		return nil
	}
	if s.GatewayApplied {
		payload := map[string]string{
			"function":         s.FunctionName,
			"functionResource": s.FunctionResource,
		}
		if err := apiutil.APIPost(ctx, s.GatewayRestoreURL, "GCP_ACCESS_TOKEN", payload, nil); err != nil {
			return fmt.Errorf("gcp gateway dev-stream restore failed: %w", err)
		}
		s.Applied = false
		s.GatewayApplied = false
		return nil
	}
	if !s.Applied || s.FunctionResource == "" {
		return nil
	}
	token := apiutil.Env("GCP_ACCESS_TOKEN")
	if token == "" {
		return nil
	}
	restoreBody := map[string]any{
		"serviceConfig": map[string]any{
			"environmentVariables": s.OriginalUpdate,
		},
	}
	b, err := json.Marshal(restoreBody)
	if err != nil {
		return err
	}
	patchURL := gcpFunctionsAPI + "/" + s.FunctionResource + "?updateMask=serviceConfig.environmentVariables"
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, patchURL, strings.NewReader(string(b)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := apiutil.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	_, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("gcp dev-stream restore patch failed with status " + resp.Status)
	}
	s.Applied = false
	return nil
}

func primaryFunctionName(cfg sdkprovider.Config, stage string) string {
	fnNames := configFunctionNames(cfg)
	if len(fnNames) > 0 {
		sort.Strings(fnNames)
		return fmt.Sprintf("%s-%s-%s", configService(cfg), stage, fnNames[0])
	}
	if cfg != nil {
		return fmt.Sprintf("%s-%s", configService(cfg), stage)
	}
	return stage
}

func configService(cfg sdkprovider.Config) string {
	if cfg == nil {
		return ""
	}
	service, _ := cfg["service"].(string)
	return strings.TrimSpace(service)
}

func providerRegion(cfg sdkprovider.Config) string {
	if cfg == nil {
		return ""
	}
	providerValue, ok := cfg["provider"]
	if !ok || providerValue == nil {
		return ""
	}
	providerMap, ok := providerValue.(map[string]any)
	if !ok {
		return ""
	}
	region, _ := providerMap["region"].(string)
	return strings.TrimSpace(region)
}

func configFunctionNames(cfg sdkprovider.Config) []string {
	if cfg == nil {
		return nil
	}
	functionsValue, ok := cfg["functions"]
	if !ok || functionsValue == nil {
		return nil
	}
	functionsMap, ok := functionsValue.(map[string]any)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(functionsMap))
	for name := range functionsMap {
		names = append(names, name)
	}
	return names
}

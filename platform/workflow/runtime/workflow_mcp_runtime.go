package runtime

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
)

// MCPClient is the runtime MCP client contract used by MCPRuntime and typed step handlers.
type MCPClient interface {
	CallTool(ctx context.Context, server, name string, args map[string]any) (map[string]any, error)
	ReadResource(ctx context.Context, server, uri string) (map[string]any, error)
	GetPrompt(ctx context.Context, server, ref string, args map[string]any) (map[string]any, error)
}

// MCPBinding is the parsed runtime MCP binding from a workflow step input.
type MCPBinding struct {
	Server     string
	Tool       string
	ToolArgs   map[string]any
	Resource   string
	Prompt     string
	PromptArgs map[string]any
}

// MCPPolicyError reports policy enforcement failures with explicit context.
type MCPPolicyError struct {
	Action string
	Server string
	Target string
	Reason string
}

func (e *MCPPolicyError) Error() string {
	return fmt.Sprintf("mcp %s %q on server %q denied: %s", e.Action, e.Target, e.Server, e.Reason)
}

// MCPRuntime encapsulates MCP binding parsing, policy checks, and call dispatch.
type MCPRuntime struct {
	Client       MCPClient
	Integrations config.MCPIntegrationsConfig
	Policy       config.MCPPolicyConfig
	// Provider names the active cloud provider (aws, gcp, azure) for provider-policy enforcement.
	Provider string
	// ActiveRegion is the provider region from which MCP calls originate.
	ActiveRegion string
}

func NewMCPRuntime(client MCPClient, integrations config.MCPIntegrationsConfig, policy config.MCPPolicyConfig) *MCPRuntime {
	if client == nil {
		client = NoopMCPClient{}
	}
	return &MCPRuntime{Client: client, Integrations: integrations, Policy: policy}
}

func ParseMCPBinding(input map[string]any) (MCPBinding, bool) {
	if input == nil {
		return MCPBinding{}, false
	}
	raw, ok := input["mcp"]
	if !ok || raw == nil {
		return MCPBinding{}, false
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return MCPBinding{}, false
	}
	b := MCPBinding{
		Server:   asInputString(obj, "server"),
		Tool:     asInputString(obj, "tool"),
		Resource: asInputString(obj, "resource"),
		Prompt:   asInputString(obj, "prompt"),
	}
	if args, ok := obj["toolArgs"].(map[string]any); ok {
		b.ToolArgs = args
	}
	if args, ok := obj["promptArgs"].(map[string]any); ok {
		b.PromptArgs = args
	}
	return b, true
}

// CallTool enforces policy, performs the MCP tool call (retrying the client call
// per retry; nil means no retry), and records the policy decision and call
// correlation exactly once — retries do not duplicate the audit metadata.
func (r *MCPRuntime) CallTool(ctx context.Context, run *state.WorkflowRun, step state.WorkflowStepRun, b MCPBinding, metadata map[string]any, retry RetryStrategy) (map[string]any, error) {
	if err := r.ensureAllowed("tool", b.Server, b.Tool, metadata); err != nil {
		return nil, err
	}
	result, err := callWithRetry(ctx, retry, func() (map[string]any, error) {
		return r.Client.CallTool(ctx, b.Server, b.Tool, b.ToolArgs)
	})
	if err != nil {
		return nil, err
	}
	if err := appendMCPCorrelation(metadata, "tool", b.Server, b.Tool, run, step); err != nil {
		return nil, err
	}
	return result, nil
}

// ReadResource enforces policy and reads the MCP resource; see CallTool for
// retry and audit semantics.
func (r *MCPRuntime) ReadResource(ctx context.Context, run *state.WorkflowRun, step state.WorkflowStepRun, b MCPBinding, metadata map[string]any, retry RetryStrategy) (map[string]any, error) {
	if err := r.ensureAllowed("resource", b.Server, b.Resource, metadata); err != nil {
		return nil, err
	}
	result, err := callWithRetry(ctx, retry, func() (map[string]any, error) {
		return r.Client.ReadResource(ctx, b.Server, b.Resource)
	})
	if err != nil {
		return nil, err
	}
	if err := appendMCPCorrelation(metadata, "resource", b.Server, b.Resource, run, step); err != nil {
		return nil, err
	}
	return result, nil
}

// GetPrompt enforces policy and fetches the MCP prompt; see CallTool for retry
// and audit semantics.
func (r *MCPRuntime) GetPrompt(ctx context.Context, run *state.WorkflowRun, step state.WorkflowStepRun, b MCPBinding, metadata map[string]any, retry RetryStrategy) (map[string]any, error) {
	if err := r.ensureAllowed("prompt", b.Server, b.Prompt, metadata); err != nil {
		return nil, err
	}
	result, err := callWithRetry(ctx, retry, func() (map[string]any, error) {
		return r.Client.GetPrompt(ctx, b.Server, b.Prompt, b.PromptArgs)
	})
	if err != nil {
		return nil, err
	}
	if err := appendMCPCorrelation(metadata, "prompt", b.Server, b.Prompt, run, step); err != nil {
		return nil, err
	}
	return result, nil
}

// maxRetryAttempts is an absolute ceiling independent of the RetryStrategy, so a
// misbehaving strategy (ShouldRetry always true) cannot loop forever.
const maxRetryAttempts = 50

// callWithRetry runs fn, retrying per the strategy until it succeeds or the
// strategy declines. A nil strategy means a single attempt. The caller records
// policy/correlation once, outside this loop, so a retried call yields a single
// audit entry. The context is honored during backoff so a cancelled or
// timed-out run stops retrying promptly instead of sleeping through it.
func callWithRetry(ctx context.Context, retry RetryStrategy, fn func() (map[string]any, error)) (map[string]any, error) {
	for attempt := 1; ; attempt++ {
		result, err := fn()
		if err == nil || retry == nil || !retry.ShouldRetry(attempt, err) || attempt >= maxRetryAttempts {
			return result, err
		}
		if ctxErr := sleepCtx(ctx, withJitter(retry.Backoff(attempt))); ctxErr != nil {
			return nil, ctxErr
		}
	}
}

// sleepCtx waits for d or until ctx is done, returning ctx.Err() if cancelled.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// withJitter applies full jitter (random in [d/2, d]) to spread retries and
// avoid synchronized thundering-herd retries across many concurrent runs.
func withJitter(d time.Duration) time.Duration {
	if d <= 0 {
		return d
	}
	half := d / 2
	return half + time.Duration(rand.Int63n(int64(half)+1))
}

func (r *MCPRuntime) ensureAllowed(action, server, target string, metadata map[string]any) error {
	server = strings.TrimSpace(server)
	target = strings.TrimSpace(target)
	if server == "" {
		return r.denyDecision(metadata, action, server, target, "server is required")
	}
	if target == "" {
		return r.denyDecision(metadata, action, server, target, "target is required")
	}
	if _, ok := r.Integrations.Servers[server]; !ok {
		return r.denyDecision(metadata, action, server, target, "server is not configured under integrations.mcp.servers")
	}

	compound := server + "." + target
	denyPatterns := ruleSetByAction(r.Policy.Deny, action)
	if pattern, ok := firstMatch(r.Policy.Deny.Servers, server); ok {
		return r.denyDecision(metadata, action, server, target, fmt.Sprintf("matched policies.mcp.deny.servers (%s)", pattern))
	}
	if pattern, ok := firstMatch(denyPatterns, compound); ok {
		return r.denyDecision(metadata, action, server, target, fmt.Sprintf("matched policies.mcp.deny.%ss (%s)", action, pattern))
	}

	// Provider-specific region/auth constraints are deny rules and must be
	// enforced before any allow grant — an explicit allow rule (or default-allow)
	// must not override a provider region restriction.
	if r.Provider != "" && len(r.Policy.Providers) > 0 {
		if pp, ok := r.Policy.Providers[r.Provider]; ok {
			if err := r.enforceProviderPolicy(server, target, action, pp, metadata); err != nil {
				return err
			}
		}
	}

	allowPatterns := ruleSetByAction(r.Policy.Allow, action)
	if pattern, ok := firstMatch(r.Policy.Allow.Servers, server); ok {
		return appendMCPPolicyDecision(metadata, action, server, target, "allowed", fmt.Sprintf("matched policies.mcp.allow.servers (%s)", pattern))
	}
	if pattern, ok := firstMatch(allowPatterns, compound); ok {
		return appendMCPPolicyDecision(metadata, action, server, target, "allowed", fmt.Sprintf("matched policies.mcp.allow.%ss (%s)", action, pattern))
	}
	if r.Policy.DefaultDeny {
		return r.denyDecision(metadata, action, server, target, "denied by policies.mcp.defaultDeny")
	}

	return appendMCPPolicyDecision(metadata, action, server, target, "allowed", "allowed by default (defaultDeny=false)")
}

// denyDecision records a denial in metadata and returns the policy error. If the
// metadata record cannot be appended (corrupted state), that error is returned
// instead so the failure is surfaced rather than silently dropped.
func (r *MCPRuntime) denyDecision(metadata map[string]any, action, server, target, reason string) error {
	if err := appendMCPPolicyDecision(metadata, action, server, target, "denied", reason); err != nil {
		return err
	}
	return &MCPPolicyError{Action: action, Server: server, Target: target, Reason: reason}
}

// enforceProviderPolicy applies per-provider region and auth rules from the MCP policy config.
func (r *MCPRuntime) enforceProviderPolicy(server, target, action string, pp config.MCPProviderPolicyRule, metadata map[string]any) error {
	if pp.DenyCrossRegion && r.ActiveRegion != "" && pp.RequiredRegion != "" && r.ActiveRegion != pp.RequiredRegion {
		return r.denyDecision(metadata, action, server, target, fmt.Sprintf("cross-region call denied (active:%s required:%s)", r.ActiveRegion, pp.RequiredRegion))
	}
	for _, dr := range pp.DenyRegions {
		if r.ActiveRegion != "" && wildcardMatch(dr, r.ActiveRegion) {
			return r.denyDecision(metadata, action, server, target, fmt.Sprintf("region %q is in provider deny list", r.ActiveRegion))
		}
	}
	if pp.RequiredAuth != "" {
		return appendMCPPolicyDecision(metadata, action, server, target, "require", fmt.Sprintf("auth-required:%s", pp.RequiredAuth))
	}
	return nil
}

func appendMCPPolicyDecision(metadata map[string]any, action, server, target, outcome, reason string) error {
	return appendMetadataRecord(metadata, "mcpPolicy", map[string]any{
		"action":  action,
		"server":  server,
		"target":  target,
		"outcome": outcome,
		"reason":  reason,
	})
}

// appendMetadataRecord appends record to the []any stored under key in metadata,
// creating the slice if absent. It returns an error (rather than panicking) when
// the existing value has an unexpected type, e.g. from corrupted or stale
// persisted run state, so a bad invariant fails the step instead of the process.
func appendMetadataRecord(metadata map[string]any, key string, record map[string]any) error {
	raw := metadata[key]
	if raw == nil {
		metadata[key] = []any{record}
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("metadata[%s] has unexpected type %T", key, raw)
	}
	metadata[key] = append(arr, record)
	return nil
}

func firstMatch(patterns []string, value string) (string, bool) {
	for _, p := range patterns {
		if wildcardMatch(p, value) {
			return strings.TrimSpace(p), true
		}
	}
	return "", false
}

func appendMCPCorrelation(metadata map[string]any, typ, server, target string, run *state.WorkflowRun, step state.WorkflowStepRun) error {
	return appendMetadataRecord(metadata, "mcpCalls", map[string]any{
		"type":         typ,
		"server":       server,
		"target":       target,
		"runId":        run.RunID,
		"stepId":       step.StepID,
		"workflowHash": run.WorkflowHash,
	})
}

func ruleSetByAction(set config.MCPPolicyRuleSet, action string) []string {
	switch action {
	case "tool":
		return set.Tools
	case "resource":
		return set.Resources
	case "prompt":
		return set.Prompts
	default:
		return nil
	}
}

// wildcardMatch reports whether value matches pattern, where '*' matches any
// (possibly empty) run of characters anywhere in the pattern. The match is
// anchored at both ends, so a deny pattern like "*-admin" or "db.*.delete"
// matches as intended rather than silently matching nothing (which would let a
// deny rule fail open).
func wildcardMatch(pattern, value string) bool {
	p := strings.TrimSpace(pattern)
	v := strings.TrimSpace(value)
	if p == "" {
		return false
	}
	return globMatch(p, v)
}

// globMatch performs anchored wildcard matching with '*' as the only
// metacharacter, using the classic two-pointer algorithm with backtracking.
func globMatch(pattern, s string) bool {
	var sIdx, pIdx int
	starIdx, sTmp := -1, -1
	for sIdx < len(s) {
		switch {
		case pIdx < len(pattern) && pattern[pIdx] == s[sIdx]:
			sIdx++
			pIdx++
		case pIdx < len(pattern) && pattern[pIdx] == '*':
			starIdx = pIdx
			sTmp = sIdx
			pIdx++
		case starIdx != -1:
			pIdx = starIdx + 1
			sTmp++
			sIdx = sTmp
		default:
			return false
		}
	}
	for pIdx < len(pattern) && pattern[pIdx] == '*' {
		pIdx++
	}
	return pIdx == len(pattern)
}

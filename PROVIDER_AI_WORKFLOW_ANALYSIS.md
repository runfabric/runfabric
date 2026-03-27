# RunFabric Provider & AI Workflow Analysis

**Date:** 26 March 2026  
**Scope:** Provider implementations, orchestration capabilities, and AI workflow execution integration

---

## Executive Summary

RunFabric has **13 provider implementations** across 13 clouds/platforms. **Only 3 providers (AWS, GCP, Azure) support native orchestration**, and **NONE of the providers directly execute AI workflows**. AI workflow execution is centralized in the RunFabric core runtime and is provider-agnostic.

### Key Findings:

- **AI Execution:** Centralized `DefaultAIStepRunner` in `platform/deploy/controlplane/workflow_ai_runtime.go`
- **Orchestration Support:** Only AWS Step Functions, GCP Cloud Workflows, Azure Durable Functions
- **Provider Role:** Providers handle compute/invoke, NOT workflow scheduling or AI step execution
- **Decoupling:** AI steps and orchestration are wired through the local `TypedStepHandler`, not provider adapters

---

## Part 1: All Implemented Providers

### Provider Inventory

| Provider            | Module                               | Capabilities                       | Orchestration        | AI Support |
| ------------------- | ------------------------------------ | ---------------------------------- | -------------------- | ---------- |
| AWS Lambda          | `extensions/providers/aws/`          | remove, invoke, logs, doctor, plan | ✅ Step Functions    | ❌ No      |
| GCP Cloud Functions | `extensions/providers/gcp/`          | (inherit from AWS)                 | ✅ Cloud Workflows   | ❌ No      |
| Azure Functions     | `extensions/providers/azure/`        | (inherit from AWS)                 | ✅ Durable Functions | ❌ No      |
| Alibaba FC          | `extensions/providers/alibaba/`      | (basic compute)                    | ❌ No                | ❌ No      |
| Cloudflare          | `extensions/providers/cloudflare/`   | (basic compute)                    | ❌ No                | ❌ No      |
| DigitalOcean        | `extensions/providers/digitalocean/` | (basic compute)                    | ❌ No                | ❌ No      |
| Fly.io              | `extensions/providers/fly/`          | (basic compute)                    | ❌ No                | ❌ No      |
| IBM OpenWhisk       | `extensions/providers/ibm/`          | (basic compute)                    | ❌ No                | ❌ No      |
| Kubernetes          | `extensions/providers/kubernetes/`   | (basic compute)                    | ❌ No                | ❌ No      |
| Linode              | `extensions/providers/linode/`       | (basic compute)                    | ❌ No                | ❌ No      |
| Netlify             | `extensions/providers/netlify/`      | (basic compute)                    | ❌ No                | ❌ No      |
| Vercel              | `extensions/providers/vercel/`       | (basic compute)                    | ❌ No                | ❌ No      |
| DevStream           | `extensions/providers/devstream/`    | (dev tunnel routing only)          | ❌ No                | ❌ No      |

**Source:** `extensions/providers/*/provider.go`, `extensions/providers/*/capabilities.go`

---

## Part 2: AI Workflow Execution System

### Architecture

AI workflow execution is **NOT** delegated to providers. Instead, it's centralized in the RunFabric core:

```
platform/deploy/controlplane/
├── workflow_ai_runtime.go           ← AIStepRunner interface + DefaultAIStepRunner impl
├── workflow_typed_steps.go          ← TypedStepHandler (coordinates AI + code + human steps)
├── workflow_runtime.go              ← WorkflowRuntime (local orchestration)
├── mcp_runtime.go                   ← MCP integration for AI tool/resource binding
└── ...
```

### AI Step Types Supported

**File:** [platform/deploy/controlplane/workflow_typed_steps.go](platform/deploy/controlplane/workflow_typed_steps.go#L15-L19)

```go
const (
    StepKindCode          = "code"
    StepKindAIRetrieval   = "ai-retrieval"
    StepKindAIGenerate    = "ai-generate"
    StepKindAIStructured  = "ai-structured"
    StepKindAIEval        = "ai-eval"
    StepKindHumanApproval = "human-approval"
)
```

### Default AI Step Runner

**File:** [platform/deploy/controlplane/workflow_ai_runtime.go](platform/deploy/controlplane/workflow_ai_runtime.go#L11-L60)

```go
// AIStepRunner is an explicit boundary for AI step execution concerns.
type AIStepRunner interface {
    ExecuteStep(ctx context.Context, run *state.WorkflowRun, step state.WorkflowStepRun,
                output, metadata map[string]any) (*StepExecutionResult, error)
}

// DefaultAIStepRunner executes AI step kinds and delegates MCP operations.
type DefaultAIStepRunner struct {
    MCPRuntime     *MCPRuntime
    PromptRenderer PromptRenderer
}
```

### AI Step Execution Methods

**File:** [platform/deploy/controlplane/workflow_ai_runtime.go](platform/deploy/controlplane/workflow_ai_runtime.go#L60-L180)

1. **`executeAIRetrieval()`** (lines 76-101)
   - Executes MCP resource read or tool call
   - Returns documents array for retrieval context
   - Does NOT invoke any provider

2. **`executeAIGenerate()`** (lines 104-139)
   - Calls MCP GetPrompt to retrieve prompt templates
   - Calls MCP CallTool for tool results
   - Renders prompt text deterministically
   - Returns generated text as model output

3. **`executeAIStructured()`** (lines 142-159)
   - Schema validation on AI-generated objects
   - Returns validated structured data

4. **`executeAIEval()`** (lines 162-176)
   - Compares score against threshold
   - Returns pass/fail decision

### Step Execution Entry Point

**File:** [platform/deploy/controlplane/workflow_typed_steps.go](platform/deploy/controlplane/workflow_typed_steps.go#L96-L120)

```go
func (h *TypedStepHandler) ExecuteStep(ctx context.Context, run *state.WorkflowRun,
    step state.WorkflowStepRun) (*StepExecutionResult, error) {
    kind := strings.ToLower(strings.TrimSpace(step.Kind))

    switch kind {
    case StepKindCode:
        // Local code execution, no provider involvement
    case StepKindAIRetrieval, StepKindAIGenerate, StepKindAIStructured, StepKindAIEval:
        return h.AIRunner.ExecuteStep(ctx, run, step, output, metadata)  // ← AI Runner
    case StepKindHumanApproval:
        // Human approval flow
    default:
        // Error
    }
}
```

### MCP Integration

**File:** [platform/deploy/controlplane/mcp_runtime.go](platform/deploy/controlplane/mcp_runtime.go)

AI steps integrate with MCP (Model Context Protocol) servers through:

- `MCPRuntime.CallTool()` - invoke remote tools
- `MCPRuntime.ReadResource()` - fetch resources
- `MCPRuntime.GetPrompt()` - retrieve prompt templates

**No provider is involved** in MCP binding or execution.

### Workflow Entry Point

**File:** [platform/workflow/app/workflow.go](platform/workflow/app/workflow.go#L27-L55)

```go
func WorkflowRun(configPath, stage, providerOverride, workflowName, runID string,
    runInput map[string]any) (*WorkflowRunResult, error) {

    // Parse config
    spec, source, warnings, err := buildWorkflowRunSpec(ctx.Config, workflowName, runID, runInput)

    // Create step handler (includes AI runner)
    handler, err := controlplane.NewTypedStepHandlerFromConfig(ctx.Config, nil)

    // Create workflow runtime (does NOT involve providers)
    runtime := controlplane.NewWorkflowRuntime(ctx.RootDir, handler)

    // Execute workflow locally
    run, runErr := runtime.StartRun(context.Background(), spec)

    return &WorkflowRunResult{...}, nil
}
```

### Evidence of Provider Non-Involvement

**Search Results:**

- No provider adapter implements `AIStepRunner` interface
- No provider adapter defines AI step handler methods
- AI step execution imports: `platform/core/state`, `platform/deploy/controlplane` (NO provider imports)
- Provider callbacks in orchestration sync are for **sync state tracking only**, not execution

---

## Part 3: Orchestration Capability Matrix

### Providers with Orchestration Support

#### 1. AWS Lambda + Step Functions

**Files:**

- [extensions/providers/aws/orchestration_capability.go](extensions/providers/aws/orchestration_capability.go)
- [extensions/providers/aws/provider.go](extensions/providers/aws/provider.go#L15-L22)
- [extensions/providers/aws/policy_hooks.go](extensions/providers/aws/policy_hooks.go)

**Orchestration Contract Implementation:**

```go
func (p *Provider) SyncOrchestrations(ctx context.Context, req sdkprovider.OrchestrationSyncRequest)
    (*sdkprovider.OrchestrationSyncResult, error)
func (p *Provider) RemoveOrchestrations(ctx context.Context, req sdkprovider.OrchestrationRemoveRequest)
    (*sdkprovider.OrchestrationSyncResult, error)
func (p *Provider) InvokeOrchestration(ctx context.Context, req sdkprovider.OrchestrationInvokeRequest)
    (*sdkprovider.InvokeResult, error)
func (p *Provider) InspectOrchestrations(ctx context.Context, req sdkprovider.OrchestrationInspectRequest)
    (map[string]any, error)
```

**Role:** Step Functions are used for provider-native orchestration (separate from RunFabric's local AI workflow runtime).

**Config:** `extensions.aws-lambda.stepFunctions` in `runfabric.yml`  
**Invoke Prefix:** `sfn:` or `stepfunction:`

---

#### 2. GCP Cloud Functions + Cloud Workflows

**Files:**

- [extensions/providers/gcp/orchestration_capability.go](extensions/providers/gcp/orchestration_capability.go)
- [extensions/providers/gcp/capabilities.go](extensions/providers/gcp/capabilities.go#L58-L71)
- [extensions/providers/gcp/policy_hooks.go](extensions/providers/gcp/policy_hooks.go)

**Orchestration Contract Implementation:**

```go
func (Runner) SyncOrchestrations(ctx context.Context, req sdkprovider.OrchestrationSyncRequest)
func (Runner) RemoveOrchestrations(ctx context.Context, req sdkprovider.OrchestrationRemoveRequest)
func (Runner) InvokeOrchestration(ctx context.Context, req sdkprovider.OrchestrationInvokeRequest)
func (Runner) InspectOrchestrations(ctx context.Context, req sdkprovider.OrchestrationInspectRequest)
```

**Implementation Details:**

- Uses GCP Workflows API (`https://workflowexecutions.googleapis.com/v1`)
- Creates/updates/deletes Cloud Workflow definitions
- Manages execution tracking
- Returns console links and operation metadata

**Config:** `extensions.gcp-functions.cloudWorkflows` in `runfabric.yml`  
**Invoke Prefix:** `cwf:` or `cloudworkflow:`

---

#### 3. Azure Functions + Durable Functions

**Files:**

- [extensions/providers/azure/orchestration_capability.go](extensions/providers/azure/orchestration_capability.go)
- [extensions/providers/azure/policy_hooks.go](extensions/providers/azure/policy_hooks.go)

**Orchestration Contract Implementation:**

```go
func (Runner) SyncOrchestrations(ctx context.Context, req sdkprovider.OrchestrationSyncRequest)
func (Runner) RemoveOrchestrations(ctx context.Context, req sdkprovider.OrchestrationRemoveRequest)
func (Runner) InvokeOrchestration(ctx context.Context, req sdkprovider.OrchestrationInvokeRequest)
func (Runner) InspectOrchestrations(ctx context.Context, req sdkprovider.OrchestrationInspectRequest)
```

**Implementation Details:**

- Manages Durable Functions orchestration apps
- Syncs orchestrator and activity function definitions
- Manages task hub storage configuration
- Uses Azure management API for lifecycle operations
- Explicit app settings management for orchestration state

**Config:** `extensions.azure-functions.durableFunctions` in `runfabric.yml`  
**Invoke Prefix:** `durable:`

---

### Providers WITHOUT Orchestration Support

**All remaining providers** (Alibaba, Cloudflare, DigitalOcean, Fly, IBM, Kubernetes, Linode, Netlify, Vercel, DevStream) implement **NO orchestration capabilites**:

```go
// Example: Alibaba Provider
func (p *Provider) SyncOrchestrations(ctx context.Context, req sdkprovider.OrchestrationSyncRequest)
    (*sdkprovider.OrchestrationSyncResult, error) {
    // NOT IMPLEMENTED
}
```

**Policy Hook Registration:**
**File:** [platform/extensions/providerpolicy/providers.go](platform/extensions/providerpolicy/providers.go#L40-L100)

Only AWS, GCP, Azure have orchestration hooks registered:

```go
awsHooks := inprocess.APIDispatchHooks{
    ...
    SyncOrchestrations:    awsprovider.SyncOrchestrationsPolicy,
    RemoveOrchestrations:  awsprovider.RemoveOrchestrationsPolicy,
    InvokeOrchestration:   awsprovider.InvokeOrchestrationPolicy,
    InspectOrchestrations: awsprovider.InspectOrchestrationsPolicy,
}

gcpHooks := inprocess.APIDispatchHooks{
    ...
    SyncOrchestrations:    gcpprovider.SyncOrchestrationsPolicy,
    RemoveOrchestrations:  gcpprovider.RemoveOrchestrationsPolicy,
    InvokeOrchestration:   gcpprovider.InvokeOrchestrationPolicy,
    InspectOrchestrations: gcpprovider.InspectOrchestrationsPolicy,
}

azureHooks := inprocess.APIDispatchHooks{
    ...
    SyncOrchestrations:    azureprovider.SyncOrchestrationsPolicy,
    RemoveOrchestrations:  azureprovider.RemoveOrchestrationsPolicy,
    InvokeOrchestration:   azureprovider.InvokeOrchestrationPolicy,
    InspectOrchestrations: azureprovider.InspectOrchestrationsPolicy,
}

alibabaHooks := inprocess.APIDispatchHooks{PrepareDevStream: alibabaprovider.PrepareDevStreamPolicy}
// ↑ NO orchestration hooks for Alibaba
```

---

## Part 4: How Workflows Are Wired

### Orchestration Sync Path

```
CLI: runfabric plan/deploy
  ↓
internal/app/ (app orchestration)
  ↓
internal/deploy/api/ (deploy API boundary)
  ↓
provider.SyncOrchestrations()
  ↓
AWS Step Functions / GCP Cloud Workflows / Azure Durable Functions
  ↓
Provider state metadata returned (console links, operation IDs)
```

**Key Point:** Provider orchestration methods only **sync state**, they do NOT execute AI workflows.

### AI Workflow Execution Path

```
CLI: runfabric workflow run
  ↓
platform/workflow/app/workflow.go: WorkflowRun()
  ↓
platform/deploy/controlplane/workflow_typed_steps.go: TypedStepHandler.ExecuteStep()
  ↓
AI Step: platform/deploy/controlplane/workflow_ai_runtime.go: DefaultAIStepRunner.ExecuteStep()
  ├─ ai-retrieval   → MCPRuntime → (no provider)
  ├─ ai-generate    → MCPRuntime + PromptRenderer → (no provider)
  ├─ ai-structured  → schema validation → (no provider)
  └─ ai-eval        → threshold check → (no provider)
  ↓
step result written to platform/core/state/core/
```

**Key Point:** Providers are NEVER involved in the workflow execution loop.

---

## Part 5: Summary of Findings

### What Providers Support

#### Orchestration-Capable Providers (3)

1. **AWS:** Step Functions (state machine definitions, execution lifecycle)
2. **GCP:** Cloud Workflows (YAML workflow definitions, execution tracking)
3. **Azure:** Durable Functions (orchestrator/activity patterns, task hub management)

#### Capabilities Across All Providers

| Capability              | AWS | GCP | Azure | Others       |
| ----------------------- | --- | --- | ----- | ------------ |
| **Deploy** (functions)  | ✅  | ✅  | ✅    | ✅ (most)    |
| **Remove** (functions)  | ✅  | ✅  | ✅    | ✅ (most)    |
| **Invoke** (functions)  | ✅  | ✅  | ✅    | ✅ (most)    |
| **Logs**                | ✅  | ✅  | ✅    | ✅ (most)    |
| **Doctor**              | ✅  | ✅  | ✅    | ⚠️ (limited) |
| **Plan**                | ✅  | ✅  | ✅    | ⚠️ (limited) |
| **Sync Orchestrations** | ✅  | ✅  | ✅    | ❌           |
| **AI Workflow Exec**    | ❌  | ❌  | ❌    | ❌           |

### What Providers Do NOT Support

#### ❌ AI Workflow Step Execution

- **None** of the 13 providers implement AI step execution
- AI steps (ai-retrieval, ai-generate, ai-structured, ai-eval) are **always** executed by RunFabric core
- Providers are completely decoupled from AI step runtime

#### ❌ Orchestration (10 Providers)

- Alibaba, Cloudflare, DigitalOcean, Fly, IBM, Kubernetes, Linode, Netlify, Vercel, DevStream
- These providers only handle compute invocation, not workflow scheduling

### Architecture Validation

**AI Workflow Isolation:**

- ✅ AI runtime lives in `platform/deploy/controlplane/` (RunFabric core)
- ✅ No provider package imports `workflow_ai_runtime`
- ✅ MCP integration also lives in core (`mcp_runtime.go`)
- ✅ Providers only imported in `internal/app/` for compose orchestration, not step execution

**Orchestration Isolation:**

- ✅ Orchestration methods are provider-specific (AWS Step Functions != GCP Cloud Workflows != Azure Durable)
- ✅ Local RunFabric workflow runtime (`WorkflowRuntime`) is independent of provider orchestration
- ✅ Document from ROADMAP confirms: Phase 4 expands provider orchestration (implies it's separate concern)

---

## Part 6: Implications for Future AI Integration

### Current Design (Correct)

- AI step execution is **cloud-agnostic** (any provider can participate)
- Workflow state stored locally (platform/core/state), not in provider-specific systems
- MCP binding provider-neutral (via config, not hardcoded)

### Recommendations

1. **Don't add AI execution to provider adapters** — keep it centralized
2. **For provider-specific AI optimization**, consider:
   - Custom prompt templating via `PromptRenderer` interface (already exists)
   - MCP server registration for cloud-specific tools (via config)
   - Post-step telemetry hooks (already available)
3. **For cross-provider AI workflows**, extend `TypedStepHandler` with provider hints, not provider implementations

---

## Appendix: File Reference

### Core AI Workflow Files

- [platform/deploy/controlplane/workflow_ai_runtime.go](platform/deploy/controlplane/workflow_ai_runtime.go) - AI step execution
- [platform/deploy/controlplane/workflow_typed_steps.go](platform/deploy/controlplane/workflow_typed_steps.go) - Step kind dispatch
- [platform/deploy/controlplane/workflow_runtime.go](platform/deploy/controlplane/workflow_runtime.go) - Local runtime
- [platform/deploy/controlplane/mcp_runtime.go](platform/deploy/controlplane/mcp_runtime.go) - MCP integration
- [platform/workflow/app/workflow.go](platform/workflow/app/workflow.go) - Workflow entry point

### Provider Orchestration Files

- [extensions/providers/aws/orchestration_capability.go](extensions/providers/aws/orchestration_capability.go)
- [extensions/providers/gcp/orchestration_capability.go](extensions/providers/gcp/orchestration_capability.go)
- [extensions/providers/azure/orchestration_capability.go](extensions/providers/azure/orchestration_capability.go)

### Policy & Dispatch

- [platform/extensions/providerpolicy/providers.go](platform/extensions/providerpolicy/providers.go) - Provider registry
- [platform/extensions/deploy/contracts.go](platform/extensions/deploy/contracts.go) - Core contracts

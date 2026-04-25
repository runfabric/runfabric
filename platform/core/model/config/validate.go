package config

import (
	"fmt"
	"strings"

	"github.com/runfabric/runfabric/platform/core/policy/secrets"
)

func Validate(cfg *Config) error {
	if strings.TrimSpace(cfg.Service) == "" {
		return fmt.Errorf("service is required")
	}
	if strings.TrimSpace(cfg.Provider.Name) == "" {
		return fmt.Errorf("provider.name is required")
	}
	if strings.TrimSpace(cfg.Provider.Runtime) == "" {
		return fmt.Errorf("provider.runtime is required")
	}
	switch source := strings.ToLower(strings.TrimSpace(cfg.Provider.Source)); source {
	case "", "builtin", "external":
		if source != "external" && strings.TrimSpace(cfg.Provider.Version) != "" {
			return fmt.Errorf("provider.version requires provider.source to be external")
		}
	default:
		return fmt.Errorf("provider.source must be builtin or external (got %q)", cfg.Provider.Source)
	}
	if len(cfg.Functions) == 0 {
		return fmt.Errorf("at least one function is required")
	}
	if err := secrets.ValidateConfigSecretMap(cfg.Secrets); err != nil {
		return err
	}

	if cfg.Backend != nil {
		switch cfg.Backend.Kind {
		case "", "local":
			// no extra fields required
		case "s3":
			if strings.TrimSpace(cfg.Backend.S3Bucket) == "" {
				return fmt.Errorf("backend.s3Bucket is required for backend.kind %q", cfg.Backend.Kind)
			}
			// LockTable is optional when using state reference format with lockfile or when backend supports it
		case "gcs":
			if err := validateGCSBackend(cfg); err != nil {
				return err
			}
		case "azblob":
			if err := validateAzblobBackend(cfg); err != nil {
				return err
			}
		case "postgres":
			if strings.TrimSpace(cfg.Backend.PostgresConnectionStringEnv) == "" {
				cfg.Backend.PostgresConnectionStringEnv = "RUNFABRIC_STATE_POSTGRES_URL"
			}
			if strings.TrimSpace(cfg.Backend.PostgresTable) == "" {
				cfg.Backend.PostgresTable = "runfabric_receipts"
			}
		case "sqlite":
			if strings.TrimSpace(cfg.Backend.SqlitePath) == "" {
				cfg.Backend.SqlitePath = ".runfabric/state.db"
			}
		case "dynamodb":
			if strings.TrimSpace(cfg.Backend.LockTable) == "" && strings.TrimSpace(cfg.Backend.ReceiptTable) == "" {
				return fmt.Errorf("backend.lockTable or backend.receiptTable is required for backend.kind dynamodb")
			}
		default:
			return fmt.Errorf("unsupported backend.kind %q (use local, s3, gcs, azblob, postgres, sqlite, or dynamodb)", cfg.Backend.Kind)
		}
	}

	for name, fn := range cfg.Functions {
		if fn.Architecture != "" {
			switch fn.Architecture {
			case "x86_64", "arm64":
			default:
				return fmt.Errorf("functions.%s.architecture must be x86_64 or arm64", name)
			}
		}

		for key := range fn.Environment {
			if strings.TrimSpace(key) == "" {
				return fmt.Errorf("functions.%s.environment contains empty key", name)
			}
		}

		for key := range fn.Secrets {
			if strings.TrimSpace(key) == "" {
				return fmt.Errorf("functions.%s.secrets contains empty key", name)
			}
		}

		for _, ev := range fn.Events {
			if ev.Queue != nil && strings.TrimSpace(ev.Queue.Queue) == "" {
				return fmt.Errorf("functions.%s queue trigger requires queue name", name)
			}
			if ev.Storage != nil && strings.TrimSpace(ev.Storage.Bucket) == "" {
				return fmt.Errorf("functions.%s storage trigger requires bucket", name)
			}
			if ev.PubSub != nil && strings.TrimSpace(ev.PubSub.Topic) == "" {
				return fmt.Errorf("functions.%s pubsub trigger requires topic", name)
			}
			if ev.HTTP == nil {
				continue
			}

			if ev.HTTP.Authorizer != nil {
				switch ev.HTTP.Authorizer.Type {
				case "jwt", "lambda", "iam":
				default:
					return fmt.Errorf("functions.%s http authorizer.type must be jwt, lambda, or iam", name)
				}
				if ev.HTTP.Authorizer.Type == "jwt" {
					if strings.TrimSpace(ev.HTTP.Authorizer.Issuer) == "" {
						return fmt.Errorf("functions.%s jwt authorizer requires issuer", name)
					}
					if len(ev.HTTP.Authorizer.Audience) == 0 {
						return fmt.Errorf("functions.%s jwt authorizer requires audience", name)
					}
				}
				if ev.HTTP.Authorizer.Type == "lambda" && strings.TrimSpace(ev.HTTP.Authorizer.Function) == "" {
					return fmt.Errorf("functions.%s lambda authorizer requires function", name)
				}
			}
		}
	}

	if err := ValidateAddons(cfg); err != nil {
		return err
	}

	if cfg.Deploy != nil {
		s := strings.TrimSpace(strings.ToLower(cfg.Deploy.Strategy))
		if s != "" && s != "all-at-once" && s != "canary" && s != "blue-green" && s != "per-function" {
			return fmt.Errorf("deploy.strategy must be all-at-once, canary, blue-green, or per-function (got %q)", cfg.Deploy.Strategy)
		}
		if s == "canary" {
			if cfg.Deploy.CanaryPercent < 0 || cfg.Deploy.CanaryPercent > 100 {
				return fmt.Errorf("deploy.canaryPercent must be 0-100 when strategy is canary (got %d)", cfg.Deploy.CanaryPercent)
			}
		}
	}
	if err := validateWorkflows(cfg.Workflows); err != nil {
		return err
	}
	if err := validateLayers(cfg.Layers); err != nil {
		return err
	}

	return nil
}

func validateWorkflows(workflows []WorkflowConfig) error {
	for wi, wf := range workflows {
		name := strings.TrimSpace(wf.Name)
		if name == "" {
			return fmt.Errorf("workflows[%d].name is required", wi)
		}
		for si, step := range wf.Steps {
			if strings.TrimSpace(step.ID) == "" {
				return fmt.Errorf("workflows[%d].steps[%d].id is required", wi, si)
			}

			kind := strings.ToLower(strings.TrimSpace(step.Kind))
			if kind == "" {
				return fmt.Errorf("workflows[%d].steps[%d].kind is required", wi, si)
			}
			input := step.Input
			switch kind {
			case "code":
				// No kind-specific required input.
			case "ai-retrieval":
				if strings.TrimSpace(readWorkflowStepString(input, "query")) == "" {
					return fmt.Errorf("workflows[%d].steps[%d] kind ai-retrieval requires input.query", wi, si)
				}
			case "ai-generate":
				if strings.TrimSpace(readWorkflowStepString(input, "prompt")) == "" {
					return fmt.Errorf("workflows[%d].steps[%d] kind ai-generate requires input.prompt", wi, si)
				}
			case "ai-structured":
				schema, _ := input["schema"].(map[string]any)
				if len(schema) == 0 {
					return fmt.Errorf("workflows[%d].steps[%d] kind ai-structured requires input.schema object", wi, si)
				}
			case "ai-eval":
				if _, ok := readWorkflowStepNumber(input, "score"); !ok {
					return fmt.Errorf("workflows[%d].steps[%d] kind ai-eval requires numeric input.score", wi, si)
				}
				if _, exists := input["threshold"]; exists {
					if _, ok := readWorkflowStepNumber(input, "threshold"); !ok {
						return fmt.Errorf("workflows[%d].steps[%d] kind ai-eval requires numeric input.threshold when set", wi, si)
					}
				}
			case "human-approval":
				decision := strings.ToLower(strings.TrimSpace(readWorkflowStepString(input, "approvalDecision")))
				if decision != "" && decision != "approve" && decision != "approved" && decision != "reject" && decision != "rejected" {
					return fmt.Errorf("workflows[%d].steps[%d] human approval input.approvalDecision must be approve/approved/reject/rejected when set", wi, si)
				}
			default:
				return fmt.Errorf("workflows[%d].steps[%d].kind %q is unsupported", wi, si, step.Kind)
			}
		}
	}
	return nil
}

func validateLayers(layers map[string]LayerConfig) error {
	for name, layer := range layers {
		if strings.TrimSpace(layer.Ref) == "" {
			return fmt.Errorf("layers.%s.ref is required", name)
		}
	}
	return nil
}

func readWorkflowStepString(input map[string]any, key string) string {
	if input == nil {
		return ""
	}
	v, _ := input[key].(string)
	return strings.TrimSpace(v)
}

func readWorkflowStepNumber(input map[string]any, key string) (float64, bool) {
	if input == nil {
		return 0, false
	}
	switch n := input[key].(type) {
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

func validateGCSBackend(cfg *Config) error {
	if cfg == nil || cfg.Backend == nil {
		return nil
	}
	bucket := strings.TrimSpace(cfg.Backend.GCSBucket)
	prefix := strings.TrimSpace(cfg.Backend.GCSPrefix)
	if bucket == "" {
		return fmt.Errorf("backend.gcsBucket is required for backend.kind %q", cfg.Backend.Kind)
	}
	if prefix == "" {
		return fmt.Errorf("backend.gcsPrefix is required for backend.kind %q", cfg.Backend.Kind)
	}
	return nil
}

func validateAzblobBackend(cfg *Config) error {
	if cfg == nil || cfg.Backend == nil {
		return nil
	}
	container := strings.TrimSpace(cfg.Backend.AzblobContainer)
	prefix := strings.TrimSpace(cfg.Backend.AzblobPrefix)
	if container == "" {
		return fmt.Errorf("backend.azblobContainer is required for backend.kind %q", cfg.Backend.Kind)
	}
	if prefix == "" {
		return fmt.Errorf("backend.azblobPrefix is required for backend.kind %q", cfg.Backend.Kind)
	}
	return nil
}

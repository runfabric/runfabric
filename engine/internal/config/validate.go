package config

import (
	"fmt"
	"strings"

	"github.com/runfabric/runfabric/engine/internal/aiflow"
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
	if len(cfg.Functions) == 0 {
		return fmt.Errorf("at least one function is required")
	}

	if cfg.Backend != nil {
		switch cfg.Backend.Kind {
		case "", "local":
			// no extra fields required
		case "s3", "aws-remote":
			if strings.TrimSpace(cfg.Backend.S3Bucket) == "" {
				return fmt.Errorf("backend.s3Bucket is required for backend.kind %q", cfg.Backend.Kind)
			}
			// LockTable is optional when using state reference format with lockfile or when backend supports it
		case "gcs", "azblob", "postgres":
			// accepted; backend-specific validation can be added later
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
		if s != "" && s != "all-at-once" && s != "canary" && s != "blue-green" {
			return fmt.Errorf("deploy.strategy must be all-at-once, canary, or blue-green (got %q)", cfg.Deploy.Strategy)
		}
		if s == "canary" {
			if cfg.Deploy.CanaryPercent < 0 || cfg.Deploy.CanaryPercent > 100 {
				return fmt.Errorf("deploy.canaryPercent must be 0-100 when strategy is canary (got %d)", cfg.Deploy.CanaryPercent)
			}
		}
	}

	if err := validateAiWorkflow(cfg.AiWorkflow, aiflow.ValidNodeType); err != nil {
		return err
	}
	return nil
}

// validateAiWorkflow validates the AI workflow graph when present and enabled. nodeTypeOK is the registry check (e.g. aiflow.ValidNodeType).
func validateAiWorkflow(aw *AiWorkflowConfig, nodeTypeOK func(string) bool) error {
	if aw == nil || !aw.Enable {
		return nil
	}
	if len(aw.Nodes) == 0 {
		return fmt.Errorf("aiWorkflow.nodes is required when aiWorkflow.enable is true")
	}
	seen := make(map[string]bool)
	for i, n := range aw.Nodes {
		id := strings.TrimSpace(n.ID)
		if id == "" {
			return fmt.Errorf("aiWorkflow.nodes[%d].id is required", i)
		}
		if seen[id] {
			return fmt.Errorf("aiWorkflow duplicate node id %q", id)
		}
		seen[id] = true
		if !nodeTypeOK(strings.TrimSpace(n.Type)) {
			return fmt.Errorf("aiWorkflow.nodes[%d].type %q is not supported (allowed: trigger, ai, data, logic, system, human)", i, n.Type)
		}
	}
	if aw.Entrypoint != "" {
		ep := strings.TrimSpace(aw.Entrypoint)
		if !seen[ep] {
			return fmt.Errorf("aiWorkflow.entrypoint %q must be a node id", aw.Entrypoint)
		}
	}
	for i, e := range aw.Edges {
		from := strings.TrimSpace(e.From)
		to := strings.TrimSpace(e.To)
		if from == "" {
			return fmt.Errorf("aiWorkflow.edges[%d].from is required", i)
		}
		if to == "" {
			return fmt.Errorf("aiWorkflow.edges[%d].to is required", i)
		}
		if !seen[from] {
			return fmt.Errorf("aiWorkflow.edges[%d].from %q is not a node id", i, e.From)
		}
		if !seen[to] {
			return fmt.Errorf("aiWorkflow.edges[%d].to %q is not a node id", i, e.To)
		}
	}
	return nil
}

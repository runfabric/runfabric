package config

import (
	"fmt"
	"strings"
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
			if strings.TrimSpace(cfg.Backend.LockTable) == "" {
				return fmt.Errorf("backend.lockTable is required for backend.kind %q", cfg.Backend.Kind)
			}
		case "gcs", "azblob", "postgres":
			// accepted; backend-specific validation can be added later
		default:
			return fmt.Errorf("unsupported backend.kind %q (use local, s3, gcs, azblob, or postgres)", cfg.Backend.Kind)
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

	return nil
}

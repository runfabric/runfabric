package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var envPattern = regexp.MustCompile(`\$\{env:([A-Za-z_][A-Za-z0-9_]*)(?:,([^}]+))?\}`)

func Resolve(cfg *Config, stage string) (*Config, error) {
	out := *cfg

	out.Service = resolveEnv(out.Service)
	out.Provider.Name = resolveEnv(out.Provider.Name)
	out.Provider.Runtime = resolveEnv(out.Provider.Runtime)
	out.Provider.Region = resolveEnv(out.Provider.Region)

	if out.Backend != nil {
		b := *out.Backend
		b.Kind = resolveEnv(b.Kind)
		b.S3Bucket = resolveEnv(b.S3Bucket)
		b.S3Prefix = resolveEnv(b.S3Prefix)
		b.LockTable = resolveEnv(b.LockTable)
		out.Backend = &b
	}

	resolvedFunctions := make(map[string]FunctionConfig, len(out.Functions))
	for name, fn := range out.Functions {
		newFn := fn
		newFn.Handler = resolveEnv(fn.Handler)
		newFn.Runtime = resolveEnv(fn.Runtime)
		newFn.Architecture = resolveEnv(fn.Architecture)

		if fn.Environment != nil {
			newFn.Environment = map[string]string{}
			for k, v := range fn.Environment {
				newFn.Environment[k] = resolveEnv(v)
			}
		}

		if fn.Secrets != nil {
			newFn.Secrets = map[string]string{}
			for k, v := range fn.Secrets {
				newFn.Secrets[k] = resolveEnv(v)
			}
		}

		if fn.Tags != nil {
			newFn.Tags = map[string]string{}
			for k, v := range fn.Tags {
				newFn.Tags[k] = resolveEnv(v)
			}
		}

		if len(fn.Layers) > 0 {
			newFn.Layers = make([]string, 0, len(fn.Layers))
			for _, layer := range fn.Layers {
				newFn.Layers = append(newFn.Layers, resolveEnv(layer))
			}
		}

		for i := range newFn.Events {
			if newFn.Events[i].HTTP != nil {
				h := newFn.Events[i].HTTP
				h.Path = resolveEnv(h.Path)
				h.Method = resolveEnv(h.Method)

				if h.Authorizer != nil {
					h.Authorizer.Type = resolveEnv(h.Authorizer.Type)
					h.Authorizer.Name = resolveEnv(h.Authorizer.Name)
					h.Authorizer.Issuer = resolveEnv(h.Authorizer.Issuer)
					h.Authorizer.Function = resolveEnv(h.Authorizer.Function)
					for j := range h.Authorizer.IdentitySources {
						h.Authorizer.IdentitySources[j] = resolveEnv(h.Authorizer.IdentitySources[j])
					}
					for j := range h.Authorizer.Audience {
						h.Authorizer.Audience[j] = resolveEnv(h.Authorizer.Audience[j])
					}
				}
			}
		}

		resolvedFunctions[name] = newFn
	}

	out.Functions = resolvedFunctions

	if stage != "" && out.Stages != nil {
		if stageCfg, ok := out.Stages[stage]; ok {

			if stageCfg.Provider != nil {
				if stageCfg.Provider.Name != "" {
					out.Provider.Name = resolveEnv(stageCfg.Provider.Name)
				}
				if stageCfg.Provider.Runtime != "" {
					out.Provider.Runtime = resolveEnv(stageCfg.Provider.Runtime)
				}
				if stageCfg.Provider.Region != "" {
					out.Provider.Region = resolveEnv(stageCfg.Provider.Region)
				}
			}

			if stageCfg.Backend != nil {
				if out.Backend == nil {
					out.Backend = &BackendConfig{}
				}
				if stageCfg.Backend.Kind != "" {
					out.Backend.Kind = resolveEnv(stageCfg.Backend.Kind)
				}
				if stageCfg.Backend.S3Bucket != "" {
					out.Backend.S3Bucket = resolveEnv(stageCfg.Backend.S3Bucket)
				}
				if stageCfg.Backend.S3Prefix != "" {
					out.Backend.S3Prefix = resolveEnv(stageCfg.Backend.S3Prefix)
				}
				if stageCfg.Backend.LockTable != "" {
					out.Backend.LockTable = resolveEnv(stageCfg.Backend.LockTable)
				}
			}

			if stageCfg.HTTP != nil {
				// merge into a stage-aware runtime representation if you already have one
				// or preserve on out.Stages[stage]
			}
			for fnName, fnOverride := range stageCfg.Functions {
				base := out.Functions[fnName]
				if fnOverride.Handler != "" {
					base.Handler = resolveEnv(fnOverride.Handler)
				}
				if fnOverride.Runtime != "" {
					base.Runtime = resolveEnv(fnOverride.Runtime)
				}
				if len(fnOverride.Events) > 0 {
					base.Events = fnOverride.Events
				}
				out.Functions[fnName] = base
			}
		}
	}

	for name, fn := range out.Functions {
		if fn.Runtime == "" {
			fn.Runtime = out.Provider.Runtime
			out.Functions[name] = fn
		}
	}

	return &out, nil
}

func resolveEnv(input string) string {
	return envPattern.ReplaceAllStringFunc(input, func(match string) string {
		sub := envPattern.FindStringSubmatch(match)
		key := sub[1]
		def := ""
		if len(sub) > 2 {
			def = strings.TrimSpace(sub[2])
		}
		if val, ok := os.LookupEnv(key); ok {
			return val
		}
		return def
	})
}

func EnsureStage(stage string) error {
	if strings.TrimSpace(stage) == "" {
		return fmt.Errorf("stage cannot be empty")
	}
	return nil
}

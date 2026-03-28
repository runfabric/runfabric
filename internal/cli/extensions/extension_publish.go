package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/runfabric/runfabric/platform/extensions/application/external"
	"github.com/spf13/cobra"
)

type publishSession struct {
	PublishID   string                            `json:"publishId"`
	RegistryURL string                            `json:"registryUrl,omitempty"`
	Files       map[string]string                 `json:"files"`
	Uploads     map[string]external.PublishUpload `json:"uploads"`
}

func newExtensionPublishCmd(opts *common.GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish an extension artifact via registry (init/upload/finalize/status)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newExtensionPublishInitCmd(opts),
		newExtensionPublishUploadCmd(opts),
		newExtensionPublishFinalizeCmd(opts),
		newExtensionPublishStatusCmd(opts),
	)
	return cmd
}

func newExtensionPublishInitCmd(opts *common.GlobalOptions) *cobra.Command {
	var version string
	var artifactPath string
	var typ string
	var pluginKind string
	var registry string
	var registryToken string
	cmd := &cobra.Command{
		Use:   "init <id>",
		Short: "Initialize a publish session and get signed upload URLs",
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("usage: runfabric extension publish init <id> --version <v> --artifact <path>")
			}
			if strings.TrimSpace(version) == "" {
				return fmt.Errorf("--version is required")
			}
			if strings.TrimSpace(artifactPath) == "" {
				return fmt.Errorf("--artifact is required")
			}
			id := strings.TrimSpace(args[0])
			reg, tok := applyRegistryConfig(registry, registryToken)

			pf, err := external.BuildPublishFileDescriptor("artifact", artifactPath)
			if err != nil {
				return err
			}
			res, err := external.PublishInit(external.PublishInitOptions{
				RegistryURL: reg,
				AuthToken:   tok,
				ID:          id,
				Version:     version,
				Type:        typ,
				PluginKind:  pluginKind,
				Files:       []external.PublishFile{pf},
				Timeout:     30 * time.Second,
			})
			if err != nil {
				return err
			}
			s := publishSession{
				PublishID:   res.PublishID,
				RegistryURL: reg,
				Files:       map[string]string{"artifact": artifactPath},
				Uploads:     map[string]external.PublishUpload{},
			}
			for _, u := range res.Uploads {
				if strings.TrimSpace(u.Key) == "" {
					u.Key = "artifact"
				}
				s.Uploads[u.Key] = u
			}
			sessionPath, err := savePublishSession(s)
			if err != nil {
				return err
			}
			if opts.JSONOutput {
				out := map[string]any{
					"publishId":   res.PublishID,
					"status":      res.Status,
					"uploads":     res.Uploads,
					"sessionPath": sessionPath,
				}
				enc := json.NewEncoder(c.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			fmt.Fprintf(c.OutOrStdout(), "Publish session initialized: %s\n", res.PublishID)
			fmt.Fprintf(c.OutOrStdout(), "Session: %s\n", sessionPath)
			fmt.Fprintln(c.OutOrStdout(), "Next: run `runfabric extension publish upload --publish-id "+res.PublishID+"`")
			return nil
		},
	}
	cmd.Flags().StringVar(&version, "version", "", "Extension version to publish (required)")
	cmd.Flags().StringVar(&artifactPath, "artifact", "", "Artifact file path to upload (required)")
	cmd.Flags().StringVar(&typ, "type", "plugin", "Extension type: plugin or addon")
	cmd.Flags().StringVar(&pluginKind, "plugin-kind", "provider", "Plugin kind when --type=plugin: provider, runtime, simulator, router")
	cmd.Flags().StringVar(&registry, "registry", "", "Registry base URL (default: https://registry.runfabric.cloud)")
	cmd.Flags().StringVar(&registryToken, "registry-token", "", "Registry bearer token")
	return cmd
}

func newExtensionPublishUploadCmd(opts *common.GlobalOptions) *cobra.Command {
	var publishID string
	var key string
	var artifactPath string
	var uploadURL string
	var method string
	var headers []string
	cmd := &cobra.Command{
		Use:   "upload",
		Short: "Upload staged publish files to signed URLs",
		RunE: func(c *cobra.Command, args []string) error {
			if strings.TrimSpace(uploadURL) != "" {
				if strings.TrimSpace(artifactPath) == "" {
					return fmt.Errorf("--artifact is required when --upload-url is set")
				}
				u := external.PublishUpload{
					Key:     strings.TrimSpace(key),
					Method:  strings.TrimSpace(method),
					URL:     strings.TrimSpace(uploadURL),
					Headers: parseKVHeaders(headers),
				}
				if u.Key == "" {
					u.Key = "artifact"
				}
				if err := external.UploadPublishFile(u, artifactPath, 60*time.Second); err != nil {
					return err
				}
				if opts.JSONOutput {
					_, _ = c.OutOrStdout().Write([]byte(`{"ok":true}` + "\n"))
					return nil
				}
				fmt.Fprintf(c.OutOrStdout(), "Uploaded %s\n", u.Key)
				return nil
			}

			if strings.TrimSpace(publishID) == "" {
				return fmt.Errorf("--publish-id is required")
			}
			s, sessionPath, err := loadPublishSession(publishID)
			if err != nil {
				return err
			}

			keys := make([]string, 0, len(s.Uploads))
			if strings.TrimSpace(key) != "" {
				keys = append(keys, strings.TrimSpace(key))
			} else {
				for k := range s.Uploads {
					keys = append(keys, k)
				}
			}
			for _, k := range keys {
				up, ok := s.Uploads[k]
				if !ok {
					return fmt.Errorf("publish session %s has no upload entry for key %q", s.PublishID, k)
				}
				path := strings.TrimSpace(s.Files[k])
				if path == "" && k == "artifact" {
					path = strings.TrimSpace(artifactPath)
				}
				if path == "" {
					return fmt.Errorf("publish session %s has no local file path for key %q", s.PublishID, k)
				}
				if err := external.UploadPublishFile(up, path, 60*time.Second); err != nil {
					return err
				}
			}

			if opts.JSONOutput {
				out := map[string]any{
					"publishId":   s.PublishID,
					"uploaded":    keys,
					"sessionPath": sessionPath,
				}
				enc := json.NewEncoder(c.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			fmt.Fprintf(c.OutOrStdout(), "Uploaded files for publish session %s\n", s.PublishID)
			return nil
		},
	}
	cmd.Flags().StringVar(&publishID, "publish-id", "", "Publish session ID from publish init")
	cmd.Flags().StringVar(&key, "key", "", "Upload a single file key from the session (default: all)")
	cmd.Flags().StringVar(&artifactPath, "artifact", "", "Override artifact file path (used with --upload-url or artifact key)")
	cmd.Flags().StringVar(&uploadURL, "upload-url", "", "Signed upload URL (direct mode; skips session lookup)")
	cmd.Flags().StringVar(&method, "method", "PUT", "HTTP method for direct upload mode")
	cmd.Flags().StringArrayVar(&headers, "header", nil, "Extra upload header in key:value format (direct mode)")
	return cmd
}

func newExtensionPublishFinalizeCmd(opts *common.GlobalOptions) *cobra.Command {
	var publishID string
	var registry string
	var registryToken string
	cmd := &cobra.Command{
		Use:   "finalize",
		Short: "Finalize a publish session",
		RunE: func(c *cobra.Command, args []string) error {
			if strings.TrimSpace(publishID) == "" {
				return fmt.Errorf("--publish-id is required")
			}
			reg, tok := applyRegistryConfig(registry, registryToken)
			res, err := external.PublishFinalize(external.PublishFinalizeOptions{
				RegistryURL: reg,
				AuthToken:   tok,
				PublishID:   publishID,
				Timeout:     30 * time.Second,
			})
			if err != nil {
				return err
			}
			if opts.JSONOutput {
				enc := json.NewEncoder(c.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(res)
			}
			fmt.Fprintf(c.OutOrStdout(), "Publish %s finalized (status=%s)\n", res.PublishID, strings.TrimSpace(res.Status))
			return nil
		},
	}
	cmd.Flags().StringVar(&publishID, "publish-id", "", "Publish session ID")
	cmd.Flags().StringVar(&registry, "registry", "", "Registry base URL")
	cmd.Flags().StringVar(&registryToken, "registry-token", "", "Registry bearer token")
	return cmd
}

func newExtensionPublishStatusCmd(opts *common.GlobalOptions) *cobra.Command {
	var publishID string
	var registry string
	var registryToken string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Get status for a publish session",
		RunE: func(c *cobra.Command, args []string) error {
			if strings.TrimSpace(publishID) == "" {
				return fmt.Errorf("--publish-id is required")
			}
			reg, tok := applyRegistryConfig(registry, registryToken)
			res, err := external.PublishStatus(external.PublishStatusOptions{
				RegistryURL: reg,
				AuthToken:   tok,
				PublishID:   publishID,
				Timeout:     30 * time.Second,
			})
			if err != nil {
				return err
			}
			if opts.JSONOutput {
				enc := json.NewEncoder(c.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(res)
			}
			fmt.Fprintf(c.OutOrStdout(), "Publish %s status: %s\n", res.PublishID, strings.TrimSpace(res.Status))
			return nil
		},
	}
	cmd.Flags().StringVar(&publishID, "publish-id", "", "Publish session ID")
	cmd.Flags().StringVar(&registry, "registry", "", "Registry base URL")
	cmd.Flags().StringVar(&registryToken, "registry-token", "", "Registry bearer token")
	return cmd
}

func applyRegistryConfig(registry, token string) (string, string) {
	rc := common.LoadRunfabricrc()
	if strings.TrimSpace(registry) == "" && external.RegistryURLFromEnv() == "" && strings.TrimSpace(rc.RegistryURL) != "" {
		registry = rc.RegistryURL
	}
	if strings.TrimSpace(token) == "" && external.RegistryTokenFromEnv() == "" && strings.TrimSpace(rc.RegistryToken) != "" {
		token = rc.RegistryToken
	}
	if strings.TrimSpace(token) == "" && external.RegistryTokenFromEnv() == "" && strings.TrimSpace(rc.RegistryToken) == "" {
		if t, err := common.RegistryTokenFromAuthStore(context.Background(), ""); err == nil {
			token = t
		}
	}
	return strings.TrimSpace(registry), strings.TrimSpace(token)
}

func publishSessionDir() (string, error) {
	home, err := external.HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "publish-sessions"), nil
}

func publishSessionPath(publishID string) (string, error) {
	dir, err := publishSessionDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, strings.TrimSpace(publishID)+".json"), nil
}

func savePublishSession(s publishSession) (string, error) {
	if strings.TrimSpace(s.PublishID) == "" {
		return "", fmt.Errorf("publish session id is required")
	}
	path, err := publishSessionPath(s.PublishID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func loadPublishSession(publishID string) (publishSession, string, error) {
	path, err := publishSessionPath(publishID)
	if err != nil {
		return publishSession{}, "", err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return publishSession{}, "", err
	}
	var s publishSession
	if err := json.Unmarshal(b, &s); err != nil {
		return publishSession{}, "", err
	}
	return s, path, nil
}

func parseKVHeaders(headers []string) map[string]string {
	out := map[string]string{}
	for _, h := range headers {
		k, v, ok := strings.Cut(h, ":")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" {
			continue
		}
		out[k] = v
	}
	return out
}

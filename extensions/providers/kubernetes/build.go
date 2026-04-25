package kubernetes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/runfabric/runfabric/extensions/runtimes"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

const pullSecretName = "ghcr-pull-secret"

type buildRoute struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Handler string `json:"handler"`
}

// buildAndPushImage builds a Docker image from the project root and pushes it
// to GHCR. Returns the full image reference (registry/name:tag).
// Skipped when runtime is not nodejs or an explicit image is already set.
func buildAndPushImage(ctx context.Context, root string, cfg sdkprovider.Config, service, stage string) (string, error) {
	routes, entries := extractBuildRoutes(cfg)
	if len(entries) == 0 {
		return "", fmt.Errorf("no function entries found in config")
	}

	tool, err := containerTool()
	if err != nil {
		return "", err
	}

	// Stable tag: hash of routes + entries so identical deploys reuse the layer cache.
	tag := buildTag(stage, routes, entries)
	registry := sdkprovider.Env("GHCR_REGISTRY")
	if registry == "" {
		registry = "ghcr.io/runfabric"
	}
	imageRef := fmt.Sprintf("%s/%s:%s", registry, service, tag)

	// Write server.js to project root (picked up by COPY in Dockerfile).
	serverJSPath := filepath.Join(root, "server.js")
	if err := os.WriteFile(serverJSPath, []byte(runtimes.NodeServerJS), 0o644); err != nil {
		return "", fmt.Errorf("write server.js: %w", err)
	}
	defer os.Remove(serverJSPath)

	// Generate Dockerfile.runfabric.
	dockerfilePath := filepath.Join(root, "Dockerfile.runfabric")
	dockerfile := generateDockerfile(entries, routes)
	if err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0o644); err != nil {
		return "", fmt.Errorf("write Dockerfile.runfabric: %w", err)
	}
	defer os.Remove(dockerfilePath)

	// docker login ghcr.io
	if token := sdkprovider.Env("GHCR_TOKEN"); token != "" {
		user := sdkprovider.Env("GHCR_USER")
		if user == "" {
			user = "runfabric"
		}
		loginCmd := exec.CommandContext(ctx, tool, "login", "ghcr.io", "-u", user, "--password-stdin")
		loginCmd.Dir = root
		loginCmd.Stdin = strings.NewReader(token)
		loginCmd.Stdout = os.Stdout
		loginCmd.Stderr = os.Stderr
		if err := loginCmd.Run(); err != nil {
			return "", fmt.Errorf("registry login: %w", err)
		}
	}

	// docker build — target linux/amd64 for cloud k8s nodes (overridable via RUNFABRIC_BUILD_PLATFORM).
	platform := sdkprovider.Env("RUNFABRIC_BUILD_PLATFORM")
	if platform == "" {
		platform = "linux/amd64"
	}
	buildCmd := exec.CommandContext(ctx, tool, "build", "--platform", platform, "-f", "Dockerfile.runfabric", "-t", imageRef, ".")
	buildCmd.Dir = root
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return "", fmt.Errorf("docker build: %w", err)
	}

	// docker push
	pushCmd := exec.CommandContext(ctx, tool, "push", imageRef)
	pushCmd.Dir = root
	pushCmd.Stdout = os.Stdout
	pushCmd.Stderr = os.Stderr
	if err := pushCmd.Run(); err != nil {
		return "", fmt.Errorf("docker push: %w", err)
	}

	return imageRef, nil
}

// containerTool returns the first available container CLI: $CONTAINER_TOOL → docker → podman.
func containerTool() (string, error) {
	if t := sdkprovider.Env("CONTAINER_TOOL"); t != "" {
		return t, nil
	}
	for _, t := range []string{"docker", "podman"} {
		if _, err := exec.LookPath(t); err == nil {
			return t, nil
		}
	}
	return "", fmt.Errorf("no container tool found (docker/podman); set CONTAINER_TOOL or set image: in runfabric.yml")
}

// extractBuildRoutes reads FunctionsConfig from cfg and returns HTTP routes + entry file paths.
func extractBuildRoutes(cfg sdkprovider.Config) (routes []buildRoute, entries map[string]string) {
	entries = map[string]string{}
	fns, _ := cfg["FunctionsConfig"].([]any)
	for _, f := range fns {
		fm, _ := f.(map[string]any)
		if fm == nil {
			continue
		}
		name := asStr(fm["Name"])
		entry := asStr(fm["Entry"])
		if name == "" || entry == "" {
			continue
		}
		entries[name] = entry

		for _, t := range asSlice(fm["Triggers"]) {
			tm, _ := t.(map[string]any)
			if tm == nil {
				continue
			}
			if !strings.EqualFold(asStr(tm["Type"]), "http") {
				continue
			}
			method := strings.ToUpper(asStr(tm["Method"]))
			if method == "" {
				method = "GET"
			}
			routes = append(routes, buildRoute{
				Method:  method,
				Path:    asStr(tm["Path"]),
				Handler: name,
			})
		}
	}
	return
}

// generateDockerfile produces a multi-stage Dockerfile that bundles each
// function entry with esbuild and runs them via server.js.
func generateDockerfile(entries map[string]string, routes []buildRoute) string {
	routesJSON, _ := json.Marshal(routes)

	var sb strings.Builder
	sb.WriteString("FROM node:20-alpine AS builder\n")
	sb.WriteString("RUN npm install -g esbuild\n")
	sb.WriteString("WORKDIR /app\n")
	sb.WriteString("COPY . .\n")

	for name, entry := range entries {
		sb.WriteString(fmt.Sprintf(
			"RUN esbuild %s --bundle --platform=node --target=node20 --outfile=dist/%s.js\n",
			entry, name,
		))
	}

	sb.WriteString("\nFROM node:20-alpine\n")
	sb.WriteString("WORKDIR /app\n")
	sb.WriteString("COPY --from=builder /app/dist ./dist\n")
	sb.WriteString("COPY server.js .\n")
	sb.WriteString(fmt.Sprintf("ENV RUNFABRIC_ROUTES='%s'\n", string(routesJSON)))
	sb.WriteString("EXPOSE 80\n")
	sb.WriteString(`CMD ["node", "server.js"]` + "\n")

	return sb.String()
}

func buildTag(stage string, routes []buildRoute, entries map[string]string) string {
	h := sha256.New()
	h.Write([]byte(stage))
	b, _ := json.Marshal(routes)
	h.Write(b)
	b, _ = json.Marshal(entries)
	h.Write(b)
	return stage + "-" + hex.EncodeToString(h.Sum(nil))[:8]
}

func asStr(v any) string {
	s, _ := v.(string)
	return s
}

func asSlice(v any) []any {
	s, _ := v.([]any)
	return s
}

// ghcrDockerConfigJSON returns the .dockerconfigjson content for a GHCR pull secret.
// Returns "" when GHCR_TOKEN is not set.
// pullSecrets returns imagePullSecrets for the pod spec when GHCR_TOKEN is set.
func pullSecrets() []corev1.LocalObjectReference {
	if sdkprovider.Env("GHCR_TOKEN") == "" {
		return nil
	}
	return []corev1.LocalObjectReference{{Name: pullSecretName}}
}

func ghcrDockerConfigJSON() string {
	token := sdkprovider.Env("GHCR_TOKEN")
	if token == "" {
		return ""
	}
	user := sdkprovider.Env("GHCR_USER")
	if user == "" {
		user = "runfabric"
	}
	authB64 := encodeBase64(user + ":" + token)
	return fmt.Sprintf(`{"auths":{"ghcr.io":{"username":%q,"password":%q,"auth":%q}}}`,
		user, token, authB64)
}

func encodeBase64(s string) string {
	const t = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	in := []byte(s)
	var out []byte
	for i := 0; i < len(in); i += 3 {
		n := int(in[i]) << 16
		if i+1 < len(in) {
			n |= int(in[i+1]) << 8
		}
		if i+2 < len(in) {
			n |= int(in[i+2])
		}
		out = append(out, t[n>>18&0x3f], t[n>>12&0x3f])
		if i+1 < len(in) {
			out = append(out, t[n>>6&0x3f])
		} else {
			out = append(out, '=')
		}
		if i+2 < len(in) {
			out = append(out, t[n&0x3f])
		} else {
			out = append(out, '=')
		}
	}
	return string(out)
}

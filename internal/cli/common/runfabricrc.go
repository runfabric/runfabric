package common

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// runfabricrc is a tiny .npmrc-like config file for CLI settings.
// Format: KEY=VALUE (one per line). Lines starting with # are comments.
//
// Supported keys (v1):
// - registry.url=https://registry.runfabric.cloud
// - registry.token=... (bearer token for registry API)
// - auth.url=https://auth.runfabric.cloud
type runfabricrc struct {
	RegistryURL   string
	RegistryToken string
	AuthURL       string
}

func LoadRunfabricrc() runfabricrc {
	// Precedence: nearest .runfabricrc in current directory ancestry, then ~/.runfabricrc.
	if cwd, err := os.Getwd(); err == nil {
		if rc, ok := readNearestRunfabricrc(cwd); ok {
			return rc
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		if rc, ok := readRunfabricrcFile(filepath.Join(home, ".runfabricrc")); ok {
			return rc
		}
	}
	return runfabricrc{}
}

func readNearestRunfabricrc(startDir string) (runfabricrc, bool) {
	dir := startDir
	for {
		if rc, ok := readRunfabricrcFile(filepath.Join(dir, ".runfabricrc")); ok {
			return rc, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return runfabricrc{}, false
}

func readRunfabricrcFile(path string) (runfabricrc, bool) {
	f, err := os.Open(path)
	if err != nil {
		return runfabricrc{}, false
	}
	defer f.Close()

	rc := runfabricrc{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		switch k {
		case "registry.url":
			rc.RegistryURL = v
		case "registry.token":
			rc.RegistryToken = v
		case "auth.url":
			rc.AuthURL = v
		}
	}
	return rc, true
}

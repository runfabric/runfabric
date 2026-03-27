package external

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type Runfabricrc struct {
	RegistryURL   string
	RegistryToken string
}

// LoadRunfabricrc searches for .runfabricrc starting from startDir up to filesystem root,
// falling back to ~/.runfabricrc. Format: KEY=VALUE, supports registry.url and registry.token.
func LoadRunfabricrc(startDir string) Runfabricrc {
	if startDir != "" {
		if rc, ok := readNearestRunfabricrc(startDir); ok {
			return rc
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		if rc, ok := readRunfabricrcFile(filepath.Join(home, ".runfabricrc")); ok {
			return rc
		}
	}
	return Runfabricrc{}
}

func readNearestRunfabricrc(startDir string) (Runfabricrc, bool) {
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
	return Runfabricrc{}, false
}

func readRunfabricrcFile(path string) (Runfabricrc, bool) {
	f, err := os.Open(path)
	if err != nil {
		return Runfabricrc{}, false
	}
	defer f.Close()

	rc := Runfabricrc{}
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
		}
	}
	return rc, true
}

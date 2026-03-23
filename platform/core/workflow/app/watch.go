package app

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

// WatchProjectDir polls the project directory (derived from configPath) for changes to
// runfabric.yml, *.yaml, *.js, *.ts, *.mjs, *.cjs and sends to the returned channel (debounced).
// Stop by closing the done channel. The returned channel is closed when done is closed or after an error.
func WatchProjectDir(configPath string, pollInterval time.Duration, done <-chan struct{}) <-chan struct{} {
	absConfig, _ := filepath.Abs(configPath)
	projectDir := filepath.Dir(absConfig)

	out := make(chan struct{}, 1)
	exts := map[string]bool{
		".yml": true, ".yaml": true,
		".js": true, ".ts": true, ".mjs": true, ".cjs": true,
	}
	ignoredDirs := map[string]bool{
		"node_modules": true,
		"dist":         true,
		"build":        true,
		".git":         true,
		".runfabric":   true,
	}

	var lastMod map[string]time.Time
	var mu sync.Mutex

	collectModTimes := func() map[string]time.Time {
		m := make(map[string]time.Time)
		_ = filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				if ignoredDirs[info.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			rel, _ := filepath.Rel(projectDir, path)
			if rel == ".." || len(rel) > 2 && rel[:3] == ".."+string(filepath.Separator) {
				return filepath.SkipDir
			}
			if exts[filepath.Ext(path)] || filepath.Base(path) == "runfabric.yml" || filepath.Base(path) == "runfabric.yaml" {
				m[path] = info.ModTime()
			}
			return nil
		})
		return m
	}

	lastMod = collectModTimes()

	go func() {
		defer close(out)
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				cur := collectModTimes()
				mu.Lock()
				changed := false
				for path, t := range cur {
					if lastMod[path] != t {
						changed = true
						break
					}
				}
				for path := range lastMod {
					if cur[path].IsZero() {
						changed = true
						break
					}
				}
				lastMod = cur
				mu.Unlock()
				if changed {
					select {
					case out <- struct{}{}:
					default:
						// already pending restart
					}
				}
			}
		}
	}()

	return out
}

package resolution

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/runfabric/runfabric/platform/extension/external"
)

type boundaryCacheEntry struct {
	once     sync.Once
	boundary *Boundary
	err      error
}

var (
	boundaryCacheMu sync.RWMutex
	boundaryCache   = map[string]*boundaryCacheEntry{}
)

// NewCached returns a process-level cached extension boundary for the given options.
// Long-lived services (daemon/config-api) should use this to avoid rebuilding provider/runtime
// registries on every request.
func NewCached(opts Options) (*Boundary, error) {
	key, err := cacheKey(opts)
	if err != nil {
		return nil, err
	}

	boundaryCacheMu.RLock()
	entry, ok := boundaryCache[key]
	boundaryCacheMu.RUnlock()
	if !ok {
		boundaryCacheMu.Lock()
		entry, ok = boundaryCache[key]
		if !ok {
			entry = &boundaryCacheEntry{}
			boundaryCache[key] = entry
		}
		boundaryCacheMu.Unlock()
	}

	entry.once.Do(func() {
		entry.boundary, entry.err = New(opts)
	})
	return entry.boundary, entry.err
}

func cacheKey(opts Options) (string, error) {
	parts := []string{
		fmt.Sprintf("includeExternal=%t", opts.IncludeExternal),
		fmt.Sprintf("preferExternal=%t", opts.PreferExternal),
	}
	if opts.IncludeExternal {
		home, err := external.HomeDir()
		if err != nil {
			return "", err
		}
		parts = append(parts, "home="+home)
	}
	if len(opts.PinnedVersions) > 0 {
		keys := make([]string, 0, len(opts.PinnedVersions))
		for id := range opts.PinnedVersions {
			keys = append(keys, id)
		}
		sort.Strings(keys)
		for _, id := range keys {
			parts = append(parts, id+"="+opts.PinnedVersions[id])
		}
	}
	return strings.Join(parts, "|"), nil
}

package runstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	core "github.com/runfabric/runfabric/platform/core/state/core"
)

// LocalRunStore is the default, single-instance backend. It stores run JSON
// under <root>/.runfabric/runs/<stage>/<runID>.json (matching the core state
// layout), implements compare-and-swap via a content hash, and serializes
// writers with an in-process keyed lock.
//
// It is NOT safe across processes/instances: the in-process lock and the
// read-check-write CAS only coordinate goroutines in this process. Use a remote
// backend for multi-instance deployments.
type LocalRunStore struct {
	Root string

	mu sync.Mutex
	// saveLocks make each Save's read-check-write CAS atomic.
	saveLocks map[string]*sync.Mutex
	// execLocks back Lock(): the coarse per-run execution lock. Kept separate
	// from saveLocks so a caller may hold the exec lock and still Save (Go
	// mutexes are not reentrant — sharing one map would deadlock).
	execLocks map[string]*sync.Mutex
}

// compile-time assertions: LocalRunStore is both a store and a locker.
var (
	_ RunStore  = (*LocalRunStore)(nil)
	_ RunLocker = (*LocalRunStore)(nil)
)

// NewLocalRunStore returns a local backend rooted at root.
func NewLocalRunStore(root string) *LocalRunStore {
	return &LocalRunStore{
		Root:      root,
		saveLocks: map[string]*sync.Mutex{},
		execLocks: map[string]*sync.Mutex{},
	}
}

func (s *LocalRunStore) Kind() string { return "local" }

func (s *LocalRunStore) runDir(stage string) string {
	return filepath.Join(s.Root, ".runfabric", "runs", stage)
}

func (s *LocalRunStore) runPath(stage, runID string) string {
	return filepath.Join(s.runDir(stage), runID+".json")
}

// keyedMutex returns the per-run mutex from the given map, creating it on first use.
func (s *LocalRunStore) keyedMutex(set map[string]*sync.Mutex, stage, runID string) *sync.Mutex {
	key := stage + "/" + runID
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := set[key]
	if !ok {
		m = &sync.Mutex{}
		set[key] = m
	}
	return m
}

func versionOf(data []byte) Version {
	sum := sha256.Sum256(data)
	return Version(hex.EncodeToString(sum[:]))
}

// Load reads a run and returns its content-hash Version.
func (s *LocalRunStore) Load(_ context.Context, stage, runID string) (*core.WorkflowRun, Version, error) {
	if stage == "" || runID == "" {
		return nil, "", fmt.Errorf("runstore: stage and runID are required")
	}
	data, err := os.ReadFile(s.runPath(stage, runID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", ErrNotFound
		}
		return nil, "", err
	}
	var run core.WorkflowRun
	if err := json.Unmarshal(data, &run); err != nil {
		return nil, "", fmt.Errorf("runstore: unmarshal run: %w", err)
	}
	return &run, versionOf(data), nil
}

// currentVersion returns the on-disk version, or "" if the run is absent.
func (s *LocalRunStore) currentVersion(stage, runID string) (Version, error) {
	data, err := os.ReadFile(s.runPath(stage, runID))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return versionOf(data), nil
}

// Save writes the run with optimistic concurrency. When expected != "", the
// write is refused with ErrVersionConflict unless the stored version matches.
func (s *LocalRunStore) Save(_ context.Context, run *core.WorkflowRun, expected Version) (Version, error) {
	if run == nil {
		return "", fmt.Errorf("runstore: nil run")
	}
	if run.RunID == "" || run.Stage == "" {
		return "", fmt.Errorf("runstore: run stage and runID are required")
	}

	// Serialize the read-check-write so the CAS is atomic within this process.
	m := s.keyedMutex(s.saveLocks, run.Stage, run.RunID)
	m.Lock()
	defer m.Unlock()

	if expected != "" {
		cur, err := s.currentVersion(run.Stage, run.RunID)
		if err != nil {
			return "", err
		}
		if cur != expected {
			return "", fmt.Errorf("%w: expected %s, found %s", ErrVersionConflict, short(expected), short(cur))
		}
	}

	run.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return "", fmt.Errorf("runstore: marshal run: %w", err)
	}
	if err := core.WriteStateFile(s.runPath(run.Stage, run.RunID), data); err != nil {
		return "", err
	}
	return versionOf(data), nil
}

// List returns up to limit runs for the stage, newest-first by StartedAt.
func (s *LocalRunStore) List(_ context.Context, stage string, limit int) ([]*core.WorkflowRun, error) {
	if limit <= 0 {
		limit = 20
	}
	entries, err := os.ReadDir(s.runDir(stage))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	type item struct {
		run *core.WorkflowRun
		t   time.Time
	}
	var items []item
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.runDir(stage), e.Name()))
		if err != nil {
			continue
		}
		var r core.WorkflowRun
		if err := json.Unmarshal(data, &r); err != nil {
			continue
		}
		t, _ := time.Parse(time.RFC3339, r.StartedAt)
		items = append(items, item{run: &r, t: t})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].t.After(items[j].t) })
	if len(items) > limit {
		items = items[:limit]
	}
	out := make([]*core.WorkflowRun, 0, len(items))
	for _, it := range items {
		out = append(out, it.run)
	}
	return out, nil
}

// Lock acquires the in-process run lock. ttl is ignored: an in-process mutex
// cannot wedge across a crash. A distributed backend must honor ttl.
func (s *LocalRunStore) Lock(ctx context.Context, stage, runID string, _ time.Duration) (func() error, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m := s.keyedMutex(s.execLocks, stage, runID)
	m.Lock()
	released := false
	return func() error {
		if released {
			return nil
		}
		released = true
		m.Unlock()
		return nil
	}, nil
}

func short(v Version) string {
	if len(v) <= 12 {
		return string(v)
	}
	return string(v[:12])
}

// Package runstore defines the pluggable persistence + locking abstraction for
// workflow run state. It exists so workflow runs can be served by more than one
// process/instance safely.
//
// The default (local filesystem) backend is single-instance only: it stores run
// JSON under .runfabric/runs and serializes writers with an in-process lock. To
// run the same workflow from multiple instances (the target deployment), a
// remote backend must provide BOTH:
//
//   - optimistic concurrency on Save (a compare-and-swap against a Version
//     token), so two instances cannot silently clobber each other's run state; and
//   - a real distributed Lock (e.g. a DynamoDB conditional-write lease or Redis
//     lock), so only one instance drives a given run at a time.
//
// This package provides the interfaces, the local implementation, a scheme
// registry (local://, dynamodb://, ...), and a DynamoDB backend that supplies
// both guarantees for multi-instance deployments (see dynamodb.go).
package runstore

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	core "github.com/runfabric/runfabric/platform/core/state/core"
)

// EnvRunStore overrides the configured run-store URI (highest precedence). This
// lets an operator point an instance at a shared backend without editing the
// project config, e.g. RUNFABRIC_RUN_STORE=dynamodb://runs?region=us-east-1
const EnvRunStore = "RUNFABRIC_RUN_STORE"

// Version is an opaque compare-and-swap token (an ETag) identifying a stored run
// revision. An empty Version means "no expectation" — create if absent, or
// overwrite unconditionally.
type Version string

var (
	// ErrNotFound is returned by Load when the run does not exist.
	ErrNotFound = errors.New("runstore: run not found")
	// ErrVersionConflict is returned by Save when the stored version no longer
	// matches the expected version (a concurrent writer won). Callers should
	// reload and retry.
	ErrVersionConflict = errors.New("runstore: version conflict")
	// ErrLockHeld is returned by Lock when another owner holds the run lock.
	ErrLockHeld = errors.New("runstore: lock held by another owner")
)

// RunStore persists workflow run records with optimistic concurrency.
type RunStore interface {
	// Load returns the run and its current Version. ErrNotFound if absent.
	Load(ctx context.Context, stage, runID string) (*core.WorkflowRun, Version, error)

	// Save persists run. When expected != "", the write must fail with
	// ErrVersionConflict unless the currently stored version equals expected
	// (compare-and-swap). When expected == "", the write is unconditional.
	// It returns the Version of the newly written record.
	Save(ctx context.Context, run *core.WorkflowRun, expected Version) (Version, error)

	// List returns up to limit runs for a stage, newest-first (best effort).
	List(ctx context.Context, stage string, limit int) ([]*core.WorkflowRun, error)

	// Kind identifies the backend (e.g. "local", "dynamodb").
	Kind() string
}

// RunLocker provides mutual exclusion for driving a single run. A local backend
// may implement this in-process; a remote backend MUST implement it with a
// distributed lease so multiple instances coordinate.
type RunLocker interface {
	// Lock blocks (subject to ctx) until the run lock is acquired, returning a
	// release function. ttl bounds how long a crashed holder can wedge the lock
	// before it is considered stale; backends that auto-renew may ignore it.
	Lock(ctx context.Context, stage, runID string, ttl time.Duration) (release func() error, err error)
}

// Config is the parsed connection info handed to a backend Factory.
type Config struct {
	// Scheme is the URI scheme (local, dynamodb, ...).
	Scheme string
	// URI is the full backend URI as provided.
	URI string
	// Root is the local workspace root (used by the local backend and as a
	// fallback working directory).
	Root string
	// Params carries scheme-specific options from the URI query string.
	Params url.Values
}

// Factory builds a RunStore from Config. Implementations should also implement
// RunLocker on the returned value when they support locking.
type Factory func(cfg Config) (RunStore, error)

var (
	registryMu sync.RWMutex
	registry   = map[string]Factory{}
)

// Register associates a URI scheme with a backend Factory. It is intended to be
// called from a backend package's init(). Re-registering a scheme overwrites the
// previous factory.
func Register(scheme string, f Factory) {
	scheme = strings.ToLower(strings.TrimSpace(scheme))
	if scheme == "" || f == nil {
		return
	}
	registryMu.Lock()
	registry[scheme] = f
	registryMu.Unlock()
}

// RegisteredSchemes returns the registered backend schemes, sorted.
func RegisteredSchemes() []string {
	registryMu.RLock()
	out := make([]string, 0, len(registry))
	for s := range registry {
		out = append(out, s)
	}
	registryMu.RUnlock()
	sort.Strings(out)
	return out
}

// Open resolves a backend from a URI. An empty URI, "local", or a bare path maps
// to the local filesystem backend rooted at fallbackRoot (or the URI path).
//
// Examples:
//
//	""                              -> local at fallbackRoot
//	"local:///abs/workspace"        -> local at /abs/workspace
//	"dynamodb://table/runs?region=us-east-1"
func Open(uri, fallbackRoot string) (RunStore, error) {
	raw := strings.TrimSpace(uri)
	if raw == "" || raw == "local" {
		return NewLocalRunStore(fallbackRoot), nil
	}
	// Bare filesystem path (no scheme): treat as local root.
	if !strings.Contains(raw, "://") {
		return NewLocalRunStore(raw), nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("runstore: parse uri %q: %w", uri, err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme == "local" {
		root := u.Path
		if root == "" {
			root = fallbackRoot
		}
		return NewLocalRunStore(root), nil
	}
	registryMu.RLock()
	f := registry[scheme]
	registryMu.RUnlock()
	if f == nil {
		return nil, fmt.Errorf("runstore: no backend registered for scheme %q (have: %s)", scheme, strings.Join(RegisteredSchemes(), ", "))
	}
	return f(Config{Scheme: scheme, URI: raw, Root: fallbackRoot, Params: u.Query()})
}

// Resolve selects the run-store backend the user configured, with precedence:
//
//  1. RUNFABRIC_RUN_STORE env var (operator override)
//  2. configuredURI (e.g. extensions.runStore from runfabric.yml)
//  3. the local filesystem backend at fallbackRoot (default)
//
// Locking is not a separate setting: the chosen backend supplies its own
// RunLocker, so selecting the store also selects how runs are coordinated.
func Resolve(configuredURI, fallbackRoot string) (RunStore, error) {
	uri := strings.TrimSpace(os.Getenv(EnvRunStore))
	if uri == "" {
		uri = strings.TrimSpace(configuredURI)
	}
	return Open(uri, fallbackRoot)
}

// LockerFor returns the RunLocker for a store when the backend supports one
// (every backend in this package does), or nil when it does not.
func LockerFor(s RunStore) RunLocker {
	if l, ok := s.(RunLocker); ok {
		return l
	}
	return nil
}

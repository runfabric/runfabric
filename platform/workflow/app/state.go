package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	statetypes "github.com/runfabric/runfabric/internal/state/types"
	"github.com/runfabric/runfabric/platform/state/backends"
)

// StateListResult is the result of StateList / StatePull.
type StateListResult struct {
	Service  string                    `json:"service"`
	Backend  string                    `json:"backend"`
	Releases []statetypes.ReleaseEntry `json:"releases"`
}

// StateList returns release entries (stages and timestamps) from the configured receipt backend.
func StateList(configPath, stage string) (any, error) {
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}
	list, err := ctx.Backends.Receipts.ListReleases()
	if err != nil {
		return nil, fmt.Errorf("state list: %w", err)
	}
	return &StateListResult{
		Service:  ctx.Config.Service,
		Backend:  ctx.Backends.Receipts.Kind(),
		Releases: list,
	}, nil
}

// StatePull is the same as StateList (list state from the configured backend).
func StatePull(configPath, stage string) (any, error) {
	return StateList(configPath, stage)
}

// StateBackupSnapshot is the JSON shape written by StateBackup and read by StateRestore.
type StateBackupSnapshot struct {
	Backend  string                         `json:"backend"`
	Service  string                         `json:"service"`
	Releases []statetypes.ReleaseEntry      `json:"releases"`
	Receipts map[string]*statetypes.Receipt `json:"receipts"`
}

// StateBackup writes all receipts from the configured backend to a JSON file.
func StateBackup(configPath, stage, outPath string) (any, error) {
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}
	if outPath == "" {
		outPath = ".runfabric/state-backup.json"
	}
	releases, err := ctx.Backends.Receipts.ListReleases()
	if err != nil {
		return nil, fmt.Errorf("state backup: %w", err)
	}
	snap := StateBackupSnapshot{
		Backend:  ctx.Backends.Receipts.Kind(),
		Service:  ctx.Config.Service,
		Releases: releases,
		Receipts: make(map[string]*statetypes.Receipt),
	}
	for _, e := range releases {
		r, err := ctx.Backends.Receipts.Load(e.Stage)
		if err != nil {
			continue
		}
		snap.Receipts[e.Stage] = r
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("state backup marshal: %w", err)
	}
	if err := os.WriteFile(outPath, data, 0o600); err != nil {
		return nil, fmt.Errorf("state backup write: %w", err)
	}
	return map[string]any{
		"path":    outPath,
		"stages":  len(snap.Receipts),
		"backend": snap.Backend,
	}, nil
}

// StateRestore reads a backup file and writes all receipts to the configured backend.
func StateRestore(configPath, stage, filePath string) (any, error) {
	if filePath == "" {
		return nil, fmt.Errorf("state restore: --file required")
	}
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("state restore read: %w", err)
	}
	var snap StateBackupSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("state restore unmarshal: %w", err)
	}
	restored := 0
	for stageName, receipt := range snap.Receipts {
		if receipt == nil {
			continue
		}
		if err := ctx.Backends.Receipts.Save(receipt); err != nil {
			return nil, fmt.Errorf("state restore save %s: %w", stageName, err)
		}
		restored++
	}
	return map[string]any{
		"restored": restored,
		"backend":  ctx.Backends.Receipts.Kind(),
	}, nil
}

// StateForceUnlock releases the deploy lock for the given service/stage.
func StateForceUnlock(configPath, stage string) (any, error) {
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}
	if err := ctx.Backends.Locks.Release(ctx.Config.Service, ctx.Stage); err != nil {
		return nil, fmt.Errorf("state force-unlock: %w", err)
	}
	return map[string]any{
		"unlocked": true,
		"service":  ctx.Config.Service,
		"stage":    ctx.Stage,
		"backend":  ctx.Backends.Locks.Kind(),
	}, nil
}

// StateMigrate copies all receipts (and optionally journal for current stage) from one backend to another.
func StateMigrate(configPath, stage, fromKind, toKind string) (any, error) {
	if fromKind == "" || toKind == "" {
		return nil, fmt.Errorf("state migrate: --from and --to required")
	}
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}
	sourceOpts := BackendOptionsForKind(ctx, fromKind)
	targetOpts := BackendOptionsForKind(ctx, toKind)
	sourceBundle, err := backends.NewBundle(context.Background(), sourceOpts)
	if err != nil {
		return nil, fmt.Errorf("state migrate source backend: %w", err)
	}
	targetBundle, err := backends.NewBundle(context.Background(), targetOpts)
	if err != nil {
		return nil, fmt.Errorf("state migrate target backend: %w", err)
	}
	releases, err := sourceBundle.Receipts.ListReleases()
	if err != nil {
		return nil, fmt.Errorf("state migrate list: %w", err)
	}
	migrated := 0
	for _, e := range releases {
		r, err := sourceBundle.Receipts.Load(e.Stage)
		if err != nil {
			continue
		}
		if err := targetBundle.Receipts.Save(r); err != nil {
			return nil, fmt.Errorf("state migrate save %s: %w", e.Stage, err)
		}
		migrated++
	}
	return map[string]any{
		"from":     fromKind,
		"to":       toKind,
		"migrated": migrated,
	}, nil
}

// StateReconcileResult is one entry from StateReconcile.
type StateReconcileEntry struct {
	Stage string `json:"stage"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// StateReconcileResult is the result of StateReconcile.
type StateReconcileResult struct {
	Service string                `json:"service"`
	Backend string                `json:"backend"`
	Entries []StateReconcileEntry `json:"entries"`
}

// StateReconcile lists releases and verifies each receipt can be loaded (report OK or error per stage).
func StateReconcile(configPath, stage string) (any, error) {
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}
	releases, err := ctx.Backends.Receipts.ListReleases()
	if err != nil {
		return nil, fmt.Errorf("state reconcile: %w", err)
	}
	out := StateReconcileResult{
		Service: ctx.Config.Service,
		Backend: ctx.Backends.Receipts.Kind(),
		Entries: make([]StateReconcileEntry, 0, len(releases)),
	}
	for _, e := range releases {
		_, err := ctx.Backends.Receipts.Load(e.Stage)
		if err != nil {
			out.Entries = append(out.Entries, StateReconcileEntry{Stage: e.Stage, OK: false, Error: err.Error()})
		} else {
			out.Entries = append(out.Entries, StateReconcileEntry{Stage: e.Stage, OK: true})
		}
	}
	return &out, nil
}

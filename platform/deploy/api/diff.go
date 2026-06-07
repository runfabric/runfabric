package api

import (
	"fmt"
	"sort"

	contracts "github.com/runfabric/runfabric/platform/core/contracts/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
)

// computeChangeset diffs desired config against the last saved receipt.
// Returns a Changeset describing exactly which functions to create, update,
// delete, or leave unchanged. When no receipt exists, all functions are creates.
func computeChangeset(cfg *config.Config, stage, root string) *contracts.Changeset {
	cs := &contracts.Changeset{
		Service:  cfg.Service,
		Stage:    stage,
		Provider: cfg.Provider.Name,
	}

	// Build desired function set from config.
	desired := make(map[string]map[string]string, len(cfg.Functions))
	for name, fn := range cfg.Functions {
		desired[name] = functionFingerprint(name, fn, cfg)
	}

	// Load current state from receipt.
	receipt, err := state.Load(root, stage)
	if err != nil || receipt == nil {
		// No prior receipt — everything is a create.
		for name, after := range desired {
			cs.Functions = append(cs.Functions, contracts.ResourceChange{
				Name:   name,
				Op:     contracts.ChangeOpCreate,
				After:  after,
				Reason: "no prior deployment found",
			})
		}
		sortChanges(cs)
		return cs
	}

	// Index current deployed functions.
	current := make(map[string]map[string]string, len(receipt.Functions))
	for _, fd := range receipt.Functions {
		current[fd.Function] = map[string]string{
			"resourceName":       fd.ResourceName,
			"resourceIdentifier": fd.ResourceIdentifier,
			"artifactSha256":     fd.ArtifactSHA256,
			"configSignature":    fd.ConfigSignature,
			"environmentHash":    fd.EnvironmentHash,
		}
	}

	// Desired functions: create or update.
	for name, after := range desired {
		before, exists := current[name]
		if !exists {
			cs.Functions = append(cs.Functions, contracts.ResourceChange{
				Name:   name,
				Op:     contracts.ChangeOpCreate,
				After:  after,
				Reason: "new function",
			})
			continue
		}
		reason := changeReason(before, after)
		if reason != "" {
			cs.Functions = append(cs.Functions, contracts.ResourceChange{
				Name:   name,
				Op:     contracts.ChangeOpUpdate,
				Before: before,
				After:  after,
				Reason: reason,
			})
		} else {
			cs.Functions = append(cs.Functions, contracts.ResourceChange{
				Name:   name,
				Op:     contracts.ChangeOpNoOp,
				Before: before,
				After:  after,
			})
		}
	}

	// Current functions not in desired → delete.
	for name, before := range current {
		if _, ok := desired[name]; !ok {
			cs.Functions = append(cs.Functions, contracts.ResourceChange{
				Name:   name,
				Op:     contracts.ChangeOpDelete,
				Before: before,
				Reason: "function removed from config",
			})
		}
	}

	sortChanges(cs)
	return cs
}

// functionFingerprint builds a stable string map representing the desired
// state of a function. Used for equality comparison against receipt state.
func functionFingerprint(name string, fn config.FunctionConfig, cfg *config.Config) map[string]string {
	fp := map[string]string{
		"name":    name,
		"runtime": fn.Runtime,
		"handler": fn.Handler,
	}
	if fn.Runtime == "" {
		fp["runtime"] = cfg.Provider.Runtime
	}
	if fn.Memory > 0 {
		fp["memory"] = fmt.Sprintf("%d", fn.Memory)
	}
	if fn.Timeout > 0 {
		fp["timeout"] = fmt.Sprintf("%d", fn.Timeout)
	}
	// configSignature is the field both the desired fingerprint and the receipt
	// populate, so it is the one used for change detection (changeReason). The
	// other entries are descriptive context for display only.
	if sig, err := config.FunctionConfigSignature(fn); err == nil {
		fp["configSignature"] = sig
	}
	return fp
}

// changeReason returns a human-readable reason if the desired function differs
// from the deployed one, or empty string if they are equivalent. It compares the
// config signature — the field present and populated on both the desired
// fingerprint and the receipt-derived state — rather than the descriptive keys,
// which do not overlap between the two maps.
func changeReason(before, after map[string]string) string {
	desired := after["configSignature"]
	if desired == "" {
		// Without a config signature we cannot prove the function is unchanged,
		// so report a change rather than risk a false no-op.
		return "config signature unavailable"
	}
	if before["configSignature"] != desired {
		return "config changed"
	}
	return ""
}

func sortChanges(cs *contracts.Changeset) {
	sort.Slice(cs.Functions, func(i, j int) bool {
		return cs.Functions[i].Name < cs.Functions[j].Name
	})
}

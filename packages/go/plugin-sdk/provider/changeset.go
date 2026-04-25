package provider

import "context"

// ChangeOp is the kind of change for a resource.
type ChangeOp string

const (
	ChangeOpCreate ChangeOp = "create"
	ChangeOpUpdate ChangeOp = "update"
	ChangeOpDelete ChangeOp = "delete"
	ChangeOpNoOp   ChangeOp = "no-op"
)

// ResourceChange is a single resource change in a Changeset.
type ResourceChange struct {
	Name   string
	Op     ChangeOp
	Before map[string]string
	After  map[string]string
	Reason string
}

// Changeset is the pre-computed diff between desired config and deployed state.
// Providers receive this via ChangesetFromContext and apply changes without recomputing.
type Changeset struct {
	Service   string
	Stage     string
	Provider  string
	Functions []ResourceChange
}

// HasChanges returns true if any function has a create, update, or delete operation.
func (c *Changeset) HasChanges() bool {
	if c == nil {
		return false
	}
	for _, ch := range c.Functions {
		if ch.Op != ChangeOpNoOp {
			return true
		}
	}
	return false
}

// ForOp returns all ResourceChanges with the given operation.
func (c *Changeset) ForOp(op ChangeOp) []ResourceChange {
	if c == nil {
		return nil
	}
	var out []ResourceChange
	for _, ch := range c.Functions {
		if ch.Op == op {
			out = append(out, ch)
		}
	}
	return out
}

type changesetKey struct{}

// ContextWithChangeset returns a new context carrying the changeset.
func ContextWithChangeset(ctx context.Context, cs *Changeset) context.Context {
	return context.WithValue(ctx, changesetKey{}, cs)
}

// ChangesetFromContext returns the Changeset stored in ctx, or nil if none.
func ChangesetFromContext(ctx context.Context) *Changeset {
	cs, _ := ctx.Value(changesetKey{}).(*Changeset)
	return cs
}

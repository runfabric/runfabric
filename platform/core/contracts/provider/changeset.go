package contracts

// ChangeOp is the kind of change for a resource.
type ChangeOp string

const (
	ChangeOpCreate ChangeOp = "create"
	ChangeOpUpdate ChangeOp = "update"
	ChangeOpDelete ChangeOp = "delete"
	ChangeOpNoOp   ChangeOp = "no-op"
)

// ResourceChange is a single resource change computed by the diff engine.
type ResourceChange struct {
	Name   string            // function name or logical resource name
	Op     ChangeOp          // create | update | delete | no-op
	Before map[string]string // previous known state (from last receipt); nil on create
	After  map[string]string // desired state (from config); nil on delete
	Reason string            // human-readable reason for the change
}

// Changeset is the computed diff between desired config and current deployed state.
// Providers receive this pre-computed; they apply changes without re-computing the diff.
type Changeset struct {
	Service   string
	Stage     string
	Provider  string
	Functions []ResourceChange // per-function changes
}

// Summary returns a count breakdown of change operations.
func (c *Changeset) Summary() (create, update, delete, noop int) {
	if c == nil {
		return
	}
	for _, ch := range c.Functions {
		switch ch.Op {
		case ChangeOpCreate:
			create++
		case ChangeOpUpdate:
			update++
		case ChangeOpDelete:
			delete++
		case ChangeOpNoOp:
			noop++
		}
	}
	return
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

package contracts

import "context"

type changesetKey struct{}

// ContextWithChangeset returns a new context carrying the pre-computed changeset.
func ContextWithChangeset(ctx context.Context, cs *Changeset) context.Context {
	return context.WithValue(ctx, changesetKey{}, cs)
}

// ChangesetFromContext returns the Changeset stored in ctx, or nil if none.
// Providers call this to read the pre-computed diff instead of recomputing it.
func ChangesetFromContext(ctx context.Context) *Changeset {
	cs, _ := ctx.Value(changesetKey{}).(*Changeset)
	return cs
}

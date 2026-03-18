package providers

// namedProvider wraps a Provider to expose a different Name(). Used to register
// the same implementation under multiple names (e.g. aws and aws-lambda).
type namedProvider struct {
	name string
	Provider
}

func (n *namedProvider) Name() string { return n.name }

// NewNamedProvider returns a Provider that delegates to p but reports Name() as name.
func NewNamedProvider(name string, p Provider) Provider {
	return &namedProvider{name: name, Provider: p}
}

// Built-in provider plugins (AWS, GCP, etc.) are registered in app.Bootstrap to avoid
// import cycles (engine/internal/extensions/provider/aws imports deployexec/state). Callers use
// NewRegistry() and register implementations from engine/internal/extensions/provider/<name>.

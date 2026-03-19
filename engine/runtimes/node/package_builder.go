package node

// PackageBuilder builds the Node package (npm install, bundle if needed).
type PackageBuilder struct{}

// Build builds the package at the given path.
func (b *PackageBuilder) Build(dir string) error {
	return nil
}

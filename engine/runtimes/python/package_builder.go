package python

// PackageBuilder builds the Python package (pip install -e ., etc.).
type PackageBuilder struct{}

// Build builds the package at the given path.
func (b *PackageBuilder) Build(dir string) error {
	return nil
}

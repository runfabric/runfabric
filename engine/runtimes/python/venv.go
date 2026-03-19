package python

// Venv manages a virtualenv for the project.
type Venv struct {
	Path string
}

// Ensure creates the venv if it does not exist.
func (v *Venv) Ensure() error {
	return nil
}

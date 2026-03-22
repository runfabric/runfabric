package python

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Venv manages a virtualenv for the project.
type Venv struct {
	Path string
}

// Ensure creates the venv if it does not exist.
func (v *Venv) Ensure() error {
	if v == nil || v.Path == "" {
		return fmt.Errorf("venv path is required")
	}
	python := "python3"
	if _, err := exec.LookPath(python); err != nil {
		python = "python"
		if _, err := exec.LookPath(python); err != nil {
			return fmt.Errorf("python interpreter not found in PATH")
		}
	}
	if _, err := os.Stat(filepath.Join(v.Path, "pyvenv.cfg")); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(v.Path), 0o755); err != nil {
		return fmt.Errorf("create venv parent dir: %w", err)
	}
	cmd := exec.Command(python, "-m", "venv", v.Path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("create venv at %s: %w", v.Path, err)
	}
	return nil
}

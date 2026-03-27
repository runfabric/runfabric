package python

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// PackageBuilder builds the Python package (pip install -e ., etc.).
type PackageBuilder struct{}

// Build builds the package at the given path.
func (b *PackageBuilder) Build(dir string) error {
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("python build dir not found: %w", err)
	}

	venv := &Venv{Path: filepath.Join(dir, ".venv")}
	if err := venv.Ensure(); err != nil {
		return err
	}

	requirements := filepath.Join(dir, "requirements.txt")
	if _, err := os.Stat(requirements); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("stat requirements.txt: %w", err)
	}

	pythonExec := filepath.Join(venv.Path, "bin", "python")
	if _, err := os.Stat(pythonExec); err != nil {
		return fmt.Errorf("python executable not found in venv: %w", err)
	}
	cmd := exec.Command(pythonExec, "-m", "pip", "install", "-r", requirements)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pip install failed: %w", err)
	}
	return nil
}

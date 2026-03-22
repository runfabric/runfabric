package app

import (
	"os"
	"os/exec"
	"path/filepath"
)

// Test runs the project's test suite (e.g. npm test, go test, pytest).
// Detects project type from root and runs the appropriate test command.
func Test(configPath string) (any, error) {
	root := filepath.Dir(configPath)
	var cmd *exec.Cmd
	if _, err := os.Stat(filepath.Join(root, "package.json")); err == nil {
		cmd = exec.Command("npm", "test")
	} else if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
		cmd = exec.Command("go", "test", "./...")
	} else if _, err := os.Stat(filepath.Join(root, "pyproject.toml")); err == nil {
		cmd = exec.Command("pytest")
	} else {
		return map[string]string{"message": "No package.json, go.mod, or pyproject.toml found; run tests manually (npm test, go test, pytest)"}, nil
	}
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return map[string]string{"message": "Tests completed successfully"}, nil
}

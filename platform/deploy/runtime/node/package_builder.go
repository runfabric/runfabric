package node

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// PackageBuilder builds the Node package (npm install, bundle if needed).
type PackageBuilder struct{}

// Build builds the package at the given path.
func (b *PackageBuilder) Build(dir string) error {
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("node build dir not found: %w", err)
	}

	packageJSON := filepath.Join(dir, "package.json")
	if _, err := os.Stat(packageJSON); os.IsNotExist(err) {
		// No package manifest means there are no dependencies to install.
		return nil
	} else if err != nil {
		return fmt.Errorf("stat package.json: %w", err)
	}

	cmdName, args := detectNodePackageCommand(dir)
	cmd := exec.Command(cmdName, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("node dependency install failed (%s %v): %w", cmdName, args, err)
	}
	return nil
}

func detectNodePackageCommand(dir string) (string, []string) {
	if fileExists(filepath.Join(dir, "pnpm-lock.yaml")) {
		return "pnpm", []string{"install", "--frozen-lockfile"}
	}
	if fileExists(filepath.Join(dir, "yarn.lock")) {
		return "yarn", []string{"install", "--frozen-lockfile"}
	}
	if fileExists(filepath.Join(dir, "bun.lockb")) || fileExists(filepath.Join(dir, "bun.lock")) {
		return "bun", []string{"install", "--frozen-lockfile"}
	}
	if fileExists(filepath.Join(dir, "package-lock.json")) || fileExists(filepath.Join(dir, "npm-shrinkwrap.json")) {
		return "npm", []string{"ci"}
	}
	return "npm", []string{"install"}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

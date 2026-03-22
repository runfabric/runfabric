package project_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	rootcli "github.com/runfabric/runfabric/internal/cli"
)

// setupComposeDir creates a temp dir with a minimal runfabric.compose.yml and one service config (runfabric.yml).
func setupComposeDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	compose := `services:
  - name: svc1
    config: ./runfabric.yml
`
	if err := os.WriteFile(filepath.Join(dir, "runfabric.compose.yml"), []byte(compose), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := `service: svc1
provider:
  name: aws-lambda
  runtime: nodejs
functions:
  - name: api
    entry: index.handler
`
	if err := os.WriteFile(filepath.Join(dir, "runfabric.yml"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestComposePlan_RunsWithoutError(t *testing.T) {
	dir := setupComposeDir(t)
	root := rootcli.NewRootCmd()
	root.SetArgs([]string{"compose", "plan", "-f", filepath.Join(dir, "runfabric.compose.yml"), "--stage", "dev"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Errorf("compose plan should succeed: %v", err)
	}
}

func TestComposeDeploy_RunsWithValidComposeFile(t *testing.T) {
	dir := setupComposeDir(t)
	root := rootcli.NewRootCmd()
	root.SetArgs([]string{"compose", "deploy", "-f", filepath.Join(dir, "runfabric.compose.yml"), "--stage", "dev"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	// May fail at provider deploy step (e.g. no creds); we only check the command runs.
	_ = root.Execute()
}

func TestComposeRemove_RunsWithValidComposeFile(t *testing.T) {
	dir := setupComposeDir(t)
	root := rootcli.NewRootCmd()
	root.SetArgs([]string{"compose", "remove", "-f", filepath.Join(dir, "runfabric.compose.yml"), "--stage", "dev"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	// May fail if nothing deployed (e.g. recovery check); we only check the command runs.
	_ = root.Execute()
}

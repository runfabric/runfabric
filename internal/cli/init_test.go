package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestYAMLQuoted(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"safe alphanumeric", "my-service", "my-service"},
		{"safe with hyphen", "my-service-1", "my-service-1"},
		{"safe with underscore", "my_service", "my_service"},
		{"safe with dot", "svc.v1", "svc.v1"},
		{"empty", "", `""`},
		{"newline injection", "ok\nmalicious: key: value", `"ok\nmalicious: key: value"`},
		{"colon injection", "name: injected", `"name: injected"`},
		{"quote in value", `say "hello"`, `"say \"hello\""`},
		{"backslash", `path\to\file`, `"path\\to\\file"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := yamlQuoted(tt.in)
			if got != tt.want {
				t.Errorf("yamlQuoted(%q) = %q, want %q", tt.in, got, tt.want)
			}
			if strings.Contains(tt.in, "\n") && !strings.HasPrefix(got, `"`) {
				t.Errorf("yamlQuoted(%q) must quote string containing newline, got %q", tt.in, got)
			}
		})
	}
}

func TestInit_ValidLang(t *testing.T) {
	dir := t.TempDir()
	opts := &GlobalOptions{}
	cmd := newInitCmd(opts)
	cmd.SetArgs([]string{"--dir", dir, "--lang", "ts", "--no-interactive"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("init --lang ts should succeed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "runfabric.yml")); err != nil {
		t.Errorf("runfabric.yml not created: %v", err)
	}
}

func TestInit_InvalidLang(t *testing.T) {
	dir := t.TempDir()
	opts := &GlobalOptions{}
	cmd := newInitCmd(opts)
	cmd.SetArgs([]string{"--dir", dir, "--lang", "rust", "--no-interactive"})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil {
		t.Error("init --lang rust should fail")
	}
}

func TestInit_ProviderTemplateMatrixRejection(t *testing.T) {
	dir := t.TempDir()
	opts := &GlobalOptions{}
	cmd := newInitCmd(opts)
	// fly-machines does not support cron per Trigger Capability Matrix
	cmd.SetArgs([]string{"--dir", dir, "--provider", "fly-machines", "--template", "cron", "--no-interactive"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil {
		t.Error("init provider=fly-machines template=cron should fail (matrix)")
	}
}

func TestInit_ProviderTemplateMatrixAccept(t *testing.T) {
	dir := t.TempDir()
	opts := &GlobalOptions{}
	cmd := newInitCmd(opts)
	cmd.SetArgs([]string{"--dir", dir, "--provider", "aws-lambda", "--template", "api", "--lang", "go", "--no-interactive"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("init provider=aws-lambda template=api lang=go should succeed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "handler.go")); err != nil {
		t.Errorf("handler.go not created: %v", err)
	}
}

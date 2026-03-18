package external

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
)

// ExternalProviderAdapter implements the legacy providers.Provider interface by spawning
// an external plugin executable and speaking the line-delimited JSON protocol over stdio.
//
// Phase 15c: this starts a fresh subprocess per call (simple + safe). We can optimize
// to a reused process with idle timeout later.
type ExternalProviderAdapter struct {
	name       string
	executable string
	timeout    time.Duration
}

func NewExternalProviderAdapter(name, executable string) *ExternalProviderAdapter {
	return &ExternalProviderAdapter{
		name:       name,
		executable: executable,
		timeout:    30 * time.Second,
	}
}

func (p *ExternalProviderAdapter) Name() string { return p.name }

func (p *ExternalProviderAdapter) Doctor(cfg *providers.Config, stage string) (*providers.DoctorResult, error) {
	var out providers.DoctorResult
	if err := p.call("Doctor", map[string]any{"config": cfg, "stage": stage}, &out); err != nil {
		return nil, err
	}
	if out.Provider == "" {
		out.Provider = p.name
	}
	return &out, nil
}

func (p *ExternalProviderAdapter) Plan(cfg *providers.Config, stage, root string) (*providers.PlanResult, error) {
	var out providers.PlanResult
	if err := p.call("Plan", map[string]any{"config": cfg, "stage": stage, "root": root}, &out); err != nil {
		return nil, err
	}
	if out.Provider == "" {
		out.Provider = p.name
	}
	return &out, nil
}

func (p *ExternalProviderAdapter) Deploy(cfg *providers.Config, stage, root string) (*providers.DeployResult, error) {
	var out providers.DeployResult
	if err := p.call("Deploy", map[string]any{"config": cfg, "stage": stage, "root": root}, &out); err != nil {
		return nil, err
	}
	if out.Provider == "" {
		out.Provider = p.name
	}
	return &out, nil
}

func (p *ExternalProviderAdapter) Remove(cfg *providers.Config, stage, root string) (*providers.RemoveResult, error) {
	var out providers.RemoveResult
	if err := p.call("Remove", map[string]any{"config": cfg, "stage": stage, "root": root}, &out); err != nil {
		return nil, err
	}
	if out.Provider == "" {
		out.Provider = p.name
	}
	return &out, nil
}

func (p *ExternalProviderAdapter) Invoke(cfg *providers.Config, stage, function string, payload []byte) (*providers.InvokeResult, error) {
	var out providers.InvokeResult
	if err := p.call("Invoke", map[string]any{"config": cfg, "stage": stage, "function": function, "payload": payload}, &out); err != nil {
		return nil, err
	}
	if out.Provider == "" {
		out.Provider = p.name
	}
	return &out, nil
}

func (p *ExternalProviderAdapter) Logs(cfg *providers.Config, stage, function string) (*providers.LogsResult, error) {
	var out providers.LogsResult
	if err := p.call("Logs", map[string]any{"config": cfg, "stage": stage, "function": function}, &out); err != nil {
		return nil, err
	}
	if out.Provider == "" {
		out.Provider = p.name
	}
	return &out, nil
}

func (p *ExternalProviderAdapter) call(method string, params any, out any) error {
	cmd := exec.Command(p.executable)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	req := Request{ID: "1", Method: method, Params: params}
	enc := json.NewEncoder(stdin)
	if err := enc.Encode(req); err != nil {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		return err
	}
	_ = stdin.Close()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("external plugin %s failed: %w (stderr: %s)", p.executable, err, stderr.String())
		}
	case <-time.After(p.timeout):
		_ = cmd.Process.Kill()
		return fmt.Errorf("external plugin %s timed out", p.executable)
	}

	sc := bufio.NewScanner(bytes.NewReader(stdout.Bytes()))
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return err
		}
		return fmt.Errorf("external plugin %s produced no response (stderr: %s)", p.executable, stderr.String())
	}
	var resp Response
	if err := json.Unmarshal(sc.Bytes(), &resp); err != nil {
		return fmt.Errorf("external plugin %s invalid response: %w", p.executable, err)
	}
	if resp.Error != nil {
		return fmt.Errorf("external plugin error [%s]: %s", resp.Error.Code, resp.Error.Message)
	}
	if out == nil {
		return nil
	}
	blob, err := json.Marshal(resp.Result)
	if err != nil {
		return err
	}
	return json.Unmarshal(blob, out)
}

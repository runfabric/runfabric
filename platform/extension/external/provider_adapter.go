package external

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/extension/wrapper"
	extRuntime "github.com/runfabric/runfabric/platform/extensions/registry/loader/runtime"
)

// ExternalProviderAdapter implements providers.ProviderPlugin by spawning
// an external plugin executable and speaking the line-delimited JSON protocol over stdio.
//
// Phase 15c: reuse a subprocess per command context (idle timeout) and add protocol
// handshake/version negotiation.
type ExternalProviderAdapter struct {
	name        string
	executable  string
	meta        providers.ProviderMeta
	timeout     time.Duration
	idleTimeout time.Duration

	mu        sync.Mutex
	proc      *pluginProc
	idleTimer *time.Timer
	seq       uint64
	debug     bool
}

const (
	maxStderrBytes     = 8 * 1024
	defaultIdleTimeout = 60 * time.Second
	envPluginDebug     = "RUNFABRIC_PLUGIN_DEBUG"
)

type pluginProc struct {
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	scanner    *bufio.Scanner
	stderr     *limitedBuffer
	handshaked bool
}

type limitedBuffer struct {
	mu  sync.Mutex
	max int
	buf []byte
}

func newLimitedBuffer(max int) *limitedBuffer {
	return &limitedBuffer{max: max}
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(p) > b.max {
		p = p[len(p)-b.max:]
	}
	b.buf = append(b.buf, p...)
	if len(b.buf) > b.max {
		b.buf = b.buf[len(b.buf)-b.max:]
	}
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

func NewExternalProviderAdapter(name, executable string, meta providers.ProviderMeta) *ExternalProviderAdapter {
	if strings.TrimSpace(meta.Name) == "" {
		meta.Name = name
	}
	return &ExternalProviderAdapter{
		name:        name,
		executable:  executable,
		meta:        meta,
		timeout:     30 * time.Second,
		idleTimeout: defaultIdleTimeout,
		debug:       isTruthyEnv(envPluginDebug),
	}
}

func (p *ExternalProviderAdapter) Meta() providers.ProviderMeta {
	meta := p.meta
	if strings.TrimSpace(meta.Name) == "" {
		meta.Name = p.name
	}
	return meta
}

func (p *ExternalProviderAdapter) ValidateConfig(ctx context.Context, req providers.ValidateConfigRequest) error {
	return nil
}

func (p *ExternalProviderAdapter) nextIDLocked() string {
	p.seq++
	return fmt.Sprintf("%d", p.seq)
}

func (p *ExternalProviderAdapter) Doctor(ctx context.Context, req providers.DoctorRequest) (*providers.DoctorResult, error) {
	var out providers.DoctorResult
	if err := p.call("Doctor", map[string]any{"config": req.Config, "stage": req.Stage}, &out); err != nil {
		return nil, err
	}
	if out.Provider == "" {
		out.Provider = p.name
	}
	return &out, nil
}

func (p *ExternalProviderAdapter) Plan(ctx context.Context, req providers.PlanRequest) (*providers.PlanResult, error) {
	var out providers.PlanResult
	if err := p.call("Plan", map[string]any{"config": req.Config, "stage": req.Stage, "root": req.Root}, &out); err != nil {
		return nil, err
	}
	if out.Provider == "" {
		out.Provider = p.name
	}
	return &out, nil
}

func (p *ExternalProviderAdapter) Deploy(ctx context.Context, req providers.DeployRequest) (*providers.DeployResult, error) {
	var out providers.DeployResult
	if err := p.call("Deploy", map[string]any{"config": req.Config, "stage": req.Stage, "root": req.Root}, &out); err != nil {
		return nil, err
	}
	if out.Provider == "" {
		out.Provider = p.name
	}
	return &out, nil
}

func (p *ExternalProviderAdapter) Remove(ctx context.Context, req providers.RemoveRequest) (*providers.RemoveResult, error) {
	var out providers.RemoveResult
	if err := p.call("Remove", map[string]any{"config": req.Config, "stage": req.Stage, "root": req.Root}, &out); err != nil {
		return nil, err
	}
	if out.Provider == "" {
		out.Provider = p.name
	}
	return &out, nil
}

func (p *ExternalProviderAdapter) Invoke(ctx context.Context, req providers.InvokeRequest) (*providers.InvokeResult, error) {
	var out providers.InvokeResult
	if err := p.call("Invoke", map[string]any{"config": req.Config, "stage": req.Stage, "function": req.Function, "payload": req.Payload}, &out); err != nil {
		return nil, err
	}
	if out.Provider == "" {
		out.Provider = p.name
	}
	return &out, nil
}

func (p *ExternalProviderAdapter) Logs(ctx context.Context, req providers.LogsRequest) (*providers.LogsResult, error) {
	var out providers.LogsResult
	if err := p.call("Logs", map[string]any{"config": req.Config, "stage": req.Stage, "function": req.Function}, &out); err != nil {
		return nil, err
	}
	if out.Provider == "" {
		out.Provider = p.name
	}
	return &out, nil
}

func (p *ExternalProviderAdapter) call(method string, params any, out any) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Stop idle timer while the plugin is in use.
	if p.idleTimer != nil {
		p.idleTimer.Stop()
		p.idleTimer = nil
	}

	if err := p.ensureProcessLocked(); err != nil {
		return err
	}
	proc := p.proc

	req := Request{
		ID:              p.nextIDLocked(),
		Method:          method,
		ProtocolVersion: extRuntime.ProtocolVersion,
		Params:          params,
	}

	if p.debug {
		fmt.Fprintf(os.Stderr, "external plugin %s call=%s id=%s\n", p.executable, method, req.ID)
	}

	if err := json.NewEncoder(proc.stdin).Encode(req); err != nil {
		p.killProcessLocked()
		return fmt.Errorf("external plugin %s failed to write request: %w", p.executable, err)
	}

	resp, err := p.readResponseLocked(req.ID)
	if err != nil {
		// Process might already be dead; clean up and surface error.
		p.killProcessLocked()
		return err
	}

	// Reset idle timer after a successful request.
	p.idleTimer = time.AfterFunc(p.idleTimeout, func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.proc != nil {
			p.killProcessLocked()
		}
	})

	if resp.Error != nil {
		if resp.Error.Code != "" {
			return fmt.Errorf("external plugin error [%s]: %s", resp.Error.Code, resp.Error.Message)
		}
		return fmt.Errorf("external plugin error: %s", resp.Error.Message)
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

func limitString(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}

func isTruthyEnv(k string) bool {
	v := os.Getenv(k)
	v = strings.ToLower(strings.TrimSpace(v))
	return v == "1" || v == "true" || v == "yes"
}

func (p *ExternalProviderAdapter) ensureProcessLocked() error {
	if p.proc != nil {
		return nil
	}
	stderr := newLimitedBuffer(maxStderrBytes)
	cmd := exec.Command(p.executable)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return err
	}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return err
	}

	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	p.proc = &pluginProc{
		cmd:        cmd,
		stdin:      stdin,
		stdout:     stdout,
		scanner:    sc,
		stderr:     stderr,
		handshaked: false,
	}

	if err := p.handshakeLocked(); err != nil {
		p.killProcessLocked()
		return err
	}
	return nil
}

func (p *ExternalProviderAdapter) handshakeLocked() error {
	if p.proc == nil {
		return fmt.Errorf("external plugin %s internal error: nil process", p.executable)
	}
	// Perform handshake once per process start.
	pid := p.nextIDLocked()
	req := Request{
		ID:              pid,
		Method:          "Handshake",
		ProtocolVersion: extRuntime.ProtocolVersion,
	}
	if p.debug {
		fmt.Fprintf(os.Stderr, "external plugin %s handshake id=%s\n", p.executable, pid)
	}
	if err := json.NewEncoder(p.proc.stdin).Encode(req); err != nil {
		return fmt.Errorf("external plugin %s handshake write failed: %w", p.executable, err)
	}
	resp, err := p.readResponseLocked(pid)
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf("external plugin %s handshake failed: %s", p.executable, resp.Error.Message)
	}

	var hs wrapper.Handshake
	b, err := json.Marshal(resp.Result)
	if err != nil {
		return fmt.Errorf("external plugin %s handshake malformed result: %w", p.executable, err)
	}
	if err := json.Unmarshal(b, &hs); err != nil {
		return fmt.Errorf("external plugin %s handshake result decode failed: %w", p.executable, err)
	}
	if strings.TrimSpace(hs.ProtocolVersion) != strings.TrimSpace(extRuntime.ProtocolVersion) {
		return fmt.Errorf(
			"external plugin %s incompatible protocolVersion: expected %s got %s (version=%s platform=%s)",
			p.executable,
			extRuntime.ProtocolVersion,
			hs.ProtocolVersion,
			hs.Version,
			hs.Platform,
		)
	}
	if len(hs.Capabilities) > 0 {
		p.meta.Capabilities = append([]string(nil), hs.Capabilities...)
	}
	if len(hs.SupportsRuntime) > 0 {
		p.meta.SupportsRuntime = append([]string(nil), hs.SupportsRuntime...)
	}
	if len(hs.SupportsTriggers) > 0 {
		p.meta.SupportsTriggers = append([]string(nil), hs.SupportsTriggers...)
	}
	if len(hs.SupportsResources) > 0 {
		p.meta.SupportsResources = append([]string(nil), hs.SupportsResources...)
	}

	p.proc.handshaked = true
	return nil
}

func (p *ExternalProviderAdapter) readResponseLocked(expectedID string) (*Response, error) {
	proc := p.proc
	if proc == nil {
		return nil, fmt.Errorf("external plugin %s internal error: nil process", p.executable)
	}
	sc := proc.scanner
	done := make(chan struct {
		ok   bool
		line []byte
		err  error
	}, 1)

	go func() {
		if sc.Scan() {
			line := append([]byte(nil), sc.Bytes()...)
			done <- struct {
				ok   bool
				line []byte
				err  error
			}{ok: true, line: line}
			return
		}
		done <- struct {
			ok   bool
			line []byte
			err  error
		}{ok: false, err: sc.Err()}
	}()

	select {
	case res := <-done:
		if !res.ok {
			// Process likely exited; collect exit error if possible.
			waitErr := proc.cmd.Wait()
			if waitErr != nil {
				return nil, fmt.Errorf(
					"external plugin %s exited unexpectedly (stderr: %s): %w",
					p.executable,
					limitString(proc.stderr.String(), maxStderrBytes),
					waitErr,
				)
			}
			if res.err != nil {
				return nil, fmt.Errorf("external plugin %s produced no response: %w", p.executable, res.err)
			}
			return nil, fmt.Errorf("external plugin %s produced no response on stdout", p.executable)
		}
		line := bytes.TrimSpace(res.line)
		if len(line) == 0 {
			return nil, fmt.Errorf("external plugin %s produced empty response on stdout", p.executable)
		}
		var resp Response
		if err := json.Unmarshal(line, &resp); err != nil {
			return nil, fmt.Errorf(
				"external plugin %s returned malformed JSON on stdout: %v (line: %s)",
				p.executable,
				err,
				limitString(string(line), 200),
			)
		}
		if resp.ID != "" && expectedID != "" && resp.ID != expectedID {
			// With sequential requests, mismatched IDs likely means corrupted stream.
			return nil, fmt.Errorf("external plugin %s response id mismatch: expected %s got %s", p.executable, expectedID, resp.ID)
		}
		return &resp, nil
	case <-time.After(p.timeout):
		_ = proc.cmd.Process.Kill()
		return nil, fmt.Errorf(
			"external plugin %s timed out after %s (stderr: %s)",
			p.executable,
			p.timeout,
			limitString(proc.stderr.String(), maxStderrBytes),
		)
	}
}

func (p *ExternalProviderAdapter) killProcessLocked() {
	if p.proc == nil {
		return
	}
	proc := p.proc
	p.proc = nil
	if p.idleTimer != nil {
		p.idleTimer.Stop()
		p.idleTimer = nil
	}
	// Close stdin and stdout to unblock any waiting reads.
	if proc.stdin != nil {
		_ = proc.stdin.Close()
	}
	if proc.stdout != nil {
		_ = proc.stdout.Close()
	}
	if proc.cmd != nil && proc.cmd.Process != nil {
		_ = proc.cmd.Process.Kill()
	}
	_ = proc.cmd.Wait()
}

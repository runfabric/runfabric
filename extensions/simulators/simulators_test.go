package simulators

import (
	"strings"
	"testing"
)

// TestDockerInvokeArgs asserts the hardened `docker run` argv for the sandboxed
// invoke path: no network, read-only rootfs, all caps dropped, no-new-privileges,
// and ONLY the job's workDir mounted read-only at /work. This is the isolation
// contract for running tenant handler code out-of-process (invoke-local).
func TestDockerInvokeArgs(t *testing.T) {
	env := []string{"RF_EVENT={}", "RF_HANDLER=handler.handler", "RF_WORKDIR=/work"}
	args := dockerInvokeArgs("/tmp/rf-invoke-local-abc", "node:20", env)

	joined := strings.Join(args, " ")
	mustContain := []string{
		"run --rm",
		"--network none",
		"--read-only",
		"--cap-drop ALL",
		"--security-opt no-new-privileges",
		"--pids-limit 256",
		"--memory 512m",
		"--cpus 1",
		"-v /tmp/rf-invoke-local-abc:/work:ro",
		"-w /work",
		"-e RF_HANDLER=handler.handler",
		"-e RF_WORKDIR=/work",
	}
	for _, want := range mustContain {
		if !strings.Contains(joined, want) {
			t.Errorf("docker args missing %q\n  got: %s", want, joined)
		}
	}

	// Must end by executing the node runner with the given image.
	if args[0] != "run" {
		t.Errorf("first arg = %q, want run", args[0])
	}
	n := len(args)
	if args[n-3] != "node" || args[n-2] != "-e" || args[n-1] != nodeRunner {
		t.Errorf("argv must end with `node -e <nodeRunner>`, got tail: %v", args[n-3:])
	}
	// The image must sit immediately before the command.
	if args[n-4] != "node:20" {
		t.Errorf("image should precede the command, got %q", args[n-4])
	}
	// Never mount anything but the one job dir, and never writable.
	if strings.Contains(joined, ":/work:rw") || strings.Contains(joined, "/workspaces") {
		t.Errorf("workspace must be mounted read-only and scoped to the job dir; got: %s", joined)
	}
}

func TestInvokeSandboxImageDefault(t *testing.T) {
	t.Setenv("RUNFABRIC_INVOKE_SANDBOX_IMAGE", "")
	if got := invokeSandboxImage(); !strings.Contains(got, "node") {
		t.Errorf("default sandbox image should contain node, got %q", got)
	}
	t.Setenv("RUNFABRIC_INVOKE_SANDBOX_IMAGE", "my/custom:1")
	if got := invokeSandboxImage(); got != "my/custom:1" {
		t.Errorf("image override not honored, got %q", got)
	}
}

func TestInvokeSandboxModeDefaultOff(t *testing.T) {
	t.Setenv("RUNFABRIC_INVOKE_SANDBOX", "")
	if invokeSandboxMode() != "" {
		t.Errorf("sandbox must default off (in-process) when unset")
	}
	t.Setenv("RUNFABRIC_INVOKE_SANDBOX", "Docker")
	if invokeSandboxMode() != "docker" {
		t.Errorf("sandbox mode should normalize case to docker")
	}
}

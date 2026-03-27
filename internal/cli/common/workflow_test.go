package common_test

import (
	"bytes"
	"encoding/json"
	"testing"

	rootcli "github.com/runfabric/runfabric/internal/cli"
)

func TestWorkflowRunStatusReplayCancel_JSON(t *testing.T) {
	dir := t.TempDir()
	cfg := `service: workflow-cli
provider:
  name: aws-lambda
  runtime: nodejs
functions:
  - name: api
    entry: index.handler
workflows:
  - name: hello-flow
    steps:
      - function: s1
      - function: s2
`
	cfgPath := writeConfig(t, dir, cfg)

	root := rootcli.NewRootCmd()
	runOut := &bytes.Buffer{}
	root.SetOut(runOut)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"workflow", "run", "-c", cfgPath, "--name", "hello-flow", "--json", "--input", `{"ticket":"A-1"}`})
	if err := root.Execute(); err != nil {
		t.Fatalf("workflow run should succeed: %v", err)
	}
	runEnv := decodeEnvelope(t, runOut.Bytes())
	if runEnv["ok"] != true {
		t.Fatalf("workflow run ok=%v", runEnv["ok"])
	}
	runResult, _ := runEnv["result"].(map[string]any)
	runObj, _ := runResult["run"].(map[string]any)
	runID, _ := runObj["runId"].(string)
	if runID == "" {
		t.Fatalf("expected runId in workflow run output: %+v", runObj)
	}

	root = rootcli.NewRootCmd()
	statusOut := &bytes.Buffer{}
	root.SetOut(statusOut)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"workflow", "status", "-c", cfgPath, "--run-id", runID, "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("workflow status should succeed: %v", err)
	}
	statusEnv := decodeEnvelope(t, statusOut.Bytes())
	if statusEnv["ok"] != true {
		t.Fatalf("workflow status ok=%v", statusEnv["ok"])
	}

	root = rootcli.NewRootCmd()
	replayOut := &bytes.Buffer{}
	root.SetOut(replayOut)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"workflow", "replay", "-c", cfgPath, "--run-id", runID, "--from-step", "s2", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("workflow replay should succeed: %v", err)
	}
	replayEnv := decodeEnvelope(t, replayOut.Bytes())
	if replayEnv["ok"] != true {
		t.Fatalf("workflow replay ok=%v", replayEnv["ok"])
	}

	root = rootcli.NewRootCmd()
	cancelOut := &bytes.Buffer{}
	root.SetOut(cancelOut)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"workflow", "cancel", "-c", cfgPath, "--run-id", runID, "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("workflow cancel should succeed: %v", err)
	}
	cancelEnv := decodeEnvelope(t, cancelOut.Bytes())
	if cancelEnv["ok"] != true {
		t.Fatalf("workflow cancel ok=%v", cancelEnv["ok"])
	}
}

func decodeEnvelope(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var env map[string]any
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("decode envelope: %v\nraw=%s", err, string(data))
	}
	return env
}

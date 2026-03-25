package aws

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
	planner "github.com/runfabric/runfabric/platform/planner/engine"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

type functionChangeSet struct {
	CodeChanged      bool
	ConfigChanged    bool
	NeedsCreate      bool
	NeedsUpdate      bool
	Reason           string
	RemoteCodeChange bool
	RemoteCfgChange  bool
}

func buildConfigSignature(fn config.FunctionConfig) (string, error) {
	payload := map[string]any{
		"runtime":      fn.Runtime,
		"handler":      fn.Handler,
		"memory":       fn.Memory,
		"timeout":      fn.Timeout,
		"architecture": fn.Architecture,
		"environment":  environmentMap(fn),
		"layers":       fn.Layers,
		"tags":         fn.Tags,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal config signature payload: %w", err)
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

func buildReceiptFunctionMap(receipt *state.Receipt) map[string]state.FunctionDeployment {
	out := map[string]state.FunctionDeployment{}
	if receipt == nil {
		return out
	}
	for _, fn := range receipt.Functions {
		out[fn.Function] = fn
	}
	return out
}

func localHexToAWSBase64(hexHash string) (string, error) {
	raw, err := hex.DecodeString(hexHash)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

func detectFunctionChange(
	functionName string,
	artifact sdkprovider.Artifact,
	desired planner.DesiredFunction,
	actual *planner.ActualFunction,
	receiptMap map[string]state.FunctionDeployment,
) functionChangeSet {
	if actual == nil {
		return functionChangeSet{
			NeedsCreate: true,
			Reason:      "function does not exist remotely",
		}
	}

	codeChanged := false
	configChanged := false
	remoteCodeChanged := false
	remoteCfgChanged := false

	awsHash, err := localHexToAWSBase64(artifact.SHA256)
	if err != nil {
		codeChanged = true
		remoteCodeChanged = true
	} else if actual.CodeSHA256 != awsHash {
		codeChanged = true
		remoteCodeChanged = true
	}

	if actual.Runtime != desired.Runtime ||
		actual.Handler != desired.Handler ||
		int(actual.Memory) != desired.Memory ||
		int(actual.Timeout) != desired.Timeout {
		configChanged = true
		remoteCfgChanged = true
	}

	prev, ok := receiptMap[functionName]
	if ok {
		if prev.ArtifactSHA256 != artifact.SHA256 && !remoteCodeChanged {
			codeChanged = true
		}
		if prev.ConfigSignature != artifact.ConfigSignature && !remoteCfgChanged {
			configChanged = true
		}
	} else {
		if !remoteCodeChanged && !remoteCfgChanged {
			codeChanged = true
			configChanged = true
		}
	}

	if codeChanged || configChanged {
		reason := "change detected"
		switch {
		case codeChanged && configChanged:
			reason = "code and config drift/change detected"
		case codeChanged:
			reason = "code drift/change detected"
		case configChanged:
			reason = "config drift/change detected"
		}

		return functionChangeSet{
			CodeChanged:      codeChanged,
			ConfigChanged:    configChanged,
			NeedsUpdate:      true,
			Reason:           reason,
			RemoteCodeChange: remoteCodeChanged,
			RemoteCfgChange:  remoteCfgChanged,
		}
	}

	return functionChangeSet{
		CodeChanged:      false,
		ConfigChanged:    false,
		NeedsCreate:      false,
		NeedsUpdate:      false,
		Reason:           "remote and local state match",
		RemoteCodeChange: false,
		RemoteCfgChange:  false,
	}
}

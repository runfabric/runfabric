package external

import (
	"context"
	"strings"

	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	simulatorcontracts "github.com/runfabric/runfabric/platform/core/contracts/simulators"
)

// ExternalSimulatorAdapter implements simulatorcontracts.Simulator over the
// external plugin stdio protocol.
type ExternalSimulatorAdapter struct {
	id     string
	meta   simulatorcontracts.Meta
	client *ExternalProviderAdapter
}

func NewExternalSimulatorAdapter(id, executable string, meta simulatorcontracts.Meta) *ExternalSimulatorAdapter {
	normalizedID := strings.TrimSpace(id)
	if strings.TrimSpace(meta.ID) == "" {
		meta.ID = normalizedID
	}
	clientMeta := providers.ProviderMeta{Name: normalizedID}
	return &ExternalSimulatorAdapter{
		id:     normalizedID,
		meta:   meta,
		client: NewExternalProviderAdapter(normalizedID, executable, clientMeta),
	}
}

func (s *ExternalSimulatorAdapter) Meta() simulatorcontracts.Meta {
	meta := s.meta
	if strings.TrimSpace(meta.ID) == "" {
		meta.ID = strings.TrimSpace(s.id)
	}
	return meta
}

func (s *ExternalSimulatorAdapter) Simulate(ctx context.Context, req simulatorcontracts.Request) (*simulatorcontracts.Response, error) {
	_ = ctx // Request/response over stdio currently does not carry cancellation.
	var out simulatorcontracts.Response
	err := s.client.call("Simulate", map[string]any{
		"service":    req.Service,
		"stage":      req.Stage,
		"function":   req.Function,
		"method":     req.Method,
		"path":       req.Path,
		"query":      req.Query,
		"headers":    req.Headers,
		"body":       req.Body,
		"workDir":    req.WorkDir,
		"handlerRef": req.HandlerRef,
		"runtime":    req.Runtime,
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

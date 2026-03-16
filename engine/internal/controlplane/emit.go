package controlplane

import "github.com/runfabric/runfabric/engine/internal/observability"

func EmitEvent(eventType, service, stage, message string, metadata map[string]string) {
	_ = observability.Emit(&observability.Event{
		Type:      eventType,
		Service:   service,
		Stage:     stage,
		Message:   message,
		Timestamp: observability.Now(),
		Metadata:  metadata,
	})
}

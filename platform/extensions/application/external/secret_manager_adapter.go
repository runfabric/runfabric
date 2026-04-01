package external

import (
	"context"
	"fmt"
	"strings"

	providers "github.com/runfabric/runfabric/internal/provider/contracts"
)

// ExternalSecretManagerAdapter resolves secret-manager references through an
// external plugin executable over the standard extension protocol.
type ExternalSecretManagerAdapter struct {
	id     string
	client *ExternalProviderAdapter
}

func NewExternalSecretManagerAdapter(id, executable string) *ExternalSecretManagerAdapter {
	normalizedID := strings.TrimSpace(id)
	clientMeta := providers.ProviderMeta{Name: normalizedID}
	return &ExternalSecretManagerAdapter{
		id:     normalizedID,
		client: NewExternalProviderAdapter(normalizedID, executable, clientMeta),
	}
}

func (a *ExternalSecretManagerAdapter) ResolveSecret(ctx context.Context, ref string) (string, error) {
	_ = ctx // External adapter protocol is request/response over stdio and currently does not carry cancellation.
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("secret reference is required")
	}

	raw, err := a.callSecretMethod("ResolveSecret", ref)
	if err != nil {
		// Compatibility fallback for plugins that expose GetSecret.
		raw, err = a.callSecretMethod("GetSecret", ref)
		if err != nil {
			return "", err
		}
	}

	value, ok := parseSecretValue(raw)
	if !ok {
		return "", fmt.Errorf("secret manager plugin %q returned unsupported response for %q", a.id, ref)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("secret manager plugin %q returned an empty value for %q", a.id, ref)
	}
	return value, nil
}

func (a *ExternalSecretManagerAdapter) callSecretMethod(method, ref string) (any, error) {
	var raw any
	if err := a.client.call(method, map[string]any{"ref": ref}, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func parseSecretValue(raw any) (string, bool) {
	switch v := raw.(type) {
	case string:
		return v, true
	case map[string]any:
		for _, key := range []string{"value", "secret", "data"} {
			if candidate, ok := v[key].(string); ok {
				return candidate, true
			}
		}
	}
	return "", false
}

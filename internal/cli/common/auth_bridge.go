package common

import "context"

// RegistryTokenFromAuthStore returns a registry token when available.
// This is a compatibility hook for commands that previously resolved tokens from auth state.
func RegistryTokenFromAuthStore(ctx context.Context, authURL string) (string, error) {
	_ = ctx
	_ = authURL
	rc := LoadRunfabricrc()
	if rc.RegistryToken != "" {
		return rc.RegistryToken, nil
	}
	return "", nil
}

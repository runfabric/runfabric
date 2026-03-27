package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/runfabric/runfabric/registry/internal/store"
)

type identity struct {
	SubjectID   string
	TenantID    string
	Roles       []string
	AuthType    string
	IsAnonymous bool
}

func anonymousIdentity() identity {
	return identity{
		SubjectID:   "anonymous",
		AuthType:    "anonymous",
		IsAnonymous: true,
		Roles:       []string{"anonymous"},
	}
}

func (s *Server) resolveIdentity(r *http.Request) (identity, error) {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth == "" {
		return anonymousIdentity(), nil
	}
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return s.resolveBearerIdentity(strings.TrimSpace(auth[len("Bearer "):]))
	}
	if strings.HasPrefix(strings.ToLower(auth), "apikey ") {
		raw := strings.TrimSpace(auth[len("ApiKey "):])
		rec, err := s.store.FindAPIKey(raw)
		if err != nil {
			return identity{}, fmt.Errorf("invalid api key")
		}
		subjectID := strings.TrimSpace(rec.UserID)
		if subjectID == "" {
			subjectID = "apikey:" + strings.TrimSpace(rec.ID)
		}
		return authenticatedIdentity(subjectID, strings.TrimSpace(rec.TenantID), normalizeRoles(rec.Roles), "api_key")
	}
	return identity{}, fmt.Errorf("unsupported authorization scheme")
}

func (s *Server) resolveBearerIdentity(token string) (identity, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return identity{}, fmt.Errorf("missing bearer token")
	}
	switch {
	case strings.EqualFold(token, "local-dev-token"):
		return authenticatedIdentity("runfabric-dev", "tenant_runfabric", []string{"admin", "publisher", "reader"}, "jwt")
	case strings.HasPrefix(strings.ToLower(token), "official-"):
		return authenticatedIdentity("runfabric-official", "tenant_runfabric", []string{"publisher", "reader"}, "jwt")
	}

	claims, err := s.parseVerifiedJWTClaims(token)
	if err != nil {
		return identity{}, fmt.Errorf("invalid bearer token")
	}
	sub := claimStringByPaths(claims, s.oidcSubjectClaim, "sub", "user_id", "subject")
	tenant := claimStringByPaths(claims, s.oidcTenantClaim, "tenant_id")
	if sub == "" || tenant == "" {
		return identity{}, fmt.Errorf("missing required identity claims")
	}
	return authenticatedIdentity(sub, tenant, s.claimRoles(claims), "jwt")
}

func authenticatedIdentity(subjectID, tenantID string, roles []string, authType string) (identity, error) {
	subjectID = strings.TrimSpace(subjectID)
	tenantID = strings.TrimSpace(tenantID)
	authType = strings.TrimSpace(authType)
	if subjectID == "" || tenantID == "" {
		return identity{}, fmt.Errorf("missing required identity claims")
	}
	return identity{
		SubjectID:   subjectID,
		TenantID:    tenantID,
		Roles:       normalizeRoles(roles),
		AuthType:    authType,
		IsAnonymous: false,
	}, nil
}

func parseJWTClaims(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed jwt")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func firstClaimString(claims map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := claims[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func claimStringByPaths(claims map[string]any, configured string, fallbacks ...string) string {
	paths := parseClaimPaths(configured)
	if len(paths) == 0 {
		paths = append(paths, fallbacks...)
	}
	for _, p := range paths {
		raw, ok := claimValueByPath(claims, p)
		if !ok {
			continue
		}
		if v, ok := raw.(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func parseClaimPaths(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		path := strings.TrimSpace(p)
		if path != "" {
			out = append(out, path)
		}
	}
	return out
}

func claimValueByPath(claims map[string]any, path string) (any, bool) {
	path = strings.TrimSpace(path)
	if path == "" || claims == nil {
		return nil, false
	}
	// First try exact key to support namespaced claims like https://tenant.example.com/id.
	if v, ok := claims[path]; ok {
		return v, true
	}
	parts := strings.Split(path, ".")
	if len(parts) == 1 {
		return nil, false
	}
	current := any(claims)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := m[part]
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

func extractRolesFromValue(raw any) []string {
	switch typed := raw.(type) {
	case []any:
		out := make([]string, 0, len(typed))
		for _, v := range typed {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.ToLower(strings.TrimSpace(s)))
			}
		}
		return dedupeRoles(out)
	case []string:
		out := make([]string, 0, len(typed))
		for _, v := range typed {
			if strings.TrimSpace(v) != "" {
				out = append(out, strings.ToLower(strings.TrimSpace(v)))
			}
		}
		return dedupeRoles(out)
	case string:
		value := strings.TrimSpace(typed)
		if value == "" {
			return nil
		}
		fields := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
			return r == ' ' || r == ','
		})
		return dedupeRoles(fields)
	default:
		return nil
	}
}

func scopeToRoles(claims map[string]any) []string {
	scopeString := ""
	if raw, ok := claimValueByPath(claims, "scope"); ok {
		if s, ok := raw.(string); ok {
			scopeString = s
		}
	}
	if strings.TrimSpace(scopeString) == "" {
		if raw, ok := claimValueByPath(claims, "scp"); ok {
			switch typed := raw.(type) {
			case string:
				scopeString = typed
			case []any:
				items := make([]string, 0, len(typed))
				for _, v := range typed {
					if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
						items = append(items, strings.TrimSpace(s))
					}
				}
				scopeString = strings.Join(items, " ")
			}
		}
	}
	scopeString = strings.TrimSpace(scopeString)
	if scopeString == "" {
		return nil
	}
	scopes := strings.Fields(scopeString)
	roles := []string{"reader"}
	for _, scope := range scopes {
		switch strings.ToLower(strings.TrimSpace(scope)) {
		case "registry:write":
			roles = append(roles, "publisher")
		case "registry:admin":
			roles = append(roles, "admin", "publisher")
		}
	}
	return dedupeRoles(roles)
}

func (s *Server) claimRoles(claims map[string]any) []string {
	for _, mode := range s.oidcRoleModes {
		switch mode {
		case "roles":
			if raw, ok := claimValueByPath(claims, defaultString(s.oidcRolesClaim, "roles")); ok {
				if roles := extractRolesFromValue(raw); len(roles) > 0 {
					return roles
				}
			}
		case "realm_access.roles":
			if raw, ok := claimValueByPath(claims, "realm_access.roles"); ok {
				if roles := extractRolesFromValue(raw); len(roles) > 0 {
					return roles
				}
			}
		case "scope":
			if roles := scopeToRoles(claims); len(roles) > 0 {
				return roles
			}
		default:
			if !strings.HasPrefix(mode, "resource_access.") || !strings.HasSuffix(mode, ".roles") {
				continue
			}
			path := mode
			if strings.Contains(path, "<client>") {
				clientID := strings.TrimSpace(s.oidcRoleClientID)
				if clientID == "" {
					clientID = strings.TrimSpace(s.oidcAudience)
				}
				if clientID == "" {
					continue
				}
				path = strings.ReplaceAll(path, "<client>", clientID)
			}
			if raw, ok := claimValueByPath(claims, path); ok {
				if roles := extractRolesFromValue(raw); len(roles) > 0 {
					return roles
				}
			}
		}
	}
	return []string{"reader"}
}

func normalizeRoles(in []string) []string {
	if len(in) == 0 {
		return []string{"reader"}
	}
	out := make([]string, 0, len(in))
	for _, r := range in {
		r = strings.ToLower(strings.TrimSpace(r))
		if r != "" {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return []string{"reader"}
	}
	return dedupeRoles(out)
}

func dedupeRoles(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, r := range in {
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}

func hasAnyRole(id identity, roles ...string) bool {
	if id.IsAnonymous {
		return false
	}
	allowed := map[string]bool{}
	for _, r := range roles {
		allowed[strings.ToLower(strings.TrimSpace(r))] = true
	}
	for _, r := range id.Roles {
		if allowed[strings.ToLower(strings.TrimSpace(r))] {
			return true
		}
	}
	return false
}

func actorFromIdentity(id identity) string {
	if id.IsAnonymous {
		return "anonymous"
	}
	if strings.TrimSpace(id.SubjectID) == "" {
		return id.AuthType
	}
	return id.SubjectID
}

func publisherFromIdentity(id identity) string {
	if strings.EqualFold(strings.TrimSpace(id.TenantID), "tenant_runfabric") {
		return "runfabric"
	}
	return "community"
}

func packageVisibilityFromString(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case store.VisibilityPublic:
		return store.VisibilityPublic
	case store.VisibilityTenant:
		return store.VisibilityTenant
	default:
		return ""
	}
}

package server

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const (
	objectPackage  = "package"
	objectRegistry = "registry"

	actionPackageRead             = "package:read"
	actionPackagePublish          = "package:publish"
	actionPackageDelete           = "package:delete"
	actionPackageManageVisibility = "package:manage_visibility"
	actionRegistryAuditRead       = "registry:audit:read"
)

type policyEngine struct {
	// allow[role][object][action] = true/false
	allow map[string]map[string]map[string]bool
	// bind[subject][tenant][role] = true
	bind map[string]map[string]map[string]bool
}

func newPolicyEngine(path string) (*policyEngine, error) {
	engine := &policyEngine{
		allow: defaultPolicyAllowMap(),
		bind:  defaultTenantBindings(),
	}
	if strings.TrimSpace(path) == "" {
		return engine, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	parsedAllow := map[string]map[string]map[string]bool{}
	parsedBindings := map[string]map[string]map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 4 {
			continue
		}
		kind := strings.ToLower(strings.TrimSpace(parts[0]))
		switch kind {
		case "p":
			// Supported formats:
			// p, role, action, allow|deny
			// p, role, object, action, allow|deny
			role := strings.ToLower(strings.TrimSpace(parts[1]))
			object := "*"
			action := ""
			effect := ""
			if len(parts) >= 5 {
				object = strings.ToLower(strings.TrimSpace(parts[2]))
				action = strings.ToLower(strings.TrimSpace(parts[3]))
				effect = strings.ToLower(strings.TrimSpace(parts[4]))
			} else {
				action = strings.ToLower(strings.TrimSpace(parts[2]))
				effect = strings.ToLower(strings.TrimSpace(parts[3]))
			}
			if role == "" || action == "" {
				continue
			}
			if object == "" {
				object = "*"
			}
			if parsedAllow[role] == nil {
				parsedAllow[role] = map[string]map[string]bool{}
			}
			if parsedAllow[role][object] == nil {
				parsedAllow[role][object] = map[string]bool{}
			}
			parsedAllow[role][object][action] = effect == "allow"
		case "g":
			// g, subject, role, tenant
			if len(parts) < 4 {
				continue
			}
			subject := strings.TrimSpace(parts[1])
			role := strings.ToLower(strings.TrimSpace(parts[2]))
			tenant := strings.TrimSpace(parts[3])
			if subject == "" || role == "" || tenant == "" {
				continue
			}
			if parsedBindings[subject] == nil {
				parsedBindings[subject] = map[string]map[string]bool{}
			}
			if parsedBindings[subject][tenant] == nil {
				parsedBindings[subject][tenant] = map[string]bool{}
			}
			parsedBindings[subject][tenant][role] = true
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(parsedAllow) > 0 {
		engine.allow = parsedAllow
	}
	if len(parsedBindings) > 0 {
		engine.bind = parsedBindings
	}
	return engine, nil
}

func defaultPolicyAllowMap() map[string]map[string]map[string]bool {
	return map[string]map[string]map[string]bool{
		"admin": {
			objectPackage: {
				actionPackageRead:             true,
				actionPackagePublish:          true,
				actionPackageDelete:           true,
				actionPackageManageVisibility: true,
			},
			objectRegistry: {
				actionRegistryAuditRead: true,
			},
		},
		"publisher": {
			objectPackage: {
				actionPackageRead:             true,
				actionPackagePublish:          true,
				actionPackageManageVisibility: true,
			},
		},
		"reader": {
			objectPackage: {
				actionPackageRead: true,
			},
		},
		"anonymous": {
			objectPackage: {
				actionPackageRead: true,
			},
		},
	}
}

func defaultTenantBindings() map[string]map[string]map[string]bool {
	return map[string]map[string]map[string]bool{
		"runfabric-dev": {
			"tenant_runfabric": {
				"admin": true,
			},
		},
		"runfabric-official": {
			"tenant_runfabric": {
				"publisher": true,
			},
		},
		"ci-bot": {
			"tenant_runfabric": {
				"admin": true,
			},
		},
	}
}

func (p *policyEngine) boundRoles(subject, tenant string) []string {
	if p == nil {
		return nil
	}
	subject = strings.TrimSpace(subject)
	tenant = strings.TrimSpace(tenant)
	if subject == "" || tenant == "" {
		return nil
	}
	tenants, ok := p.bind[subject]
	if !ok {
		return nil
	}
	rolesForTenant, ok := tenants[tenant]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(rolesForTenant))
	for role := range rolesForTenant {
		out = append(out, role)
	}
	return out
}

func (p *policyEngine) subjectHasBindings(subject string) bool {
	if p == nil {
		return false
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return false
	}
	tenants, ok := p.bind[subject]
	if !ok {
		return false
	}
	return len(tenants) > 0
}

func (p *policyEngine) allowRole(role, object, action string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	object = strings.ToLower(strings.TrimSpace(object))
	action = strings.ToLower(strings.TrimSpace(action))
	if role == "" || action == "" {
		return false
	}
	objs, ok := p.allow[role]
	if !ok {
		return false
	}
	if object != "" {
		if acts, ok := objs[object]; ok {
			if allowed, ok := acts[action]; ok && allowed {
				return true
			}
		}
	}
	if acts, ok := objs["*"]; ok {
		if allowed, ok := acts[action]; ok && allowed {
			return true
		}
	}
	return false
}

func (p *policyEngine) Allow(id identity, object, action string) bool {
	if p == nil {
		return false
	}
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "" {
		return false
	}
	roles := p.boundRoles(id.SubjectID, id.TenantID)
	if len(roles) == 0 {
		// If this subject is explicitly tenant-bound in policy, no binding for the current
		// tenant means deny-by-default instead of trusting token-provided roles.
		if p.subjectHasBindings(id.SubjectID) {
			return false
		}
		roles = id.Roles
	}
	for _, role := range roles {
		if p.allowRole(role, object, action) {
			return true
		}
	}
	return false
}

func (s *Server) authorize(id identity, object, action string) error {
	if s.policy == nil {
		return fmt.Errorf("policy engine unavailable")
	}
	if !s.policy.Allow(id, object, action) {
		return fmt.Errorf("forbidden")
	}
	return nil
}

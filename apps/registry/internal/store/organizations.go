package store

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

var errOrganizationsUnsupported = errors.New("organizations are not supported on this metadata backend yet")

// Organization roles, npm-style: owners manage the org and members, admins
// manage members + publish, developers publish.
const (
	OrgRoleOwner     = "owner"
	OrgRoleAdmin     = "admin"
	OrgRoleDeveloper = "developer"
)

// Organization is an npm-style org that owns a package namespace/scope and has
// members with roles.
type Organization struct {
	ID          string      `json:"id"`
	Slug        string      `json:"slug"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Visibility  string      `json:"defaultVisibility,omitempty"` // public | tenant
	CreatedBy   string      `json:"createdBy"`
	CreatedAt   string      `json:"createdAt"`
	UpdatedAt   string      `json:"updatedAt"`
	Members     []OrgMember `json:"members"`
}

// OrgMember is a user's membership + role within an organization.
type OrgMember struct {
	UserID string `json:"userId"`
	Email  string `json:"email,omitempty"`
	Role   string `json:"role"`
}

type CreateOrganizationInput struct {
	Slug           string
	Name           string
	Description    string
	Visibility     string
	CreatedBy      string
	CreatedByEmail string
}

type ListOrganizationsInput struct {
	// When set, only organizations the user is a member of are returned.
	MemberUserID string
}

type OrgMemberInput struct {
	Slug        string
	UserID      string
	Email       string
	Role        string
	ActorUserID string
}

func normalizeOrgRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case OrgRoleOwner:
		return OrgRoleOwner
	case OrgRoleAdmin:
		return OrgRoleAdmin
	case OrgRoleDeveloper, "member", "":
		return OrgRoleDeveloper
	default:
		return ""
	}
}

// slug: lowercase, alphanumeric + dashes (like an npm scope name).
func normalizeOrgSlug(slug string) string {
	s := strings.ToLower(strings.TrimSpace(slug))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else if r == ' ' || r == '_' {
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func cloneOrganization(org *Organization) *Organization {
	if org == nil {
		return nil
	}
	cp := *org
	cp.Members = append([]OrgMember(nil), org.Members...)
	return &cp
}

func (org *Organization) memberRole(userID string) string {
	userID = strings.TrimSpace(userID)
	for _, m := range org.Members {
		if m.UserID == userID {
			return m.Role
		}
	}
	return ""
}

// --- Store facade: routes to the active metadata backend ---

func (s *Store) CreateOrganization(in CreateOrganizationInput) (*Organization, error) {
	repo, err := s.metadataRepo()
	if err != nil {
		return nil, err
	}
	return repo.CreateOrganization(in)
}

func (s *Store) ListOrganizations(in ListOrganizationsInput) ([]*Organization, error) {
	repo, err := s.metadataRepo()
	if err != nil {
		return nil, err
	}
	return repo.ListOrganizations(in)
}

func (s *Store) GetOrganization(slug string) (*Organization, error) {
	repo, err := s.metadataRepo()
	if err != nil {
		return nil, err
	}
	return repo.GetOrganization(normalizeOrgSlug(slug))
}

func (s *Store) AddOrganizationMember(in OrgMemberInput) (*Organization, error) {
	repo, err := s.metadataRepo()
	if err != nil {
		return nil, err
	}
	return repo.AddOrganizationMember(in)
}

func (s *Store) RemoveOrganizationMember(slug, userID string) (*Organization, error) {
	repo, err := s.metadataRepo()
	if err != nil {
		return nil, err
	}
	return repo.RemoveOrganizationMember(normalizeOrgSlug(slug), strings.TrimSpace(userID))
}

// --- JSON (in-memory + file) implementation ---

func (s *Store) createOrganizationJSON(in CreateOrganizationInput) (*Organization, error) {
	slug := normalizeOrgSlug(in.Slug)
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = slug
	}
	createdBy := strings.TrimSpace(in.CreatedBy)
	if slug == "" || createdBy == "" {
		return nil, fmt.Errorf("slug and actor are required")
	}
	visibility := normalizeVisibility(in.Visibility)
	if visibility == "" {
		visibility = VisibilityPublic
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Organizations[slug] != nil {
		return nil, fmt.Errorf("organization already exists")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	org := &Organization{
		ID:          fmt.Sprintf("org_%d", time.Now().UTC().UnixNano()),
		Slug:        slug,
		Name:        name,
		Description: strings.TrimSpace(in.Description),
		Visibility:  visibility,
		CreatedBy:   createdBy,
		CreatedAt:   now,
		UpdatedAt:   now,
		Members: []OrgMember{
			{UserID: createdBy, Email: strings.TrimSpace(in.CreatedByEmail), Role: OrgRoleOwner},
		},
	}
	s.data.Organizations[slug] = org
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return cloneOrganization(org), nil
}

func (s *Store) listOrganizationsJSON(in ListOrganizationsInput) ([]*Organization, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	member := strings.TrimSpace(in.MemberUserID)
	out := make([]*Organization, 0, len(s.data.Organizations))
	for _, org := range s.data.Organizations {
		if member != "" && org.memberRole(member) == "" {
			continue
		}
		out = append(out, cloneOrganization(org))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

func (s *Store) getOrganizationJSON(slug string) (*Organization, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	org := s.data.Organizations[slug]
	if org == nil {
		return nil, fmt.Errorf("organization not found")
	}
	return cloneOrganization(org), nil
}

func (s *Store) addOrganizationMemberJSON(in OrgMemberInput) (*Organization, error) {
	slug := normalizeOrgSlug(in.Slug)
	userID := strings.TrimSpace(in.UserID)
	role := normalizeOrgRole(in.Role)
	if slug == "" || userID == "" {
		return nil, fmt.Errorf("slug and user are required")
	}
	if role == "" {
		return nil, fmt.Errorf("role must be owner, admin, or developer")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	org := s.data.Organizations[slug]
	if org == nil {
		return nil, fmt.Errorf("organization not found")
	}
	updated := false
	for i := range org.Members {
		if org.Members[i].UserID == userID {
			org.Members[i].Role = role
			if strings.TrimSpace(in.Email) != "" {
				org.Members[i].Email = strings.TrimSpace(in.Email)
			}
			updated = true
			break
		}
	}
	if !updated {
		org.Members = append(org.Members, OrgMember{UserID: userID, Email: strings.TrimSpace(in.Email), Role: role})
	}
	org.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return cloneOrganization(org), nil
}

func (s *Store) removeOrganizationMemberJSON(slug, userID string) (*Organization, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	org := s.data.Organizations[slug]
	if org == nil {
		return nil, fmt.Errorf("organization not found")
	}
	kept := org.Members[:0]
	removed := false
	owners := 0
	for _, m := range org.Members {
		if m.Role == OrgRoleOwner {
			owners++
		}
	}
	for _, m := range org.Members {
		if m.UserID == userID {
			// Don't allow removing the last owner.
			if m.Role == OrgRoleOwner && owners <= 1 {
				return nil, fmt.Errorf("cannot remove the last owner")
			}
			removed = true
			continue
		}
		kept = append(kept, m)
	}
	if !removed {
		return nil, fmt.Errorf("member not found")
	}
	org.Members = append([]OrgMember(nil), kept...)
	org.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return cloneOrganization(org), nil
}

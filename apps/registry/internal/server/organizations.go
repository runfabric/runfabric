package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/runfabric/runfabric/registry/internal/store"
)

// GET  /v1/orgs           list the caller's organizations
// POST /v1/orgs           create an organization (caller becomes owner)
func (s *Server) handleOrgsRoot(w http.ResponseWriter, r *http.Request) {
	id, ok := s.identityOrWriteError(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		if id.IsAnonymous {
			writeJSON(w, http.StatusOK, map[string]any{"items": []any{}})
			return
		}
		orgs, err := s.store.ListOrganizations(store.ListOrganizationsInput{MemberUserID: id.SubjectID})
		if err != nil {
			s.orgError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": orgs})
	case http.MethodPost:
		if id.IsAnonymous {
			s.unauthorized(w, r, "sign in to create an organization")
			return
		}
		var body struct {
			Slug        string `json:"slug"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Visibility  string `json:"visibility"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			s.badRequest(w, r, "invalid JSON body")
			return
		}
		org, err := s.store.CreateOrganization(store.CreateOrganizationInput{
			Slug:        body.Slug,
			Name:        body.Name,
			Description: body.Description,
			Visibility:  body.Visibility,
			CreatedBy:   id.SubjectID,
		})
		if err != nil {
			s.orgError(w, r, err)
			return
		}
		s.audit(r, "org_create", "ok", map[string]any{"slug": org.Slug})
		writeJSON(w, http.StatusCreated, org)
	default:
		writeAPIError(w, r, http.StatusMethodNotAllowed, apiError{Code: "METHOD_NOT_ALLOWED", Message: "method not allowed", RequestID: requestIDFromRequest(r)})
	}
}

// /v1/orgs/{slug}
// /v1/orgs/{slug}/members
// /v1/orgs/{slug}/members/{userId}
func (s *Server) handleOrgRoutes(w http.ResponseWriter, r *http.Request) {
	id, ok := s.identityOrWriteError(w, r)
	if !ok {
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/v1/orgs/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeAPIError(w, r, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "not found", RequestID: requestIDFromRequest(r)})
		return
	}
	slug := parts[0]

	// /v1/orgs/{slug}
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			writeAPIError(w, r, http.StatusMethodNotAllowed, apiError{Code: "METHOD_NOT_ALLOWED", Message: "method not allowed", RequestID: requestIDFromRequest(r)})
			return
		}
		org, err := s.store.GetOrganization(slug)
		if err != nil {
			s.orgError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, org)
		return
	}

	// /v1/orgs/{slug}/members ...
	if parts[1] == "members" {
		s.handleOrgMembers(w, r, id, slug, parts[2:])
		return
	}

	writeAPIError(w, r, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "not found", RequestID: requestIDFromRequest(r)})
}

func (s *Server) handleOrgMembers(w http.ResponseWriter, r *http.Request, id identity, slug string, tail []string) {
	org, err := s.store.GetOrganization(slug)
	if err != nil {
		s.orgError(w, r, err)
		return
	}
	callerRole := orgMemberRole(org, id.SubjectID)

	// GET /v1/orgs/{slug}/members
	if len(tail) == 0 && r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{"items": org.Members})
		return
	}

	// Managing members requires owner/admin of the org.
	if callerRole != store.OrgRoleOwner && callerRole != store.OrgRoleAdmin {
		s.forbidden(w, r, "only organization owners and admins can manage members")
		return
	}

	// POST /v1/orgs/{slug}/members
	if len(tail) == 0 && r.Method == http.MethodPost {
		var body struct {
			UserID string `json:"userId"`
			Email  string `json:"email"`
			Role   string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			s.badRequest(w, r, "invalid JSON body")
			return
		}
		updated, err := s.store.AddOrganizationMember(store.OrgMemberInput{
			Slug:        slug,
			UserID:      body.UserID,
			Email:       body.Email,
			Role:        body.Role,
			ActorUserID: id.SubjectID,
		})
		if err != nil {
			s.orgError(w, r, err)
			return
		}
		s.audit(r, "org_member_add", "ok", map[string]any{"slug": slug, "member": body.UserID})
		writeJSON(w, http.StatusOK, updated)
		return
	}

	// DELETE /v1/orgs/{slug}/members/{userId}
	if len(tail) == 1 && r.Method == http.MethodDelete {
		updated, err := s.store.RemoveOrganizationMember(slug, tail[0])
		if err != nil {
			s.orgError(w, r, err)
			return
		}
		s.audit(r, "org_member_remove", "ok", map[string]any{"slug": slug, "member": tail[0]})
		writeJSON(w, http.StatusOK, updated)
		return
	}

	writeAPIError(w, r, http.StatusMethodNotAllowed, apiError{Code: "METHOD_NOT_ALLOWED", Message: "method not allowed", RequestID: requestIDFromRequest(r)})
}

func orgMemberRole(org *store.Organization, userID string) string {
	userID = strings.TrimSpace(userID)
	for _, m := range org.Members {
		if m.UserID == userID {
			return m.Role
		}
	}
	return ""
}

func (s *Server) badRequest(w http.ResponseWriter, r *http.Request, msg string) {
	writeAPIError(w, r, http.StatusBadRequest, apiError{Code: "BAD_REQUEST", Message: msg, RequestID: requestIDFromRequest(r)})
}

// orgError maps store errors to appropriate HTTP statuses.
func (s *Server) orgError(w http.ResponseWriter, r *http.Request, err error) {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "not found"):
		writeAPIError(w, r, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: msg, RequestID: requestIDFromRequest(r)})
	case strings.Contains(msg, "already exists"):
		writeAPIError(w, r, http.StatusConflict, apiError{Code: "CONFLICT", Message: msg, RequestID: requestIDFromRequest(r)})
	case strings.Contains(msg, "required") || strings.Contains(msg, "must be") || strings.Contains(msg, "cannot remove") || strings.Contains(msg, "not supported"):
		writeAPIError(w, r, http.StatusBadRequest, apiError{Code: "BAD_REQUEST", Message: msg, RequestID: requestIDFromRequest(r)})
	default:
		writeAPIError(w, r, http.StatusInternalServerError, apiError{Code: "INTERNAL", Message: "organization request failed", Details: map[string]any{"cause": msg}, RequestID: requestIDFromRequest(r)})
	}
}

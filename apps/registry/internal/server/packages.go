package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/runfabric/runfabric/registry/internal/store"
)

func (s *Server) handlePackagesRoot(w http.ResponseWriter, r *http.Request) {
	id, ok := s.identityOrWriteError(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		if s.readCachedJSON(w, r, "packages_list") {
			return
		}
		if id.IsAnonymous && !s.allowAnonymousRead {
			s.unauthorized(w, r, "anonymous reads are disabled")
			return
		}
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		ns := strings.TrimSpace(r.URL.Query().Get("namespace"))
		items, err := s.store.ListVisiblePackages(store.PackageFilter{
			TenantID:      id.TenantID,
			IncludePublic: true,
			PublicOnly:    id.IsAnonymous,
			Namespace:     ns,
			Query:         q,
		})
		if err != nil {
			writeAPIError(w, r, http.StatusInternalServerError, apiError{Code: "INTERNAL", Message: "failed to list packages", Details: map[string]any{"cause": err.Error()}, RequestID: requestIDFromRequest(r)})
			s.audit(r, "packages_list", "error", map[string]any{"cause": err.Error()})
			return
		}
		resp := make([]map[string]any, 0, len(items))
		for _, p := range items {
			resp = append(resp, map[string]any{
				"id":             p.ID,
				"tenant_id":      p.TenantID,
				"namespace":      p.Namespace,
				"name":           p.Name,
				"latest_version": p.LatestVersion,
				"visibility":     p.Visibility,
				"created_by":     p.CreatedBy,
				"created_at":     p.CreatedAt,
				"updated_at":     p.UpdatedAt,
			})
		}
		s.writeAndCacheJSON(w, r, http.StatusOK, map[string]any{"items": resp, "count": len(resp), "anonymous": id.IsAnonymous}, "packages_list", 20*time.Second)
		s.audit(r, "packages_list", "ok", map[string]any{"count": len(resp), "actor": actorFromIdentity(id)})
	case http.MethodPost:
		if id.IsAnonymous {
			s.unauthorized(w, r, "authentication required")
			return
		}
		if err := s.authorize(id, objectPackage, actionPackagePublish); err != nil {
			s.forbidden(w, r, "insufficient role: requires admin or publisher")
			return
		}
		var req struct {
			Namespace  string `json:"namespace"`
			Name       string `json:"name"`
			Visibility string `json:"visibility"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, r, http.StatusBadRequest, apiError{Code: "INVALID_REQUEST", Message: "invalid JSON body", Details: map[string]any{"cause": err.Error()}, RequestID: requestIDFromRequest(r)})
			return
		}
		pkg, err := s.store.CreatePackage(store.CreatePackageInput{
			TenantID:   id.TenantID,
			Namespace:  req.Namespace,
			Name:       req.Name,
			Visibility: req.Visibility,
			CreatedBy:  id.SubjectID,
		})
		if err != nil {
			writeAPIError(w, r, http.StatusBadRequest, apiError{Code: "VALIDATION_FAILED", Message: err.Error(), RequestID: requestIDFromRequest(r)})
			s.audit(r, "package_create", "error", map[string]any{"namespace": req.Namespace, "name": req.Name, "cause": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, pkg)
		s.invalidateCacheScope()
		s.audit(r, "package_create", "ok", map[string]any{"id": pkg.ID, "namespace": pkg.Namespace, "name": pkg.Name, "tenant": pkg.TenantID})
	default:
		s.methodNotAllowed(w, r)
	}
}

func (s *Server) handlePackagesRoutes(w http.ResponseWriter, r *http.Request) {
	id, ok := s.identityOrWriteError(w, r)
	if !ok {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/packages/")
	parts := splitPath(path)
	if len(parts) < 2 {
		s.handleNotFound(w, r)
		return
	}
	namespace, _ := url.PathUnescape(parts[0])
	name, _ := url.PathUnescape(parts[1])
	if namespace == "" || name == "" {
		s.handleNotFound(w, r)
		return
	}

	if len(parts) == 2 {
		s.handlePackageRootByName(w, r, id, namespace, name)
		return
	}
	if len(parts) >= 3 && parts[2] == "versions" {
		s.handlePackageVersionsRoutes(w, r, id, namespace, name, parts[3:])
		return
	}
	s.handleNotFound(w, r)
}

func (s *Server) handlePackageRootByName(w http.ResponseWriter, r *http.Request, id identity, namespace, name string) {
	switch r.Method {
	case http.MethodGet:
		if s.readCachedJSON(w, r, "package_get") {
			return
		}
		if id.IsAnonymous && !s.allowAnonymousRead {
			s.unauthorized(w, r, "anonymous reads are disabled")
			return
		}
		pkg, err := s.store.GetVisiblePackage(id.TenantID, namespace, name, true)
		if err != nil {
			writeAPIError(w, r, http.StatusNotFound, apiError{Code: "PACKAGE_NOT_FOUND", Message: "package not found", Details: map[string]any{"namespace": namespace, "name": name}, RequestID: requestIDFromRequest(r)})
			s.audit(r, "package_get", "error", map[string]any{"namespace": namespace, "name": name})
			return
		}
		versions, _ := s.store.ListPackageVersions(pkg.ID)
		resp := map[string]any{
			"package":  pkg,
			"versions": versions,
		}
		s.writeAndCacheJSON(w, r, http.StatusOK, resp, "package_get", 20*time.Second)
		s.audit(r, "package_get", "ok", map[string]any{"id": pkg.ID, "tenant": pkg.TenantID})
	case http.MethodPatch:
		if id.IsAnonymous {
			s.unauthorized(w, r, "authentication required")
			return
		}
		if err := s.authorize(id, objectPackage, actionPackageManageVisibility); err != nil {
			s.forbidden(w, r, "insufficient role: requires admin")
			return
		}
		var req struct {
			Visibility string `json:"visibility"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, r, http.StatusBadRequest, apiError{Code: "INVALID_REQUEST", Message: "invalid JSON body", Details: map[string]any{"cause": err.Error()}, RequestID: requestIDFromRequest(r)})
			return
		}
		pkg, err := s.store.UpdatePackageVisibility(store.UpdatePackageVisibilityInput{
			TenantID:   id.TenantID,
			Namespace:  namespace,
			Name:       name,
			Visibility: req.Visibility,
		})
		if err != nil {
			writeAPIError(w, r, http.StatusBadRequest, apiError{Code: "VALIDATION_FAILED", Message: err.Error(), RequestID: requestIDFromRequest(r)})
			s.audit(r, "package_patch", "error", map[string]any{"namespace": namespace, "name": name, "cause": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, pkg)
		s.invalidateCacheScope()
		s.audit(r, "package_patch", "ok", map[string]any{"id": pkg.ID, "visibility": pkg.Visibility})
	case http.MethodDelete:
		if id.IsAnonymous {
			s.unauthorized(w, r, "authentication required")
			return
		}
		if err := s.authorize(id, objectPackage, actionPackageDelete); err != nil {
			s.forbidden(w, r, "insufficient role: requires admin")
			return
		}
		if err := s.store.DeletePackage(store.DeletePackageInput{
			TenantID:  id.TenantID,
			Namespace: namespace,
			Name:      name,
		}); err != nil {
			writeAPIError(w, r, http.StatusBadRequest, apiError{Code: "VALIDATION_FAILED", Message: err.Error(), RequestID: requestIDFromRequest(r)})
			s.audit(r, "package_delete", "error", map[string]any{"namespace": namespace, "name": name, "cause": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
		s.invalidateCacheScope()
		s.audit(r, "package_delete", "ok", map[string]any{"namespace": namespace, "name": name, "tenant": id.TenantID})
	default:
		s.methodNotAllowed(w, r)
	}
}

func (s *Server) handlePackageVersionsRoutes(w http.ResponseWriter, r *http.Request, id identity, namespace, name string, rest []string) {
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			if s.readCachedJSON(w, r, "package_versions") {
				return
			}
			if id.IsAnonymous && !s.allowAnonymousRead {
				s.unauthorized(w, r, "anonymous reads are disabled")
				return
			}
			pkg, err := s.store.GetVisiblePackage(id.TenantID, namespace, name, true)
			if err != nil {
				writeAPIError(w, r, http.StatusNotFound, apiError{Code: "PACKAGE_NOT_FOUND", Message: "package not found", RequestID: requestIDFromRequest(r)})
				return
			}
			versions, err := s.store.ListPackageVersions(pkg.ID)
			if err != nil {
				writeAPIError(w, r, http.StatusInternalServerError, apiError{Code: "INTERNAL", Message: "failed to list versions", Details: map[string]any{"cause": err.Error()}, RequestID: requestIDFromRequest(r)})
				return
			}
			s.writeAndCacheJSON(w, r, http.StatusOK, map[string]any{"package": pkg, "versions": versions}, "package_versions", 20*time.Second)
		case http.MethodPost:
			if id.IsAnonymous {
				s.unauthorized(w, r, "authentication required")
				return
			}
			if err := s.authorize(id, objectPackage, actionPackagePublish); err != nil {
				s.forbidden(w, r, "insufficient role: requires admin or publisher")
				return
			}
			pkg, err := s.store.GetVisiblePackage(id.TenantID, namespace, name, false)
			if err != nil {
				writeAPIError(w, r, http.StatusNotFound, apiError{Code: "PACKAGE_NOT_FOUND", Message: "package not found", RequestID: requestIDFromRequest(r)})
				return
			}
			if pkg.TenantID != id.TenantID {
				s.forbidden(w, r, "tenant mismatch")
				return
			}
			var req struct {
				Version     string         `json:"version"`
				Manifest    map[string]any `json:"manifest_json"`
				ArtifactKey string         `json:"artifact_key"`
				Checksum    string         `json:"checksum"`
				SizeBytes   int64          `json:"size_bytes"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeAPIError(w, r, http.StatusBadRequest, apiError{Code: "INVALID_REQUEST", Message: "invalid JSON body", Details: map[string]any{"cause": err.Error()}, RequestID: requestIDFromRequest(r)})
				return
			}
			if req.Version == "" {
				writeAPIError(w, r, http.StatusBadRequest, apiError{Code: "VALIDATION_FAILED", Message: "version is required", RequestID: requestIDFromRequest(r)})
				return
			}
			rec, err := s.store.PublishPackageVersion(store.PublishPackageVersionInput{
				TenantID:    id.TenantID,
				Namespace:   namespace,
				Name:        name,
				Version:     req.Version,
				Manifest:    req.Manifest,
				ArtifactKey: req.ArtifactKey,
				Checksum:    req.Checksum,
				SizeBytes:   req.SizeBytes,
				PublishedBy: id.SubjectID,
			})
			if err != nil {
				writeAPIError(w, r, http.StatusBadRequest, apiError{Code: "VALIDATION_FAILED", Message: err.Error(), RequestID: requestIDFromRequest(r)})
				return
			}
			writeJSON(w, http.StatusCreated, rec)
			s.invalidateCacheScope()
			s.audit(r, "package_publish_version", "ok", map[string]any{"packageId": pkg.ID, "version": rec.Version, "tenant": rec.TenantID})
		default:
			s.methodNotAllowed(w, r)
		}
		return
	}

	version, _ := url.PathUnescape(rest[0])
	if strings.TrimSpace(version) == "" {
		s.handleNotFound(w, r)
		return
	}
	pkg, err := s.store.GetVisiblePackage(id.TenantID, namespace, name, true)
	if err != nil {
		writeAPIError(w, r, http.StatusNotFound, apiError{Code: "PACKAGE_NOT_FOUND", Message: "package not found", RequestID: requestIDFromRequest(r)})
		return
	}
	rec, err := s.store.GetPackageVersion(pkg.ID, version)
	if err != nil {
		writeAPIError(w, r, http.StatusNotFound, apiError{Code: "VERSION_NOT_FOUND", Message: "version not found", RequestID: requestIDFromRequest(r)})
		return
	}
	expectedArtifactKey := packageArtifactKeyFor(pkg.TenantID, pkg.Namespace, pkg.Name, rec.Version)

	if len(rest) == 1 {
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w, r)
			return
		}
		if s.readCachedJSON(w, r, "package_version_get") {
			return
		}
		if id.IsAnonymous && !s.allowAnonymousRead && pkg.Visibility == store.VisibilityPublic {
			s.unauthorized(w, r, "anonymous reads are disabled")
			return
		}
		s.writeAndCacheJSON(w, r, http.StatusOK, map[string]any{"package": pkg, "version": rec}, "package_version_get", 20*time.Second)
		s.audit(r, "package_version_get", "ok", map[string]any{"packageId": pkg.ID, "version": rec.Version})
		return
	}

	switch rest[1] {
	case "upload-url":
		if r.Method != http.MethodPost {
			s.methodNotAllowed(w, r)
			return
		}
		if id.IsAnonymous {
			s.unauthorized(w, r, "authentication required")
			return
		}
		if err := s.authorize(id, objectPackage, actionPackagePublish); err != nil {
			s.forbidden(w, r, "insufficient role: requires admin or publisher")
			return
		}
		if pkg.TenantID != id.TenantID {
			s.forbidden(w, r, "tenant mismatch")
			return
		}
		if strings.TrimSpace(rec.ArtifactKey) != expectedArtifactKey {
			s.forbidden(w, r, "artifact key is outside tenant package boundary")
			return
		}
		expires := 15 * time.Minute
		u := s.presignArtifactURL(r, rec.ArtifactKey, http.MethodPut, expires)
		if su, ok := s.presignS3(rec.ArtifactKey, http.MethodPut, expires); ok {
			u = su
		} else if strings.TrimSpace(s.s3BaseURL) != "" {
			u = s.s3BaseURL + "/" + url.PathEscape(rec.ArtifactKey)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"method":      http.MethodPut,
			"url":         u,
			"artifactKey": rec.ArtifactKey,
			"expiresIn":   int(expires.Seconds()),
		})
		s.audit(r, "artifact_upload_url", "ok", map[string]any{"artifactKey": rec.ArtifactKey, "packageId": pkg.ID, "version": rec.Version})
	case "download-url":
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w, r)
			return
		}
		if id.IsAnonymous && !s.allowAnonymousRead {
			s.unauthorized(w, r, "anonymous reads are disabled")
			return
		}
		if strings.TrimSpace(rec.ArtifactKey) != expectedArtifactKey {
			s.forbidden(w, r, "artifact key is outside tenant package boundary")
			return
		}
		expires := 15 * time.Minute
		u := s.presignArtifactURL(r, rec.ArtifactKey, http.MethodGet, expires)
		if su, ok := s.presignS3(rec.ArtifactKey, http.MethodGet, expires); ok {
			u = su
		} else if strings.TrimSpace(s.s3BaseURL) != "" {
			u = s.s3BaseURL + "/" + url.PathEscape(rec.ArtifactKey)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"method":      http.MethodGet,
			"url":         u,
			"artifactKey": rec.ArtifactKey,
			"expiresIn":   int(expires.Seconds()),
		})
		s.audit(r, "artifact_download_url", "ok", map[string]any{"artifactKey": rec.ArtifactKey, "packageId": pkg.ID, "version": rec.Version})
	default:
		s.handleNotFound(w, r)
	}
}

func splitPath(path string) []string {
	raw := strings.Split(strings.TrimSpace(path), "/")
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (s *Server) identityOrWriteError(w http.ResponseWriter, r *http.Request) (identity, bool) {
	id, err := s.resolveIdentity(r)
	if err == nil {
		return id, true
	}
	s.unauthorized(w, r, err.Error())
	return identity{}, false
}

func (s *Server) unauthorized(w http.ResponseWriter, r *http.Request, msg string) {
	writeAPIError(w, r, http.StatusUnauthorized, apiError{
		Code:      "UNAUTHORIZED",
		Message:   msg,
		RequestID: requestIDFromRequest(r),
	})
}

func (s *Server) forbidden(w http.ResponseWriter, r *http.Request, msg string) {
	writeAPIError(w, r, http.StatusForbidden, apiError{
		Code:      "FORBIDDEN",
		Message:   msg,
		RequestID: requestIDFromRequest(r),
	})
}

func (s *Server) presignArtifactURL(r *http.Request, artifactKey, method string, ttl time.Duration) string {
	exp := strconv.FormatInt(time.Now().UTC().Add(ttl).Unix(), 10)
	sig := s.signArtifactToken(artifactKey, method, exp)
	path := "/artifacts/" + url.PathEscape(strings.TrimSpace(artifactKey))
	return absoluteURL(r, path) + "?method=" + url.QueryEscape(strings.ToUpper(strings.TrimSpace(method))) + "&exp=" + exp + "&sig=" + sig
}

func packageArtifactKeyFor(tenantID, namespace, name, version string) string {
	return fmt.Sprintf(
		"tenants/%s/packages/%s/%s/%s/artifact.tar.gz",
		strings.TrimSpace(tenantID),
		strings.ToLower(strings.TrimSpace(namespace)),
		strings.ToLower(strings.TrimSpace(name)),
		strings.TrimSpace(version),
	)
}

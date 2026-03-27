package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/runfabric/runfabric/registry/internal/store"
)

type Options struct {
	Store                 *store.Store
	WebDir                string
	UIAuthURL             string
	UIDocsURL             string
	AllowAnonymousRead    bool
	ArtifactSigningSecret string
	RedisAddr             string
	OIDCIssuer            string
	OIDCAudience          string
	OIDCJWKSURL           string
	OIDCSubjectClaim      string
	OIDCTenantClaim       string
	OIDCRolesClaim        string
	OIDCRoleModes         string
	OIDCRoleClientID      string
	OIDCAudienceMode      string
	OIDCAllowedJWTAlgs    string
	CasbinPolicyPath      string
	S3BaseURL             string
	S3Bucket              string
	S3Region              string
	S3Endpoint            string
	S3AccessKeyID         string
	S3SecretAccessKey     string
	S3SessionToken        string
}

type Server struct {
	store                 *store.Store
	webDir                string
	webEnabled            bool
	webIndexPath          string
	uiAuthURL             string
	uiDocsURL             string
	limiter               *rateLimiter
	allowAnonymousRead    bool
	artifactSigningSecret string
	cache                 responseCache
	cacheEpoch            atomic.Int64
	oidcIssuer            string
	oidcAudience          string
	oidcAudienceMode      string
	oidcJWKSURL           string
	oidcSubjectClaim      string
	oidcTenantClaim       string
	oidcRolesClaim        string
	oidcRoleModes         []string
	oidcRoleClientID      string
	oidcAllowedJWTAlgs    map[string]bool
	oidcDiscovery         *oidcDiscoveryCache
	jwksCache             *jwksCache
	policy                *policyEngine
	s3BaseURL             string
	s3Presigner           *s3Presigner
}

func New(opts Options) (*Server, error) {
	if opts.Store == nil {
		return nil, fmt.Errorf("server store is required")
	}
	secret := strings.TrimSpace(opts.ArtifactSigningSecret)
	if secret == "" {
		secret = "runfabric-local-artifact-signing-secret"
	}
	policy, err := newPolicyEngine(strings.TrimSpace(opts.CasbinPolicyPath))
	if err != nil {
		return nil, fmt.Errorf("load policy engine: %w", err)
	}
	webDir := strings.TrimSpace(opts.WebDir)
	webEnabled := false
	webIndexPath := ""
	if webDir != "" {
		indexPath := filepath.Join(webDir, "index.html")
		if info, statErr := os.Stat(indexPath); statErr == nil && !info.IsDir() {
			webEnabled = true
			webIndexPath = indexPath
		} else {
			log.Printf("registry web ui disabled: index not found at %s", indexPath)
		}
	}
	return &Server{
		store:                 opts.Store,
		webDir:                webDir,
		webEnabled:            webEnabled,
		webIndexPath:          webIndexPath,
		uiAuthURL:             strings.TrimSpace(opts.UIAuthURL),
		uiDocsURL:             strings.TrimSpace(opts.UIDocsURL),
		limiter:               newRateLimiter(),
		allowAnonymousRead:    opts.AllowAnonymousRead,
		artifactSigningSecret: secret,
		cache:                 newLayeredCache(opts.RedisAddr),
		oidcIssuer:            strings.TrimSpace(opts.OIDCIssuer),
		oidcAudience:          strings.TrimSpace(opts.OIDCAudience),
		oidcAudienceMode:      normalizeAudienceMode(opts.OIDCAudienceMode),
		oidcJWKSURL:           strings.TrimSpace(opts.OIDCJWKSURL),
		oidcSubjectClaim:      defaultString(strings.TrimSpace(opts.OIDCSubjectClaim), "sub"),
		oidcTenantClaim:       defaultString(strings.TrimSpace(opts.OIDCTenantClaim), "tenant_id"),
		oidcRolesClaim:        defaultString(strings.TrimSpace(opts.OIDCRolesClaim), "roles"),
		oidcRoleModes:         parseRoleModes(opts.OIDCRoleModes),
		oidcRoleClientID:      strings.TrimSpace(opts.OIDCRoleClientID),
		oidcAllowedJWTAlgs:    parseAllowedJWTAlgorithms(opts.OIDCAllowedJWTAlgs),
		oidcDiscovery:         newOIDCDiscoveryCache(),
		jwksCache:             newJWKSCache(),
		policy:                policy,
		s3BaseURL:             strings.TrimRight(strings.TrimSpace(opts.S3BaseURL), "/"),
		s3Presigner:           newS3Presigner(opts),
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/ready", s.handleHealthz)
	mux.HandleFunc("/v1/ui/config", s.handleUIConfig)
	mux.HandleFunc("/v1/extensions/resolve", s.handleResolve)
	mux.HandleFunc("/v1/extensions/list", s.handleList)
	mux.HandleFunc("/v1/extensions/search", s.handleSearch)
	mux.HandleFunc("/v1/extensions/publish/init", s.handlePublishInit)
	mux.HandleFunc("/v1/extensions/publish/finalize", s.handlePublishFinalize)
	mux.HandleFunc("/v1/extensions/", s.handleExtensionRoutes)
	mux.HandleFunc("/v1/publish/", s.handlePublishStatus)
	mux.HandleFunc("/v1/uploads/", s.handleUploads)
	mux.HandleFunc("/v1/audit", s.handleAudit)
	mux.HandleFunc("/packages", s.handlePackagesRoot)
	mux.HandleFunc("/packages/", s.handlePackagesRoutes)
	mux.HandleFunc("/artifacts/", s.handleArtifacts)
	mux.HandleFunc("/bin", s.handleDeterministicBin)
	mux.HandleFunc("/", s.handleFrontendOrNotFound)

	return logRequests(withRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.allowRequest(w, r) {
			return
		}
		mux.ServeHTTP(w, r)
	})))
}

func (s *Server) allowRequest(w http.ResponseWriter, r *http.Request) bool {
	limit, window, group := classifyRateLimit(r)
	if limit <= 0 {
		return true
	}
	actor := bearerToken(r)
	if actor == "" {
		actor = remoteAddrIP(r)
	}
	ok, resetAt, remaining := s.limiter.Allow(group+":"+actor, limit, window)
	if ok {
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		w.Header().Set("X-RateLimit-Reset", resetAt.Format(time.RFC3339))
		return true
	}
	writeAPIError(w, r, http.StatusTooManyRequests, apiError{
		Code:      "RATE_LIMITED",
		Message:   "rate limit exceeded",
		Details:   map[string]any{"group": group, "limit": limit, "windowSeconds": int(window.Seconds())},
		Hint:      "retry after the rate limit window resets",
		DocsURL:   "https://runfabric.cloud/docs/extensions/registry#rate-limits",
		RequestID: requestIDFromRequest(r),
	})
	s.audit(r, "rate_limit", "rejected", map[string]any{"group": group, "limit": limit})
	return false
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeAPIError(w, r, http.StatusNotFound, apiError{
		Code:      "NOT_FOUND",
		Message:   "Not found",
		RequestID: requestIDFromRequest(r),
	})
	s.audit(r, "not_found", "error", nil)
}

func (s *Server) methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	writeAPIError(w, r, http.StatusMethodNotAllowed, apiError{
		Code:      "INVALID_REQUEST",
		Message:   "method not allowed",
		Details:   map[string]any{"method": r.Method},
		RequestID: requestIDFromRequest(r),
	})
	s.audit(r, "method_not_allowed", "error", map[string]any{"method": r.Method})
}

func (s *Server) handleResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r)
		return
	}
	if s.readCachedJSON(w, r, "extensions_resolve") {
		return
	}
	q := r.URL.Query()
	id := strings.TrimSpace(q.Get("id"))
	core := strings.TrimSpace(q.Get("core"))
	goos := strings.TrimSpace(q.Get("os"))
	arch := strings.TrimSpace(q.Get("arch"))
	version := strings.TrimSpace(q.Get("version"))
	if id == "" || core == "" || goos == "" || arch == "" {
		missing := []string{}
		if id == "" {
			missing = append(missing, "id")
		}
		if core == "" {
			missing = append(missing, "core")
		}
		if goos == "" {
			missing = append(missing, "os")
		}
		if arch == "" {
			missing = append(missing, "arch")
		}
		writeAPIError(w, r, http.StatusBadRequest, apiError{
			Code:      "INVALID_REQUEST",
			Message:   "missing required query parameters",
			Details:   map[string]any{"missing": missing},
			Hint:      "provide id, core, os, and arch query parameters",
			DocsURL:   "https://runfabric.cloud/docs/extensions/registry#resolve",
			RequestID: requestIDFromRequest(r),
		})
		s.audit(r, "resolve", "error", map[string]any{"missing": missing})
		return
	}
	resolved, err := s.store.Resolve(store.ResolveInput{ID: id, Core: core, OS: goos, Arch: arch, Version: version})
	if err != nil {
		writeAPIError(w, r, http.StatusNotFound, apiError{
			Code:      "EXTENSION_NOT_FOUND",
			Message:   "no compatible extension version found",
			Details:   map[string]any{"id": id, "core": core, "os": goos, "arch": arch, "version": version},
			Hint:      "check id/version or use /v1/extensions/search",
			DocsURL:   "https://runfabric.cloud/docs/extensions/search",
			RequestID: requestIDFromRequest(r),
		})
		s.audit(r, "resolve", "error", map[string]any{"id": id, "error": err.Error()})
		return
	}
	artifact := map[string]any{
		"type":      resolved.Artifact.Type,
		"format":    resolved.Artifact.Format,
		"url":       absoluteURL(r, resolved.Artifact.URL),
		"sizeBytes": resolved.Artifact.SizeBytes,
		"checksum": map[string]any{
			"algorithm": resolved.Artifact.ChecksumAlgorithm,
			"value":     resolved.Artifact.ChecksumValue,
		},
	}
	if resolved.Artifact.Signature != nil {
		artifact["signature"] = map[string]any{
			"algorithm":   resolved.Artifact.Signature.Algorithm,
			"value":       resolved.Artifact.Signature.Value,
			"publicKeyId": resolved.Artifact.Signature.PublicKeyID,
		}
	}
	resolvedPayload := map[string]any{
		"id":          resolved.Extension.ID,
		"name":        resolved.Extension.Name,
		"type":        resolved.Extension.Type,
		"pluginKind":  resolved.Extension.PluginKind,
		"version":     resolved.Version.Version,
		"publisher":   map[string]any{"id": resolved.Publisher.ID, "name": resolved.Publisher.Name, "verified": resolved.Publisher.Verified, "trust": resolved.Publisher.Trust},
		"description": resolved.Version.Description,
		"compatibility": func() map[string]any {
			if resolved.Version.Compatibility != nil {
				return resolved.Version.Compatibility
			}
			return map[string]any{"core": resolved.Version.CoreConstraint}
		}(),
		"permissions":  resolved.Version.Permissions,
		"capabilities": resolved.Version.Capabilities,
		"artifact":     artifact,
		"manifest":     withAbsoluteURLs(r, resolved.Version.Manifest),
		"integrity":    withAbsoluteURLs(r, resolved.Version.Integrity),
		"install":      resolved.Version.Install,
	}
	resp := map[string]any{
		"request":  map[string]any{"id": id, "core": core, "os": goos, "arch": arch},
		"resolved": resolvedPayload,
		"meta": map[string]any{
			"resolvedAt":      time.Now().UTC().Format(time.RFC3339),
			"registryVersion": "v1",
			"requestId":       requestIDFromRequest(r),
		},
	}
	s.writeAndCacheJSON(w, r, http.StatusOK, resp, "extensions_resolve", 60*time.Second)
	s.audit(r, "resolve", "ok", map[string]any{"id": resolved.Extension.ID, "version": resolved.Version.Version})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r)
		return
	}
	if s.readCachedJSON(w, r, "extensions_search") {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	typ := strings.TrimSpace(r.URL.Query().Get("type"))
	kind := strings.TrimSpace(r.URL.Query().Get("pluginKind"))
	page, _ := strconvAtoiDefault(r.URL.Query().Get("page"), 1)
	pageSize, _ := strconvAtoiDefault(r.URL.Query().Get("pageSize"), 20)
	out, err := s.store.Search(store.SearchInput{Query: q, Type: typ, PluginKind: kind, Page: page, PageSize: pageSize})
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, apiError{Code: "INTERNAL", Message: "search failed", Details: map[string]any{"cause": err.Error()}, RequestID: requestIDFromRequest(r)})
		s.audit(r, "search", "error", map[string]any{"cause": err.Error()})
		return
	}
	s.writeAndCacheJSON(w, r, http.StatusOK, out, "extensions_search", 20*time.Second)
	s.audit(r, "search", "ok", map[string]any{"q": q, "total": out.Total})
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r)
		return
	}
	if s.readCachedJSON(w, r, "extensions_list") {
		return
	}
	if strings.TrimSpace(r.URL.Query().Get("q")) != "" {
		writeAPIError(w, r, http.StatusBadRequest, apiError{
			Code:      "INVALID_REQUEST",
			Message:   "q is not supported for list",
			Details:   map[string]any{"unsupported": []string{"q"}},
			Hint:      "use /v1/extensions/search for text queries",
			RequestID: requestIDFromRequest(r),
		})
		s.audit(r, "list", "error", map[string]any{"cause": "unsupported q"})
		return
	}

	typ := strings.TrimSpace(r.URL.Query().Get("type"))
	kind := strings.TrimSpace(r.URL.Query().Get("pluginKind"))
	page, _ := strconvAtoiDefault(r.URL.Query().Get("page"), 1)
	pageSize, _ := strconvAtoiDefault(r.URL.Query().Get("pageSize"), 20)
	sortBy := strings.TrimSpace(r.URL.Query().Get("sortBy"))
	order := strings.TrimSpace(r.URL.Query().Get("order"))

	out, err := s.store.List(store.ListInput{Type: typ, PluginKind: kind, Page: page, PageSize: pageSize, SortBy: sortBy, Order: order})
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, apiError{Code: "INTERNAL", Message: "list failed", Details: map[string]any{"cause": err.Error()}, RequestID: requestIDFromRequest(r)})
		s.audit(r, "list", "error", map[string]any{"cause": err.Error()})
		return
	}

	s.writeAndCacheJSON(w, r, http.StatusOK, out, "extensions_list", 20*time.Second)
	s.audit(r, "list", "ok", map[string]any{"type": typ, "pluginKind": kind, "total": out.Total})
}

func (s *Server) handleExtensionRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/extensions/")
	if strings.TrimSpace(path) == "" {
		s.handleNotFound(w, r)
		return
	}
	parts := strings.Split(path, "/")
	id, _ := url.PathUnescape(parts[0])
	if id == "" {
		s.handleNotFound(w, r)
		return
	}
	if len(parts) == 1 {
		s.handleExtensionDetail(w, r, id)
		return
	}
	if parts[1] == "versions" {
		if len(parts) == 2 {
			s.handleExtensionVersions(w, r, id)
			return
		}
		if len(parts) == 3 {
			v, _ := url.PathUnescape(parts[2])
			s.handleExtensionVersionDetail(w, r, id, v)
			return
		}
	}
	if parts[1] == "advisories" && len(parts) == 2 {
		s.handleExtensionAdvisories(w, r, id)
		return
	}
	s.handleNotFound(w, r)
}

func (s *Server) handleExtensionDetail(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r)
		return
	}
	if s.readCachedJSON(w, r, "extension_detail") {
		return
	}
	ext, pub, err := s.store.GetExtension(id)
	if err != nil {
		writeAPIError(w, r, http.StatusNotFound, apiError{Code: "EXTENSION_NOT_FOUND", Message: "extension not found", Details: map[string]any{"id": id}, RequestID: requestIDFromRequest(r)})
		s.audit(r, "extension_detail", "error", map[string]any{"id": id})
		return
	}
	versions, _ := s.store.ListVersions(id)
	latest := ""
	if len(versions) > 0 {
		latest = versions[0].Version
	}
	resp := map[string]any{
		"id":          ext.ID,
		"name":        ext.Name,
		"type":        ext.Type,
		"pluginKind":  ext.PluginKind,
		"description": ext.Description,
		"aliases":     ext.Aliases,
		"publisher": map[string]any{
			"id":       pub.ID,
			"name":     pub.Name,
			"verified": pub.Verified,
			"trust":    pub.Trust,
		},
		"latestVersion": latest,
	}
	s.writeAndCacheJSON(w, r, http.StatusOK, resp, "extension_detail", 60*time.Second)
	s.audit(r, "extension_detail", "ok", map[string]any{"id": ext.ID})
}

func (s *Server) handleExtensionVersions(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r)
		return
	}
	if s.readCachedJSON(w, r, "extension_versions") {
		return
	}
	versions, err := s.store.ListVersions(id)
	if err != nil {
		writeAPIError(w, r, http.StatusNotFound, apiError{Code: "EXTENSION_NOT_FOUND", Message: "extension not found", Details: map[string]any{"id": id}, RequestID: requestIDFromRequest(r)})
		s.audit(r, "extension_versions", "error", map[string]any{"id": id})
		return
	}
	s.writeAndCacheJSON(w, r, http.StatusOK, map[string]any{"id": id, "versions": versions}, "extension_versions", 60*time.Second)
	s.audit(r, "extension_versions", "ok", map[string]any{"id": id, "count": len(versions)})
}

func (s *Server) handleExtensionVersionDetail(w http.ResponseWriter, r *http.Request, id, version string) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r)
		return
	}
	if s.readCachedJSON(w, r, "extension_version_detail") {
		return
	}
	v, ext, pub, err := s.store.GetVersion(id, version)
	if err != nil {
		writeAPIError(w, r, http.StatusNotFound, apiError{Code: "VERSION_NOT_FOUND", Message: "extension version not found", Details: map[string]any{"id": id, "version": version}, RequestID: requestIDFromRequest(r)})
		s.audit(r, "extension_version_detail", "error", map[string]any{"id": id, "version": version})
		return
	}
	arts := make([]map[string]any, 0, len(v.Artifacts))
	for _, a := range v.Artifacts {
		entry := map[string]any{
			"type":      a.Type,
			"format":    a.Format,
			"url":       absoluteURL(r, a.URL),
			"sizeBytes": a.SizeBytes,
			"checksum": map[string]any{
				"algorithm": a.ChecksumAlgorithm,
				"value":     a.ChecksumValue,
			},
			"os":   a.OS,
			"arch": a.Arch,
		}
		if a.Signature != nil {
			entry["signature"] = a.Signature
		}
		arts = append(arts, entry)
	}
	resp := map[string]any{
		"id":          ext.ID,
		"name":        ext.Name,
		"type":        ext.Type,
		"pluginKind":  ext.PluginKind,
		"version":     v.Version,
		"status":      v.ReleaseStatus,
		"description": v.Description,
		"publisher":   map[string]any{"id": pub.ID, "name": pub.Name, "verified": pub.Verified, "trust": pub.Trust},
		"compatibility": func() map[string]any {
			if v.Compatibility != nil {
				return v.Compatibility
			}
			return map[string]any{"core": v.CoreConstraint}
		}(),
		"capabilities": v.Capabilities,
		"permissions":  v.Permissions,
		"manifest":     withAbsoluteURLs(r, v.Manifest),
		"integrity":    withAbsoluteURLs(r, v.Integrity),
		"install":      v.Install,
		"artifact":     arts,
	}
	s.writeAndCacheJSON(w, r, http.StatusOK, resp, "extension_version_detail", 120*time.Second)
	s.audit(r, "extension_version_detail", "ok", map[string]any{"id": ext.ID, "version": v.Version})
}

func (s *Server) handleExtensionAdvisories(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r)
		return
	}
	if s.readCachedJSON(w, r, "extension_advisories") {
		return
	}
	advisories, err := s.store.ListAdvisories(id)
	if err != nil {
		writeAPIError(w, r, http.StatusNotFound, apiError{Code: "EXTENSION_NOT_FOUND", Message: "extension not found", Details: map[string]any{"id": id}, RequestID: requestIDFromRequest(r)})
		s.audit(r, "extension_advisories", "error", map[string]any{"id": id})
		return
	}
	s.writeAndCacheJSON(w, r, http.StatusOK, map[string]any{"id": id, "advisories": advisories}, "extension_advisories", 20*time.Second)
	s.audit(r, "extension_advisories", "ok", map[string]any{"id": id, "count": len(advisories)})
}

func (s *Server) handlePublishInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, r)
		return
	}
	id, ok := s.identityOrWriteError(w, r)
	if !ok {
		s.audit(r, "publish_init", "error", map[string]any{"reason": "unauthorized"})
		return
	}
	if id.IsAnonymous {
		s.unauthorized(w, r, "authentication required")
		s.audit(r, "publish_init", "error", map[string]any{"reason": "anonymous"})
		return
	}
	if err := s.authorize(id, objectPackage, actionPackagePublish); err != nil {
		s.forbidden(w, r, "insufficient role: requires admin or publisher")
		s.audit(r, "publish_init", "error", map[string]any{"reason": "forbidden", "roles": id.Roles})
		return
	}
	publisher := publisherFromIdentity(id)
	var req struct {
		Extension map[string]any `json:"extension"`
		Files     []struct {
			Key       string `json:"key"`
			Name      string `json:"name"`
			SizeBytes int64  `json:"sizeBytes"`
			Checksum  struct {
				Algorithm string `json:"algorithm"`
				Value     string `json:"value"`
			} `json:"checksum"`
		} `json:"files"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, apiError{Code: "INVALID_REQUEST", Message: "invalid JSON body", Details: map[string]any{"cause": err.Error()}, RequestID: requestIDFromRequest(r)})
		s.audit(r, "publish_init", "error", map[string]any{"cause": err.Error()})
		return
	}
	extID, _ := req.Extension["id"].(string)
	version, _ := req.Extension["version"].(string)
	typ, _ := req.Extension["type"].(string)
	pluginKind, _ := req.Extension["pluginKind"].(string)
	desc, _ := req.Extension["description"].(string)
	files := make([]store.PublishFileInput, 0, len(req.Files))
	for _, f := range req.Files {
		files = append(files, store.PublishFileInput{Key: f.Key, Name: f.Name, SizeBytes: f.SizeBytes, Algorithm: f.Checksum.Algorithm, Checksum: f.Checksum.Value})
	}
	session, err := s.store.CreatePublishSession(store.PublishInitInput{Publisher: publisher, ID: extID, Version: version, Type: typ, PluginKind: pluginKind, Description: desc, Files: files})
	if err != nil {
		writeAPIError(w, r, http.StatusBadRequest, apiError{Code: "VALIDATION_FAILED", Message: err.Error(), RequestID: requestIDFromRequest(r)})
		s.audit(r, "publish_init", "error", map[string]any{"id": extID, "version": version, "cause": err.Error()})
		return
	}
	keys := make([]string, 0, len(session.Files))
	for k := range session.Files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	uploads := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		uploads = append(uploads, map[string]any{"key": key, "method": http.MethodPut, "url": absoluteURL(r, "/v1/uploads/"+url.PathEscape(session.ID)+"/"+url.PathEscape(key))})
	}
	writeJSON(w, http.StatusOK, map[string]any{"publishId": session.ID, "status": session.Status, "uploads": uploads})
	s.audit(r, "publish_init", "ok", map[string]any{"publishId": session.ID, "id": extID, "version": version})
}

func (s *Server) handleUploads(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/v1/uploads/")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		writeAPIError(w, r, http.StatusBadRequest, apiError{Code: "INVALID_REQUEST", Message: "upload path must be /v1/uploads/{publishId}/{key}", RequestID: requestIDFromRequest(r)})
		s.audit(r, "publish_upload", "error", map[string]any{"path": r.URL.Path})
		return
	}
	publishID, _ := url.PathUnescape(parts[0])
	key, _ := url.PathUnescape(parts[1])
	if strings.Contains(key, "..") {
		writeAPIError(w, r, http.StatusBadRequest, apiError{Code: "INVALID_REQUEST", Message: "invalid upload key", RequestID: requestIDFromRequest(r)})
		return
	}
	switch r.Method {
	case http.MethodPut:
		body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024*1024))
		if err != nil {
			writeAPIError(w, r, http.StatusBadRequest, apiError{Code: "INVALID_REQUEST", Message: "failed to read upload body", Details: map[string]any{"cause": err.Error()}, RequestID: requestIDFromRequest(r)})
			s.audit(r, "publish_upload", "error", map[string]any{"publishId": publishID, "key": key, "cause": err.Error()})
			return
		}
		if err := s.store.UploadPublishFile(publishID, key, body); err != nil {
			writeAPIError(w, r, http.StatusBadRequest, apiError{Code: "VALIDATION_FAILED", Message: err.Error(), Details: map[string]any{"publishId": publishID, "key": key}, RequestID: requestIDFromRequest(r)})
			s.audit(r, "publish_upload", "error", map[string]any{"publishId": publishID, "key": key, "cause": err.Error()})
			return
		}
		w.WriteHeader(http.StatusOK)
		s.audit(r, "publish_upload", "ok", map[string]any{"publishId": publishID, "key": key, "sizeBytes": len(body)})
	case http.MethodGet:
		p := filepath.Join(s.store.UploadsDir(), publishID, key)
		b, err := os.ReadFile(p)
		if err != nil {
			writeAPIError(w, r, http.StatusNotFound, apiError{Code: "FILE_NOT_FOUND", Message: "upload file not found", Details: map[string]any{"publishId": publishID, "key": key}, RequestID: requestIDFromRequest(r)})
			s.audit(r, "upload_download", "error", map[string]any{"publishId": publishID, "key": key})
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(b)))
		_, _ = w.Write(b)
		s.audit(r, "upload_download", "ok", map[string]any{"publishId": publishID, "key": key, "sizeBytes": len(b)})
	default:
		s.methodNotAllowed(w, r)
	}
}

func (s *Server) handlePublishFinalize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, r)
		return
	}
	id, ok := s.identityOrWriteError(w, r)
	if !ok {
		s.audit(r, "publish_finalize", "error", map[string]any{"reason": "unauthorized"})
		return
	}
	if id.IsAnonymous {
		s.unauthorized(w, r, "authentication required")
		s.audit(r, "publish_finalize", "error", map[string]any{"reason": "anonymous"})
		return
	}
	if err := s.authorize(id, objectPackage, actionPackagePublish); err != nil {
		s.forbidden(w, r, "insufficient role: requires admin or publisher")
		s.audit(r, "publish_finalize", "error", map[string]any{"reason": "forbidden", "roles": id.Roles})
		return
	}
	var req struct {
		PublishID string `json:"publishId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, apiError{Code: "INVALID_REQUEST", Message: "invalid JSON body", Details: map[string]any{"cause": err.Error()}, RequestID: requestIDFromRequest(r)})
		s.audit(r, "publish_finalize", "error", map[string]any{"cause": err.Error()})
		return
	}
	session, err := s.store.FinalizePublish(req.PublishID)
	if err != nil {
		writeAPIError(w, r, http.StatusBadRequest, apiError{Code: "VALIDATION_FAILED", Message: err.Error(), Details: map[string]any{"publishId": req.PublishID}, RequestID: requestIDFromRequest(r)})
		s.audit(r, "publish_finalize", "error", map[string]any{"publishId": req.PublishID, "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"publishId": session.ID, "status": session.Status})
	s.invalidateCacheScope()
	s.audit(r, "publish_finalize", "ok", map[string]any{"publishId": session.ID})
}

func (s *Server) handlePublishStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r)
		return
	}
	publishID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/v1/publish/"))
	if publishID == "" {
		writeAPIError(w, r, http.StatusBadRequest, apiError{Code: "VALIDATION_FAILED", Message: "publish id is required", RequestID: requestIDFromRequest(r)})
		s.audit(r, "publish_status", "error", nil)
		return
	}
	session, err := s.store.PublishStatus(publishID)
	if err != nil {
		writeAPIError(w, r, http.StatusNotFound, apiError{Code: "PUBLISH_NOT_FOUND", Message: "publish session not found", Details: map[string]any{"publishId": publishID}, RequestID: requestIDFromRequest(r)})
		s.audit(r, "publish_status", "error", map[string]any{"publishId": publishID})
		return
	}
	files := make([]map[string]any, 0, len(session.Files))
	keys := make([]string, 0, len(session.Files))
	for k := range session.Files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		f := session.Files[key]
		files = append(files, map[string]any{"key": f.Key, "uploaded": f.Uploaded, "sizeBytes": f.UploadedSizeBytes})
	}
	writeJSON(w, http.StatusOK, map[string]any{"publishId": session.ID, "status": session.Status, "files": files, "updatedAt": session.UpdatedAt})
	s.audit(r, "publish_status", "ok", map[string]any{"publishId": session.ID, "status": session.Status})
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r)
		return
	}
	id, ok := s.identityOrWriteError(w, r)
	if !ok {
		return
	}
	if id.IsAnonymous || s.authorize(id, objectRegistry, actionRegistryAuditRead) != nil {
		s.forbidden(w, r, "insufficient role: requires admin")
		return
	}
	limit, _ := strconvAtoiDefault(r.URL.Query().Get("limit"), 100)
	all := s.store.ListAudit(limit)
	events := make([]store.AuditEvent, 0, len(all))
	for _, ev := range all {
		if strings.TrimSpace(ev.TenantID) == strings.TrimSpace(id.TenantID) {
			events = append(events, ev)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (s *Server) handleDeterministicBin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r)
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	version := strings.TrimSpace(r.URL.Query().Get("version"))
	goos := strings.TrimSpace(r.URL.Query().Get("os"))
	arch := strings.TrimSpace(r.URL.Query().Get("arch"))
	b := []byte(id + "@" + version + ":" + goos + "-" + arch)
	sum := sha256.Sum256(b)
	w.Header().Set("X-Checksum-SHA256", hex.EncodeToString(sum[:]))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}

func absoluteURL(r *http.Request, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err == nil && u.IsAbs() {
		return raw
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}
	return scheme + "://" + r.Host + raw
}

func withAbsoluteURLs(r *http.Request, in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := map[string]any{}
	for k, v := range in {
		if s, ok := v.(string); ok && (strings.HasPrefix(s, "/") || strings.HasPrefix(strings.ToLower(s), "http")) {
			out[k] = absoluteURL(r, s)
			continue
		}
		out[k] = v
	}
	return out
}

func (s *Server) audit(r *http.Request, action, status string, details map[string]any, provided ...identity) {
	id := s.identityForAudit(r, provided...)
	actorID := strings.TrimSpace(id.SubjectID)
	if actorID == "" {
		if id.IsAnonymous {
			actorID = "anonymous"
		} else {
			actorID = "unknown"
		}
	}
	tenantID := strings.TrimSpace(id.TenantID)
	if tenantID == "" {
		if id.IsAnonymous {
			tenantID = "public"
		} else {
			tenantID = "unknown"
		}
	}
	ev := store.AuditEvent{
		Time:      time.Now().UTC().Format(time.RFC3339),
		Action:    action,
		ActorID:   actorID,
		TenantID:  tenantID,
		Status:    status,
		RequestID: requestIDFromRequest(r),
		Details:   details,
	}
	s.store.RecordAudit(ev)
	log.Printf("audit action=%s status=%s actor_id=%s tenant_id=%s requestId=%s", action, status, ev.ActorID, ev.TenantID, ev.RequestID)
}

func (s *Server) identityForAudit(r *http.Request, provided ...identity) identity {
	if len(provided) > 0 {
		return provided[0]
	}
	if r == nil {
		return identity{SubjectID: "unknown", TenantID: "unknown", AuthType: "unknown"}
	}
	id, err := s.resolveIdentity(r)
	if err == nil {
		return id
	}
	if strings.TrimSpace(r.Header.Get("Authorization")) == "" {
		return anonymousIdentity()
	}
	return identity{SubjectID: "unknown", TenantID: "unknown", AuthType: "unknown"}
}

func publisherFromAuth(r *http.Request) (string, bool) {
	t := bearerToken(r)
	if t == "" {
		return "", false
	}
	t = strings.ToLower(strings.TrimSpace(t))
	switch {
	case t == "local-dev-token", strings.HasPrefix(t, "official-"):
		return "runfabric", true
	default:
		return "community", true
	}
}

func bearerToken(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return ""
	}
	return strings.TrimSpace(auth[len("Bearer "):])
}

func remoteAddrIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}

type apiError struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	Hint      string         `json:"hint,omitempty"`
	DocsURL   string         `json:"docsUrl,omitempty"`
	RequestID string         `json:"requestId"`
}

func writeAPIError(w http.ResponseWriter, r *http.Request, status int, err apiError) {
	if strings.TrimSpace(err.RequestID) == "" {
		err.RequestID = requestIDFromRequest(r)
	}
	writeJSON(w, status, map[string]any{"error": err})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

var requestSeq uint64
var requestSeqMu sync.Mutex

func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := requestIDFromRequest(r)
		r.Header.Set("X-Request-Id", id)
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r)
	})
}

func requestIDFromRequest(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-Request-Id")); v != "" {
		return v
	}
	requestSeqMu.Lock()
	requestSeq++
	seq := requestSeq
	requestSeqMu.Unlock()
	return fmt.Sprintf("req_local_%s_%d_%d", runtime.GOOS, time.Now().UTC().UnixNano(), seq)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s (%s)", r.Method, r.URL.Path, time.Since(start).Truncate(time.Millisecond))
	})
}

func classifyRateLimit(r *http.Request) (limit int, window time.Duration, group string) {
	path := r.URL.Path
	switch {
	case path == "/packages":
		if r.Method == http.MethodGet {
			return 120, time.Minute, "packages_list"
		}
		return 40, time.Minute, "packages_write"
	case strings.HasPrefix(path, "/packages/"):
		if r.Method == http.MethodGet {
			return 120, time.Minute, "packages_read"
		}
		return 40, time.Minute, "packages_write"
	case strings.HasPrefix(path, "/artifacts/") && r.Method == http.MethodPut:
		return 30, time.Minute, "artifact_put"
	case strings.HasPrefix(path, "/artifacts/"):
		return 120, time.Minute, "artifact_get"
	case path == "/v1/extensions/resolve":
		return 120, time.Minute, "resolve"
	case path == "/v1/extensions/list":
		return 60, time.Minute, "list"
	case path == "/v1/extensions/search":
		return 60, time.Minute, "search"
	case path == "/v1/extensions/publish/init":
		return 10, time.Minute, "publish_init"
	case path == "/v1/extensions/publish/finalize":
		return 10, time.Minute, "publish_finalize"
	case strings.HasPrefix(path, "/v1/publish/"):
		return 30, time.Minute, "publish_status"
	case strings.HasPrefix(path, "/v1/uploads/") && r.Method == http.MethodPut:
		return 30, time.Minute, "upload_put"
	case strings.HasPrefix(path, "/v1/uploads/"):
		return 120, time.Minute, "upload_get"
	case strings.HasPrefix(path, "/v1/extensions/"):
		return 120, time.Minute, "extensions_read"
	default:
		return 120, time.Minute, "default"
	}
}

type rateCounter struct {
	count int
	start time.Time
}

type rateLimiter struct {
	mu   sync.Mutex
	data map[string]*rateCounter
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{data: map[string]*rateCounter{}}
}

func (rl *rateLimiter) Allow(key string, limit int, window time.Duration) (bool, time.Time, int) {
	now := time.Now().UTC()
	rl.mu.Lock()
	defer rl.mu.Unlock()
	c, ok := rl.data[key]
	if !ok || now.Sub(c.start) >= window {
		rl.data[key] = &rateCounter{count: 1, start: now}
		return true, now.Add(window), limit - 1
	}
	if c.count >= limit {
		return false, c.start.Add(window), 0
	}
	c.count++
	return true, c.start.Add(window), limit - c.count
}

func strconvAtoiDefault(raw string, def int) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def, err
	}
	return v, nil
}

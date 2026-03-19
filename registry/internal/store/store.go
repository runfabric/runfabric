package store

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	LocalDevPublicKeyID  = "local-dev"
	LocalDevPublicKeyB64 = "A6EHv/POEL4dcN0Y50vAmWfk1jCbpQ1fHdyGZBJVMbg="
)

type Publisher struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Verified bool   `json:"verified"`
	Trust    string `json:"trust"`
}

type Signature struct {
	Algorithm   string `json:"algorithm"`
	Value       string `json:"value"`
	PublicKeyID string `json:"publicKeyId"`
}

type Artifact struct {
	Type                string     `json:"type"`
	Format              string     `json:"format"`
	URL                 string     `json:"url"`
	SizeBytes           int64      `json:"sizeBytes"`
	ChecksumAlgorithm   string     `json:"checksumAlgorithm"`
	ChecksumValue       string     `json:"checksumValue"`
	Signature           *Signature `json:"signature,omitempty"`
	OS                  string     `json:"os,omitempty"`
	Arch                string     `json:"arch,omitempty"`
	DeterministicBinary bool       `json:"deterministicBinary,omitempty"`
}

type ExtensionVersion struct {
	Version        string            `json:"version"`
	ReleaseStatus  string            `json:"releaseStatus"`
	Description    string            `json:"description,omitempty"`
	CoreConstraint string            `json:"coreConstraint,omitempty"`
	Capabilities   []string          `json:"capabilities,omitempty"`
	Permissions    []string          `json:"permissions,omitempty"`
	Compatibility  map[string]any    `json:"compatibility,omitempty"`
	Manifest       map[string]any    `json:"manifest,omitempty"`
	Integrity      map[string]any    `json:"integrity,omitempty"`
	Install        map[string]any    `json:"install,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	Artifacts      []Artifact        `json:"artifacts"`
	PublishedAt    string            `json:"publishedAt"`
}

type Extension struct {
	ID          string                       `json:"id"`
	Aliases     []string                     `json:"aliases,omitempty"`
	Name        string                       `json:"name"`
	Type        string                       `json:"type"` // addon | plugin
	PluginKind  string                       `json:"pluginKind,omitempty"`
	Description string                       `json:"description,omitempty"`
	PublisherID string                       `json:"publisherId"`
	Versions    map[string]*ExtensionVersion `json:"versions"`
}

type Advisory struct {
	ID              string            `json:"id"`
	ExtensionID     string            `json:"extensionId"`
	Severity        string            `json:"severity"`
	Summary         string            `json:"summary"`
	AffectedRange   string            `json:"affectedRange,omitempty"`
	PatchedVersions []string          `json:"patchedVersions,omitempty"`
	URL             string            `json:"url,omitempty"`
	PublishedAt     string            `json:"publishedAt"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

type PublishSessionFile struct {
	Key               string `json:"key"`
	Name              string `json:"name"`
	DeclaredSizeBytes int64  `json:"declaredSizeBytes"`
	DeclaredAlgorithm string `json:"declaredAlgorithm"`
	DeclaredChecksum  string `json:"declaredChecksum"`
	Uploaded          bool   `json:"uploaded"`
	UploadedSizeBytes int64  `json:"uploadedSizeBytes,omitempty"`
}

type PublishSession struct {
	ID         string                         `json:"id"`
	Status     string                         `json:"status"`
	Publisher  string                         `json:"publisher"`
	Extension  map[string]string              `json:"extension"`
	Files      map[string]*PublishSessionFile `json:"files"`
	CreatedAt  string                         `json:"createdAt"`
	UpdatedAt  string                         `json:"updatedAt"`
	UploadedAt string                         `json:"uploadedAt,omitempty"`
}

type AuditEvent struct {
	Time      string         `json:"time"`
	Action    string         `json:"action"`
	ActorID   string         `json:"actor_id"`
	TenantID  string         `json:"tenant_id"`
	Status    string         `json:"status"`
	RequestID string         `json:"requestId"`
	Details   map[string]any `json:"details,omitempty"`
}

type Data struct {
	Publishers      map[string]Publisher                          `json:"publishers"`
	Extensions      map[string]*Extension                         `json:"extensions"`
	Advisories      map[string][]Advisory                         `json:"advisories"`
	PublishSessions map[string]*PublishSession                    `json:"publishSessions"`
	AuditLog        []AuditEvent                                  `json:"auditLog,omitempty"`
	Packages        map[string]*RegistryPackage                   `json:"packages,omitempty"`
	PackageVersions map[string]map[string]*RegistryPackageVersion `json:"packageVersions,omitempty"`
	APIKeys         map[string]*APIKeyRecord                      `json:"apiKeys,omitempty"`
}

type OpenOptions struct {
	DBPath                string
	UploadsDir            string
	MetadataProvider      string
	PostgresDSN           string
	PostgresDriver        string
	MongoDBURI            string
	MongoDBDatabase       string
	MongoDBConnectTimeout time.Duration
	SeedLocalDevData      bool
}

type Store struct {
	mu            sync.RWMutex
	dbPath        string
	uploadsDir    string
	data          *Data
	metadata      MetadataRepository
	metadataClose func() error
}

type ResolveInput struct {
	ID      string
	Core    string
	OS      string
	Arch    string
	Version string
}

type ResolveOutput struct {
	Extension *Extension
	Version   *ExtensionVersion
	Publisher Publisher
	Artifact  Artifact
}

type SearchInput struct {
	Query      string
	Type       string
	PluginKind string
	Page       int
	PageSize   int
}

type SearchItem struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Type          string         `json:"type"`
	PluginKind    string         `json:"pluginKind,omitempty"`
	Description   string         `json:"description,omitempty"`
	LatestVersion string         `json:"latestVersion,omitempty"`
	Publisher     map[string]any `json:"publisher,omitempty"`
}

type SearchOutput struct {
	Items    []SearchItem `json:"items"`
	Page     int          `json:"page"`
	PageSize int          `json:"pageSize"`
	Total    int          `json:"total"`
}

type PublishFileInput struct {
	Key       string
	Name      string
	SizeBytes int64
	Algorithm string
	Checksum  string
}

type PublishInitInput struct {
	Publisher   string
	ID          string
	Version     string
	Type        string
	PluginKind  string
	Description string
	Files       []PublishFileInput
}

func Open(opts OpenOptions) (*Store, error) {
	dbPath := strings.TrimSpace(opts.DBPath)
	if dbPath == "" {
		dbPath = filepath.Join("data", "registry.db.json")
	}
	uploads := strings.TrimSpace(opts.UploadsDir)
	if uploads == "" {
		uploads = filepath.Join(filepath.Dir(dbPath), "uploads")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(uploads, 0o755); err != nil {
		return nil, err
	}

	s := &Store{dbPath: dbPath, uploadsDir: uploads}
	provider := normalizeMetadataProvider(opts)
	backend, err := openMetadataBackend(s, provider, opts)
	if err != nil {
		return nil, err
	}
	s.metadata = backend.repo
	s.metadataClose = backend.closeFn
	if err := s.loadOrSeed(opts.SeedLocalDevData); err != nil {
		if s.metadataClose != nil {
			_ = s.metadataClose()
		}
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s == nil || s.metadataClose == nil {
		return nil
	}
	return s.metadataClose()
}

func (s *Store) UploadsDir() string {
	return s.uploadsDir
}

func (s *Store) loadOrSeed(seedLocalDevData bool) error {
	b, err := os.ReadFile(s.dbPath)
	if err == nil {
		var data Data
		if err := json.Unmarshal(b, &data); err != nil {
			return fmt.Errorf("load db: %w", err)
		}
		normalizeData(&data)
		s.data = &data
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	seed := seededData(seedLocalDevData)
	s.data = seed
	return s.saveLocked()
}

func normalizeData(d *Data) {
	if d.Publishers == nil {
		d.Publishers = map[string]Publisher{}
	}
	if d.Extensions == nil {
		d.Extensions = map[string]*Extension{}
	}
	if d.Advisories == nil {
		d.Advisories = map[string][]Advisory{}
	}
	if d.PublishSessions == nil {
		d.PublishSessions = map[string]*PublishSession{}
	}
	if d.Packages == nil {
		d.Packages = map[string]*RegistryPackage{}
	}
	if d.PackageVersions == nil {
		d.PackageVersions = map[string]map[string]*RegistryPackageVersion{}
	}
	if d.APIKeys == nil {
		d.APIKeys = map[string]*APIKeyRecord{}
	}
}

func seededData(seedLocalDevData bool) *Data {
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	sentrySum := sha256.Sum256([]byte("sentry@1.2.0"))

	providerArtifacts := make([]Artifact, 0, 4)
	for _, platform := range []struct{ os, arch string }{
		{"darwin", "arm64"},
		{"darwin", "amd64"},
		{"linux", "amd64"},
		{"linux", "arm64"},
	} {
		bytes := deterministicBinaryBytes("provider-aws", "1.0.0", platform.os, platform.arch)
		sum := sha256.Sum256(bytes)
		sig := ed25519.Sign(priv, bytes)
		providerArtifacts = append(providerArtifacts, Artifact{
			Type:                "binary",
			Format:              "executable",
			URL:                 "/bin?id=provider-aws&version=1.0.0&os=${os}&arch=${arch}",
			SizeBytes:           int64(len(bytes)),
			ChecksumAlgorithm:   "sha256",
			ChecksumValue:       hex.EncodeToString(sum[:]),
			Signature:           &Signature{Algorithm: "ed25519", Value: base64.StdEncoding.EncodeToString(sig), PublicKeyID: LocalDevPublicKeyID},
			OS:                  platform.os,
			Arch:                platform.arch,
			DeterministicBinary: true,
		})
	}

	now := time.Now().UTC().Format(time.RFC3339)
	out := &Data{
		Publishers: map[string]Publisher{
			"runfabric": {ID: "runfabric", Name: "RunFabric", Verified: true, Trust: "official"},
			"community": {ID: "community", Name: "Community", Verified: false, Trust: "community"},
		},
		Extensions: map[string]*Extension{
			"sentry": {
				ID:          "sentry",
				Aliases:     []string{"addon-sentry"},
				Name:        "Sentry",
				Type:        "addon",
				Description: "Error tracking and performance monitoring for serverless functions",
				PublisherID: "runfabric",
				Versions: map[string]*ExtensionVersion{
					"1.2.0": {
						Version:        "1.2.0",
						ReleaseStatus:  "published",
						CoreConstraint: ">=0.8.0",
						Permissions:    []string{"env:write", "fs:build-write", "handler:wrap", "network:outbound"},
						Compatibility: map[string]any{
							"core":      ">=0.8.0",
							"runtimes":  []string{"node"},
							"providers": []string{"aws", "cloudflare", "vercel"},
						},
						Artifacts: []Artifact{{
							Type:              "addon",
							Format:            "tgz",
							URL:               "https://cdn.runfabric.cloud/extensions/addons/sentry/1.2.0/addon.tgz",
							SizeBytes:         84231,
							ChecksumAlgorithm: "sha256",
							ChecksumValue:     hex.EncodeToString(sentrySum[:]),
						}},
						Manifest: map[string]any{
							"url":       "https://cdn.runfabric.cloud/extensions/addons/sentry/1.2.0/manifest.json",
							"schemaUrl": "https://cdn.runfabric.cloud/extensions/addons/sentry/1.2.0/config.schema.json",
						},
						Integrity: map[string]any{
							"sbomUrl":       "https://cdn.runfabric.cloud/extensions/addons/sentry/1.2.0/sbom.spdx.json",
							"provenanceUrl": "https://cdn.runfabric.cloud/extensions/addons/sentry/1.2.0/provenance.intoto.jsonl",
						},
						Install:     map[string]any{"path": "addons/sentry/1.2.0", "postInstall": []string{}},
						PublishedAt: now,
					},
				},
			},
			"provider-aws": {
				ID:          "provider-aws",
				Aliases:     []string{"aws-provider"},
				Name:        "AWS Provider",
				Type:        "plugin",
				PluginKind:  "provider",
				Description: "Deploy and operate functions on AWS",
				PublisherID: "runfabric",
				Versions: map[string]*ExtensionVersion{
					"1.0.0": {
						Version:        "1.0.0",
						ReleaseStatus:  "published",
						CoreConstraint: ">=0.9.0",
						Capabilities:   []string{"validateConfig", "doctor", "plan", "deploy", "remove", "invoke", "logs"},
						Compatibility:  map[string]any{"core": ">=0.9.0"},
						Manifest: map[string]any{
							"url": "https://cdn.runfabric.cloud/extensions/plugins/providers/aws/1.0.0/manifest.json",
						},
						Artifacts:   providerArtifacts,
						PublishedAt: now,
					},
				},
			},
		},
		Advisories: map[string][]Advisory{
			"sentry": {
				{ID: "adv_sentry_2026_0001", ExtensionID: "sentry", Severity: "low", Summary: "Upgrade recommended for latest source map handling", AffectedRange: "<1.2.0", PatchedVersions: []string{"1.2.0"}, URL: "https://runfabric.cloud/security/advisories/adv_sentry_2026_0001", PublishedAt: now},
			},
		},
		PublishSessions: map[string]*PublishSession{},
		AuditLog:        []AuditEvent{},
		Packages:        map[string]*RegistryPackage{},
		PackageVersions: map[string]map[string]*RegistryPackageVersion{},
		APIKeys:         map[string]*APIKeyRecord{},
	}
	if seedLocalDevData {
		out.APIKeys[HashAPIKey("rk_local_dev")] = &APIKeyRecord{
			ID:        "key_local_dev",
			KeyHash:   HashAPIKey("rk_local_dev"),
			UserID:    "ci-bot",
			TenantID:  "tenant_runfabric",
			Roles:     []string{"admin", "publisher", "reader"},
			CreatedAt: now,
		}
	}
	return out
}

func deterministicBinaryBytes(id, version, goos, arch string) []byte {
	return []byte(id + "@" + version + ":" + goos + "-" + arch)
}

func (s *Store) saveLocked() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.dbPath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.dbPath)
}

func (s *Store) RecordAudit(ev AuditEvent) {
	repo, err := s.metadataRepo()
	if err != nil {
		return
	}
	_ = repo.RecordAudit(ev)
}

func (s *Store) ListAudit(limit int) []AuditEvent {
	repo, err := s.metadataRepo()
	if err != nil {
		return nil
	}
	out, err := repo.ListAudit(limit)
	if err != nil {
		return nil
	}
	return out
}

func (s *Store) recordAuditJSON(ev AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.AuditLog = append(s.data.AuditLog, ev)
	if len(s.data.AuditLog) > 5000 {
		s.data.AuditLog = append([]AuditEvent(nil), s.data.AuditLog[len(s.data.AuditLog)-5000:]...)
	}
	return s.saveLocked()
}

func (s *Store) listAuditJSON(limit int) ([]AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	n := len(s.data.AuditLog)
	if n == 0 {
		return nil, nil
	}
	start := n - limit
	if start < 0 {
		start = 0
	}
	out := append([]AuditEvent(nil), s.data.AuditLog[start:]...)
	return out, nil
}

func (s *Store) Resolve(in ResolveInput) (*ResolveOutput, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id := canonicalIDLocked(s.data.Extensions, strings.TrimSpace(in.ID))
	ext, ok := s.data.Extensions[id]
	if !ok {
		return nil, fmt.Errorf("extension not found")
	}
	pub, ok := s.data.Publishers[ext.PublisherID]
	if !ok {
		return nil, fmt.Errorf("publisher not found")
	}
	core := strings.TrimSpace(in.Core)
	goos := strings.TrimSpace(in.OS)
	arch := strings.TrimSpace(in.Arch)
	pin := strings.TrimSpace(in.Version)

	candidates := make([]*ExtensionVersion, 0, len(ext.Versions))
	for _, v := range ext.Versions {
		if v == nil || strings.TrimSpace(v.ReleaseStatus) != "published" {
			continue
		}
		if pin != "" && strings.TrimSpace(v.Version) != pin {
			continue
		}
		if !coreCompatible(core, effectiveCoreConstraint(v)) {
			continue
		}
		if _, ok := selectArtifact(v, ext.Type, goos, arch); !ok {
			continue
		}
		candidates = append(candidates, v)
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no compatible version")
	}
	sort.Slice(candidates, func(i, j int) bool {
		cmp := compareVersion(candidates[i].Version, candidates[j].Version)
		if cmp != 0 {
			return cmp > 0
		}
		if candidates[i].PublishedAt != candidates[j].PublishedAt {
			return candidates[i].PublishedAt > candidates[j].PublishedAt
		}
		return candidates[i].Version > candidates[j].Version
	})
	picked := candidates[0]
	artifact, _ := selectArtifact(picked, ext.Type, goos, arch)
	artifact = materializeArtifact(ext.ID, picked.Version, artifact, goos, arch)
	if pub.Verified && artifact.Signature == nil {
		return nil, fmt.Errorf("verified publisher artifact requires signature")
	}
	return &ResolveOutput{Extension: cloneExtensionMeta(ext), Version: cloneVersionMeta(picked), Publisher: pub, Artifact: artifact}, nil
}

func effectiveCoreConstraint(v *ExtensionVersion) string {
	if strings.TrimSpace(v.CoreConstraint) != "" {
		return strings.TrimSpace(v.CoreConstraint)
	}
	if v.Compatibility != nil {
		if c, ok := v.Compatibility["core"].(string); ok {
			return strings.TrimSpace(c)
		}
	}
	return ""
}

func materializeArtifact(id, version string, a Artifact, goos, arch string) Artifact {
	out := a
	out.URL = strings.ReplaceAll(out.URL, "${os}", url.QueryEscape(goos))
	out.URL = strings.ReplaceAll(out.URL, "${arch}", url.QueryEscape(arch))
	if !out.DeterministicBinary {
		return out
	}
	b := deterministicBinaryBytes(id, version, goos, arch)
	sum := sha256.Sum256(b)
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	sig := ed25519.Sign(priv, b)
	out.SizeBytes = int64(len(b))
	out.ChecksumAlgorithm = "sha256"
	out.ChecksumValue = hex.EncodeToString(sum[:])
	out.Signature = &Signature{Algorithm: "ed25519", Value: base64.StdEncoding.EncodeToString(sig), PublicKeyID: LocalDevPublicKeyID}
	return out
}

func selectArtifact(v *ExtensionVersion, extType, goos, arch string) (Artifact, bool) {
	if len(v.Artifacts) == 0 {
		return Artifact{}, false
	}
	if extType != "plugin" {
		return v.Artifacts[0], true
	}
	for _, a := range v.Artifacts {
		if strings.EqualFold(strings.TrimSpace(a.OS), goos) && strings.EqualFold(strings.TrimSpace(a.Arch), arch) {
			return a, true
		}
	}
	for _, a := range v.Artifacts {
		if strings.TrimSpace(a.OS) == "" && strings.TrimSpace(a.Arch) == "" {
			return a, true
		}
	}
	return Artifact{}, false
}

func cloneExtensionMeta(ext *Extension) *Extension {
	if ext == nil {
		return nil
	}
	return &Extension{
		ID:          ext.ID,
		Aliases:     append([]string(nil), ext.Aliases...),
		Name:        ext.Name,
		Type:        ext.Type,
		PluginKind:  ext.PluginKind,
		Description: ext.Description,
		PublisherID: ext.PublisherID,
	}
}

func cloneVersionMeta(v *ExtensionVersion) *ExtensionVersion {
	if v == nil {
		return nil
	}
	out := *v
	out.Capabilities = append([]string(nil), v.Capabilities...)
	out.Permissions = append([]string(nil), v.Permissions...)
	out.Artifacts = append([]Artifact(nil), v.Artifacts...)
	if v.Compatibility != nil {
		out.Compatibility = map[string]any{}
		for k, vv := range v.Compatibility {
			out.Compatibility[k] = vv
		}
	}
	if v.Manifest != nil {
		out.Manifest = map[string]any{}
		for k, vv := range v.Manifest {
			out.Manifest[k] = vv
		}
	}
	if v.Integrity != nil {
		out.Integrity = map[string]any{}
		for k, vv := range v.Integrity {
			out.Integrity[k] = vv
		}
	}
	if v.Install != nil {
		out.Install = map[string]any{}
		for k, vv := range v.Install {
			out.Install[k] = vv
		}
	}
	return &out
}

func canonicalIDLocked(ext map[string]*Extension, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if _, ok := ext[raw]; ok {
		return raw
	}
	for id, e := range ext {
		for _, alias := range e.Aliases {
			if strings.EqualFold(strings.TrimSpace(alias), raw) {
				return id
			}
		}
	}
	return raw
}

func (s *Store) Search(in SearchInput) (*SearchOutput, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	q := strings.ToLower(strings.TrimSpace(in.Query))
	typ := strings.ToLower(strings.TrimSpace(in.Type))
	kind := strings.ToLower(strings.TrimSpace(in.PluginKind))
	page := in.Page
	if page <= 0 {
		page = 1
	}
	sz := in.PageSize
	if sz <= 0 {
		sz = 20
	}
	if sz > 100 {
		sz = 100
	}

	items := make([]SearchItem, 0, len(s.data.Extensions))
	for _, ext := range s.data.Extensions {
		if typ != "" && strings.ToLower(ext.Type) != typ {
			continue
		}
		if kind != "" && strings.ToLower(ext.PluginKind) != kind {
			continue
		}
		hay := strings.ToLower(ext.ID + " " + ext.Name + " " + ext.Description + " " + strings.Join(ext.Aliases, " "))
		if q != "" && !strings.Contains(hay, q) {
			continue
		}
		latest := latestPublishedVersion(ext)
		pub := s.data.Publishers[ext.PublisherID]
		items = append(items, SearchItem{
			ID:            ext.ID,
			Name:          ext.Name,
			Type:          ext.Type,
			PluginKind:    ext.PluginKind,
			Description:   ext.Description,
			LatestVersion: latest,
			Publisher: map[string]any{
				"id":       pub.ID,
				"name":     pub.Name,
				"verified": pub.Verified,
				"trust":    pub.Trust,
			},
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ID != items[j].ID {
			return items[i].ID < items[j].ID
		}
		return items[i].LatestVersion > items[j].LatestVersion
	})
	total := len(items)
	start := (page - 1) * sz
	if start > total {
		start = total
	}
	end := start + sz
	if end > total {
		end = total
	}
	return &SearchOutput{Items: items[start:end], Page: page, PageSize: sz, Total: total}, nil
}

func latestPublishedVersion(ext *Extension) string {
	best := ""
	for _, v := range ext.Versions {
		if v == nil || strings.TrimSpace(v.ReleaseStatus) != "published" {
			continue
		}
		if best == "" || compareVersion(v.Version, best) > 0 {
			best = v.Version
		}
	}
	return best
}

func (s *Store) GetExtension(id string) (*Extension, Publisher, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id = canonicalIDLocked(s.data.Extensions, id)
	ext, ok := s.data.Extensions[id]
	if !ok {
		return nil, Publisher{}, fmt.Errorf("extension not found")
	}
	pub, ok := s.data.Publishers[ext.PublisherID]
	if !ok {
		return nil, Publisher{}, fmt.Errorf("publisher not found")
	}
	out := cloneExtensionMeta(ext)
	return out, pub, nil
}

func (s *Store) ListVersions(id string) ([]*ExtensionVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id = canonicalIDLocked(s.data.Extensions, id)
	ext, ok := s.data.Extensions[id]
	if !ok {
		return nil, fmt.Errorf("extension not found")
	}
	out := make([]*ExtensionVersion, 0, len(ext.Versions))
	for _, v := range ext.Versions {
		out = append(out, cloneVersionMeta(v))
	}
	sort.Slice(out, func(i, j int) bool {
		cmp := compareVersion(out[i].Version, out[j].Version)
		if cmp != 0 {
			return cmp > 0
		}
		return out[i].PublishedAt > out[j].PublishedAt
	})
	return out, nil
}

func (s *Store) GetVersion(id, version string) (*ExtensionVersion, *Extension, Publisher, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id = canonicalIDLocked(s.data.Extensions, id)
	ext, ok := s.data.Extensions[id]
	if !ok {
		return nil, nil, Publisher{}, fmt.Errorf("extension not found")
	}
	v, ok := ext.Versions[strings.TrimSpace(version)]
	if !ok || v == nil {
		return nil, nil, Publisher{}, fmt.Errorf("version not found")
	}
	pub := s.data.Publishers[ext.PublisherID]
	return cloneVersionMeta(v), cloneExtensionMeta(ext), pub, nil
}

func (s *Store) ListAdvisories(id string) ([]Advisory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id = canonicalIDLocked(s.data.Extensions, id)
	if _, ok := s.data.Extensions[id]; !ok {
		return nil, fmt.Errorf("extension not found")
	}
	adv := append([]Advisory(nil), s.data.Advisories[id]...)
	sort.Slice(adv, func(i, j int) bool {
		if adv[i].PublishedAt != adv[j].PublishedAt {
			return adv[i].PublishedAt > adv[j].PublishedAt
		}
		return adv[i].ID < adv[j].ID
	})
	return adv, nil
}

func (s *Store) CreatePublishSession(in PublishInitInput) (*PublishSession, error) {
	if strings.TrimSpace(in.ID) == "" || strings.TrimSpace(in.Version) == "" {
		return nil, fmt.Errorf("extension id and version are required")
	}
	typ := strings.ToLower(strings.TrimSpace(in.Type))
	if typ == "" {
		typ = "plugin"
	}
	if typ != "plugin" && typ != "addon" {
		return nil, fmt.Errorf("type must be plugin or addon")
	}
	if typ == "plugin" {
		pk := strings.ToLower(strings.TrimSpace(in.PluginKind))
		if pk != "provider" && pk != "runtime" && pk != "simulator" {
			return nil, fmt.Errorf("pluginKind must be provider, runtime, or simulator")
		}
	}
	if len(in.Files) == 0 {
		return nil, fmt.Errorf("at least one file is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	publishID := fmt.Sprintf("pub_local_%d", time.Now().UTC().UnixNano())
	now := time.Now().UTC().Format(time.RFC3339)
	session := &PublishSession{
		ID:        publishID,
		Status:    "staged",
		Publisher: strings.TrimSpace(in.Publisher),
		Extension: map[string]string{
			"id":          strings.TrimSpace(in.ID),
			"version":     strings.TrimSpace(in.Version),
			"type":        typ,
			"pluginKind":  strings.TrimSpace(in.PluginKind),
			"description": strings.TrimSpace(in.Description),
		},
		Files:     map[string]*PublishSessionFile{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	for _, f := range in.Files {
		key := strings.TrimSpace(f.Key)
		if key == "" {
			key = "artifact"
		}
		alg := strings.ToLower(strings.TrimSpace(f.Algorithm))
		if alg == "" {
			alg = "sha256"
		}
		if alg != "sha256" {
			return nil, fmt.Errorf("file %s checksum algorithm must be sha256", key)
		}
		chk := strings.ToLower(strings.TrimSpace(f.Checksum))
		if chk == "" {
			return nil, fmt.Errorf("file %s checksum is required", key)
		}
		session.Files[key] = &PublishSessionFile{
			Key:               key,
			Name:              strings.TrimSpace(f.Name),
			DeclaredSizeBytes: f.SizeBytes,
			DeclaredAlgorithm: alg,
			DeclaredChecksum:  chk,
			Uploaded:          false,
		}
	}
	s.data.PublishSessions[publishID] = session
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	cloned := *session
	return &cloned, nil
}

func (s *Store) UploadPublishFile(publishID, key string, body []byte) error {
	publishID = strings.TrimSpace(publishID)
	key = strings.TrimSpace(key)
	if publishID == "" || key == "" {
		return fmt.Errorf("publish id and key are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.data.PublishSessions[publishID]
	if !ok {
		return fmt.Errorf("publish session not found")
	}
	file, ok := sess.Files[key]
	if !ok {
		return fmt.Errorf("publish file key not staged")
	}
	sum := sha256.Sum256(body)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, file.DeclaredChecksum) {
		return fmt.Errorf("checksum mismatch for key %s", key)
	}
	if file.DeclaredSizeBytes > 0 && file.DeclaredSizeBytes != int64(len(body)) {
		return fmt.Errorf("size mismatch for key %s", key)
	}
	dir := filepath.Join(s.uploadsDir, publishID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, key), body, 0o644); err != nil {
		return err
	}
	file.Uploaded = true
	file.UploadedSizeBytes = int64(len(body))
	sess.Status = "uploaded"
	sess.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	sess.UploadedAt = sess.UpdatedAt
	if err := s.saveLocked(); err != nil {
		return err
	}
	return nil
}

func (s *Store) PublishStatus(publishID string) (*PublishSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.data.PublishSessions[strings.TrimSpace(publishID)]
	if !ok {
		return nil, fmt.Errorf("publish session not found")
	}
	out := *sess
	out.Files = map[string]*PublishSessionFile{}
	for k, v := range sess.Files {
		cp := *v
		out.Files[k] = &cp
	}
	out.Extension = map[string]string{}
	for k, v := range sess.Extension {
		out.Extension[k] = v
	}
	return &out, nil
}

func (s *Store) FinalizePublish(publishID string) (*PublishSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.data.PublishSessions[strings.TrimSpace(publishID)]
	if !ok {
		return nil, fmt.Errorf("publish session not found")
	}
	for _, f := range sess.Files {
		if !f.Uploaded {
			return nil, fmt.Errorf("all staged files must be uploaded before finalize")
		}
	}

	extID := strings.TrimSpace(sess.Extension["id"])
	ver := strings.TrimSpace(sess.Extension["version"])
	typ := strings.TrimSpace(sess.Extension["type"])
	pluginKind := strings.TrimSpace(sess.Extension["pluginKind"])
	desc := strings.TrimSpace(sess.Extension["description"])
	if desc == "" {
		desc = extID
	}

	ext, ok := s.data.Extensions[extID]
	if !ok {
		ext = &Extension{
			ID:          extID,
			Name:        extID,
			Type:        typ,
			PluginKind:  pluginKind,
			Description: desc,
			PublisherID: sess.Publisher,
			Versions:    map[string]*ExtensionVersion{},
		}
		s.data.Extensions[extID] = ext
	}
	if _, exists := ext.Versions[ver]; exists {
		return nil, fmt.Errorf("version %s already exists", ver)
	}

	keys := make([]string, 0, len(sess.Files))
	for k := range sess.Files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	artifactKey := keys[0]
	file := sess.Files[artifactKey]
	binPath := filepath.Join(s.uploadsDir, sess.ID, artifactKey)
	b, err := os.ReadFile(binPath)
	if err != nil {
		return nil, err
	}
	fmtGuess := artifactFormatFromName(file.Name)
	artType := "addon"
	if typ == "plugin" {
		artType = "binary"
	}
	artifact := Artifact{
		Type:              artType,
		Format:            fmtGuess,
		URL:               "/v1/uploads/" + url.PathEscape(sess.ID) + "/" + url.PathEscape(artifactKey),
		SizeBytes:         int64(len(b)),
		ChecksumAlgorithm: "sha256",
		ChecksumValue:     file.DeclaredChecksum,
	}
	if pub, ok := s.data.Publishers[sess.Publisher]; ok && pub.Verified && typ == "plugin" {
		seed := make([]byte, 32)
		for i := range seed {
			seed[i] = byte(i)
		}
		priv := ed25519.NewKeyFromSeed(seed)
		sig := ed25519.Sign(priv, b)
		artifact.Signature = &Signature{Algorithm: "ed25519", Value: base64.StdEncoding.EncodeToString(sig), PublicKeyID: LocalDevPublicKeyID}
	}
	if pub, ok := s.data.Publishers[sess.Publisher]; ok && pub.Verified && typ == "plugin" && artifact.Signature == nil {
		return nil, fmt.Errorf("plugin artifacts require signature for verified publisher policy")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	ext.Versions[ver] = &ExtensionVersion{
		Version:        ver,
		ReleaseStatus:  "published",
		Description:    desc,
		CoreConstraint: ">=0.9.0",
		Compatibility:  map[string]any{"core": ">=0.9.0"},
		Artifacts:      []Artifact{artifact},
		PublishedAt:    now,
	}
	sess.Status = "published"
	sess.UpdatedAt = now
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	out := *sess
	return &out, nil
}

func artifactFormatFromName(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.HasSuffix(lower, ".tar.gz"):
		return "tar.gz"
	case strings.HasSuffix(lower, ".tgz"):
		return "tgz"
	case strings.HasSuffix(lower, ".zip"):
		return "zip"
	default:
		return "executable"
	}
}

func coreCompatible(core, constraint string) bool {
	constraint = strings.TrimSpace(constraint)
	if constraint == "" || constraint == "*" {
		return true
	}
	core = strings.TrimSpace(core)
	if core == "" {
		return false
	}
	parts := strings.Split(constraint, ",")
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		op, ver := splitConstraint(p)
		cmp := compareVersion(core, ver)
		switch op {
		case ">=":
			if cmp < 0 {
				return false
			}
		case ">":
			if cmp <= 0 {
				return false
			}
		case "<=":
			if cmp > 0 {
				return false
			}
		case "<":
			if cmp >= 0 {
				return false
			}
		case "=", "":
			if cmp != 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func splitConstraint(s string) (op, version string) {
	for _, prefix := range []string{">=", "<=", ">", "<", "="} {
		if strings.HasPrefix(s, prefix) {
			return prefix, strings.TrimSpace(strings.TrimPrefix(s, prefix))
		}
	}
	return "=", strings.TrimSpace(s)
}

func compareVersion(a, b string) int {
	am, an, ap, aok := parseSemver(a)
	bm, bn, bp, bok := parseSemver(b)
	if aok && bok {
		if am != bm {
			return sign(am - bm)
		}
		if an != bn {
			return sign(an - bn)
		}
		if ap != bp {
			return sign(ap - bp)
		}
		return 0
	}
	if aok && !bok {
		return 1
	}
	if !aok && bok {
		return -1
	}
	if a == b {
		return 0
	}
	if a > b {
		return 1
	}
	return -1
}

func parseSemver(v string) (int, int, int, bool) {
	v = strings.TrimSpace(strings.TrimPrefix(v, "v"))
	if v == "" {
		return 0, 0, 0, false
	}
	if idx := strings.Index(v, "+"); idx >= 0 {
		v = v[:idx]
	}
	if idx := strings.Index(v, "-"); idx >= 0 {
		v = v[:idx]
	}
	parts := strings.Split(v, ".")
	if len(parts) < 1 {
		return 0, 0, 0, false
	}
	m, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, false
	}
	n := 0
	p := 0
	if len(parts) > 1 {
		n, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, 0, false
		}
	}
	if len(parts) > 2 {
		p, err = strconv.Atoi(parts[2])
		if err != nil {
			return 0, 0, 0, false
		}
	}
	return m, n, p, true
}

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	default:
		return 0
	}
}

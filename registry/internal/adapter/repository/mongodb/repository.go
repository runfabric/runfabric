package mongodb

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

const (
	VisibilityPublic = "public"
	VisibilityTenant = "tenant"
)

type Repository struct {
	client          *mongo.Client
	db              *mongo.Database
	packages        *mongo.Collection
	packageVersions *mongo.Collection
	apiKeys         *mongo.Collection
	auditEvents     *mongo.Collection
}

type Package struct {
	ID            string
	TenantID      string
	Namespace     string
	Name          string
	LatestVersion string
	Visibility    string
	CreatedBy     string
	CreatedAt     string
	UpdatedAt     string
}

type PackageVersion struct {
	ID           string
	PackageID    string
	TenantID     string
	Version      string
	ManifestJSON map[string]any
	ArtifactKey  string
	Checksum     string
	SizeBytes    int64
	PublishedBy  string
	PublishedAt  string
}

type APIKeyRecord struct {
	ID        string
	KeyHash   string
	UserID    string
	TenantID  string
	Roles     []string
	ExpiresAt string
	RevokedAt string
	CreatedAt string
}

type PackageFilter struct {
	TenantID      string
	IncludePublic bool
	PublicOnly    bool
	Namespace     string
	Query         string
}

type CreatePackageInput struct {
	TenantID   string
	Namespace  string
	Name       string
	Visibility string
	CreatedBy  string
}

type PublishPackageVersionInput struct {
	TenantID    string
	Namespace   string
	Name        string
	Version     string
	Manifest    map[string]any
	ArtifactKey string
	Checksum    string
	SizeBytes   int64
	PublishedBy string
}

type UpdatePackageVisibilityInput struct {
	TenantID   string
	Namespace  string
	Name       string
	Visibility string
}

type DeletePackageInput struct {
	TenantID  string
	Namespace string
	Name      string
}

type AuditEvent struct {
	Time      string
	Action    string
	ActorID   string
	TenantID  string
	Status    string
	RequestID string
	Details   map[string]any
}

type packageDocument struct {
	ID            string    `bson:"_id"`
	TenantID      string    `bson:"tenant_id"`
	Namespace     string    `bson:"namespace"`
	Name          string    `bson:"name"`
	LatestVersion string    `bson:"latest_version,omitempty"`
	Visibility    string    `bson:"visibility"`
	CreatedBy     string    `bson:"created_by"`
	CreatedAt     time.Time `bson:"created_at"`
	UpdatedAt     time.Time `bson:"updated_at"`
}

type packageVersionDocument struct {
	ID           string         `bson:"_id"`
	PackageID    string         `bson:"package_id"`
	TenantID     string         `bson:"tenant_id"`
	Version      string         `bson:"version"`
	ManifestJSON map[string]any `bson:"manifest_json,omitempty"`
	ArtifactKey  string         `bson:"artifact_key"`
	Checksum     string         `bson:"checksum,omitempty"`
	SizeBytes    int64          `bson:"size_bytes,omitempty"`
	PublishedBy  string         `bson:"published_by"`
	PublishedAt  time.Time      `bson:"published_at"`
}

type apiKeyDocument struct {
	ID        string     `bson:"_id"`
	KeyHash   string     `bson:"key_hash"`
	UserID    string     `bson:"user_id"`
	TenantID  string     `bson:"tenant_id"`
	Roles     []string   `bson:"roles"`
	ExpiresAt *time.Time `bson:"expires_at,omitempty"`
	RevokedAt *time.Time `bson:"revoked_at,omitempty"`
	CreatedAt time.Time  `bson:"created_at"`
}

type auditDocument struct {
	EventTime time.Time      `bson:"event_time"`
	Action    string         `bson:"action"`
	ActorID   string         `bson:"actor_id"`
	TenantID  string         `bson:"tenant_id"`
	Status    string         `bson:"status"`
	RequestID string         `bson:"request_id,omitempty"`
	Details   map[string]any `bson:"details,omitempty"`
}

func Connect(ctx context.Context, uri, database string) (*Repository, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(strings.TrimSpace(uri)))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, err
	}
	return New(client, database), nil
}

func New(client *mongo.Client, database string) *Repository {
	database = strings.TrimSpace(database)
	if database == "" {
		database = "runfabric_registry"
	}
	db := client.Database(database)
	return &Repository{
		client:          client,
		db:              db,
		packages:        db.Collection("packages"),
		packageVersions: db.Collection("package_versions"),
		apiKeys:         db.Collection("api_keys"),
		auditEvents:     db.Collection("audit_events"),
	}
}

func (r *Repository) Close(ctx context.Context) error {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Disconnect(ctx)
}

func (r *Repository) Enabled() bool {
	return r != nil && r.client != nil && r.db != nil
}

func (r *Repository) Migrate(ctx context.Context) error {
	if !r.Enabled() {
		return fmt.Errorf("mongodb repository is not configured")
	}
	if _, err := r.packages.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "namespace", Value: 1}, {Key: "name", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		return err
	}
	if _, err := r.packages.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "tenant_id", Value: 1}}}); err != nil {
		return err
	}
	if _, err := r.packages.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "visibility", Value: 1}}}); err != nil {
		return err
	}
	if _, err := r.packageVersions.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "package_id", Value: 1}, {Key: "version", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		return err
	}
	if _, err := r.packageVersions.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "package_id", Value: 1}}}); err != nil {
		return err
	}
	if _, err := r.packageVersions.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "tenant_id", Value: 1}}}); err != nil {
		return err
	}
	if _, err := r.apiKeys.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "key_hash", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		return err
	}
	if _, err := r.auditEvents.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "tenant_id", Value: 1}}}); err != nil {
		return err
	}
	return nil
}

func (r *Repository) SeedAPIKey(ctx context.Context, id, keyHash, userID, tenantID string, roles []string) error {
	now := time.Now().UTC()
	doc := apiKeyDocument{
		ID:        strings.TrimSpace(id),
		KeyHash:   strings.TrimSpace(keyHash),
		UserID:    strings.TrimSpace(userID),
		TenantID:  strings.TrimSpace(tenantID),
		Roles:     append([]string(nil), roles...),
		CreatedAt: now,
	}
	_, err := r.apiKeys.UpdateOne(ctx,
		bson.M{"key_hash": doc.KeyHash},
		bson.M{"$setOnInsert": doc},
		options.Update().SetUpsert(true),
	)
	return err
}

func (r *Repository) FindAPIKeyByHash(ctx context.Context, keyHash string) (*APIKeyRecord, error) {
	var doc apiKeyDocument
	err := r.apiKeys.FindOne(ctx, bson.M{"key_hash": strings.TrimSpace(keyHash)}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("api key not found")
		}
		return nil, err
	}
	if doc.RevokedAt != nil {
		return nil, fmt.Errorf("api key revoked")
	}
	if doc.ExpiresAt != nil && time.Now().UTC().After(doc.ExpiresAt.UTC()) {
		return nil, fmt.Errorf("api key expired")
	}
	out := &APIKeyRecord{
		ID:        doc.ID,
		KeyHash:   doc.KeyHash,
		UserID:    doc.UserID,
		TenantID:  doc.TenantID,
		Roles:     append([]string(nil), doc.Roles...),
		CreatedAt: doc.CreatedAt.UTC().Format(time.RFC3339),
	}
	if doc.ExpiresAt != nil {
		out.ExpiresAt = doc.ExpiresAt.UTC().Format(time.RFC3339)
	}
	if doc.RevokedAt != nil {
		out.RevokedAt = doc.RevokedAt.UTC().Format(time.RFC3339)
	}
	return out, nil
}

func (r *Repository) ListVisiblePackages(ctx context.Context, in PackageFilter) ([]*Package, error) {
	andClauses := bson.A{}
	tenantID := strings.TrimSpace(in.TenantID)
	if in.PublicOnly {
		andClauses = append(andClauses, bson.M{"visibility": VisibilityPublic})
	} else if tenantID != "" && in.IncludePublic {
		andClauses = append(andClauses, bson.M{"$or": bson.A{bson.M{"visibility": VisibilityPublic}, bson.M{"tenant_id": tenantID}}})
	} else if tenantID != "" {
		andClauses = append(andClauses, bson.M{"tenant_id": tenantID})
	}
	ns := normalizeSegment(in.Namespace)
	if ns != "" {
		andClauses = append(andClauses, bson.M{"namespace": ns})
	}
	q := strings.ToLower(strings.TrimSpace(in.Query))
	if q != "" {
		expr := primitiveContainsRegex(q)
		andClauses = append(andClauses, bson.M{"$or": bson.A{bson.M{"namespace": expr}, bson.M{"name": expr}}})
	}
	filter := bson.M{}
	switch len(andClauses) {
	case 0:
		filter = bson.M{}
	case 1:
		filter = andClauses[0].(bson.M)
	default:
		filter = bson.M{"$and": andClauses}
	}
	cur, err := r.packages.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}, {Key: "namespace", Value: 1}, {Key: "name", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	out := []*Package{}
	for cur.Next(ctx) {
		var doc packageDocument
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		out = append(out, fromPackageDoc(doc))
	}
	return out, cur.Err()
}

func (r *Repository) GetVisiblePackage(ctx context.Context, tenantID, namespace, name string, includePublic bool) (*Package, error) {
	tenantID = strings.TrimSpace(tenantID)
	namespace = normalizeSegment(namespace)
	name = normalizeSegment(name)
	filter := bson.M{"namespace": namespace, "name": name}
	if includePublic {
		filter["$or"] = bson.A{bson.M{"tenant_id": tenantID}, bson.M{"visibility": VisibilityPublic}}
	} else {
		filter["tenant_id"] = tenantID
	}
	var doc packageDocument
	err := r.packages.FindOne(ctx, filter, options.FindOne().SetSort(bson.D{{Key: "updated_at", Value: -1}})).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("package not found")
		}
		return nil, err
	}
	return fromPackageDoc(doc), nil
}

func (r *Repository) CreatePackage(ctx context.Context, in CreatePackageInput) (*Package, error) {
	tenantID := strings.TrimSpace(in.TenantID)
	namespace := normalizeSegment(in.Namespace)
	name := normalizeSegment(in.Name)
	visibility := normalizeVisibility(in.Visibility)
	createdBy := strings.TrimSpace(in.CreatedBy)
	if tenantID == "" || namespace == "" || name == "" || createdBy == "" {
		return nil, fmt.Errorf("tenant_id, namespace, name, and actor are required")
	}
	if visibility == "" {
		return nil, fmt.Errorf("visibility must be public or tenant")
	}
	now := time.Now().UTC()
	doc := packageDocument{
		ID:         fmt.Sprintf("pkg_%d", now.UnixNano()),
		TenantID:   tenantID,
		Namespace:  namespace,
		Name:       name,
		Visibility: visibility,
		CreatedBy:  createdBy,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if _, err := r.packages.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, fmt.Errorf("package already exists")
		}
		return nil, err
	}
	return fromPackageDoc(doc), nil
}

func (r *Repository) PublishPackageVersion(ctx context.Context, in PublishPackageVersionInput) (*PackageVersion, error) {
	tenantID := strings.TrimSpace(in.TenantID)
	namespace := normalizeSegment(in.Namespace)
	name := normalizeSegment(in.Name)
	version := strings.TrimSpace(in.Version)
	publishedBy := strings.TrimSpace(in.PublishedBy)
	if tenantID == "" || namespace == "" || name == "" || version == "" || publishedBy == "" {
		return nil, fmt.Errorf("tenant_id, namespace, name, version, and actor are required")
	}
	pkg, err := r.GetVisiblePackage(ctx, tenantID, namespace, name, false)
	if err != nil {
		return nil, err
	}
	if pkg.TenantID != tenantID {
		return nil, fmt.Errorf("tenant mismatch")
	}
	now := time.Now().UTC()
	doc := packageVersionDocument{
		ID:           fmt.Sprintf("pkgv_%d", now.UnixNano()),
		PackageID:    pkg.ID,
		TenantID:     tenantID,
		Version:      version,
		ManifestJSON: cloneMap(in.Manifest),
		ArtifactKey:  canonicalArtifactKey(tenantID, namespace, name, version),
		Checksum:     strings.TrimSpace(in.Checksum),
		SizeBytes:    in.SizeBytes,
		PublishedBy:  publishedBy,
		PublishedAt:  now,
	}
	if _, err := r.packageVersions.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, fmt.Errorf("version already exists")
		}
		return nil, err
	}
	_, _ = r.packages.UpdateByID(ctx, pkg.ID, bson.M{
		"$set": bson.M{
			"latest_version": pickLatestVersion(pkg.LatestVersion, version),
			"updated_at":     now,
		},
	})
	return fromPackageVersionDoc(doc), nil
}

func (r *Repository) ListPackageVersions(ctx context.Context, packageID string) ([]*PackageVersion, error) {
	cur, err := r.packageVersions.Find(ctx, bson.M{"package_id": strings.TrimSpace(packageID)})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	out := []*PackageVersion{}
	for cur.Next(ctx) {
		var doc packageVersionDocument
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		out = append(out, fromPackageVersionDoc(doc))
	}
	if err := cur.Err(); err != nil {
		return nil, err
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

func (r *Repository) GetPackageVersion(ctx context.Context, packageID, version string) (*PackageVersion, error) {
	var doc packageVersionDocument
	err := r.packageVersions.FindOne(ctx, bson.M{"package_id": strings.TrimSpace(packageID), "version": strings.TrimSpace(version)}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("version not found")
		}
		return nil, err
	}
	return fromPackageVersionDoc(doc), nil
}

func (r *Repository) UpdatePackageVisibility(ctx context.Context, in UpdatePackageVisibilityInput) (*Package, error) {
	tenantID := strings.TrimSpace(in.TenantID)
	namespace := normalizeSegment(in.Namespace)
	name := normalizeSegment(in.Name)
	visibility := normalizeVisibility(in.Visibility)
	if tenantID == "" || namespace == "" || name == "" {
		return nil, fmt.Errorf("tenant_id, namespace, and name are required")
	}
	if visibility == "" {
		return nil, fmt.Errorf("visibility must be public or tenant")
	}
	var doc packageDocument
	err := r.packages.FindOneAndUpdate(
		ctx,
		bson.M{"tenant_id": tenantID, "namespace": namespace, "name": name},
		bson.M{"$set": bson.M{"visibility": visibility, "updated_at": time.Now().UTC()}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("package not found")
		}
		return nil, err
	}
	return fromPackageDoc(doc), nil
}

func (r *Repository) DeletePackage(ctx context.Context, in DeletePackageInput) error {
	tenantID := strings.TrimSpace(in.TenantID)
	namespace := normalizeSegment(in.Namespace)
	name := normalizeSegment(in.Name)
	if tenantID == "" || namespace == "" || name == "" {
		return fmt.Errorf("tenant_id, namespace, and name are required")
	}
	var doc packageDocument
	if err := r.packages.FindOne(ctx, bson.M{"tenant_id": tenantID, "namespace": namespace, "name": name}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return fmt.Errorf("package not found")
		}
		return err
	}
	if _, err := r.packages.DeleteOne(ctx, bson.M{"_id": doc.ID}); err != nil {
		return err
	}
	_, _ = r.packageVersions.DeleteMany(ctx, bson.M{"package_id": doc.ID})
	return nil
}

func (r *Repository) RecordAudit(ctx context.Context, ev AuditEvent) error {
	tm := time.Now().UTC()
	if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(ev.Time)); err == nil {
		tm = parsed.UTC()
	}
	actorID := strings.TrimSpace(ev.ActorID)
	if actorID == "" {
		actorID = "unknown"
	}
	tenantID := strings.TrimSpace(ev.TenantID)
	if tenantID == "" {
		tenantID = "unknown"
	}
	_, err := r.auditEvents.InsertOne(ctx, auditDocument{
		EventTime: tm,
		Action:    strings.TrimSpace(ev.Action),
		ActorID:   actorID,
		TenantID:  tenantID,
		Status:    strings.TrimSpace(ev.Status),
		RequestID: strings.TrimSpace(ev.RequestID),
		Details:   cloneMap(ev.Details),
	})
	return err
}

func (r *Repository) ListAudit(ctx context.Context, limit int) ([]AuditEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	cur, err := r.auditEvents.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "event_time", Value: -1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	out := []AuditEvent{}
	for cur.Next(ctx) {
		var doc auditDocument
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		out = append(out, AuditEvent{
			Time:      doc.EventTime.UTC().Format(time.RFC3339),
			Action:    doc.Action,
			ActorID:   doc.ActorID,
			TenantID:  doc.TenantID,
			Status:    doc.Status,
			RequestID: doc.RequestID,
			Details:   cloneMap(doc.Details),
		})
	}
	return out, cur.Err()
}

func fromPackageDoc(doc packageDocument) *Package {
	return &Package{
		ID:            doc.ID,
		TenantID:      doc.TenantID,
		Namespace:     doc.Namespace,
		Name:          doc.Name,
		LatestVersion: doc.LatestVersion,
		Visibility:    doc.Visibility,
		CreatedBy:     doc.CreatedBy,
		CreatedAt:     doc.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:     doc.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func fromPackageVersionDoc(doc packageVersionDocument) *PackageVersion {
	return &PackageVersion{
		ID:           doc.ID,
		PackageID:    doc.PackageID,
		TenantID:     doc.TenantID,
		Version:      doc.Version,
		ManifestJSON: cloneMap(doc.ManifestJSON),
		ArtifactKey:  doc.ArtifactKey,
		Checksum:     doc.Checksum,
		SizeBytes:    doc.SizeBytes,
		PublishedBy:  doc.PublishedBy,
		PublishedAt:  doc.PublishedAt.UTC().Format(time.RFC3339),
	}
}

func primitiveContainsRegex(value string) bson.M {
	return bson.M{"$regex": primitiveRegex(value)}
}

func primitiveRegex(value string) primitive.Regex {
	return primitive.Regex{Pattern: regexp.QuoteMeta(strings.ToLower(strings.TrimSpace(value))), Options: "i"}
}

func normalizeVisibility(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case VisibilityPublic:
		return VisibilityPublic
	case VisibilityTenant, "":
		if strings.TrimSpace(v) == "" {
			return VisibilityTenant
		}
		return VisibilityTenant
	default:
		return ""
	}
}

func normalizeSegment(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	return strings.ToLower(v)
}

func pickLatestVersion(current, incoming string) string {
	if strings.TrimSpace(current) == "" {
		return strings.TrimSpace(incoming)
	}
	if compareVersion(strings.TrimSpace(incoming), strings.TrimSpace(current)) > 0 {
		return strings.TrimSpace(incoming)
	}
	return strings.TrimSpace(current)
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func canonicalArtifactKey(tenantID, namespace, name, version string) string {
	return fmt.Sprintf(
		"tenants/%s/packages/%s/%s/%s/artifact.tar.gz",
		strings.TrimSpace(tenantID),
		normalizeSegment(namespace),
		normalizeSegment(name),
		strings.TrimSpace(version),
	)
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

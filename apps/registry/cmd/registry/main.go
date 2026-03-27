package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/runfabric/runfabric/registry/internal/server"
	"github.com/runfabric/runfabric/registry/internal/store"
)

func main() {
	var configPath string
	var listen string
	var webDir string
	var uiAuthURL string
	var uiDocsURL string
	var dbPath string
	var uploadsDir string
	var metadataProvider string
	var seedLocalDevData bool
	var postgresDSN string
	var postgresDriver string
	var mongodbURI string
	var mongodbDatabase string
	var redisAddr string
	var allowAnonymousRead bool
	var artifactSigningSecret string
	var oidcIssuer string
	var oidcAudience string
	var oidcJWKSURL string
	var oidcSubjectClaim string
	var oidcTenantClaim string
	var oidcRolesClaim string
	var oidcRoleModes string
	var oidcRoleClientID string
	var oidcAudienceMode string
	var oidcAllowedJWTAlgs string
	var casbinPolicyPath string
	var s3BaseURL string
	var s3Bucket string
	var s3Region string
	var s3Endpoint string
	var s3AccessKeyID string
	var s3SecretAccessKey string
	var s3SessionToken string
	flag.StringVar(&configPath, "config", "", "optional path to registry YAML config (env: REGISTRY_CONFIG)")
	flag.StringVar(&listen, "listen", "127.0.0.1:8787", "host:port to listen on")
	flag.StringVar(&webDir, "web-dir", "", "optional path to registry web static assets (for example ./web/dist)")
	flag.StringVar(&uiAuthURL, "ui-auth-url", "", "optional auth login URL used by registry web UI SSO button")
	flag.StringVar(&uiDocsURL, "ui-docs-url", "", "optional CLI docs URL used by registry web footer")
	flag.StringVar(&dbPath, "db", "", "path to registry JSON db (default: ./data/registry.db.json)")
	flag.StringVar(&uploadsDir, "uploads", "", "directory for staged/published upload blobs (default: ./data/uploads)")
	flag.StringVar(&metadataProvider, "metadata-provider", "auto", "metadata provider: auto|json|postgres|mongodb")
	flag.BoolVar(&seedLocalDevData, "seed-local-dev-data", false, "seed local-dev fixtures (for example rk_local_dev api key) in fresh metadata stores")
	flag.StringVar(&postgresDSN, "postgres-dsn", "", "optional Postgres DSN for package/auth/audit metadata")
	flag.StringVar(&postgresDriver, "postgres-driver", "pgx", "database/sql driver name for Postgres DSN")
	flag.StringVar(&mongodbURI, "mongodb-uri", "", "optional MongoDB connection URI for package/auth/audit metadata")
	flag.StringVar(&mongodbDatabase, "mongodb-database", "runfabric_registry", "MongoDB database name for metadata collections")
	flag.StringVar(&redisAddr, "redis-addr", "", "optional redis address host:port for read cache")
	flag.BoolVar(&allowAnonymousRead, "allow-anonymous-read", true, "allow anonymous read on public package endpoints")
	flag.StringVar(&artifactSigningSecret, "artifact-signing-secret", "", "secret for signing artifact upload/download URLs (default: REGISTRY_ARTIFACT_SIGNING_SECRET or local-dev value)")
	flag.StringVar(&oidcIssuer, "oidc-issuer", "", "optional OIDC issuer to validate on Bearer JWT")
	flag.StringVar(&oidcAudience, "oidc-audience", "", "optional OIDC audience to validate on Bearer JWT")
	flag.StringVar(&oidcJWKSURL, "oidc-jwks-url", "", "optional OIDC JWKS URL for JWT signature verification")
	flag.StringVar(&oidcSubjectClaim, "oidc-subject-claim", "", "OIDC claim key/path used for subject identity (default: sub)")
	flag.StringVar(&oidcTenantClaim, "oidc-tenant-claim", "", "OIDC claim key/path used for tenant identity (default: tenant_id)")
	flag.StringVar(&oidcRolesClaim, "oidc-roles-claim", "", "OIDC claim key/path used for roles mode=roles (default: roles)")
	flag.StringVar(&oidcRoleModes, "oidc-role-modes", "", "role extraction precedence modes: roles,realm_access.roles,resource_access.<client>.roles,scope")
	flag.StringVar(&oidcRoleClientID, "oidc-role-client-id", "", "client id used for resource_access.<client>.roles role mode")
	flag.StringVar(&oidcAudienceMode, "oidc-audience-mode", "", "audience validation mode: exact|includes|skip (default: exact)")
	flag.StringVar(&oidcAllowedJWTAlgs, "oidc-allowed-jwt-algs", "", "comma-separated JWT alg allowlist (for example RS256,ES256)")
	flag.StringVar(&casbinPolicyPath, "casbin-policy", "internal/adapter/policy/casbin/policy.csv", "casbin-style policy file for role/action enforcement")
	flag.StringVar(&s3BaseURL, "s3-base-url", "", "optional S3/S3-compatible base URL for artifact upload/download URLs")
	flag.StringVar(&s3Bucket, "s3-bucket", "", "optional S3 bucket for presigned artifact URLs")
	flag.StringVar(&s3Region, "s3-region", "", "optional S3 region for presigned artifact URLs")
	flag.StringVar(&s3Endpoint, "s3-endpoint", "", "optional S3 endpoint override (default: https://s3.<region>.amazonaws.com)")
	flag.StringVar(&s3AccessKeyID, "s3-access-key-id", "", "optional S3 access key ID for URL presigning")
	flag.StringVar(&s3SecretAccessKey, "s3-secret-access-key", "", "optional S3 secret access key for URL presigning")
	flag.StringVar(&s3SessionToken, "s3-session-token", "", "optional S3 session token for URL presigning")
	flag.Parse()

	flagSet := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { flagSet[f.Name] = true })
	if !flagSet["config"] && strings.TrimSpace(configPath) == "" {
		configPath = strings.TrimSpace(os.Getenv("REGISTRY_CONFIG"))
	}

	// Precedence: flags > env > config > defaults.
	if strings.TrimSpace(configPath) != "" {
		cfg, err := loadConfigFile(configPath)
		if err != nil {
			log.Fatalf("load config file: %v", err)
		}
		if !flagSet["listen"] && strings.TrimSpace(cfg.Server.Listen) != "" {
			listen = strings.TrimSpace(cfg.Server.Listen)
		}
		if !flagSet["web-dir"] && strings.TrimSpace(cfg.Server.WebDir) != "" {
			webDir = strings.TrimSpace(cfg.Server.WebDir)
		}
		if !flagSet["ui-auth-url"] && strings.TrimSpace(cfg.Server.UIAuthURL) != "" {
			uiAuthURL = strings.TrimSpace(cfg.Server.UIAuthURL)
		}
		if !flagSet["ui-docs-url"] && strings.TrimSpace(cfg.Server.UIDocsURL) != "" {
			uiDocsURL = strings.TrimSpace(cfg.Server.UIDocsURL)
		}
		if !flagSet["db"] && strings.TrimSpace(cfg.Storage.Config.DBPath) != "" {
			dbPath = strings.TrimSpace(cfg.Storage.Config.DBPath)
		}
		if !flagSet["uploads"] && strings.TrimSpace(cfg.Storage.Config.UploadsDir) != "" {
			uploadsDir = strings.TrimSpace(cfg.Storage.Config.UploadsDir)
		}
		if !flagSet["metadata-provider"] && strings.TrimSpace(cfg.Storage.Provider) != "" {
			metadataProvider = strings.TrimSpace(cfg.Storage.Provider)
		}
		if !flagSet["seed-local-dev-data"] && cfg.Storage.SeedLocalDevData != nil {
			seedLocalDevData = *cfg.Storage.SeedLocalDevData
		}
		if !flagSet["postgres-dsn"] && strings.TrimSpace(cfg.Storage.Config.DSN) != "" {
			postgresDSN = strings.TrimSpace(cfg.Storage.Config.DSN)
		}
		if !flagSet["postgres-driver"] && strings.TrimSpace(cfg.Storage.Config.Driver) != "" {
			postgresDriver = strings.TrimSpace(cfg.Storage.Config.Driver)
		}
		if !flagSet["mongodb-uri"] && strings.TrimSpace(cfg.Storage.Config.URI) != "" {
			mongodbURI = strings.TrimSpace(cfg.Storage.Config.URI)
		}
		if !flagSet["mongodb-database"] && strings.TrimSpace(cfg.Storage.Config.Database) != "" {
			mongodbDatabase = strings.TrimSpace(cfg.Storage.Config.Database)
		}
		if !flagSet["redis-addr"] && strings.TrimSpace(cfg.Storage.Cache.RedisAddr) != "" {
			redisAddr = strings.TrimSpace(cfg.Storage.Cache.RedisAddr)
		}
		if !flagSet["allow-anonymous-read"] && cfg.Auth.AllowAnonymousRead != nil {
			allowAnonymousRead = *cfg.Auth.AllowAnonymousRead
		}
		if !flagSet["artifact-signing-secret"] && strings.TrimSpace(cfg.Auth.ArtifactSigningSecret) != "" {
			artifactSigningSecret = strings.TrimSpace(cfg.Auth.ArtifactSigningSecret)
		}
		if !flagSet["oidc-issuer"] && strings.TrimSpace(cfg.Auth.OIDC.Issuer) != "" {
			oidcIssuer = strings.TrimSpace(cfg.Auth.OIDC.Issuer)
		}
		if !flagSet["oidc-audience"] && strings.TrimSpace(cfg.Auth.OIDC.Audience) != "" {
			oidcAudience = strings.TrimSpace(cfg.Auth.OIDC.Audience)
		}
		if !flagSet["oidc-jwks-url"] && strings.TrimSpace(cfg.Auth.OIDC.JWKSURL) != "" {
			oidcJWKSURL = strings.TrimSpace(cfg.Auth.OIDC.JWKSURL)
		}
		if !flagSet["oidc-subject-claim"] && strings.TrimSpace(cfg.Auth.OIDC.SubjectClaim) != "" {
			oidcSubjectClaim = strings.TrimSpace(cfg.Auth.OIDC.SubjectClaim)
		}
		if !flagSet["oidc-tenant-claim"] && strings.TrimSpace(cfg.Auth.OIDC.TenantClaim) != "" {
			oidcTenantClaim = strings.TrimSpace(cfg.Auth.OIDC.TenantClaim)
		}
		if !flagSet["oidc-roles-claim"] && strings.TrimSpace(cfg.Auth.OIDC.RolesClaim) != "" {
			oidcRolesClaim = strings.TrimSpace(cfg.Auth.OIDC.RolesClaim)
		}
		if !flagSet["oidc-role-modes"] && strings.TrimSpace(cfg.Auth.OIDC.RoleModes) != "" {
			oidcRoleModes = strings.TrimSpace(cfg.Auth.OIDC.RoleModes)
		}
		if !flagSet["oidc-role-client-id"] && strings.TrimSpace(cfg.Auth.OIDC.RoleClientID) != "" {
			oidcRoleClientID = strings.TrimSpace(cfg.Auth.OIDC.RoleClientID)
		}
		if !flagSet["oidc-audience-mode"] && strings.TrimSpace(cfg.Auth.OIDC.AudienceMode) != "" {
			oidcAudienceMode = strings.TrimSpace(cfg.Auth.OIDC.AudienceMode)
		}
		if !flagSet["oidc-allowed-jwt-algs"] && strings.TrimSpace(cfg.Auth.OIDC.AllowedJWTAlgs) != "" {
			oidcAllowedJWTAlgs = strings.TrimSpace(cfg.Auth.OIDC.AllowedJWTAlgs)
		}
		if !flagSet["casbin-policy"] && strings.TrimSpace(cfg.Auth.CasbinPolicyPath) != "" {
			casbinPolicyPath = strings.TrimSpace(cfg.Auth.CasbinPolicyPath)
		}
		if !flagSet["s3-base-url"] && strings.TrimSpace(cfg.Artifacts.Config.BaseURL) != "" {
			s3BaseURL = strings.TrimSpace(cfg.Artifacts.Config.BaseURL)
		}
		if !flagSet["s3-bucket"] && strings.TrimSpace(cfg.Artifacts.Config.Bucket) != "" {
			s3Bucket = strings.TrimSpace(cfg.Artifacts.Config.Bucket)
		}
		if !flagSet["s3-region"] && strings.TrimSpace(cfg.Artifacts.Config.Region) != "" {
			s3Region = strings.TrimSpace(cfg.Artifacts.Config.Region)
		}
		if !flagSet["s3-endpoint"] && strings.TrimSpace(cfg.Artifacts.Config.Endpoint) != "" {
			s3Endpoint = strings.TrimSpace(cfg.Artifacts.Config.Endpoint)
		}
		if !flagSet["s3-access-key-id"] && strings.TrimSpace(cfg.Artifacts.Config.AccessKeyID) != "" {
			s3AccessKeyID = strings.TrimSpace(cfg.Artifacts.Config.AccessKeyID)
		}
		if !flagSet["s3-secret-access-key"] && strings.TrimSpace(cfg.Artifacts.Config.SecretAccessKey) != "" {
			s3SecretAccessKey = strings.TrimSpace(cfg.Artifacts.Config.SecretAccessKey)
		}
		if !flagSet["s3-session-token"] && strings.TrimSpace(cfg.Artifacts.Config.SessionToken) != "" {
			s3SessionToken = strings.TrimSpace(cfg.Artifacts.Config.SessionToken)
		}
	}

	if !flagSet["listen"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_LISTEN")); v != "" {
			listen = v
		}
	}
	if !flagSet["web-dir"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_WEB_DIR")); v != "" {
			webDir = v
		}
	}
	if !flagSet["ui-auth-url"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_UI_AUTH_URL")); v != "" {
			uiAuthURL = v
		}
	}
	if !flagSet["ui-docs-url"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_UI_DOCS_URL")); v != "" {
			uiDocsURL = v
		}
	}
	if !flagSet["db"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_DB_PATH")); v != "" {
			dbPath = v
		}
	}
	if !flagSet["uploads"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_UPLOADS_DIR")); v != "" {
			uploadsDir = v
		}
	}
	if !flagSet["artifact-signing-secret"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_ARTIFACT_SIGNING_SECRET")); v != "" {
			artifactSigningSecret = v
		}
	}
	if !flagSet["allow-anonymous-read"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_ALLOW_ANONYMOUS_READ")); v != "" {
			if parsed, err := strconv.ParseBool(v); err == nil {
				allowAnonymousRead = parsed
			}
		}
	}
	if !flagSet["metadata-provider"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_METADATA_PROVIDER")); v != "" {
			metadataProvider = v
		}
	}
	if !flagSet["seed-local-dev-data"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_SEED_LOCAL_DEV_DATA")); v != "" {
			if parsed, err := strconv.ParseBool(v); err == nil {
				seedLocalDevData = parsed
			}
		}
	}
	if !flagSet["postgres-dsn"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_POSTGRES_DSN")); v != "" {
			postgresDSN = v
		}
	}
	if !flagSet["postgres-driver"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_POSTGRES_DRIVER")); v != "" {
			postgresDriver = v
		}
	}
	if !flagSet["mongodb-uri"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_MONGODB_URI")); v != "" {
			mongodbURI = v
		}
	}
	if !flagSet["mongodb-database"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_MONGODB_DATABASE")); v != "" {
			mongodbDatabase = v
		}
	}
	if !flagSet["redis-addr"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_REDIS_ADDR")); v != "" {
			redisAddr = v
		}
	}
	if !flagSet["oidc-issuer"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_OIDC_ISSUER")); v != "" {
			oidcIssuer = v
		}
	}
	if !flagSet["oidc-audience"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_OIDC_AUDIENCE")); v != "" {
			oidcAudience = v
		}
	}
	if !flagSet["oidc-jwks-url"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_OIDC_JWKS_URL")); v != "" {
			oidcJWKSURL = v
		}
	}
	if !flagSet["oidc-subject-claim"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_OIDC_SUBJECT_CLAIM")); v != "" {
			oidcSubjectClaim = v
		}
	}
	if !flagSet["oidc-tenant-claim"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_OIDC_TENANT_CLAIM")); v != "" {
			oidcTenantClaim = v
		}
	}
	if !flagSet["oidc-roles-claim"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_OIDC_ROLES_CLAIM")); v != "" {
			oidcRolesClaim = v
		}
	}
	if !flagSet["oidc-role-modes"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_OIDC_ROLE_MODES")); v != "" {
			oidcRoleModes = v
		}
	}
	if !flagSet["oidc-role-client-id"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_OIDC_ROLE_CLIENT_ID")); v != "" {
			oidcRoleClientID = v
		}
	}
	if !flagSet["oidc-audience-mode"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_OIDC_AUDIENCE_MODE")); v != "" {
			oidcAudienceMode = v
		}
	}
	if !flagSet["oidc-allowed-jwt-algs"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_OIDC_ALLOWED_JWT_ALGS")); v != "" {
			oidcAllowedJWTAlgs = v
		}
	}
	if !flagSet["casbin-policy"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_CASBIN_POLICY")); v != "" {
			casbinPolicyPath = v
		}
	}
	if !flagSet["s3-base-url"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_S3_BASE_URL")); v != "" {
			s3BaseURL = v
		}
	}
	if !flagSet["s3-bucket"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_S3_BUCKET")); v != "" {
			s3Bucket = v
		}
	}
	if !flagSet["s3-region"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_S3_REGION")); v != "" {
			s3Region = v
		}
	}
	if !flagSet["s3-endpoint"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_S3_ENDPOINT")); v != "" {
			s3Endpoint = v
		}
	}
	if !flagSet["s3-access-key-id"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_S3_ACCESS_KEY_ID")); v != "" {
			s3AccessKeyID = v
		}
	}
	if !flagSet["s3-secret-access-key"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_S3_SECRET_ACCESS_KEY")); v != "" {
			s3SecretAccessKey = v
		}
	}
	if !flagSet["s3-session-token"] {
		if v := strings.TrimSpace(os.Getenv("REGISTRY_S3_SESSION_TOKEN")); v != "" {
			s3SessionToken = v
		}
	}

	st, err := store.Open(store.OpenOptions{
		DBPath:           dbPath,
		UploadsDir:       uploadsDir,
		MetadataProvider: metadataProvider,
		SeedLocalDevData: seedLocalDevData,
		PostgresDSN:      postgresDSN,
		PostgresDriver:   postgresDriver,
		MongoDBURI:       mongodbURI,
		MongoDBDatabase:  mongodbDatabase,
	})
	if err != nil {
		log.Fatalf("open registry store: %v", err)
	}
	srv, err := server.New(server.Options{
		Store:                 st,
		WebDir:                webDir,
		UIAuthURL:             uiAuthURL,
		UIDocsURL:             uiDocsURL,
		AllowAnonymousRead:    allowAnonymousRead,
		ArtifactSigningSecret: artifactSigningSecret,
		RedisAddr:             redisAddr,
		OIDCIssuer:            oidcIssuer,
		OIDCAudience:          oidcAudience,
		OIDCJWKSURL:           oidcJWKSURL,
		OIDCSubjectClaim:      oidcSubjectClaim,
		OIDCTenantClaim:       oidcTenantClaim,
		OIDCRolesClaim:        oidcRolesClaim,
		OIDCRoleModes:         oidcRoleModes,
		OIDCRoleClientID:      oidcRoleClientID,
		OIDCAudienceMode:      oidcAudienceMode,
		OIDCAllowedJWTAlgs:    oidcAllowedJWTAlgs,
		CasbinPolicyPath:      casbinPolicyPath,
		S3BaseURL:             s3BaseURL,
		S3Bucket:              s3Bucket,
		S3Region:              s3Region,
		S3Endpoint:            s3Endpoint,
		S3AccessKeyID:         s3AccessKeyID,
		S3SecretAccessKey:     s3SecretAccessKey,
		S3SessionToken:        s3SessionToken,
	})
	if err != nil {
		log.Fatalf("init server: %v", err)
	}

	httpServer := &http.Server{
		Addr:              listen,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	log.Printf("registry listening on http://%s", listen)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		_ = st.Close()
		log.Fatal(err)
	}
	if err := st.Close(); err != nil {
		log.Printf("registry store close error: %v", err)
	}
}

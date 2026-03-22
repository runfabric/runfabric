package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// runtimeConfigFile uses the simplified config shape: storage.provider + storage.config, artifacts.provider + artifacts.config.
type runtimeConfigFile struct {
	Server struct {
		Listen    string `yaml:"listen"`
		WebDir    string `yaml:"web_dir"`
		UIAuthURL string `yaml:"ui_auth_url"`
		UIDocsURL string `yaml:"ui_docs_url"`
	} `yaml:"server"`
	Storage struct {
		Provider         string `yaml:"provider"`
		SeedLocalDevData *bool  `yaml:"seed_local_dev_data"`
		Config           struct {
			DSN        string `yaml:"dsn"`
			Driver     string `yaml:"driver"`
			URI        string `yaml:"uri"`
			Database   string `yaml:"database"`
			DBPath     string `yaml:"db_path"`
			UploadsDir string `yaml:"uploads_dir"`
		} `yaml:"config"`
		Cache struct {
			RedisAddr string `yaml:"redis_addr"`
		} `yaml:"cache"`
	} `yaml:"storage"`
	Auth struct {
		AllowAnonymousRead    *bool  `yaml:"allow_anonymous_read"`
		ArtifactSigningSecret string `yaml:"artifact_signing_secret"`
		CasbinPolicyPath      string `yaml:"casbin_policy_path"`
		OIDC                  struct {
			Issuer         string `yaml:"issuer"`
			Audience       string `yaml:"audience"`
			JWKSURL        string `yaml:"jwks_url"`
			SubjectClaim   string `yaml:"subject_claim"`
			TenantClaim    string `yaml:"tenant_claim"`
			RolesClaim     string `yaml:"roles_claim"`
			RoleModes      string `yaml:"role_modes"`
			RoleClientID   string `yaml:"role_client_id"`
			AudienceMode   string `yaml:"audience_mode"`
			AllowedJWTAlgs string `yaml:"allowed_jwt_algs"`
		} `yaml:"oidc"`
	} `yaml:"auth"`
	Artifacts struct {
		Provider string `yaml:"provider"`
		Config   struct {
			BaseURL         string `yaml:"base_url"`
			Bucket          string `yaml:"bucket"`
			Region          string `yaml:"region"`
			Endpoint        string `yaml:"endpoint"`
			AccessKeyID     string `yaml:"access_key_id"`
			SecretAccessKey string `yaml:"secret_access_key"`
			SessionToken    string `yaml:"session_token"`
		} `yaml:"config"`
	} `yaml:"artifacts"`
}

func loadConfigFile(path string) (*runtimeConfigFile, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("config path is empty")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	cfg := &runtimeConfigFile{}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

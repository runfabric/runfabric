package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// CacheKeyGenerator produces stable, scoped cache keys for MCP call results.
// Provider-specific implementations embed routing context (region, project, subscription)
// to prevent cross-boundary cache pollution in distributed backends.
type CacheKeyGenerator interface {
	CacheKey(server, action, target string, args map[string]any) string
}

// DefaultCacheKeyGenerator hashes server+action+target+arg-keys for a stable provider-neutral key.
type DefaultCacheKeyGenerator struct{}

func (DefaultCacheKeyGenerator) CacheKey(server, action, target string, args map[string]any) string {
	return hashCacheKey(server, action, target, argsKey(args))
}

// AWSCacheKeyGenerator includes the AWS region in the cache key to prevent cross-region
// pollution in backends like DynamoDB where data may be regionally replicated.
type AWSCacheKeyGenerator struct {
	Region string
}

func (g AWSCacheKeyGenerator) CacheKey(server, action, target string, args map[string]any) string {
	region := strings.TrimSpace(g.Region)
	if region == "" {
		region = "us-east-1"
	}
	return hashCacheKey("aws", region, server, action, target, argsKey(args))
}

// GCPCacheKeyGenerator includes the GCP project ID in the cache key to prevent
// cross-project pollution in Firestore-backed cache shards.
type GCPCacheKeyGenerator struct {
	Project string
}

func (g GCPCacheKeyGenerator) CacheKey(server, action, target string, args map[string]any) string {
	project := strings.TrimSpace(g.Project)
	if project == "" {
		project = "default-project"
	}
	return hashCacheKey("gcp", project, server, action, target, argsKey(args))
}

// AzureCacheKeyGenerator includes subscription and region in the key to avoid pollution
// across tenants and regions in Azure Cache for Redis deployments.
type AzureCacheKeyGenerator struct {
	Subscription string
	Region       string
}

func (g AzureCacheKeyGenerator) CacheKey(server, action, target string, args map[string]any) string {
	sub := strings.TrimSpace(g.Subscription)
	if sub == "" {
		sub = "default-sub"
	}
	region := strings.TrimSpace(g.Region)
	return hashCacheKey("azure", sub, region, server, action, target, argsKey(args))
}

// ProviderCacheKeyGenerator returns the appropriate CacheKeyGenerator for a cloud provider.
// Falls back to DefaultCacheKeyGenerator for unknown providers.
func ProviderCacheKeyGenerator(provider, region, project, subscription string) CacheKeyGenerator {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "aws-lambda":
		return AWSCacheKeyGenerator{Region: region}
	case "gcp-functions":
		return GCPCacheKeyGenerator{Project: project}
	case "azure-functions":
		return AzureCacheKeyGenerator{Subscription: subscription, Region: region}
	default:
		return DefaultCacheKeyGenerator{}
	}
}

// hashCacheKey computes a 16-char hex prefix of the SHA-256 of the joined parts.
func hashCacheKey(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// argsKey produces a stable, sorted string from arg map keys for inclusion in cache keys.
func argsKey(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return fmt.Sprintf("args[%s]", strings.Join(keys, ","))
}

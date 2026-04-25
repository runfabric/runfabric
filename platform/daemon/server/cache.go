package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultAPICacheKeyPrefix   = "runfabric:daemon:api:"
	defaultAPICacheStagePrefix = "runfabric:daemon:api:stage:"
)

// apiCache backs Config API responses in Redis (validate, resolve, plan, releases). Invalidate on deploy/remove.
type apiCache struct {
	client      *redis.Client
	ttl         time.Duration
	keyPrefix   string
	stagePrefix string
}

// cachedResponse is stored in Redis.
type cachedResponse struct {
	Status int    `json:"status"`
	Body   []byte `json:"body"`
}

func newAPICache(redisURL string, ttl time.Duration) *apiCache {
	redisURL = strings.TrimSpace(redisURL)
	if redisURL == "" || (!strings.HasPrefix(redisURL, "redis://") && !strings.HasPrefix(redisURL, "rediss://")) {
		return nil
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil
	}
	keyPrefix := strings.TrimSpace(os.Getenv("RUNFABRIC_DAEMON_CACHE_PREFIX"))
	if keyPrefix == "" {
		keyPrefix = defaultAPICacheKeyPrefix
	}
	if !strings.HasSuffix(keyPrefix, ":") {
		keyPrefix += ":"
	}
	stagePrefix := keyPrefix + "stage:"
	return &apiCache{
		client:      redis.NewClient(opt),
		ttl:         ttl,
		keyPrefix:   keyPrefix,
		stagePrefix: stagePrefix,
	}
}

func (c *apiCache) key(endpoint, bodyHash, stage string) string {
	return c.keyPrefix + endpoint + ":" + bodyHash + ":" + stage
}

func (c *apiCache) stageSetKey(stage string) string {
	return c.stagePrefix + stage
}

func (c *apiCache) get(endpoint, bodyHash, stage string) (status int, body []byte, ok bool) {
	if c == nil || c.client == nil {
		return 0, nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	k := c.key(endpoint, bodyHash, stage)
	b, err := c.client.Get(ctx, k).Bytes()
	if err == redis.Nil {
		return 0, nil, false
	}
	if err != nil {
		return 0, nil, false
	}
	var cr cachedResponse
	if err := json.Unmarshal(b, &cr); err != nil {
		return 0, nil, false
	}
	return cr.Status, cr.Body, true
}

func (c *apiCache) set(endpoint, bodyHash, stage string, status int, body []byte, ttl time.Duration) {
	if c == nil || c.client == nil {
		return
	}
	if ttl <= 0 {
		ttl = c.ttl
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	k := c.key(endpoint, bodyHash, stage)
	cr := cachedResponse{Status: status, Body: body}
	b, _ := json.Marshal(cr)
	_ = c.client.Set(ctx, k, b, ttl).Err()
	_ = c.client.SAdd(ctx, c.stageSetKey(stage), k).Err()
}

func (c *apiCache) invalidateStage(stage string) {
	if c == nil || c.client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	setKey := c.stageSetKey(stage)
	keys, err := c.client.SMembers(ctx, setKey).Result()
	if err != nil {
		return
	}
	for _, k := range keys {
		_ = c.client.Del(ctx, k).Err()
	}
	_ = c.client.Del(ctx, setKey).Err()
}

// apiCacheTTL returns TTL per endpoint (validate/resolve/plan/releases).
func apiCacheTTL(endpoint string) time.Duration {
	switch endpoint {
	case "validate":
		return 10 * time.Minute
	case "releases":
		return 1 * time.Minute
	default:
		return 5 * time.Minute // resolve, plan
	}
}

// apiCacheMiddleware wraps the Config API handler with Redis caching for validate, resolve, plan, releases. On deploy/remove success, invalidates cache for that stage.
func apiCacheMiddleware(cache *apiCache, defaultStage string, next http.Handler) http.Handler {
	cacheable := map[string]bool{"validate": true, "resolve": true, "plan": true, "releases": true}
	mutating := map[string]bool{"deploy": true, "remove": true}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		stage := r.URL.Query().Get("stage")
		if stage == "" {
			stage = defaultStage
		}

		body, err := io.ReadAll(r.Body)
		_ = r.Body.Close()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		h := sha256.Sum256(body)
		bodyHash := hex.EncodeToString(h[:])

		if cache != nil && cacheable[path] {
			if status, cachedBody, ok := cache.get(path, bodyHash, stage); ok {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_, _ = w.Write(cachedBody)
				return
			}
		}

		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK, body: &bytes.Buffer{}}
		next.ServeHTTP(rec, r)

		if cache != nil {
			if cacheable[path] && rec.status >= 200 && rec.status < 300 {
				ttl := apiCacheTTL(path)
				if cache.ttl > 0 && ttl > cache.ttl {
					ttl = cache.ttl
				}
				cache.set(path, bodyHash, stage, rec.status, rec.body.Bytes(), ttl)
			}
			if mutating[path] && rec.status >= 200 && rec.status < 300 {
				cache.invalidateStage(stage)
			}
		}
	})
}

type responseRecorder struct {
	http.ResponseWriter
	status int
	body   *bytes.Buffer
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

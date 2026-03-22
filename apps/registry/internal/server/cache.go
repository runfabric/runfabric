package server

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type responseCache interface {
	Get(key string) ([]byte, bool)
	Set(key string, value []byte, ttl time.Duration)
}

type noopCache struct{}

func (noopCache) Get(string) ([]byte, bool)         { return nil, false }
func (noopCache) Set(string, []byte, time.Duration) {}

type memoryCache struct {
	mu   sync.RWMutex
	data map[string]cacheEntry
}

type cacheEntry struct {
	value   []byte
	expires time.Time
}

func newMemoryCache() *memoryCache {
	return &memoryCache{data: map[string]cacheEntry{}}
}

func (m *memoryCache) Get(key string) ([]byte, bool) {
	now := time.Now().UTC()
	m.mu.RLock()
	entry, ok := m.data[key]
	m.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if !entry.expires.IsZero() && now.After(entry.expires) {
		m.mu.Lock()
		delete(m.data, key)
		m.mu.Unlock()
		return nil, false
	}
	return append([]byte(nil), entry.value...), true
}

func (m *memoryCache) Set(key string, value []byte, ttl time.Duration) {
	exp := time.Time{}
	if ttl > 0 {
		exp = time.Now().UTC().Add(ttl)
	}
	m.mu.Lock()
	m.data[key] = cacheEntry{value: append([]byte(nil), value...), expires: exp}
	m.mu.Unlock()
}

type redisCache struct {
	addr    string
	timeout time.Duration
}

func newRedisCache(addr string, timeout time.Duration) *redisCache {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil
	}
	return &redisCache{addr: addr, timeout: timeout}
}

func (r *redisCache) Get(key string) ([]byte, bool) {
	if r == nil {
		return nil, false
	}
	reply, err := r.doCommand("GET", key)
	if err != nil || len(reply) == 0 {
		return nil, false
	}
	switch reply[0] {
	case '$':
		if bytes.Equal(reply, []byte("$-1\r\n")) {
			return nil, false
		}
		return parseBulkReply(reply)
	default:
		return nil, false
	}
}

func (r *redisCache) Set(key string, value []byte, ttl time.Duration) {
	if r == nil {
		return
	}
	secs := int(ttl.Seconds())
	if secs <= 0 {
		secs = 60
	}
	_, _ = r.doCommand("SETEX", key, strconv.Itoa(secs), string(value))
}

func (r *redisCache) doCommand(parts ...string) ([]byte, error) {
	conn, err := net.DialTimeout("tcp", r.addr, r.timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(r.timeout))
	var req bytes.Buffer
	req.WriteString(fmt.Sprintf("*%d\r\n", len(parts)))
	for _, p := range parts {
		req.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(p), p))
	}
	if _, err := conn.Write(req.Bytes()); err != nil {
		return nil, err
	}
	reader := bufio.NewReader(conn)
	reply, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	prefix := byte(0)
	if len(reply) > 0 {
		prefix = reply[0]
	}
	if prefix == '$' {
		n, err := parseBulkLength(reply)
		if err != nil {
			return nil, err
		}
		if n < 0 {
			return reply, nil
		}
		body := make([]byte, n+2)
		if _, err := reader.Read(body); err != nil {
			return nil, err
		}
		return append(reply, body...), nil
	}
	return reply, nil
}

func parseBulkLength(header []byte) (int, error) {
	s := strings.TrimSpace(strings.TrimPrefix(string(header), "$"))
	return strconv.Atoi(s)
}

func parseBulkReply(reply []byte) ([]byte, bool) {
	i := bytes.Index(reply, []byte("\r\n"))
	if i <= 1 {
		return nil, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(reply[1:i])))
	if err != nil || n < 0 {
		return nil, false
	}
	start := i + 2
	end := start + n
	if end > len(reply) {
		return nil, false
	}
	return append([]byte(nil), reply[start:end]...), true
}

type layeredCache struct {
	mem   *memoryCache
	redis *redisCache
}

func newLayeredCache(redisAddr string) responseCache {
	mem := newMemoryCache()
	redis := newRedisCache(redisAddr, 2*time.Second)
	if redis == nil {
		return mem
	}
	return &layeredCache{mem: mem, redis: redis}
}

func (l *layeredCache) Get(key string) ([]byte, bool) {
	if v, ok := l.mem.Get(key); ok {
		return v, true
	}
	if l.redis != nil {
		if v, ok := l.redis.Get(key); ok {
			l.mem.Set(key, v, 30*time.Second)
			return v, true
		}
	}
	return nil, false
}

func (l *layeredCache) Set(key string, value []byte, ttl time.Duration) {
	l.mem.Set(key, value, ttl)
	if l.redis != nil {
		l.redis.Set(key, value, ttl)
	}
}

package server

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

type jwksCache struct {
	mu        sync.RWMutex
	keys      map[string]crypto.PublicKey
	sourceURL string
	fetchedAt time.Time
}

func newJWKSCache() *jwksCache {
	return &jwksCache{keys: map[string]crypto.PublicKey{}}
}

type oidcDiscoveryCache struct {
	mu        sync.RWMutex
	jwksURL   string
	fetchedAt time.Time
}

func newOIDCDiscoveryCache() *oidcDiscoveryCache {
	return &oidcDiscoveryCache{}
}

func (s *Server) parseVerifiedJWTClaims(token string) (map[string]any, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed jwt")
	}
	headerRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	var header map[string]any
	if err := json.Unmarshal(headerRaw, &header); err != nil {
		return nil, err
	}

	claims, err := parseJWTClaims(token)
	if err != nil {
		return nil, err
	}
	if err := s.verifyStandardClaims(claims); err != nil {
		return nil, err
	}
	jwksURL, err := s.effectiveJWKSURL()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(jwksURL) == "" {
		return claims, nil
	}
	alg := strings.ToUpper(strings.TrimSpace(firstClaimString(header, "alg")))
	if alg == "" {
		alg = "RS256"
	}
	if !s.oidcAllowedJWTAlgs[alg] {
		return nil, fmt.Errorf("unsupported jwt alg: %s", alg)
	}
	kid := strings.TrimSpace(firstClaimString(header, "kid"))
	pub, err := s.lookupJWKSKey(jwksURL, kid)
	if err != nil {
		return nil, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, err
	}
	signingInput := []byte(parts[0] + "." + parts[1])
	if err := verifyJWTSignature(alg, pub, signingInput, sig); err != nil {
		return nil, fmt.Errorf("jwt signature verification failed")
	}
	return claims, nil
}

func (s *Server) verifyStandardClaims(claims map[string]any) error {
	now := time.Now().UTC().Unix()
	if exp, ok := claimInt64(claims, "exp"); ok && now >= exp {
		return fmt.Errorf("token expired")
	}
	if nbf, ok := claimInt64(claims, "nbf"); ok && now < nbf {
		return fmt.Errorf("token not yet valid")
	}
	if issuer := strings.TrimSpace(s.oidcIssuer); issuer != "" {
		if got := strings.TrimSpace(firstClaimString(claims, "iss")); got != issuer {
			return fmt.Errorf("token issuer mismatch")
		}
	}
	if audience := strings.TrimSpace(s.oidcAudience); audience != "" {
		mode := normalizeAudienceMode(s.oidcAudienceMode)
		if mode != "skip" && !claimAudienceContains(claims, audience, mode) {
			return fmt.Errorf("token audience mismatch")
		}
	}
	return nil
}

func claimInt64(claims map[string]any, key string) (int64, bool) {
	v, ok := claims[key]
	if !ok || v == nil {
		return 0, false
	}
	switch typed := v.(type) {
	case float64:
		return int64(typed), true
	case int64:
		return typed, true
	case json.Number:
		n, err := typed.Int64()
		if err == nil {
			return n, true
		}
	}
	return 0, false
}

func claimAudienceContains(claims map[string]any, audience string, mode string) bool {
	raw, ok := claims["aud"]
	if !ok {
		return false
	}
	audience = strings.TrimSpace(audience)
	switch typed := raw.(type) {
	case string:
		val := strings.TrimSpace(typed)
		if mode == "includes" && strings.Contains(val, audience) {
			return true
		}
		if val == audience {
			return true
		}
		for _, part := range strings.FieldsFunc(val, func(r rune) bool {
			return r == ' ' || r == ','
		}) {
			if strings.TrimSpace(part) == audience {
				return true
			}
		}
		return false
	case []any:
		for _, v := range typed {
			s, ok := v.(string)
			if !ok {
				continue
			}
			s = strings.TrimSpace(s)
			if s == audience {
				return true
			}
			if mode == "includes" && strings.Contains(s, audience) {
				return true
			}
		}
	}
	return false
}

func (s *Server) lookupJWKSKey(jwksURL, kid string) (crypto.PublicKey, error) {
	if s.jwksCache == nil {
		return nil, fmt.Errorf("jwks cache unavailable")
	}
	s.jwksCache.mu.RLock()
	if strings.TrimSpace(s.jwksCache.sourceURL) == strings.TrimSpace(jwksURL) &&
		time.Since(s.jwksCache.fetchedAt) < 5*time.Minute &&
		len(s.jwksCache.keys) > 0 {
		if key := s.pickJWKSKeyLocked(kid); key != nil {
			s.jwksCache.mu.RUnlock()
			return key, nil
		}
	}
	s.jwksCache.mu.RUnlock()

	if err := s.refreshJWKSKeys(jwksURL); err != nil {
		return nil, err
	}
	s.jwksCache.mu.RLock()
	defer s.jwksCache.mu.RUnlock()
	if key := s.pickJWKSKeyLocked(kid); key != nil {
		return key, nil
	}
	return nil, fmt.Errorf("jwks key not found")
}

func (s *Server) pickJWKSKeyLocked(kid string) crypto.PublicKey {
	if s.jwksCache == nil {
		return nil
	}
	if strings.TrimSpace(kid) != "" {
		if k, ok := s.jwksCache.keys[strings.TrimSpace(kid)]; ok {
			return k
		}
	}
	for _, k := range s.jwksCache.keys {
		return k
	}
	return nil
}

func (s *Server) refreshJWKSKeys(jwksURL string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest(http.MethodGet, jwksURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("jwks fetch failed with status %s", resp.Status)
	}
	var doc struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			Use string `json:"use"`
			Alg string `json:"alg"`
			N   string `json:"n"`
			E   string `json:"e"`
			Crv string `json:"crv"`
			X   string `json:"x"`
			Y   string `json:"y"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return err
	}
	keys := map[string]crypto.PublicKey{}
	for _, k := range doc.Keys {
		kty := strings.ToUpper(strings.TrimSpace(k.Kty))
		if kty == "" {
			continue
		}
		var pub crypto.PublicKey
		switch kty {
		case "RSA":
			nBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(k.N))
			if err != nil {
				continue
			}
			eBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(k.E))
			if err != nil {
				continue
			}
			n := new(big.Int).SetBytes(nBytes)
			eInt := 0
			for _, b := range eBytes {
				eInt = eInt<<8 + int(b)
			}
			if n == nil || eInt <= 1 {
				continue
			}
			pub = &rsa.PublicKey{N: n, E: eInt}
		case "EC":
			ec, err := parseECPublicKey(k.Crv, k.X, k.Y)
			if err != nil {
				continue
			}
			pub = ec
		default:
			continue
		}
		kid := strings.TrimSpace(k.Kid)
		if kid == "" {
			kid = fmt.Sprintf("kid_%d", len(keys)+1)
		}
		keys[kid] = pub
	}
	if len(keys) == 0 {
		return fmt.Errorf("jwks contains no usable keys")
	}
	s.jwksCache.mu.Lock()
	s.jwksCache.keys = keys
	s.jwksCache.sourceURL = strings.TrimSpace(jwksURL)
	s.jwksCache.fetchedAt = time.Now().UTC()
	s.jwksCache.mu.Unlock()
	return nil
}

func (s *Server) effectiveJWKSURL() (string, error) {
	if strings.TrimSpace(s.oidcJWKSURL) != "" {
		return strings.TrimSpace(s.oidcJWKSURL), nil
	}
	issuer := strings.TrimSpace(s.oidcIssuer)
	if issuer == "" {
		return "", nil
	}
	if s.oidcDiscovery == nil {
		s.oidcDiscovery = newOIDCDiscoveryCache()
	}
	s.oidcDiscovery.mu.RLock()
	if strings.TrimSpace(s.oidcDiscovery.jwksURL) != "" && time.Since(s.oidcDiscovery.fetchedAt) < 10*time.Minute {
		j := s.oidcDiscovery.jwksURL
		s.oidcDiscovery.mu.RUnlock()
		return j, nil
	}
	s.oidcDiscovery.mu.RUnlock()

	discoveryURL := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest(http.MethodGet, discoveryURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("oidc discovery failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("oidc discovery failed with status %s", resp.Status)
	}
	var doc struct {
		JWKSURI string `json:"jwks_uri"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return "", fmt.Errorf("oidc discovery parse failed: %w", err)
	}
	if strings.TrimSpace(doc.JWKSURI) == "" {
		return "", fmt.Errorf("oidc discovery missing jwks_uri")
	}
	s.oidcDiscovery.mu.Lock()
	s.oidcDiscovery.jwksURL = strings.TrimSpace(doc.JWKSURI)
	s.oidcDiscovery.fetchedAt = time.Now().UTC()
	s.oidcDiscovery.mu.Unlock()
	return strings.TrimSpace(doc.JWKSURI), nil
}

func parseECPublicKey(curveName, xB64, yB64 string) (*ecdsa.PublicKey, error) {
	var curve elliptic.Curve
	switch strings.ToUpper(strings.TrimSpace(curveName)) {
	case "P-256":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	case "P-521":
		curve = elliptic.P521()
	default:
		return nil, fmt.Errorf("unsupported EC curve")
	}
	xBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(xB64))
	if err != nil {
		return nil, err
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(yB64))
	if err != nil {
		return nil, err
	}
	x := new(big.Int).SetBytes(xBytes)
	y := new(big.Int).SetBytes(yBytes)
	if !curve.IsOnCurve(x, y) {
		return nil, fmt.Errorf("ec key point is not on curve")
	}
	return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
}

func verifyJWTSignature(alg string, key crypto.PublicKey, signingInput, signature []byte) error {
	hash, err := hashForAlgorithm(alg)
	if err != nil {
		return err
	}
	switch {
	case strings.HasPrefix(alg, "RS"):
		pub, ok := key.(*rsa.PublicKey)
		if !ok {
			return fmt.Errorf("jwks key type mismatch for %s", alg)
		}
		digest, err := hashInput(hash, signingInput)
		if err != nil {
			return err
		}
		return rsa.VerifyPKCS1v15(pub, hash, digest, signature)
	case strings.HasPrefix(alg, "ES"):
		pub, ok := key.(*ecdsa.PublicKey)
		if !ok {
			return fmt.Errorf("jwks key type mismatch for %s", alg)
		}
		digest, err := hashInput(hash, signingInput)
		if err != nil {
			return err
		}
		r, s, err := decodeJWTECDSASignature(signature, pub)
		if err != nil {
			return err
		}
		if !ecdsa.Verify(pub, digest, r, s) {
			return fmt.Errorf("ecdsa verify failed")
		}
		return nil
	default:
		return fmt.Errorf("unsupported jwt alg: %s", alg)
	}
}

func hashForAlgorithm(alg string) (crypto.Hash, error) {
	switch strings.ToUpper(strings.TrimSpace(alg)) {
	case "RS256", "ES256":
		return crypto.SHA256, nil
	case "RS384", "ES384":
		return crypto.SHA384, nil
	case "RS512", "ES512":
		return crypto.SHA512, nil
	default:
		return 0, fmt.Errorf("unsupported jwt alg: %s", alg)
	}
}

func hashInput(hash crypto.Hash, input []byte) ([]byte, error) {
	switch hash {
	case crypto.SHA256:
		sum := sha256.Sum256(input)
		return sum[:], nil
	case crypto.SHA384:
		sum := sha512.Sum384(input)
		return sum[:], nil
	case crypto.SHA512:
		sum := sha512.Sum512(input)
		return sum[:], nil
	default:
		return nil, fmt.Errorf("unsupported hash")
	}
}

func decodeJWTECDSASignature(sig []byte, pub *ecdsa.PublicKey) (*big.Int, *big.Int, error) {
	if pub == nil || pub.Params() == nil {
		return nil, nil, fmt.Errorf("missing ecdsa key")
	}
	size := (pub.Params().BitSize + 7) / 8
	if len(sig) != 2*size {
		return nil, nil, fmt.Errorf("invalid ecdsa signature length")
	}
	r := new(big.Int).SetBytes(sig[:size])
	s := new(big.Int).SetBytes(sig[size:])
	if r.Sign() <= 0 || s.Sign() <= 0 {
		return nil, nil, fmt.Errorf("invalid ecdsa signature")
	}
	return r, s, nil
}

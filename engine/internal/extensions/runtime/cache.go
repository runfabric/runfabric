package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func BuildKey(parts map[string]any) (string, error) {
	b, err := json.Marshal(parts)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

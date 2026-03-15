package transactions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func Archive(root string, jf *JournalFile) (string, error) {
	dir := filepath.Join(root, ".runfabric", "journal-archive")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	path := filepath.Join(dir, fmt.Sprintf("%s-%s-%d.json", jf.Service, jf.Stage, time.Now().Unix()))
	data, err := json.MarshalIndent(jf, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

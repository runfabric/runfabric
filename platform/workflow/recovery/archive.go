package recovery

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/runfabric/runfabric/platform/core/state/transactions"
)

func ArchiveJournal(root string, jf *transactions.JournalFile) (string, error) {
	if jf == nil {
		return "", fmt.Errorf("nil journal")
	}

	dir := filepath.Join(root, ".runfabric", "journal-archive")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create archive dir: %w", err)
	}

	filename := fmt.Sprintf("%s-%s-%d.archived.json", jf.Service, jf.Stage, time.Now().Unix())
	path := filepath.Join(dir, filename)

	data, err := json.MarshalIndent(jf, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal archived journal: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write archived journal: %w", err)
	}

	return path, nil
}

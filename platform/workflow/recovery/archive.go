package recovery

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	statecore "github.com/runfabric/runfabric/platform/core/state/core"
	"github.com/runfabric/runfabric/platform/core/state/transactions"
)

func ArchiveJournal(root string, jf *transactions.JournalFile) (string, error) {
	if jf == nil {
		return "", fmt.Errorf("nil journal")
	}

	filename := fmt.Sprintf("%s-%s-%d.archived.json", jf.Service, jf.Stage, time.Now().Unix())
	path := filepath.Join(root, ".runfabric", "journal-archive", filename)

	data, err := json.MarshalIndent(jf, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal archived journal: %w", err)
	}

	// Journals hold deploy transaction I/O (often connection strings/credentials);
	// write owner-only and atomically via the shared state writer.
	if err := statecore.WriteStateFile(path, data); err != nil {
		return "", fmt.Errorf("write archived journal: %w", err)
	}

	return path, nil
}

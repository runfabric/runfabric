package transactions

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	statecore "github.com/runfabric/runfabric/platform/core/state/core"
)

func Archive(root string, jf *JournalFile) (string, error) {
	dir := filepath.Join(root, ".runfabric", "journal-archive")
	path := filepath.Join(dir, fmt.Sprintf("%s-%s-%d.json", jf.Service, jf.Stage, time.Now().Unix()))
	data, err := json.MarshalIndent(jf, "", "  ")
	if err != nil {
		return "", err
	}

	if err := statecore.WriteStateFile(path, data); err != nil {
		return "", err
	}
	return path, nil
}

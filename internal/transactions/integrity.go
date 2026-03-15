package transactions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func ComputeChecksum(j *JournalFile) (string, error) {
	copyJ := *j
	copyJ.Checksum = ""

	data, err := json.Marshal(copyJ)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func VerifyChecksum(j *JournalFile) (bool, error) {
	expected, err := ComputeChecksum(j)
	if err != nil {
		return false, err
	}
	return j.Checksum == expected, nil
}

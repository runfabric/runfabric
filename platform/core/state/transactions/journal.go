package transactions

import "time"

type Journal struct {
	file    *JournalFile
	backend Backend
}

// NewJournalFromFile wraps an existing JournalFile so the engine can resume from
// previously completed checkpoints rather than starting fresh.
func NewJournalFromFile(file *JournalFile, backend Backend) *Journal {
	return &Journal{file: file, backend: backend}
}

func NewJournal(service, stage, operation string, backend Backend) *Journal {
	now := time.Now().UTC().Format(time.RFC3339)

	return &Journal{
		file: &JournalFile{
			Service:       service,
			Stage:         stage,
			Operation:     operation,
			Status:        StatusActive,
			StartedAt:     now,
			UpdatedAt:     now,
			Version:       1,
			AttemptCount:  0,
			LastAttemptAt: "",
			Checkpoints:   []JournalCheckpoint{},
			Operations:    []Operation{},
		},
		backend: backend,
	}
}

func (j *Journal) Save() error {
	return j.persist()
}

func (j *Journal) Record(op Operation) error {
	j.file.Operations = append(j.file.Operations, op)
	return j.persist()
}

func (j *Journal) Checkpoint(name, status string) error {
	updated := false
	for i := range j.file.Checkpoints {
		if j.file.Checkpoints[i].Name == name {
			j.file.Checkpoints[i].Status = status
			updated = true
			break
		}
	}
	if !updated {
		j.file.Checkpoints = append(j.file.Checkpoints, JournalCheckpoint{
			Name:   name,
			Status: status,
		})
	}
	return j.backend.Save(j.file)
}

func (j *Journal) IncrementAttempt() error {
	j.file.AttemptCount++
	j.file.LastAttemptAt = time.Now().UTC().Format(time.RFC3339)
	return j.backend.Save(j.file)
}

func (j *Journal) MarkRollingBack() error {
	j.file.Status = StatusRollingBack
	return j.backend.Save(j.file)
}

func (j *Journal) MarkRolledBack() error {
	j.file.Status = StatusRolledBack
	return j.backend.Save(j.file)
}

func (j *Journal) MarkCompleted() error {
	j.file.Status = StatusCompleted
	return j.backend.Save(j.file)
}

func (j *Journal) MarkArchived() error {
	j.file.Status = StatusArchived
	return j.backend.Save(j.file)
}

func (j *Journal) Delete() error {
	return j.backend.Delete(j.file.Service, j.file.Stage)
}

func (j *Journal) Reverse() []Operation {
	out := make([]Operation, 0, len(j.file.Operations))
	for i := len(j.file.Operations) - 1; i >= 0; i-- {
		out = append(out, j.file.Operations[i])
	}
	return out
}

func (j *Journal) File() *JournalFile {
	return j.file
}

func (j *Journal) persist() error {
	j.file.Version++
	checksum, err := ComputeChecksum(j.file)
	if err != nil {
		return err
	}
	j.file.Checksum = checksum
	return j.backend.Save(j.file)
}

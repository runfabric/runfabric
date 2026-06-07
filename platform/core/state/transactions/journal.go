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
	return j.commit()
}

func (j *Journal) Record(op Operation) error {
	j.file.Operations = append(j.file.Operations, op)
	return j.commit()
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
	return j.commit()
}

func (j *Journal) IncrementAttempt() error {
	j.file.AttemptCount++
	j.file.LastAttemptAt = time.Now().UTC().Format(time.RFC3339)
	return j.commit()
}

func (j *Journal) MarkRollingBack() error {
	j.file.Status = StatusRollingBack
	return j.commit()
}

func (j *Journal) MarkRolledBack() error {
	j.file.Status = StatusRolledBack
	return j.commit()
}

func (j *Journal) MarkCompleted() error {
	j.file.Status = StatusCompleted
	return j.commit()
}

func (j *Journal) MarkArchived() error {
	j.file.Status = StatusArchived
	return j.commit()
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

// commit bumps the version, stamps UpdatedAt, and recomputes the integrity
// checksum over the current file contents, then hands the file to the backend to
// write. Every mutator goes through here so (a) the persisted checksum always
// matches the bytes on disk — the backend must not mutate the file afterward —
// and (b) every write advances the version, which lets the backend reject a
// concurrent write that did not observe this revision (lost-update detection).
func (j *Journal) commit() error {
	j.file.Version++
	j.file.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	checksum, err := ComputeChecksum(j.file)
	if err != nil {
		return err
	}
	j.file.Checksum = checksum
	return j.backend.Save(j.file)
}

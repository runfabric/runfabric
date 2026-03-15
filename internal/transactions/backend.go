package transactions

type Backend interface {
	Load(service, stage string) (*JournalFile, error)
	Save(j *JournalFile) error
	Delete(service, stage string) error
}

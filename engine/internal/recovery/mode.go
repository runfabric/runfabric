package recovery

type Mode string

const (
	ModeRollback Mode = "rollback"
	ModeResume   Mode = "resume"
	ModeInspect  Mode = "inspect"
)

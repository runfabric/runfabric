package errors

import "fmt"

type ConflictError struct {
	Backend         string
	Service         string
	Stage           string
	Resource        string
	CurrentVersion  int
	IncomingVersion int
	Action          string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf(
		"journal conflict backend=%s service=%s stage=%s resource=%s current=%d incoming=%d; action=%s",
		e.Backend,
		e.Service,
		e.Stage,
		e.Resource,
		e.CurrentVersion,
		e.IncomingVersion,
		e.Action,
	)
}

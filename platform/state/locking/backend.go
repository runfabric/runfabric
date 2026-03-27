package locking

import "time"

type Backend interface {
	Acquire(service, stage, operation string, staleAfter time.Duration) (*Handle, error)
	Read(service, stage string) (*LockRecord, error)
	Release(service, stage string) error
}

type LeaseBackend interface {
	Backend
	Renew(service, stage, ownerToken string, leaseFor time.Duration) error
	Steal(service, stage, newOperation string, staleAfter time.Duration) (*Handle, error)
	ReleaseOwned(service, stage, ownerToken string) error
}

type Releaser interface {
	Release(service, stage string) error
}

type Renewer interface {
	Renew(service, stage, ownerToken string, leaseFor time.Duration) error
}

type OwnedReleaser interface {
	ReleaseOwned(service, stage, ownerToken string) error
}

type Handle struct {
	Service    string
	Stage      string
	OwnerToken string
	Held       bool
	r          Releaser
	n          Renewer
	or         OwnedReleaser
}

func NewHandle(service, stage, ownerToken string, r Releaser, n Renewer, or OwnedReleaser) *Handle {
	return &Handle{
		Service:    service,
		Stage:      stage,
		OwnerToken: ownerToken,
		Held:       true,
		r:          r,
		n:          n,
		or:         or,
	}
}

func (h *Handle) Release() error {
	if h == nil || !h.Held {
		return nil
	}

	var err error
	if h.or != nil && h.OwnerToken != "" {
		err = h.or.ReleaseOwned(h.Service, h.Stage, h.OwnerToken)
	} else {
		err = h.r.Release(h.Service, h.Stage)
	}
	if err != nil {
		return err
	}

	h.Held = false
	return nil
}

func (h *Handle) Renew(leaseFor time.Duration) error {
	if h == nil || !h.Held || h.n == nil {
		return nil
	}
	return h.n.Renew(h.Service, h.Stage, h.OwnerToken, leaseFor)
}

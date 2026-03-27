package app

import state "github.com/runfabric/runfabric/platform/core/state/core"

// ReleasesResult is the result of Releases (deploy history per stage).
type ReleasesResult struct {
	Service  string               `json:"service"`
	Releases []state.ReleaseEntry `json:"releases"`
}

// Releases returns deployment history (stages and updated timestamps) from the receipt backend.
func Releases(configPath string) (any, error) {
	ctx, err := Bootstrap(configPath, "dev", "")
	if err != nil {
		return nil, err
	}
	list, err := ctx.Backends.Receipts.ListReleases()
	if err != nil {
		return nil, err
	}
	return &ReleasesResult{
		Service:  ctx.Config.Service,
		Releases: list,
	}, nil
}

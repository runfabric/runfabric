package lifecycle

import "github.com/runfabric/runfabric/internal/app"

func resolveAppService(opts *GlobalOptions) app.AppService {
	if opts != nil && opts.AppService != nil {
		return opts.AppService
	}
	return app.NewAppService()
}

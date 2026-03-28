package lifecycle

import "github.com/runfabric/runfabric/internal/cli/common"

func resolveAppService(opts *common.GlobalOptions) common.AppService {
	if opts != nil && opts.AppService != nil {
		return opts.AppService
	}
	return common.NewAppService()
}

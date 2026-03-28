package daemon

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/runfabric/runfabric/internal/cli/daemoncmd"
	"github.com/runfabric/runfabric/platform/extensions/registry/loader/runtime"
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	opts := &common.GlobalOptions{AppService: common.NewAppService()}

	cmd := common.NewBootstrappedRootCmd(common.RootSpec{
		Use:     "runfabricd",
		Short:   "RunFabric Daemon CLI",
		Long:    "RunFabric daemon-focused CLI for long-running API server operations.",
		Version: runtime.Version,
	}, opts)

	daemonCmd := daemoncmd.NewDaemonCmd(opts, "runfabricd")
	cmd.AddCommand(daemonCmd.Commands()...)
	cmd.Flags().AddFlagSet(daemonCmd.Flags())
	cmd.RunE = daemonCmd.RunE

	return cmd
}

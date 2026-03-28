package worker

import (
	"fmt"

	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/runfabric/runfabric/platform/extensions/registry/loader/runtime"
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	opts := &common.GlobalOptions{AppService: common.NewAppService()}

	cmd := common.NewBootstrappedRootCmd(common.RootSpec{
		Use:     "runfabricw",
		Short:   "RunFabric Worker CLI",
		Long:    "RunFabric workload-plane CLI for workflow runtime operations.",
		Version: runtime.Version,
	}, opts)

	cmd.AddCommand(common.NewWorkflowCmd(opts))
	cmd.AddCommand(
		newGuardCommand("doctor"),
		newGuardCommand("plan"),
		newGuardCommand("build"),
		newGuardCommand("package"),
		newGuardCommand("deploy"),
		newGuardCommand("deploy-function"),
		newGuardCommand("remove"),
		newGuardCommand("recover"),
		newGuardCommand("recover-dry-run"),
		newGuardCommand("invoke"),
		newGuardCommand("init"),
		newGuardCommand("generate"),
		newGuardCommand("list"),
		newGuardCommand("inspect"),
		newGuardCommand("compose"),
		newGuardCommand("test"),
		newGuardCommand("config-api"),
		newGuardCommand("extensions"),
		newGuardCommand("state"),
		newGuardCommand("migrate"),
		newGuardCommand("auth"),
		newGuardCommand("daemon"),
		newGuardCommand("dashboard"),
		newGuardCommand("debug"),
		newGuardCommand("docs"),
		newGuardCommand("releases"),
		newGuardCommand("router"),
	)

	return cmd
}

func newGuardCommand(use string, aliases ...string) *cobra.Command {
	cmd := &cobra.Command{
		Use:                use,
		Aliases:            aliases,
		Hidden:             true,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			invoked := cmd.CalledAs()
			if invoked == "" {
				invoked = cmd.Name()
			}
			return fmt.Errorf("command %q is not available in runfabricw; runfabricw only supports workload runtime commands under \"workflow\". Use runfabric for control-plane commands", invoked)
		},
	}
	cmd.SilenceUsage = true
	return cmd
}

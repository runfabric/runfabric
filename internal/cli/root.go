package cli

import (
	"github.com/runfabric/runfabric/internal/cli/admin"
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/runfabric/runfabric/internal/cli/configuration"
	"github.com/runfabric/runfabric/internal/cli/extensions"
	"github.com/runfabric/runfabric/internal/cli/infrastructure"
	"github.com/runfabric/runfabric/internal/cli/invocation"
	"github.com/runfabric/runfabric/internal/cli/lifecycle"
	"github.com/runfabric/runfabric/internal/cli/project"
	"github.com/runfabric/runfabric/internal/cli/router"
	"github.com/runfabric/runfabric/platform/extensions/registry/loader/runtime"
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	opts := &common.GlobalOptions{AppService: common.NewAppService()}

	cmd := common.NewBootstrappedRootCmd(common.RootSpec{
		Use:     "runfabric",
		Short:   "RunFabric CLI",
		Long:    "RunFabric is a multi-provider serverless framework with a unified config and CLI workflow for services, functions, resources, and workflows.",
		Version: runtime.Version,
	}, opts)

	registerCLICommands(cmd, opts)

	return cmd
}

func registerCLICommands(cmd *cobra.Command, opts *common.GlobalOptions) {
	var allCommands []*cobra.Command
	allCommands = append(allCommands, lifecycle.RegisterCommands(opts)...)
	allCommands = append(allCommands, invocation.RegisterCommands(opts)...)
	allCommands = append(allCommands, project.RegisterCommands(opts)...)
	allCommands = append(allCommands, configuration.RegisterCommands(opts)...)
	allCommands = append(allCommands, extensions.RegisterCommands(opts)...)
	allCommands = append(allCommands, infrastructure.RegisterCommands(opts)...)
	allCommands = append(allCommands, admin.RegisterCommands(opts)...)
	allCommands = append(allCommands, router.RegisterCommands(opts)...)
	allCommands = append(allCommands, common.NewWorkflowCmd(opts))
	cmd.AddCommand(allCommands...)
}

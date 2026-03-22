package app

import (
	"time"

	"github.com/runfabric/runfabric/platform/core/model/config"
	statecore "github.com/runfabric/runfabric/platform/core/state/core"
	coreapp "github.com/runfabric/runfabric/platform/core/workflow/app"
	"github.com/runfabric/runfabric/platform/core/workflow/recovery"
	"github.com/runfabric/runfabric/platform/deploy/source"
)

type AppContext = coreapp.AppContext
type BuildOptions = coreapp.BuildOptions
type BuildResult = coreapp.BuildResult
type DashboardData = coreapp.DashboardData
type WorkflowRunResult = coreapp.WorkflowRunResult

// AppService is the contract boundary between CLI and workflow app operations.
type AppService interface {
	Plan(configPath, stage, providerOverride string) (any, error)
	Deploy(configPath, stage, functionName string, rollbackOnFailure, noRollbackOnFailure bool, extraEnv map[string]string, providerOverride string) (any, error)
	Remove(configPath, stage, providerOverride string) (any, error)
}

type defaultAppService struct{}

func NewAppService() AppService {
	return defaultAppService{}
}

func (defaultAppService) Plan(configPath, stage, providerOverride string) (any, error) {
	return Plan(configPath, stage, providerOverride)
}

func (defaultAppService) Deploy(configPath, stage, functionName string, rollbackOnFailure, noRollbackOnFailure bool, extraEnv map[string]string, providerOverride string) (any, error) {
	return Deploy(configPath, stage, functionName, rollbackOnFailure, noRollbackOnFailure, extraEnv, providerOverride)
}

func (defaultAppService) Remove(configPath, stage, providerOverride string) (any, error) {
	return Remove(configPath, stage, providerOverride)
}

func BackendDoctor(configPath, stage string) (any, error) {
	return coreapp.BackendDoctor(configPath, stage)
}
func Plan(configPath, stage, providerOverride string) (any, error) {
	return coreapp.Plan(configPath, stage, providerOverride)
}
func Build(configPath string, opts BuildOptions) (*BuildResult, error) {
	return coreapp.Build(configPath, opts)
}
func Deploy(configPath, stage, functionName string, rollbackOnFailure, noRollbackOnFailure bool, extraEnv map[string]string, providerOverride string) (any, error) {
	return coreapp.Deploy(configPath, stage, functionName, rollbackOnFailure, noRollbackOnFailure, extraEnv, providerOverride)
}
func Remove(configPath, stage, providerOverride string) (any, error) {
	return coreapp.Remove(configPath, stage, providerOverride)
}
func Invoke(configPath, stage, function, providerOverride string, payload []byte) (any, error) {
	return coreapp.Invoke(configPath, stage, function, providerOverride, payload)
}
func Logs(configPath, stage, function, providerOverride, service string) (any, error) {
	return coreapp.Logs(configPath, stage, function, providerOverride, service)
}
func Traces(configPath, stage, providerOverride string, all bool, service string) (any, error) {
	return coreapp.Traces(configPath, stage, providerOverride, all, service)
}
func Metrics(configPath, stage, providerOverride string, all bool, service string) (any, error) {
	return coreapp.Metrics(configPath, stage, providerOverride, all, service)
}
func Releases(configPath string) (any, error)       { return coreapp.Releases(configPath) }
func List(configPath, stage string) (any, error)    { return coreapp.List(configPath, stage) }
func Inspect(configPath, stage string) (any, error) { return coreapp.Inspect(configPath, stage) }
func Recover(configPath, stage string, mode recovery.Mode) (any, error) {
	return coreapp.Recover(configPath, stage, mode)
}
func RecoverDryRun(configPath, stage string) (any, error) {
	return coreapp.RecoverDryRun(configPath, stage)
}
func Unlock(configPath, stage string, force bool) (any, error) {
	return coreapp.Unlock(configPath, stage, force)
}
func LockSteal(configPath, stage string) (any, error) { return coreapp.LockSteal(configPath, stage) }
func StateList(configPath, stage string) (any, error) { return coreapp.StateList(configPath, stage) }
func StatePull(configPath, stage string) (any, error) { return coreapp.StatePull(configPath, stage) }
func StateBackup(configPath, stage, outPath string) (any, error) {
	return coreapp.StateBackup(configPath, stage, outPath)
}
func StateRestore(configPath, stage, filePath string) (any, error) {
	return coreapp.StateRestore(configPath, stage, filePath)
}
func StateForceUnlock(configPath, stage string) (any, error) {
	return coreapp.StateForceUnlock(configPath, stage)
}
func StateMigrate(configPath, stage, fromKind, toKind string) (any, error) {
	return coreapp.StateMigrate(configPath, stage, fromKind, toKind)
}
func StateReconcile(configPath, stage string) (any, error) {
	return coreapp.StateReconcile(configPath, stage)
}
func BackendMigrate(configPath, stage, target string) (any, error) {
	return coreapp.BackendMigrate(configPath, stage, target)
}
func Test(configPath string) (any, error) { return coreapp.Test(configPath) }
func Debug(configPath, stage, host, port string) (any, error) {
	return coreapp.Debug(configPath, stage, host, port)
}
func Dashboard(configPath, stage string) (*DashboardData, error) {
	return coreapp.Dashboard(configPath, stage)
}
func Bootstrap(configPath, stage, providerOverride string) (*AppContext, error) {
	return coreapp.Bootstrap(configPath, stage, providerOverride)
}
func WorkflowRun(configPath, stage, providerOverride, workflowName, runID string, runInput map[string]any) (*WorkflowRunResult, error) {
	return coreapp.WorkflowRun(configPath, stage, providerOverride, workflowName, runID, runInput)
}
func WorkflowStatus(configPath, stage, runID string) (*statecore.WorkflowRun, error) {
	return coreapp.WorkflowStatus(configPath, stage, runID)
}
func WorkflowCancel(configPath, stage, runID string) (*statecore.WorkflowRun, error) {
	return coreapp.WorkflowCancel(configPath, stage, runID)
}
func WorkflowReplay(configPath, stage, providerOverride, runID, stepID string) (*statecore.WorkflowRun, error) {
	return coreapp.WorkflowReplay(configPath, stage, providerOverride, runID, stepID)
}
func CallLocal(configPath, stage, host, port string, serve bool) (any, error) {
	return coreapp.CallLocal(configPath, stage, host, port, serve)
}
func CallLocalServe(configPath, stage, host, port string) (shutdownChan <-chan struct{}, restart func(), err error) {
	return coreapp.CallLocalServe(configPath, stage, host, port)
}
func WatchProjectDir(configPath string, pollInterval time.Duration, done <-chan struct{}) <-chan struct{} {
	return coreapp.WatchProjectDir(configPath, pollInterval, done)
}
func PrepareDevStreamTunnel(configPath, stage, tunnelURL string) (restore func(), err error) {
	return coreapp.PrepareDevStreamTunnel(configPath, stage, tunnelURL)
}
func FabricDeploy(configPath, stage string, rollbackOnFailure, noRollbackOnFailure bool) (*statecore.FabricState, error) {
	return coreapp.FabricDeploy(configPath, stage, rollbackOnFailure, noRollbackOnFailure)
}
func FabricHealth(configPath, stage string) (*statecore.FabricState, error) {
	return coreapp.FabricHealth(configPath, stage)
}
func FabricTargets(cfg *config.Config) []string { return coreapp.FabricTargets(cfg) }
func ComposeDeploy(composePath, stage string, rollbackOnFailure, noRollbackOnFailure bool) (any, error) {
	return coreapp.ComposeDeploy(composePath, stage, rollbackOnFailure, noRollbackOnFailure)
}
func ComposeRemove(composePath, stage string) (any, error) {
	return coreapp.ComposeRemove(composePath, stage)
}

func FetchDeploySource(sourceURL string) (extractRoot, resolvedConfig string, cleanup func(), err error) {
	return source.FetchAndExtract(sourceURL)
}

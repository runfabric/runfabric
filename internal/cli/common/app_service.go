package common

import "github.com/runfabric/runfabric/platform/workflow/app"

// AppService is the dashboard/daemon action contract used by CLI root commands.
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
	return app.Plan(configPath, stage, providerOverride)
}

func (defaultAppService) Deploy(configPath, stage, functionName string, rollbackOnFailure, noRollbackOnFailure bool, extraEnv map[string]string, providerOverride string) (any, error) {
	return app.Deploy(configPath, stage, functionName, rollbackOnFailure, noRollbackOnFailure, extraEnv, providerOverride)
}

func (defaultAppService) Remove(configPath, stage, providerOverride string) (any, error) {
	return app.Remove(configPath, stage, providerOverride)
}

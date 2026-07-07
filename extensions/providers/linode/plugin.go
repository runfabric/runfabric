package main

import (
	"context"
	"net/http"
	"os"
	"time"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

const (
	defaultLinodeAPIBaseURL = "https://api.linode.com/v4"
	defaultTokenEnv         = "LINODE_TOKEN"
	deployCommandEnv        = "LINODE_DEPLOY_CMD"
	removeCommandEnv        = "LINODE_REMOVE_CMD"
	invokeCommandEnv        = "LINODE_INVOKE_CMD"
	logsCommandEnv          = "LINODE_LOGS_CMD"
)

type commandRunner func(ctx context.Context, cwd, command string, env []string) ([]byte, error)

type plugin struct {
	provider      string
	apiBaseURL    string
	httpClient    *http.Client
	runCommand    commandRunner
	getenv        func(string) string
	deploymentNow func() time.Time
}

func newPlugin() *plugin {
	return &plugin{
		provider:   "linode",
		apiBaseURL: defaultLinodeAPIBaseURL,
		httpClient: &http.Client{Timeout: 20 * time.Second},
		runCommand: defaultCommandRunner,
		getenv:     os.Getenv,
		deploymentNow: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (p *plugin) Meta() sdkprovider.Meta {
	return sdkprovider.Meta{
		Name:            p.provider,
		Version:         "0.1.0",
		PluginVersion:   "1",
		SupportsRuntime: []string{"nodejs", "python"},
		SupportsTriggers: []string{
			"http",
		},
		SupportsResources: []string{},
	}
}

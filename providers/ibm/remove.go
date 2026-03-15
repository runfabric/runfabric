package ibm

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/runfabric/runfabric/internal/apiutil"
	"github.com/runfabric/runfabric/internal/config"
	"github.com/runfabric/runfabric/internal/providers"
	"github.com/runfabric/runfabric/internal/state"
)

// Remover deletes OpenWhisk actions via DELETE /api/v1/namespaces/.../actions/...
type Remover struct{}

func (Remover) Remove(ctx context.Context, cfg *config.Config, stage, root string, receipt *state.Receipt) (*providers.RemoveResult, error) {
	auth := apiutil.Env("IBM_OPENWHISK_AUTH")
	if auth == "" {
		return &providers.RemoveResult{Provider: "ibm-openwhisk", Removed: true}, nil
	}
	apihost := apiutil.Env("IBM_OPENWHISK_API_HOST")
	if apihost == "" {
		apihost = "https://us-south.functions.cloud.ibm.com"
	}
	if !strings.HasPrefix(apihost, "http") {
		apihost = "https://" + apihost
	}
	namespace := apiutil.Env("IBM_OPENWHISK_NAMESPACE")
	if namespace == "" {
		namespace = "_"
	}
	baseURL := strings.TrimSuffix(apihost, "/") + "/api/v1/namespaces/" + namespace + "/actions/"
	for fnName := range cfg.Functions {
		actionName := fmt.Sprintf("%s_%s_%s", cfg.Service, stage, fnName)
		req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, baseURL+actionName, nil)
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(auth)))
		apiutil.DefaultClient.Do(req)
	}
	return &providers.RemoveResult{Provider: "ibm-openwhisk", Removed: true}, nil
}

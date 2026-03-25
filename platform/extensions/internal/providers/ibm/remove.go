package ibm

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/runfabric/runfabric/platform/deploy/apiutil"
	"github.com/runfabric/runfabric/platform/extensions/sdkbridge"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Remover deletes OpenWhisk actions via DELETE /api/v1/namespaces/.../actions/...
type Remover struct{}

func (Remover) Remove(ctx context.Context, cfg sdkprovider.Config, stage, root string, receipt any) (*sdkprovider.RemoveResult, error) {
	coreCfg, err := sdkbridge.ToCoreConfig(cfg)
	if err != nil {
		return nil, err
	}
	_ = coreCfg
	auth := apiutil.Env("IBM_OPENWHISK_AUTH")
	if auth == "" {
		return &sdkprovider.RemoveResult{Provider: "ibm-openwhisk", Removed: true}, nil
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
	for fnName := range coreCfg.Functions {
		actionName := fmt.Sprintf("%s_%s_%s", coreCfg.Service, stage, fnName)
		req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, baseURL+actionName, nil)
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(auth)))
		apiutil.DefaultClient.Do(req)
	}
	return &sdkprovider.RemoveResult{Provider: "ibm-openwhisk", Removed: true}, nil
}

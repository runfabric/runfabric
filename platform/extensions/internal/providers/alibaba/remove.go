package alibaba

import (
	"context"
	"fmt"
	"strings"

	"github.com/runfabric/runfabric/platform/deploy/apiutil"
	"github.com/runfabric/runfabric/platform/extensions/sdkbridge"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Remover deletes FC functions and service via signed OpenAPI.
type Remover struct{}

func (Remover) Remove(ctx context.Context, cfg sdkprovider.Config, stage, root string, receipt any) (*sdkprovider.RemoveResult, error) {
	coreCfg, err := sdkbridge.ToCoreConfig(cfg)
	if err != nil {
		return nil, err
	}
	_ = coreCfg
	rv := apiutil.DecodeReceipt(receipt)
	accessKey := apiutil.Env("ALIBABA_ACCESS_KEY_ID")
	secretKey := apiutil.Env("ALIBABA_ACCESS_KEY_SECRET")
	if accessKey == "" || secretKey == "" {
		return &sdkprovider.RemoveResult{Provider: "alibaba-fc", Removed: true}, nil
	}
	accountID := apiutil.Env("ALIBABA_FC_ACCOUNT_ID")
	if accountID == "" {
		accountID = rv.Metadata["account_id"]
	}
	region := rv.Outputs["region"]
	if region == "" {
		region = coreCfg.Provider.Region
	}
	if region == "" {
		region = "cn-hangzhou"
	}
	serviceName := rv.Outputs["service_name"]
	if serviceName == "" {
		serviceName = coreCfg.Service + "-" + stage
	}
	client := newFCClient(accountID, region, accessKey, secretKey)
	// Delete each function we created (from metadata)
	for fnName := range coreCfg.Functions {
		funcName := rv.Metadata["function_"+fnName]
		if funcName == "" {
			funcName = fmt.Sprintf("%s-%s-%s", coreCfg.Service, stage, fnName)
		}
		_, _ = client.DeleteFunction(ctx, serviceName, funcName)
	}
	_, err = client.DeleteService(ctx, serviceName)
	if err != nil && !strings.Contains(err.Error(), "NotFound") && !strings.Contains(err.Error(), "not found") {
		return nil, fmt.Errorf("DeleteService: %w", err)
	}
	return &sdkprovider.RemoveResult{Provider: "alibaba-fc", Removed: true}, nil
}

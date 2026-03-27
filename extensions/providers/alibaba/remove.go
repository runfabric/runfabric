package alibaba

import (
	"context"
	"fmt"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Remover deletes FC functions and service via signed OpenAPI.
type Remover struct{}

func (Remover) Remove(ctx context.Context, cfg sdkprovider.Config, stage, root string, receipt any) (*sdkprovider.RemoveResult, error) {
	service := sdkprovider.Service(cfg)
	functions := sdkprovider.Functions(cfg)
	rv := sdkprovider.DecodeReceipt(receipt)
	accessKey := sdkprovider.Env("ALIBABA_ACCESS_KEY_ID")
	secretKey := sdkprovider.Env("ALIBABA_ACCESS_KEY_SECRET")
	if accessKey == "" || secretKey == "" {
		return &sdkprovider.RemoveResult{Provider: "alibaba-fc", Removed: true}, nil
	}
	accountID := sdkprovider.Env("ALIBABA_FC_ACCOUNT_ID")
	if accountID == "" {
		accountID = rv.Metadata["account_id"]
	}
	region := rv.Outputs["region"]
	if region == "" {
		region = sdkprovider.ProviderRegion(cfg)
	}
	if region == "" {
		region = "cn-hangzhou"
	}
	serviceName := rv.Outputs["service_name"]
	if serviceName == "" {
		serviceName = service + "-" + stage
	}
	client := newFCClient(accountID, region, accessKey, secretKey)
	// Delete each function we created (from metadata)
	for fnName := range functions {
		funcName := rv.Metadata["function_"+fnName]
		if funcName == "" {
			funcName = fmt.Sprintf("%s-%s-%s", service, stage, fnName)
		}
		_, _ = client.DeleteFunction(ctx, serviceName, funcName)
	}
	_, err := client.DeleteService(ctx, serviceName)
	if err != nil && !strings.Contains(err.Error(), "NotFound") && !strings.Contains(err.Error(), "not found") {
		return nil, fmt.Errorf("DeleteService: %w", err)
	}
	return &sdkprovider.RemoveResult{Provider: "alibaba-fc", Removed: true}, nil
}

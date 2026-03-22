package alibaba

import (
	"context"
	"fmt"
	"strings"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
	"github.com/runfabric/runfabric/platform/deploy/apiutil"
)

// Remover deletes FC functions and service via signed OpenAPI.
type Remover struct{}

func (Remover) Remove(ctx context.Context, cfg *config.Config, stage, root string, receipt *state.Receipt) (*providers.RemoveResult, error) {
	accessKey := apiutil.Env("ALIBABA_ACCESS_KEY_ID")
	secretKey := apiutil.Env("ALIBABA_ACCESS_KEY_SECRET")
	if accessKey == "" || secretKey == "" {
		return &providers.RemoveResult{Provider: "alibaba-fc", Removed: true}, nil
	}
	accountID := apiutil.Env("ALIBABA_FC_ACCOUNT_ID")
	if accountID == "" {
		accountID = receipt.Metadata["account_id"]
	}
	region := receipt.Outputs["region"]
	if region == "" {
		region = cfg.Provider.Region
	}
	if region == "" {
		region = "cn-hangzhou"
	}
	serviceName := receipt.Outputs["service_name"]
	if serviceName == "" {
		serviceName = cfg.Service + "-" + stage
	}
	client := newFCClient(accountID, region, accessKey, secretKey)
	// Delete each function we created (from metadata)
	for fnName := range cfg.Functions {
		funcName := receipt.Metadata["function_"+fnName]
		if funcName == "" {
			funcName = fmt.Sprintf("%s-%s-%s", cfg.Service, stage, fnName)
		}
		_, _ = client.DeleteFunction(ctx, serviceName, funcName)
	}
	_, err := client.DeleteService(ctx, serviceName)
	if err != nil && !strings.Contains(err.Error(), "NotFound") && !strings.Contains(err.Error(), "not found") {
		return nil, fmt.Errorf("DeleteService: %w", err)
	}
	return &providers.RemoveResult{Provider: "alibaba-fc", Removed: true}, nil
}

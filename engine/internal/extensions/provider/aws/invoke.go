package aws

import (
	"context"
	"time"

	"github.com/runfabric/runfabric/engine/internal/extensions/providers"

	lambdav2 "github.com/aws/aws-sdk-go-v2/service/lambda"
)

func (p *Provider) Invoke(cfg *providers.Config, stage, function string, payload []byte) (*providers.InvokeResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	clients, err := loadClients(ctx, cfg.Provider.Region)
	if err != nil {
		return nil, err
	}

	name := functionName(cfg, stage, function)

	out, err := clients.Lambda.Invoke(ctx, &lambdav2.InvokeInput{
		FunctionName: &name,
		Payload:      payload,
	})
	if err != nil {
		return nil, err
	}

	return &providers.InvokeResult{
		Provider: p.Name(),
		Function: function,
		Output:   string(out.Payload),
	}, nil
}

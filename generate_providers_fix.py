import os

providers = [
    "gcp", "azure", "vercel", "netlify", 
    "alibaba", "digitalocean", "fly", "ibm"
]

adapter_template = """package {provider}

import (
	"context"

	"github.com/runfabric/runfabric/internal/config"
	"github.com/runfabric/runfabric/internal/transactions"
	"github.com/runfabric/runfabric/providers"
)

type Adapter struct {{}}

func NewAdapter() providers.Adapter {{
	return &Adapter{{}}
}}

func (a *Adapter) Name() string {{
	return "{provider}"
}}

func (a *Adapter) BuildPlan(ctx context.Context, cfg *config.Config, stage string, root string, journal *transactions.Journal) (providers.Plan, error) {{
	return &plan{{}}, nil
}}

type plan struct {{}}

func (p *plan) Execute(ctx context.Context) (*providers.DeployResult, error) {{
	return &providers.DeployResult{{}}, nil
}}

func (p *plan) Rollback(ctx context.Context) error {{
	return nil
}}
"""

for p in providers:
    d = os.path.join("providers", p)
    os.makedirs(d, exist_ok=True)
    with open(os.path.join(d, "adapter.go"), "w") as f:
        f.write(adapter_template.format(provider=p))

print("Created provider stubs with correct contract.")

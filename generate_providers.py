import os

providers = [
    "gcp", "azure", "vercel", "netlify", 
    "alibaba", "digitalocean", "fly", "ibm"
]

adapter_template = """package {provider}

import (
	"context"
	"runfabric/internal/config"
	"runfabric/providers"
)

type Adapter struct {{}}

func NewAdapter() providers.ProviderAdapter {{
	return &Adapter{{}}
}}

func (a *Adapter) Info() providers.ProviderInfo {{
	return providers.ProviderInfo{{
		ID:   "{provider}",
		Name: "{provider}",
	}}
}}

func (a *Adapter) Doctor(ctx context.Context, cfg *config.ServiceConfig) (*providers.DoctorResult, error) {{
	return &providers.DoctorResult{{Ok: true}}, nil
}}

func (a *Adapter) Invoke(ctx context.Context, fnName string, payload []byte) ([]byte, error) {{
	return nil, nil
}}

func (a *Adapter) Logs(ctx context.Context, fnName string) error {{
	return nil
}}
"""

for p in providers:
    d = os.path.join("providers", p)
    os.makedirs(d, exist_ok=True)
    with open(os.path.join(d, "adapter.go"), "w") as f:
        f.write(adapter_template.format(provider=p))

print("Created provider stubs.")

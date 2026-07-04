package alibaba

import sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

// CredentialVars declares the credential env vars this provider reads
// (region falls back to provider.region in config, then cn-hangzhou).
var CredentialVars = []sdkprovider.CredentialVar{
	{EnvKey: "ALIBABA_ACCESS_KEY_ID", Header: "X-Provider-Alibaba-Access-Key-Id", Required: true},
	{EnvKey: "ALIBABA_ACCESS_KEY_SECRET", Header: "X-Provider-Alibaba-Access-Key-Secret", Required: true},
	{EnvKey: "ALIBABA_FC_ACCOUNT_ID", Header: "X-Provider-Alibaba-Account-Id", Required: true},
	{EnvKey: "ALIBABA_FC_REGION", Header: "X-Provider-Alibaba-Region", Placeholder: "cn-hangzhou"},
}

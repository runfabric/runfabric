package ibm

import sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

// CredentialVars declares the credential env vars this provider reads
// (API host defaults to us-south, namespace to "_", when unset).
var CredentialVars = []sdkprovider.CredentialVar{
	{EnvKey: "IBM_OPENWHISK_AUTH", Header: "X-Provider-Ibm-Auth", Required: true},
	{EnvKey: "IBM_OPENWHISK_API_HOST", Header: "X-Provider-Ibm-Api-Host", Placeholder: "https://us-south.functions.cloud.ibm.com"},
	{EnvKey: "IBM_OPENWHISK_NAMESPACE", Header: "X-Provider-Ibm-Namespace"},
}

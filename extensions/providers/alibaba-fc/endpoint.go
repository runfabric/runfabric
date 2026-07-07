package alibaba

import (
	"fmt"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// alibabaAPIBase returns the Alibaba Function Compute host. When
// ALIBABA_ENDPOINT_URL is set it overrides the real per-account/region host (for
// a mock, proxy, or emulator), resolved at call time so production is untouched
// when unset; otherwise it falls back to the real FC host built from fcHostFmt.
func alibabaAPIBase(accountID, region string) string {
	if b := strings.TrimSpace(sdkprovider.Env("ALIBABA_ENDPOINT_URL")); b != "" {
		return strings.TrimRight(b, "/")
	}
	return strings.TrimRight(fmt.Sprintf(fcHostFmt, accountID, region), "/")
}

// alibabaFunctionURL returns the HTTP invoke URL for a deployed function. Real FC
// serves it at <host>/<version>/proxy/<service>/<function>/; because host is
// resolved through alibabaAPIBase, an ALIBABA_ENDPOINT_URL override fronts the
// same path so a mock or emulator can serve invocations.
func alibabaFunctionURL(host, serviceName, functionName string) string {
	return fmt.Sprintf("%s/%s/proxy/%s/%s/", strings.TrimRight(host, "/"), fcAPIVersion, serviceName, functionName)
}

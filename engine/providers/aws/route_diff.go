package aws

import (
	"strings"

	"github.com/runfabric/runfabric/engine/internal/planner"
)

func routeNeedsUpdate(
	desired planner.DesiredRoute,
	actual planner.ActualRoute,
	integrations map[string]planner.ActualIntegration,
	expectedLambdaARN string,
) bool {
	if actual.RouteKey != desired.RouteKey {
		return true
	}

	integ, ok := integrations[actual.IntegrationID]
	if !ok {
		return true
	}

	return !strings.EqualFold(integ.IntegrationURI, expectedLambdaARN)
}

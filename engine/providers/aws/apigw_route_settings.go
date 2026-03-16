package aws

import "github.com/runfabric/runfabric/engine/internal/config"

func routeSettingsChanged(rs *config.RouteSettings) bool {
	return rs != nil && (rs.ThrottlingBurstLimit > 0 || rs.ThrottlingRateLimit > 0)
}

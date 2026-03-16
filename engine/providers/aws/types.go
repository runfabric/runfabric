package aws

func str(v string) *string {
	return &v
}

type HTTPAPI struct {
	APIID       string
	APIEndpoint string
	StageName   string
}

type HTTPRouteBinding struct {
	RouteKey      string
	RouteID       string
	IntegrationID string
	URL           string
}

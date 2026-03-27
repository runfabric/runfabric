package engine

type DesiredFunction struct {
	Name            string
	Runtime         string
	Handler         string
	Memory          int
	Timeout         int
	CodeSHA256      string
	ConfigSignature string
}

type DesiredHTTPAPI struct {
	Name string
}

type DesiredRoute struct {
	RouteKey     string
	FunctionName string
	Path         string
	Method       string
}

type DesiredState struct {
	Functions []DesiredFunction
	HTTPAPI   *DesiredHTTPAPI
	Routes    []DesiredRoute
}

type ActualFunction struct {
	Name               string
	Runtime            string
	Handler            string
	ResourceIdentifier string
	Memory             int32
	Timeout            int32
	CodeSHA256         string
}

type ActualHTTPAPI struct {
	ID       string
	Name     string
	Endpoint string
}

type ActualRoute struct {
	ID            string
	RouteKey      string
	Target        string
	IntegrationID string
}

type ActualIntegration struct {
	ID             string
	IntegrationURI string
}

type ActualState struct {
	Functions    []ActualFunction
	HTTPAPI      *ActualHTTPAPI
	Routes       []ActualRoute
	Integrations []ActualIntegration
}

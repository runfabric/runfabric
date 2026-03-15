package planner

type ActionType string

const (
	ActionCreate ActionType = "create"
	ActionUpdate ActionType = "update"
	ActionDelete ActionType = "delete"
	ActionBuild  ActionType = "build"
	ActionNoop   ActionType = "noop"
)

type ResourceType string

const (
	ResourceLambda       ResourceType = "lambda"
	ResourceHTTPAPI      ResourceType = "http_api"
	ResourceHTTPRoute    ResourceType = "http_route"
	ResourceIntegration  ResourceType = "integration"
	ResourceSchedule     ResourceType = "schedule"
	ResourceQueue        ResourceType = "queue"
	ResourceStorage      ResourceType = "storage"
	ResourceEventBridge  ResourceType = "eventbridge"
	ResourcePubSub       ResourceType = "pubsub"
	ResourceKafka        ResourceType = "kafka"
	ResourceRabbitMQ     ResourceType = "rabbitmq"
)

type PlanAction struct {
	ID          string               `json:"id"`
	Type        ActionType           `json:"type"`
	Resource    ResourceType         `json:"resource"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	DependsOn   []string             `json:"dependsOn,omitempty"`
	Metadata    map[string]string    `json:"metadata,omitempty"`
	Changes     map[string][2]string `json:"changes,omitempty"`
}

type Plan struct {
	Provider string       `json:"provider"`
	Service  string       `json:"service"`
	Stage    string       `json:"stage"`
	Actions  []PlanAction `json:"actions"`
}

package exec

// Provider-agnostic checkpoint names used by the universal deploy engine.
const (
	CheckpointDeployFunctions = "deploy_functions"
	CheckpointSaveReceipt     = "save_receipt"
)

// AWS-specific checkpoint names retained for the Lambda provider.
const (
	CheckpointPackageArtifacts = "package_artifacts"
	CheckpointDiscoverState    = "discover_state"
	CheckpointEnsureHTTPAPI    = "ensure_http_api"
	CheckpointEnsureLambdas    = "ensure_lambdas"
	CheckpointEnsureRoutes     = "ensure_routes"
	CheckpointEnsureTriggers   = "ensure_triggers"
	CheckpointReconcileStale   = "reconcile_stale"
)

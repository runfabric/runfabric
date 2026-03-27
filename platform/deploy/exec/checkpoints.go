package exec

const (
	CheckpointPackageArtifacts = "package_artifacts"
	CheckpointDiscoverState    = "discover_state"
	CheckpointEnsureHTTPAPI    = "ensure_http_api"
	CheckpointEnsureLambdas    = "ensure_lambdas"
	CheckpointEnsureRoutes     = "ensure_routes"
	CheckpointEnsureTriggers   = "ensure_triggers" // cron, queue, storage, eventbridge
	CheckpointReconcileStale   = "reconcile_stale"
	CheckpointSaveReceipt      = "save_receipt"
)

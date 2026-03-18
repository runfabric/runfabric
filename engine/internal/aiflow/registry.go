package aiflow

// MVP node types for Phase 14.1. Extend as needed.
const (
	NodeTypeTrigger = "trigger"
	NodeTypeAI      = "ai"
	NodeTypeData    = "data"
	NodeTypeLogic   = "logic"
	NodeTypeSystem  = "system"
	NodeTypeHuman   = "human"
)

// NodeTypes is the central registry of supported node types (type -> allowed). Used for validation.
var NodeTypes = map[string]bool{
	NodeTypeTrigger: true,
	NodeTypeAI:      true,
	NodeTypeData:    true,
	NodeTypeLogic:   true,
	NodeTypeSystem:  true,
	NodeTypeHuman:   true,
}

// ValidNodeType returns true if typ is a supported node type.
func ValidNodeType(typ string) bool {
	return NodeTypes[typ]
}

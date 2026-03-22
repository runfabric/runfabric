package secrets

// LookupFunc returns a value for key, or false when key is missing.
type LookupFunc func(key string) (string, bool)

// StringResolver resolves secret references in string values.
type StringResolver interface {
	ResolveString(input string) (string, error)
}

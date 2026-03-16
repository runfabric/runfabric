package dynamodb

import "context"

func str(v string) *string {
	return &v
}

func contextBackground() context.Context {
	return context.Background()
}

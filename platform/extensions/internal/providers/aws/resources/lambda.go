// Lambda resource types (runtime, handler, memory, timeout).
package resources

type LambdaFunction struct {
	Name       string
	Runtime    string
	Handler    string
	MemorySize int
	Timeout    int
}

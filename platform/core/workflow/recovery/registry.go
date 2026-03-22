package recovery

import "fmt"

type ProviderHandlerFactory func(journal any) Handler

type Registry struct {
	factories map[string]ProviderHandlerFactory
}

func NewRegistry() *Registry {
	return &Registry{
		factories: map[string]ProviderHandlerFactory{},
	}
}

func (r *Registry) Register(provider string, factory ProviderHandlerFactory) {
	r.factories[provider] = factory
}

func (r *Registry) Get(provider string, journal any) (Handler, error) {
	f, ok := r.factories[provider]
	if !ok {
		return nil, fmt.Errorf("no recovery handler registered for provider %q", provider)
	}
	return f(journal), nil
}

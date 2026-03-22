package provider

import (
	"fmt"

	appErrs "github.com/runfabric/runfabric/platform/core/model/errors"
)

// ErrProviderNotFound returns an error when a provider name is not registered.
func ErrProviderNotFound(name string) error {
	return appErrs.Wrap(appErrs.CodeProviderNotFound, fmt.Sprintf("provider %q not registered", name), nil)
}

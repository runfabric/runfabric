package providers

import (
	"fmt"

	appErrs "github.com/runfabric/runfabric/engine/internal/errors"
)

func ErrProviderNotFound(name string) error {
	return appErrs.Wrap(appErrs.CodeProviderNotFound, fmt.Sprintf("provider %q not registered", name), nil)
}

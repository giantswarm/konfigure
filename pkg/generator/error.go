package generator

import (
	"errors"
	"os"

	"github.com/giantswarm/microerror"
)

var invalidConfigError = &microerror.Error{
	Kind: "invalidConfigError",
}

// IsInvalidConfig asserts invalidConfigError.
func IsInvalidConfig(err error) bool {
	return microerror.Cause(err) == invalidConfigError
}

var notFoundError = &microerror.Error{
	Kind: "notFoundError",
}

// IsNotFound asserts notFoundError.
func IsNotFound(err error) bool {
	if errors.Is(err, os.ErrNotExist) {
		return true
	}

	return microerror.Cause(err) == notFoundError
}

var failedToDecryptError = &microerror.Error{
	Kind: "failedToDecryptError",
}

// IsFailedToDecryptError asserts failedToDecryptError.
func IsFailedToDecryptError(err error) bool {
	return microerror.Cause(err) == failedToDecryptError
}

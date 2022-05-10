package sopsenv

import (
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
	return microerror.Cause(err) == notFoundError
}

var pgpImportError = &microerror.Error{
	Kind: "pgpImportError",
}

// isPGPImportError asserts pgpImportError.
func IsPGPImportError(err error) bool {
	return microerror.Cause(err) == pgpImportError
}

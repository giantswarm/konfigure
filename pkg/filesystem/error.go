package filesystem

import (
	"github.com/giantswarm/microerror"
)

var invalidPathError = &microerror.Error{
	Kind: "invalidPathError",
}

// IsInvalidPath asserts invalidPathError.
func IsInvalidPath(err error) bool {
	return microerror.Cause(err) == invalidPathError
}

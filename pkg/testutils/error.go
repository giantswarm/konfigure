package testutils

import (
	"errors"
	"os"
	"reflect"
)

type NotFoundError struct {
	message string
}

func (e *NotFoundError) Error() string {
	return "NotFoundError: " + e.message
}

func (e *NotFoundError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e) || errors.Is(target, os.ErrNotExist)
}

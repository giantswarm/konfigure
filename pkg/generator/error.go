package generator

import (
	"errors"
	"os"
	"reflect"
)

type InvalidConfigError struct {
	message string
}

func (e *InvalidConfigError) Error() string {
	return "InvalidConfigError: " + e.message
}

func (e *InvalidConfigError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

type NotFoundError struct {
	message string
}

func (e *NotFoundError) Error() string {
	return "NotFoundError: " + e.message
}

func (e *NotFoundError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e) || errors.Is(target, os.ErrNotExist)
}

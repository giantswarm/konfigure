package kustomizepatch

import (
	"reflect"
)

type ExecutionFailedError struct {
	message string
}

func (e *ExecutionFailedError) Error() string {
	return "ExecutionFailedError: " + e.message
}

func (e *ExecutionFailedError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

type InvalidConfigError struct {
	message string
}

func (e *InvalidConfigError) Error() string {
	return "InvalidConfigError: " + e.message
}

func (e *InvalidConfigError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

type InvalidFlagError struct {
	message string
}

func (e *InvalidFlagError) Error() string {
	return "InvalidFlagError: " + e.message
}

func (e *InvalidFlagError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

package lint

import (
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

type InvalidFlagError struct {
	message string
}

func (e *InvalidFlagError) Error() string {
	return "InvalidFlagError: " + e.message
}

func (e *InvalidFlagError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

type LinterFoundIssuesError struct {
	message string
}

func (e *LinterFoundIssuesError) Error() string {
	return "LinterFoundIssuesError: " + e.message
}

func (e *LinterFoundIssuesError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

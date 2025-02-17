package sopsenv

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

type NotFoundError struct {
	message string
}

func (e *NotFoundError) Error() string {
	return "NotFoundError: " + e.message
}

func (e *NotFoundError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

type PgpImportError struct {
	message string
}

func (e *PgpImportError) Error() string {
	return "PgpImportError: " + e.message
}

func (e *PgpImportError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

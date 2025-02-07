package lint

import "reflect"

type ExecutionFailedError struct {
	message string
}

func (e *ExecutionFailedError) Error() string {
	return "ExecutionFailedError: " + e.message
}

func (e *ExecutionFailedError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

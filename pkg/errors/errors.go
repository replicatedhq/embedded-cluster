package errors

import "errors"

type FatalError struct {
	Err error
}

func (e *FatalError) Error() string {
	return e.Err.Error()
}

func (e *FatalError) Unwrap() error {
	return e.Err
}

func NewFatalError(err error) error {
	return &FatalError{Err: err}
}

func IsFatalError(err error) bool {
	var target *FatalError
	return errors.As(err, &target)
}

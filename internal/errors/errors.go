package errors

import "fmt"

type Code string

const (
	CodeConfigLoad       Code = "CONFIG_LOAD_ERROR"
	CodeConfigResolve    Code = "CONFIG_RESOLVE_ERROR"
	CodeConfigValidation Code = "CONFIG_VALIDATION_ERROR"
	CodeProviderNotFound Code = "PROVIDER_NOT_FOUND"
	CodeDeployFailed     Code = "DEPLOY_FAILED"
	CodeRemoveFailed     Code = "REMOVE_FAILED"
	CodeInvokeFailed     Code = "INVOKE_FAILED"
	CodeLogsFailed       Code = "LOGS_FAILED"
)

type AppError struct {
	Code    Code
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func Wrap(code Code, message string, err error) error {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

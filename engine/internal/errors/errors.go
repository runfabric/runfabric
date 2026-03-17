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
	Hint    string // Optional next-step hint (e.g. "run runfabric doctor", "set AWS_ACCESS_KEY_ID")
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

// Wrap returns an AppError with optional hint for user-facing "next step" guidance.
func Wrap(code Code, message string, err error) error {
	return &AppError{Code: code, Message: message, Err: err}
}

// WrapWithHint returns an AppError with a hint (e.g. "run runfabric doctor", "check GCP_PROJECT_ID").
func WrapWithHint(code Code, message string, err error, hint string) error {
	return &AppError{Code: code, Message: message, Hint: hint, Err: err}
}

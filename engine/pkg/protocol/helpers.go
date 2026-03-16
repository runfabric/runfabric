package protocol

import (
	"errors"

	appErrs "github.com/runfabric/runfabric/engine/internal/errors"
)

func Success(command string, data any) *Response {
	return &Response{
		OK:      true,
		Command: command,
		Data:    data,
	}
}

func Failure(command string, err *ErrBody) *Response {
	return &Response{
		OK:      false,
		Command: command,
		Error:   err,
	}
}

func FromError(err error) *ErrBody {
	var appErr *appErrs.AppError
	if errors.As(err, &appErr) {
		return &ErrBody{
			Code:    string(appErr.Code),
			Message: appErr.Message,
		}
	}

	return &ErrBody{
		Code:    "UNKNOWN_ERROR",
		Message: err.Error(),
	}
}

package bizerr

import (
	"encoding/json"
	"errors"
	"net/http"
)

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

type ValidationError struct {
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields"`
}

func (v ValidationError) Error() string {
	data, _ := json.Marshal(v)
	return string(data)
}

type BizError interface {
	HTTPCode() int
	Message() string
	Error() string
	ValidationErrors() map[string]string
	IsValidationError() bool
}

type bizError struct {
	httpCode        int
	errorNo         int
	err             error
	validationError *ValidationError
}

func New(httpCode int, err error) BizError {
	return &bizError{
		httpCode: httpCode,
		err:      err,
	}
}

func NewValidationError(message string, fields map[string]string) BizError {
	return &bizError{
		httpCode: http.StatusBadRequest,
		err:      errors.New(message),
		validationError: &ValidationError{
			Message: message,
			Fields:  fields,
		},
	}
}

func (b bizError) HTTPCode() int {
	return b.httpCode
}

func (b bizError) Message() string {
	if b.err != nil {
		return b.err.Error()
	}
	return ""
}

func (b bizError) Error() string {
	return b.err.Error()
}

func (b bizError) ValidationErrors() map[string]string {
	if b.validationError != nil {
		return b.validationError.Fields
	}
	return nil
}

func (b bizError) IsValidationError() bool {
	return b.validationError != nil
}

func NewFieldError(field, message, code string) FieldError {
	return FieldError{
		Field:   field,
		Message: message,
		Code:    code,
	}
}

func NewSingleFieldError(field, message string) BizError {
	return NewValidationError("validation failed", map[string]string{
		field: message,
	})
}

func NewMultipleFieldErrors(fields map[string]string) BizError {
	return NewValidationError("validation failed", fields)
}

var (
	ErrBadRequest          = func(err error) BizError { return New(http.StatusBadRequest, err) }
	ErrInternalServerError = func(err error) BizError {
		return New(http.StatusInternalServerError, err)
	}
	ErrUnauthorized = func() BizError { return New(http.StatusUnauthorized, errors.New("please login you account")) }
	ErrForbidden    = func() BizError { return New(http.StatusForbidden, errors.New("http forbidden")) }
	ErrNotFound     = func() BizError { return New(http.StatusNotFound, errors.New("404 Not Found")) }
)

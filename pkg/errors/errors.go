package errors

import (
	"errors"
	"fmt"
)

// AppError 业务错误，code 对应 API 错误码
type AppError struct {
	Code    int
	Message string
	Cause   error
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.Cause }

// Is 同 Code 即视为同一错误，便于 errors.Is 在 WithMessage 派生后仍能命中
func (e *AppError) Is(target error) bool {
	var t *AppError
	if !errors.As(target, &t) {
		return false
	}
	return e.Code == t.Code
}

// WithMessage 派生新实例覆盖文案
func (e *AppError) WithMessage(msg string) *AppError {
	return &AppError{Code: e.Code, Message: msg, Cause: e.Cause}
}

// WithCause 派生新实例附原因
func (e *AppError) WithCause(cause error) *AppError {
	return &AppError{Code: e.Code, Message: e.Message, Cause: cause}
}

// 错误码常量，对外文档在 docs/api.md「Response envelope and error codes」，改码要同步
const (
	CodeOK           = 0
	CodeInvalidParam = 4001
	CodeNotFound     = 4004
	CodeConflict     = 4009
	CodeBusinessRule = 4222
	CodeInternal     = 5000
)

// 预定义错误
var (
	ErrInvalidParam = &AppError{Code: CodeInvalidParam, Message: "请求参数无效"}
	ErrNotFound     = &AppError{Code: CodeNotFound, Message: "资源不存在"}
	ErrConflict     = &AppError{Code: CodeConflict, Message: "资源已存在"}
	ErrBusinessRule = &AppError{Code: CodeBusinessRule, Message: "业务规则违反"}
	ErrInternal     = &AppError{Code: CodeInternal, Message: "服务端错误"}
)

// HTTPStatus 给定 AppError 返回应使用的 HTTP 状态码
func HTTPStatus(err error) int {
	var ae *AppError
	if !errors.As(err, &ae) {
		return 500
	}
	switch ae.Code {
	case CodeOK:
		return 200
	case CodeInvalidParam:
		return 400
	case CodeNotFound:
		return 404
	case CodeConflict:
		return 409
	case CodeBusinessRule:
		return 422
	default:
		return 500
	}
}

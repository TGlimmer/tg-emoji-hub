package response

import (
	stderrors "errors"
	"net/http"

	"github.com/gin-gonic/gin"

	apperr "github.com/TGlimmer/tg-emoji-hub/pkg/errors"
)

// Body 统一响应结构
type Body struct {
	Code    int    `json:"code"`
	Data    any    `json:"data"`
	Message string `json:"message"`
}

// OK 成功响应
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{Code: apperr.CodeOK, Data: data, Message: "ok"})
}

// Err 自定义错误响应
func Err(c *gin.Context, code int, msg string) {
	c.JSON(httpStatusForCode(code), Body{Code: code, Data: nil, Message: msg})
}

// Fail 从 error 推断 code 与 HTTP 状态
func Fail(c *gin.Context, err error) {
	var ae *apperr.AppError
	if stderrors.As(err, &ae) {
		c.JSON(apperr.HTTPStatus(ae), Body{Code: ae.Code, Data: nil, Message: ae.Message})
		return
	}
	// 兜底：未识别错误一律 5000
	c.JSON(http.StatusInternalServerError, Body{Code: apperr.CodeInternal, Data: nil, Message: "服务端错误"})
}

func httpStatusForCode(code int) int {
	switch code {
	case apperr.CodeOK:
		return http.StatusOK
	case apperr.CodeInvalidParam:
		return http.StatusBadRequest
	case apperr.CodeNotFound:
		return http.StatusNotFound
	case apperr.CodeConflict:
		return http.StatusConflict
	case apperr.CodeBusinessRule:
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

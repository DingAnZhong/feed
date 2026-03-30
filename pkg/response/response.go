package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	CodeSuccess   = 0
	CodeServerErr = 500
	CodeParamErr  = 400
	CodeAuthErr   = 401
)

type ResponseBody struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

func Response(c *gin.Context, code int, msg string, data any) {
	c.JSON(http.StatusOK, ResponseBody{
		Code: code,
		Msg:  msg,
		Data: data,
	})
}

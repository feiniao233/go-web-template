package response

import "github.com/gin-gonic/gin"

type body struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

type pageBody struct {
	body
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

func Success(c *gin.Context, status int, msg string, data any) {
	c.JSON(status, body{Code: 200, Msg: msg, Data: data})
}

func Page(c *gin.Context, msg string, data any, total int64, page, pageSize int) {
	c.JSON(200, pageBody{body: body{Code: 200, Msg: msg, Data: data}, Total: total, Page: page, PageSize: pageSize})
}

func Error(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, body{Code: status, Msg: msg, Data: nil})
}

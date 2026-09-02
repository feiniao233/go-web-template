package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"go-web-template/internal/requestid"
)

func TestRequestIDPropagation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	router := gin.New()
	router.Use(Request(logger))
	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, requestid.FromContext(c.Request.Context()))
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "device-42")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, "device-42", res.Header().Get("X-Request-ID"))
	assert.Equal(t, "device-42", res.Body.String())
}

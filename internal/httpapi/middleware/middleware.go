package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"go-web-template/internal/httpapi/response"
	"go-web-template/internal/requestid"
)

func Request(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		requestID := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if !validRequestID(requestID) {
			requestID = uuid.NewString()
		}
		c.Header("X-Request-ID", requestID)
		c.Set("request_id", requestID)
		c.Request = c.Request.WithContext(requestid.NewContext(c.Request.Context(), requestID))
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.WithFields(logrus.Fields{"request_id": requestID, "panic": recovered}).Error("panic recovered")
				if !c.Writer.Written() {
					response.Error(c, http.StatusInternalServerError, "internal server error")
				}
				c.Abort()
			}
			logger.WithFields(logrus.Fields{"request_id": requestID, "method": c.Request.Method, "path": c.Request.URL.Path, "status": c.Writer.Status(), "duration_ms": time.Since(start).Milliseconds()}).Info("request")
		}()
		c.Next()
	}
}

func validRequestID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, char := range id {
		if char < 33 || char > 126 {
			return false
		}
	}
	return true
}

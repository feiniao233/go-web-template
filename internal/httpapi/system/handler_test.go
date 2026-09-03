package system

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"go-web-template/internal/health"
)

func TestReadyChecksRegisteredDependencies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name   string
		checks []health.Check
		status int
	}{
		{name: "no dependencies", status: http.StatusOK},
		{name: "available", checks: []health.Check{{Name: "database", Ping: func(context.Context) error { return nil }}}, status: http.StatusOK},
		{name: "unavailable", checks: []health.Check{{Name: "redis", Ping: func(context.Context) error { return errors.New("down") }}}, status: http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			Register(router, test.checks)
			res := httptest.NewRecorder()
			router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/ready", nil))
			assert.Equal(t, test.status, res.Code)
		})
	}
}

func TestReadyRunsChecksConcurrently(t *testing.T) {
	gin.SetMode(gin.TestMode)
	checks := []health.Check{
		{Name: "database", Ping: func(context.Context) error { time.Sleep(120 * time.Millisecond); return nil }},
		{Name: "redis", Ping: func(context.Context) error { time.Sleep(120 * time.Millisecond); return nil }},
	}
	router := gin.New()
	Register(router, checks)
	res := httptest.NewRecorder()
	start := time.Now()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/ready", nil))
	assert.Equal(t, http.StatusOK, res.Code)
	assert.Less(t, time.Since(start), 220*time.Millisecond)
}

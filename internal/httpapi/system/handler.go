package system

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"go-web-template/internal/buildinfo"
	"go-web-template/internal/health"
	"go-web-template/internal/httpapi/response"
)

type handler struct {
	checks []health.Check
}

func Register(router *gin.Engine, checks []health.Check) {
	h := handler{checks: checks}
	router.GET("/health", h.health)
	router.GET("/ready", h.ready)
	router.GET("/version", h.version)
}

func (h handler) health(c *gin.Context) {
	response.Success(c, http.StatusOK, "ok", gin.H{"status": "ok"})
}

func (h handler) ready(c *gin.Context) {
	for _, check := range h.checks {
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
		err := check.Ping(ctx)
		cancel()
		if err != nil {
			response.Error(c, http.StatusServiceUnavailable, check.Name+" is unavailable")
			return
		}
	}
	response.Success(c, http.StatusOK, "ok", gin.H{"status": "ready"})
}

func (h handler) version(c *gin.Context) {
	response.Success(c, http.StatusOK, "ok", gin.H{"version": buildinfo.Version, "commit": buildinfo.Commit, "build_time": buildinfo.BuildTime})
}

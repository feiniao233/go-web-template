package system

import (
	"context"
	"net/http"
	"sync"
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
	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
	defer cancel()

	statuses := make(map[string]string, len(h.checks))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, check := range h.checks {
		check := check
		wg.Add(1)
		go func() {
			defer wg.Done()
			status := "ok"
			if err := check.Ping(ctx); err != nil {
				status = "unavailable"
			}
			mu.Lock()
			statuses[check.Name] = status
			mu.Unlock()
		}()
	}
	wg.Wait()

	for _, status := range statuses {
		if status != "ok" {
			response.Error(c, http.StatusServiceUnavailable, "dependency is unavailable")
			return
		}
	}
	response.Success(c, http.StatusOK, "ok", gin.H{"status": "ready", "checks": statuses})
}

func (h handler) version(c *gin.Context) {
	response.Success(c, http.StatusOK, "ok", gin.H{"version": buildinfo.Version, "commit": buildinfo.Commit, "build_time": buildinfo.BuildTime})
}

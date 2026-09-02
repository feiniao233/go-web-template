package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"go-web-template/internal/health"
	"go-web-template/internal/httpapi/middleware"
	noteapi "go-web-template/internal/httpapi/note"
	"go-web-template/internal/httpapi/response"
	"go-web-template/internal/httpapi/system"
	"go-web-template/internal/note"
)

type Dependencies struct {
	Logger               *logrus.Logger
	Readiness            []health.Check
	Notes                *note.Service
	HTTPPrefix           string
	GinMode              string
	CORSAllowedOrigins   []string
	CORSAllowCredentials bool
}

func NewRouter(dependencies Dependencies) http.Handler {
	gin.SetMode(dependencies.GinMode)
	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.Use(middleware.Request(dependencies.Logger), middleware.CORS(dependencies.CORSAllowedOrigins, dependencies.CORSAllowCredentials))
	router.NoRoute(func(c *gin.Context) { response.Error(c, http.StatusNotFound, "not found") })
	router.NoMethod(func(c *gin.Context) { response.Error(c, http.StatusMethodNotAllowed, "method not allowed") })
	system.Register(router, dependencies.Readiness)
	api := router.Group(dependencies.HTTPPrefix)
	noteapi.Register(api, dependencies.Logger, dependencies.Notes)
	return router
}

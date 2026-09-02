package frontend

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"

	"go-web-template/internal/httpapi/response"
)

//go:embed dist/*
var assets embed.FS

func Handler(apiPrefix string) gin.HandlerFunc {
	dist, _ := fs.Sub(assets, "dist")
	files := http.FileServer(http.FS(dist))
	return func(c *gin.Context) {
		if apiPrefix != "" && (c.Request.URL.Path == apiPrefix || strings.HasPrefix(c.Request.URL.Path, apiPrefix+"/")) {
			response.Error(c, http.StatusNotFound, "not found")
			return
		}
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			response.Error(c, http.StatusNotFound, "not found")
			return
		}
		name := strings.TrimPrefix(path.Clean(c.Request.URL.Path), "/")
		if name == "." {
			name = "index.html"
		}
		if _, err := fs.Stat(dist, name); err != nil {
			c.Request.URL.Path = "/"
		}
		files.ServeHTTP(c.Writer, c.Request)
	}
}

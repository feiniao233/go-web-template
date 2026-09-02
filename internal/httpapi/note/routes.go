package noteapi

import (
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"go-web-template/internal/note"
)

func Register(router *gin.RouterGroup, logger *logrus.Logger, service *note.Service) {
	h := handler{logger: logger, service: service}
	router.GET("/notes", h.list)
	router.POST("/notes", h.create)
	router.GET("/notes/:id", h.get)
	router.PUT("/notes/:id", h.update)
	router.DELETE("/notes/:id", h.delete)
}

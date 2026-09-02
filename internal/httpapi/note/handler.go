package noteapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"go-web-template/internal/httpapi/response"
	"go-web-template/internal/note"
)

type handler struct {
	logger  *logrus.Logger
	service *note.Service
}

type input struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

func decode(c *gin.Context) (input, bool) {
	defer c.Request.Body.Close()
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var values input
	if err := decoder.Decode(&values); err != nil {
		writeDecodeError(c, err)
		return input{}, false
	}
	if err := decoder.Decode(&struct{}{}); err == nil {
		response.Error(c, http.StatusBadRequest, "body must contain one JSON object")
		return input{}, false
	} else if !errors.Is(err, io.EOF) {
		writeDecodeError(c, err)
		return input{}, false
	}
	values.Title = strings.TrimSpace(values.Title)
	if values.Title == "" || utf8.RuneCountInString(values.Title) > 200 || len(values.Content) > 10000 {
		response.Error(c, http.StatusBadRequest, "title is required (max 200 characters); content max 10000 bytes")
		return input{}, false
	}
	return values, true
}

func writeDecodeError(c *gin.Context, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		response.Error(c, http.StatusRequestEntityTooLarge, "request body is too large")
		return
	}
	response.Error(c, http.StatusBadRequest, "invalid JSON body")
}

func id(c *gin.Context) (int64, bool) {
	value, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || value <= 0 {
		response.Error(c, http.StatusBadRequest, "id must be a positive integer")
		return 0, false
	}
	return value, true
}

func pagination(c *gin.Context) (int, int, bool) {
	page, pageSize := 1, 20
	var err error
	if value := c.Query("page"); value != "" {
		page, err = strconv.Atoi(value)
		if err != nil || page <= 0 {
			response.Error(c, http.StatusBadRequest, "page must be a positive integer")
			return 0, 0, false
		}
	}
	if value := c.Query("page_size"); value != "" {
		pageSize, err = strconv.Atoi(value)
		if err != nil || pageSize <= 0 || pageSize > 100 {
			response.Error(c, http.StatusBadRequest, "page_size must be between 1 and 100")
			return 0, 0, false
		}
	}
	if page > int(^uint(0)>>1)/pageSize {
		response.Error(c, http.StatusBadRequest, "page is too large")
		return 0, 0, false
	}
	return page, pageSize, true
}

func (h handler) list(c *gin.Context) {
	page, pageSize, ok := pagination(c)
	if !ok {
		return
	}
	notes, total, err := h.service.List(c.Request.Context(), page, pageSize)
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.Page(c, "ok", notes, total, page, pageSize)
}

func (h handler) get(c *gin.Context) {
	noteID, ok := id(c)
	if !ok {
		return
	}
	n, err := h.service.Get(c.Request.Context(), noteID)
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "ok", n)
}

func (h handler) create(c *gin.Context) {
	values, ok := decode(c)
	if !ok {
		return
	}
	n, err := h.service.Create(c.Request.Context(), note.Values{Title: values.Title, Content: values.Content})
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, http.StatusCreated, "created", n)
}

func (h handler) update(c *gin.Context) {
	noteID, ok := id(c)
	if !ok {
		return
	}
	values, ok := decode(c)
	if !ok {
		return
	}
	n, err := h.service.Update(c.Request.Context(), noteID, note.Values{Title: values.Title, Content: values.Content})
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "ok", n)
}

func (h handler) delete(c *gin.Context) {
	noteID, ok := id(c)
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), noteID); err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "deleted", nil)
}

func (h handler) writeError(c *gin.Context, err error) {
	if errors.Is(err, note.ErrNotFound) {
		response.Error(c, http.StatusNotFound, err.Error())
		return
	}
	h.logger.WithError(err).Error("note operation failed")
	response.Error(c, http.StatusServiceUnavailable, "database is unavailable")
}

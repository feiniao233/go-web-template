package frontend

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSPAAndAPINotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.NoRoute(Handler("/api/v1"))

	spa := httptest.NewRecorder()
	router.ServeHTTP(spa, httptest.NewRequest(http.MethodGet, "/devices/1", nil))
	require.Equal(t, http.StatusOK, spa.Code)
	assert.Contains(t, spa.Body.String(), "Go Web Service")

	api := httptest.NewRecorder()
	router.ServeHTTP(api, httptest.NewRequest(http.MethodGet, "/api/v1/missing", nil))
	require.Equal(t, http.StatusNotFound, api.Code)
	assert.JSONEq(t, `{"code":404,"msg":"not found","data":null}`, api.Body.String())

	post := httptest.NewRecorder()
	router.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/devices", nil))
	require.Equal(t, http.StatusNotFound, post.Code)
}

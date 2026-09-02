package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"go-web-template/internal/httpapi/response"
	"go-web-template/internal/note"
)

func testDependencies() Dependencies {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	return Dependencies{
		Logger: logger, Notes: note.NewService(note.NewRepository(nil)),
		HTTPPrefix: "/api/v1", GinMode: "test", CORSAllowedOrigins: []string{"*"},
	}
}

func TestRouterResponseShapeAndBoundaries(t *testing.T) {
	router := NewRouter(testDependencies())
	for _, test := range []struct {
		path   string
		status int
	}{
		{"/health", http.StatusOK},
		{"/ready", http.StatusOK},
		{"/version", http.StatusOK},
		{"/api/v1/notes", http.StatusServiceUnavailable},
		{"/api/v1/notes/not-a-number", http.StatusBadRequest},
		{"/api/v1/notes?page=0", http.StatusBadRequest},
		{"/api/v1/notes?page_size=101", http.StatusBadRequest},
	} {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, test.path, nil))
		require.Equal(t, test.status, res.Code, test.path)
		require.NotEmpty(t, res.Header().Get("X-Request-ID"))
		assert.Equal(t, "nosniff", res.Header().Get("X-Content-Type-Options"))
		var body map[string]any
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
		assert.Equal(t, float64(test.status), body["code"])
		assert.Contains(t, body, "msg")
		assert.Contains(t, body, "data")
		assert.NotContains(t, body, "success")
		assert.NotContains(t, body, "datas")
		if test.status >= 400 {
			assert.Nil(t, body["data"])
		}
		if test.path == "/version" {
			data, ok := body["data"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "dev", data["version"])
			assert.Contains(t, data, "commit")
			assert.Contains(t, data, "build_time")
		}
	}
}

func TestNoteRequestValidation(t *testing.T) {
	router := NewRouter(testDependencies())
	for _, test := range []struct {
		name   string
		body   string
		status int
	}{
		{name: "unicode title", body: `{"title":"` + strings.Repeat("中", 200) + `","content":""}`, status: http.StatusServiceUnavailable},
		{name: "title too long", body: `{"title":"` + strings.Repeat("中", 201) + `","content":""}`, status: http.StatusBadRequest},
		{name: "body too large", body: `{"title":"ok","content":"` + strings.Repeat("x", 1<<20) + `"}`, status: http.StatusRequestEntityTooLarge},
	} {
		t.Run(test.name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/notes", strings.NewReader(test.body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(res, req)
			assert.Equal(t, test.status, res.Code)
		})
	}
}

func TestCORSPreflight(t *testing.T) {
	router := NewRouter(testDependencies())
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/notes", nil)
	req.Header.Set("Origin", "https://client.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	req.Header.Set("Access-Control-Request-Headers", "Authorization")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	assert.Equal(t, http.StatusNoContent, res.Code)
	assert.Equal(t, "*", res.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, res.Header().Get("Access-Control-Allow-Methods"), "GET")
	assert.Contains(t, res.Header().Get("Access-Control-Allow-Headers"), "Authorization")

	req = httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "https://client.example")
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	assert.Contains(t, res.Header().Get("Access-Control-Expose-Headers"), "X-Request-Id")
}

func TestCreatedAndDeletedResponseCodes(t *testing.T) {
	for _, test := range []struct {
		httpStatus int
		msg        string
	}{
		{http.StatusCreated, "created"},
		{http.StatusOK, "deleted"},
	} {
		res := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(res)
		response.Success(c, test.httpStatus, test.msg, nil)
		assert.Equal(t, test.httpStatus, res.Code)
		var body struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
		assert.Equal(t, 200, body.Code)
		assert.Equal(t, test.msg, body.Msg)
	}
}

func TestNotesPagination(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		DisableAutomaticPing: true,
		Logger:               gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)
	now := time.Now().UTC()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "notes"`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "notes" ORDER BY id LIMIT $1 OFFSET $2`)).
		WithArgs(2, 2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "content", "created_at", "updated_at"}).
			AddRow(3, "third", "note", now, now))
	deps := testDependencies()
	deps.Notes = note.NewService(note.NewRepository(db))
	res := httptest.NewRecorder()
	NewRouter(deps).ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/notes?page=2&page_size=2", nil))

	require.Equal(t, http.StatusOK, res.Code)
	var body struct {
		Code     int   `json:"code"`
		Total    int64 `json:"total"`
		Page     int   `json:"page"`
		PageSize int   `json:"page_size"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
	assert.Equal(t, 200, body.Code)
	assert.Equal(t, int64(3), body.Total)
	assert.Equal(t, 2, body.Page)
	assert.Equal(t, 2, body.PageSize)
	require.NoError(t, mock.ExpectationsWereMet())
}

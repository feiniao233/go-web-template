package note

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestRepositoryList(t *testing.T) {
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
			AddRow(1, "hello", "world", now, now))
	notes, total, err := NewRepository(db).List(context.Background(), 2, 2)

	require.NoError(t, err)
	require.Len(t, notes, 1)
	assert.Equal(t, int64(1), notes[0].ID)
	assert.Equal(t, "hello", notes[0].Title)
	assert.Equal(t, int64(3), total)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUnavailablePreservesCause(t *testing.T) {
	cause := errors.New("connection reset")
	err := unavailable(cause)

	assert.ErrorIs(t, err, ErrUnavailable)
	assert.ErrorIs(t, err, cause)
}

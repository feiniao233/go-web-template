package tdengine

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/taosdata/driver-go/v3/taosWS"
)

var ErrDisabled = errors.New("TDengine is not configured")

type Options struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type TDengine struct {
	db *sql.DB
}

func Open(dsn string, options Options) (*TDengine, error) {
	if dsn == "" {
		return &TDengine{}, nil
	}
	db, err := sql.Open("taosWS", dsn)
	if err != nil {
		return nil, fmt.Errorf("open TDengine: %w", err)
	}
	db.SetMaxOpenConns(options.MaxOpenConns)
	db.SetMaxIdleConns(options.MaxIdleConns)
	db.SetConnMaxLifetime(options.ConnMaxLifetime)
	return &TDengine{db: db}, nil
}

func (t *TDengine) Enabled() bool { return t != nil && t.db != nil }

func (t *TDengine) DB() *sql.DB {
	if t == nil {
		return nil
	}
	return t.db
}

func (t *TDengine) Ping(ctx context.Context) error {
	if !t.Enabled() {
		return ErrDisabled
	}
	return t.db.PingContext(ctx)
}

func (t *TDengine) Close() error {
	if !t.Enabled() {
		return nil
	}
	return t.db.Close()
}

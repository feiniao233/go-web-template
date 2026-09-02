package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"time"

	clickhousego "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

var ErrDisabled = errors.New("ClickHouse is not configured")

type ClickHouse struct {
	client driver.Conn
}

type Options struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

func Open(dsn string, pool Options) (*ClickHouse, error) {
	if dsn == "" {
		return &ClickHouse{}, nil
	}
	options, err := clickhousego.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse ClickHouse DSN: %w", err)
	}
	options.MaxOpenConns = pool.MaxOpenConns
	options.MaxIdleConns = pool.MaxIdleConns
	options.ConnMaxLifetime = pool.ConnMaxLifetime
	client, err := clickhousego.Open(options)
	if err != nil {
		return nil, fmt.Errorf("open ClickHouse: %w", err)
	}
	return &ClickHouse{client: client}, nil
}

func (c *ClickHouse) Enabled() bool { return c != nil && c.client != nil }

func (c *ClickHouse) Client() driver.Conn {
	if c == nil {
		return nil
	}
	return c.client
}

func (c *ClickHouse) Ping(ctx context.Context) error {
	if !c.Enabled() {
		return ErrDisabled
	}
	return c.client.Ping(ctx)
}

func (c *ClickHouse) Close() error {
	if !c.Enabled() {
		return nil
	}
	return c.client.Close()
}

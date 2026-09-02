package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var ErrDisabled = errors.New("database is not configured")

type Database struct {
	sql *sql.DB
	orm *gorm.DB
}

type Options struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	SlowThreshold   time.Duration
	LogLevel        string
	Logger          *logrus.Logger
}

func New(sqlDB *sql.DB, dialector gorm.Dialector, options Options) (*Database, error) {
	sqlDB.SetMaxOpenConns(options.MaxOpenConns)
	sqlDB.SetMaxIdleConns(options.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(options.ConnMaxLifetime)
	logger := options.Logger
	if logger == nil {
		logger = logrus.New()
	}
	levels := map[string]gormlogger.LogLevel{
		"silent": gormlogger.Silent,
		"error":  gormlogger.Error,
		"warn":   gormlogger.Warn,
		"info":   gormlogger.Info,
	}
	orm, err := gorm.Open(dialector, &gorm.Config{
		NowFunc: func() time.Time { return time.Now().UTC() },
		Logger: gormlogger.New(logger, gormlogger.Config{
			SlowThreshold: options.SlowThreshold,
			LogLevel:      levels[options.LogLevel],
			Colorful:      false,
		}),
	})
	if err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("initialize GORM: %w", err)
	}
	return &Database{sql: sqlDB, orm: orm}, nil
}

func Disabled() *Database { return &Database{} }

func (d *Database) Enabled() bool { return d != nil && d.sql != nil }

func (d *Database) ORM() *gorm.DB {
	if d == nil {
		return nil
	}
	return d.orm
}

func (d *Database) SQL() *sql.DB {
	if d == nil {
		return nil
	}
	return d.sql
}

func (d *Database) Ping(ctx context.Context) error {
	if !d.Enabled() {
		return ErrDisabled
	}
	return d.sql.PingContext(ctx)
}

func (d *Database) Close() error {
	if !d.Enabled() {
		return nil
	}
	return d.sql.Close()
}

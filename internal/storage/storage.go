package storage

import (
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"

	"go-web-template/internal/config"
	"go-web-template/internal/health"
	"go-web-template/internal/platform/database"
	postgresstore "go-web-template/internal/platform/database/postgres"
	redisstore "go-web-template/internal/platform/redis"
	postgresmigrations "go-web-template/migrations/postgres"
)

type closer interface {
	Close() error
}

type Storage struct {
	Primary *database.Database
	Redis   *redisstore.Redis
	checks  []health.Check
	closers []closer
}

func Open(cfg config.Config, logger *logrus.Logger) (*Storage, error) {
	primary, err := openPrimary(cfg, logger)
	if err != nil {
		return nil, err
	}
	if primary.Enabled() && cfg.Database.MigrateOnStart {
		if err := postgresmigrations.Up(primary.SQL()); err != nil {
			_ = primary.Close()
			return nil, fmt.Errorf("migrate PostgreSQL: %w", err)
		}
	}
	redisClient, err := redisstore.Open(cfg.Redis.URL)
	if err != nil {
		_ = primary.Close()
		return nil, err
	}
	storage := &Storage{Primary: primary, Redis: redisClient, closers: []closer{redisClient, primary}}
	if primary.Enabled() {
		storage.checks = append(storage.checks, health.Check{Name: "database", Ping: primary.Ping})
	}
	if redisClient.Enabled() {
		storage.checks = append(storage.checks, health.Check{Name: "redis", Ping: redisClient.Ping})
	}
	return storage, nil
}

func openPrimary(cfg config.Config, logger *logrus.Logger) (*database.Database, error) {
	primary, err := postgresstore.Open(cfg.Database.DSN, database.Options{
		MaxOpenConns: cfg.Database.MaxOpenConns, MaxIdleConns: cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime, SlowThreshold: cfg.Database.SlowThreshold,
		LogLevel: cfg.Database.LogLevel, Logger: logger,
	})
	if err != nil {
		return nil, err
	}
	return primary, nil
}

func Migrate(cfg config.Config, logger *logrus.Logger, command string, steps int) (uint, bool, error) {
	primary, err := openPrimary(cfg, logger)
	if err != nil {
		return 0, false, err
	}
	defer primary.Close()
	if !primary.Enabled() {
		return 0, false, fmt.Errorf("DATABASE_DSN is required")
	}
	switch command {
	case "up":
		err = postgresmigrations.Up(primary.SQL())
	case "down":
		if steps <= 0 {
			return 0, false, fmt.Errorf("steps must be positive")
		}
		err = postgresmigrations.Down(primary.SQL(), steps)
	case "version":
		return postgresmigrations.Version(primary.SQL())
	default:
		return 0, false, fmt.Errorf("unsupported migration command %q", command)
	}
	if err != nil {
		return 0, false, err
	}
	version, dirty, versionErr := postgresmigrations.Version(primary.SQL())
	return version, dirty, versionErr
}

func (s *Storage) Checks() []health.Check { return s.checks }

func (s *Storage) Close() error {
	var errs []error
	for _, resource := range s.closers {
		if err := resource.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

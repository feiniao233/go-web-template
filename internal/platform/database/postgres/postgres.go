package postgres

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	postgresdialect "gorm.io/driver/postgres"

	"go-web-template/internal/platform/database"
)

func Open(dsn string, options database.Options) (*database.Database, error) {
	if dsn == "" {
		return database.Disabled(), nil
	}
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL: %w", err)
	}
	db, err := database.New(sqlDB, postgresdialect.New(postgresdialect.Config{Conn: sqlDB}), options)
	if err != nil {
		return nil, fmt.Errorf("initialize PostgreSQL: %w", err)
	}
	return db, nil
}

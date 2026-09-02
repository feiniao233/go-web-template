package mysql

import (
	"database/sql"
	"fmt"

	mysqldialect "gorm.io/driver/mysql"

	"go-web-template/internal/platform/database"
)

func Open(dsn string, options database.Options) (*database.Database, error) {
	if dsn == "" {
		return database.Disabled(), nil
	}
	sqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open MySQL: %w", err)
	}
	db, err := database.New(sqlDB, mysqldialect.New(mysqldialect.Config{Conn: sqlDB}), options)
	if err != nil {
		return nil, fmt.Errorf("initialize MySQL: %w", err)
	}
	return db, nil
}

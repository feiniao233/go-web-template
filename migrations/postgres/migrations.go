package postgres

import (
	"database/sql"
	"embed"
	"errors"

	"github.com/golang-migrate/migrate/v4"
	postgresdriver "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed *.sql
var files embed.FS

func Up(db *sql.DB) error {
	migration, err := open(db)
	if err != nil {
		return err
	}
	err = migration.Up()
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	return err
}

func Down(db *sql.DB, steps int) error {
	migration, err := open(db)
	if err != nil {
		return err
	}
	err = migration.Steps(-steps)
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	return err
}

func Version(db *sql.DB) (uint, bool, error) {
	migration, err := open(db)
	if err != nil {
		return 0, false, err
	}
	version, dirty, err := migration.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, nil
	}
	return version, dirty, err
}

func open(db *sql.DB) (*migrate.Migrate, error) {
	source, err := iofs.New(files, ".")
	if err != nil {
		return nil, err
	}
	driver, err := postgresdriver.WithInstance(db, &postgresdriver.Config{})
	if err != nil {
		return nil, err
	}
	migration, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return nil, err
	}
	return migration, nil
}

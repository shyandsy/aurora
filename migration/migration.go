package migration

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	goose "github.com/pressly/goose/v3"

	"github.com/shyandsy/di"
)

func RunMigrations(container di.Container) error {
	var sqlDB *sql.DB
	if err := container.Resolve(sqlDB); err != nil {
		return errors.New("sql.DB not found")
	}

	if err := goose.SetDialect("mysql"); err != nil {
		return fmt.Errorf("failed to set dialect: %v", err)
	}

	migrationsDir := getMigrationsFolder()
	if err := goose.Up(sqlDB, migrationsDir); err != nil {
		if errors.Is(err, goose.ErrNoMigrationFiles) {
			return nil
		}
		return fmt.Errorf("failed to run migrations: %v", err)
	}

	return nil
}

func getMigrationsFolder() string {
	wd, err := os.Getwd()
	if err != nil {
		return "migrations"
	}
	return filepath.Join(wd, "migrations")
}

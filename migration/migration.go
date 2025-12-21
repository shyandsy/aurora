package migration

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	goose "github.com/pressly/goose/v3"

	auroraConfig "github.com/shyandsy/aurora/config"
	"github.com/shyandsy/di"
)

func RunMigrations(container di.Container) error {
	var sqlDB *sql.DB
	if err := container.Find(&sqlDB); err != nil {
		return errors.New("sql.DB not found")
	}

	if err := goose.SetDialect("mysql"); err != nil {
		return fmt.Errorf("failed to set dialect: %v", err)
	}

	// Load migration configuration (table name prefix)
	var migrationCfg auroraConfig.MigrationConfig
	if err := auroraConfig.ResolveConfig(&migrationCfg); err != nil {
		// If config loading fails, use default table name (do not set custom table name)
		log.Printf("Failed to load MigrationConfig, using default goose table name: %v", err)
	}

	// Only set custom table name if table prefix is configured
	// If TablePrefix is empty, use goose default table name "goose_db_version"
	if migrationCfg.TablePrefix != "" {
		tableName := migrationCfg.GetTableName()
		goose.SetTableName(tableName)
		log.Printf("Using custom goose table name: %s", tableName)
	} else {
		log.Printf("Using default goose table name: goose_db_version")
	}

	migrationsDir := getMigrationsFolder()
	if err := goose.Up(sqlDB, migrationsDir); err != nil {
		if errors.Is(err, goose.ErrNoMigrationFiles) {
			log.Println("Database migrations no files found")
			return nil
		}
		return fmt.Errorf("failed to run migrations: %v", err)
	}

	currentVersion, err := goose.GetDBVersion(sqlDB)
	if err != nil {
		log.Printf("Database migrations completed successfully (unable to get version: %v)", err)
	} else {
		log.Printf("Database migrations completed successfully, current version: %d", currentVersion)
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

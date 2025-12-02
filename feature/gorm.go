package feature

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/shyandsy/aurora/config"
	"github.com/shyandsy/aurora/contracts"

	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type gormFeature struct {
	db     *gorm.DB
	config *config.DatabaseConfig
}

func NewGormFeature() contracts.Features {
	cfg := &config.DatabaseConfig{}
	if err := config.ResolveConfig(cfg); err != nil {
		log.Fatalf("Failed to load database config: %v", err)
	}
	return &gormFeature{config: cfg}
}

func (f *gormFeature) Name() string {
	return "gorm"
}

func (f *gormFeature) Setup(app contracts.App) error {
	if err := f.config.Validate(); err != nil {
		return fmt.Errorf("database configuration validation failed: %w", err)
	}

	var sqlDB *sql.DB
	var err error
	f.db, sqlDB, err = f.provideDatabase()
	if err != nil {
		return fmt.Errorf("failed to provide database: %w", err)
	}

	app.Provide(f.db)
	app.Provide(sqlDB)

	return nil
}

func (f *gormFeature) Close() error {
	if f.db != nil {
		sqlDB, err := f.db.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}

func (f *gormFeature) provideDatabase() (*gorm.DB, *sql.DB, error) {
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	var db *gorm.DB
	var err error

	switch f.config.Driver {
	case "mysql":
		db, err = gorm.Open(mysql.Open(f.config.DSN), gormConfig)
	case "sqlite":
		db, err = gorm.Open(sqlite.Open(f.config.DSN), gormConfig)
	default:
		log.Fatalf("Unsupported database driver: %s, supported drivers: mysql, sqlite", f.config.Driver)
	}

	if err != nil {
		log.Fatalf("Failed to connect to %s database: %v", f.config.Driver, err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get underlying sql.DB: %v", err)
	}

	sqlDB.SetMaxIdleConns(f.config.MaxIdleConns)
	sqlDB.SetMaxOpenConns(f.config.MaxOpenConns)

	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("Failed to ping %s database: %v", f.config.Driver, err)
	}

	return db, sqlDB, nil
}

package bootstrap

import (
	"fmt"

	"github.com/shyandsy/aurora/app"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/feature"
	"github.com/shyandsy/aurora/migration"
)

// InitDefaultApp creates and configures a default Aurora App instance
func InitDefaultApp() contracts.App {
	a := app.NewApp()

	server := feature.NewServerFeature()
	a.AddFeature(server)

	a.AddFeature(feature.NewGormFeature())
	a.AddFeature(feature.NewRedisFeature())
	a.AddFeature(feature.NewJWTFeature())
	a.AddFeature(feature.NewI18NFeature())
	a.AddFeature(feature.NewMailFeature())

	if err := migration.RunMigrations(a); err != nil {
		panic(fmt.Errorf("database migration failed: %w", err))
	}

	return a
}

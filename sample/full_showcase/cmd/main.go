package main

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/shyandsy/aurora/app"
	auroraFeature "github.com/shyandsy/aurora/feature"
	"github.com/shyandsy/aurora/logger"
	"github.com/shyandsy/aurora/migration"
	"github.com/shyandsy/aurora/sample/full_showcase/controller"
)

func main() {
	// Load .env if present (env vars take precedence)
	if err := godotenv.Load(); err != nil {
		// Missing .env is ok; config may come from env
		log.Printf("Warning: .env file not found, using environment variables: %v", err)
	}

	// Create app (no Mail feature)
	a := app.NewApp()

	// Add features
	server := auroraFeature.NewServerFeature()
	a.AddFeature(server)
	a.AddFeature(auroraFeature.NewGormFeature())
	a.AddFeature(auroraFeature.NewRedisFeature())
	a.AddFeature(auroraFeature.NewJWTFeature())
	a.AddFeature(auroraFeature.NewI18NFeature())
	// MailFeature omitted for this sample

	// Run migrations
	if err := migration.RunMigrations(a); err != nil {
		logger.Errorf("migration failed: %v", err)
		return
	}

	// Register DI providers
	registerProviders(a)

	// Register routes
	a.RegisterRoutes(controller.GetRoutes(a))

	// Run server (blocks until shutdown)
	if err := a.Run(); err != nil {
		logger.Errorf("app run failed: %v", err)
		return
	}

	logger.Info("app exited")
}

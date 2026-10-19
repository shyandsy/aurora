package main

import (
	"log"
	"time"

	"github.com/joho/godotenv"
	"github.com/shyandsy/aurora/app"
	"github.com/shyandsy/aurora/config"
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
	// JWT 由项目显式构造(框架不读 env、也不定 env 名)。本单域 sample 用一个自带 env tag 的
	// 本地结构体从 env 读出,再映射进 aurora 的 config.JWTConfig。env 名(JWT_SECRET 等)由项目
	// 在这里决定;多域项目(同一进程多把密钥)改用各自前缀(ADMIN_/CUSTOMER_…)即可。
	var jwtEnv struct {
		Secret        string        `env:"JWT_SECRET"`
		Issuer        string        `env:"JWT_ISSUER"`
		ExpireTime    time.Duration `env:"JWT_EXPIRE_TIME"`
		RefreshExpire time.Duration `env:"JWT_REFRESH_EXPIRE_TIME,omitempty"`
	}
	if err := config.ResolveConfig(&jwtEnv); err != nil {
		log.Fatalf("load JWT config failed: %v", err)
	}
	a.AddFeature(auroraFeature.NewJWTFeature(&config.JWTConfig{
		Secret:            jwtEnv.Secret,
		Issuer:            jwtEnv.Issuer,
		ExpireTime:        jwtEnv.ExpireTime,
		RefreshExpireTime: jwtEnv.RefreshExpire,
	}))
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

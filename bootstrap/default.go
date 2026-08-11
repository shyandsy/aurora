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
	// 发信不再是自动注册的 feature —— 见 feature/mail 包:app 用 mail.NewSMTP(...) 等按需构造 Mailer,
	// 自己决定供应商/授权/凭据来源(env、DB、临时都行),不再从 env 焊死一个全局 EmailService。

	if err := migration.RunMigrations(a); err != nil {
		panic(fmt.Errorf("database migration failed: %w", err))
	}

	return a
}

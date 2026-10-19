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
	a.AddFeature(feature.NewI18NFeature())
	// 以下三样都**不进默认集**,由各服务按需、自行构造后 AddFeature:
	//   - JWT:需要一把密钥,而"用哪把、env 叫什么"是**项目的 policy**,框架不替项目决定、也不保留
	//     一个偷读 JWT_SECRET 的隐式默认。鉴权服务 app.AddFeature(feature.NewJWTFeature(cfg))(须在 redis 之后),
	//     由项目用自己带 env tag 的结构体读出(单域 JWT_SECRET、多域按自己的前缀)再组 *config.JWTConfig。
	//   - tokenguard(token 会话有效性):opt-in,鉴权服务 app.AddFeature(
	//     tokenguard.NewTokenGuardFeature(jc.RefreshExpireOrDefault()))(须在 redis + jwt 之后)。
	//   - mail:见 feature/mail 包,app 用 mail.NewSMTP(...) 等按需构造 Mailer,自己决定供应商/凭据来源,
	//     不再从 env 焊死一个全局 EmailService。
	// 纯 worker / 非鉴权服务这三样都不必带。

	if err := migration.RunMigrations(a); err != nil {
		panic(fmt.Errorf("database migration failed: %w", err))
	}

	return a
}

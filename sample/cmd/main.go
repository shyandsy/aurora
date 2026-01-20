package main

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/shyandsy/aurora/app"
	auroraFeature "github.com/shyandsy/aurora/feature"
	"github.com/shyandsy/aurora/logger"
	"github.com/shyandsy/aurora/migration"
	"github.com/shyandsy/aurora/sample/controller"
)

func main() {
	// 加载 .env 文件（如果存在）
	// 注意：如果环境变量已经设置，.env 文件中的值不会覆盖它们
	if err := godotenv.Load(); err != nil {
		// .env 文件不存在不是错误，可能是通过环境变量传入配置
		log.Printf("Warning: .env file not found, using environment variables: %v", err)
	}

	// 创建应用（手动初始化，不包含 Mail 功能）
	a := app.NewApp()

	// 添加必需的 Feature
	server := auroraFeature.NewServerFeature()
	a.AddFeature(server)
	a.AddFeature(auroraFeature.NewGormFeature())
	a.AddFeature(auroraFeature.NewRedisFeature())
	a.AddFeature(auroraFeature.NewJWTFeature())
	a.AddFeature(auroraFeature.NewI18NFeature())
	// 注意：不添加 MailFeature，因为 sample 项目不需要邮件功能

	// 运行数据库迁移
	if err := migration.RunMigrations(a); err != nil {
		logger.Errorf("数据库迁移失败: %v", err)
		return
	}

	// 注册所有 providers
	registerProviders(a)

	// 注册路由
	a.RegisterRoutes(controller.GetRoutes(a))

	// 启动服务器（会阻塞直到收到退出信号）
	if err := a.Run(); err != nil {
		logger.Errorf("应用运行失败: %v", err)
		return
	}

	logger.Info("应用已完全退出")
}

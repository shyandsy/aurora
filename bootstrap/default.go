package bootstrap

import (
	"fmt"

	"github.com/shyandsy/aurora/app" // 导入具体的 App 实现
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/migration"

	// 导入所有内置 Features 的具体实现，只有这里需要知道它们
	"github.com/shyandsy/aurora/feature"
)

// InitDefaultApp 创建并配置一个默认的 Aurora App 实例
func InitDefaultApp() contracts.App {
	a := app.NewApp() // 实例化具体的 App 实现

	// 1. 强制注入的核心 Features
	server := feature.NewServerFeature()
	a.AddFeature(server) // AddFeature 方法接受 contracts.Features 接口

	// Gorm 和 Redis 作为基础服务自动注入
	a.AddFeature(feature.NewGormFeature())
	a.AddFeature(feature.NewRedisFeature())
	a.AddFeature(feature.NewJWTFeature())

	if err := migration.RunMigrations(a); err != nil {
		panic(fmt.Errorf("database migration failed: %w", err))
	}

	return a
}

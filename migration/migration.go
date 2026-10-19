package migration

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	goose "github.com/pressly/goose/v3"

	auroraConfig "github.com/shyandsy/aurora/config"
	"github.com/shyandsy/di"
)

// RunMigrations 执行数据库迁移。支持「两套有序执行」:
//   - 若工作目录下存在 migrations-schema/(共享模块如 user 自带的建表/改表结构迁移),
//     **先**用独立版本表 {prefix}schema_goose_db_version 跑它(schema 套,归 aurora 模块维护);
//   - **再**用 {prefix}goose_db_version 跑 migrations/ 里项目自己的业务迁移(data 套,归项目维护)。
//
// 两套各自独立版本表、各自编号、各自维护;schema 恒先于 data。两个都是普通磁盘目录、
// 两次 goose.Up(默认 osFS),没有任何内嵌/Provider 机制。没有 migrations-schema/ 的服务
// 就是单套,行为与之前完全一致(向后兼容)。
func RunMigrations(container di.Container) error {
	var sqlDB *sql.DB
	if err := container.Find(&sqlDB); err != nil {
		return errors.New("sql.DB not found")
	}

	var dbCfg auroraConfig.DatabaseConfig
	if err := auroraConfig.ResolveConfig(&dbCfg); err != nil {
		return fmt.Errorf("failed to resolve database config for migrations: %v", err)
	}
	dialect := gooseDialect(dbCfg.Driver)
	if err := goose.SetDialect(dialect); err != nil {
		return fmt.Errorf("failed to set dialect %q: %v", dialect, err)
	}

	// Load migration configuration (table name prefix)
	var migrationCfg auroraConfig.MigrationConfig
	if err := auroraConfig.ResolveConfig(&migrationCfg); err != nil {
		// If config loading fails, use default table name (do not set custom table name)
		log.Printf("Failed to load MigrationConfig, using default goose table name: %v", err)
	}

	// 1) schema 套:共享模块自带的建表/改表迁移(目录 migrations-schema),有则先跑、独立版本表。
	if schemaDir := migrationsFolder("migrations-schema"); dirHasSQL(schemaDir) {
		if err := applyMigrations(sqlDB, migrationCfg.GetSchemaTableName(), schemaDir, "schema"); err != nil {
			return err
		}
	}

	// 2) data 套:项目自己的业务迁移(目录 migrations),后跑。
	return applyMigrations(sqlDB, migrationCfg.GetTableName(), migrationsFolder("migrations"), "data")
}

// applyMigrations 对指定版本表 + 目录跑一趟 goose.Up(调用方已 SetDialect)。
// 每趟先 SetTableName 切到本套版本表(goose 包级全局,顺序调用安全);目录无迁移文件视为成功跳过。
func applyMigrations(sqlDB *sql.DB, tableName, dir, stream string) error {
	goose.SetTableName(tableName)
	log.Printf("[migrate:%s] goose table=%s dir=%s", stream, tableName, dir)
	if err := goose.Up(sqlDB, dir); err != nil {
		if errors.Is(err, goose.ErrNoMigrationFiles) {
			log.Printf("[migrate:%s] no migration files, skip", stream)
			return nil
		}
		return fmt.Errorf("failed to run %s migrations: %v", stream, err)
	}
	if v, err := goose.GetDBVersion(sqlDB); err == nil {
		log.Printf("[migrate:%s] completed, current version: %d", stream, v)
	}
	return nil
}

// migrationsFolder 返回 <工作目录>/<name>(绝对路径,goose 默认 osFS 直接读)。
func migrationsFolder(name string) string {
	wd, err := os.Getwd()
	if err != nil {
		return name
	}
	return filepath.Join(wd, name)
}

// dirHasSQL 判断目录存在且至少含一个 .sql 文件。
func dirHasSQL(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			return true
		}
	}
	return false
}

func gooseDialect(driver string) string {
	switch driver {
	case "postgres", "postgresql", "pgx":
		return "postgres"
	case "sqlite":
		return "sqlite3"
	default:
		return driver
	}
}

package user

import (
	"strings"
	"testing"

	"github.com/shyandsy/aurora/modules/user/model/entity"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// openTestDB 起一个内存 sqlite 并把全部受管实体的表建好,模拟"结构齐全"的库。
//
// 不用 AutoMigrate:实体的 gorm 列类型带 MySQL 专属写法(`ON UPDATE CURRENT_TIMESTAMP`、
// 库级重名的 `idx_name` 索引…),sqlite 建不出来(记忆:fake sqlite 建不出真 MySQL schema)。
// 而 verify 只校验表/列**存在**、不在乎类型,故从实体自省出列名、建 sqlite 友好的极简表(全 TEXT)——
// 既不重复维护 schema,又绕开所有 MySQL-ism。列集与 verify 同源自实体,恰好用来验证遍历逻辑本身。
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	for _, e := range guardedEntities() {
		stmt := &gorm.Statement{DB: db}
		if err := stmt.Parse(e); err != nil {
			t.Fatalf("parse %T: %v", e, err)
		}
		var cols []string
		for _, f := range stmt.Schema.Fields {
			if f.DBName == "" {
				continue
			}
			cols = append(cols, "`"+f.DBName+"` TEXT")
		}
		sql := "CREATE TABLE `" + stmt.Schema.Table + "` (" + strings.Join(cols, ", ") + ")"
		if err := db.Exec(sql).Error; err != nil {
			t.Fatalf("create %s: %v", stmt.Schema.Table, err)
		}
	}
	return db
}

// 结构齐全 → 通过。
func TestVerifySchema_AllPresent(t *testing.T) {
	db := openTestDB(t)
	if err := verifySchema(db); err != nil {
		t.Fatalf("结构齐全应通过,却报: %v", err)
	}
}

// 缺表 → 报「表缺失」且点名该表。
func TestVerifySchema_MissingTable(t *testing.T) {
	db := openTestDB(t)
	if err := db.Migrator().DropTable(&entity.UserSession{}); err != nil {
		t.Fatalf("drop table: %v", err)
	}
	err := verifySchema(db)
	if err == nil {
		t.Fatal("缺表应报错,却通过")
	}
	if !strings.Contains(err.Error(), "表缺失") || !strings.Contains(err.Error(), "user_session") {
		t.Fatalf("错误应点名缺失的 user_session 表,got: %v", err)
	}
}

// 缺列 → 报「缺列」且点名表与列。用 raw DROP COLUMN 去掉一个**非索引**列
// (避开 gorm 在 sqlite 上的整表重建会重建索引、再撞全局重名的问题)。
func TestVerifySchema_MissingColumn(t *testing.T) {
	db := openTestDB(t)
	if err := db.Exec("ALTER TABLE user_users DROP COLUMN totp_recovery_codes").Error; err != nil {
		t.Fatalf("drop column: %v", err)
	}
	err := verifySchema(db)
	if err == nil {
		t.Fatal("缺列应报错,却通过")
	}
	if !strings.Contains(err.Error(), "缺列") ||
		!strings.Contains(err.Error(), "user_users") ||
		!strings.Contains(err.Error(), "totp_recovery_codes") {
		t.Fatalf("错误应点名 user_users 缺 totp_recovery_codes 列,got: %v", err)
	}
}

// 空库(什么都没建)→ 全部表缺失,一次性汇总(证明 goose 源指错/没跑迁移会被拦下)。
func TestVerifySchema_EmptyDB(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	verr := verifySchema(db)
	if verr == nil {
		t.Fatal("空库应报错")
	}
	for _, tbl := range []string{
		"user_users", "user_roles", "user_features", "user_role_features",
		"user_session", "user_microservice_token_features", "user_microservice_token_features_token",
	} {
		if !strings.Contains(verr.Error(), tbl) {
			t.Fatalf("空库错误应点名 %s,got: %v", tbl, verr)
		}
	}
}

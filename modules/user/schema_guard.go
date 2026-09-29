package user

import (
	"fmt"
	"strings"

	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/modules/user/model/entity"
	"gorm.io/gorm"
)

// schema_guard.go —— 启动期 schema 一致性闸。
//
// 迁移在 bootstrap.InitDefaultApp() 里跑,早于本模块 Setup;等 Setup 时 goose 已跑完/已 baseline。
// 这道闸在此刻校验「模块代码所需的表 + 列,库里是否真的都在」,不满足即 FATAL —— 让存量接入
// (本地阶段自己写回填 / baseline 版本号)变安全:baseline 写漏号、存量表 drift、goose 源指错,
// 第一次启动就死在这里,不会静默固化成运行时 500。
//
// 刻意只查**结构**(表在/列在),绝不查**内容**(库里有哪些 feature 行是各项目自己的事,归 host,
// 见设计稿「表结构归 aurora、权限数据归各项目」)。
//
// 「所需 schema」从模块自己的 entity 自省得来(TableName + gorm column tag),不另维护描述文件——
// entity 就是代码对 schema 的唯一契约,自省保证永不和迁移漂移。

// guardedEntities 是本模块代码通过 gorm 直接映射的实体;闸逐个断言其表与列存在。
// (user_ui_feature_grants / user_feature_dependencies 无独立 entity,靠 join/展开访问,不在此列。)
func guardedEntities() []any {
	return []any{
		&entity.User{},
		&entity.Role{},
		&entity.Feature{},
		&entity.RoleFeature{},
		&entity.UserSession{},
		&entity.MicroserviceTokenFeature{},
		&entity.MicroserviceTokenFeatureToken{},
	}
}

// verifySchema 校验每个受管实体的表与其映射的每一列都存在;返回所有缺失项的汇总错误(nil=通过)。
func verifySchema(db *gorm.DB) error {
	m := db.Migrator()
	var problems []string

	for _, e := range guardedEntities() {
		stmt := &gorm.Statement{DB: db}
		if err := stmt.Parse(e); err != nil {
			problems = append(problems, fmt.Sprintf("%T: 解析 schema 失败: %v", e, err))
			continue
		}
		table := stmt.Schema.Table

		if !m.HasTable(e) {
			problems = append(problems, fmt.Sprintf("表缺失: %s", table))
			continue // 表都没有,不必再逐列报
		}

		for _, f := range stmt.Schema.Fields {
			if f.DBName == "" {
				continue // 关联字段 / gorm:"-",无对应列,跳过
			}
			if !m.HasColumn(e, f.DBName) {
				problems = append(problems, fmt.Sprintf("%s 缺列: %s", table, f.DBName))
			}
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("模块所需结构与库不符:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return nil
}

// resolveDB 从容器解析 *gorm.DB(与各 datalayer 同款 inject:"" 姿势)。
func resolveDB(app contracts.App) (*gorm.DB, error) {
	holder := &struct {
		DB *gorm.DB `inject:""`
	}{}
	if err := app.Resolve(holder); err != nil {
		return nil, err
	}
	return holder.DB, nil
}

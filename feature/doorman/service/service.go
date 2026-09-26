package service

// 本文件是 service 层对根包(feature 装配)暴露的**唯一入口**:把「注册表 + 存储」装成运行时 Doorman
// 与管理面 Console。存储实现(dbStore)、各内部契约都不导出——根包只拿 core.Doorman / Console / Pruner。

import (
	"time"

	"github.com/shyandsy/aurora/feature/doorman/core"
	"gorm.io/gorm"
)

// Pruner 清理早于 before 的决策流水(每批最多 batch 条),返回删除总数。retention goroutine 用它,
// 不必知道存储细节。见 dbStore.pruneDecisions。
type Pruner func(before time.Time, batch int) (int64, error)

// NewWithStore 默认装配:基于 *gorm.DB 自建存储,产出运行时 Doorman、管理面 Console、决策流水清理器。
// 三张 doorman_* 表由宿主服务的 goose 建(不自 DDL)。ttl<=0 用引擎默认(30s)。
func NewWithStore(reg *core.Registry, db *gorm.DB, ttl time.Duration) (core.Doorman, Console, Pruner) {
	store := newDBStore(db)
	d := core.New(reg, store, store, store, ttl) // store 同时是 RuleSource/PolicyStore/DecisionRecorder
	return d, newConsole(reg, store), store.pruneDecisions
}

// NewWithSource 高级 override 装配:业务自带规则来源(不落 doorman 表,也就没有 Console / 策略 / 流水)。
// 用于「只想用评估引擎、规则从别处来」的场景。ttl<=0 用引擎默认。
func NewWithSource(reg *core.Registry, src core.RuleSource, ttl time.Duration) core.Doorman {
	return core.New(reg, src, nil, nil, ttl)
}

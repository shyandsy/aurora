package service

// 本文件是 Console 的**策略**面:「风险等级 → 动作」映射的读写。

import (
	"fmt"

	"github.com/shyandsy/aurora/feature/doorman/core"
	"github.com/shyandsy/aurora/feature/doorman/model/dto"
)

func (a *console) PolicyKinds(scope string) dto.PolicyKindsDTO {
	return dto.PolicyKindsDTO{RiskLevels: core.RiskLevelStrings(), Actions: a.reg.ActionsFor(scope)}
}

func (a *console) GetPolicy(scope string) (map[string]string, error) {
	return a.store.Policy(scope)
}

// SetPolicy 整体覆盖某 scope 的策略。校验:scope 已注册;风险等级合法;动作名在该 scope 登记的动作目录内
// (与运行时同源,挡在存库前)。动作名为空串视为"该等级不配"(从映射里剔除)。
func (a *console) SetPolicy(scope string, mapping map[string]string) error {
	if err := a.requireScope(scope); err != nil {
		return err
	}
	allowed := map[string]bool{}
	for _, t := range a.reg.ActionsFor(scope) {
		allowed[t] = true
	}
	clean := make(map[string]string, len(mapping))
	for level, action := range mapping {
		if !core.RiskLevel(level).Valid() {
			return fmt.Errorf("未知风险等级 %q", level)
		}
		if action == "" {
			continue // 空 = 该等级不配(回退业务默认)
		}
		if !allowed[action] {
			return fmt.Errorf("scope %q 不支持动作 %q", scope, action)
		}
		clean[level] = action
	}
	return a.store.setPolicy(scope, clean)
}

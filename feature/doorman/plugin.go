package doorman

import "encoding/json"

// Check 一条**已配置好**的条件判定:读 Context(含 Attempt 与 Store)→ 命中与否。
type Check func(*Context) bool

// Condition 一个条件「类别」插件。这是 doorman 唯一的扩展点:实现它 + 注册。
// 自带配置:Compile 把该类别的 JSON 参数解成自己的类型、校验、造出 Check;Fields 描述配置字段。
type Condition interface {
	Type() string                                  // 类别唯一标识,如 "ua_match"
	Compile(params json.RawMessage) (Check, error) // 解析 + 校验自己的参数 → 可执行 Check
	Fields() []Field                               // 自己的配置字段(配置页表单);无配置返回 nil
}

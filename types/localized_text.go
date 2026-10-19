package types

import (
	"database/sql/driver"
	"encoding/json"
)

// LocalizedText 是「语言代码 → 文案」的多语言映射,如 {"zh-CN":"客户","en":"Customers"}。
// 存 JSON 列用(实现 driver.Valuer + sql.Scanner,供 GORM);API 输出即 JSON 对象。
//
// 用 map 而非固定字段结构体:支持哪些语言、默认语言、回退策略都**不写死在代码里**——
// 加/调整语言只改数据(迁移/DB)与前端 i18n 配置,这个类型和后端代码零改动。
// 语言选择/回退由消费端(前端)按其 i18n 当前语言做,本类型不内置任何固定语言。
type LocalizedText map[string]string

// Value 实现 driver.Valuer:nil → NULL;非 nil(含空 map)→ JSON。
func (t LocalizedText) Value() (driver.Value, error) {
	if t == nil {
		return nil, nil
	}
	return json.Marshal(t)
}

// Scan 实现 sql.Scanner:NULL / 空 / "null" → nil;否则按 JSON 解析。
func (t *LocalizedText) Scan(value interface{}) error {
	if value == nil {
		*t = nil
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		marshaled, err := json.Marshal(v)
		if err != nil {
			return err
		}
		bytes = marshaled
	}
	if len(bytes) == 0 || string(bytes) == "null" {
		*t = nil
		return nil
	}
	return json.Unmarshal(bytes, t)
}

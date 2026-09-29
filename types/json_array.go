package types

import (
	"database/sql/driver"
	"encoding/json"
)

// JSONArray is a generic type for JSON array of any type
// It implements driver.Valuer and sql.Scanner interfaces for GORM compatibility
type JSONArray[T any] []T

// Value implements the driver.Valuer interface
func (a JSONArray[T]) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	return json.Marshal(a)
}

// Scan implements the sql.Scanner interface
func (a *JSONArray[T]) Scan(value interface{}) error {
	if value == nil {
		*a = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return json.Unmarshal([]byte(value.(string)), a)
	}
	return json.Unmarshal(bytes, a)
}

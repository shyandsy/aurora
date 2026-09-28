package types

import (
	"database/sql/driver"
	"encoding/json"
)

// JSONObject is a type for JSON object (map[string]interface{})
// It implements driver.Valuer and sql.Scanner interfaces for GORM compatibility
type JSONObject map[string]interface{}

// Value implements the driver.Valuer interface
func (j JSONObject) Value() (driver.Value, error) {
	// Explicitly check for nil first
	// If j is nil, return nil to write NULL to database
	// If j is an empty map (len == 0), still marshal it as {} (not NULL)
	if j == nil {
		return nil, nil
	}
	// If j is an empty map, marshal it as {} (not NULL)
	// This allows distinguishing between NULL and empty object {}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface
func (j *JSONObject) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		// Try to marshal and unmarshal if it's not a known type
		marshaled, err := json.Marshal(v)
		if err != nil {
			return err
		}
		bytes = marshaled
	}

	// Handle empty bytes or JSON string "null"
	if len(bytes) == 0 {
		*j = nil
		return nil
	}

	// Check if it's the JSON string "null"
	if string(bytes) == "null" {
		*j = nil
		return nil
	}

	return json.Unmarshal(bytes, j)
}

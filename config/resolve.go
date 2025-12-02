package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func ResolveConfig(v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr {
		return fmt.Errorf("ResolveConfig: expected pointer, got %v", rv.Kind())
	}

	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("ResolveConfig: expected struct, got %v", rv.Kind())
	}

	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		fieldType := rt.Field(i)

		if !field.CanSet() {
			continue
		}

		envTag := fieldType.Tag.Get("env")
		if envTag == "" {
			continue
		}

		parts := strings.Split(envTag, ",")
		envKey := strings.TrimSpace(parts[0])
		hasOmitEmpty := len(parts) > 1 && strings.TrimSpace(parts[1]) == "omitempty"

		envValue := os.Getenv(envKey)

		if envValue == "" {
			if hasOmitEmpty {
				continue
			}
			return fmt.Errorf("ResolveConfig: required environment variable %s is not set (field: %s)", envKey, fieldType.Name)
		}
		if err := setFieldValue(field, envValue, fieldType.Name); err != nil {
			return fmt.Errorf("ResolveConfig: failed to set field %s: %w", fieldType.Name, err)
		}
	}

	return nil
}

func setFieldValue(field reflect.Value, value string, fieldName string) error {
	if !field.CanSet() {
		return fmt.Errorf("field %s cannot be set", fieldName)
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(value)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if field.Type() == reflect.TypeOf(time.Duration(0)) {
			duration, err := time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("invalid duration format: %w", err)
			}
			field.SetInt(int64(duration))
		} else {
			intValue, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid integer format: %w", err)
			}
			if field.OverflowInt(intValue) {
				return fmt.Errorf("integer overflow for %s", fieldName)
			}
			field.SetInt(intValue)
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintValue, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid unsigned integer format: %w", err)
		}
		if field.OverflowUint(uintValue) {
			return fmt.Errorf("unsigned integer overflow for %s", fieldName)
		}
		field.SetUint(uintValue)

	case reflect.Float32, reflect.Float64:
		floatValue, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid float format: %w", err)
		}
		if field.OverflowFloat(floatValue) {
			return fmt.Errorf("float overflow for %s", fieldName)
		}
		field.SetFloat(floatValue)

	case reflect.Bool:
		boolValue, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean format: %w", err)
		}
		field.SetBool(boolValue)

	case reflect.Slice:
		if field.Type().Elem().Kind() == reflect.String {
			values := strings.Split(value, ",")
			slice := reflect.MakeSlice(field.Type(), 0, len(values))
			for _, v := range values {
				trimmed := strings.TrimSpace(v)
				if trimmed != "" {
					slice = reflect.Append(slice, reflect.ValueOf(trimmed))
				}
			}
			field.Set(slice)
		} else {
			return fmt.Errorf("unsupported slice element type: %v", field.Type().Elem().Kind())
		}

	default:
		return fmt.Errorf("unsupported field type: %v", field.Kind())
	}

	return nil
}

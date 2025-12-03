package config

import (
	"os"
	"testing"
	"time"
)

// TestResolveConfig_BasicTypes tests basic type field assignment
func TestResolveConfig_BasicTypes(t *testing.T) {
	type TestConfig struct {
		StringField string  `env:"TEST_STRING"`
		IntField    int     `env:"TEST_INT"`
		UintField   uint    `env:"TEST_UINT"`
		FloatField  float64 `env:"TEST_FLOAT"`
		BoolField   bool    `env:"TEST_BOOL"`
	}

	os.Setenv("TEST_STRING", "test_value")
	os.Setenv("TEST_INT", "42")
	os.Setenv("TEST_UINT", "100")
	os.Setenv("TEST_FLOAT", "3.14")
	os.Setenv("TEST_BOOL", "true")
	defer func() {
		os.Unsetenv("TEST_STRING")
		os.Unsetenv("TEST_INT")
		os.Unsetenv("TEST_UINT")
		os.Unsetenv("TEST_FLOAT")
		os.Unsetenv("TEST_BOOL")
	}()

	cfg := &TestConfig{}
	err := ResolveConfig(cfg)
	if err != nil {
		t.Fatalf("ResolveConfig failed: %v", err)
	}

	if cfg.StringField != "test_value" {
		t.Errorf("StringField = %v, want test_value", cfg.StringField)
	}
	if cfg.IntField != 42 {
		t.Errorf("IntField = %v, want 42", cfg.IntField)
	}
	if cfg.UintField != 100 {
		t.Errorf("UintField = %v, want 100", cfg.UintField)
	}
	if cfg.FloatField != 3.14 {
		t.Errorf("FloatField = %v, want 3.14", cfg.FloatField)
	}
	if cfg.BoolField != true {
		t.Errorf("BoolField = %v, want true", cfg.BoolField)
	}
}

// TestResolveConfig_TypeConversion tests type conversion
func TestResolveConfig_TypeConversion(t *testing.T) {
	type TestConfig struct {
		Int8Field    int8    `env:"TEST_INT8"`
		Int16Field   int16   `env:"TEST_INT16"`
		Int32Field   int32   `env:"TEST_INT32"`
		Int64Field   int64   `env:"TEST_INT64"`
		Uint8Field   uint8   `env:"TEST_UINT8"`
		Uint16Field  uint16  `env:"TEST_UINT16"`
		Uint32Field  uint32  `env:"TEST_UINT32"`
		Uint64Field  uint64  `env:"TEST_UINT64"`
		Float32Field float32 `env:"TEST_FLOAT32"`
	}

	os.Setenv("TEST_INT8", "127")
	os.Setenv("TEST_INT16", "32767")
	os.Setenv("TEST_INT32", "2147483647")
	os.Setenv("TEST_INT64", "9223372036854775807")
	os.Setenv("TEST_UINT8", "255")
	os.Setenv("TEST_UINT16", "65535")
	os.Setenv("TEST_UINT32", "4294967295")
	os.Setenv("TEST_UINT64", "18446744073709551615")
	os.Setenv("TEST_FLOAT32", "3.14159")
	defer func() {
		os.Unsetenv("TEST_INT8")
		os.Unsetenv("TEST_INT16")
		os.Unsetenv("TEST_INT32")
		os.Unsetenv("TEST_INT64")
		os.Unsetenv("TEST_UINT8")
		os.Unsetenv("TEST_UINT16")
		os.Unsetenv("TEST_UINT32")
		os.Unsetenv("TEST_UINT64")
		os.Unsetenv("TEST_FLOAT32")
	}()

	cfg := &TestConfig{}
	err := ResolveConfig(cfg)
	if err != nil {
		t.Fatalf("ResolveConfig failed: %v", err)
	}

	if cfg.Int8Field != 127 {
		t.Errorf("Int8Field = %v, want 127", cfg.Int8Field)
	}
	if cfg.Uint8Field != 255 {
		t.Errorf("Uint8Field = %v, want 255", cfg.Uint8Field)
	}
}

// TestResolveConfig_Duration tests time.Duration type
func TestResolveConfig_Duration(t *testing.T) {
	type TestConfig struct {
		DurationField time.Duration `env:"TEST_DURATION"`
	}

	os.Setenv("TEST_DURATION", "1h30m")
	defer os.Unsetenv("TEST_DURATION")

	cfg := &TestConfig{}
	err := ResolveConfig(cfg)
	if err != nil {
		t.Fatalf("ResolveConfig failed: %v", err)
	}

	expected := 90 * time.Minute
	if cfg.DurationField != expected {
		t.Errorf("DurationField = %v, want %v", cfg.DurationField, expected)
	}
}

// TestResolveConfig_Slice tests slice type
func TestResolveConfig_Slice(t *testing.T) {
	type TestConfig struct {
		StringSlice []string `env:"TEST_SLICE"`
	}

	os.Setenv("TEST_SLICE", "a,b,c,d")
	defer os.Unsetenv("TEST_SLICE")

	cfg := &TestConfig{}
	err := ResolveConfig(cfg)
	if err != nil {
		t.Fatalf("ResolveConfig failed: %v", err)
	}

	expected := []string{"a", "b", "c", "d"}
	if len(cfg.StringSlice) != len(expected) {
		t.Fatalf("StringSlice length = %d, want %d", len(cfg.StringSlice), len(expected))
	}
	for i, v := range expected {
		if cfg.StringSlice[i] != v {
			t.Errorf("StringSlice[%d] = %v, want %v", i, cfg.StringSlice[i], v)
		}
	}
}

// TestResolveConfig_SliceWithSpaces tests slice with spaces
func TestResolveConfig_SliceWithSpaces(t *testing.T) {
	type TestConfig struct {
		StringSlice []string `env:"TEST_SLICE"`
	}

	os.Setenv("TEST_SLICE", "a, b , c ,d")
	defer os.Unsetenv("TEST_SLICE")

	cfg := &TestConfig{}
	err := ResolveConfig(cfg)
	if err != nil {
		t.Fatalf("ResolveConfig failed: %v", err)
	}

	expected := []string{"a", "b", "c", "d"}
	if len(cfg.StringSlice) != len(expected) {
		t.Fatalf("StringSlice length = %d, want %d", len(cfg.StringSlice), len(expected))
	}
}

// TestResolveConfig_RequiredFieldMissing tests error when required field is missing
func TestResolveConfig_RequiredFieldMissing(t *testing.T) {
	type TestConfig struct {
		RequiredField string `env:"TEST_REQUIRED"`
	}

	os.Unsetenv("TEST_REQUIRED")

	cfg := &TestConfig{}
	err := ResolveConfig(cfg)
	if err == nil {
		t.Fatal("ResolveConfig should fail when required field is missing")
	}

	if err.Error() == "" {
		t.Error("Error message should not be empty")
	}
}

// TestResolveConfig_OmitEmpty tests omitempty tag
func TestResolveConfig_OmitEmpty(t *testing.T) {
	type TestConfig struct {
		RequiredField string `env:"TEST_REQUIRED"`
		OptionalField string `env:"TEST_OPTIONAL,omitempty"`
		OptionalInt   int    `env:"TEST_OPTIONAL_INT,omitempty"`
	}

	os.Setenv("TEST_REQUIRED", "required_value")
	defer os.Unsetenv("TEST_REQUIRED")

	cfg := &TestConfig{}
	err := ResolveConfig(cfg)
	if err != nil {
		t.Fatalf("ResolveConfig failed: %v", err)
	}

	if cfg.RequiredField != "required_value" {
		t.Errorf("RequiredField = %v, want required_value", cfg.RequiredField)
	}
	if cfg.OptionalField != "" {
		t.Errorf("OptionalField = %v, want empty string (zero value)", cfg.OptionalField)
	}
	if cfg.OptionalInt != 0 {
		t.Errorf("OptionalInt = %v, want 0 (zero value)", cfg.OptionalInt)
	}
}

// TestResolveConfig_InvalidPointer tests non-pointer parameter
func TestResolveConfig_InvalidPointer(t *testing.T) {
	type TestConfig struct {
		Field string `env:"TEST_FIELD"`
	}

	cfg := TestConfig{}
	err := ResolveConfig(cfg)
	if err == nil {
		t.Fatal("ResolveConfig should fail when parameter is not a pointer")
	}
}

// TestResolveConfig_InvalidType tests non-struct parameter
func TestResolveConfig_InvalidType(t *testing.T) {
	var str string
	err := ResolveConfig(&str)
	if err == nil {
		t.Fatal("ResolveConfig should fail when parameter is not a struct")
	}
}

// TestResolveConfig_TypeConversionError tests type conversion failure
func TestResolveConfig_TypeConversionError(t *testing.T) {
	type TestConfig struct {
		IntField int `env:"TEST_INT"`
	}

	os.Setenv("TEST_INT", "not_a_number")
	defer os.Unsetenv("TEST_INT")

	cfg := &TestConfig{}
	err := ResolveConfig(cfg)
	if err == nil {
		t.Fatal("ResolveConfig should fail when type conversion fails")
	}
}

// TestResolveConfig_DurationConversionError tests Duration conversion failure
func TestResolveConfig_DurationConversionError(t *testing.T) {
	type TestConfig struct {
		DurationField time.Duration `env:"TEST_DURATION"`
	}

	os.Setenv("TEST_DURATION", "invalid_duration")
	defer os.Unsetenv("TEST_DURATION")

	cfg := &TestConfig{}
	err := ResolveConfig(cfg)
	if err == nil {
		t.Fatal("ResolveConfig should fail when duration conversion fails")
	}
}

// TestResolveConfig_BoolConversion tests boolean conversion
func TestResolveConfig_BoolConversion(t *testing.T) {
	type TestConfig struct {
		BoolTrue  bool `env:"TEST_BOOL_TRUE"`
		BoolFalse bool `env:"TEST_BOOL_FALSE"`
		Bool1     bool `env:"TEST_BOOL_1"`
		Bool0     bool `env:"TEST_BOOL_0"`
	}

	os.Setenv("TEST_BOOL_TRUE", "true")
	os.Setenv("TEST_BOOL_FALSE", "false")
	os.Setenv("TEST_BOOL_1", "1")
	os.Setenv("TEST_BOOL_0", "0")
	defer func() {
		os.Unsetenv("TEST_BOOL_TRUE")
		os.Unsetenv("TEST_BOOL_FALSE")
		os.Unsetenv("TEST_BOOL_1")
		os.Unsetenv("TEST_BOOL_0")
	}()

	cfg := &TestConfig{}
	err := ResolveConfig(cfg)
	if err != nil {
		t.Fatalf("ResolveConfig failed: %v", err)
	}

	if cfg.BoolTrue != true {
		t.Errorf("BoolTrue = %v, want true", cfg.BoolTrue)
	}
	if cfg.BoolFalse != false {
		t.Errorf("BoolFalse = %v, want false", cfg.BoolFalse)
	}
	if cfg.Bool1 != true {
		t.Errorf("Bool1 = %v, want true", cfg.Bool1)
	}
	if cfg.Bool0 != false {
		t.Errorf("Bool0 = %v, want false", cfg.Bool0)
	}
}

// TestResolveConfig_NoEnvTag tests field without env tag
func TestResolveConfig_NoEnvTag(t *testing.T) {
	type TestConfig struct {
		WithTag    string `env:"TEST_WITH_TAG"`
		WithoutTag string
	}

	os.Setenv("TEST_WITH_TAG", "value")
	defer os.Unsetenv("TEST_WITH_TAG")

	cfg := &TestConfig{}
	cfg.WithoutTag = "original_value"

	err := ResolveConfig(cfg)
	if err != nil {
		t.Fatalf("ResolveConfig failed: %v", err)
	}

	if cfg.WithTag != "value" {
		t.Errorf("WithTag = %v, want value", cfg.WithTag)
	}
	if cfg.WithoutTag != "original_value" {
		t.Errorf("WithoutTag should remain unchanged, got %v", cfg.WithoutTag)
	}
}

// TestResolveConfig_EmptyString tests empty string handling
func TestResolveConfig_EmptyString(t *testing.T) {
	type TestConfig struct {
		RequiredField string `env:"TEST_REQUIRED"`
		OptionalField string `env:"TEST_OPTIONAL,omitempty"`
	}

	os.Setenv("TEST_REQUIRED", "")
	os.Setenv("TEST_OPTIONAL", "")
	defer func() {
		os.Unsetenv("TEST_REQUIRED")
		os.Unsetenv("TEST_OPTIONAL")
	}()

	cfg := &TestConfig{}
	err := ResolveConfig(cfg)
	if err == nil {
		t.Fatal("ResolveConfig should fail when required field is empty string")
	}

	os.Unsetenv("TEST_REQUIRED")
	os.Unsetenv("TEST_OPTIONAL")
	cfg2 := &TestConfig{}
	err2 := ResolveConfig(cfg2)
	if err2 == nil {
		t.Fatal("ResolveConfig should fail when required field is not set")
	}
}

// TestResolveConfig_Overflow tests numeric overflow
func TestResolveConfig_Overflow(t *testing.T) {
	type TestConfig struct {
		Int8Field int8 `env:"TEST_INT8"`
	}

	os.Setenv("TEST_INT8", "1000")
	defer os.Unsetenv("TEST_INT8")

	cfg := &TestConfig{}
	err := ResolveConfig(cfg)
	if err == nil {
		t.Fatal("ResolveConfig should fail when value overflows")
	}
}

// TestResolveConfig_UnsupportedSliceType tests unsupported slice type
func TestResolveConfig_UnsupportedSliceType(t *testing.T) {
	type TestConfig struct {
		IntSlice []int `env:"TEST_INT_SLICE"`
	}

	os.Setenv("TEST_INT_SLICE", "1,2,3")
	defer os.Unsetenv("TEST_INT_SLICE")

	cfg := &TestConfig{}
	err := ResolveConfig(cfg)
	if err == nil {
		t.Fatal("ResolveConfig should fail for unsupported slice element type")
	}
}

// TestResolveConfig_RealWorldExample tests real-world scenario: DatabaseConfig
func TestResolveConfig_RealWorldExample(t *testing.T) {
	os.Setenv("DB_DRIVER", "mysql")
	os.Setenv("DB_DSN", "user:pass@tcp(localhost:3306)/db")
	os.Setenv("DB_MAX_IDLE_CONNS", "10")
	os.Setenv("DB_MAX_OPEN_CONNS", "100")
	defer func() {
		os.Unsetenv("DB_DRIVER")
		os.Unsetenv("DB_DSN")
		os.Unsetenv("DB_MAX_IDLE_CONNS")
		os.Unsetenv("DB_MAX_OPEN_CONNS")
	}()

	cfg := &DatabaseConfig{}
	err := ResolveConfig(cfg)
	if err != nil {
		t.Fatalf("ResolveConfig failed: %v", err)
	}

	if cfg.Driver != "mysql" {
		t.Errorf("Driver = %v, want mysql", cfg.Driver)
	}
	if cfg.DSN != "user:pass@tcp(localhost:3306)/db" {
		t.Errorf("DSN = %v, want user:pass@tcp(localhost:3306)/db", cfg.DSN)
	}
	if cfg.MaxIdleConns != 10 {
		t.Errorf("MaxIdleConns = %v, want 10", cfg.MaxIdleConns)
	}
	if cfg.MaxOpenConns != 100 {
		t.Errorf("MaxOpenConns = %v, want 100", cfg.MaxOpenConns)
	}
}

// TestResolveConfig_RealWorldExampleRedis tests real-world scenario: RedisConfig
func TestResolveConfig_RealWorldExampleRedis(t *testing.T) {
	os.Setenv("REDIS_ADDR", "localhost:6379")
	os.Setenv("REDIS_PASSWORD", "secret")
	os.Setenv("REDIS_DB", "1")
	defer func() {
		os.Unsetenv("REDIS_ADDR")
		os.Unsetenv("REDIS_PASSWORD")
		os.Unsetenv("REDIS_DB")
	}()

	cfg := &RedisConfig{}
	err := ResolveConfig(cfg)
	if err != nil {
		t.Fatalf("ResolveConfig failed: %v", err)
	}

	if cfg.Addr != "localhost:6379" {
		t.Errorf("Addr = %v, want localhost:6379", cfg.Addr)
	}
	if cfg.Password != "secret" {
		t.Errorf("Password = %v, want secret", cfg.Password)
	}
	if cfg.DB != 1 {
		t.Errorf("DB = %v, want 1", cfg.DB)
	}
}

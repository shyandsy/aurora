package config

// MigrationConfig database migration configuration
type MigrationConfig struct {
	// TablePrefix is an optional prefix for goose table name
	// If set, goose version table will use this prefix
	// Example: if set to "admin_", the table name will be "admin_goose_db_version"
	// If not set or empty, goose default table name "goose_db_version" will be used (SetTableName will not be called)
	// Configure via environment variable GOOSE_TABLE_PREFIX
	TablePrefix string `env:"GOOSE_TABLE_PREFIX,omitempty"`
}

func (m *MigrationConfig) Key() string {
	return "migration"
}

func (m *MigrationConfig) Validate() error {
	// Table prefix can be empty, no validation needed
	return nil
}

// GetTableName returns the goose version table for the DATA stream (项目业务迁移)。
// TablePrefix 为空即 "goose_db_version"(空串拼接天然等价,无需分支)。
func (m *MigrationConfig) GetTableName() string {
	return m.TablePrefix + "goose_db_version"
}

// GetSchemaTableName returns the goose version table for the SCHEMA stream
// (共享模块自带的建表/改表结构迁移,先于 data 套执行、独立版本表)。
func (m *MigrationConfig) GetSchemaTableName() string {
	return m.TablePrefix + "schema_goose_db_version"
}

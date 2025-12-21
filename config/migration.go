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

// GetTableName returns the full goose table name
// If TablePrefix is empty, returns default table name "goose_db_version"
// If TablePrefix is not empty, returns "{prefix}goose_db_version"
func (m *MigrationConfig) GetTableName() string {
	if m.TablePrefix == "" {
		return "goose_db_version"
	}
	return m.TablePrefix + "goose_db_version"
}

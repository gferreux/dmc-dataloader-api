package model

import "time"

// AppConfig is the process configuration loaded from the config file and DMC_ env vars.
type AppConfig struct {
	AppName    string `mapstructure:"-" validate:"required"`
	AppVersion string `mapstructure:"-" validate:"required"`
	BuildDate  string `mapstructure:"-" validate:"required"`

	Log           LogConfig                `mapstructure:"log"           validate:"required"`
	Server        ServerConfig             `mapstructure:"server"        validate:"required"`
	Firestore     FirestoreConfig          `mapstructure:"firestore"     validate:"required"`
	Organizations OrganizationSourceConfig `mapstructure:"organizations" validate:"required"`
	Derive        DeriveConfig             `mapstructure:"derive"        validate:"required"`
	Auth          AuthConfig               `mapstructure:"auth"          validate:"required"`
	SFTPGo        SFTPGoConfig             `mapstructure:"sftpgo"`
}

// LogConfig controls slog output.
type LogConfig struct {
	Level string `mapstructure:"level" validate:"required,oneof=debug info warn error"`
	JSON  bool   `mapstructure:"json"`
}

// ServerConfig controls the HTTP server.
type ServerConfig struct {
	Addr            string        `mapstructure:"addr"             validate:"required"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"     validate:"required"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"    validate:"required"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout" validate:"required"`
	CORSOrigins     string        `mapstructure:"cors_origins"`
}

// FirestoreConfig selects the named database that stores load_config documents.
type FirestoreConfig struct {
	ProjectID  string `mapstructure:"project_id"  validate:"required"`
	DatabaseID string `mapstructure:"database_id" validate:"required"`
	Collection string `mapstructure:"collection"  validate:"required"`
}

// OrganizationSourceConfig selects the Firestore database that stores organizations
// and advertiser accounts. It is separate from the load_config database because the
// project and database are not confirmed to be the same.
type OrganizationSourceConfig struct {
	ProjectID          string `mapstructure:"project_id"          validate:"required"`
	DatabaseID         string `mapstructure:"database_id"         validate:"required"`
	Collection         string `mapstructure:"collection"          validate:"required"`
	AccountsCollection string `mapstructure:"accounts_collection" validate:"required"`
}

// AuthConfig selects how callers are identified.
// Mode "none" disables IAP checks for local development.
// Mode "iap" verifies the X-Goog-IAP-JWT-Assertion header against IAPAudience.
// IAPAudience is required when mode is iap. It is the audience of the Cloud Run
// IAP backend, typically /projects/PROJECT_NUMBER/global/backendServices/SERVICE_ID.
type AuthConfig struct {
	Mode        string `mapstructure:"mode"         validate:"required,oneof=none iap"`
	IAPAudience string `mapstructure:"iap_audience"`
}

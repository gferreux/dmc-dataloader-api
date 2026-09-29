package model

import "time"

// AppConfig is the process configuration loaded from the config file and DMC_ env vars.
type AppConfig struct {
	AppName    string `mapstructure:"-" validate:"required"`
	AppVersion string `mapstructure:"-" validate:"required"`
	BuildDate  string `mapstructure:"-" validate:"required"`

	Log       LogConfig       `mapstructure:"log"       validate:"required"`
	Server    ServerConfig    `mapstructure:"server"    validate:"required"`
	Firestore FirestoreConfig `mapstructure:"firestore" validate:"required"`
	Auth      AuthConfig      `mapstructure:"auth"      validate:"required"`
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

// AuthConfig selects the pluggable authenticator.
// Mode "none" trusts the platform (IAP or Cloud Run IAM) to block anonymous callers.
// Mode "iap" requires the X-Goog-IAP-JWT-Assertion header and, when IAPAudience is set,
// verifies that JWT.
type AuthConfig struct {
	Mode        string `mapstructure:"mode"         validate:"required,oneof=none iap"`
	IAPAudience string `mapstructure:"iap_audience"`
}

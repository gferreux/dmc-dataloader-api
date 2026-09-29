package model

import "time"

type AppConfig struct {
	AppName    string `mapstructure:"-" validate:"required"`
	AppVersion string `mapstructure:"-" validate:"required"`
	BuildDate  string `mapstructure:"-" validate:"required"`

	Log    LogConfig    `mapstructure:"log"    validate:"required"`
	Server ServerConfig `mapstructure:"server" validate:"required"`
}

type LogConfig struct {
	Level string `mapstructure:"level" validate:"required,oneof=debug info warn error"`
	JSON  bool   `mapstructure:"json"`
}

type ServerConfig struct {
	Addr            string        `mapstructure:"addr"             validate:"required"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"     validate:"required"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout" validate:"required"`
}

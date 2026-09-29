package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
)

// Build-time variables, injected via -ldflags (see Dockerfile).
var (
	AppName    = "dmc-dataloader-api"
	AppVersion = "dev"
	BuildDate  = "unknown"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "config.yaml", "path to the configuration file")
	flag.Parse()

	appConfig, err := loadConfig(*configPath)
	if err != nil {
		return err
	}

	setupLogger(appConfig.Log)

	slog.Info("Starting application",
		"appName", appConfig.AppName,
		"appVersion", appConfig.AppVersion,
		"buildDate", appConfig.BuildDate,
	)

	return InitializeApp(appConfig).Run(context.Background())
}

func loadConfig(path string) (model.AppConfig, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetEnvPrefix("DMC")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return model.AppConfig{}, fmt.Errorf("reading config file %q: %w", path, err)
	}

	var appConfig model.AppConfig
	if err := v.Unmarshal(&appConfig); err != nil {
		return model.AppConfig{}, fmt.Errorf("unmarshalling config: %w", err)
	}

	appConfig.AppName = AppName
	appConfig.AppVersion = AppVersion
	appConfig.BuildDate = BuildDate

	if err := validator.New().Struct(appConfig); err != nil {
		return model.AppConfig{}, fmt.Errorf("validating config: %w", err)
	}

	return appConfig, nil
}

func setupLogger(cfg model.LogConfig) {
	var level slog.Level

	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if cfg.JSON {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}

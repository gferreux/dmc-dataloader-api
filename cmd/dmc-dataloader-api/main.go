package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
)

// Build-time variables, injected via -ldflags (see Dockerfile).
//
//nolint:gochecknoglobals // -X ldflags can only set package-level variables
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

	application, cleanup, err := InitializeApp(context.Background(), appConfig)
	if err != nil {
		return err
	}

	defer cleanup()

	slog.Info("Firestore target",
		"project", appConfig.Firestore.ProjectID,
		"database", appConfig.Firestore.DatabaseID,
		"collection", appConfig.Firestore.Collection,
		"auth", appConfig.Auth.Mode,
	)
	slog.Info("Organization source",
		"project", appConfig.Organizations.ProjectID,
		"database", appConfig.Organizations.DatabaseID,
		"organizations", appConfig.Organizations.Collection,
		"accounts", appConfig.Organizations.AccountsCollection,
	)

	return application.Run(context.Background())
}

func setDefaults(viperConfig *viper.Viper) {
	derive := model.DevDeriveConfig()
	orgs := model.DevOrganizationSource()

	viperConfig.SetDefault("log.level", "info")
	viperConfig.SetDefault("log.json", true)
	viperConfig.SetDefault("server.addr", ":3000")
	viperConfig.SetDefault("server.read_timeout", 5*time.Second)
	viperConfig.SetDefault("server.write_timeout", 15*time.Second)
	viperConfig.SetDefault("server.shutdown_timeout", 10*time.Second)
	viperConfig.SetDefault("server.cors_origins", "")
	viperConfig.SetDefault("firestore.project_id", "dmc-datastores-dev-becb")
	viperConfig.SetDefault("firestore.database_id", "dmc-data-loader-dev")
	viperConfig.SetDefault("firestore.collection", "load_config")
	viperConfig.SetDefault("organizations.project_id", orgs.ProjectID)
	viperConfig.SetDefault("organizations.database_id", orgs.DatabaseID)
	viperConfig.SetDefault("organizations.collection", orgs.Collection)
	viperConfig.SetDefault("organizations.accounts_collection", orgs.AccountsCollection)
	setDeriveDefaults(viperConfig, "derive.advertiser", derive.Advertiser)
	setDeriveDefaults(viperConfig, "derive.publisher", derive.Publisher)
	viperConfig.SetDefault("auth.mode", "none")
	viperConfig.SetDefault("auth.iap_audience", "")
}

func setDeriveDefaults(viperConfig *viper.Viper, prefix string, partner model.PartnerDeriveConfig) {
	viperConfig.SetDefault(prefix+".raw_bucket", partner.RawBucket)
	viperConfig.SetDefault(prefix+".staging_bucket", partner.StagingBucket)
	viperConfig.SetDefault(prefix+".notification.project_id", partner.Notification.ProjectID)
	viperConfig.SetDefault(prefix+".notification.topic_id", partner.Notification.TopicID)
	viperConfig.SetDefault(prefix+".destination.project_id", partner.Destination.ProjectID)
	viperConfig.SetDefault(prefix+".destination.dataset_id", partner.Destination.DatasetID)

	for fileType, table := range partner.Destination.Tables {
		viperConfig.SetDefault(prefix+".destination.tables."+fileType, table)
	}
}

func loadConfig(path string) (model.AppConfig, error) {
	viperConfig := viper.New()
	viperConfig.SetConfigFile(path)
	viperConfig.SetEnvPrefix("DMC")
	viperConfig.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viperConfig.AutomaticEnv()
	setDefaults(viperConfig)

	for _, key := range configEnvKeys() {
		if err := viperConfig.BindEnv(key); err != nil {
			return model.AppConfig{}, fmt.Errorf("binding env %s: %w", key, err)
		}
	}

	if err := viperConfig.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !os.IsNotExist(err) && !errors.As(err, &notFound) {
			return model.AppConfig{}, fmt.Errorf("reading config file %q: %w", path, err)
		}
	}

	var appConfig model.AppConfig
	if err := viperConfig.Unmarshal(&appConfig); err != nil {
		return model.AppConfig{}, fmt.Errorf("unmarshalling config: %w", err)
	}

	appConfig.Derive = appConfig.Derive.WithTableDefaults()

	appConfig.AppName = AppName
	appConfig.AppVersion = AppVersion
	appConfig.BuildDate = BuildDate

	if os.Getenv("DMC_SERVER_ADDR") == "" {
		if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
			appConfig.Server.Addr = ":" + port
		}
	}

	if err := validator.New().Struct(appConfig); err != nil {
		return model.AppConfig{}, fmt.Errorf("validating config: %w", err)
	}

	return appConfig, nil
}

func configEnvKeys() []string {
	return []string{
		"log.level",
		"log.json",
		"server.addr",
		"server.read_timeout",
		"server.write_timeout",
		"server.shutdown_timeout",
		"server.cors_origins",
		"firestore.project_id",
		"firestore.database_id",
		"firestore.collection",
		"organizations.project_id",
		"organizations.database_id",
		"organizations.collection",
		"organizations.accounts_collection",
		"derive.advertiser.raw_bucket",
		"derive.advertiser.staging_bucket",
		"derive.advertiser.notification.project_id",
		"derive.advertiser.notification.topic_id",
		"derive.advertiser.destination.project_id",
		"derive.advertiser.destination.dataset_id",
		"derive.publisher.raw_bucket",
		"derive.publisher.staging_bucket",
		"derive.publisher.notification.project_id",
		"derive.publisher.notification.topic_id",
		"derive.publisher.destination.project_id",
		"derive.publisher.destination.dataset_id",
		"derive.publisher.destination.tables.optin",
		"derive.publisher.destination.tables.optout",
		"auth.mode",
		"auth.iap_audience",
	}
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

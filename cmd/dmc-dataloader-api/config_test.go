package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigEnvOverridesFileAndPort(t *testing.T) {
	t.Setenv("DMC_FIRESTORE_PROJECT_ID", "from-env")
	t.Setenv("DMC_FIRESTORE_DATABASE_ID", "named-db")
	t.Setenv("DMC_FIRESTORE_COLLECTION", "load_config")
	t.Setenv("DMC_AUTH_MODE", "none")
	t.Setenv("PORT", "8080")
	t.Setenv("DMC_SERVER_ADDR", "")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := []byte(`
log:
  level: debug
  json: false
server:
  addr: :3000
  read_timeout: 5s
  write_timeout: 15s
  shutdown_timeout: 10s
  cors_origins: http://localhost:4200
firestore:
  project_id: from-file
  database_id: from-file
  collection: from-file
auth:
  mode: none
`)
	require.NoError(t, os.WriteFile(path, body, 0o600))

	cfg, err := loadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "from-env", cfg.Firestore.ProjectID)
	assert.Equal(t, "named-db", cfg.Firestore.DatabaseID)
	assert.Equal(t, "load_config", cfg.Firestore.Collection)
	assert.Equal(t, ":8080", cfg.Server.Addr)
	assert.Equal(t, 5*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, "http://localhost:4200", cfg.Server.CORSOrigins)
}

func TestLoadConfigMissingFileUsesDefaults(t *testing.T) {
	for _, key := range []string{
		"DMC_FIRESTORE_PROJECT_ID",
		"DMC_FIRESTORE_DATABASE_ID",
		"DMC_FIRESTORE_COLLECTION",
		"DMC_AUTH_MODE",
		"DMC_SERVER_ADDR",
		"PORT",
	} {
		t.Setenv(key, "placeholder")
		require.NoError(t, os.Unsetenv(key))
	}

	cfg, err := loadConfig(filepath.Join(t.TempDir(), "missing.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "dmc-datastores-dev-becb", cfg.Firestore.ProjectID)
	assert.Equal(t, "dmc-data-loader-dev", cfg.Firestore.DatabaseID)
	assert.Equal(t, "load_config", cfg.Firestore.Collection)
	assert.Equal(t, ":3000", cfg.Server.Addr)
	assert.Equal(t, "none", cfg.Auth.Mode)
}

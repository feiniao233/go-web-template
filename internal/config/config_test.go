package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var configKeys = []string{
	"HTTP_ADDR", "HTTP_PREFIX", "HTTP_READ_HEADER_TIMEOUT", "HTTP_READ_TIMEOUT", "HTTP_WRITE_TIMEOUT",
	"HTTP_IDLE_TIMEOUT", "HTTP_MAX_HEADER_BYTES", "GIN_MODE", "DATABASE_DSN", "DB_MAX_OPEN_CONNS", "DB_MAX_IDLE_CONNS",
	"DB_CONN_MAX_LIFETIME", "DB_SLOW_THRESHOLD", "DB_LOG_LEVEL", "MIGRATE_ON_START", "CLICKHOUSE_DSN", "TDENGINE_DSN",
	"TELEMETRY_MAX_OPEN_CONNS", "TELEMETRY_MAX_IDLE_CONNS", "TELEMETRY_CONN_MAX_LIFETIME", "REDIS_URL", "CORS_ALLOWED_ORIGINS",
	"CORS_ALLOW_CREDENTIALS", "SHUTDOWN_TIMEOUT", "LOG_LEVEL", "LOG_FILE",
	"MQTT_URL", "MQTT_CLIENT_ID", "MQTT_USERNAME", "MQTT_PASSWORD", "MQTT_CONNECT_TIMEOUT", "MQTT_KEEP_ALIVE",
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range configKeys {
		old, exists := os.LookupEnv(key)
		require.NoError(t, os.Unsetenv(key))
		t.Cleanup(func() {
			if exists {
				_ = os.Setenv(key, old)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}
}

func TestLoadPrecedenceAndDefaults(t *testing.T) {
	t.Chdir(t.TempDir())
	clearConfigEnv(t)
	require.NoError(t, os.WriteFile("config.yaml", []byte("http:\n  addr: yaml:8080\n  prefix: api/v2/\ndatabase:\n  dsn: yaml-db\nredis:\n  url: yaml-redis\nshutdown_timeout: 2s\n"), 0o600))
	require.NoError(t, os.Setenv("HTTP_ADDR", "env:8080"))
	require.NoError(t, os.Setenv("DATABASE_DSN", "env-db"))
	require.NoError(t, os.Setenv("SHUTDOWN_TIMEOUT", "3s"))
	require.NoError(t, os.Setenv("CORS_ALLOWED_ORIGINS", "https://a.example,https://b.example"))

	c, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, "env:8080", c.HTTP.Addr)
	assert.Equal(t, "/api/v2", c.HTTP.Prefix)
	assert.Equal(t, "env-db", c.Database.DSN)
	assert.Equal(t, "yaml-redis", c.Redis.URL)
	assert.Equal(t, 3*time.Second, c.ShutdownTimeout)
	assert.Equal(t, []string{"https://a.example", "https://b.example"}, c.CORS.AllowedOrigins)
	assert.Equal(t, 25, c.Database.MaxOpenConns)
	assert.Equal(t, 200*time.Millisecond, c.Database.SlowThreshold)
	assert.Equal(t, "release", c.HTTP.GinMode)
	assert.Equal(t, 5*time.Second, c.HTTP.ReadHeaderTimeout)
	assert.Equal(t, 30*time.Second, c.HTTP.WriteTimeout)
	assert.Equal(t, "info", c.Log.Level)
	assert.Equal(t, "service", c.MQTT.ClientID)
	assert.Equal(t, 10*time.Second, c.MQTT.ConnectTimeout)
}

func TestLoadValidation(t *testing.T) {
	for _, test := range []struct {
		name  string
		key   string
		value string
	}{
		{"gin mode", "GIN_MODE", "bad"},
		{"pool sizes", "DB_MAX_IDLE_CONNS", "26"},
		{"duration", "DB_SLOW_THRESHOLD", "0s"},
		{"log level", "DB_LOG_LEVEL", "debug"},
		{"prefix", "HTTP_PREFIX", "api//v1"},
		{"HTTP settings", "HTTP_MAX_HEADER_BYTES", "0"},
		{"application log level", "LOG_LEVEL", "fatal"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			clearConfigEnv(t)
			t.Setenv(test.key, test.value)
			_, err := Load("")
			require.Error(t, err)
		})
	}

	t.Run("wildcard credentials", func(t *testing.T) {
		t.Chdir(t.TempDir())
		clearConfigEnv(t)
		t.Setenv("CORS_ALLOW_CREDENTIALS", "true")
		_, err := Load("")
		require.Error(t, err)
	})

	t.Run("MQTT client ID", func(t *testing.T) {
		t.Chdir(t.TempDir())
		clearConfigEnv(t)
		t.Setenv("MQTT_URL", "tcp://localhost:1883")
		t.Setenv("MQTT_CLIENT_ID", " ")
		_, err := Load("")
		require.Error(t, err)
	})
}

func TestLoadExplicitConfigFile(t *testing.T) {
	t.Chdir(t.TempDir())
	clearConfigEnv(t)
	filename := t.TempDir() + "/service.yaml"
	require.NoError(t, os.WriteFile(filename, []byte("http:\n  addr: \":9090\"\n  write_timeout: 0s\n"), 0o600))

	c, err := Load(filename)
	require.NoError(t, err)
	assert.Equal(t, ":9090", c.HTTP.Addr)
	assert.Zero(t, c.HTTP.WriteTimeout)

	_, err = Load(filename + ".missing")
	require.Error(t, err)
}

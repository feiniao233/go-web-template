package config

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

type Config struct {
	HTTP            HTTPConfig      `mapstructure:"http"`
	Database        DatabaseConfig  `mapstructure:"database"`
	Telemetry       TelemetryConfig `mapstructure:"telemetry"`
	Redis           RedisConfig     `mapstructure:"redis"`
	MQTT            MQTTConfig      `mapstructure:"mqtt"`
	CORS            CORSConfig      `mapstructure:"cors"`
	Log             LogConfig       `mapstructure:"log"`
	ShutdownTimeout time.Duration   `mapstructure:"shutdown_timeout"`
}

type HTTPConfig struct {
	Addr              string        `mapstructure:"addr"`
	Prefix            string        `mapstructure:"prefix"`
	ReadHeaderTimeout time.Duration `mapstructure:"read_header_timeout"`
	ReadTimeout       time.Duration `mapstructure:"read_timeout"`
	WriteTimeout      time.Duration `mapstructure:"write_timeout"`
	IdleTimeout       time.Duration `mapstructure:"idle_timeout"`
	MaxHeaderBytes    int           `mapstructure:"max_header_bytes"`
	GinMode           string        `mapstructure:"gin_mode"`
}

type DatabaseConfig struct {
	DSN             string        `mapstructure:"dsn"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	SlowThreshold   time.Duration `mapstructure:"slow_threshold"`
	LogLevel        string        `mapstructure:"log_level"`
	MigrateOnStart  bool          `mapstructure:"migrate_on_start"`
}

type TelemetryConfig struct {
	ClickHouseDSN   string        `mapstructure:"clickhouse_dsn"`
	TDengineDSN     string        `mapstructure:"tdengine_dsn"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

type RedisConfig struct {
	URL string `mapstructure:"url"`
}

type MQTTConfig struct {
	URL            string        `mapstructure:"url"`
	ClientID       string        `mapstructure:"client_id"`
	Username       string        `mapstructure:"username"`
	Password       string        `mapstructure:"password"`
	ConnectTimeout time.Duration `mapstructure:"connect_timeout"`
	KeepAlive      time.Duration `mapstructure:"keep_alive"`
}

type CORSConfig struct {
	AllowedOrigins   []string `mapstructure:"allowed_origins"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
}

type LogConfig struct {
	Level string `mapstructure:"level"`
	File  string `mapstructure:"file"`
}

func Load(configFile string) (Config, error) {
	v := viper.New()
	configFileRequired := configFile != ""
	if configFile == "" {
		configFile = "config.yaml"
	}
	v.SetConfigFile(configFile)
	settings := []struct {
		key          string
		env          string
		defaultValue any
	}{
		{"http.addr", "HTTP_ADDR", ":8080"},
		{"http.prefix", "HTTP_PREFIX", "/api/v1"},
		{"http.read_header_timeout", "HTTP_READ_HEADER_TIMEOUT", "5s"},
		{"http.read_timeout", "HTTP_READ_TIMEOUT", "15s"},
		{"http.write_timeout", "HTTP_WRITE_TIMEOUT", "30s"},
		{"http.idle_timeout", "HTTP_IDLE_TIMEOUT", "60s"},
		{"http.max_header_bytes", "HTTP_MAX_HEADER_BYTES", 1 << 20},
		{"http.gin_mode", "GIN_MODE", "release"},
		{"database.dsn", "DATABASE_DSN", ""},
		{"database.max_open_conns", "DB_MAX_OPEN_CONNS", 25},
		{"database.max_idle_conns", "DB_MAX_IDLE_CONNS", 5},
		{"database.conn_max_lifetime", "DB_CONN_MAX_LIFETIME", "30m"},
		{"database.slow_threshold", "DB_SLOW_THRESHOLD", "200ms"},
		{"database.log_level", "DB_LOG_LEVEL", "warn"},
		{"database.migrate_on_start", "MIGRATE_ON_START", true},
		{"telemetry.clickhouse_dsn", "CLICKHOUSE_DSN", ""},
		{"telemetry.tdengine_dsn", "TDENGINE_DSN", ""},
		{"telemetry.max_open_conns", "TELEMETRY_MAX_OPEN_CONNS", 10},
		{"telemetry.max_idle_conns", "TELEMETRY_MAX_IDLE_CONNS", 5},
		{"telemetry.conn_max_lifetime", "TELEMETRY_CONN_MAX_LIFETIME", "30m"},
		{"redis.url", "REDIS_URL", ""},
		{"mqtt.url", "MQTT_URL", ""},
		{"mqtt.client_id", "MQTT_CLIENT_ID", "service"},
		{"mqtt.username", "MQTT_USERNAME", ""},
		{"mqtt.password", "MQTT_PASSWORD", ""},
		{"mqtt.connect_timeout", "MQTT_CONNECT_TIMEOUT", "10s"},
		{"mqtt.keep_alive", "MQTT_KEEP_ALIVE", "30s"},
		{"cors.allowed_origins", "CORS_ALLOWED_ORIGINS", []string{"*"}},
		{"cors.allow_credentials", "CORS_ALLOW_CREDENTIALS", false},
		{"shutdown_timeout", "SHUTDOWN_TIMEOUT", "10s"},
		{"log.level", "LOG_LEVEL", "info"},
		{"log.file", "LOG_FILE", ""},
	}
	for _, setting := range settings {
		v.SetDefault(setting.key, setting.defaultValue)
		if err := v.BindEnv(setting.key, setting.env); err != nil {
			return Config{}, fmt.Errorf("bind %s: %w", setting.env, err)
		}
	}
	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if configFileRequired || (!errors.As(err, &notFound) && !errors.Is(err, os.ErrNotExist)) {
			return Config{}, fmt.Errorf("read %s: %w", configFile, err)
		}
	}
	var c Config
	if err := v.Unmarshal(&c, viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToSliceHookFunc(","),
		mapstructure.StringToTimeDurationHookFunc(),
	))); err != nil {
		return Config{}, fmt.Errorf("decode configuration: %w", err)
	}
	if err := c.validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func (c *Config) validate() error {
	c.HTTP.Prefix = strings.TrimSpace(c.HTTP.Prefix)
	if c.HTTP.Prefix == "/" {
		c.HTTP.Prefix = ""
	} else if c.HTTP.Prefix != "" {
		if !strings.HasPrefix(c.HTTP.Prefix, "/") {
			c.HTTP.Prefix = "/" + c.HTTP.Prefix
		}
		c.HTTP.Prefix = strings.TrimRight(c.HTTP.Prefix, "/")
		if strings.Contains(c.HTTP.Prefix, "//") || strings.ContainsAny(c.HTTP.Prefix, "?#:* \t\r\n") {
			return fmt.Errorf("HTTP_PREFIX must be a valid path prefix")
		}
	}
	if !slices.Contains([]string{"debug", "release", "test"}, c.HTTP.GinMode) {
		return fmt.Errorf("GIN_MODE must be debug, release, or test")
	}
	if c.ShutdownTimeout <= 0 || c.Database.ConnMaxLifetime <= 0 || c.Database.SlowThreshold <= 0 || c.Telemetry.ConnMaxLifetime <= 0 || c.MQTT.ConnectTimeout <= 0 || c.MQTT.KeepAlive <= 0 {
		return fmt.Errorf("duration settings must be positive")
	}
	if c.MQTT.URL != "" && strings.TrimSpace(c.MQTT.ClientID) == "" {
		return fmt.Errorf("MQTT_CLIENT_ID is required when MQTT_URL is set")
	}
	if c.HTTP.ReadHeaderTimeout <= 0 || c.HTTP.ReadTimeout < 0 || c.HTTP.WriteTimeout < 0 || c.HTTP.IdleTimeout < 0 || c.HTTP.MaxHeaderBytes <= 0 {
		return fmt.Errorf("HTTP server settings are invalid")
	}
	if c.Database.MaxOpenConns < 0 || c.Database.MaxIdleConns < 0 || (c.Database.MaxOpenConns > 0 && c.Database.MaxIdleConns > c.Database.MaxOpenConns) {
		return fmt.Errorf("database pool settings are invalid")
	}
	if c.Telemetry.MaxOpenConns < 0 || c.Telemetry.MaxIdleConns < 0 || (c.Telemetry.MaxOpenConns > 0 && c.Telemetry.MaxIdleConns > c.Telemetry.MaxOpenConns) {
		return fmt.Errorf("telemetry pool settings are invalid")
	}
	if !slices.Contains([]string{"silent", "error", "warn", "info"}, c.Database.LogLevel) {
		return fmt.Errorf("DB_LOG_LEVEL must be silent, error, warn, or info")
	}
	if !slices.Contains([]string{"trace", "debug", "info", "warn", "error"}, c.Log.Level) {
		return fmt.Errorf("LOG_LEVEL must be trace, debug, info, warn, or error")
	}
	if len(c.CORS.AllowedOrigins) == 0 {
		return fmt.Errorf("CORS_ALLOWED_ORIGINS cannot be empty")
	}
	for i := range c.CORS.AllowedOrigins {
		c.CORS.AllowedOrigins[i] = strings.TrimSpace(c.CORS.AllowedOrigins[i])
		if c.CORS.AllowedOrigins[i] == "" {
			return fmt.Errorf("CORS_ALLOWED_ORIGINS cannot contain empty values")
		}
	}
	if c.CORS.AllowCredentials && slices.Contains(c.CORS.AllowedOrigins, "*") {
		return fmt.Errorf("CORS credentials cannot be used with wildcard origins")
	}
	return nil
}

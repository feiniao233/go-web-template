package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMySQLClickHouseSelection(t *testing.T) {
	opts := options{module: "example.com/device-service", database: "mysql", telemetry: "clickhouse", redis: true, mqtt: true, redisStream: true, frontend: "embed"}
	require.NoError(t, validate(&opts))

	storage := renderStorage(opts)
	assert.Contains(t, storage, "database/mysql")
	assert.Contains(t, storage, "platform/clickhouse")
	assert.NotContains(t, storage, "database/postgres")
	assert.NotContains(t, storage, "platform/tdengine")
	assert.Contains(t, deletionList(opts), "internal/platform/database/postgres")
	assert.Contains(t, renderCompose(opts), "image: mysql:8.4")
	assert.Contains(t, renderCompose(opts), "clickhouse/clickhouse-server")
	assert.Contains(t, renderCompose(opts), "eclipse-mosquitto:2")
	assert.Contains(t, renderIntegrations(opts), "platform/mqtt")
	assert.NotContains(t, deletionList(opts), "internal/platform/redisstream")
	assert.NotContains(t, deletionList(opts), "internal/httpapi/frontend")
}

func TestRejectsInvalidSelection(t *testing.T) {
	err := validate(&options{module: "not a module", database: "oracle", telemetry: "none"})
	require.Error(t, err)
}

func TestRedisStreamRequiresRedis(t *testing.T) {
	err := validate(&options{module: "example.com/service", database: "none", telemetry: "none", redisStream: true})
	require.ErrorContains(t, err, "requires redis")
}

func TestFilterNestedYAML(t *testing.T) {
	yaml := "database:\n  dsn: x\ntelemetry:\n  clickhouse_dsn: x\n  tdengine_dsn: y\nredis:\n  url: x\nmqtt:\n  url: x\nlog:\n  level: info\n"
	filtered := filterYAMLExample(yaml, options{database: "mysql", telemetry: "clickhouse", redis: false, mqtt: false})
	assert.Contains(t, filtered, "database:")
	assert.Contains(t, filtered, "clickhouse_dsn:")
	assert.NotContains(t, filtered, "tdengine_dsn:")
	assert.NotContains(t, filtered, "redis:")
	assert.NotContains(t, filtered, "mqtt:")
	assert.Contains(t, filtered, "log:")
}

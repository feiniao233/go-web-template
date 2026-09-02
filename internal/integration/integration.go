package integration

import (
	"github.com/sirupsen/logrus"

	"go-web-template/internal/config"
	"go-web-template/internal/health"
	mqttclient "go-web-template/internal/platform/mqtt"
)

type Integrations struct {
	MQTT   *mqttclient.Client
	checks []health.Check
}

func Open(cfg config.Config, logger *logrus.Logger) (*Integrations, error) {
	client, err := mqttclient.Open(mqttclient.Options{
		URL: cfg.MQTT.URL, ClientID: cfg.MQTT.ClientID, Username: cfg.MQTT.Username, Password: cfg.MQTT.Password,
		ConnectTimeout: cfg.MQTT.ConnectTimeout, KeepAlive: cfg.MQTT.KeepAlive, Logger: logger,
	})
	if err != nil {
		return nil, err
	}
	result := &Integrations{MQTT: client}
	if client.Enabled() {
		result.checks = append(result.checks, health.Check{Name: "mqtt", Ping: client.Ping})
	}
	return result, nil
}

func (i *Integrations) Checks() []health.Check { return i.checks }
func (i *Integrations) Close() error           { return i.MQTT.Close() }

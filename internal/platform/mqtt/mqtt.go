package mqtt

import (
	"context"
	"errors"
	"fmt"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/sirupsen/logrus"
)

var ErrDisconnected = errors.New("mqtt is disconnected")

type Options struct {
	URL            string
	ClientID       string
	Username       string
	Password       string
	ConnectTimeout time.Duration
	KeepAlive      time.Duration
	Logger         *logrus.Logger
}

type Client struct {
	client  paho.Client
	timeout time.Duration
}

func Open(options Options) (*Client, error) {
	if options.URL == "" {
		return &Client{}, nil
	}
	clientOptions := paho.NewClientOptions().
		AddBroker(options.URL).
		SetClientID(options.ClientID).
		SetUsername(options.Username).
		SetPassword(options.Password).
		SetConnectTimeout(options.ConnectTimeout).
		SetKeepAlive(options.KeepAlive).
		SetAutoReconnect(true).
		SetResumeSubs(true)
	if options.Logger != nil {
		clientOptions.SetOnConnectHandler(func(paho.Client) { options.Logger.Info("mqtt connected") })
		clientOptions.SetConnectionLostHandler(func(_ paho.Client, err error) {
			options.Logger.WithError(err).Warn("mqtt connection lost")
		})
	}
	client := paho.NewClient(clientOptions)
	token := client.Connect()
	if !token.WaitTimeout(options.ConnectTimeout) {
		client.Disconnect(0)
		return nil, fmt.Errorf("connect MQTT: timeout after %s", options.ConnectTimeout)
	}
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("connect MQTT: %w", err)
	}
	return &Client{client: client, timeout: options.ConnectTimeout}, nil
}

func (c *Client) Enabled() bool { return c != nil && c.client != nil }

func (c *Client) Ping(context.Context) error {
	if !c.Enabled() || !c.client.IsConnectionOpen() {
		return ErrDisconnected
	}
	return nil
}

func (c *Client) Publish(topic string, qos byte, retained bool, payload []byte) error {
	if qos > 2 {
		return fmt.Errorf("invalid MQTT QoS %d", qos)
	}
	if topic == "" {
		return fmt.Errorf("MQTT topic is required")
	}
	if !c.Enabled() {
		return ErrDisconnected
	}
	token := c.client.Publish(topic, qos, retained, payload)
	if !token.WaitTimeout(c.timeout) {
		return fmt.Errorf("publish MQTT message: timeout after %s", c.timeout)
	}
	return token.Error()
}

func (c *Client) Subscribe(topic string, qos byte, handler paho.MessageHandler) error {
	if qos > 2 {
		return fmt.Errorf("invalid MQTT QoS %d", qos)
	}
	if topic == "" || handler == nil {
		return fmt.Errorf("MQTT topic and handler are required")
	}
	if !c.Enabled() {
		return ErrDisconnected
	}
	token := c.client.Subscribe(topic, qos, handler)
	if !token.WaitTimeout(c.timeout) {
		return fmt.Errorf("subscribe MQTT topic: timeout after %s", c.timeout)
	}
	return token.Error()
}

func (c *Client) Close() error {
	if c.Enabled() {
		c.client.Disconnect(250)
	}
	return nil
}

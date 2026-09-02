package mqtt

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMessageValidation(t *testing.T) {
	client := &Client{}
	require.ErrorContains(t, client.Publish("topic", 3, false, nil), "QoS")
	require.ErrorContains(t, client.Subscribe("", 0, nil), "topic")
}

package redisstream

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRejectsInvalidSettings(t *testing.T) {
	_, err := New(nil, "events", "workers", "worker-1", time.Second, 10)
	require.Error(t, err)
}

func TestHandlerErrorOption(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	defer client.Close()

	called := false
	consumer, err := New(client, "events", "workers", "worker-1", time.Second, 10, WithHandlerError(func(_ context.Context, message redis.XMessage, err error) {
		called = message.ID == "1-0" && errors.Is(err, context.Canceled)
	}))
	require.NoError(t, err)
	require.NotNil(t, consumer.onHandlerError)
	consumer.onHandlerError(context.Background(), redis.XMessage{ID: "1-0"}, context.Canceled)
	require.True(t, called)
}

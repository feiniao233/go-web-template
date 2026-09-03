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

func TestRetryConfiguration(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	defer client.Close()

	consumer, err := New(client, "events", "workers", "worker-1", time.Second, 10, WithRetry(30*time.Second, 5, ""))
	require.NoError(t, err)
	require.Equal(t, 30*time.Second, consumer.retryIdle)
	require.Equal(t, int64(5), consumer.maxDeliveries)
	require.Equal(t, "events:dlq", consumer.dlqStream)

	consumer, err = New(client, "events", "workers", "worker-1", time.Second, 10, WithRetry(30*time.Second, 5, "custom-dlq"))
	require.NoError(t, err)
	require.Equal(t, "custom-dlq", consumer.dlqStream)
}

func TestRejectsInvalidRetrySettings(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	defer client.Close()

	_, err := New(client, "events", "workers", "worker-1", time.Second, 10, WithRetry(30*time.Second, 0, ""))
	require.ErrorContains(t, err, "max deliveries")

	_, err = New(client, "events", "workers", "worker-1", time.Second, 10, WithRetry(-time.Second, 5, ""))
	require.ErrorContains(t, err, "retry settings")
}

func TestRejectMarksError(t *testing.T) {
	cause := errors.New("invalid payload")
	err := Reject(cause)
	require.ErrorIs(t, err, ErrReject)
	require.ErrorIs(t, err, cause)
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

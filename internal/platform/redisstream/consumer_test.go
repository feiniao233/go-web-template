package redisstream

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
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

func TestConsumerRuntime(t *testing.T) {
	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("REDIS_TEST_ADDR is not set")
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = client.Close() })
	require.NoError(t, client.Ping(t.Context()).Err())

	t.Run("retries to default DLQ", func(t *testing.T) {
		stream := fmt.Sprintf("redisstream:test:%d", time.Now().UnixNano())
		group := stream + ":group"
		t.Cleanup(func() { client.Del(context.Background(), stream, stream+":dlq") })

		consumer, err := New(client, stream, group, "worker", 20*time.Millisecond, 10, WithRetry(10*time.Millisecond, 3, ""))
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)
		done := make(chan error, 1)
		var calls atomic.Int64
		go func() {
			done <- consumer.Run(ctx, func(context.Context, redis.XMessage) error {
				calls.Add(1)
				return errors.New("failed")
			})
		}()
		messageID, err := client.XAdd(t.Context(), &redis.XAddArgs{Stream: stream, Values: map[string]any{"event": "test"}}).Result()
		require.NoError(t, err)

		require.Eventually(t, func() bool { return client.XLen(t.Context(), stream+":dlq").Val() == 1 }, 5*time.Second, 10*time.Millisecond)
		require.Eventually(t, func() bool {
			pending, err := client.XPending(t.Context(), stream, group).Result()
			return err == nil && pending.Count == 0
		}, time.Second, 10*time.Millisecond)
		dlq, err := client.XRangeN(t.Context(), stream+":dlq", "-", "+", 1).Result()
		require.NoError(t, err)
		require.Equal(t, messageID, dlq[0].Values["original_id"])
		require.Equal(t, "3", fmt.Sprint(dlq[0].Values["delivery_count"]))
		require.Equal(t, int64(3), calls.Load())

		cancel()
		requireConsumerStopped(t, done)
	})

	t.Run("recovers pending on startup", func(t *testing.T) {
		stream := fmt.Sprintf("redisstream:test:%d", time.Now().UnixNano())
		group := stream + ":group"
		t.Cleanup(func() { client.Del(context.Background(), stream) })
		require.NoError(t, client.XGroupCreateMkStream(t.Context(), stream, group, "0").Err())
		messageID, err := client.XAdd(t.Context(), &redis.XAddArgs{Stream: stream, Values: map[string]any{"event": "pending"}}).Result()
		require.NoError(t, err)
		_, err = client.XReadGroup(t.Context(), &redis.XReadGroupArgs{
			Group: group, Consumer: "worker", Streams: []string{stream, ">"}, Count: 1,
		}).Result()
		require.NoError(t, err)

		consumer, err := New(client, stream, group, "worker", 20*time.Millisecond, 10)
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)
		done := make(chan error, 1)
		handled := make(chan string, 1)
		go func() {
			done <- consumer.Run(ctx, func(_ context.Context, message redis.XMessage) error {
				handled <- message.ID
				return nil
			})
		}()
		select {
		case id := <-handled:
			require.Equal(t, messageID, id)
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for pending message")
		}
		require.Eventually(t, func() bool {
			pending, err := client.XPending(t.Context(), stream, group).Result()
			return err == nil && pending.Count == 0
		}, time.Second, 10*time.Millisecond)

		cancel()
		requireConsumerStopped(t, done)
	})
}

func requireConsumerStopped(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for consumer to stop")
	}
}

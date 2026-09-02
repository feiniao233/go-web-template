package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorkerErrorCancelsGroup(t *testing.T) {
	group := New()
	cancelled := make(chan struct{})
	group.Add("waiting", func(ctx context.Context) error {
		<-ctx.Done()
		close(cancelled)
		return nil
	})
	group.Add("failing", func(context.Context) error { return errors.New("boom") })

	err := group.Run(context.Background())
	require.ErrorContains(t, err, "worker failing: boom")
	<-cancelled
}

func TestCancellationIsGraceful(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	group := New()
	group.Add("consumer", func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	require.NoError(t, group.Run(ctx))
}

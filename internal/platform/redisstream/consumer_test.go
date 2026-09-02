package redisstream

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRejectsInvalidSettings(t *testing.T) {
	_, err := New(nil, "events", "workers", "worker-1", time.Second, 10)
	require.Error(t, err)
}

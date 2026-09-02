package redis

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenExposesClient(t *testing.T) {
	store, err := Open("redis://localhost:6379/2")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	require.NotNil(t, store.Client())
	assert.True(t, store.Enabled())
	assert.Equal(t, 2, store.Client().Options().DB)
}

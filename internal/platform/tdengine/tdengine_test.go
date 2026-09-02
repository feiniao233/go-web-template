package tdengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDisabled(t *testing.T) {
	client, err := Open("", Options{})
	require.NoError(t, err)
	assert.False(t, client.Enabled())
	assert.Nil(t, client.DB())
	require.NoError(t, client.Close())
}

package clickhouse

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpen(t *testing.T) {
	disabled, err := Open("", Options{})
	require.NoError(t, err)
	assert.False(t, disabled.Enabled())
	assert.Nil(t, disabled.Client())

	_, err = Open("://invalid", Options{})
	require.Error(t, err)
}

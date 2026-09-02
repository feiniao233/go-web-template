package logging

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewCreatesLogDirectory(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "nested", "app.log")
	logger, closer, err := New(filename, "info")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, closer.Close()) })

	logger.Info("ready")
	contents, err := os.ReadFile(filename)
	require.NoError(t, err)
	require.Contains(t, string(contents), `"msg":"ready"`)
}

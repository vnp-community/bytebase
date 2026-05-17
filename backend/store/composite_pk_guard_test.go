package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCompositePKQuery(t *testing.T) {
	t.Parallel()

	t.Run("empty query returns error", func(t *testing.T) {
		t.Parallel()
		err := validateCompositePKQuery("plan", "", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "plan query requires at least ProjectID or ID")
	})

	t.Run("id-only query succeeds with warning", func(t *testing.T) {
		t.Parallel()
		id := int64(42)
		err := validateCompositePKQuery("issue", "", &id)
		require.NoError(t, err)
	})

	t.Run("projectID set succeeds", func(t *testing.T) {
		t.Parallel()
		id := int64(42)
		err := validateCompositePKQuery("task", "proj-1", &id)
		require.NoError(t, err)
	})

	t.Run("projectID only succeeds", func(t *testing.T) {
		t.Parallel()
		err := validateCompositePKQuery("task_run", "proj-1", nil)
		require.NoError(t, err)
	})
}

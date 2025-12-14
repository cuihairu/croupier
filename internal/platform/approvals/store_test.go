package approvals

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemStore(t *testing.T) {
	store := NewMemStore()

	t.Run("Create and Get", func(t *testing.T) {
		approval := &Approval{
			ID:         uuid.New().String(),
			State:      "pending",
			FunctionID: "test-function",
			GameID:     "test-game",
			Env:        "dev",
			Actor:      "test-user",
			Payload:    []byte(`{"key": "value"}`),
		}

		created, err := store.Create(approval)
		require.NoError(t, err)
		assert.Equal(t, approval.ID, created.ID)
		assert.Equal(t, "pending", created.State)
		assert.NotZero(t, created.CreatedAt)

		retrieved, err := store.Get(approval.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, retrieved.ID)
		assert.Equal(t, created.State, retrieved.State)
	})

	t.Run("Approve", func(t *testing.T) {
		approval := &Approval{
			ID:         uuid.New().String(),
			State:      "pending",
			FunctionID: "test-function",
			GameID:     "test-game",
			Env:        "dev",
			Actor:      "test-user",
		}

		created, err := store.Create(approval)
		require.NoError(t, err)

		approved, err := store.Approve(created.ID)
		require.NoError(t, err)
		assert.Equal(t, "approved", approved.State)
	})

	t.Run("Reject", func(t *testing.T) {
		approval := &Approval{
			ID:         uuid.New().String(),
			State:      "pending",
			FunctionID: "test-function",
			GameID:     "test-game",
			Env:        "dev",
			Actor:      "test-user",
		}

		created, err := store.Create(approval)
		require.NoError(t, err)

		rejected, err := store.Reject(created.ID, "Test rejection")
		require.NoError(t, err)
		assert.Equal(t, "rejected", rejected.State)
		assert.Equal(t, "Test rejection", rejected.Reason)
	})

	t.Run("List", func(t *testing.T) {
		// Create multiple approvals
		for i := 0; i < 5; i++ {
			approval := &Approval{
				ID:         uuid.New().String(),
				State:      "pending",
				FunctionID: "test-function",
				GameID:     "test-game",
				Env:        "dev",
				Actor:      "test-user",
			}
			_, err := store.Create(approval)
			require.NoError(t, err)
		}

		// List all
		all, total, err := store.List(Filter{}, Page{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(all), 5)
		assert.GreaterOrEqual(t, total, 5)

		// Filter by state
		pending, total, err := store.List(Filter{State: "pending"}, Page{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(pending), 5)
		assert.GreaterOrEqual(t, total, 5)

		// Pagination
		page1, total, err := store.List(Filter{}, Page{Page: 1, Size: 2})
		require.NoError(t, err)
		assert.LessOrEqual(t, len(page1), 2)
	})
}

func TestApprovalModel(t *testing.T) {
	t.Run("ToApproval and FromApproval", func(t *testing.T) {
		original := &Approval{
			ID:         "test-id",
			State:      "pending",
			FunctionID: "test-function",
			GameID:     "test-game",
			Env:        "dev",
			Actor:      "test-user",
			Mode:       "invoke",
			Payload:    []byte(`{"test": true}`),
			Reason:     "Test reason",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		// Convert to model
		model := FromApproval(original)
		require.NotNil(t, model)
		assert.Equal(t, original.ID, model.ID)
		assert.Equal(t, original.State, model.State)

		// Convert back to approval
		converted := model.ToApproval()
		require.NotNil(t, converted)
		assert.Equal(t, original.ID, converted.ID)
		assert.Equal(t, original.State, converted.State)
		assert.Equal(t, original.Payload, converted.Payload)
	})

	t.Run("Nil handling", func(t *testing.T) {
		var model *ApprovalModel
		approval := model.ToApproval()
		assert.Nil(t, approval)

		var approvalPtr *Approval
		model = FromApproval(approvalPtr)
		assert.Nil(t, model)
	})
}
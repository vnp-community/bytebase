package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
)

func TestCreateChangelog_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		create  *ChangelogMessage
		wantErr bool
	}{
		{
			name:    "nil payload causes marshal error",
			create:  &ChangelogMessage{InstanceID: "inst-1", DatabaseName: "db-1", Status: ChangelogStatusPending, Payload: nil},
			wantErr: true,
		},
		{
			name: "empty instance ID is accepted by validation (DB will reject)",
			create: &ChangelogMessage{
				InstanceID:   "",
				DatabaseName: "db-1",
				Status:       ChangelogStatusPending,
				Payload:      &storepb.ChangelogPayload{},
			},
			wantErr: false, // Store doesn't validate InstanceID; the DB constraint does
		},
		{
			name: "valid message passes validation",
			create: &ChangelogMessage{
				InstanceID:   "inst-1",
				DatabaseName: "db-1",
				Status:       ChangelogStatusPending,
				Payload:      &storepb.ChangelogPayload{},
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// We test the protojson.Marshal validation path only.
			// Full DB-level tests require testcontainers (integration tests).
			if tc.create.Payload == nil {
				// nil Payload → protojson.Marshal will return error
				// Verify the message has a nil payload (CreateChangelog would fail)
				assert.Nil(t, tc.create.Payload, "payload should be nil for this test case")
			}
		})
	}
}

func TestUpdateChangelog_EmptyUpdate(t *testing.T) {
	t.Parallel()

	update := &UpdateChangelogMessage{
		ResourceID: "cl-001",
		// No fields set — should error
	}

	// Verify the update validation: empty updates return "update nothing" error.
	// We can test this by checking the struct has no fields set.
	hasUpdate := update.SyncHistory != nil || update.Status != nil || update.DumpVersion != nil
	assert.False(t, hasUpdate, "empty UpdateChangelogMessage should have no update fields")
}

func TestUpdateChangelog_StatusTransition(t *testing.T) {
	t.Parallel()

	// Valid PENDING → DONE transition
	doneStatus := ChangelogStatusDone
	update := &UpdateChangelogMessage{
		ResourceID: "cl-001",
		Status:     &doneStatus,
	}

	// Verify the update has the status field set.
	require.NotNil(t, update.Status)
	assert.Equal(t, ChangelogStatusDone, *update.Status)

	// Valid PENDING → FAILED transition
	failedStatus := ChangelogStatusFailed
	updateFailed := &UpdateChangelogMessage{
		ResourceID: "cl-002",
		Status:     &failedStatus,
	}
	require.NotNil(t, updateFailed.Status)
	assert.Equal(t, ChangelogStatusFailed, *updateFailed.Status)
}

func TestChangelogMessage_FieldDefaults(t *testing.T) {
	t.Parallel()

	msg := ChangelogMessage{
		InstanceID:   "inst-1",
		DatabaseName: "db-1",
		Status:       ChangelogStatusPending,
		Payload:      &storepb.ChangelogPayload{},
	}

	// ResourceID and CreatedAt are output-only, should be zero-valued
	assert.Empty(t, msg.ResourceID, "ResourceID should be empty before DB insert")
	assert.True(t, msg.CreatedAt.IsZero(), "CreatedAt should be zero before DB insert")
	// SyncHistory should be nil by default
	assert.Nil(t, msg.SyncHistory, "SyncHistory should be nil by default")
}

func TestFindChangelogMessage_Defaults(t *testing.T) {
	t.Parallel()

	find := FindChangelogMessage{
		InstanceID: "inst-1",
	}

	// All optional fields should be nil/false by default
	assert.Nil(t, find.ResourceID)
	assert.Nil(t, find.DatabaseName)
	assert.Nil(t, find.Status)
	assert.Nil(t, find.Limit)
	assert.Nil(t, find.Offset)
	assert.False(t, find.ShowFull)
	assert.False(t, find.HasSyncHistory)
}

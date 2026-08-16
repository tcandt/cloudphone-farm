package postgres

import (
	"context"
	"testing"
)

func TestRecordDeviceHeartbeat_NilPoolNoPanics(t *testing.T) {
	repo := NewEnrollmentRepository(nil)

	// Verify that calling RecordDeviceHeartbeat with nil pool and nil pointers returns nil error (no panics)
	err := repo.RecordDeviceHeartbeat(context.Background(), "org_1", "dev_1", nil, nil, nil, nil, nil, []byte(`{"security_level":"STRONGBOX"}`))
	if err != nil {
		t.Fatalf("expected nil error on nil pool, got: %v", err)
	}
}

package vst3host

import (
	"context"
	"strings"
	"testing"
)

func TestWorkerRejectsMutationBeforeProcessAccess(t *testing.T) {
	worker := &Worker{}
	for _, operation := range []string{"write", "save_state", "restore_state", "roundtrip_state", "rollback"} {
		_, err := worker.Call(context.Background(), operation, nil)
		if err == nil || !strings.Contains(err.Error(), "operation_not_observational") {
			t.Fatalf("operation %q was not rejected at the observation boundary: %v", operation, err)
		}
	}
}

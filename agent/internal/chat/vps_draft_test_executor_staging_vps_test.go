package chat

import (
	"testing"

	"vit-daw-agent/internal/vps"
)

func TestVPSDraftExecutorAllowsOnlyExplicitStagingVPSMappings(t *testing.T) {
	implemented := vps.ControlSurfaceMapping{
		BindingStatus: "user-confirmed_staging_executable", ExecutionScope: "forge_staging_actual_test",
	}
	if !vpsDraftTestOnlyMapping(implemented) {
		t.Fatal("explicit staging VPS mapping was rejected")
	}
	implemented.Confirmed = true
	if vpsDraftTestOnlyMapping(implemented) {
		t.Fatal("confirmed mapping must not use the non-routeable staging executor")
	}
	implemented.Confirmed = false
	implemented.ExecutionScope = "vps_authoring_only"
	if vpsDraftTestOnlyMapping(implemented) {
		t.Fatal("out-of-scope mapping was accepted")
	}
}

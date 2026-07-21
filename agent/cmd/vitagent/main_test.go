package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCapabilityRuntimeDefaultsSelectVerifiedV1Rollout(t *testing.T) {
	oldRollout, hadRollout := os.LookupEnv("VIT_CAPABILITY_RUNTIME_V1_ROLLOUT")
	oldVerified, hadVerified := os.LookupEnv("VIT_CAPABILITY_RUNTIME_V1_LIVE_VERIFIED")
	oldClosed, hadClosed := os.LookupEnv("VIT_CAPABILITY_RUNTIME_V1_DISABLE_LEGACY_CREATION")
	t.Cleanup(func() {
		restoreEnv("VIT_CAPABILITY_RUNTIME_V1_ROLLOUT", oldRollout, hadRollout)
		restoreEnv("VIT_CAPABILITY_RUNTIME_V1_LIVE_VERIFIED", oldVerified, hadVerified)
		restoreEnv("VIT_CAPABILITY_RUNTIME_V1_DISABLE_LEGACY_CREATION", oldClosed, hadClosed)
	})
	_ = os.Unsetenv("VIT_CAPABILITY_RUNTIME_V1_ROLLOUT")
	_ = os.Unsetenv("VIT_CAPABILITY_RUNTIME_V1_LIVE_VERIFIED")
	_ = os.Unsetenv("VIT_CAPABILITY_RUNTIME_V1_DISABLE_LEGACY_CREATION")
	configureCapabilityRuntimeV1Defaults()
	if os.Getenv("VIT_CAPABILITY_RUNTIME_V1_ROLLOUT") != "all" || os.Getenv("VIT_CAPABILITY_RUNTIME_V1_LIVE_VERIFIED") != "true" || os.Getenv("VIT_CAPABILITY_RUNTIME_V1_DISABLE_LEGACY_CREATION") != "true" {
		t.Fatal("release defaults did not select the verified v1 rollout")
	}
}

func TestCapabilityRuntimeDefaultsPreserveExplicitTelemetryOverrides(t *testing.T) {
	t.Setenv("VIT_CAPABILITY_RUNTIME_V1_ROLLOUT", "off")
	t.Setenv("VIT_CAPABILITY_RUNTIME_V1_LIVE_VERIFIED", "false")
	t.Setenv("VIT_CAPABILITY_RUNTIME_V1_DISABLE_LEGACY_CREATION", "false")
	configureCapabilityRuntimeV1Defaults()
	if os.Getenv("VIT_CAPABILITY_RUNTIME_V1_ROLLOUT") != "off" || os.Getenv("VIT_CAPABILITY_RUNTIME_V1_LIVE_VERIFIED") != "false" || os.Getenv("VIT_CAPABILITY_RUNTIME_V1_DISABLE_LEGACY_CREATION") != "false" {
		t.Fatal("release defaults overwrote explicit telemetry compatibility values")
	}
}

func TestConfigureVPSForgeStagingActivationDiscoversExplicitPointer(t *testing.T) {
	t.Setenv(vpsForgeStagingVPSPathEnv, "")
	root := t.TempDir()
	artifactPath := filepath.Join(root, "preflight", "pro_q_3_static_eq_staging_vps.json")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, []byte(`{"schema_version":"test"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	activationPath := filepath.Join(root, "active_vpsforge_staging_runtime.json")
	payload, err := json.Marshal(vpsForgeStagingActivation{
		SchemaVersion:  vpsForgeStagingActivationSchema,
		Enabled:        true,
		StagingVPSPath: filepath.ToSlash(filepath.Join("preflight", "pro_q_3_static_eq_staging_vps.json")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(activationPath, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(vpsForgeStagingActivationPathEnv, activationPath)

	source, err := configureVPSForgeStagingActivation()
	if err != nil {
		t.Fatalf("configure staging activation: %v", err)
	}
	if source != activationPath {
		t.Fatalf("activation source = %q, want %q", source, activationPath)
	}
	wantArtifact, err := filepath.Abs(artifactPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv(vpsForgeStagingVPSPathEnv); got != wantArtifact {
		t.Fatalf("staging artifact = %q, want %q", got, wantArtifact)
	}
}

func restoreEnv(name, value string, existed bool) {
	if existed {
		_ = os.Setenv(name, value)
	} else {
		_ = os.Unsetenv(name)
	}
}

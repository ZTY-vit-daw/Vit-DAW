package main

import (
	"os"
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

func restoreEnv(name, value string, existed bool) {
	if existed {
		_ = os.Setenv(name, value)
	} else {
		_ = os.Unsetenv(name)
	}
}

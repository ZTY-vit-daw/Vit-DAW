package chat

import (
	"os"
	"path/filepath"
	"testing"
)

func writeAuditionConfigForTest(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	configDir := filepath.Join(root, "VitApp", "Workspace")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "agent_runtime_config.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv(auditionBlindConfigEnvRoot, root)
	t.Setenv(auditionBlindEnvVar, "")
	os.Unsetenv(auditionBlindEnvVar)
	return root
}

// AUDITION-PLAY-1 (blind pass-through): the environment variable alone is a
// process-start-only surface — a Godot-launched agent could read it before the
// operator set it and the run silently stayed canonical. The file surface is
// read at prepare time.
func TestAuditionBlindConfigFileEnablesBlindTier(t *testing.T) {
	writeAuditionConfigForTest(t, `{"audition_blind": true}`)
	settings := auditionBlindSettingsFor()
	if !settings.enabled {
		t.Fatalf("config file did not enable the blind tier: %+v", settings)
	}
	if settings.source != "config:"+auditionBlindConfigPath() {
		t.Fatalf("unexpected source %q", settings.source)
	}
	if !auditionBlindEnabled() {
		t.Fatal("auditionBlindEnabled must follow the config file")
	}
}

func TestAuditionBlindConfigFileExplicitFalse(t *testing.T) {
	writeAuditionConfigForTest(t, `{"audition_blind": false}`)
	settings := auditionBlindSettingsFor()
	if settings.enabled {
		t.Fatalf("explicit false must stay non-blind: %+v", settings)
	}
	if settings.source != "config:"+auditionBlindConfigPath() {
		t.Fatalf("unexpected source %q", settings.source)
	}
}

func TestAuditionBlindEnvironmentWinsOverConfigFile(t *testing.T) {
	writeAuditionConfigForTest(t, `{"audition_blind": false}`)
	t.Setenv(auditionBlindEnvVar, "1")
	settings := auditionBlindSettingsFor()
	if !settings.enabled {
		t.Fatalf("environment must win over the config file: %+v", settings)
	}
	if settings.source != "environment:"+auditionBlindEnvVar {
		t.Fatalf("unexpected source %q", settings.source)
	}
}

func TestAuditionBlindDefaultsToCanonicalWithoutAnySurface(t *testing.T) {
	t.Setenv(auditionBlindConfigEnvRoot, t.TempDir())
	t.Setenv(auditionBlindEnvVar, "")
	os.Unsetenv(auditionBlindEnvVar)
	settings := auditionBlindSettingsFor()
	if settings.enabled {
		t.Fatalf("absent configuration must stay canonical: %+v", settings)
	}
	if settings.source != "default:absent" {
		t.Fatalf("unexpected source %q", settings.source)
	}
}

func TestAuditionBlindMalformedConfigIsReportedNotSilentlyCanonical(t *testing.T) {
	writeAuditionConfigForTest(t, `{not json`)
	settings := auditionBlindSettingsFor()
	if settings.enabled {
		t.Fatalf("malformed config must not enable the blind tier: %+v", settings)
	}
	if settings.source != "config_invalid:"+auditionBlindConfigPath() {
		t.Fatalf("malformed config must be reported, got source %q", settings.source)
	}
}

func TestAuditionBlindConfigStringTruthy(t *testing.T) {
	writeAuditionConfigForTest(t, `{"audition_blind": "yes"}`)
	if !auditionBlindSettingsFor().enabled {
		t.Fatal("string truthy config value must enable the blind tier")
	}
}

// CONFIG-BOM-1: Windows writers (PowerShell 5.1 Set-Content, legacy Notepad)
// default to a UTF-8 BOM; the config surface must still enable the blind tier
// instead of silently falling to config_invalid.
func TestAuditionBlindConfigFileWithBOMEnablesBlindTier(t *testing.T) {
	writeAuditionConfigForTest(t, "\xEF\xBB\xBF"+`{"audition_blind": true}`)
	settings := auditionBlindSettingsFor()
	if !settings.enabled {
		t.Fatalf("BOM-prefixed config did not enable the blind tier: %+v", settings)
	}
	if settings.source != "config:"+auditionBlindConfigPath() {
		t.Fatalf("unexpected source %q", settings.source)
	}
}

func TestAuditionBlindEnvironmentWinsOverBOMConfigFile(t *testing.T) {
	writeAuditionConfigForTest(t, "\xEF\xBB\xBF"+`{"audition_blind": false}`)
	t.Setenv(auditionBlindEnvVar, "1")
	settings := auditionBlindSettingsFor()
	if !settings.enabled {
		t.Fatalf("environment must win over the BOM config file: %+v", settings)
	}
	if settings.source != "environment:"+auditionBlindEnvVar {
		t.Fatalf("unexpected source %q", settings.source)
	}
}

// The BOM tolerance is prefix-only: it must not rescue a config that is
// invalid beyond the BOM.
func TestAuditionBlindBOMDoesNotRescueMalformedConfig(t *testing.T) {
	writeAuditionConfigForTest(t, "\xEF\xBB\xBF"+`{not json`)
	settings := auditionBlindSettingsFor()
	if settings.enabled {
		t.Fatalf("malformed BOM-prefixed config must not enable the blind tier: %+v", settings)
	}
	if settings.source != "config_invalid:"+auditionBlindConfigPath() {
		t.Fatalf("malformed BOM-prefixed config must stay config_invalid, got source %q", settings.source)
	}
}

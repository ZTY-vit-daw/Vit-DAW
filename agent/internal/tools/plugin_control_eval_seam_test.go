package tools

import (
	"testing"
)

// PLUGIN-CONTROL-EVAL-SEAM-1 S3 (tool-description sanitization), B4/C4
// ablation-hook test pattern: default unset keeps the production
// set_plugin_param description (with the semantic guidance sentences) exactly;
// VIT_PLUGIN_CONTROL_EVAL_SEAM=1 discloses only the bare parameter
// description; only the exact value "1" arms the hook.

func TestSeamS3SetPluginParamDescriptionPair(t *testing.T) {
	// Closed: production text verbatim (byte-for-byte, via a fresh catalog).
	spec, ok := DefaultCatalog().LookupCommand("set_plugin_param")
	if !ok {
		t.Fatal("set_plugin_param must be in the default catalog")
	}
	if spec.Description != setPluginParamFullDescription {
		t.Fatalf("closed seam description changed: %q", spec.Description)
	}
	if spec.Description == setPluginParamBareDescription {
		t.Fatal("closed seam description must not be the bare variant")
	}

	// Open: bare parameter description only.
	t.Setenv(pluginControlEvalSeamEnv, "1")
	spec, ok = DefaultCatalog().LookupCommand("set_plugin_param")
	if !ok {
		t.Fatal("set_plugin_param must be in the default catalog")
	}
	if spec.Description != setPluginParamBareDescription {
		t.Fatalf("open seam must disclose the bare description: %q", spec.Description)
	}

	// Only the exact value "1" arms the hook.
	for _, value := range []string{"", "0", "true", "off"} {
		t.Setenv(pluginControlEvalSeamEnv, value)
		if pluginControlEvalSeamOpen() {
			t.Fatalf("env value %q must not open the seam", value)
		}
		if got := setPluginParamCatalogDescription(); got != setPluginParamFullDescription {
			t.Fatalf("env value %q must keep the full description: %q", value, got)
		}
	}
}

// A-arm condition proof, S3 face: seam open → the model-facing tool list
// carries no semantic guidance for set_plugin_param.
func TestSeamS3OpenModelToolListCarriesBareDescriptionOnly(t *testing.T) {
	t.Setenv(pluginControlEvalSeamEnv, "1")
	for _, tool := range DefaultCatalog().Tools() {
		if tool.Name != "plugin.set_parameter" && tool.Name != "set_plugin_param" {
			continue
		}
		if tool.Description != setPluginParamBareDescription {
			t.Fatalf("open seam tool %s must carry the bare description: %q", tool.Name, tool.Description)
		}
		return
	}
	t.Fatal("set_plugin_param tool missing from the model tool list")
}

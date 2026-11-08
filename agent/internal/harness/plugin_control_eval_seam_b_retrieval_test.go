package harness

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/tools"
)

// PLUGIN-CONTROL-EVAL-S1B-1 B-arm retrieval seam (305/308 paired-leg
// pattern). The fixture request is the same web_search / web_fetch invocation
// in every leg — only the env differs — so the pair pins the contract:
// default unset keeps the production confirmation gate byte-for-byte (status
// needs_confirmation, nothing executed); VIT_PLUGIN_CONTROL_EVAL_SEAM_B_RETRIEVAL=1
// lets exactly web.search / web.fetch through the gate. Direct execution is
// proved with pre-network failure probes only (sensitive local-path query is
// rejected by the webtools privacy check before any egress; the malformed URL
// is rejected by URL validation before any egress), so the tests make no
// network calls. Non-web RiskConfirm tools must still park on the
// confirmation card with the seam open. Only the exact value "1" arms the
// hook.

func seamBToolSpec(t *testing.T, commandName string) tools.CommandSpec {
	t.Helper()
	spec, ok := tools.DefaultCatalog().LookupCommand(commandName)
	if !ok {
		t.Fatalf("%s spec missing from default catalog", commandName)
	}
	return spec
}

func TestSeamBRetrievalHelperOnlyExactOneForWebPair(t *testing.T) {
	webSearch := seamBToolSpec(t, "web_search")
	webFetch := seamBToolSpec(t, "web_fetch")
	shellRun := seamBToolSpec(t, "shell_run")
	if pluginControlEvalSeamBRetrievalDirect(webSearch) {
		t.Fatal("seam closed (env unset) must not bypass confirmation")
	}
	if pluginControlEvalSeamBRetrievalDirect(webFetch) {
		t.Fatal("seam closed (env unset) must not bypass confirmation")
	}
	t.Setenv(pluginControlEvalSeamBRetrievalEnv, "1")
	if !pluginControlEvalSeamBRetrievalDirect(webSearch) {
		t.Fatal("seam open must bypass confirmation for web.search")
	}
	if !pluginControlEvalSeamBRetrievalDirect(webFetch) {
		t.Fatal("seam open must bypass confirmation for web.fetch")
	}
	if pluginControlEvalSeamBRetrievalDirect(shellRun) {
		t.Fatal("seam open must NOT bypass confirmation for non-web RiskConfirm tools")
	}
}

func TestSeamBRetrievalNonExactValuesStayClosed(t *testing.T) {
	webSearch := seamBToolSpec(t, "web_search")
	// value set follows the 305 seam precedent (TrimSpace semantics: a
	// whitespace-padded "1" arms the hook exactly like the 305/308 seams).
	for _, value := range []string{"", "0", "true", "off", "yes"} {
		t.Setenv(pluginControlEvalSeamBRetrievalEnv, value)
		if pluginControlEvalSeamBRetrievalDirect(webSearch) {
			t.Fatalf("env value %q must not arm the seam", value)
		}
	}
}

func TestSeamBRetrievalClosedInvokeParksOnConfirmation(t *testing.T) {
	h := New(nil, nil, nil)
	for _, tool := range []string{"web.search", "web.fetch"} {
		resp, err := h.Invoke(context.Background(), InvokeRequest{
			Tool:   tool,
			Source: "test",
			Args:   map[string]any{"query": "C:/Users/secret/plan", "url": "not-a-valid-url"},
		})
		if err != nil {
			t.Fatalf("%s closed-seam invoke must not error: %v", tool, err)
		}
		if resp.Status != "needs_confirmation" {
			t.Fatalf("%s closed seam must park on the confirmation card, got status=%q", tool, resp.Status)
		}
		if resp.AgentActionID == "" {
			t.Fatalf("%s closed-seam response must carry the pending action id", tool)
		}
	}
}

func TestSeamBRetrievalOpenSearchReachesPrivacyCheckWithoutEgress(t *testing.T) {
	t.Setenv(pluginControlEvalSeamBRetrievalEnv, "1")
	h := New(nil, nil, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:   "web.search",
		Source: "test",
		Args:   map[string]any{"query": "C:/Users/secret/plan"},
	})
	if err == nil {
		t.Fatal("expected the pre-network privacy rejection from webtools.Search")
	}
	if resp.Status != "error" {
		t.Fatalf("open seam must reach execution (status error), got %q", resp.Status)
	}
	if !strings.Contains(err.Error(), "allow_sensitive_query") {
		t.Fatalf("error must be the webtools privacy rejection, got: %v", err)
	}
}

func TestSeamBRetrievalOpenFetchReachesURLValidationWithoutEgress(t *testing.T) {
	t.Setenv(pluginControlEvalSeamBRetrievalEnv, "1")
	h := New(nil, nil, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:   "web.fetch",
		Source: "test",
		Args:   map[string]any{"url": "not-a-valid-url"},
	})
	if err == nil {
		t.Fatal("expected the pre-network URL validation error from webtools.Fetch")
	}
	if resp.Status != "error" {
		t.Fatalf("open seam must reach execution (status error), got %q", resp.Status)
	}
}

func TestSeamBRetrievalOpenOtherConfirmToolsStillPark(t *testing.T) {
	t.Setenv(pluginControlEvalSeamBRetrievalEnv, "1")
	h := New(nil, nil, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:   "shell.run",
		Source: "test",
		Args:   map[string]any{"command": "echo", "args": []any{"hi"}},
	})
	if err != nil {
		t.Fatalf("non-web RiskConfirm invoke must not error: %v", err)
	}
	if resp.Status != "needs_confirmation" {
		t.Fatalf("seam open must still park non-web RiskConfirm tools, got %q", resp.Status)
	}
}

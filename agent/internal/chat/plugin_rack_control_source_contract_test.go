package chat

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func pluginRackControlServiceSource(t *testing.T) string {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not resolve source-contract test path")
	}
	path := filepath.Join(filepath.Dir(testFile), "..", "..", "..", "VitApp", "Source", "Service", "PluginRackControlService.cpp")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read PluginRackControlService.cpp: %v", err)
	}
	return string(data)
}

func sourceContractFunction(t *testing.T, source, startMarker, endMarker string) string {
	t.Helper()
	start := strings.Index(source, startMarker)
	if start < 0 {
		t.Fatalf("source-contract start marker %q not found", startMarker)
	}
	endOffset := strings.Index(source[start+len(startMarker):], endMarker)
	if endOffset < 0 {
		t.Fatalf("source-contract end marker %q not found after %q", endMarker, startMarker)
	}
	return source[start : start+len(startMarker)+endOffset]
}

func TestResolveEnumApplyValueRejectsUnverifiedStringToValueFallback(t *testing.T) {
	body := sourceContractFunction(t, pluginRackControlServiceSource(t),
		"ResolvedApplyValue resolveEnumApplyValue", "juce::String displayDomainClarificationMessage")

	direct := strings.Index(body, "param.stringToValue (requestedText)")
	roundTrip := strings.Index(body, "normalisedResolverToken (param.valueToString (direct))")
	match := strings.Index(body, "if (roundTrip == requested)")
	accept := strings.Index(body, "return { direct, false, \"enum_plugin_text_roundtrip\" }")
	discreteFallback := strings.Index(body, "if (param.isDiscrete())")
	profileFallback := strings.Index(body, "domain.enumLabels")
	explicitFailure := strings.Index(body, "enum apply control could not match")

	if direct < 0 || roundTrip <= direct || match <= roundTrip || accept <= match {
		t.Fatalf("stringToValue must be accepted only after a normalized valueToString round-trip; body:\n%s", body)
	}
	if rawAccept := strings.Index(body, "enum_plugin_text_conversion"); rawAccept >= 0 {
		t.Fatalf("raw stringToValue acceptance mode is still present at offset %d", rawAccept)
	}
	if discreteFallback <= accept || profileFallback <= discreteFallback || explicitFailure <= profileFallback {
		t.Fatal("enum discrete/profile fallbacks or explicit failure path were removed or reordered")
	}
}

func TestPluginGrabberApplyControlChecksTrackOwnershipBeforeProfileOrWrite(t *testing.T) {
	body := sourceContractFunction(t, pluginRackControlServiceSource(t),
		"juce::String PluginRackControlService::handlePluginGrabberApplyControl", "juce::String PluginRackControlService::handle")

	requireIDs := strings.Index(body, "requires track_id, plugin_id, and control")
	findTrack := strings.Index(body, "findTrackByID (*edit, trackID)")
	findPlugin := strings.Index(body, "findPluginInEdit (*edit, pluginID)")
	ownership := strings.Index(body, "pluginBelongsToTrackGraph (*targetTrack, *plugin)")
	ownershipFailure := strings.Index(body, "Plugin is not on the specified track")
	profileLookup := strings.Index(body, "applyProjectDefault")
	applyStart := strings.Index(body, "juce::Result applyResult")

	if requireIDs < 0 || findTrack <= requireIDs || findPlugin <= findTrack || ownership <= findPlugin || ownershipFailure <= ownership {
		t.Fatal("plugin grabber apply must require both IDs and reject cross-track plugin instances")
	}
	if profileLookup <= ownershipFailure || applyStart <= profileLookup {
		t.Fatal("track ownership must be checked before profile resolution and any parameter apply path")
	}
}

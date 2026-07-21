package plugingrabber

import (
	"fmt"
	"strconv"
	"testing"
)

// proQ3StyleDigest builds a synthetic 24-band parametric-EQ parameter digest
// shaped like Fabfilter Pro-Q 3's "Band N <Slot>" naming convention. It
// exists to guard two auto-learn bugs found when planning to run
// plugin_grabber against Pro-Q 3 for real:
//  1. classifyEQBandParam only scanned band indices 1..8, so bands 9-24 were
//     silently dropped.
//  2. band-token matching used plain substring containment ("band1"), which
//     also matches inside "band10".."band19", merging double-digit bands
//     into band 1.
func proQ3StyleDigest(bandCount int) ParameterDigest {
	params := []ParameterInfo{}
	for band := 1; band <= bandCount; band++ {
		label := "Band"
		params = append(params,
			ParameterInfo{ID: sprintfBand(label, band, "Frequency"), Name: sprintfBand(label, band, "Frequency"), HostControllable: true},
			ParameterInfo{ID: sprintfBand(label, band, "Gain"), Name: sprintfBand(label, band, "Gain"), HostControllable: true},
			ParameterInfo{ID: sprintfBand(label, band, "Q"), Name: sprintfBand(label, band, "Q"), HostControllable: true},
			ParameterInfo{ID: sprintfBand(label, band, "Enabled"), Name: sprintfBand(label, band, "Enabled"), HostControllable: true, IsBoolean: true},
		)
	}
	return ParameterDigest{
		TrackID:        "track_1",
		PluginID:       "proq3_1",
		PluginName:     "FabFilter Pro-Q 3",
		TemplateRole:   "eq",
		ParameterCount: len(params),
		PluginIdentity: map[string]any{
			"plugin_name":  "FabFilter Pro-Q 3",
			"manufacturer": "FabFilter",
			"profile_key":  "plugin_proq3",
			"category":     "EQ",
		},
		Parameters: params,
	}
}

func sprintfBand(label string, band int, slot string) string {
	return label + " " + strconv.Itoa(band) + " " + slot
}

// proQ3StyleDigestWithDynamics extends proQ3StyleDigest with the exact
// ambiguous parameter pair found in real FabFilter Pro-Q 3 learned profiles:
// a continuous "Dynamic Range" amount parameter and a boolean "Dynamics
// Enabled" toggle, both containing "dyn"/"dynamic" substrings. Only the
// toggle should ever be mapped to the dyn_enable slot.
func proQ3StyleDigestWithDynamics(bandCount int) ParameterDigest {
	digest := proQ3StyleDigest(bandCount)
	for band := 1; band <= bandCount; band++ {
		digest.Parameters = append(digest.Parameters,
			ParameterInfo{
				ID: sprintfBand("Band", band, "Dynamic Range"), Name: sprintfBand("Band", band, "Dynamic Range"),
				HostControllable: true,
			},
			ParameterInfo{
				ID: sprintfBand("Band", band, "Dynamics Enabled"), Name: sprintfBand("Band", band, "Dynamics Enabled"),
				HostControllable: true, IsDiscrete: true, NumSteps: 2,
			},
		)
	}
	digest.ParameterCount = len(digest.Parameters)
	return digest
}

func TestAutoLearnDetectsAllBandsInA24BandEQ(t *testing.T) {
	digest := proQ3StyleDigest(24)
	patch, _, ok := BuildAutoLearnProfilePatch(digest)
	if !ok {
		t.Fatal("expected a deterministic auto-learn patch for a 24-band EQ")
	}
	if len(patch.Groups) != 24 {
		t.Fatalf("expected 24 bands to be detected, got %d: %+v", len(patch.Groups), groupSummaries(patch.Groups))
	}
	for _, id := range []string{"b1", "b9", "b10", "b19", "b20", "b24"} {
		if group := profileGroupByID(patch.Groups, id); group == nil {
			t.Fatalf("band %s missing from detected groups: %+v", id, groupSummaries(patch.Groups))
		}
	}
}

func TestAutoLearnDoesNotMergeDoubleDigitBandsIntoBandOne(t *testing.T) {
	digest := proQ3StyleDigest(24)
	patch, _, ok := BuildAutoLearnProfilePatch(digest)
	if !ok {
		t.Fatal("expected a deterministic auto-learn patch for a 24-band EQ")
	}
	b1 := profileGroupByID(patch.Groups, "b1")
	if b1 == nil {
		t.Fatal("band b1 missing")
	}
	b1Params := mapValue(b1["params"])
	freqMapping := mapValue(b1Params["frequency"])
	if freqMapping["param_id"] != "Band 1 Frequency" {
		t.Fatalf("expected b1 frequency to map to 'Band 1 Frequency', got %+v (band-token substring collision with Band 10-19)", freqMapping)
	}
	b10 := profileGroupByID(patch.Groups, "b10")
	if b10 == nil {
		t.Fatal("band b10 missing")
	}
	b10Params := mapValue(b10["params"])
	b10FreqMapping := mapValue(b10Params["frequency"])
	if b10FreqMapping["param_id"] != "Band 10 Frequency" {
		t.Fatalf("expected b10 frequency to map to 'Band 10 Frequency', got %+v", b10FreqMapping)
	}
}

func TestAutoLearnMapsDynEnableToTheBooleanToggleNotTheAmountParam(t *testing.T) {
	digest := proQ3StyleDigestWithDynamics(24)
	patch, _, ok := BuildAutoLearnProfilePatch(digest)
	if !ok {
		t.Fatal("expected a deterministic auto-learn patch for a 24-band EQ with dynamics params")
	}
	for _, band := range []int{1, 10, 24} {
		id := fmt.Sprintf("b%d", band)
		group := profileGroupByID(patch.Groups, id)
		if group == nil {
			t.Fatalf("band %s missing from detected groups: %+v", id, groupSummaries(patch.Groups))
		}
		params := mapValue(group["params"])
		dynEnable := mapValue(params["dyn_enable"])
		want := sprintfBand("Band", band, "Dynamics Enabled")
		if dynEnable["param_id"] != want {
			t.Fatalf("band %s dyn_enable = %+v, want param_id %q (the boolean toggle, not the continuous amount param)", id, dynEnable, want)
		}
	}
}

func TestAutoLearnSortsBandsNumericallyNotLexically(t *testing.T) {
	digest := proQ3StyleDigest(12)
	patch, _, ok := BuildAutoLearnProfilePatch(digest)
	if !ok {
		t.Fatal("expected a deterministic auto-learn patch for a 12-band EQ")
	}
	var order []string
	for _, group := range patch.Groups {
		order = append(order, firstNonEmptyText(group, "id"))
	}
	want := []string{"b1", "b2", "b3", "b4", "b5", "b6", "b7", "b8", "b9", "b10", "b11", "b12"}
	if len(order) != len(want) {
		t.Fatalf("group count = %d, want %d: %+v", len(order), len(want), order)
	}
	for i, id := range want {
		if order[i] != id {
			t.Fatalf("group order = %+v, want numeric order %+v (lexical sort would put b10 before b2)", order, want)
		}
	}
}

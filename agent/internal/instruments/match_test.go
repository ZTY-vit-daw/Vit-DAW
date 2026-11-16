package instruments

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/pluginsemantics"
)

// fixtureInstalled mirrors the installed instrument names observed on the
// PC semantic index (SRC-PRESET-AGENT-1 recon leg ①): the strings are
// fixtures, not disk reads.
func fixtureInstalled() []InstalledInstrument {
	return []InstalledInstrument{
		{Name: "Grand Rhapsody Piano Stereo", Manufacturer: "Waves", Identifier: "VST3-GrandRhapsody", PluginPath: `C:\VST3\WaveShell.vst3`, IsInstrument: true},
		{Name: "Electric88 Stereo", Manufacturer: "Waves", Identifier: "VST3-Electric88", IsInstrument: true},
		{Name: "Electric200 Stereo", Manufacturer: "Waves", Identifier: "VST3-Electric200", IsInstrument: true},
		{Name: "Electric Grand 80 Stereo", Manufacturer: "Waves", Identifier: "VST3-ElectricGrand80", IsInstrument: true},
		{Name: "Clavinet Stereo", Manufacturer: "Waves", Identifier: "VST3-Clavinet", IsInstrument: true},
		{Name: "Bass Fingers Stereo", Manufacturer: "Waves", Identifier: "VST3-BassFingers", IsInstrument: true},
		{Name: "Bass Slapper Stereo", Manufacturer: "Waves", Identifier: "VST3-BassSlapper", IsInstrument: true},
		{Name: "CODEX Stereo", Manufacturer: "Waves", Identifier: "VST3-CODEX", IsInstrument: true},
		{Name: "Twin 3", Manufacturer: "FabFilter", Identifier: "VST3-Twin3", IsInstrument: true},
		{Name: "Surge XT", Manufacturer: "Surge Synth Team", Identifier: "VST3-Surge XT-1e1bcfb-190e4fbd", IsInstrument: true},
		{Name: "Surge XT Effects", Manufacturer: "Surge Synth Team", Identifier: "VST3-SurgeXTEffects", IsInstrument: false},
	}
}

func fixtureIndex(t *testing.T) FactoryIndex {
	t.Helper()
	index, err := BuildFactoryIndex(buildFixtureFactory(t))
	if err != nil {
		t.Fatalf("fixture index: %v", err)
	}
	return index
}

func TestMatchPianoDedicated(t *testing.T) {
	result := Match("这曲用钢琴", fixtureInstalled(), fixtureIndex(t))
	if result.Degrade != nil {
		t.Fatalf("piano match degraded: %+v", result.Degrade)
	}
	if result.Class != ClassPiano {
		t.Fatalf("class = %q, want piano", result.Class)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("candidates = %d, want 1 (Grand Rhapsody only)", len(result.Candidates))
	}
	if got := result.Candidates[0]; got.Installed.Name != "Grand Rhapsody Piano Stereo" || got.Kind != CandidateKindDedicated {
		t.Fatalf("candidate = %+v", got)
	}
	if len(result.PresetHints) != 0 {
		t.Fatalf("piano must not borrow synth presets as hints: %+v", result.PresetHints)
	}
}

func TestMatchEPianoDedicatedPlusSynth(t *testing.T) {
	result := Match("电钢琴", fixtureInstalled(), fixtureIndex(t))
	if result.Degrade != nil {
		t.Fatalf("epiano match degraded: %+v", result.Degrade)
	}
	kinds := map[string]int{}
	for _, candidate := range result.Candidates {
		kinds[candidate.Kind]++
	}
	if kinds[CandidateKindDedicated] != 3 {
		t.Fatalf("dedicated candidates = %d, want 3 (Electric88/200/Grand80)", kinds[CandidateKindDedicated])
	}
	if kinds[CandidateKindSynth] != 1 {
		t.Fatalf("synth candidate missing: %+v", result.Candidates)
	}
	if len(result.PresetHints) != 1 || result.PresetHints[0].Category != "Keys" {
		t.Fatalf("preset hints = %+v, want one Keys hint", result.PresetHints)
	}
	examples := strings.Join(result.PresetHints[0].Examples, ",")
	if !strings.Contains(examples, "EP 1") || !strings.Contains(examples, "EP 2") {
		t.Fatalf("epiano hint examples = %q, want EP 1 and EP 2", examples)
	}
}

func TestMatchGMTokenAndName(t *testing.T) {
	installed := fixtureInstalled()
	index := fixtureIndex(t)

	byToken := Match("gm:82", installed, index)
	if byToken.Degrade != nil || byToken.Class != ClassLead || byToken.GMProgram == nil || byToken.GMProgram.Number != 82 {
		t.Fatalf("gm:82 = class %q program %+v degrade %+v", byToken.Class, byToken.GMProgram, byToken.Degrade)
	}
	if len(byToken.Candidates) != 1 || byToken.Candidates[0].Installed.Name != "Surge XT" {
		t.Fatalf("gm:82 candidates = %+v, want Surge XT only", byToken.Candidates)
	}

	byName := Match("Lead 2 (sawtooth)", installed, index)
	if byName.Degrade != nil || byName.Class != ClassLead || byName.GMProgram == nil || byName.GMProgram.Number != 82 {
		t.Fatalf("GM name match = class %q program %+v degrade %+v", byName.Class, byName.GMProgram, byName.Degrade)
	}
}

func TestMatchGMOutOfBandNumber(t *testing.T) {
	result := Match("gm:999", fixtureInstalled(), fixtureIndex(t))
	if result.Degrade == nil || result.Degrade.Code != DegradeNoClass {
		t.Fatalf("gm:999 degrade = %+v, want no_instrument_class", result.Degrade)
	}
}

func TestMatchDrumsHonestApproximation(t *testing.T) {
	result := Match("这首歌用鼓", fixtureInstalled(), fixtureIndex(t))
	if result.Degrade != nil {
		t.Fatalf("drums match degraded: %+v", result.Degrade)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].Installed.Name != "Surge XT" {
		t.Fatalf("drums candidates = %+v, want Surge XT only", result.Candidates)
	}
	if len(result.PresetHints) != 1 || !result.PresetHints[0].Approximate {
		t.Fatalf("drums hints = %+v, want one approximate Percussion hint", result.PresetHints)
	}
	if !strings.Contains(result.PresetHints[0].Note, "GM") {
		t.Fatalf("drums hint note must state the GM keying limitation: %q", result.PresetHints[0].Note)
	}
}

func TestMatchStringsDegradesHonestly(t *testing.T) {
	result := Match("弦乐铺一层", fixtureInstalled(), fixtureIndex(t))
	if result.Degrade == nil || result.Degrade.Code != DegradeNoCandidate {
		t.Fatalf("strings degrade = %+v, want no_installed_candidate", result.Degrade)
	}
	if len(result.Candidates) != 0 {
		t.Fatalf("strings must not invent candidates: %+v", result.Candidates)
	}
}

func TestMatchUnknownTextDegrades(t *testing.T) {
	for _, query := range []string{"水果沙拉", "", "   "} {
		result := Match(query, fixtureInstalled(), fixtureIndex(t))
		if result.Degrade == nil || result.Degrade.Code != DegradeNoClass {
			t.Fatalf("query %q degrade = %+v, want no_instrument_class", query, result.Degrade)
		}
	}
}

func TestMatchEmptyInstalledList(t *testing.T) {
	result := Match("钢琴", nil, fixtureIndex(t))
	if result.Degrade == nil || result.Degrade.Code != DegradeEmptyInstallList {
		t.Fatalf("empty installed degrade = %+v, want installed_instrument_list_empty", result.Degrade)
	}
}

func TestMatchSurgeIndexUnavailable(t *testing.T) {
	// A class that only has a preset face must degrade when the index is
	// missing, but a dedicated class (piano) still resolves.
	drums := Match("用鼓", fixtureInstalled(), FactoryIndex{})
	if drums.Degrade == nil || drums.Degrade.Code != DegradeNoSurgeIndex {
		t.Fatalf("drums without index degrade = %+v, want surge_factory_index_unavailable", drums.Degrade)
	}
	piano := Match("钢琴", fixtureInstalled(), FactoryIndex{})
	if piano.Degrade != nil || len(piano.Candidates) != 1 {
		t.Fatalf("piano without index = %+v %+v, want dedicated candidate only", piano.Degrade, piano.Candidates)
	}
}

func TestMatchBassDedicatedAndSynth(t *testing.T) {
	result := Match("贝斯", fixtureInstalled(), fixtureIndex(t))
	if result.Degrade != nil {
		t.Fatalf("bass match degraded: %+v", result.Degrade)
	}
	var dedicated int
	for _, candidate := range result.Candidates {
		if candidate.Kind == CandidateKindDedicated {
			dedicated++
		}
	}
	if dedicated != 2 {
		t.Fatalf("bass dedicated candidates = %d, want 2 (Fingers/Slapper)", dedicated)
	}
	if len(result.PresetHints) != 1 || result.PresetHints[0].Category != "Basses" {
		t.Fatalf("bass hints = %+v", result.PresetHints)
	}
}

func TestMatchExcludesSurgeEffectsSibling(t *testing.T) {
	installed := []InstalledInstrument{
		{Name: "Surge XT Effects", Identifier: "VST3-FX", IsInstrument: false},
		{Name: "Surge XT", Identifier: "VST3-Synth", IsInstrument: true},
	}
	result := Match("合成器", installed, fixtureIndex(t))
	if result.Degrade != nil {
		t.Fatalf("synth match degraded: %+v", result.Degrade)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].Installed.Name != "Surge XT" {
		t.Fatalf("synth candidates = %+v, want Surge XT (not the Effects sibling)", result.Candidates)
	}
}

func TestFromPluginSemanticsFilters(t *testing.T) {
	entries := []pluginsemantics.Entry{
		{Name: "Surge XT", IsInstrument: true, Identifier: "VST3-Surge"},
		{Name: "Surge XT Effects", IsInstrument: false, Identifier: "VST3-SurgeFX"},
		{Name: "Pro-Q 3", IsInstrument: false, Identifier: "VST3-ProQ3"},
	}
	installed := FromPluginSemantics(entries)
	if len(installed) != 1 || installed[0].Name != "Surge XT" {
		t.Fatalf("FromPluginSemantics = %+v, want only Surge XT", installed)
	}
}

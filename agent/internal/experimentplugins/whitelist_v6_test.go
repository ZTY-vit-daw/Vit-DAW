package experimentplugins

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"vit-daw-agent/internal/processorattestation"
)

// Schema v6 (FIX-PLUGIN-SELECT-1): every family value evolves from one object
// to a candidate entry array (entry shape identical to v5 field-for-field).
// The loader accepts v5 (single object normalizes to a single-element list)
// and v6, fails closed on unknown versions, and rejects duplicate identifiers
// inside one family so admission membership always resolves one entry.

func fixtureSecondStaticEQPlugin() StaticEQPlugin {
	return StaticEQPlugin{
		PluginName:       "Second EQ",
		Manufacturer:     "Other",
		Format:           "VST3",
		PluginIdentifier: "second-eq",
		PluginPath:       "/plugins/other-eq.vst3",
		Bands: []Band{
			{CenterHz: 120, GainParamIDCH1: "eq2_120_ch1", GainParamIDCH2: "eq2_120_ch2"},
			{CenterHz: 2500, GainParamIDCH1: "eq2_2500_ch1", GainParamIDCH2: "eq2_2500_ch2"},
		},
	}
}

// red ④ first half: a v5 file (single object) loads as a single-element
// candidate list with byte-identical entry content, and the canonical
// in-memory schema version is v6.
func TestLoadParsesV5SingleObjectAsSingleElementList(t *testing.T) {
	raw := `{"schema_version":"` + SchemaVersionV5 + `","static_eq":{` +
		`"plugin_name":"Example EQ","manufacturer":"Example","format":"VST3",` +
		`"plugin_identifier":"example-eq","plugin_path":"` + fixturePluginPath + `",` +
		`"bands":[{"center_hz":100,"gain_param_id_ch1":"gain_100_ch1","gain_param_id_ch2":"gain_100_ch2"},` +
		`{"center_hz":1000,"gain_param_id_ch1":"gain_1000_ch1","gain_param_id_ch2":"gain_1000_ch2"},` +
		`{"center_hz":10000,"gain_param_id_ch1":"gain_10000_ch1","gain_param_id_ch2":"gain_10000_ch2"}]}}`
	path := writeFixture(t, "whitelist_v5_single_object.json", raw)
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SchemaVersion != SchemaVersion {
		t.Fatalf("canonical schema version=%q want %q", loaded.SchemaVersion, SchemaVersion)
	}
	if len(loaded.StaticEQ) != 1 {
		t.Fatalf("v5 single object must load as one entry, got %d", len(loaded.StaticEQ))
	}
	if want := fixtureStaticEQPlugin(); !reflect.DeepEqual(loaded.StaticEQ[0], want) {
		t.Fatalf("v5 entry content drifted: got=%+v want=%+v", loaded.StaticEQ[0], want)
	}
}

// red ④ second half: a v5 file loads, marshals as v6, and that v6 file loads
// back to the semantically identical whitelist (round trip).
func TestLoadV5RoundTripsThroughV6(t *testing.T) {
	v5 := Whitelist{
		SchemaVersion: SchemaVersionV5,
		StaticEQ:      StaticEQPlugins{fixtureStaticEQPlugin()},
		DeEsser:       DeEsserPlugins{fixtureDeEsserPlugin()},
	}
	path := writeFixture(t, "whitelist_v5_roundtrip.json", marshalOrPanic(v5))
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.StaticEQ) != 1 || len(loaded.DeEsser) != 1 {
		t.Fatalf("v5 load must keep single-element lists: %+v", loaded)
	}
	again := writeFixture(t, "whitelist_v6_roundtrip.json", marshalOrPanic(loaded))
	reloaded, err := Load(again)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, reloaded) {
		t.Fatalf("v6 round trip drifted: got=%+v want=%+v", reloaded, loaded)
	}
	if reloaded.SchemaVersion != SchemaVersion {
		t.Fatalf("round-tripped schema version=%q want %q", reloaded.SchemaVersion, SchemaVersion)
	}
}

func TestLoadParsesV6CandidateLists(t *testing.T) {
	whitelist := Whitelist{
		SchemaVersion: SchemaVersion,
		StaticEQ:      StaticEQPlugins{fixtureStaticEQPlugin(), fixtureSecondStaticEQPlugin()},
	}
	path := writeFixture(t, "whitelist_v6_candidates.json", marshalOrPanic(whitelist))
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, whitelist) {
		t.Fatalf("loaded=%+v want=%+v", loaded, whitelist)
	}
}

func TestLoadRejectsUnknownSchemaVersionFailClosed(t *testing.T) {
	raw := `{"schema_version":"vit.free_state_experiment_plugins.v4","static_eq":[]}`
	path := writeFixture(t, "whitelist_v4_unknown.json", raw)
	_, err := Load(path)
	if err == nil {
		t.Fatal("unknown schema version was accepted")
	}
	if !strings.Contains(err.Error(), "schema_version") {
		t.Fatalf("fail-closed error should name schema_version: %v", err)
	}
}

func TestLoadRejectsDuplicateIdentifiersInOneFamily(t *testing.T) {
	first := fixtureStaticEQPlugin()
	second := fixtureSecondStaticEQPlugin()
	second.PluginIdentifier = first.PluginIdentifier
	whitelist := Whitelist{SchemaVersion: SchemaVersion, StaticEQ: StaticEQPlugins{first, second}}
	path := writeFixture(t, "whitelist_v6_duplicate_identifier.json", marshalOrPanic(whitelist))
	_, err := Load(path)
	if err == nil {
		t.Fatal("duplicate identifiers inside one family were accepted")
	}
	if !strings.Contains(err.Error(), "duplicate") || !strings.Contains(err.Error(), first.PluginIdentifier) {
		t.Fatalf("duplicate-identifier error should name the duplicated id: %v", err)
	}
}

func TestLoadRejectsInvalidEntriesInV6Lists(t *testing.T) {
	invalid := fixtureSecondStaticEQPlugin()
	invalid.Bands = nil
	whitelist := Whitelist{SchemaVersion: SchemaVersion, StaticEQ: StaticEQPlugins{fixtureStaticEQPlugin(), invalid}}
	path := writeFixture(t, "whitelist_v6_invalid_entry.json", marshalOrPanic(whitelist))
	if _, err := Load(path); err == nil {
		t.Fatal("invalid second entry in a v6 list was accepted")
	}
}

// ---- membership selection semantics -----------------------------------------

// red ⑤ core: one candidate without a pin resolves to the only entry (today's
// behavior preserved); a wrong pin keeps the historical exact-equality error.
func TestSelectSingleCandidateFamily(t *testing.T) {
	whitelist := Whitelist{SchemaVersion: SchemaVersion, StaticEQ: StaticEQPlugins{fixtureStaticEQPlugin()}}
	selected, err := whitelist.SelectStaticEQ("")
	if err != nil {
		t.Fatal(err)
	}
	if selected.PluginIdentifier != "example-eq" {
		t.Fatalf("no-pin single candidate must resolve to the only entry: %+v", selected)
	}
	if selected, err = whitelist.SelectStaticEQ("example-eq"); err != nil || selected.PluginIdentifier != "example-eq" {
		t.Fatalf("matching pin must resolve: %+v err=%v", selected, err)
	}
	_, err = whitelist.SelectStaticEQ("some-other")
	if err == nil || !strings.Contains(err.Error(), "pinned plugin_identifier") ||
		!strings.Contains(err.Error(), "whitelist admits") {
		t.Fatalf("wrong pin on a single-candidate family must keep the historical error class: %v", err)
	}
}

// red ① core: among multiple candidates the pinned member resolves to exactly
// the selected entry.
func TestSelectMultiCandidateFamilyRespectsPinnedChoice(t *testing.T) {
	whitelist := Whitelist{SchemaVersion: SchemaVersion, StaticEQ: StaticEQPlugins{fixtureStaticEQPlugin(), fixtureSecondStaticEQPlugin()}}
	selected, err := whitelist.SelectStaticEQ("second-eq")
	if err != nil {
		t.Fatal(err)
	}
	if selected.PluginName != "Second EQ" || selected.PluginPath != "/plugins/other-eq.vst3" ||
		len(selected.Bands) != 2 || selected.Bands[0].GainParamIDCH1 != "eq2_120_ch1" {
		t.Fatalf("pinned choice must bind to the selected entry: %+v", selected)
	}
}

// red ② core: a non-member pin is refused fail-closed naming the admitted set.
func TestSelectMultiCandidateFamilyRefusesNonMember(t *testing.T) {
	whitelist := Whitelist{SchemaVersion: SchemaVersion, StaticEQ: StaticEQPlugins{fixtureStaticEQPlugin(), fixtureSecondStaticEQPlugin()}}
	_, err := whitelist.SelectStaticEQ("intruder")
	if err == nil {
		t.Fatal("non-member pin was accepted")
	}
	if !strings.Contains(err.Error(), "whitelist admits") ||
		!strings.Contains(err.Error(), "example-eq") || !strings.Contains(err.Error(), "second-eq") {
		t.Fatalf("non-member refusal must name the admitted identifiers: %v", err)
	}
}

// More than one candidate without a pin is ambiguous and must fail closed
// instead of silently picking a default.
func TestSelectMultiCandidateFamilyRequiresPin(t *testing.T) {
	whitelist := Whitelist{SchemaVersion: SchemaVersion, StaticEQ: StaticEQPlugins{fixtureStaticEQPlugin(), fixtureSecondStaticEQPlugin()}}
	_, err := whitelist.SelectStaticEQ("")
	if err == nil {
		t.Fatal("ambiguous unpinned multi-candidate family was accepted")
	}
	if !strings.Contains(err.Error(), "requires a pinned plugin_identifier") {
		t.Fatalf("ambiguity refusal wording wrong: %v", err)
	}
}

func TestSelectUnconfiguredFamily(t *testing.T) {
	whitelist := Whitelist{SchemaVersion: SchemaVersion}
	if _, err := whitelist.SelectDeEsser(""); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("unconfigured family must keep its not-configured class: %v", err)
	}
}

// The admission predicate validates the SELECTED entry's subject, so a
// multi-candidate family with one promoted member still admits that member.
func TestValidateAdmissionRunsAgainstSelectedEntry(t *testing.T) {
	dir := t.TempDir()
	promotedPath := filepath.Join(dir, "Promoted EQ.vst3")
	if err := os.WriteFile(promotedPath, []byte("promoted-eq-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := processorattestation.FingerprintPath(promotedPath)
	if err != nil {
		t.Fatal(err)
	}
	promoted := fixtureStaticEQPlugin()
	promoted.PluginPath = promotedPath
	otherPath := filepath.Join(dir, "Second EQ.vst3")
	if err := os.WriteFile(otherPath, []byte("other-eq-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	other := fixtureSecondStaticEQPlugin()
	other.PluginPath = otherPath
	whitelist := Whitelist{SchemaVersion: SchemaVersion, StaticEQ: StaticEQPlugins{promoted, other}}
	library := promotedStaticEQLibrary(t, admissionSubject(promotedPath), fingerprint)
	if err := whitelist.ValidateStaticEQAdmission(library, "example-eq"); err != nil {
		t.Fatalf("selected promoted entry refused: %v", err)
	}
	if err := whitelist.ValidateStaticEQAdmission(library, "second-eq"); err == nil ||
		!strings.Contains(err.Error(), "not PCA-promoted") {
		t.Fatalf("selected non-promoted entry must be refused by the PCA admission check: %v", err)
	}
}

package agentprotocol

import (
	"strings"
	"testing"
)

// REFSCHEMA-L0-1 red-first table-driven tests. Grammar authority:
// coord/decisions/2026-09-27-g1-ref-schema-ruling.md (L0 文法定版要点).
// Registry initial values authority: coord/runs/L1-1-RECON-1/EVIDENCE_REFS_INVENTORY.md §2.

func withRefSchemaWarnLogger(t *testing.T, hook func(string)) {
	t.Helper()
	prev := RefSchemaWarnLogger
	RefSchemaWarnLogger = hook
	t.Cleanup(func() { RefSchemaWarnLogger = prev })
}

func mustParseRef(t *testing.T, raw string) ParsedRef {
	t.Helper()
	got, err := ParseRef(raw)
	if err != nil {
		t.Fatalf("ParseRef(%q) unexpected error: %v", raw, err)
	}
	if got.State != RefStateParsed {
		t.Fatalf("ParseRef(%q) state = %q, want %q", raw, got.State, RefStateParsed)
	}
	if got.Ref == nil {
		t.Fatalf("ParseRef(%q) parsed state has nil Ref", raw)
	}
	return got
}

func TestParseRefValidTable(t *testing.T) {
	cases := []struct {
		name       string
		raw        string
		kind       string
		scopeKind  string
		scopeValue string
		allTime    bool
		start      int64
		end        int64
		snapshot   string
		hash       string
	}{
		{
			name: "dom all-time un-CASed", raw: "vit://dom/track:voc_main/t=all@req_1#-",
			kind: "dom", scopeKind: "track", scopeValue: "voc_main",
			allTime: true, snapshot: "req_1", hash: "-",
		},
		{
			name: "fxm ranged with sha256", raw: "vit://fxm/track:voc/t=0..480000@rr_7#sha256:0123456789abcdef",
			kind: "fxm", scopeKind: "track", scopeValue: "voc",
			start: 0, end: 480000, snapshot: "rr_7", hash: "sha256:0123456789abcdef",
		},
		{
			name: "rlm project scope legacy snapshot id", raw: "vit://rlm/project:default/t=48000..96000@obs_20260921T120000_ab12cd34#sha256:ffffffffffffffff",
			kind: "rlm", scopeKind: "project", scopeValue: "default",
			start: 48000, end: 96000, snapshot: "obs_20260921T120000_ab12cd34", hash: "sha256:ffffffffffffffff",
		},
		{
			name: "escaped reserved chars in scope value", raw: "vit://com/track:a%2Fb%3Ac%40d%23e%25f/t=all@s#-",
			kind: "com", scopeKind: "track", scopeValue: "a/b:c@d#e%f",
			allTime: true, snapshot: "s", hash: "-",
		},
		{
			name: "escaped reserved chars in snapshot", raw: "vit://dom/track:x/t=all@r%40r%23r#-",
			kind: "dom", scopeKind: "track", scopeValue: "x",
			allTime: true, snapshot: "r@r#r", hash: "-",
		},
		{
			name: "unicode and space pass through verbatim", raw: "vit://dom/track:voc 主唱/t=all@s#-",
			kind: "dom", scopeKind: "track", scopeValue: "voc 主唱",
			allTime: true, snapshot: "s", hash: "-",
		},
		{
			name: "zero-length sample window is syntactic", raw: "vit://dom/track:x/t=0..0@s#-",
			kind: "dom", scopeKind: "track", scopeValue: "x",
			start: 0, end: 0, snapshot: "s", hash: "-",
		},
		{
			name: "already-escaped-looking literal stays literal", raw: "vit://dom/track:a%252Fb/t=all@s#-",
			kind: "dom", scopeKind: "track", scopeValue: "a%2Fb",
			allTime: true, snapshot: "s", hash: "-",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mustParseRef(t, tc.raw)
			r := got.Ref
			if r.Kind != tc.kind {
				t.Errorf("Kind = %q, want %q", r.Kind, tc.kind)
			}
			if r.ScopeKind != tc.scopeKind {
				t.Errorf("ScopeKind = %q, want %q", r.ScopeKind, tc.scopeKind)
			}
			if r.ScopeValue != tc.scopeValue {
				t.Errorf("ScopeValue = %q, want %q", r.ScopeValue, tc.scopeValue)
			}
			if r.Window == nil {
				t.Fatalf("Window = nil, want explicit window (ruling #2)")
			}
			if r.Window.AllTime != tc.allTime {
				t.Errorf("Window.AllTime = %v, want %v", r.Window.AllTime, tc.allTime)
			}
			if r.Window.SampleStart != tc.start || r.Window.SampleEnd != tc.end {
				t.Errorf("Window samples = %d..%d, want %d..%d", r.Window.SampleStart, r.Window.SampleEnd, tc.start, tc.end)
			}
			if r.Snapshot != tc.snapshot {
				t.Errorf("Snapshot = %q, want %q", r.Snapshot, tc.snapshot)
			}
			if r.Hash != tc.hash {
				t.Errorf("Hash = %q, want %q", r.Hash, tc.hash)
			}
			if got.Raw != tc.raw {
				t.Errorf("Raw = %q, want original %q", got.Raw, tc.raw)
			}
		})
	}
}

func TestParseRefRejectsTable(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"empty string", ""},
		{"missing window segment", "vit://dom/track:x@req_1#-"},
		{"missing hash segment", "vit://dom/track:x/t=all@req_1"},
		{"missing snapshot", "vit://dom/track:x/t=all@#-"},
		{"only kind and scope", "vit://dom/track:x"},
		{"bare scheme", "vit://"},
		{"raw @ inside scope makes window vanish", "vit://dom/track:a@b@snap#-"},
		{"bad escape percent not hex", "vit://dom/track:a%zz/t=all@s#-"},
		{"dangling percent", "vit://dom/track:a%/t=all@s#-"},
		{"truncated escape", "vit://dom/track:a%2/t=all@s#-"},
		{"raw reserved colon after structural split", "vit://dom/track:a:b/t=all@s#-"},
		{"window uppercase all", "vit://dom/track:x/t=ALL@s#-"},
		{"window non numeric", "vit://dom/track:x/t=1..s@s#-"},
		{"window empty", "vit://dom/track:x/t=@s#-"},
		{"window leading zero", "vit://dom/track:x/t=01..2@s#-"},
		{"window missing range", "vit://dom/track:x/t=1@s#-"},
		{"window all with suffix", "vit://dom/track:x/t=all @s#-"},
		{"hash short", "vit://dom/track:x/t=all@s#sha256:0123"},
		{"hash uppercase hex", "vit://dom/track:x/t=all@s#sha256:0123456789ABCDEF"},
		{"hash non hex", "vit://dom/track:x/t=all@s#sha256:0123456789abcdeg"},
		{"hash bare hex without scheme", "vit://dom/track:x/t=all@s#0123456789abcdef"},
		{"hash double dash", "vit://dom/track:x/t=all@s#--"},
		{"hash garbage", "vit://dom/track:x/t=all@s#garbage"},
		{"uppercase kind", "vit://DOM/track:x/t=all@s#-"},
		{"empty scope value", "vit://dom/track:/t=all@s#-"},
		{"empty scope kind", "vit://dom/:x/t=all@s#-"},
		{"empty scope both", "vit://dom//t=all@s#-"},
		{"extra path segment", "vit://dom/track:x/t=all/extra@s#-"},
		{"raw slash inside scope value", "vit://dom/track:a/b/t=all@s#-"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseRef(tc.raw)
			if err == nil {
				t.Fatalf("ParseRef(%q) expected grammar rejection, got %+v", tc.raw, got)
			}
		})
	}
}

func TestParseRefThreeStates(t *testing.T) {
	ResetOpaqueWarnState()
	withRefSchemaWarnLogger(t, nil)

	cases := []struct {
		name  string
		raw   string
		state RefState
	}{
		{"valid vit ref is parsed", "vit://dom/track:x/t=all@s#-", RefStateParsed},
		{"dom content id is legacy", "dom_0123456789abcdef0123", RefStateLegacy},
		{"fxm content id is legacy", "fxm_0123456789abcdef0123", RefStateLegacy},
		{"com content id is legacy", "com_0123456789abcdef0123", RefStateLegacy},
		{"rlm content id is legacy", "rlm_0123456789abcdef", RefStateLegacy},
		{"mixboard request id is legacy", "mixboard_20260921T120000.000000000", RefStateLegacy},
		{"kernel prepared request id is legacy", "kernel_prepared_band_energy_summary_clip_voc_01", RefStateLegacy},
		{"unregistered scheme head is opaque", "dad.l3.noise_floor", RefStateOpaque},
		{"colon scheme unregistered is opaque", "mix.read:track.1.fast.levels", RefStateOpaque},
		{"near miss prefix is opaque", "domX_0123456789abcdef0123", RefStateOpaque},
		{"unregistered vit kind is opaque", "vit://dad.l3/track:x/t=all@s#-", RefStateOpaque},
		{"foreign uri is opaque", "http://dom/track:x/t=all@s#-", RefStateOpaque},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseRef(tc.raw)
			if err != nil {
				t.Fatalf("ParseRef(%q) unexpected error: %v", tc.raw, err)
			}
			if got.State != tc.state {
				t.Fatalf("state = %q, want %q", got.State, tc.state)
			}
			if got.Raw != tc.raw {
				t.Errorf("Raw = %q, want passthrough %q", got.Raw, tc.raw)
			}
			switch tc.state {
			case RefStateParsed:
				if got.Ref == nil || got.Legacy != nil {
					t.Errorf("parsed state must fill Ref only, got %+v", got)
				}
			case RefStateLegacy:
				if got.Legacy == nil || got.Ref != nil {
					t.Errorf("legacy state must fill Legacy only, got %+v", got)
				}
			case RefStateOpaque:
				if got.Ref != nil || got.Legacy != nil {
					t.Errorf("opaque state must fill neither, got %+v", got)
				}
			}
		})
	}
}

func TestParseRefLegacyTranslationTable(t *testing.T) {
	cases := []struct {
		raw    string
		prefix string
		family string
		kind   string
		slot   string
		value  string
	}{
		{"dom_0123456789abcdef0123", "dom_", RefFamilyProjectionContentID, "dom", RefSlotHash, "0123456789abcdef0123"},
		{"fxm_0123456789abcdef0123", "fxm_", RefFamilyProjectionContentID, "fxm", RefSlotHash, "0123456789abcdef0123"},
		{"com_0123456789abcdef0123", "com_", RefFamilyProjectionContentID, "com", RefSlotHash, "0123456789abcdef0123"},
		{"rlm_0123456789abcdef", "rlm_", RefFamilyProjectionContentID, "rlm", RefSlotHash, "0123456789abcdef"},
		{"mixboard_20260921T120000.000000000", "mixboard_", RefFamilySnapshotRequestID, "", RefSlotSnapshot, "20260921T120000.000000000"},
		{"kernel_prepared_band_energy_summary_clip_voc_01", "kernel_prepared_", RefFamilySnapshotRequestID, "", RefSlotSnapshot, "band_energy_summary_clip_voc_01"},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := ParseRef(tc.raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Legacy == nil {
				t.Fatalf("legacy translation missing")
			}
			l := got.Legacy
			if l.LegacyPrefix != tc.prefix || l.Family != tc.family || l.TargetKind != tc.kind || l.Slot != tc.slot || l.Value != tc.value {
				t.Errorf("translation = %+v, want prefix=%q family=%q kind=%q slot=%q value=%q", l, tc.prefix, tc.family, tc.kind, tc.slot, tc.value)
			}
		})
	}
}

func TestRegistryInitialValues(t *testing.T) {
	want := []LegacyPrefixEntry{
		{LegacyPrefix: "dom_", Family: RefFamilyProjectionContentID, TargetKind: "dom", Slot: RefSlotHash, Anchor: "L1-1 §2 A1 dom/projection.go:530-537"},
		{LegacyPrefix: "fxm_", Family: RefFamilyProjectionContentID, TargetKind: "fxm", Slot: RefSlotHash, Anchor: "L1-1 §2 A2 fxm/projection.go:207-213"},
		{LegacyPrefix: "com_", Family: RefFamilyProjectionContentID, TargetKind: "com", Slot: RefSlotHash, Anchor: "L1-1 §2 A3 com/projection.go:467-473"},
		{LegacyPrefix: "rlm_", Family: RefFamilyProjectionContentID, TargetKind: "rlm", Slot: RefSlotHash, Anchor: "L1-1 §2 A4 rlm/projection.go:738-749"},
		{LegacyPrefix: "mixboard_", Family: RefFamilySnapshotRequestID, TargetKind: "", Slot: RefSlotSnapshot, Anchor: "L1-1 §2 G1 harness/harness.go:6848-6849"},
		{LegacyPrefix: "kernel_prepared_", Family: RefFamilySnapshotRequestID, TargetKind: "", Slot: RefSlotSnapshot, Anchor: "L1-1 §2 G2 harness/harness.go:4188-4200"},
	}
	got := LegacyPrefixRegistry()
	if len(got) != len(want) {
		t.Fatalf("registry size = %d, want %d (A 类四变体 + G 类前缀族)", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
	kinds := RegisteredRefKinds()
	wantKinds := []string{"dom", "fxm", "com", "rlm"}
	if len(kinds) != len(wantKinds) {
		t.Fatalf("kinds = %v, want %v", kinds, wantKinds)
	}
	for i := range wantKinds {
		if kinds[i] != wantKinds[i] {
			t.Errorf("kinds[%d] = %q, want %q", i, kinds[i], wantKinds[i])
		}
	}
	// Registry copy must not alias the internal table.
	got[0].LegacyPrefix = "mutated_"
	if LegacyPrefixRegistry()[0].LegacyPrefix != "dom_" {
		t.Errorf("LegacyPrefixRegistry() returns an aliased table")
	}
}

func TestFormatRefGolden(t *testing.T) {
	cases := []struct {
		name string
		ref  Ref
		want string
	}{
		{
			name: "all time un-CASed",
			ref: Ref{Kind: "dom", ScopeKind: "track", ScopeValue: "voc_main", Window: &TimeWindow{AllTime: true}, Snapshot: "req_1", Hash: "-"},
			want: "vit://dom/track:voc_main/t=all@req_1#-",
		},
		{
			name: "ranged with hash",
			ref: Ref{Kind: "fxm", ScopeKind: "track", ScopeValue: "voc", Window: &TimeWindow{SampleStart: 0, SampleEnd: 480000}, Snapshot: "rr_7", Hash: "sha256:0123456789abcdef"},
			want: "vit://fxm/track:voc/t=0..480000@rr_7#sha256:0123456789abcdef",
		},
		{
			name: "reserved chars escaped, space verbatim",
			ref: Ref{Kind: "dom", ScopeKind: "track", ScopeValue: "a/b:c@d#e%f g", Window: &TimeWindow{AllTime: true}, Snapshot: "r/r", Hash: "-"},
			want: "vit://dom/track:a%2Fb%3Ac%40d%23e%25f g/t=all@r%2Fr#-",
		},
		{
			name: "escape introducer escaped first",
			ref: Ref{Kind: "rlm", ScopeKind: "project", ScopeValue: "%2F", Window: &TimeWindow{AllTime: true}, Snapshot: "s", Hash: "-"},
			want: "vit://rlm/project:%252F/t=all@s#-",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FormatRef(tc.ref)
			if err != nil {
				t.Fatalf("FormatRef unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("FormatRef = %q, want %q", got, tc.want)
			}
			// Golden strings must also parse back to the same structure.
			parsed := mustParseRef(t, got)
			if parsed.Ref.Kind != tc.ref.Kind || parsed.Ref.ScopeKind != tc.ref.ScopeKind ||
				parsed.Ref.ScopeValue != tc.ref.ScopeValue || parsed.Ref.Snapshot != tc.ref.Snapshot ||
				parsed.Ref.Hash != tc.ref.Hash {
				t.Errorf("round trip mismatch: %+v vs %+v", parsed.Ref, tc.ref)
			}
		})
	}
}

func TestFormatRefRejects(t *testing.T) {
	valid := Ref{Kind: "dom", ScopeKind: "track", ScopeValue: "x", Window: &TimeWindow{AllTime: true}, Snapshot: "s", Hash: "-"}
	cases := []struct {
		name string
		mut  func(Ref) Ref
	}{
		{"empty kind", func(r Ref) Ref { r.Kind = ""; return r }},
		{"unregistered kind", func(r Ref) Ref { r.Kind = "dad.l3"; return r }},
		{"uppercase kind", func(r Ref) Ref { r.Kind = "Dom"; return r }},
		{"nil window", func(r Ref) Ref { r.Window = nil; return r }},
		{"empty scope kind", func(r Ref) Ref { r.ScopeKind = ""; return r }},
		{"empty scope value", func(r Ref) Ref { r.ScopeValue = ""; return r }},
		{"empty snapshot", func(r Ref) Ref { r.Snapshot = ""; return r }},
		{"empty hash", func(r Ref) Ref { r.Hash = ""; return r }},
		{"bare hex hash", func(r Ref) Ref { r.Hash = "0123456789abcdef"; return r }},
		{"uppercase hex hash", func(r Ref) Ref { r.Hash = "sha256:0123456789ABCDEF"; return r }},
		{"short hash", func(r Ref) Ref { r.Hash = "sha256:0123"; return r }},
		{"negative window bound", func(r Ref) Ref { r.Window = &TimeWindow{SampleStart: -1, SampleEnd: 4}; return r }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := FormatRef(tc.mut(valid)); err == nil {
				t.Fatalf("FormatRef expected rejection")
			}
			if err := tc.mut(valid).Validate(); err == nil {
				t.Errorf("Validate expected rejection")
			}
		})
	}
}

func TestRefEscapeRoundTrip(t *testing.T) {
	nasty := []string{
		"a", "a/b", "a:b", "a@b", "a#b", "a%b", "100%", "%", "//", "@@", "##",
		"a%2Fb", "vit://dom/track:x", "voc 主唱", "tab\tvalue", "%2f", "%2F", "%zz",
		"back\\slash", "a..b", "0123456789abcdef",
	}
	for _, s := range nasty {
		ref := Ref{Kind: "dom", ScopeKind: "track", ScopeValue: s, Window: &TimeWindow{AllTime: true}, Snapshot: s, Hash: "-"}
		formatted, err := FormatRef(ref)
		if err != nil {
			t.Fatalf("FormatRef scope=%q unexpected error: %v", s, err)
		}
		parsed := mustParseRef(t, formatted)
		if parsed.Ref.ScopeValue != s {
			t.Errorf("scope round trip: got %q, want %q (formatted %q)", parsed.Ref.ScopeValue, s, formatted)
		}
		if parsed.Ref.Snapshot != s {
			t.Errorf("snapshot round trip: got %q, want %q (formatted %q)", parsed.Ref.Snapshot, s, formatted)
		}
	}
}

func TestOpaqueWarnOnceAndCounts(t *testing.T) {
	ResetOpaqueWarnState()
	var lines []string
	withRefSchemaWarnLogger(t, func(line string) { lines = append(lines, line) })

	for i := 0; i < 3; i++ {
		got, err := ParseRef("dad.l3.noise_floor")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.State != RefStateOpaque {
			t.Fatalf("state = %q, want opaque", got.State)
		}
	}
	for i := 0; i < 2; i++ {
		if _, err := ParseRef("mix.read:track.1.fast.levels"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if _, err := ParseRef("vit://dad.l3/track:x/t=all@s#-"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// One WARN per opaque value, three distinct values seen.
	if len(lines) != 3 {
		t.Fatalf("WARN lines = %d, want 3 (once per distinct value): %v", len(lines), lines)
	}
	for _, line := range lines {
		if !strings.Contains(line, "[agentprotocol.refs]") || !strings.Contains(line, "state=opaque") {
			t.Errorf("WARN line missing markers: %q", line)
		}
	}
	counts := OpaqueWarnCounts()
	if got := counts["dad.l3.noise_floor"]; got != 3 {
		t.Errorf("count[dad.l3.noise_floor] = %d, want 3", got)
	}
	if got := counts["mix.read"]; got != 2 {
		t.Errorf("count[mix.read] = %d, want 2 (key = scheme head before first colon)", got)
	}
	// Snapshot must be a copy: mutating it must not affect internal state.
	counts["dad.l3.noise_floor"] = 999
	if OpaqueWarnCounts()["dad.l3.noise_floor"] != 3 {
		t.Errorf("OpaqueWarnCounts() returned an aliased map")
	}
}

func TestOpaqueNilLoggerStaysSilentAndCounts(t *testing.T) {
	ResetOpaqueWarnState()
	withRefSchemaWarnLogger(t, nil)
	got, err := ParseRef("project_package.project_stereo_spread")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.State != RefStateOpaque {
		t.Fatalf("state = %q, want opaque", got.State)
	}
	if OpaqueWarnCounts()["project_package.project_stereo_spread"] != 1 {
		t.Errorf("count must be recorded even when logger is nil")
	}
}

func TestParseRefConcurrentOpaqueCounting(t *testing.T) {
	ResetOpaqueWarnState()
	withRefSchemaWarnLogger(t, nil)
	done := make(chan struct{})
	for w := 0; w < 4; w++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for i := 0; i < 50; i++ {
				if _, err := ParseRef("dad.l2_render_probe:rr_7"); err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		}()
	}
	for w := 0; w < 4; w++ {
		<-done
	}
	if got := OpaqueWarnCounts()["dad.l2_render_probe"]; got != 200 {
		t.Errorf("concurrent count = %d, want 200", got)
	}
}

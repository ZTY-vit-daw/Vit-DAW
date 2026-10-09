package fxm

import (
	"encoding/hex"
	"strings"
	"testing"

	"vit-daw-agent/internal/agentprotocol"
)

// vitRefTestInput 构造 ready 态 A/B 观察输入（同现有测试的确定性口径）。
func vitRefTestInput(createdAt string) Input {
	window := MeasurementWindow{SourceRevision: "source-1", StartSeconds: 0, EndSeconds: 10, SampleRate: 48000, ChannelCount: 2}
	quality := QualityEvidence{Deterministic: true, LatencyCompensated: true, Nonzero: true, Coverage: 1}
	return Input{
		ObservationID: "obs-1", MixSessionID: "mix-1", CreatedAt: createdAt,
		TargetRef: map[string]any{"kind": "track", "id": "1007"},
		Chain:     ChainIdentity{ChainHash: "chain-1", Plugins: []PluginIdentity{{Order: 1, Name: "TDR Nova", ProbeID: "probe-nova"}}}, Conditions: window,
		Baseline:  Measurement{ID: "baseline-1", Stage: "bypass_chain", Status: "ready", SourceRevision: "source-1", Window: window, RMSDBFS: number(-18), PeakDBFS: number(-6), LatencySamples: integer(0), Bands: []BandMetric{{ID: "presence", MinHz: 2000, MaxHz: 6000, EnergyDB: -22}}, Quality: quality, EvidenceRefs: []string{"render:baseline"}},
		Processed: Measurement{ID: "processed-1", Stage: "processed_chain", Status: "ready", SourceRevision: "source-1", Window: window, RMSDBFS: number(-16.5), PeakDBFS: number(-4), LatencySamples: integer(0), Bands: []BandMetric{{ID: "presence", MinHz: 2000, MaxHz: 6000, EnergyDB: -19.4}}, Quality: quality, EvidenceRefs: []string{"render:processed"}},
		ProbeRefs: []string{"probe:probe-nova"},
	}
}

// TestProjectionIDVitRefForm REFSCHEMA-M3：构造器产出 vit://fxm 形态，
// ParseRef 落 parsed 态且各段承载核对；hash 段=种子前 16 hex；实例身份回归
// （F6 fxm=实例身份：GeneratedAt 参与 → 变化即 ID 变）；legacy 形态兼容读
// 回归（旧记录 fxm_<20hex> 经注册表 legacy 条目可解析）。
func TestProjectionIDVitRefForm(t *testing.T) {
	projection := Build(vitRefTestInput("2026-07-17T00:00:00Z"))
	if !strings.HasPrefix(projection.ProjectionID, "vit://fxm/") {
		t.Fatalf("projection id form = %q", projection.ProjectionID)
	}
	parsed, err := agentprotocol.ParseRef(projection.ProjectionID)
	if err != nil || parsed.State != agentprotocol.RefStateParsed {
		t.Fatalf("parse state = %v err=%v", parsed.State, err)
	}
	if parsed.Ref.Kind != "fxm" || parsed.Ref.ScopeKind != "track" || parsed.Ref.ScopeValue != "1007" || parsed.Ref.Snapshot != "obs-1" {
		t.Fatalf("segments = %+v", parsed.Ref)
	}
	if parsed.Ref.Window == nil || !parsed.Ref.Window.AllTime {
		t.Fatalf("window = %+v", parsed.Ref.Window)
	}
	sum := projectionSeedHash(projection)
	if want := "sha256:" + hex.EncodeToString(sum[:])[:16]; parsed.Ref.Hash != want {
		t.Fatalf("hash segment = %q want %q", parsed.Ref.Hash, want)
	}
	if regen := Build(vitRefTestInput("2027-01-01T00:00:00Z")); regen.ProjectionID == projection.ProjectionID {
		t.Fatalf("instance identity lost: GeneratedAt change must move the fxm id")
	}
	legacy, err := agentprotocol.ParseRef("fxm_" + strings.Repeat("b", 20))
	if err != nil || legacy.State != agentprotocol.RefStateLegacy {
		t.Fatalf("legacy parse state = %v err=%v", legacy.State, err)
	}
	if legacy.Legacy == nil || legacy.Legacy.TargetKind != "fxm" || legacy.Legacy.Slot != agentprotocol.RefSlotHash {
		t.Fatalf("legacy translation = %+v", legacy.Legacy)
	}
}

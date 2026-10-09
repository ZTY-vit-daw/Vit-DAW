package capabilitycontext

import (
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentprotocol"
)

// REFSCHEMA-M2X-1 红先行测试：B1 gain staging 的 mix.read: 数据键族 refs
// 迁 vit://mom 形态（scope=mix.read:<数据键>、snapshot=observation_id，与
// M2 mom 包消费面同型）；project.state / mix.observe 族不在本卡范围，legacy
// 字面量保留。

const gainStagingM2XObservationID = "obs_20261009_b1m2x"

func gainStagingM2XProjectState() map[string]any {
	return map[string]any{
		"tracks": []map[string]any{{
			"track_id":   "vocal_1",
			"track_name": "Vocal",
			"volume_db":  0.0,
			"clips": []map[string]any{{
				"clip_id":          "clip_vocal",
				"type":             "audio",
				"clip_gain_db":     0.0,
				"duration_seconds": 12.0,
			}},
		}},
	}
}

func gainStagingM2XMixResult(observationID string) map[string]any {
	observation := map[string]any{
		"target_ref": map[string]any{"kind": "project", "id": "current"},
		"project_package": map[string]any{
			"tracks": []map[string]any{{
				"track_id":    "vocal_1",
				"role_guess":  "vocal",
				"peak_dbfs":   -6.0,
				"rms_dbfs":    -20.0,
				"headroom_db": 6.0,
			}},
			"limitations": []any{"kernel_l3_loudness_summary_missing"},
		},
	}
	if observationID != "" {
		observation["observation_id"] = observationID
	}
	return map[string]any{"observation": observation}
}

func gainStagingM2XHasRef(refs []string, want string) bool {
	for _, ref := range refs {
		if ref == want {
			return true
		}
	}
	return false
}

// 迁移点 5（gain_staging.go addUnique 三条）：三条 mix.read: 全部产出
// vit://mom 黄金形态且 ParseRef RefStateParsed；同函数内 project.state 与
// mix.observe:<id> 族保留 legacy 字面量。
func TestGainStagingPackMixReadRefsMigratedToVitMom(t *testing.T) {
	pack := BuildGainStagingPack(GainStagingInput{
		UserIntent:     "检查 B1 gain staging",
		ProjectState:   gainStagingM2XProjectState(),
		MixObservation: gainStagingM2XMixResult(gainStagingM2XObservationID),
		GeneratedAt:    time.Date(2026, 10, 9, 1, 2, 3, 0, time.UTC),
	})
	wantRefs := []string{
		"vit://mom/mix.read:project.tracks.summary/t=all@" + gainStagingM2XObservationID + "#-",
		"vit://mom/mix.read:project.risks.headroom/t=all@" + gainStagingM2XObservationID + "#-",
		"vit://mom/mix.read:project.rankings.peak/t=all@" + gainStagingM2XObservationID + "#-",
	}
	for _, want := range wantRefs {
		if !gainStagingM2XHasRef(pack.EvidenceRefs, want) {
			t.Fatalf("pack evidence refs missing migrated %q: %#v", want, pack.EvidenceRefs)
		}
		parsed, err := agentprotocol.ParseRef(want)
		if err != nil {
			t.Fatalf("ParseRef(%q) error: %v", want, err)
		}
		if parsed.State != agentprotocol.RefStateParsed {
			t.Fatalf("ParseRef(%q) state = %v, want parsed", want, parsed.State)
		}
		if parsed.Ref.Kind != "mom" || parsed.Ref.ScopeKind != "mix.read" {
			t.Fatalf("ref %q segments = %+v, want kind mom scope_kind mix.read", want, parsed.Ref)
		}
		if parsed.Ref.Snapshot != gainStagingM2XObservationID {
			t.Fatalf("ref %q snapshot = %q, want observation id %q", want, parsed.Ref.Snapshot, gainStagingM2XObservationID)
		}
		if parsed.Ref.Window == nil || !parsed.Ref.Window.AllTime || parsed.Ref.Hash != "-" {
			t.Fatalf("ref %q window/hash = %+v/%q, want t=all and \"-\"", want, parsed.Ref.Window, parsed.Ref.Hash)
		}
	}
	if !gainStagingM2XHasRef(pack.EvidenceRefs, "project.state") {
		t.Fatalf("out-of-scope project.state literal must stay: %#v", pack.EvidenceRefs)
	}
	if !gainStagingM2XHasRef(pack.EvidenceRefs, "mix.observe:"+gainStagingM2XObservationID) {
		t.Fatalf("out-of-scope mix.observe literal must stay: %#v", pack.EvidenceRefs)
	}
	for _, ref := range pack.EvidenceRefs {
		if strings.HasPrefix(ref, "mix.read:") {
			t.Fatalf("pack evidence refs still carry legacy mix.read literal %q", ref)
		}
	}
}

// 身份族语义（M2 同款宁缺勿假）：观察包无 observation_id 时不再发无身份
// mix.read refs；bare "mix.observe" legacy 字面量保留。
func TestGainStagingMixReadRefsRequireObservationIdentity(t *testing.T) {
	pack := BuildGainStagingPack(GainStagingInput{
		UserIntent:     "检查 B1 gain staging",
		ProjectState:   gainStagingM2XProjectState(),
		MixObservation: gainStagingM2XMixResult(""),
		GeneratedAt:    time.Date(2026, 10, 9, 1, 2, 3, 0, time.UTC),
	})
	for _, ref := range pack.EvidenceRefs {
		if strings.HasPrefix(ref, "mix.read:") || strings.HasPrefix(ref, agentprotocol.RefSchemePrefix+"mom/mix.read:") {
			t.Fatalf("identity-less pack must not emit mix.read refs: %q", ref)
		}
	}
	if !gainStagingM2XHasRef(pack.EvidenceRefs, "mix.observe") {
		t.Fatalf("bare mix.observe literal must survive without observation identity: %#v", pack.EvidenceRefs)
	}
}

// legacy 兼容读回归：旧落盘 mix.read: 字面量保持宽容 opaque 透传，不因
// 迁移拒读（与 tim 侧同款回归钉）。
func TestGainStagingLegacyMixReadRefsRemainParseable(t *testing.T) {
	agentprotocol.ResetOpaqueWarnState()
	defer agentprotocol.ResetOpaqueWarnState()
	for _, legacy := range []string{
		"mix.read:project.tracks.summary",
		"mix.read:project.risks.headroom",
		"mix.read:project.rankings.peak",
	} {
		parsed, err := agentprotocol.ParseRef(legacy)
		if err != nil {
			t.Fatalf("legacy %q parse error: %v", legacy, err)
		}
		if parsed.State != agentprotocol.RefStateOpaque || parsed.Raw != legacy {
			t.Fatalf("legacy %q = state %v raw %q, want opaque passthrough", legacy, parsed.State, parsed.Raw)
		}
	}
}

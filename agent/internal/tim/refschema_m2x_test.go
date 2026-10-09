package tim

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/agentprotocol"
)

// REFSCHEMA-M2X-1 红先行测试：tim 侧 mix.read: 数据键族 refs 迁 vit://mom
// 形态（scope 承载数据键族、snapshot=observation_id，与 M2 mom 包 13 处
// 消费面同型）；dad: 族与 project.state: 族不在本卡范围，原样保留。

const m2xObservationID = "obs_20261009_m2x1"

func m2xHasRef(refs []string, want string) bool {
	for _, ref := range refs {
		if ref == want {
			return true
		}
	}
	return false
}

func m2xProjectPackage() map[string]any {
	return map[string]any{
		"track_count": 2,
		"tracks": []any{
			map[string]any{
				"track_id":   "wav_1",
				"track_name": "Wave Track",
				"clip_count": 1,
				"primary_clip": map[string]any{
					"clip_id":             "clip_w",
					"current_source_path": "D:\\Mix\\wave.wav",
					"length_seconds":      10.0,
				},
				"acoustic": map[string]any{"status": "ready", "peak_dbfs": -3.0, "rms_dbfs": -18.0},
			},
			map[string]any{
				"track_id":   "empty_1",
				"track_name": "Empty Track",
				"clip_count": 0,
			},
		},
	}
}

func m2xRequireParsedMomRef(t *testing.T, raw string) {
	t.Helper()
	parsed, err := agentprotocol.ParseRef(raw)
	if err != nil {
		t.Fatalf("ParseRef(%q) error: %v", raw, err)
	}
	if parsed.State != agentprotocol.RefStateParsed {
		t.Fatalf("ParseRef(%q) state = %v, want parsed", raw, parsed.State)
	}
	ref := parsed.Ref
	if ref.Kind != "mom" {
		t.Fatalf("ref %q kind = %q, want mom", raw, ref.Kind)
	}
	if ref.ScopeKind != "mix.read" {
		t.Fatalf("ref %q scope_kind = %q, want mix.read", raw, ref.ScopeKind)
	}
	if ref.Snapshot != m2xObservationID {
		t.Fatalf("ref %q snapshot = %q, want observation id %q", raw, ref.Snapshot, m2xObservationID)
	}
	if ref.Window == nil || !ref.Window.AllTime {
		t.Fatalf("ref %q window = %+v, want t=all", raw, ref.Window)
	}
	if ref.Hash != "-" {
		t.Fatalf("ref %q hash = %q, want explicit un-CASed \"-\"", raw, ref.Hash)
	}
	if reformatted, err := agentprotocol.FormatRef(*ref); err != nil || reformatted != raw {
		t.Fatalf("canonical round trip broke %q -> %q (err %v)", raw, reformatted, err)
	}
}

func m2xForbidLegacyMixReadFamily(t *testing.T, refs []string, surface string) {
	t.Helper()
	for _, ref := range refs {
		if strings.HasPrefix(ref, "mix.read:") {
			t.Fatalf("%s still carries legacy mix.read literal %q", surface, ref)
		}
		if strings.HasPrefix(ref, agentprotocol.RefSchemePrefix+"mom/mix.read:") {
			m2xRequireParsedMomRef(t, ref)
		}
	}
}

// 迁移点 2（projection.go EvidenceRefs 四条）：三条 mix.read: 全部产出
// vit://mom 黄金形态；dad:track_waveform_envelopes 不在本卡范围，legacy
// 字面量原样保留。
func TestProjectionEvidenceRefsMigratedToVitMom(t *testing.T) {
	proj := Build(Input{ObservationID: m2xObservationID, ProjectPackage: m2xProjectPackage()})
	wantRefs := []string{
		"vit://mom/mix.read:project.tracks.summary/t=all@" + m2xObservationID + "#-",
		"vit://mom/mix.read:project.acoustic.tracks/t=all@" + m2xObservationID + "#-",
		"vit://mom/mix.read:project.limitations/t=all@" + m2xObservationID + "#-",
	}
	for _, want := range wantRefs {
		if !m2xHasRef(proj.EvidenceRefs, want) {
			t.Fatalf("projection evidence refs missing migrated %q: %#v", want, proj.EvidenceRefs)
		}
		m2xRequireParsedMomRef(t, want)
	}
	if !m2xHasRef(proj.EvidenceRefs, "dad:track_waveform_envelopes") {
		t.Fatalf("out-of-scope dad: literal must stay: %#v", proj.EvidenceRefs)
	}
	m2xForbidLegacyMixReadFamily(t, proj.EvidenceRefs, "projection evidence refs")
}

// 迁移点 1+3（no_project_tracks issue 与轨道级 issues）：Issue.EvidenceRefs
// 同族同型迁移，ParseRef 全部 RefStateParsed。
func TestProjectionIssuesCarryVitMomRefs(t *testing.T) {
	proj := Build(Input{ObservationID: m2xObservationID, ProjectPackage: m2xProjectPackage()})
	var emptyTrackRefs []string
	for _, issue := range proj.Issues {
		if issue.Code == "empty_track" {
			emptyTrackRefs = issue.EvidenceRefs
		}
		m2xForbidLegacyMixReadFamily(t, issue.EvidenceRefs, "issue "+issue.Code)
	}
	wantSummary := "vit://mom/mix.read:project.tracks.summary/t=all@" + m2xObservationID + "#-"
	wantAcoustic := "vit://mom/mix.read:project.acoustic.tracks/t=all@" + m2xObservationID + "#-"
	if !m2xHasRef(emptyTrackRefs, wantSummary) || !m2xHasRef(emptyTrackRefs, wantAcoustic) {
		t.Fatalf("empty_track issue refs missing migrated forms: %#v", emptyTrackRefs)
	}

	noTracks := Build(Input{ObservationID: m2xObservationID, ProjectPackage: map[string]any{"tracks": []any{}}})
	foundNoTracks := false
	for _, issue := range noTracks.Issues {
		if issue.Code != "no_project_tracks" {
			continue
		}
		foundNoTracks = true
		if !m2xHasRef(issue.EvidenceRefs, wantSummary) {
			t.Fatalf("no_project_tracks issue refs missing migrated form: %#v", issue.EvidenceRefs)
		}
		m2xForbidLegacyMixReadFamily(t, issue.EvidenceRefs, "issue no_project_tracks")
	}
	if !foundNoTracks {
		t.Fatalf("no_project_tracks issue not emitted: %#v", noTracks.Issues)
	}
}

// 迁移点 4（asserter 变量块）：断言行 evidence refs 迁 vit://mom 且携带
// 同一 observation 身份；plugin_hygiene 的 project.state: 族不在本卡范围，
// legacy 字面量保留。
func TestAssertionRowsCarryVitMomRefs(t *testing.T) {
	proj := Build(Input{ObservationID: m2xObservationID, ProjectPackage: m2xProjectPackage()})
	sawMomRef := false
	sawHygieneLegacy := false
	for _, row := range proj.Assertions {
		for _, ref := range row.EvidenceRefs {
			if strings.HasPrefix(ref, "mix.read:") {
				t.Fatalf("assertion row %s/%s still carries legacy mix.read literal %q", row.Asserter, row.Check, ref)
			}
			if strings.HasPrefix(ref, agentprotocol.RefSchemePrefix+"mom/mix.read:") {
				sawMomRef = true
			}
			if ref == "project.state:plugin_list_hygiene" && row.Asserter == "plugin_hygiene" {
				sawHygieneLegacy = true
			}
		}
	}
	if !sawMomRef {
		t.Fatalf("assertion rows carry no vit://mom refs: %#v", proj.Assertions)
	}
	if !sawHygieneLegacy {
		t.Fatalf("plugin_hygiene rows lost the out-of-scope project.state: literal: %#v", proj.Assertions)
	}
}

// 身份族语义（M2 同款宁缺勿假）：无 ObservationID 的投影不再发无身份
// mix.read refs——vit://mom 文法不允许伪造 snapshot 段；dad: 字面量不受
// 身份缺失影响，原样保留。
func TestProjectionWithoutObservationIdentityOmitsMixReadRefs(t *testing.T) {
	proj := Build(Input{ProjectPackage: m2xProjectPackage()})
	for _, ref := range proj.EvidenceRefs {
		if strings.HasPrefix(ref, "mix.read:") || strings.HasPrefix(ref, "vit://mom/mix.read:") {
			t.Fatalf("identity-less projection must not emit mix.read refs: %q", ref)
		}
	}
	if !m2xHasRef(proj.EvidenceRefs, "dad:track_waveform_envelopes") {
		t.Fatalf("dad: literal must survive without observation identity: %#v", proj.EvidenceRefs)
	}
	for _, issue := range proj.Issues {
		for _, ref := range issue.EvidenceRefs {
			if strings.HasPrefix(ref, "mix.read:") || strings.HasPrefix(ref, "vit://mom/mix.read:") {
				t.Fatalf("identity-less issue %s must not emit mix.read refs: %q", issue.Code, ref)
			}
		}
	}
	for _, row := range proj.Assertions {
		for _, ref := range row.EvidenceRefs {
			if strings.HasPrefix(ref, "mix.read:") || strings.HasPrefix(ref, "vit://mom/mix.read:") {
				t.Fatalf("identity-less assertion row %s/%s must not emit mix.read refs: %q", row.Asserter, row.Check, ref)
			}
		}
	}
}

// legacy 兼容读回归（M2 同款）：旧落盘 mix.read: 字面量在 ParseRef 三态里
// 保持宽容 opaque 透传（注册表从未收录该前缀），不因迁移拒读。
func TestLegacyMixReadRefsRemainParseable(t *testing.T) {
	agentprotocol.ResetOpaqueWarnState()
	defer agentprotocol.ResetOpaqueWarnState()
	for _, legacy := range []string{
		"mix.read:project.tracks.summary",
		"mix.read:project.acoustic.tracks",
		"mix.read:project.limitations",
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

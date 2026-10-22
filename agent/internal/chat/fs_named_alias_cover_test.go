package chat

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/shadow"
)

// FS-NAMED-ALIAS-COVER-1 unit faces: the closed alias-table extension
// (钢琴/吉他/贝斯/弦乐/合成器), the 157 priority and ambiguity semantics
// preserved over the extended table (exact-name first, alias families, zero
// or multiple matches fail open), the legacy three families zero-regression,
// and the unresolved-naming disclosure surface (FS-NAMED-ALIAS-COVER-1 ③).
// Fixture track names mirror the 159 six-stem layout with synthetic ids
// (AGENTS §10 sealed discipline: no real project track names).

func fsNamedAliasSixStemFixtureProject() *shadow.Project {
	proj := shadow.New(nil)
	proj.Initialize(map[string]any{
		"project_uuid": "proj-fsnamed-alias-sixstem",
		"tracks": []any{
			map[string]any{"id": "9107", "name": "Fixture Bass", "is_audio_track": true},
			map[string]any{"id": "9112", "name": "Fixture Drums", "is_audio_track": true},
			map[string]any{"id": "9117", "name": "Fixture Guitar", "is_audio_track": true},
			map[string]any{"id": "9122", "name": "Fixture Other", "is_audio_track": true},
			map[string]any{"id": "9127", "name": "Fixture Piano", "is_audio_track": true},
			map[string]any{"id": "9132", "name": "Fixture Vocals", "is_audio_track": true},
		},
	})
	return proj
}

func fsNamedAliasSixStemServer() *Server {
	return &Server{harness: harness.NewWithSender(nil, fsNamedAliasSixStemFixtureProject(), nil)}
}

func fsNamedAliasPositiveServer() *Server {
	proj := shadow.New(nil)
	proj.Initialize(map[string]any{
		"project_uuid": "proj-fsnamed-alias-positive",
		"tracks": []any{
			map[string]any{"id": "9201", "name": "Fixture Strings Section", "is_audio_track": true},
			map[string]any{"id": "9202", "name": "Fixture Synth Pad", "is_audio_track": true},
			map[string]any{"id": "9203", "name": "Fixture Grand Piano", "is_audio_track": true},
			map[string]any{"id": "9204", "name": "Fixture Neutral", "is_audio_track": true},
		},
	})
	return &Server{harness: harness.NewWithSender(nil, proj, nil)}
}

func fsNamedAliasAmbiguityServer() *Server {
	proj := shadow.New(nil)
	proj.Initialize(map[string]any{
		"project_uuid": "proj-fsnamed-alias-ambiguity",
		"tracks": []any{
			map[string]any{"id": "9301", "name": "Fixture Synth Lead", "is_audio_track": true},
			map[string]any{"id": "9302", "name": "Fixture Synth Bass", "is_audio_track": true},
		},
	})
	return &Server{harness: harness.NewWithSender(nil, proj, nil)}
}

// TestFSNamedAliasCoverNewFamilyHits pins the five new families over the
// 159 six-stem mirror, including the exact 159 A2/A4 utterances that failed
// open pre-extension.
func TestFSNamedAliasCoverNewFamilyHits(t *testing.T) {
	server := fsNamedAliasSixStemServer()
	cases := []struct {
		name     string
		text     string
		expected string
	}{
		{"159 A2 replica piano naming", "请先观察工程，再给一个有界的小步改进建议：给钢琴加一点混响", "9127"},
		{"piano latin alias", "add a touch of reverb to the piano", "9127"},
		{"keyboard alias maps to piano keyword", "键盘能不能再亮一点", "9127"},
		{"159 A4 replica guitar naming", "请先观察工程，再给一个有界的小步改进建议：给这条吉他轨加一个轻微的延迟", "9117"},
		{"guitar latin alias", "the guitar needs a slight delay", "9117"},
		{"bass chinese alias", "把贝斯的低频收一收", "9107"},
		{"bass latin alias", "tighten up the bass line", "9107"},
		{"strings family without a family track fails open", "弦乐再亮一点", ""},
		{"synth family without a family track fails open", "合成器铺底压一点", ""},
		{"empty text fails open", "   ", ""},
	}
	for _, testCase := range cases {
		if got := server.resolveFreeStateUserNamedTarget(testCase.text); got != testCase.expected {
			t.Errorf("%s: resolve(%q) = %q, want %q", testCase.name, testCase.text, got, testCase.expected)
		}
	}
}

// TestFSNamedAliasCoverStringsSynthHits covers the families the six-stem
// mirror cannot (strings/synth tracks exist here), including latin aliases
// and keyword-side stems (grand piano).
func TestFSNamedAliasCoverStringsSynthHits(t *testing.T) {
	server := fsNamedAliasPositiveServer()
	cases := []struct {
		name     string
		text     string
		expected string
	}{
		{"strings chinese alias", "弦乐的空气感再多一点", "9201"},
		{"strings latin alias", "the strings are buried", "9201"},
		{"synth chinese alias", "合成器的铺底太厚了", "9202"},
		{"synth latin alias", "the synth pad is too loud", "9202"},
		{"keyboard alias over keyword-stemmed piano name", "键盘颗粒感清楚一点", "9203"},
		{"piano chinese alias over keyword-stemmed piano name", "钢琴再温润一点", "9203"},
	}
	for _, testCase := range cases {
		if got := server.resolveFreeStateUserNamedTarget(testCase.text); got != testCase.expected {
			t.Errorf("%s: resolve(%q) = %q, want %q", testCase.name, testCase.text, got, testCase.expected)
		}
	}
}

// TestFSNamedAliasCoverAmbiguityFailsOpen pins the 157 ambiguity semantics
// over the extended table: same-family multiplicity, cross-family double
// naming, and the multi-character Chinese alias rule (琴 never fires alone).
func TestFSNamedAliasCoverAmbiguityFailsOpen(t *testing.T) {
	ambiguity := fsNamedAliasAmbiguityServer()
	if got := ambiguity.resolveFreeStateUserNamedTarget("合成器整体压一点"); got != "" {
		t.Errorf("same-family synth ambiguity resolved: %q", got)
	}
	// The bass family stays precise inside a synth-prefixed track name.
	if got := ambiguity.resolveFreeStateUserNamedTarget("贝斯单独收一点"); got != "9302" {
		t.Errorf("bass family missed the synth-prefixed bass track: %q", got)
	}
	sixStem := fsNamedAliasSixStemServer()
	if got := sixStem.resolveFreeStateUserNamedTarget("人声和钢琴一起处理一下"); got != "" {
		t.Errorf("cross-family double naming resolved: %q", got)
	}
	if got := sixStem.resolveFreeStateUserNamedTarget("这首曲子的琴声真好听"); got != "" {
		t.Errorf("single-char 琴 fired a family alias: %q", got)
	}
	// The 157-era single-char protection keeps holding for the legacy families.
	if got := sixStem.resolveFreeStateUserNamedTarget("这个安排鼓励了我"); got != "" {
		t.Errorf("single-char 鼓 fired a family alias: %q", got)
	}
}

// TestFSNamedAliasCoverLegacyFamiliesZeroRegression re-pins the three 157
// families (人声/伴奏/鼓组) over the six-stem mirror: the extended table
// resolves them exactly as before.
func TestFSNamedAliasCoverLegacyFamiliesZeroRegression(t *testing.T) {
	server := fsNamedAliasSixStemServer()
	cases := []struct {
		name     string
		text     string
		expected string
	}{
		{"legacy vocals chinese alias", "人声有点闷，透一点", "9132"},
		{"legacy vocals latin alias", "the vocals need more air", "9132"},
		{"legacy drums unique hit", "鼓组能不能更紧一点", "9112"},
		{"legacy exact track-name containment", "把 Fixture Other 收一点", "9122"},
		{"legacy unresolvable text fails open", "整体响度再大一点", ""},
	}
	for _, testCase := range cases {
		if got := server.resolveFreeStateUserNamedTarget(testCase.text); got != testCase.expected {
			t.Errorf("%s: resolve(%q) = %q, want %q", testCase.name, testCase.text, got, testCase.expected)
		}
	}
	// Fail-open hardware unchanged: a server without a harness never guesses.
	if got := (&Server{}).resolveFreeStateUserNamedTarget("钢琴"); got != "" {
		t.Fatalf("nil-harness server resolved a target: %q", got)
	}
}

// TestFSNamedAliasCoverUnresolvedDisclosure drives the ③ surface end to end:
// prepare sets the turn-scoped marker when a closed-table alias failed open,
// the response wrapper appends the machine-owned line, rewraps dedupe, and
// prepare clears stale copies on every other turn (echo/snapshot protection).
func TestFSNamedAliasCoverUnresolvedDisclosure(t *testing.T) {
	server := fsNamedAliasSixStemServer()
	want := "指名轨『弦乐』未在轨集中识别，按自动选择处理。"
	prepared, ok := server.prepareFreeStateReasoningContext("conv-fsnac-d1", "给弦乐加一点混响", map[string]any{"free_state_diagnostic_only": true})
	if !ok {
		t.Fatalf("diagnostic-only prepare did not start a loop")
	}
	marker := firstMapFromAny(prepared[userNamedTargetUnresolvedContextKey])
	if len(marker) == 0 || firstStringFromMap(marker, "named_alias") != "弦乐" {
		t.Fatalf("unresolved marker missing or malformed: %+v", marker)
	}
	out := server.bindFreeStateContextToResponse(ChatResponse{ConversationID: "conv-fsnac-d1", Reply: "已按自动选择处理。"}, prepared)
	if !strings.Contains(out.Reply, want) {
		t.Fatalf("unresolved disclosure line missing: %q", out.Reply)
	}
	if firstStringFromMap(firstMapFromAny(out.WorkflowData[userNamedTargetUnresolvedContextKey]), "named_alias") != "弦乐" {
		t.Fatalf("structured unresolved marker missing on WorkflowData")
	}
	twice := server.bindFreeStateContextToResponse(out, prepared)
	if strings.Count(twice.Reply, want) != 1 {
		t.Fatalf("unresolved disclosure duplicated on rewrap: %q", twice.Reply)
	}
	// The empty-reply face degrades to the disclosure line alone.
	bare := server.bindFreeStateContextToResponse(ChatResponse{ConversationID: "conv-fsnac-d1"}, prepared)
	if bare.Reply != want {
		t.Fatalf("empty-reply disclosure drifted: %q", bare.Reply)
	}
}

// TestFSNamedAliasCoverUnresolvedMarkerWriteBarrier pins prepare as the
// single writer: an aliased-but-unresolvable turn sets the marker, an
// unrelated turn clears even a stale echoed copy, a resolved naming never
// sets it, and a sticky pointer from an earlier turn suppresses it (the
// naming stays bound, so no miss-disclosure may fire).
func TestFSNamedAliasCoverUnresolvedMarkerWriteBarrier(t *testing.T) {
	diagnostic := func() map[string]any { return map[string]any{"free_state_diagnostic_only": true} }
	staleEcho := func() map[string]any {
		return map[string]any{
			"free_state_diagnostic_only":        true,
			userNamedTargetUnresolvedContextKey: map[string]any{"named_alias": "钢琴", "applied_at": "stale_echo"},
		}
	}
	// Unrelated turn: a stale echoed marker must not survive prepare.
	fresh := fsNamedAliasSixStemServer()
	cleared, ok := fresh.prepareFreeStateReasoningContext("conv-fsnac-d2", "整体响度好一点", staleEcho())
	if !ok {
		t.Fatalf("unrelated-turn prepare did not start a loop")
	}
	if _, present := cleared[userNamedTargetUnresolvedContextKey]; present {
		t.Fatalf("stale echoed marker survived an unrelated prepare: %+v", cleared[userNamedTargetUnresolvedContextKey])
	}
	// Resolved naming: the resolution face owns the turn, no miss-disclosure.
	resolved := fsNamedAliasSixStemServer()
	resolvedCtx, ok := resolved.prepareFreeStateReasoningContext("conv-fsnac-d3", "人声有点闷，透一点", diagnostic())
	if !ok {
		t.Fatalf("resolved-turn prepare did not start a loop")
	}
	if _, present := resolvedCtx[userNamedTargetUnresolvedContextKey]; present {
		t.Fatalf("resolved naming produced an unresolved marker")
	}
	// Sticky pointer: an unresolvable follow-up naming keeps the earlier
	// binding, so the miss-disclosure must stay silent (declared boundary).
	sticky := fsNamedAliasSixStemServer()
	if _, ok := sticky.prepareFreeStateReasoningContext("conv-fsnac-d4", "人声有点闷，透一点", diagnostic()); !ok {
		t.Fatalf("sticky first prepare did not start a loop")
	}
	stickyCtx, ok := sticky.prepareFreeStateReasoningContext("conv-fsnac-d4", "给弦乐加一点混响", diagnostic())
	if !ok {
		t.Fatalf("sticky follow-up prepare failed")
	}
	if _, present := stickyCtx[userNamedTargetUnresolvedContextKey]; present {
		t.Fatalf("sticky pointer turn produced an unresolved marker")
	}
	// Without the marker the reply is untouched (fail-open).
	plain := fsNamedAliasSixStemServer()
	plainCtx, ok := plain.prepareFreeStateReasoningContext("conv-fsnac-d5", "整体响度好一点", diagnostic())
	if !ok {
		t.Fatalf("plain prepare did not start a loop")
	}
	plainOut := plain.bindFreeStateContextToResponse(ChatResponse{ConversationID: "conv-fsnac-d5", Reply: "普通回复"}, plainCtx)
	if strings.Contains(plainOut.Reply, "未在轨集中识别") {
		t.Fatalf("disclosure invented without an unresolved naming: %q", plainOut.Reply)
	}
	// Disclosure builder wording is a machine constant.
	if got := userNamedTargetUnresolvedDisclosure(map[string]any{"named_alias": "钢琴"}); got != "指名轨『钢琴』未在轨集中识别，按自动选择处理。" {
		t.Fatalf("disclosure wording drifted: %q", got)
	}
	if got := userNamedTargetUnresolvedDisclosure(map[string]any{}); got != "" {
		t.Fatalf("empty marker disclosed a line: %q", got)
	}
}

// TestFSNamedAliasCoverPrepareCapturesNewFamilyTarget mirrors the 157
// prepare-capture face for the extended families: the A2 utterance lands in
// loop.UserNamedTarget through the real prepare boundary.
func TestFSNamedAliasCoverPrepareCapturesNewFamilyTarget(t *testing.T) {
	server := fsNamedAliasSixStemServer()
	if _, ok := server.prepareFreeStateReasoningContext("conv-fsnac-p1", "给钢琴加一点混响", map[string]any{"free_state_diagnostic_only": true}); !ok {
		t.Fatalf("prepare did not start a loop")
	}
	loop, ok := server.freeStateLoop("conv-fsnac-p1")
	if !ok || loop.UserNamedTarget != "9127" {
		t.Fatalf("prepare did not capture the piano named target: ok=%v target=%q", ok, loop.UserNamedTarget)
	}
	if freeStateLoopMap(loop)["user_named_target"] != "9127" {
		t.Fatalf("transport loop map dropped user_named_target")
	}
}

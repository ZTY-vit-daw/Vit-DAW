package carriers

// L1-4-IMPL-B 四层载体 T-C1~C5（CONTEXT_LAYERING_V1_DESIGN §6.3）。
// 全部隔离工作区（t.TempDir()，AGENTS §10）；装配路径确定性（不注入
// 时钟——T-C5 前提）。红锚点（改造前可复现）：四层无载体——规则文本
// 是入口包内联常量、偏好/环境/账本无文件载体，PrefixService 报告只有
// 入口自己的稳定段；本测试组断言的层序/失配/链式/兼容/确定性面在
// 改造前均不存在（编译期即红）。

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/promptruntime"
	"vit-daw-agent/rules/ruleset"
)

var tcFixedTime = time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)

func tcBaseOptions(t *testing.T) Options {
	t.Helper()
	return Options{
		WorkspaceDir:    t.TempDir(),
		ProjectDir:      t.TempDir(),
		Family:          ruleset.FamilyChat,
		ModeInstruction: "Ask Vit is in default mode. Handle the current user turn.",
		CommandCatalog:  "- get_project_state: read DAW state\n- track.mute: mute a track",
		ExpectedUserID:  UserIDLocal,
		Now:             tcFixedTime,
	}
}

func tcAssemble(t *testing.T, service promptruntime.PrefixService, sessionKey string, opts Options) (promptruntime.Assembly, promptruntime.AssemblyReport, Bundle) {
	t.Helper()
	bundle := Assemble(context.Background(), opts)
	assembly, report, err := service.Assemble(context.Background(), promptruntime.PrefixRequest{
		AssemblyInput: promptruntime.AssemblyInput{SystemSections: bundle.Sections},
		SessionKey:    sessionKey,
		LayerStates:   bundle.LayerStates,
	})
	if err != nil {
		t.Fatalf("prefix assemble: %v", err)
	}
	return assembly, report, bundle
}

func tcRenderedLayerOrder(report promptruntime.AssemblyReport) []string {
	ids := []string{}
	for _, layer := range report.Layers {
		if layer.State == StateRendered {
			ids = append(ids, layer.LayerID)
		}
	}
	return ids
}

func tcSectionIDs(bundle Bundle) []string {
	ids := make([]string, 0, len(bundle.Sections))
	for _, section := range bundle.Sections {
		ids = append(ids, section.ID)
	}
	return ids
}

func tcWriteProfileFixture(t *testing.T, dir string) {
	t.Helper()
	err := AppendProfileEntry(dir, ProfileEntry{
		PrefID:       "p-ref-level",
		Statement:    "Mixing reference track stays within the -6 dB peak budget.",
		Class:        ProfileClassCore,
		EvidenceRefs: []string{"vit://rlm/project@snap1#-"},
		ConfirmedAt:  tcFixedTime,
		Source:       "d13-2026-10-07-1",
	})
	if err != nil {
		t.Fatalf("profile fixture: %v", err)
	}
}

func tcWriteLedgerFixture(t *testing.T, projectDir string) {
	t.Helper()
	for _, entry := range []struct{ kind, statement string }{
		{LedgerKindGoal, "Delivery target: streaming loudness profile."},
		{LedgerKindDecision, "Keep vocal fader at -12 dB for this project."},
	} {
		if _, err := AppendLedgerEntry(projectDir, entry.kind, "settled", entry.statement, []string{"vit://mom/mix@snap1#-"}, 0, tcFixedTime); err != nil {
			t.Fatalf("ledger fixture: %v", err)
		}
	}
}

func tcFullFixture(t *testing.T, opts Options) Options {
	t.Helper()
	tcWriteProfileFixture(t, opts.WorkspaceDir)
	tcWriteLedgerFixture(t, opts.ProjectDir)
	if _, err := AppendGenesis(opts.ProjectDir, GenesisFacts{
		TracksSummary:   []string{"3 tracks: Drums, Bass, Vox"},
		BusTopology:     "master <- Drums/Bass/Vox",
		DeliveryTargets: []string{"streaming -14 LUFS"},
	}, tcFixedTime); err != nil {
		t.Fatalf("genesis fixture: %v", err)
	}
	opts.EnvComponents = map[string]string{"os": "windows", "arch": "amd64"}
	opts.PluginSemanticRev = "rev-7"
	return opts
}

// TestLayerOrderFixedAndMissingLayersFailOpen（T-C1）：四层 Section 顺序
// 固定 rules→profile→env→ledger→目录；缺层 fail-open（absent 渲染跳过+
// 报告标注）；损坏层 corrupt+WARN 非静默。
func TestLayerOrderFixedAndMissingLayersFailOpen(t *testing.T) {
	service := promptruntime.NewPrefixService()
	opts := tcFullFixture(t, tcBaseOptions(t))
	_, report, bundle := tcAssemble(t, service, "tc1-full", opts)

	wantOrder := []string{LayerRules, LayerProfile, LayerEnv, LayerLedger, LayerCatalog}
	if got := tcSectionIDs(bundle); !equalStrings(got, wantOrder) {
		t.Fatalf("section order = %v, want %v", got, wantOrder)
	}
	if got := tcRenderedLayerOrder(report); !equalStrings(got, wantOrder) {
		t.Fatalf("rendered layer order = %v, want %v", got, wantOrder)
	}

	// 缺层 fail-open：旧工程包/旧会话体（无四层文件）=空层常态。
	emptyOpts := tcBaseOptions(t)
	emptyBundle := Assemble(context.Background(), emptyOpts)
	if got := tcSectionIDs(emptyBundle); !equalStrings(got, []string{LayerRules, LayerCatalog}) {
		t.Fatalf("empty-workspace section order = %v, want [rules catalog]", got)
	}
	for _, layer := range []string{LayerProfile, LayerEnv, LayerLedger} {
		if emptyBundle.LayerStates[layer] != StateAbsent {
			t.Fatalf("layer %s state = %q, want absent", layer, emptyBundle.LayerStates[layer])
		}
	}
	if len(emptyBundle.Warnings) != 0 {
		t.Fatalf("absent layers must not warn (old project is the normal path): %v", emptyBundle.Warnings)
	}
	_, emptyReport, _ := tcAssemble(t, service, "tc1-empty", emptyOpts)
	states := map[string]string{}
	for _, layer := range emptyReport.Layers {
		states[layer.LayerID] = layer.State
	}
	for _, layer := range []string{LayerProfile, LayerEnv, LayerLedger} {
		if states[layer] != StateAbsent {
			t.Fatalf("report row %s = %q, want absent", layer, states[layer])
		}
	}

	// 损坏层：corrupt + WARN 不静默吞。
	corruptOpts := tcBaseOptions(t)
	if err := os.WriteFile(filepath.Join(corruptOpts.WorkspaceDir, ProfileFilename), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("corrupt profile fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(corruptOpts.WorkspaceDir, EnvFilename), []byte("[]"), 0o644); err != nil {
		t.Fatalf("corrupt env fixture: %v", err)
	}
	corruptBundle := Assemble(context.Background(), corruptOpts)
	for _, layer := range []string{LayerProfile, LayerEnv} {
		if corruptBundle.LayerStates[layer] != StateCorrupt {
			t.Fatalf("corrupt layer %s state = %q", layer, corruptBundle.LayerStates[layer])
		}
	}
	joined := strings.Join(corruptBundle.Warnings, "\n")
	if !strings.Contains(joined, "profile layer corrupt") || !strings.Contains(joined, "env layer corrupt") {
		t.Fatalf("corrupt layers must WARN explicitly, got %v", corruptBundle.Warnings)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}

// TestAggregationKeyMismatchExplicitBreak（T-C2）：user_id 失配 / env 指纹
// 失配 → 显式断裂（profile_updated / env_changed）+ L3 降权重建非静默沿用。
func TestAggregationKeyMismatchExplicitBreak(t *testing.T) {
	service := promptruntime.NewPrefixService()

	// L2 聚合键失配：turn1 正常渲染 → turn2 换 user_id=other 的档。
	opts := tcBaseOptions(t)
	tcWriteProfileFixture(t, opts.WorkspaceDir)
	_, firstReport, _ := tcAssemble(t, service, "tc2-profile", opts)
	if firstReport.Layers[0].LayerID != LayerRules {
		t.Fatalf("unexpected first layer: %+v", firstReport.Layers[0])
	}
	profileRendered := false
	for _, layer := range firstReport.Layers {
		if layer.LayerID == LayerProfile && layer.State == StateRendered {
			profileRendered = true
		}
	}
	if !profileRendered {
		t.Fatalf("turn 1 must render the profile layer: %+v", firstReport.Layers)
	}

	other := &ProfileDoc{SchemaVersion: "user_profile.v1", UserID: "someone-else", Entries: []ProfileEntry{{
		PrefID: "p-other", Statement: "Someone else's preference.", Class: ProfileClassCore,
		EvidenceRefs: []string{"vit://mom/mix@snap#-"}, Status: ProfileStatusActive,
		ConfirmedAt: tcFixedTime, Source: "d13-other",
	}}}
	if err := SaveProfile(opts.WorkspaceDir, other); err != nil {
		t.Fatalf("rewrite profile: %v", err)
	}
	_, secondReport, secondBundle := tcAssemble(t, service, "tc2-profile", opts)
	if secondBundle.LayerStates[LayerProfile] != StateCorrupt {
		t.Fatalf("user_id mismatch must keep the profile layer unrendered, got %q", secondBundle.LayerStates[LayerProfile])
	}
	joined := strings.Join(secondBundle.Warnings, "\n")
	if !strings.Contains(joined, "user_id mismatch") || !strings.Contains(joined, "not silently reused") {
		t.Fatalf("mismatch must WARN explicitly: %v", secondBundle.Warnings)
	}
	breakSeen := false
	for _, event := range secondReport.Breaks {
		if event.LayerID == LayerProfile && event.Reason == promptruntime.BreakProfileUpdated {
			breakSeen = true
		}
	}
	if !breakSeen {
		t.Fatalf("profile_updated break missing on user_id mismatch: %+v", secondReport.Breaks)
	}

	// L3 指纹失配：turn1 建卡（带 verified 蒸馏行）→ turn2 换分量 →
	// 旧卡 superseded 留档+新卡建立+蒸馏行降权+env_changed 断裂。
	envOpts := tcBaseOptions(t)
	envOpts.EnvComponents = map[string]string{"os": "windows", "arch": "amd64"}
	envOpts.PluginSemanticRev = "rev-7"
	if _, _, _, err := EnsureEnvCard(envOpts.WorkspaceDir, envOpts.EnvComponents, envOpts.PluginSemanticRev, tcFixedTime); err != nil {
		t.Fatalf("ensure env card: %v", err)
	}
	// 手工给当前卡挂一条 verified 蒸馏行（写入器 API 的最小等价物）。
	state := LoadEnvCard(envOpts.WorkspaceDir)
	if state.current == nil {
		t.Fatalf("env card missing after ensure")
	}
	state.current.DistilledNotes = []EnvNote{{
		NoteID: "n-clock", Statement: "Interface clock drifts after sleep; re-verify.",
		EvidenceRefs: []string{"vit://tom/project@snap#-"}, VerifiedAt: tcFixedTime, Status: EnvNoteVerified,
	}}
	if err := SaveEnvCard(envOpts.WorkspaceDir, state.doc); err != nil {
		t.Fatalf("save env card with note: %v", err)
	}
	_, _, _ = tcAssemble(t, service, "tc2-env", envOpts)
	envOpts.EnvComponents = map[string]string{"os": "windows", "arch": "arm64"}
	_, envSecond, envBundle := tcAssemble(t, service, "tc2-env", envOpts)
	if !envBundle.EnvChanged {
		t.Fatalf("fingerprint mismatch must set EnvChanged")
	}
	envBreak := false
	for _, event := range envSecond.Breaks {
		if event.LayerID == LayerEnv && event.Reason == promptruntime.BreakEnvChanged {
			envBreak = true
		}
	}
	if !envBreak {
		t.Fatalf("env_changed break missing on fingerprint mismatch: %+v", envSecond.Breaks)
	}
	if !strings.Contains(strings.Join(envBundle.Warnings, "\n"), "superseded") {
		t.Fatalf("rebuild must WARN (not silent): %v", envBundle.Warnings)
	}
	reloaded := LoadEnvCard(envOpts.WorkspaceDir)
	if len(reloaded.doc.Cards) != 2 || reloaded.doc.Cards[0].SupersededBy == "" {
		t.Fatalf("old card must stay archived with superseded_by link: %+v", reloaded.doc.Cards)
	}
	if reloaded.current == nil || len(reloaded.current.DistilledNotes) != 1 ||
		reloaded.current.DistilledNotes[0].Status != EnvNoteDownweighted {
		t.Fatalf("distilled note must carry over downweighted (not silently verified): %+v", reloaded.current.DistilledNotes)
	}
}

// TestLedgerChainTamperDetection（T-C3）：构造 prev_hash 链，篡改中段
// 条目 → 校验器检出（chain_mismatch）；未知 Kind fail-closed 拒载。
func TestLedgerChainTamperDetection(t *testing.T) {
	projectDir := t.TempDir()
	tcWriteLedgerFixture(t, projectDir)
	if _, err := AppendLedgerEntry(projectDir, LedgerKindConstraint, "", "No clip gain for mix moves.", nil, 0, tcFixedTime); err != nil {
		t.Fatalf("third entry: %v", err)
	}
	entries, err := ReadLedger(projectDir)
	if err != nil {
		t.Fatalf("clean read: %v", err)
	}
	if len(entries) < 3 {
		t.Fatalf("fixture too small: %d entries", len(entries))
	}

	// 篡改中段：改写第 2 条 statement（保留 prev_hash）——改写前缀即
	// 红测素材（卡面约束）。
	path := ledgerPath(projectDir)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	var tampered LedgerEntry
	if err := json.Unmarshal([]byte(lines[1]), &tampered); err != nil {
		t.Fatalf("parse middle entry: %v", err)
	}
	tampered.Statement = "TAMPERED: rewrite history in the middle"
	encoded, err := json.Marshal(tampered)
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	lines[1] = string(encoded)
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("rewrite ledger: %v", err)
	}
	_, err = ReadLedger(projectDir)
	var ledgerErr *LedgerError
	if !errors.As(err, &ledgerErr) {
		t.Fatalf("tampered ledger must fail with *LedgerError, got %v", err)
	}
	if ledgerErr.Kind != "chain_mismatch" || ledgerErr.EntryIndex != 3 {
		t.Fatalf("expected chain_mismatch detected at entry #3, got %+v", ledgerErr)
	}

	// 未知 Kind：fail-closed 拒载+类型化错误（§8.3）。
	otherDir := t.TempDir()
	if _, err := AppendLedgerEntry(otherDir, "mixing_advice", "", "bass too loud in trap", nil, 0, tcFixedTime); err == nil {
		t.Fatalf("unknown kind must be refused at write time")
	}
	badLine := map[string]any{
		"entry_id": 1, "prev_hash": "", "kind": "mixing_advice",
		"statement": "domain guidance smuggled into the ledger", "created_at": tcFixedTime.Format(time.RFC3339Nano),
	}
	bad, _ := json.Marshal(badLine)
	if err := os.MkdirAll(filepath.Dir(ledgerPath(otherDir)), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(ledgerPath(otherDir), append(bad, '\n'), 0o644); err != nil {
		t.Fatalf("write bad ledger: %v", err)
	}
	_, err = ReadLedger(otherDir)
	if !errors.As(err, &ledgerErr) || ledgerErr.Kind != "unknown_kind" {
		t.Fatalf("unknown kind must be refused at load time with unknown_kind, got %v", err)
	}
}

// TestOldProjectCompatRoundtripAndSchemaFreeze（T-C4）：旧工程包/旧会话体
// 无四层文件加载零报错；写入器往返一致；既有持久化 schema 零变更
// （载体只写三个新文件，工程/会话既有文件面零触碰）。
func TestOldProjectCompatRoundtripAndSchemaFreeze(t *testing.T) {
	opts := tcBaseOptions(t)

	// 旧工程包：无四层文件 → 零报错（无 corrupt、无 WARN）。
	legacyBundle := Assemble(context.Background(), opts)
	legacyStates := map[string]string{}
	for id, state := range legacyBundle.LayerStates {
		legacyStates[id] = state
		if state != StateAbsent {
			t.Fatalf("old project layer %s state = %q, want absent", id, state)
		}
	}
	if len(legacyBundle.Warnings) != 0 {
		t.Fatalf("old project must load with zero warnings: %v", legacyBundle.Warnings)
	}

	// 往返：L2 追加+推翻、L3 失配重建、L4 追加+撤销，重载语义一致。
	tcWriteProfileFixture(t, opts.WorkspaceDir)
	err := SupersedeProfileEntry(opts.WorkspaceDir, "p-ref-level", ProfileEntry{
		PrefID: "p-ref-level-2", Statement: "Reference track peak budget loosened to -3 dB.",
		Class: ProfileClassConditional, Condition: "genre=trap",
		EvidenceRefs: []string{"vit://rlm/project@snap2#-"}, ConfirmedAt: tcFixedTime, Source: "d13-2026-10-07-2",
	})
	if err != nil {
		t.Fatalf("supersede profile: %v", err)
	}
	profile := LoadProfile(opts.WorkspaceDir, UserIDLocal)
	if profile.state != StateRendered || len(profile.doc.Entries) != 2 {
		t.Fatalf("profile roundtrip: state=%s entries=%d", profile.state, len(profile.doc.Entries))
	}
	active := profile.doc.ActiveEntries()
	if len(active) != 1 || active[0].PrefID != "p-ref-level-2" || active[0].Class != ProfileClassConditional {
		t.Fatalf("active set after supersede = %+v", active)
	}
	old := profile.doc.Entries[0]
	if old.Status != ProfileStatusSuperseded || old.SupersededBy != "p-ref-level-2" {
		t.Fatalf("superseded chain broken: %+v", old)
	}

	if _, err := AppendGenesis(opts.ProjectDir, GenesisFacts{
		TracksSummary:   []string{"3 tracks: Drums, Bass, Vox"},
		BusTopology:     "master <- Drums/Bass/Vox",
		DeliveryTargets: []string{"streaming -14 LUFS"},
	}, tcFixedTime); err != nil {
		t.Fatalf("genesis: %v", err)
	}
	tcWriteLedgerFixture(t, opts.ProjectDir)
	if _, err := AppendLedgerEntry(opts.ProjectDir, LedgerKindUserOverride, "override", "User keeps bass fader manual.", nil, 2, tcFixedTime); err != nil {
		t.Fatalf("override entry: %v", err)
	}
	entries, err := ReadLedger(opts.ProjectDir)
	if err != nil {
		t.Fatalf("ledger roundtrip: %v", err)
	}
	if len(entries) != 6 || entries[5].Supersedes != 2 || entries[5].Kind != LedgerKindUserOverride {
		t.Fatalf("ledger roundtrip entries = %+v", entries)
	}
	rendered := RenderLedger(entries)
	if !strings.Contains(rendered, "supersedes #2") || !strings.Contains(rendered, "3 tracks") {
		t.Fatalf("ledger render missing supersede/genesis rows: %s", rendered)
	}

	opts.EnvComponents = map[string]string{"os": "windows"}
	if _, _, _, err := EnsureEnvCard(opts.WorkspaceDir, opts.EnvComponents, "rev-1", tcFixedTime); err != nil {
		t.Fatalf("ensure env: %v", err)
	}

	// 既有 schema 零变更：载体只写三个新文件，工作区/工程包无其他落盘。
	workspaceFiles, err := os.ReadDir(opts.WorkspaceDir)
	if err != nil {
		t.Fatalf("list workspace: %v", err)
	}
	names := []string{}
	for _, file := range workspaceFiles {
		names = append(names, file.Name())
	}
	sort.Strings(names)
	if !equalStrings(names, []string{EnvFilename, ProfileFilename}) {
		t.Fatalf("workspace must contain exactly the two carrier files, got %v", names)
	}
	var projectFiles []string
	err = filepath.WalkDir(opts.ProjectDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, relErr := filepath.Rel(opts.ProjectDir, path)
		if relErr != nil {
			return relErr
		}
		projectFiles = append(projectFiles, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walk project: %v", err)
	}
	if len(projectFiles) != 1 || projectFiles[0] != "ledger/project_ledger.v1.jsonl" {
		t.Fatalf("project dir must contain exactly the ledger file, got %v", projectFiles)
	}
}

// TestSameInputAssemblyDeterministic100x（T-C5）：同输入装配 ×100，
// PrefixFingerprint 与全部层 content_hash 全等（前缀字节承诺的前提）。
func TestSameInputAssemblyDeterministic100x(t *testing.T) {
	opts := tcFullFixture(t, tcBaseOptions(t))
	service := promptruntime.NewPrefixService()
	first := ""
	layerHashes := map[string]string{}
	for run := 0; run < 100; run++ {
		_, report, _ := tcAssemble(t, service, "tc5", opts)
		if report.PrefixFingerprint == "" {
			t.Fatalf("run %d: empty fingerprint", run)
		}
		if first == "" {
			first = report.PrefixFingerprint
			for _, layer := range report.Layers {
				layerHashes[layer.LayerID] = layer.ContentHash
			}
			continue
		}
		if report.PrefixFingerprint != first {
			t.Fatalf("run %d: fingerprint %s != %s", run, report.PrefixFingerprint, first)
		}
		for _, layer := range report.Layers {
			if layerHashes[layer.LayerID] != layer.ContentHash {
				t.Fatalf("run %d: layer %s content_hash drifted", run, layer.LayerID)
			}
		}
	}
}

// TestGenesisRendersOnceAndTopoDeltaKind：genesis 头部段只渲染一次
// （幂等），条目 Kind=topology_delta 族；后续拓扑变化走追加 delta。
func TestGenesisRendersOnceAndTopoDeltaKind(t *testing.T) {
	projectDir := t.TempDir()
	appended, err := AppendGenesis(projectDir, GenesisFacts{
		TracksSummary:   []string{"track A", "track B"},
		BusTopology:     "master <- A/B",
		DeliveryTargets: []string{"streaming"},
	}, tcFixedTime)
	if err != nil || appended != 3 {
		t.Fatalf("genesis first pass: appended=%d err=%v", appended, err)
	}
	again, err := AppendGenesis(projectDir, GenesisFacts{TracksSummary: []string{"changed"}}, tcFixedTime)
	if err != nil || again != 0 {
		t.Fatalf("genesis must be idempotent: appended=%d err=%v", again, err)
	}
	entries, err := ReadLedger(projectDir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	for _, entry := range entries {
		if entry.Kind != LedgerKindTopologyDelta || entry.Phase != "genesis" {
			t.Fatalf("genesis entries must be topology_delta/genesis: %+v", entry)
		}
	}
	if !strings.Contains(entries[0].Statement, "tom_overview: track A | track B") {
		t.Fatalf("genesis tom_overview row malformed: %s", entries[0].Statement)
	}
	delta, err := AppendLedgerEntry(projectDir, LedgerKindTopologyDelta, "", "track C added", nil, 0, tcFixedTime)
	if err != nil || delta.EntryID != 4 {
		t.Fatalf("post-genesis delta append: %+v err=%v", delta, err)
	}
}

// TestProfileWriterRequiresAttestation（§8.1 L2 行）：无确权来源不入档。
func TestProfileWriterRequiresAttestation(t *testing.T) {
	dir := t.TempDir()
	err := AppendProfileEntry(dir, ProfileEntry{
		PrefID: "p-no-attest", Statement: "Preference without attestation.", Class: ProfileClassCore,
		ConfirmedAt: tcFixedTime,
	})
	if err == nil || !strings.Contains(err.Error(), "no attestation") {
		t.Fatalf("unattested entry must be refused, got %v", err)
	}
	err = AppendProfileEntry(dir, ProfileEntry{
		PrefID: "p-enum", Statement: "Bad class.", Class: "vibe",
		EvidenceRefs: []string{"vit://mom/mix@s#-"}, ConfirmedAt: tcFixedTime, Source: "d13-1",
	})
	if err == nil || !strings.Contains(err.Error(), "closed enum") {
		t.Fatalf("closed-enum class must be refused, got %v", err)
	}
}

// TestEnvNoteStatusClosedEnumFailClosed：蒸馏行 Status 越界=拒载该行+WARN。
func TestEnvNoteStatusClosedEnumFailClosed(t *testing.T) {
	dir := t.TempDir()
	doc := &EnvDoc{SchemaVersion: "env_instance.v1", Cards: []EnvInstanceCard{{
		InstanceID: "deadbeef", CreatedAt: tcFixedTime, Components: map[string]string{"os": "windows"},
		DistilledNotes: []EnvNote{
			{NoteID: "n-1", Statement: "good note", EvidenceRefs: []string{"vit://tom/p@s#-"}, VerifiedAt: tcFixedTime, Status: EnvNoteVerified},
			{NoteID: "n-2", Statement: "bogus status", EvidenceRefs: []string{"vit://tom/p@s#-"}, VerifiedAt: tcFixedTime, Status: "maybe"},
		},
	}}}
	if err := SaveEnvCard(dir, doc); err != nil {
		t.Fatalf("save: %v", err)
	}
	state := LoadEnvCard(dir)
	if state.state != StateRendered || state.current == nil {
		t.Fatalf("env load state = %s", state.state)
	}
	if len(state.current.DistilledNotes) != 1 || state.current.DistilledNotes[0].NoteID != "n-1" {
		t.Fatalf("bogus note must be skipped: %+v", state.current.DistilledNotes)
	}
	if len(state.warnings) != 1 || !strings.Contains(state.warnings[0], "closed enum") {
		t.Fatalf("bogus note must WARN: %v", state.warnings)
	}
}

// TestSectionKindAndStableContract（§2.0 通用段模型）：L1 Static、L2-L4
// Session、全部 Stable=true、CacheKey=层版本身份；目录 Static 尾段。
func TestSectionKindAndStableContract(t *testing.T) {
	opts := tcFullFixture(t, tcBaseOptions(t))
	bundle := Assemble(context.Background(), opts)
	wantKind := map[string]promptruntime.SectionKind{
		LayerRules:   promptruntime.SectionStatic,
		LayerProfile: promptruntime.SectionSession,
		LayerEnv:     promptruntime.SectionSession,
		LayerLedger:  promptruntime.SectionSession,
		LayerCatalog: promptruntime.SectionStatic,
	}
	seen := 0
	for _, section := range bundle.Sections {
		kind, ok := wantKind[section.ID]
		if !ok {
			t.Fatalf("unexpected section %s", section.ID)
		}
		if section.Kind != kind {
			t.Fatalf("section %s kind = %s, want %s", section.ID, section.Kind, kind)
		}
		if !section.Stable {
			t.Fatalf("section %s must be Stable", section.ID)
		}
		if strings.TrimSpace(section.CacheKey) == "" {
			t.Fatalf("section %s missing CacheKey (layer version identity)", section.ID)
		}
		seen++
	}
	if seen != len(wantKind) {
		t.Fatalf("expected %d sections, saw %d", len(wantKind), seen)
	}
	if !strings.HasPrefix(bundle.Sections[0].CacheKey, "ruleset.") {
		t.Fatalf("L1 CacheKey must be the ruleset version, got %q", bundle.Sections[0].CacheKey)
	}
}

// TestFullStackSystemMessageLayout：四层+目录经 PrefixService 渲染进单条
// system 消息，L1 规则在前、目录在尾（§2.0 消息边界）。
func TestFullStackSystemMessageLayout(t *testing.T) {
	service := promptruntime.NewPrefixService()
	opts := tcFullFixture(t, tcBaseOptions(t))
	assembly, report, _ := tcAssemble(t, service, "layout", opts)
	system := ""
	for _, message := range assembly.Messages {
		if strings.EqualFold(message.Role, "system") {
			system = message.Content
		}
	}
	if system == "" {
		t.Fatalf("no system message rendered")
	}
	rulesAt := strings.Index(system, "You are Ask Vit")
	profileAt := strings.Index(system, "p-ref-level")
	envAt := strings.Index(system, "Environment instance")
	ledgerAt := strings.Index(system, "Project ledger")
	catalogAt := strings.Index(system, "Available DAW command catalog")
	for name, at := range map[string]int{"rules": rulesAt, "profile": profileAt, "env": envAt, "ledger": ledgerAt, "catalog": catalogAt} {
		if at < 0 {
			t.Fatalf("layer %s missing from system message", name)
		}
	}
	if !(rulesAt < profileAt && profileAt < envAt && envAt < ledgerAt && ledgerAt < catalogAt) {
		t.Fatalf("layer byte order violated in system message: rules=%d profile=%d env=%d ledger=%d catalog=%d",
			rulesAt, profileAt, envAt, ledgerAt, catalogAt)
	}
	if report.PrefixBytes == 0 || len(report.Breaks) != 0 {
		t.Fatalf("first assembly must have zero breaks and non-zero prefix bytes: %+v", report)
	}
}

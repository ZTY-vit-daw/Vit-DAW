package contextruntime

// history_refs_tb_test.go — T-B6（§6.2 退场一致性组）：伴随索引三态分布。
//
// 含混合 refs（parsed/legacy/opaque/malformed）的历史装配：
//   - HistoryRefEntry.ParseState 与 agentprotocol 直通一致；
//   - opaque 残骸不进可重拉面（Handle 空）但文本残骸保留（历史文本零改写）；
//   - parsed 且可 Resolve → Handle/Freshness 物化层透传；unresolved → 空；
//   - 账本反指（可回指时）；assistant-only 扫描；同 ref 去重。

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/contextruntime/carriers"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/promptruntime"
	"vit-daw-agent/internal/queryengine"
)

func tb6History() []llm.Message {
	return []llm.Message{
		{Role: "user", Content: "请检查 T1。引用 vit://tim/track:T1/t=all@obs_u#-（user 消息不进索引）"},
		{Role: "assistant", Content: "已观察。证据：vit://mom/track:T1/t=all@obs_1#sha256:0123456789abcdef；" +
			"旧引用 dom_0123456789abcdef；外部工件 weird://legacy-artifact；残缺 vit://mom/no-scope"},
		{Role: "assistant", Content: "补充：再次提及 vit://mom/track:T1/t=all@obs_1#sha256:0123456789abcdef（去重）。"},
	}
}

// TestTB6CompanionIndexTriStateDirectThrough：ParseState 三态直通 +
// 可重拉面限定 parsed + 文本残骸保留。
func TestTB6CompanionIndexTriStateDirectThrough(t *testing.T) {
	// Expand 桩：parsed refs 中可 Resolve 的回 handle/freshness，
	// obs_1 之外（tim user 消息不进；残缺不 parsed）——测试可重拉面。
	stubExpand := func(ctx context.Context, refs []string) ([]queryengine.ExpandedRef, error) {
		out := make([]queryengine.ExpandedRef, 0, len(refs))
		for _, ref := range refs {
			if strings.Contains(ref, "@obs_1") {
				out = append(out, queryengine.ExpandedRef{Ref: ref, Handle: "evidence://aabb", Bytes: 128, Freshness: queryengine.FreshnessMaterialReuse})
				continue
			}
			out = append(out, queryengine.ExpandedRef{Ref: ref, Freshness: "unresolved"})
		}
		return out, nil
	}
	projectDir := t.TempDir()
	if _, err := carriers.AppendLedgerEntry(projectDir, carriers.LedgerKindObservationConclusion, "",
		"T1 响度收敛结论", []string{"vit://mom/track:T1/t=all@obs_1#sha256:0123456789abcdef"}, 0,
		fixedClock(t)()); err != nil {
		t.Fatalf("ledger fixture: %v", err)
	}

	history := tb6History()
	indexer := &HistoryRefIndexer{
		Expand: stubExpand,
		Ledger: func() []carriers.LedgerEntry {
			entries, err := carriers.ReadLedger(projectDir)
			if err != nil {
				t.Fatalf("ledger read: %v", err)
			}
			return entries
		},
	}
	entries := indexer.Build(context.Background(), history, "turn-7")

	byRef := map[string]promptruntime.HistoryRefEntry{}
	for _, entry := range entries {
		byRef[entry.Ref] = entry
	}
	if len(entries) != 4 {
		t.Fatalf("T-B6 red: want 4 unique refs (user 消息不计、重复去重), got %d: %+v", len(entries), entries)
	}

	// 三态直通：与 agentprotocol.ParseRef 逐 ref 对照。
	for _, entry := range entries {
		parsed, err := agentprotocol.ParseRef(entry.Ref)
		want := string(parsed.State)
		if err != nil {
			want = string(agentprotocol.RefStateOpaque) // malformed → opaque 形态留痕
		}
		if entry.ParseState != want {
			t.Fatalf("T-B6 red: %s ParseState=%s, agentprotocol direct=%s", entry.Ref, entry.ParseState, want)
		}
		if entry.LastSeenTurn != "turn-7" {
			t.Fatalf("LastSeenTurn must annotate the assembling turn: %+v", entry)
		}
	}

	parsedRef := "vit://mom/track:T1/t=all@obs_1#sha256:0123456789abcdef"
	if e := byRef[parsedRef]; e.Handle != "evidence://aabb" || e.Freshness != queryengine.FreshnessMaterialReuse {
		t.Fatalf("resolved parsed ref must carry handle+freshness: %+v", e)
	} else if e.LedgerEntry != 1 {
		t.Fatalf("ledger back-pointer (可回指时) must fill: %+v", e)
	}
	if e := byRef["dom_0123456789abcdef"]; e.ParseState != string(agentprotocol.RefStateLegacy) || e.Handle != "" {
		t.Fatalf("legacy ref: state=legacy, no handle (无完整坐标): %+v", e)
	}
	// opaque：不进可重拉面（Handle/Freshness 空），文本残骸保留。
	if e := byRef["weird://legacy-artifact"]; e.ParseState != string(agentprotocol.RefStateOpaque) || e.Handle != "" || e.Freshness != "" {
		t.Fatalf("opaque residue must stay out of the re-pullable surface: %+v", e)
	}
	if e := byRef["vit://mom/no-scope"]; e.ParseState != string(agentprotocol.RefStateOpaque) {
		t.Fatalf("malformed vit:// residue annotated opaque: %+v", e)
	}

	// 文本残骸保留：历史文本零改写（§5.2 不改写审计面）。
	original := tb6History()
	for i := range history {
		if history[i].Content != original[i].Content || history[i].Role != original[i].Role {
			t.Fatalf("companion index must not rewrite history text (message %d)", i)
		}
	}
}

// TestTB6PureParseNoDeps：零值索引器（无 Expand/Ledger）仍完整产出三态
// 注记（生产 v1 挂点形态——Handle/Freshness/反指缺席为空，不臆造）。
func TestTB6PureParseNoDeps(t *testing.T) {
	entries := (&HistoryRefIndexer{}).Build(context.Background(), tb6History(), "turn-1")
	if len(entries) != 4 {
		t.Fatalf("pure-parse build must still index 4 refs: %+v", entries)
	}
	for _, entry := range entries {
		if entry.Handle != "" || entry.Freshness != "" || entry.LedgerEntry != 0 {
			t.Fatalf("no-dep build must leave handle/freshness/ledger empty: %+v", entry)
		}
	}
}

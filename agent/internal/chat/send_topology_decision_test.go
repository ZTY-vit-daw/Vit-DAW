package chat

import (
	"testing"
)

// AGENT-SENDROUTE-SEMANTICS-1 验收：拓扑决策表四分支（设计件 §2.2）。
// 决策是启发式（词法+上下文），误判走 pendingplan 用户拦截——这里只钉
// 决策函数本身的分支边界。

func TestDecidePluginLoadTopologyFourBranches(t *testing.T) {
	cases := []struct {
		name  string
		input SendTopologyDecisionInput
		want  SendTopologyDecision
	}{
		{
			name: "insert default: single track, no share signal",
			input: SendTopologyDecisionInput{
				UserText:         "给 vocals 加一个板式混响",
				SourceTrackCount: 1,
				HasSpatialIntent: true,
			},
			want: SendDecisionInsertDefault,
		},
		{
			name: "send structure: share signal plus two named sources",
			input: SendTopologyDecisionInput{
				UserText:         "给 vocals 和 guitar 共用一个板式混响",
				SourceTrackCount: 2,
				HasSpatialIntent: true,
			},
			want: SendDecisionSendStructure,
		},
		{
			name: "agent suggest: single-track spatial ask with >=2 spatial peers",
			input: SendTopologyDecisionInput{
				UserText:           "给 snare 加一点空间",
				SourceTrackCount:   1,
				HasSpatialIntent:   true,
				SpatialFamilyPeers: 2,
			},
			want: SendDecisionAgentSuggest,
		},
		{
			name: "insert locked: explicit single-track lock wording",
			input: SendTopologyDecisionInput{
				UserText:           "给 vocals 和 guitar 共用一个板式混响，但 kick 只给这条轨",
				SourceTrackCount:   2,
				HasSpatialIntent:   true,
				SpatialFamilyPeers: 3,
			},
			want: SendDecisionInsertLocked,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := decidePluginLoadTopology(tc.input); got != tc.want {
				t.Fatalf("decidePluginLoadTopology(%+v) = %s, want %s", tc.input, got, tc.want)
			}
		})
	}
}

func TestDecidePluginLoadTopologyShareSignalWithoutTwoSourcesStaysInsert(t *testing.T) {
	// 共享话术但话术内只解析到一条轨：不升级为 send 结构（单轨走 insert 面）。
	input := SendTopologyDecisionInput{
		UserText:         "给 vocals 共用一个混响",
		SourceTrackCount: 1,
		HasSpatialIntent: true,
	}
	if got := decidePluginLoadTopology(input); got != SendDecisionInsertDefault {
		t.Fatalf("share signal with single source should stay insert default, got %s", got)
	}
}

func TestResolveSendSourceTracksMatchesNamedTracksOnly(t *testing.T) {
	state := map[string]any{
		"tracks": []any{
			map[string]any{"track_id": "T1", "track_name": "vocals"},
			map[string]any{"track_id": "T2", "track_name": "guitar"},
			map[string]any{"track_id": "T3", "track_name": "bass"},
		},
	}
	got := resolveSendSourceTracks("给 vocals 和 guitar 共用一个板式混响", state)
	if len(got) != 2 {
		t.Fatalf("expected 2 sources, got %d (%v)", len(got), got)
	}
	if got[0].TrackID != "T1" || got[1].TrackID != "T2" {
		t.Fatalf("unexpected order/content: %+v", got)
	}
	// 未点名任何轨：零命中。
	if got := resolveSendSourceTracks("给这些轨共用混响", state); len(got) != 0 {
		t.Fatalf("unnamed request should resolve no sources, got %d", len(got))
	}
}

func TestCountSpatialFamilyPeersExcludesTargetAndCountsRackMarkers(t *testing.T) {
	state := map[string]any{
		"tracks": []any{
			map[string]any{"track_id": "T1", "rack": map[string]any{"nodes": []any{
				map[string]any{"plugin_path": `C:\VST\TDR Nova.vst3`},
			}}},
			map[string]any{"track_id": "T2", "rack": map[string]any{"nodes": []any{
				map[string]any{"plugin_path": `C:\VST\FabFilter Pro-R Reverb.vst3`},
			}}},
			map[string]any{"track_id": "T3", "rack": map[string]any{"nodes": []any{
				map[string]any{"plugin_path": `C:\VST\Valhalla Room.vst3`},
			}}},
		},
	}
	// 排除 T1（目标轨）后：T2(Pro-R)+T3(Room) 命中空间词法标记 → 2。
	if got := countSpatialFamilyPeers(state, map[string]bool{"T1": true}); got != 2 {
		t.Fatalf("expected 2 spatial peers, got %d", got)
	}
}

func TestSendFXTrackNameForPluginQuery(t *testing.T) {
	if got := sendFXTrackNameForPluginQuery("板式混响 plate reverb"); got != "Reverb Bus" {
		t.Fatalf("reverb query should map to Reverb Bus, got %s", got)
	}
	if got := sendFXTrackNameForPluginQuery("tape delay 延迟"); got != "Delay Bus" {
		t.Fatalf("delay query should map to Delay Bus, got %s", got)
	}
}

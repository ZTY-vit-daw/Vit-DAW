package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CONTRACT-1 RED④：两套今日真实事件流脱敏入库 webui fixtures，可被测试加载
// ——GUI-1 组合回放测试的基线（C4 前置）。边界：fixtures 目录只放 JSON，
// webui 代码不动。
const contractFixturesDir = "../../webui/src/trace/__fixtures__"

type contractFixtureFile struct {
	Name          string
	MinEvents     int
	WantEventType map[string]int
}

// 2026-09-05 取证实证的两种事件流形态：
//   - mtny2v9x（33 事件）：run 级 turn（裸 run_id 域）与实验 turn
//     （turn:free_state_* 前缀域）并存 + scheduler_chain 终局 + audition。
//   - mto15xxx（15 事件）：item-only 观察轮（M12 复现形态）——无实验轨迹
//     节点，切片边界 turn.completed(waiting_continue) + 链终局 chain_result。
var contractFixtureFiles = []contractFixtureFile{
	{
		Name:      "2026-09-05-webui-mtny2v9x-33events.json",
		MinEvents: 33,
		WantEventType: map[string]int{
			"turn.started":                       1,
			"trajectory.turn.started":            2,
			"turn.completed":                     3,
			"trajectory.turn.completed":          1,
			"trajectory.intent.framed":           1,
			"trajectory.user_judgment.requested": 1,
			"trajectory.round.decision":          1,
		},
	},
	{
		Name:      "2026-09-05-webui-mto15xxx-15events.json",
		MinEvents: 15,
		WantEventType: map[string]int{
			"turn.started":              1,
			"turn.completed":            2,
			"item.started":              5,
			"item.completed":            5,
			"trajectory.turn.started":   1,
			"trajectory.turn.completed": 1,
		},
	},
}

func loadContractFixture(t *testing.T, name string) []AgentEvent {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(contractFixturesDir, name))
	if err != nil {
		t.Fatalf("load fixture %s: %v", name, err)
	}
	var payload struct {
		Events []AgentEvent `json:"events"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("parse fixture %s: %v", name, err)
	}
	if len(payload.Events) == 0 {
		t.Fatalf("fixture %s carries no events", name)
	}
	return payload.Events
}

func TestContractReplayFixturesLoad(t *testing.T) {
	for _, fixture := range contractFixtureFiles {
		events := loadContractFixture(t, fixture.Name)
		if len(events) < fixture.MinEvents {
			t.Fatalf("fixture %s: want >=%d events, got %d", fixture.Name, fixture.MinEvents, len(events))
		}
		counts := map[string]int{}
		var lastSeq int64
		for _, event := range events {
			if event.Type == "" {
				t.Fatalf("fixture %s: event without type at seq %d", fixture.Name, event.Seq)
			}
			if event.Seq <= lastSeq {
				t.Fatalf("fixture %s: seq must be increasing (%d after %d)", fixture.Name, event.Seq, lastSeq)
			}
			lastSeq = event.Seq
			counts[event.Type]++
		}
		for eventType, want := range fixture.WantEventType {
			if counts[eventType] < want {
				t.Fatalf("fixture %s: event type %s want >=%d, got %d", fixture.Name, eventType, want, counts[eventType])
			}
		}
	}
}

// 回放基线必须保住取证实证的轨迹双域形态：mtny2v9x 同时含裸 run_id 域与
// turn:free_state_* 前缀域的 turn_id——C0 双读/双写迁移的兼容性依据。
func TestContractFixtureCapturesDualTurnDomains(t *testing.T) {
	events := loadContractFixture(t, "2026-09-05-webui-mtny2v9x-33events.json")
	sawRunDomain, sawFreeStateDomain := false, false
	for _, event := range events {
		switch {
		case strings.HasPrefix(event.TurnID, "run_"):
			sawRunDomain = true
		case strings.HasPrefix(event.TurnID, "turn:free_state"):
			sawFreeStateDomain = true
		}
	}
	if !sawRunDomain || !sawFreeStateDomain {
		t.Fatalf("fixture must capture both turn domains: run=%t free_state=%t", sawRunDomain, sawFreeStateDomain)
	}
}

// mto15xxx 是 M12 复现形态：链终局（chain_result）存在且该回合无任何非
// turn 轨迹节点——settle_slice 标记与 UI 隐藏谓词的对照样本。
func TestContractFixtureCapturesItemOnlySettleShape(t *testing.T) {
	events := loadContractFixture(t, "2026-09-05-webui-mto15xxx-15events.json")
	var sawChainResult, sawNonTurnTrajectory bool
	for _, event := range events {
		if event.ItemID == "chain_result" && event.Type == "turn.completed" {
			sawChainResult = true
		}
		if strings.HasPrefix(event.Type, "trajectory.") &&
			event.Type != "trajectory.turn.started" && event.Type != "trajectory.turn.completed" {
			sawNonTurnTrajectory = true
		}
	}
	if !sawChainResult {
		t.Fatal("fixture must capture the chain_result settle terminal")
	}
	if sawNonTurnTrajectory {
		t.Fatal("M12 repro shape must not contain non-turn trajectory nodes")
	}
}

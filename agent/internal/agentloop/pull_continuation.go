package agentloop

// pull_continuation.go — pull 模式恢复载体（L1-5-IMPL-D 腿2，
// HARNESS_V1_DESIGN §11.3-4 主管裁定第 4 项 + ADAPTER 提案 §5.2）。
//
// 边界（v1 冻结）：
//   - 仅同进程恢复：pullSession 活状态（runState/ledger/draft 句柄）不序列
//     化；跨进程 pull checkpoint 显式拒绝并报产品边界（不重放已执行动作）；
//   - 持久化 schema 版本化：旧 continuation 无 pull 字段=push 语义 fail-open
//     （走 legacy Continue，零行为变化）；未知版本/绑定不符=拒绝并报因
//     （保留数据，不默默转 push）；
//   - apply 后 receipt 未保存的跨进程崩溃面走执行协调器 Reconcile/回读，
//     不在本载体职责内（§5.3：unknown apply=fail-closed 上交）。

import (
	"fmt"
	"strings"
	"sync"
)

// pullContinuationSchemaVersion 是 pull checkpoint 的 schema 版本（版本化
// fail 边界：未知版本拒绝）。
const pullContinuationSchemaVersion = 1

// PullCheckpoint 是 pull 侧账本/窗口/边界提交状态的快照（提案 §5.2 表：
// pull 状态+窗与边界两行的最小面）。
type PullCheckpoint struct {
	NextCycle       uint64   `json:"next_cycle"`
	CompletedCycles int      `json:"completed_cycles"`
	ModelCalls      int      `json:"model_calls"`
	ToolAttempts    int      `json:"tool_attempts"`
	ProbeSpent      float64  `json:"probe_spent"`
	ProbeCostKnown  bool     `json:"probe_cost_known"`
	MaxCycles       int      `json:"max_cycles,omitempty"`
	MaxProbeCost    float64  `json:"max_probe_cost,omitempty"`
	SettledReceipts []string `json:"settled_receipts,omitempty"`
	ClosedBatchIDs  []string `json:"closed_batch_ids,omitempty"`
	Generation      int      `json:"generation"`
	SessionKey      string   `json:"session_key"`
	GoalID          string   `json:"goal_id"`
	RunID           string   `json:"run_id"`
	ProjectDir      string   `json:"project_dir,omitempty"`
}

// PullContinuation 包装既有 Continuation 加版本化 pull checkpoint。
// 旧记录（无 Pull 字段）一律解释为原 push 语义（fail-open，零行为变化）。
type PullContinuation struct {
	SchemaVersion int             `json:"pull_schema_version,omitempty"`
	HarnessMode   string          `json:"harness_mode,omitempty"`
	Continuation  *Continuation   `json:"continuation"`
	Pull          *PullCheckpoint `json:"pull_checkpoint,omitempty"`
}

// IsPull 报告该载体是否携带 pull checkpoint（否则按旧 push 语义解释）。
func (pc *PullContinuation) IsPull() bool {
	return pc != nil && pc.Pull != nil
}

// ValidatePull 校验恢复前置（schema 版本+身份绑定）：未知版本/绑定不符
// 返回显式错误（保留数据，不默默转 push）。
func (pc *PullContinuation) ValidatePull(goalID, runID, projectDir string) error {
	if pc == nil || pc.Pull == nil {
		return fmt.Errorf("not a pull continuation")
	}
	if pc.SchemaVersion != pullContinuationSchemaVersion {
		return fmt.Errorf("pull continuation schema version %d unsupported (want %d): rejected, data preserved", pc.SchemaVersion, pullContinuationSchemaVersion)
	}
	if pc.HarnessMode != "" && pc.HarnessMode != "pull" {
		return fmt.Errorf("pull continuation harness_mode %q is not pull", pc.HarnessMode)
	}
	if pc.Pull.GoalID != "" && goalID != "" && pc.Pull.GoalID != goalID {
		return fmt.Errorf("pull continuation goal binding mismatch: checkpoint=%q resume=%q", pc.Pull.GoalID, goalID)
	}
	if pc.Pull.RunID != "" && runID != "" && pc.Pull.RunID != runID {
		return fmt.Errorf("pull continuation run binding mismatch: checkpoint=%q resume=%q", pc.Pull.RunID, runID)
	}
	if pc.Pull.ProjectDir != "" && projectDir != "" && pc.Pull.ProjectDir != projectDir {
		return fmt.Errorf("pull continuation project binding mismatch: checkpoint=%q resume=%q", pc.Pull.ProjectDir, projectDir)
	}
	return nil
}

// continuationForResume 在 Suspend 提交时构造恢复载体（既有 Continuation
// 全体字段由 r.result 的 draft 组装面持有；此处从 draft 取 legacy 面并包上
// pull checkpoint）。draft.Continuation 为 nil（终局）时返回 nil。
func (s *pullSession) continuationForResume(draft Result) *PullContinuation {
	legacy := draft.Continuation
	if legacy == nil {
		return nil
	}
	settled := make([]string, 0, len(s.ledger.settled))
	for receipt := range s.ledger.settled {
		settled = append(settled, receipt)
	}
	closed := make([]string, 0, len(s.closed))
	for batchID := range s.closed {
		closed = append(closed, batchID)
	}
	s.resumeGeneration++
	return &PullContinuation{
		SchemaVersion: pullContinuationSchemaVersion,
		HarnessMode:   "pull",
		Continuation:  legacy,
		Pull: &PullCheckpoint{
			NextCycle:       s.ledger.nextCycle + 1,
			CompletedCycles: s.ledger.completedCycles,
			ModelCalls:      s.ledger.modelCalls,
			ToolAttempts:    s.ledger.toolAttempts,
			ProbeSpent:      s.ledger.probeSpent,
			ProbeCostKnown:  s.ledger.probeCostKnown,
			MaxCycles:       s.ledger.maxCycles,
			MaxProbeCost:    s.ledger.maxProbeCost,
			SettledReceipts: settled,
			ClosedBatchIDs:  closed,
			Generation:      s.resumeGeneration,
			SessionKey:      s.prefixKey,
			GoalID:          s.state.goal.GoalID,
			RunID:           s.state.goal.RunID,
			ProjectDir:      runProjectDirFromState(s.state),
		},
	}
}

// pullSessionRegistry 是同进程 pull 会话登记面（v1 恢复边界：DraftID/
// ledger/活 state 不序列化，跨请求续跑靠进程内会话缓存；进程重启后 pull
// checkpoint 显式拒绝）。键=GoalID（一个 goal 同时刻至多一个活会话）。
var pullSessionRegistry sync.Map // map[string]*pullSession

func pullSessionRegister(goalID string, session *pullSession) {
	if strings.TrimSpace(goalID) == "" {
		return
	}
	pullSessionRegistry.Store(strings.TrimSpace(goalID), session)
}

func pullSessionLookup(goalID string) (*pullSession, bool) {
	if strings.TrimSpace(goalID) == "" {
		return nil, false
	}
	session, ok := pullSessionRegistry.Load(strings.TrimSpace(goalID))
	if !ok {
		return nil, false
	}
	pull, ok := session.(*pullSession)
	return pull, ok
}

func pullSessionRelease(goalID string) {
	if strings.TrimSpace(goalID) == "" {
		return
	}
	pullSessionRegistry.Delete(strings.TrimSpace(goalID))
}

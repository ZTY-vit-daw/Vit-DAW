package harness

import (
	"context"
	"strings"

	"vit-daw-agent/internal/projectstore"
	"vit-daw-agent/internal/projectworkspace"
)

// 观察链 project identity 对账层（M3-OBS-IDENTITY-BIND-1）。
//
// 内核在 new_project/save_as 上合法铸造新工程 id（fork 语义，edit state 随之
// 漂移）。经 agent 命令/通知面到达的生命周期变化会在相应钩子里再绑定 active
// store；绕过这些面的变化（raw kernel ZMQ、外部自动化）则让 bake 侧持久化
// 身份（projectstore.Current）与读取侧解析身份（shadow state）分裂，
// MergeStatus 随后正确地把新鲜事实判 stale（project_mismatch）——判定本身是
// 防御语义，缺陷在上游无对账。本挂点让观察链在解析读取身份之前跟随内核的
// live edit 身份：
//   - fork 跟随：内核声明 live 身份是当前绑定身份的子代（parent_project_uuid
//     == 当前绑定 id，即 save_as/new-from 铸造）→ 复用既有传播件
//     ForkDerived+ForkAgentStore 把事实再绑定到 live 身份后激活；
//   - switch 跟随：其余分歧（外部 open 他工程等）→ 只激活 live 身份自己的
//     store，不搬运旧身份的事实。
//
// 单一真相源=内核当前 loaded project 的 edit state UUID；落盘/观察/导入层跟随。
// 全程 fail-open：跟随失败仅 WARN，观察按先前绑定继续（=修复前行为）。

// reconcileProjectIdentityForObservation 对齐观察读取身份与事实持久化身份。
// 返回（可能已刷新的）state，供调用方继续原观察流程。
func (h *Harness) reconcileProjectIdentityForObservation(ctx context.Context, state map[string]any) map[string]any {
	if h == nil {
		return state
	}
	h.identityReconcileMu.Lock()
	defer h.identityReconcileMu.Unlock()

	liveID, livePath := observationLiveIdentity(state)
	// 无具体 live id（未保存会话的 "current"/空）不对账：真实内核的
	// project_uuid 恒为具体值，泛型态只出现在无工程会话——此时保持零内核
	// 交互（read-first 观察的既有契约），漂移恢复由下方具体 id 分支覆盖。
	if mixObservationGenericProjectIdentity(liveID) {
		return state
	}
	roots, haveStore := projectstore.Current()
	if !haveStore || strings.TrimSpace(roots.ProjectUUID) == "" {
		h.followLiveProjectIdentity("", "", liveID, livePath, "bind")
		return state
	}
	if sameReconcileIdentity(roots.ProjectUUID, liveID) {
		// 同 id 异路径（工程目录搬迁/换名后重开）：再激活到 live 路径，沿用
		// chat 侧 activateCurrentProjectWorkspace 的同 id 再激活语义。
		if strings.TrimSpace(livePath) != "" && !samePath(roots.ProjectPath, livePath) {
			if _, err := h.ActivateProjectStore(livePath, liveID); err != nil && h.logger != nil {
				h.logger.Warn("[identity-reconcile] same-id re-activate live=%s path=%s failed: %v", liveID, livePath, err)
			}
		}
		return state
	}
	// shadow 可能滞后于最近一次内核身份变化：先用新鲜内核视图重比，再决定跟随。
	// 新鲜视图若不携带具体身份（无 uuid），不得用它覆盖携带身份的旧视图。
	if fresh := h.refreshStateForIdentityReconcile(ctx, state, "project_identity_reconcile_mismatch"); !mixObservationGenericProjectIdentity(firstString(fresh, "project_uuid", "project_id")) {
		state = fresh
		if freshID, freshPath := observationLiveIdentity(state); !mixObservationGenericProjectIdentity(freshID) {
			liveID, livePath = freshID, freshPath
		}
	}
	if sameReconcileIdentity(roots.ProjectUUID, liveID) {
		return state
	}
	mode := "switch"
	if parent := strings.TrimSpace(firstString(state, "parent_project_uuid")); parent != "" && sameReconcileIdentity(parent, roots.ProjectUUID) {
		mode = "fork"
	}
	h.followLiveProjectIdentity(roots.ProjectPath, roots.ProjectUUID, liveID, livePath, mode)
	return state
}

// refreshStateForIdentityReconcile 刷新 shadow 并返回新的 user state；刷新不可
// 用（无内核/失败）时原样返回旧 state。
func (h *Harness) refreshStateForIdentityReconcile(ctx context.Context, state map[string]any, reason string) map[string]any {
	if h.shadow == nil || h.kernel == nil {
		return state
	}
	refresh := h.refreshShadowWithStatus(ctx, reason)
	if !boolValueDefault(refresh["shadow_refreshed"], false) {
		return state
	}
	return h.UserStateSummary(ctx)
}

// followLiveProjectIdentity 把 active store 跟随到内核 live 身份。mode:
// bind（尚无绑定时直接绑定）/fork（子代身份，事实随行再绑定）/switch（无关
// 身份，只激活目标自己的 store）。无路径的未保存身份不跟随（读面纪律：不为
// 读取物化 default draft，也不把 uuid 未核实的进程 draft 槽位当路径，
// FIX-START-DRAFT-SIDE-EFFECT-1）。
func (h *Harness) followLiveProjectIdentity(sourcePath, sourceUUID, liveID, livePath, mode string) {
	targetPath := strings.TrimSpace(livePath)
	if targetPath == "" {
		// 唯一允许的无路径回退：chat 侧已为同一 id 激活过的 workspace 提示。
		if hintPath, hintUUID := h.ProjectPathHint(); hintPath != "" && sameReconcileIdentity(hintUUID, liveID) {
			targetPath = hintPath
		}
	}
	if targetPath == "" {
		if h.logger != nil {
			h.logger.Info("[identity-reconcile] skip %s-follow live=%s: unsaved project has no verified path (no draft materialization on read paths)", mode, liveID)
		}
		return
	}
	if mode == "fork" {
		if _, err := projectworkspace.ForkDerived(sourcePath, sourceUUID, targetPath, liveID); err != nil && h.logger != nil {
			h.logger.Warn("[identity-reconcile] derived fork %s->%s failed (facts stay under the source identity): %v", sourceUUID, liveID, err)
		}
		if _, err := projectstore.ForkAgentStore(sourcePath, sourceUUID, targetPath, liveID); err != nil && h.logger != nil {
			h.logger.Warn("[identity-reconcile] agent store fork %s->%s failed: %v", sourceUUID, liveID, err)
		}
	}
	if _, err := h.ActivateProjectStore(targetPath, liveID); err != nil {
		if h.logger != nil {
			h.logger.Warn("[identity-reconcile] %s-follow activate live=%s path=%s failed: %v", mode, liveID, targetPath, err)
		}
		return
	}
	if h.logger != nil {
		h.logger.Info("[identity-reconcile] %s-follow live=%s path=%s source=%s", mode, liveID, targetPath, firstNonEmpty(sourceUUID, "-"))
	}
}

func observationLiveIdentity(state map[string]any) (string, string) {
	id := firstString(state, "project_uuid", "project_id")
	path := firstString(state, "project_path", "current_project_path")
	return id, path
}

func sameReconcileIdentity(a, b string) bool {
	return strings.EqualFold(projectstore.SafeName(a), projectstore.SafeName(b))
}

package harness

import (
	"context"
	"fmt"
	"os"
	"strings"

	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/tools"
)

// CONV-OPEN-TOOL-1（池序 419）: version.conversation.open 组合缝。
//
// B2b 左栏【打开此工作树并继续此对话】只需要一次工程重载：本工具把一个
// 「校验/守卫 → （跨工作树时复用既有的 switch autosave）→ 在目标工作树
// 副本内物化目标对话节点 → 恰好一次 withProjectReload」组合成一个 agent 侧
// 动作。旧工具（version.checkout / version.node_checkout /
// version.worktree_checkout）的分派与语义保持逐字不变，本工具只加组合缝。
//
// 成功后的目标副本状态：
//   - 目标节点所属分支的 HEAD 恰好就是该节点 commit 时（B2b 对话流的常规形态），
//     复用 history.Checkout 的分支语义：物化该 commit、写 active branch/HEAD/
//     active node（非 detached）。
//   - 否则回退到现有 node checkout 的 detached 语义（物化该 commit、写 active
//     node），并在结果里披露 branch_warning，不静默假装切了分支。
//
// 失败语义（卡面 §5）：
//   - reload 之前的任何失败 → 返回 error，withProjectReload 不被调用，当前内核
//     工程保持原样；result.target_materialized 如实披露目标副本是否已物化
//     （false=未发生；"unknown"=物化调用报错、可能已部分写入；true=已物化）。
//   - 内核 open_project 失败 → 返回带原因的 error；目标副本已物化的事实与
//     refresh.warning 一并披露，不静默成功。
func (h *Harness) conversationOpen(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	const toolName = "version.conversation.open"
	requestedNodeID := strings.TrimSpace(firstString(cmd, "node_id"))
	targetName := strings.TrimSpace(firstString(cmd, "name"))
	projectPathArg := strings.TrimSpace(firstString(cmd, "project_file_path"))
	requestedBranch := strings.TrimSpace(firstString(cmd, "branch"))

	result := map[string]any{
		"status":              "ok",
		"requested_node_id":   requestedNodeID,
		"requested_branch":    requestedBranch,
		"project_file_path":   projectPathArg,
		"target_materialized": false,
		"kernel_reloaded":     false,
		"same_worktree":       false,
	}
	fail := func(err error) (map[string]any, error) {
		return result, err
	}
	if h == nil {
		return fail(fmt.Errorf("%s requires a live harness", toolName))
	}
	if requestedNodeID == "" {
		return fail(fmt.Errorf("%s requires non-empty node_id", toolName))
	}
	if targetName == "" && projectPathArg == "" {
		return fail(fmt.Errorf("%s requires a target worktree identity: name or project_file_path", toolName))
	}
	currentPath, _ := h.CurrentProjectIdentity(ctx)
	currentPath = strings.TrimSpace(currentPath)
	if currentPath == "" {
		if hintPath, _ := h.ProjectPathHint(); hintPath != "" {
			currentPath = strings.TrimSpace(hintPath)
		}
	}
	if currentPath == "" {
		return fail(fmt.Errorf("%s cannot resolve the current project path; open a saved project first", toolName))
	}
	result["project_path"] = currentPath

	listed, err := history.WorktreeList(map[string]any{"project_path": currentPath})
	if err != nil {
		return fail(fmt.Errorf("list Project History worktrees: %w", err))
	}
	target, err := resolveConversationOpenWorktree(mapRowsFromAny(listed["worktrees"]), targetName, projectPathArg)
	if err != nil {
		return fail(err)
	}
	result["worktree"] = target.row
	result["worktree_name"] = target.name
	result["worktree_is_root"] = target.isRoot
	result["project_file_path"] = target.projectFilePath
	if info, statErr := os.Stat(target.projectFilePath); statErr != nil || info.IsDir() {
		return fail(fmt.Errorf("worktree project file does not exist: %s", target.projectFilePath))
	}
	sameWorktree := samePath(target.projectFilePath, currentPath)
	result["same_worktree"] = sameWorktree

	// 校验目标工程历史里的分支与节点；任何校验失败都发生在 autosave / 物化 /
	// 内核 reload 之前，因此当前工程完全不受影响。
	targetStatus, err := history.Status(map[string]any{"project_path": target.projectFilePath})
	if err != nil {
		return fail(fmt.Errorf("read target worktree Project History: %w", err))
	}
	if available, ok := boolValue(targetStatus["available"]); ok && !available {
		return fail(fmt.Errorf("target worktree has no Project History: %s", target.projectFilePath))
	}
	node, nodeCommitID, err := conversationOpenNode(targetStatus, requestedNodeID)
	if err != nil {
		return fail(err)
	}
	result["target_commit_id"] = nodeCommitID

	branches := mapFromAny(targetStatus["branches"])
	nodeBranch := strings.TrimSpace(firstString(node, "branch"))
	effectiveBranch := ""
	branchWarning := ""
	if requestedBranch != "" {
		branchHead := strings.TrimSpace(firstString(branches, requestedBranch))
		if branchHead == "" {
			return fail(fmt.Errorf("branch not found in target worktree: %s", requestedBranch))
		}
		result["requested_branch_head_commit_id"] = branchHead
		if nodeBranch != requestedBranch {
			branchWarning = fmt.Sprintf("conversation node %s belongs to branch %q, not requested branch %q; opening the node with node-checkout semantics", requestedNodeID, nodeBranch, requestedBranch)
		} else if branchHead != nodeCommitID {
			branchWarning = fmt.Sprintf("requested branch %s head %s does not match conversation node commit %s; opening the node with node-checkout semantics", requestedBranch, branchHead, nodeCommitID)
		} else {
			effectiveBranch = requestedBranch
		}
	} else if nodeBranch != "" {
		if branchHead := strings.TrimSpace(firstString(branches, nodeBranch)); branchHead != "" && branchHead == nodeCommitID {
			effectiveBranch = nodeBranch
		}
	}
	if branchWarning != "" {
		result["branch_warning"] = branchWarning
		result["warnings"] = appendStringAny(result["warnings"], branchWarning)
	}

	// 跨工作树：复用现有 version_worktree_checkout 的切换前 autosave 语义
	// （当前工程的未保存内容按既有行为先做一次 switch checkpoint，失败只披露
	// 警告、不静默丢改动）。同工作树等价于 node checkout，不做 switch autosave。
	if !sameWorktree {
		autosaveArgs := tools.CloneCommand(cmd)
		autosaveArgs["project_file_path"] = target.projectFilePath
		autosave := h.worktreeCheckoutArgs(ctx, autosaveArgs)
		if commitID := firstString(autosave, "switch_autosave_commit_id"); commitID != "" {
			result["switch_autosave_commit_id"] = commitID
		}
		if warning := firstString(autosave, "switch_autosave_warning"); warning != "" {
			result["switch_autosave_warning"] = warning
			result["warnings"] = appendStringAny(result["warnings"], warning)
		}
	}

	// 物化目标节点：分支可用时按分支 checkout（节点 commit == 分支 HEAD），
	// 否则按现有 node checkout 语义在目标 commit 上 detached 打开。
	var materialize map[string]any
	if effectiveBranch != "" {
		materialize, err = history.Checkout(map[string]any{
			"project_path": target.projectFilePath,
			"commit_id":    nodeCommitID,
			"branch":       effectiveBranch,
		})
		if err != nil {
			result["target_materialized"] = "unknown"
			result["materialize_error"] = err.Error()
			return result, fmt.Errorf("materialize conversation node %s into worktree %s: %w", requestedNodeID, target.name, err)
		}
		result["mode"] = "branch_checkout"
		result["branch"] = effectiveBranch
	} else {
		materialize, err = history.NodeCheckout(map[string]any{
			"project_path": target.projectFilePath,
			"node_id":      requestedNodeID,
		})
		if err != nil {
			result["target_materialized"] = "unknown"
			result["materialize_error"] = err.Error()
			return result, fmt.Errorf("materialize conversation node %s into worktree %s: %w", requestedNodeID, target.name, err)
		}
		result["mode"] = "node_checkout"
	}
	result["target_materialized"] = true
	mergeConversationOpenWarnings(result, materialize)

	// 物化后复核目标副本的 HEAD/active branch/active node，确保 reload 前目标
	// 状态真的就位；复核失败同样不 reload。
	verified, err := history.Status(map[string]any{"project_path": target.projectFilePath})
	if err != nil {
		return result, fmt.Errorf("verify target worktree conversation state: %w", err)
	}
	verifiedGraph := mapFromAny(verified["conversation_graph"])
	activeNodeID := strings.TrimSpace(firstString(verified, "active_node_id"))
	activeBranch := strings.TrimSpace(firstString(verified, "active_branch"))
	activeWorktree := strings.TrimSpace(firstString(verified, "active_worktree"))
	detached := boolValueDefault(verified["detached"], false)
	head := strings.TrimSpace(firstString(verified, "head"))
	if head != nodeCommitID {
		return result, fmt.Errorf("target worktree HEAD %q does not match conversation node commit %q", head, nodeCommitID)
	}
	if activeNodeID == "" {
		return result, fmt.Errorf("target worktree has no active conversation node after materializing %s", requestedNodeID)
	}
	if effectiveBranch != "" && (detached || activeBranch != effectiveBranch) {
		return result, fmt.Errorf("target worktree did not activate branch %q (active_branch=%q detached=%t)", effectiveBranch, activeBranch, detached)
	}
	if activeNodeID != requestedNodeID {
		activeCommit := conversationOpenNodeCommitID(verifiedGraph, activeNodeID)
		if activeCommit != nodeCommitID || effectiveBranch == "" {
			return result, fmt.Errorf("target worktree active conversation node %q does not match requested node %q", activeNodeID, requestedNodeID)
		}
		result["active_node_normalized"] = true
	}
	result["active_node_id"] = activeNodeID
	result["active_branch"] = activeBranch
	result["active_worktree"] = activeWorktree
	result["detached"] = detached
	result["head"] = head

	// 最后且仅一次工程重载：内核 open_project + get_project_state。
	result = h.withProjectReload(ctx, "version_conversation_open", result)
	refresh := mapFromAny(result["refresh"])
	if !boolValueDefault(refresh["kernel_reloaded"], false) {
		reason := strings.TrimSpace(firstString(refresh, "warning"))
		if reason == "" {
			reason = "kernel open_project did not complete"
		}
		return result, fmt.Errorf("kernel project reload failed after materializing the target worktree copy: %s", reason)
	}
	result["kernel_reloaded"] = true
	return result, nil
}

type conversationOpenWorktree struct {
	row             map[string]any
	name            string
	projectFilePath string
	isRoot          bool
}

// resolveConversationOpenWorktree resolves the target worktree by name and/or
// project_file_path against the Project History worktree list of the current
// project's repository root. Both identifiers may be supplied, but then they
// must point at the same worktree; unknown or conflicting identities are
// rejected without touching any project state.
func resolveConversationOpenWorktree(rows []map[string]any, name, projectFilePath string) (conversationOpenWorktree, error) {
	var byName, byPath *conversationOpenWorktree
	for _, row := range rows {
		if row == nil {
			continue
		}
		item := conversationOpenWorktree{
			row:             row,
			name:            strings.TrimSpace(fmt.Sprint(row["name"])),
			projectFilePath: strings.TrimSpace(fmt.Sprint(row["project_file_path"])),
			isRoot:          boolValueDefault(row["is_root"], false),
		}
		if item.name == "<nil>" {
			item.name = ""
		}
		if item.projectFilePath == "<nil>" {
			item.projectFilePath = ""
		}
		if item.projectFilePath == "" {
			item.projectFilePath = strings.TrimSpace(fmt.Sprint(row["path"]))
		}
		if item.projectFilePath == "" || item.projectFilePath == "<nil>" {
			continue
		}
		candidate := item
		if name != "" && byName == nil && item.name == name {
			byName = &candidate
		}
		if projectFilePath != "" && byPath == nil && samePath(item.projectFilePath, projectFilePath) {
			byPath = &candidate
		}
	}
	if name != "" && projectFilePath != "" {
		if byName == nil || byPath == nil || !samePath(byName.projectFilePath, byPath.projectFilePath) {
			return conversationOpenWorktree{}, fmt.Errorf("worktree identity mismatch: name=%s project_file_path=%s", name, projectFilePath)
		}
		return *byName, nil
	}
	if byName != nil {
		return *byName, nil
	}
	if byPath != nil {
		return *byPath, nil
	}
	if name != "" {
		return conversationOpenWorktree{}, fmt.Errorf("worktree not found: %s", name)
	}
	return conversationOpenWorktree{}, fmt.Errorf("worktree not found: %s", projectFilePath)
}

// conversationOpenNode looks up one conversation node in a target project's
// Project History status projection and returns its checkpoint commit.
func conversationOpenNode(status map[string]any, nodeID string) (map[string]any, string, error) {
	graph := mapFromAny(status["conversation_graph"])
	for _, node := range mapRowsFromAny(graph["nodes"]) {
		if strings.TrimSpace(fmt.Sprint(node["id"])) != nodeID {
			continue
		}
		commitID := strings.TrimSpace(fmt.Sprint(node["commit_id"]))
		if commitID == "" || commitID == "<nil>" {
			return nil, "", fmt.Errorf("conversation node has no checkpoint: %s", nodeID)
		}
		return node, commitID, nil
	}
	return nil, "", fmt.Errorf("conversation node not found in target worktree history: %s", nodeID)
}

func conversationOpenNodeCommitID(graph map[string]any, nodeID string) string {
	for _, node := range mapRowsFromAny(graph["nodes"]) {
		if strings.TrimSpace(fmt.Sprint(node["id"])) == nodeID {
			return strings.TrimSpace(fmt.Sprint(node["commit_id"]))
		}
	}
	return ""
}

func mergeConversationOpenWarnings(result, source map[string]any) {
	if result == nil || source == nil {
		return
	}
	for _, warning := range stringSliceFromAny(source["warnings"]) {
		result["warnings"] = appendStringAny(result["warnings"], warning)
	}
}

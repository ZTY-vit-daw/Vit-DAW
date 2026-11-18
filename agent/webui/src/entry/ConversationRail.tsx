/*
 * CONV-RAIL-CROSS-1（421，B2b）：工程面左栏「对话流」树（工作区→工作树→分支）。
 *
 * 纯呈现组件：数据全部来自 props（ProjectSurface 经 /agent/ui/state + version.status
 * 懒加载缓存注入），组件自身零请求、零动作——点击只把「分支目标行」交给上层：
 * - 当前工作树分支 → 上层决定清预览/只读预览，动作走 version.checkout；
 * - 其它工作树分支 → 上层只读预览，动作走 version.conversation_open；
 * - 工作区级「其他分支」→ 只读预览（无继续动作）。
 * 其它工作树组默认折叠；展开时由上层懒加载 version.status（一次一取，可手动刷新）。
 * 动作态（busy/checkout_blocked/error）由上层回灌，按钮禁用与原因文案在本组件呈现。
 *
 * 渲染位置：ProjectSurface 经 createPortal 注入 EntryShell 的左栏体槽
 * （data-entry-rail-slot）；无 rail context（fixture/gallery/静态渲染）时不渲染，
 * 既有渲染树逐字不变。
 */

import { useMemo } from "react";
import type { AgentUIState } from "../types";
import { asRecord } from "../lib/util";
import {
  conversationWorktreeGroups,
  type ConversationBranchTarget,
  type ConversationWorktreeStatusMap
} from "./conversationData";
import "./conversationRail.css";

export interface ConversationRailProps {
  /** /agent/ui/state 投影（null=尚未取到，左栏显示空态）。 */
  uiState: AgentUIState | null;
  /** 已展开的其它工作树 key 列表（ProjectSurface 持有，跨 surface 切换不丢）。 */
  expandedWorktreeKeys: string[];
  /** 其它工作树 version.status 懒加载缓存（ProjectSurface 持有）。 */
  worktreeStatus: ConversationWorktreeStatusMap;
  /** 正在预览的分支（null=未预览，当前对话）。 */
  previewTarget: { worktreeKey: string; branch: string } | null;
  /** agent Turn 运行中：切换/新建对话禁用（与 HistoryPane 同守卫）。 */
  checkoutBlocked: boolean;
  /** 动作进行中：禁止重复点击。 */
  busy: boolean;
  /** 最近一次动作失败原因（保留预览并显示，不自动重试）。 */
  error: string;
  /** 点击分支行：当前分支=清预览；其它=只读预览（是否跨工作树由分支目标自带）。 */
  onSelectBranch: (target: ConversationBranchTarget) => void;
  /** 展开/折叠其它工作树组（展开时上层懒加载 status）。 */
  onToggleWorktree: (key: string, projectFilePath: string) => void;
  /** 手动刷新其它工作树 status（展开一次只取一次，刷新=显式重取）。 */
  onRefreshWorktree: (key: string, projectFilePath: string) => void;
  /** 【＋ 新对话】：从当前活动节点/当前 HEAD 分叉。 */
  onNewConversation: () => void;
}

function shortCommit(commitID: string): string {
  const clean = commitID.trim();
  return clean.length > 10 ? clean.slice(0, 10) : clean;
}

export function ConversationRail({
  uiState,
  expandedWorktreeKeys,
  worktreeStatus,
  previewTarget,
  checkoutBlocked,
  busy,
  error,
  onSelectBranch,
  onToggleWorktree,
  onRefreshWorktree,
  onNewConversation
}: ConversationRailProps) {
  const history = asRecord(uiState?.project_history);
  const groups = useMemo(
    () => conversationWorktreeGroups(history, worktreeStatus),
    [history, worktreeStatus]
  );
  // 427：无活动节点不再是禁用理由——只要存在工程身份（project_path/active branch/head），
  // 新对话即可从当前 HEAD 分叉（agent 侧 version.branch_create 省略 from_node_id 时如此）。
  const newDisabled = busy || checkoutBlocked || !groups.hasProjectIdentity;
  const newHint = checkoutBlocked
    ? "Agent Turn 正在运行；请先 Stop Turn 再新建对话。"
    : groups.activeNodeId
      ? "从当前对话的活动节点分叉出新对话"
      : "从当前工程状态新建对话";

  const isPreviewing = (target: ConversationBranchTarget): boolean =>
    Boolean(previewTarget && previewTarget.worktreeKey === target.worktreeKey && previewTarget.branch === target.branch);

  const renderBranchButton = (target: ConversationBranchTarget, inActiveWorktree: boolean) => {
    const previewing = isPreviewing(target);
    // 全局「当前分支高亮」只标当前活动工作树的活动分支；其它工作树组只做
    // checked-out 提示，避免多组同时出现 data-conversation-branch-active=true。
    const activeHighlight = inActiveWorktree && target.active;
    const checkedOut = target.active && !inActiveWorktree;
    return (
      <li key={`${target.worktreeKey}:${target.branch}`} className="v5-conversation-branch-item">
        <button
          type="button"
          className={[
            "v5-conversation-branch",
            activeHighlight ? "v5-conversation-branch--active" : "",
            previewing ? "v5-conversation-branch--preview" : ""
          ]
            .filter(Boolean)
            .join(" ")}
          data-conversation-branch={target.branch}
          data-conversation-branch-worktree={target.worktreeKey}
          data-conversation-branch-active={activeHighlight ? "true" : "false"}
          data-conversation-branch-checked-out={checkedOut ? "true" : "false"}
          data-conversation-branch-preview={previewing ? "true" : "false"}
          aria-current={activeHighlight ? "true" : undefined}
          title={target.subtitle}
          onClick={() => onSelectBranch(target)}
        >
          <span className="v5-conversation-branch-title" data-conversation-branch-title>
            {target.title}
          </span>
          <span className="v5-conversation-branch-subtitle" data-conversation-branch-subtitle>
            {target.subtitle}
          </span>
          {activeHighlight ? (
            <span className="v5-conversation-branch-current" data-conversation-branch-current>
              当前
            </span>
          ) : checkedOut ? (
            <span className="v5-conversation-branch-current" data-conversation-branch-checked-out-label>
              已检出
            </span>
          ) : null}
        </button>
      </li>
    );
  };

  const activeGroup = groups.activeGroup;
  const showEmpty =
    !activeGroup && groups.otherGroups.length === 0 && groups.otherBranches.length === 0;
  return (
    <section className="v5-conversation-rail" data-conversation-rail aria-label="对话流">
      <div className="v5-conversation-rail-head" data-conversation-rail-head>
        <span className="v5-conversation-rail-title" data-conversation-rail-title>
          对话流
        </span>
        <button
          type="button"
          className="v5-btn v5-btn--secondary v5-conversation-new"
          data-conversation-new
          disabled={newDisabled}
          title={newHint}
          onClick={onNewConversation}
        >
          ＋ 新对话
        </button>
      </div>
      {showEmpty ? (
        <p className="v5-conversation-empty" data-conversation-empty>
          暂无历史对话
        </p>
      ) : null}
      {activeGroup ? (
        <>
          <div
            className="v5-conversation-worktree"
            data-conversation-worktree
            data-conversation-worktree-active="true"
            data-conversation-worktree-path={activeGroup.projectFilePath}
            title={activeGroup.projectFilePath || activeGroup.path}
          >
            <span className="v5-conversation-worktree-label" data-conversation-worktree-label>
              {activeGroup.label}
            </span>
            <span className="v5-conversation-worktree-path" data-conversation-worktree-path>
              {activeGroup.projectFilePath || activeGroup.path}
            </span>
            {groups.activeBranch ? (
              <span className="v5-conversation-worktree-branch" data-conversation-worktree-branch>
                当前分支 {groups.activeBranch}
              </span>
            ) : null}
          </div>
          {activeGroup.branches.length === 0 ? (
            <p className="v5-conversation-empty" data-conversation-worktree-empty>
              该工作树暂无历史对话
            </p>
          ) : (
            <ul className="v5-conversation-branches" data-conversation-branches aria-label="当前工作树对话流">
              {activeGroup.branches.map((target) => renderBranchButton(target, true))}
            </ul>
          )}
        </>
      ) : null}
      {groups.otherGroups.map((group) => {
        const expanded = expandedWorktreeKeys.includes(group.key);
        const status = group.loading ? "loading" : group.error ? "error" : group.loaded ? "ready" : "idle";
        return (
          <div
            key={group.key}
            className="v5-conversation-worktree-group"
            data-conversation-worktree-group
            data-conversation-worktree-group-key={group.key}
            data-conversation-worktree-group-expanded={expanded ? "true" : "false"}
            data-conversation-worktree-group-status={status}
            data-conversation-worktree-group-head={group.headCommitID}
          >
            <button
              type="button"
              className="v5-conversation-worktree-toggle"
              data-conversation-worktree-toggle
              aria-expanded={expanded}
              title={group.projectFilePath || group.path}
              onClick={() => onToggleWorktree(group.key, group.projectFilePath)}
            >
              <span className="v5-conversation-worktree-chevron" aria-hidden="true">
                {expanded ? "▾" : "▸"}
              </span>
              <span className="v5-conversation-worktree-label" data-conversation-worktree-group-label>
                {group.label}
              </span>
              <span className="v5-conversation-worktree-path" data-conversation-worktree-group-path>
                {group.projectFilePath || group.path}
              </span>
              <span className="v5-conversation-worktree-branch" data-conversation-worktree-group-branch>
                {group.currentBranch
                  ? `当前分支 ${group.currentBranch}`
                  : group.headCommitID
                    ? `head ${shortCommit(group.headCommitID)}`
                    : "分支未知"}
              </span>
            </button>
            {expanded ? (
              <>
              {group.error ? (
                <div className="v5-conversation-worktree-error" data-conversation-worktree-error>
                  <span>无法读取该工作树历史。</span>
                  <span className="v5-conversation-worktree-error-text" data-conversation-worktree-error-text>
                    {group.error}
                  </span>
                  <button
                    type="button"
                    className="v5-btn v5-btn--secondary"
                    data-conversation-worktree-retry
                    onClick={() => onRefreshWorktree(group.key, group.projectFilePath)}
                  >
                    重试
                  </button>
                </div>
              ) : group.loading ? (
                <p className="v5-conversation-worktree-loading" data-conversation-worktree-loading>
                  正在读取工作树历史…
                </p>
              ) : group.branches.length === 0 ? (
                <p className="v5-conversation-empty" data-conversation-worktree-empty>
                  该工作树暂无历史对话
                </p>
              ) : (
                <ul className="v5-conversation-branches" data-conversation-branches aria-label={`${group.label} 对话流`}>
                  {group.branches.map((target) => renderBranchButton(target, false))}
                </ul>
              )}
              {!group.loading && !group.error ? (
                <div className="v5-conversation-worktree-actions" data-conversation-worktree-actions>
                  <button
                    type="button"
                    className="v5-btn v5-btn--text"
                    data-conversation-worktree-refresh
                    disabled={busy}
                    onClick={() => onRefreshWorktree(group.key, group.projectFilePath)}
                  >
                    刷新
                  </button>
                </div>
              ) : null}
              </>
            ) : null}
          </div>
        );
      })}
      {groups.otherBranches.length > 0 ? (
        <div className="v5-conversation-other-branches" data-conversation-other-branches>
          <div className="v5-conversation-other-head" data-conversation-other-head>
            <span>其他分支</span>
            <span className="v5-conversation-other-note">未归属任何工作树，只读</span>
          </div>
          <ul className="v5-conversation-branches" data-conversation-branches aria-label="其他分支">
            {groups.otherBranches.map((target) => {
              const previewing = isPreviewing(target);
              return (
                <li key={`other:${target.branch}`} className="v5-conversation-branch-item">
                  <button
                    type="button"
                    className={["v5-conversation-branch", "v5-conversation-branch--other", previewing ? "v5-conversation-branch--preview" : ""]
                      .filter(Boolean)
                      .join(" ")}
                    data-conversation-other-branch={target.branch}
                    data-conversation-branch-preview={previewing ? "true" : "false"}
                    title={target.subtitle}
                    onClick={() => onSelectBranch(target)}
                  >
                    <span className="v5-conversation-branch-title" data-conversation-branch-title>
                      {target.title}
                    </span>
                    <span className="v5-conversation-branch-subtitle" data-conversation-branch-subtitle>
                      {target.subtitle}
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
        </div>
      ) : null}
      {checkoutBlocked ? (
        <p className="v5-conversation-blocked" data-conversation-blocked>
          Agent Turn 正在运行；请先 Stop Turn 再继续或新建对话。
        </p>
      ) : null}
      {error ? (
        <p className="v5-conversation-error" data-conversation-error>
          {error}
        </p>
      ) : null}
      {busy ? (
        <p className="v5-conversation-busy" data-conversation-busy>
          正在处理…
        </p>
      ) : null}
    </section>
  );
}

export default ConversationRail;

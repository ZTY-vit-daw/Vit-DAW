/*
 * CONV-RAIL-1（418，B2a）：工程面左栏「对话流」树（工作区→当前工作树→分支）。
 *
 * 纯呈现组件：数据全部来自 props（ProjectSurface 经 /agent/ui/state 注入），
 * 组件自身零请求、零动作——点击只把分支名交给上层：
 * - 当前分支 → 上层清除预览；非当前分支 → 上层进入只读预览（不调 checkout）；
 * - 【＋ 新对话】→ 上层 branch_create（从当前活动节点分叉）。
 * 动作态（busy/checkout_blocked/error）由上层回灌，按钮禁用与原因文案在本组件呈现。
 *
 * 渲染位置：ProjectSurface 经 createPortal 注入 EntryShell 的左栏体槽
 * （data-entry-rail-slot）；无 rail context（fixture/gallery/静态渲染）时不渲染，
 * 既有渲染树逐字不变。
 */

import { useMemo } from "react";
import type { AgentUIState } from "../types";
import { conversationRailView } from "./conversationData";
import "./conversationRail.css";

export interface ConversationRailProps {
  /** /agent/ui/state 投影（null=尚未取到，左栏显示空态）。 */
  uiState: AgentUIState | null;
  /** 正在预览的分支名（null=未预览，当前对话）。 */
  previewBranch: string | null;
  /** agent Turn 运行中：切换/新建对话禁用（与 HistoryPane 同守卫）。 */
  checkoutBlocked: boolean;
  /** 动作进行中：禁止重复点击。 */
  busy: boolean;
  /** 最近一次动作失败原因（保留预览并显示，不自动重试）。 */
  error: string;
  /** 点击分支行：当前分支=清预览；非当前=只读预览。 */
  onSelectBranch: (branch: string) => void;
  /** 【＋ 新对话】：从当前活动节点分叉。 */
  onNewConversation: () => void;
}

export function ConversationRail({
  uiState,
  previewBranch,
  checkoutBlocked,
  busy,
  error,
  onSelectBranch,
  onNewConversation
}: ConversationRailProps) {
  const view = useMemo(() => conversationRailView(uiState), [uiState]);
  // SHELL-PROJECT-STREAM-FIX-1（427）：无活动节点不再是禁用理由——只要存在工程身份
  // （project_path/active branch/head），新对话即可从当前 HEAD 分叉（agent 侧
  // version.branch_create 省略 from_node_id 时按当前 HEAD 建分支）。
  const newDisabled = busy || checkoutBlocked || !view.hasProjectIdentity;
  const newHint = checkoutBlocked
    ? "Agent Turn 正在运行；请先 Stop Turn 再新建对话。"
    : view.activeNodeId
      ? "从当前对话的活动节点分叉出新对话"
      : "从当前工程状态新建对话";
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
      {view.worktree ? (
        <div
          className="v5-conversation-worktree"
          data-conversation-worktree
          data-conversation-worktree-active={view.worktree.active ? "true" : "false"}
          data-conversation-worktree-path={view.worktree.projectFilePath}
          title={view.worktree.projectFilePath || view.worktree.path}
        >
          <span className="v5-conversation-worktree-label" data-conversation-worktree-label>
            {view.worktree.label}
          </span>
          <span className="v5-conversation-worktree-path" data-conversation-worktree-path>
            {view.worktree.projectFilePath || view.worktree.path}
          </span>
        </div>
      ) : null}
      {view.branches.length === 0 ? (
        <p className="v5-conversation-empty" data-conversation-empty>
          暂无历史对话
        </p>
      ) : (
        <ul className="v5-conversation-branches" data-conversation-branches aria-label="当前工作树对话流">
          {view.branches.map((branch) => {
            const previewing = branch.name === previewBranch;
            return (
              <li key={branch.name} className="v5-conversation-branch-item">
                <button
                  type="button"
                  className={[
                    "v5-conversation-branch",
                    branch.active ? "v5-conversation-branch--active" : "",
                    previewing ? "v5-conversation-branch--preview" : ""
                  ]
                    .filter(Boolean)
                    .join(" ")}
                  data-conversation-branch={branch.name}
                  data-conversation-branch-active={branch.active ? "true" : "false"}
                  data-conversation-branch-preview={previewing ? "true" : "false"}
                  aria-current={branch.active ? "true" : undefined}
                  title={branch.subtitle}
                  onClick={() => onSelectBranch(branch.name)}
                >
                  <span className="v5-conversation-branch-title" data-conversation-branch-title>
                    {branch.title}
                  </span>
                  <span className="v5-conversation-branch-subtitle" data-conversation-branch-subtitle>
                    {branch.subtitle}
                  </span>
                  {branch.active ? (
                    <span className="v5-conversation-branch-current" data-conversation-branch-current>
                      当前
                    </span>
                  ) : null}
                </button>
              </li>
            );
          })}
        </ul>
      )}
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

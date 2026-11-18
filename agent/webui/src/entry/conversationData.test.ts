/*
 * CONV-RAIL-1（418，B2a）：对话流左栏纯数据层单测。
 *
 * fixture 纪律：本文件所有工程名/分支名/消息均为测试 fixture，不代表真实工程或
 * 真实 agent 数据；不把示例工程名/轨道名硬编码进生产代码。
 */

import { describe, expect, it } from "vitest";
import type { AgentUIState, JsonRecord } from "../types";
import {
  conversationActiveBranchName,
  conversationActiveNodeCommitID,
  conversationActiveNodeId,
  conversationBranchMessages,
  conversationBranchNames,
  conversationBranchRows,
  conversationBranchTitle,
  conversationRailView,
  conversationWorktree,
  formatConversationActivity,
  nextConversationBranchName
} from "./conversationData";

function graphNode(
  id: string,
  kind: string,
  text: string,
  branch: string,
  commitID: string,
  parentNodeID: string,
  createdAt: string
): JsonRecord {
  return {
    id,
    kind,
    text,
    text_preview: text,
    branch,
    commit_id: commitID,
    parent_node_id: parentNodeID,
    created_at: createdAt
  };
}

/** 两条对话流：main（当前，4 条消息）+ 从 base-vit 分叉的 feature-x（2 条消息）。 */
function conversationHistory(): JsonRecord {
  return {
    project_path: "C:/fixture/alpha.vit",
    root_project_path: "C:/fixture/alpha.vit",
    active_branch: "main",
    active_node_id: "n-main-vit",
    active_worktree: "",
    detached: false,
    branches: { main: "c4", "feature-x": "f2" },
    worktrees: [],
    conversation_graph: {
      project_path: "C:/fixture/alpha.vit",
      active_branch: "main",
      active_node_id: "n-main-vit",
      nodes: [
        graphNode("n-base-ask", "ask", "把贝斯轨压低 2dB", "main", "c1", "", "2026-11-18T19:00:00"),
        graphNode("n-base-vit", "vit", "已把贝斯轨压低 2dB。", "main", "c2", "n-base-ask", "2026-11-18T19:01:00"),
        graphNode("n-main-ask", "ask", "把鼓组整体再亮一点", "main", "c3", "n-base-vit", "2026-11-18T19:05:00"),
        graphNode("n-main-vit", "vit", "鼓组高频已提升。", "main", "c4", "n-main-ask", "2026-11-18T19:06:00"),
        graphNode("n-fx-marker", "branch_marker", "branch feature-x", "feature-x", "c2", "n-base-vit", "2026-11-18T19:10:00"),
        graphNode("n-fx-ask", "ask", "贝斯再加一点压缩", "feature-x", "f1", "n-fx-marker", "2026-11-18T19:11:00"),
        graphNode("n-fx-vit", "vit", "贝斯压缩已加 2:1。", "feature-x", "f2", "n-fx-ask", "2026-11-18T19:12:00")
      ]
    }
  };
}

function uiState(history: JsonRecord): AgentUIState {
  return { status: "ok", checkout_blocked: false, project_history: history };
}

describe("conversationRailView：当前工作树 + 分支树", () => {
  it("左栏树 = 当前工作树分组 + 分支行（当前分支高亮，顺序稳定）", () => {
    const view = conversationRailView(uiState(conversationHistory()));
    expect(view.worktree?.label).toBe("主工作树");
    expect(view.worktree?.projectFilePath).toBe("C:/fixture/alpha.vit");
    expect(view.worktree?.active).toBe(true);
    expect(view.branches.map((branch) => branch.name)).toEqual(["feature-x", "main"]);
    expect(view.branches.find((branch) => branch.name === "main")?.active).toBe(true);
    expect(view.branches.find((branch) => branch.name === "feature-x")?.active).toBe(false);
    expect(view.activeBranch).toBe("main");
    expect(view.activeNodeId).toBe("n-main-vit");
    expect(view.hasHistory).toBe(true);
  });

  it("分支行标题 = 分支自身片段首条有意义用户消息；副标 = 分支名 · 最近活动", () => {
    const history = conversationHistory();
    const rows = conversationBranchRows(history);
    const main = rows.find((row) => row.name === "main");
    const feature = rows.find((row) => row.name === "feature-x");
    expect(main?.title).toBe("把贝斯轨压低 2dB");
    // feature-x 自身片段的第一条用户消息（不取继承的 main 片段）。
    expect(feature?.title).toBe("贝斯再加一点压缩");
    expect(main?.subtitle).toBe("main · 当前对话");
    expect(feature?.subtitle).toContain("feature-x · 最近活动 ");
    expect(feature?.subtitle).toContain("11-18 19:12");
    expect(feature?.head).toBe("f2");
    expect(feature?.headNodeId).toBe("n-fx-vit");
    expect(feature?.nodeCount).toBe(3);
    expect(feature?.previewAvailable).toBe(true);
  });

  it("新建但尚无用户消息的分支显示「新对话」；只读预览=分叉点继承路径（同 agent 口径）", () => {
    const history = conversationHistory();
    const graph = history.conversation_graph as JsonRecord;
    (graph.nodes as JsonRecord[]).push(
      graphNode("n-new-marker", "branch_marker", "branch chat-20261118-200311", "chat-20261118-200311", "c4", "n-main-vit", "2026-11-18T20:03:11")
    );
    (history.branches as JsonRecord)["chat-20261118-200311"] = "c4";
    expect(conversationBranchTitle(history, "chat-20261118-200311")).toBe("新对话");
    // 分支 head 回溯 parent 链：新分支的消息路径 = 分叉点之前继承的对话（marker 本身
    // 不是消息）；与 agent conversationMessagesForGraph 对空分支的口径一致，不伪造新消息。
    expect(conversationBranchMessages(history, "chat-20261118-200311").map((message) => message.id)).toEqual([
      "n-base-ask",
      "n-base-vit",
      "n-main-ask",
      "n-main-vit"
    ]);
    const row = conversationBranchRows(history).find(
      (branch) => branch.name === "chat-20261118-200311"
    );
    expect(row?.previewAvailable).toBe(true);
    expect(row?.subtitle).toContain("chat-20261118-200311 · 最近活动 ");
  });

  it("空历史 → 无工作树、无分支（左栏「暂无历史对话」）", () => {
    const view = conversationRailView(uiState({}));
    expect(view.worktree).toBeNull();
    expect(view.branches).toEqual([]);
    expect(view.hasHistory).toBe(false);
    expect(conversationRailView(null).hasHistory).toBe(false);
  });

  it("工作树缺 worktrees 记录时从 project_path 回退为「主工作树」", () => {
    const worktree = conversationWorktree({ project_path: "C:/fixture/alpha.vit" });
    expect(worktree?.label).toBe("主工作树");
    expect(worktree?.projectFilePath).toBe("C:/fixture/alpha.vit");
    expect(worktree?.active).toBe(true);
  });
});

describe("conversationBranchMessages：只读预览路径", () => {
  it("从分支 head 回溯 parent 链，ask→user / vit→vit，跳过分支点与快照节点", () => {
    const messages = conversationBranchMessages(conversationHistory(), "feature-x");
    expect(messages.map((message) => message.id)).toEqual([
      "n-base-ask",
      "n-base-vit",
      "n-fx-ask",
      "n-fx-vit"
    ]);
    expect(messages.map((message) => message.author)).toEqual(["user", "vit", "user", "vit"]);
    expect(messages[2].text).toBe("贝斯再加一点压缩");
  });

  it("当前分支预览 = 当前活动节点路径（与 agent conversation_messages 同口径）", () => {
    const messages = conversationBranchMessages(conversationHistory(), "main");
    expect(messages.map((message) => message.id)).toEqual([
      "n-base-ask",
      "n-base-vit",
      "n-main-ask",
      "n-main-vit"
    ]);
    expect(messages.every((message) => message.author === "user" || message.author === "vit")).toBe(true);
  });

  it("不存在的分支 → 空预览（不伪造消息）", () => {
    expect(conversationBranchMessages(conversationHistory(), "no-such-branch")).toEqual([]);
  });
});

describe("conversationActive*：活动分支/节点/commit", () => {
  it("活动分支/节点读取 history 字段，缺省回退 graph", () => {
    const history = conversationHistory();
    expect(conversationActiveBranchName(history)).toBe("main");
    expect(conversationActiveNodeId(history)).toBe("n-main-vit");
    expect(conversationActiveNodeCommitID(history)).toBe("c4");
    const graphOnly: JsonRecord = {
      conversation_graph: {
        active_branch: "graph-branch",
        active_node_id: "n-graph"
      }
    };
    expect(conversationActiveBranchName(graphOnly)).toBe("graph-branch");
    expect(conversationActiveNodeId(graphOnly)).toBe("n-graph");
    expect(conversationActiveNodeCommitID(graphOnly)).toBe("");
  });

  it("detached 状态不冒充普通分支", () => {
    expect(conversationActiveBranchName({ detached: true, active_branch: "main" })).toBe("detached");
  });

  it("分支名清单：branches 键排序；缺 branches 时从节点回退", () => {
    expect(conversationBranchNames(conversationHistory())).toEqual(["feature-x", "main"]);
    const fromNodes: JsonRecord = {
      conversation_graph: {
        nodes: [
          graphNode("a", "ask", "一", "beta", "c1", "", "2026-11-18T19:00:00"),
          graphNode("b", "ask", "二", "alpha", "c2", "a", "2026-11-18T19:01:00")
        ]
      }
    };
    expect(conversationBranchNames(fromNodes)).toEqual(["alpha", "beta"]);
  });
});

describe("formatConversationActivity：最近活动展示", () => {
  it("本地时间稳定格式化；空/非法返回空串（不伪造活动时间）", () => {
    expect(formatConversationActivity("2026-11-18T19:12:00")).toBe("11-18 19:12");
    expect(formatConversationActivity("")).toBe("");
    expect(formatConversationActivity("not-a-date")).toBe("");
  });
});

describe("nextConversationBranchName：确定性新对话分支名", () => {
  it("时间戳命名，撞名追加 -2/-3 后缀", () => {
    const now = new Date(2026, 10, 18, 20, 3, 11);
    expect(nextConversationBranchName([], now)).toBe("chat-20261118-200311");
    expect(nextConversationBranchName(["main", "chat-20261118-200311"], now)).toBe("chat-20261118-200311-2");
    expect(
      nextConversationBranchName(["chat-20261118-200311", "chat-20261118-200311-2"], now)
    ).toBe("chat-20261118-200311-3");
  });
});

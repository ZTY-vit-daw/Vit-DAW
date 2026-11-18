/*
 * CONV-RAIL-CROSS-1（421，B2b）：工作树分组 + 分支归属过滤 + 跨工作树分支目标 单测。
 * 纯函数：零请求、零 DOM；fixture 全部为测试数据。
 */

import { describe, expect, it } from "vitest";
import type { JsonRecord } from "../types";
import {
  conversationTargetIsActiveBranch,
  conversationWorktreeGroups as groups,
  OTHER_BRANCHES_KEY
} from "./conversationData";

const ROOT = "C:/fixture/alpha.vit";
const BETA = "C:/fixture/worktrees/wt-beta/alpha_wt-beta.vit";
const GAMMA = "C:/fixture/worktrees/wt-gamma/alpha_wt-gamma.vit";

function node(
  id: string,
  kind: string,
  text: string,
  branch: string,
  commitID: string,
  parentNodeID: string,
  createdAt = "2026-11-18T19:00:00"
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

function worktreeRow(name: string, projectFilePath: string, head: string): JsonRecord {
  return {
    name,
    path: projectFilePath.replace(/\/[^/]+$/, ""),
    project_file_path: projectFilePath,
    commit_id: head,
    origin_commit_id: head,
    worktree_head_commit_id: head
  };
}

/** 根工作树 history：main（active）+ feature-x（本树对话），另有 repo 级分支 refs。 */
function rootHistory(options: { worktrees?: JsonRecord[] } = {}): JsonRecord {
  return {
    project_path: ROOT,
    root_project_path: ROOT,
    active_branch: "main",
    active_node_id: "n-main-vit",
    active_worktree: "",
    detached: false,
    head: "c4",
    branches: { main: "c4", "feature-x": "f2", shared: "s1", "beta-topic": "w1", orphan: "z9" },
    worktrees: options.worktrees ?? [],
    conversation_graph: {
      project_path: ROOT,
      active_branch: "main",
      active_node_id: "n-main-vit",
      nodes: [
        node("n-base-ask", "ask", "把贝斯轨压低 2dB", "main", "c1", ""),
        node("n-main-vit", "vit", "已压低。", "main", "c4", "n-base-ask"),
        node("n-fx-ask", "ask", "贝斯再加一点压缩", "feature-x", "f2", "n-base-ask")
      ]
    }
  };
}

/** wt-beta 的 version.status 投影：active_branch=shared（head 与 shared ref 不同，测 active 优先）。 */
function betaHistory(): JsonRecord {
  return {
    project_path: BETA,
    root_project_path: ROOT,
    active_branch: "shared",
    active_node_id: "n-beta-shared-vit",
    active_worktree: "wt-beta",
    detached: false,
    head: "w1",
    branches: { main: "b1", "beta-topic": "w1", shared: "s1" },
    worktrees: [],
    conversation_graph: {
      project_path: BETA,
      active_branch: "shared",
      active_node_id: "n-beta-shared-vit",
      nodes: [
        node("n-beta-topic-ask", "ask", "在这个工作树里写新鼓组", "beta-topic", "w0", ""),
        node("n-beta-topic-vit", "vit", "鼓组片段已写入工作树副本。", "beta-topic", "w1", "n-beta-topic-ask"),
        node("n-beta-shared-ask", "ask", "把这段鼓组再压紧一点", "shared", "s0", ""),
        node("n-beta-shared-vit", "vit", "已压紧。", "shared", "s1", "n-beta-shared-ask")
      ]
    }
  };
}

function groupedHistory(): JsonRecord {
  return rootHistory({ worktrees: [worktreeRow("wt-beta", BETA, "w1"), worktreeRow("wt-gamma", GAMMA, "g1")] });
}

function branchNamesByGroup(view: ReturnType<typeof groups>): string[][] {
  return [
    view.activeGroup?.branches.map((branch) => branch.name) ?? [],
    ...view.otherGroups.map((group) => group.branches.map((branch) => branch.name)),
    view.otherBranches.map((branch) => branch.name)
  ];
}

describe("conversationWorktreeGroups：工作树分组与懒加载态", () => {
  it("多工作树：当前组 + 其它工作树组（未加载时分支隐藏、head 来自 worktree 行）", () => {
    const view = groupedHistory();
    const groupsView = groups(view);
    expect(groupsView.activeGroup?.label).toBe("主工作树");
    expect(groupsView.activeGroup?.branches.map((branch) => branch.name)).toEqual(["feature-x", "main"]);
    expect(groupsView.otherGroups.map((group) => group.label)).toEqual(["wt-beta", "wt-gamma"]);
    const beta = groupsView.otherGroups.find((group) => group.name === "wt-beta");
    expect(beta?.loaded).toBe(false);
    expect(beta?.branches).toEqual([]);
    expect(beta?.headCommitID).toBe("w1");
    expect(beta?.currentBranch).toBe("");
    // 工作区级其他分支：无归属的 repo 级 ref（orphan/shared 加载前暂未归属）。
    expect(groupsView.otherBranches.map((branch) => branch.name)).toEqual(["orphan", "shared"]);
  });

  it("加载其它工作树 status 后：active_branch 所有者优先，head 归属与图证据归位", () => {
    const view = groupedHistory();
    const groupsView = groups(view, {
      [groups(view).otherGroups[0].key]: { history: betaHistory() }
    });
    const beta = groupsView.otherGroups.find((group) => group.name === "wt-beta");
    expect(beta?.loaded).toBe(true);
    expect(beta?.currentBranch).toBe("shared");
    // shared：head s1 与 wt-beta head w1 不同，仅因 active_branch 归属 wt-beta（规则①）。
    expect(beta?.branches.map((branch) => branch.name)).toEqual(["beta-topic", "shared"]);
    // beta-topic：head w1 == worktree head（规则②）。
    expect(beta?.branches.find((branch) => branch.name === "beta-topic")?.headNodeId).toBe("n-beta-topic-vit");
    // 当前工作树组不重复展示已归属其它工作树的分支（规则收口）。
    expect(groupsView.activeGroup?.branches.map((branch) => branch.name)).toEqual(["feature-x", "main"]);
    // 其他分支组只剩真正无法归属的 orphan（只读）。
    expect(groupsView.otherBranches.map((branch) => branch.name)).toEqual(["orphan"]);
  });

  it("同一分支只出现在一个分组（唯一性）且工作区级其他分支为只读", () => {
    const view = groupedHistory();
    const statusKey = groups(view).otherGroups.find((group) => group.name === "wt-beta")?.key ?? "";
    const groupsView = groups(view, { [statusKey]: { history: betaHistory() } });
    const flat = branchNamesByGroup(groupsView).flat();
    expect(new Set(flat).size).toBe(flat.length);
    expect(flat).toContain("shared");
    expect(flat).toContain("feature-x");
    expect(flat).toContain("orphan");
    expect(flat.filter((name) => name === "beta-topic")).toHaveLength(1);
    const orphan = groupsView.otherBranches[0];
    expect(orphan.readOnly).toBe(true);
    expect(orphan.worktreeKey).toBe(OTHER_BRANCHES_KEY);
    expect(orphan.projectFilePath).toBe("");
    const betaShared = groupsView.otherGroups
      .find((group) => group.name === "wt-beta")
      ?.branches.find((branch) => branch.name === "shared");
    expect(betaShared?.isActiveWorktree).toBe(false);
    expect(betaShared?.readOnly).toBe(false);
    expect(betaShared?.worktreeKey).toBe(statusKey);
    expect(betaShared?.projectFilePath).toBe(BETA);
  });

  it("加载态/错误态透传：错误时组内显示原因、不伪造分支", () => {
    const view = groupedHistory();
    const betaKey = groups(view).otherGroups.find((group) => group.name === "wt-beta")?.key ?? "";
    const loading = groups(view, { [betaKey]: { loading: true } });
    expect(loading.otherGroups.find((group) => group.name === "wt-beta")).toMatchObject({
      loading: true,
      loaded: false,
      error: "",
      branches: []
    });
    const failed = groups(view, { [betaKey]: { error: "status unavailable (fixture)" } });
    expect(failed.otherGroups.find((group) => group.name === "wt-beta")).toMatchObject({
      loading: false,
      loaded: false,
      error: "status unavailable (fixture)",
      branches: []
    });
  });

  it("head 命中多个工作树时视为无法归属 → 工作区级其他分支（不猜）", () => {
    const history = rootHistory({
      worktrees: [worktreeRow("wt-a", "C:/fixture/worktrees/wt-a/a.vit", "dup"), worktreeRow("wt-b", "C:/fixture/worktrees/wt-b/b.vit", "dup")]
    });
    (history.branches as JsonRecord)["dup-branch"] = "dup";
    const groupsView = groups(history);
    expect(groupsView.otherBranches.map((branch) => branch.name)).toContain("dup-branch");
    expect(groupsView.activeGroup?.branches.map((branch) => branch.name)).not.toContain("dup-branch");
  });

  it("其它工作树的 active_branch 从当前组过滤（避免同一分支重复）", () => {
    // wt-beta 的 conversation_open 已把 feature-x 打开为 active（模拟切换后的 status）。
    const beta = betaHistory();
    beta.active_branch = "feature-x";
    beta.conversation_graph = {
      nodes: [node("n-beta-feature", "ask", "在 beta 继续这条对话", "feature-x", "x1", "")]
    };
    const view = groupedHistory();
    const betaKey = groups(view).otherGroups.find((group) => group.name === "wt-beta")?.key ?? "";
    const groupsView = groups(view, { [betaKey]: { history: beta } });
    expect(groupsView.otherGroups.find((group) => group.name === "wt-beta")?.branches.map((branch) => branch.name)).toContain("feature-x");
    expect(groupsView.activeGroup?.branches.map((branch) => branch.name)).not.toContain("feature-x");
  });

  it("单工作树项目：有对话节点的分支留在当前组（B2a 连续性），仅有 ref 的分支进其他分支组", () => {
    const history = rootHistory();
    const groupsView = groups(history);
    expect(groupsView.activeGroup?.branches.map((branch) => branch.name)).toEqual(["feature-x", "main"]);
    expect(groupsView.otherGroups).toEqual([]);
    expect(groupsView.otherBranches.map((branch) => branch.name)).toEqual(["beta-topic", "orphan", "shared"]);
    expect(groupsView.activeBranch).toBe("main");
    expect(groupsView.activeNodeId).toBe("n-main-vit");
  });

  it("空历史 → 无组、空态（不伪造工作树/分支）", () => {
    const groupsView = groups({});
    expect(groupsView.activeGroup).toBeNull();
    expect(groupsView.otherGroups).toEqual([]);
    expect(groupsView.otherBranches).toEqual([]);
    expect(groupsView.hasHistory).toBe(false);
  });
});

describe("conversationTargetIsActiveBranch：continue vs conversation_open 判据", () => {
  it("当前工作树当前分支 = 现有【继续此对话】；其它工作树/其它分支 = 跨工作树动作", () => {
    const view = groupedHistory();
    const activeGroup = groups(view).activeGroup;
    const main = activeGroup?.branches.find((branch) => branch.name === "main");
    const feature = activeGroup?.branches.find((branch) => branch.name === "feature-x");
    expect(main && activeGroup ? conversationTargetIsActiveBranch(groups(view), main) : false).toBe(true);
    expect(feature && activeGroup ? conversationTargetIsActiveBranch(groups(view), feature) : true).toBe(false);
    const betaKey = groups(view).otherGroups.find((group) => group.name === "wt-beta")?.key ?? "";
    const betaView = groups(view, { [betaKey]: { history: betaHistory() } });
    const cross = betaView.otherGroups.find((group) => group.name === "wt-beta")?.branches[0];
    expect(cross ? conversationTargetIsActiveBranch(betaView, cross) : true).toBe(false);
  });
});

import { describe, expect, it } from "vitest";
import {
  classifyHistoryScopeChange,
  concreteWorkspacePath,
  historyScopeKeyFromParts,
  historyScopePartsFromUIState,
  shouldAdoptStoredConversationOnScopeEvolution,
  type HistoryScopeParts
} from "./historyScope";
import {
  loadStoredConversationMessages,
  loadStoredScopedConversationID,
  migrateStoredConversationScope,
  resolveHistorySyncMessages,
  saveStoredConversationMessages,
  saveStoredScopedConversationID
} from "./App";
import type { AgentUIState, ChatMessage } from "./types";

function uiStateWithProjectHistory(projectHistory: Record<string, unknown>): AgentUIState {
  return { project_history: projectHistory };
}

function parts(overrides: Partial<HistoryScopeParts> = {}): HistoryScopeParts {
  return {
    projectPath: "",
    rootProjectPath: "",
    projectUUID: "",
    stateDir: "",
    historyDir: "",
    activeWorktree: "",
    activeBranch: "",
    ...overrides
  };
}

function chatMessage(id: string, role: ChatMessage["role"], content: string, createdAt: number): ChatMessage {
  return { id, role, content, createdAt, status: "sent" } as ChatMessage;
}

function chainResultMessage(id: string, content: string, createdAt: number): ChatMessage {
  return {
    ...chatMessage(id, "assistant", content, createdAt),
    source_id: id,
    lifecycle: "transient",
    persistence: "none",
    message_kind: "activity"
  } as ChatMessage;
}

// 终审复现 fixture（2026-09-05）：新会话发诊断 → 链运行中首批 checkpoint 落地，
// state_dir/history_dir/active_worktree/active_branch 从空物化为真实值，
// project_path 全程稳定（内核状态提供）。
const preMaterializationState = uiStateWithProjectHistory({
  project_path: "D:/Vit_DAW/VitApp/Workspace/demo.vitproj",
  state_dir: "",
  history_dir: "",
  active_worktree: "",
  active_branch: ""
});
const postMaterializationState = uiStateWithProjectHistory({
  project_path: "D:/Vit_DAW/VitApp/Workspace/demo.vitproj",
  state_dir: "D:/Vit_DAW/VitApp/Workspace/.vit/state",
  history_dir: "D:/Vit_DAW/VitApp/Workspace/.vit/history",
  active_worktree: "wt_demo_20260905",
  active_branch: "vit/demo",
  project_uuid: "uuid-demo-1"
});

describe("CONTRACT-3 history scope", () => {
  it("①会话中途 state_dir/worktree/branch 物化不换键、不判切换，消息流与事件 seq 语义保留", () => {
    const before = historyScopePartsFromUIState(preMaterializationState);
    const after = historyScopePartsFromUIState(postMaterializationState);
    expect(after.stateDir).not.toBe("");
    expect(after.activeWorktree).not.toBe("");

    // 键稳定化：晚物化字段出键——物化前后键一致
    expect(historyScopeKeyFromParts(before)).toBe(historyScopeKeyFromParts(after));

    // 演进/切换分离：即便键成分（worktree/branch）变化，同工程内不得判为 switch
    const change = classifyHistoryScopeChange({
      previousKey: historyScopeKeyFromParts(before),
      previousConcretePath: concreteWorkspacePath(before),
      previousUUID: before.projectUUID,
      nextParts: after
    });
    expect(change).not.toBe("switch");

    // 消息流保留：同会话内解析为合并而非清流重置
    const currentStream = [
      chatMessage("user_1", "user", "帮我诊断这条总线", 1000),
      chatMessage("assistant_1", "assistant", "开始观察混音状态……", 2000),
      chainResultMessage("agent_event_goal_1_chain_result", "链终局回复", 3000)
    ];
    const resolved = resolveHistorySyncMessages({
      changeKind: change,
      current: currentStream,
      historyMessages: []
    });
    expect(resolved).toEqual(currentStream);
  });

  it("②真切换工程（projectPath 变化）照旧清流，旧工程终局不带入新工程", () => {
    const projectA = parts({ projectPath: "D:/Vit_DAW/VitApp/Workspace/a.vitproj" });
    const projectB = parts({ projectPath: "D:/Vit_DAW/VitApp/Workspace/b.vitproj" });
    const change = classifyHistoryScopeChange({
      previousKey: historyScopeKeyFromParts(projectA),
      previousConcretePath: concreteWorkspacePath(projectA),
      previousUUID: "",
      nextParts: projectB
    });
    expect(change).toBe("switch");

    const resolved = resolveHistorySyncMessages({
      changeKind: "switch",
      current: [
        chatMessage("user_1", "user", "工程 A 的问题", 1000),
        chainResultMessage("agent_event_goal_1_chain_result", "工程 A 的终局", 2000)
      ],
      historyMessages: []
    });
    // 清流照旧：回落问候语，不带旧工程消息
    expect(resolved).toHaveLength(1);
    expect(resolved[0].id).toContain("intro");
    expect(resolved.some((message) => message.content.includes("工程 A"))).toBe(false);
  });

  it("③演进（root 物化/键变化）重新锚定存储桶：迁移合并不丢历史消息", () => {
    const before = parts({ projectPath: "D:/work/wt_demo" }); // worktree：root 未物化
    const after = parts({
      projectPath: "D:/work/wt_demo",
      rootProjectPath: "D:/work/root_project", // 首批 checkpoint 后物化
      stateDir: "D:/work/root_project/.vit/state"
    });
    expect(historyScopeKeyFromParts(before)).not.toBe(historyScopeKeyFromParts(after));
    const change = classifyHistoryScopeChange({
      previousKey: historyScopeKeyFromParts(before),
      previousConcretePath: concreteWorkspacePath(before),
      previousUUID: "",
      nextParts: after
    });
    expect(change).toBe("evolution");

    // 演进解析：保留在流消息并合并历史投影（非清流）
    const currentStream = [
      chatMessage("user_1", "user", "继续处理", 1000),
      chatMessage("assistant_1", "assistant", "过程回复", 2000)
    ];
    const historyReplay = [chatMessage("history_n1", "assistant", "历史终局回复", 1500)];
    const resolved = resolveHistorySyncMessages({
      changeKind: change,
      current: currentStream,
      historyMessages: historyReplay
    });
    expect(resolved.some((message) => message.id === "user_1")).toBe(true);
    expect(resolved.some((message) => message.id === "assistant_1")).toBe(true);
    expect(resolved.some((message) => message.id === "history_n1")).toBe(true);

    // 存储桶迁移：旧桶已存消息 + 会话 id 迁到新桶，一条不丢
    const windowBackup = (globalThis as { window?: unknown }).window;
    const store = new Map<string, string>();
    (globalThis as { window?: unknown }).window = {
      localStorage: {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => void store.set(key, value),
        removeItem: (key: string) => void store.delete(key)
      }
    };
    try {
      const previousScope = historyScopeKeyFromParts(before);
      const nextScope = historyScopeKeyFromParts(after);
      const conversationID = "webui_scope_migration";
      saveStoredScopedConversationID(previousScope, conversationID);
      saveStoredConversationMessages(conversationID, previousScope, currentStream);

      migrateStoredConversationScope(previousScope, nextScope, conversationID);

      expect(loadStoredScopedConversationID(nextScope)).toBe(conversationID);
      const migrated = loadStoredConversationMessages(conversationID, nextScope);
      expect(migrated.some((message) => message.id === "user_1")).toBe(true);
      expect(migrated.some((message) => message.id === "assistant_1")).toBe(true);
    } finally {
      (globalThis as { window?: unknown }).window = windowBackup;
    }
  });

  it("④刷新后终局消息不消失：空 Project History 不得覆盖已回放的终局流（C3）", () => {
    const replayed = [
      chatMessage("user_1", "user", "帮我做总线诊断", 1000),
      chainResultMessage("agent_event_goal_1_chain_result", "链终局回复（事件重放）", 3000)
    ];
    // 刷新后 scope 首锚迟到：Project History 尚空（服务端历史写入滞后/未落），
    // 已由事件重放交付的终局必须在场
    const resolved = resolveHistorySyncMessages({
      changeKind: "initial",
      current: replayed,
      historyMessages: []
    });
    expect(resolved).toEqual(replayed);

    // Project History 到位后：以历史为恢复权威，同时幂等带回事件路径终局（GUI-1 ② 语义）
    const historyNow = [chatMessage("history_n1", "assistant", "历史里的终局回复", 1500)];
    const resolvedWithHistory = resolveHistorySyncMessages({
      changeKind: "initial",
      current: replayed,
      historyMessages: historyNow
    });
    expect(resolvedWithHistory.some((message) => message.id === "history_n1")).toBe(true);
    expect(
      resolvedWithHistory.some((message) => (message.source_id ?? message.id) === "agent_event_goal_1_chain_result")
    ).toBe(true);
  });

  it("工程关闭再打开另一工程（unsaved 中转）仍判为切换", () => {
    const projectA = parts({ projectPath: "D:/work/a.vitproj" });
    const unsaved = parts({});
    const projectB = parts({ projectPath: "D:/work/b.vitproj" });
    let key = historyScopeKeyFromParts(projectA);
    let concrete = concreteWorkspacePath(projectA);
    let uuid = "";
    let change = classifyHistoryScopeChange({ previousKey: key, previousConcretePath: concrete, previousUUID: uuid, nextParts: unsaved });
    expect(change).toBe("evolution"); // 关闭工程不清流
    const unsavedConcrete = concreteWorkspacePath(unsaved);
    if (unsavedConcrete) {
      concrete = unsavedConcrete;
    }
    key = historyScopeKeyFromParts(unsaved);
    change = classifyHistoryScopeChange({ previousKey: key, previousConcretePath: concrete, previousUUID: uuid, nextParts: projectB });
    expect(change).toBe("switch"); // 换到另一工程必须清流
  });

  it("同路径下 project uuid 更换（工程重建）判为切换", () => {
    const before = parts({ projectPath: "D:/work/a.vitproj", projectUUID: "uuid-1" });
    const after = parts({ projectPath: "D:/work/a.vitproj", projectUUID: "uuid-2" });
    const change = classifyHistoryScopeChange({
      previousKey: historyScopeKeyFromParts(before),
      previousConcretePath: concreteWorkspacePath(before),
      previousUUID: before.projectUUID,
      nextParts: after
    });
    expect(change).toBe("switch");
  });

  it("uuid 首次物化不产生抖动（键不变、判 none）", () => {
    const before = parts({ projectPath: "D:/work/a.vitproj" });
    const after = parts({ projectPath: "D:/work/a.vitproj", projectUUID: "uuid-1" });
    const change = classifyHistoryScopeChange({
      previousKey: historyScopeKeyFromParts(before),
      previousConcretePath: concreteWorkspacePath(before),
      previousUUID: "",
      nextParts: after
    });
    expect(change).toBe("none");
  });
});

// B9 症4（刷新后轨迹整体消失）：刷新挂载首拍 scope 未物化（unsaved::root）→
// initial 分支造新随机会话 id → 演进到真实 scope 判 evolution 保留新 id →
// 存档锚定 id 不被回读，事件回放打到空缓冲，轨迹块消失（终局消息走服务端图
// 水合独立存活）。evolution 分支的采纳判定在此钉死。
describe("B9 症4：scope 演进时采纳存档锚定会话 id（刷新轨迹水合）", () => {
  it("空流 + 存档 id 存在且不同 → 采纳（刷新恢复形态）", () => {
    expect(
      shouldAdoptStoredConversationOnScopeEvolution({
        storedConversationID: "webui_mtwwegtp",
        currentConversationID: "webui_newrandom",
        hasMeaningfulMessages: false
      })
    ).toBe(true);
  });

  it("当前流已有有效消息 → 不采纳（不劫持现役 unsaved 会话）", () => {
    expect(
      shouldAdoptStoredConversationOnScopeEvolution({
        storedConversationID: "webui_mtwwegtp",
        currentConversationID: "webui_newrandom",
        hasMeaningfulMessages: true
      })
    ).toBe(false);
  });

  it("无存档 id / 存档与当前一致 → 不采纳", () => {
    expect(
      shouldAdoptStoredConversationOnScopeEvolution({
        storedConversationID: "",
        currentConversationID: "webui_newrandom",
        hasMeaningfulMessages: false
      })
    ).toBe(false);
    expect(
      shouldAdoptStoredConversationOnScopeEvolution({
        storedConversationID: "webui_mtwwegtp",
        currentConversationID: "webui_mtwwegtp",
        hasMeaningfulMessages: false
      })
    ).toBe(false);
  });
});

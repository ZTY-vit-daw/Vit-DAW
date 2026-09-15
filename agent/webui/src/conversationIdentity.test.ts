import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  hasMeaningfulChatMessages,
  loadStoredScopedConversationID,
  migrateStoredConversationScope,
  resolveScopedConversationID,
  saveStoredScopedConversationID,
  serverConversationIDFromContinuations
} from "./App";
import { historyScopeKeyFromParts, shouldAdoptStoredConversationOnScopeEvolution, type HistoryScopeParts } from "./historyScope";
import { emptyAuditionState, reduceAuditionEvents } from "./audition";
import { emptyTrajectoryState, reduceTrajectoryEvents, trajectoryTurns } from "./trajectory";
import { AuditionJudgeCard } from "./trajectory/TrajectoryAuditionPanel";
import { TraceBlock } from "./trace/TraceBlock";
import { mockMultiRoundTrajectoryEvents } from "./trajectoryMock";
import type { ChatMessage } from "./types";

// CONV-ID-BOOT-1（卡 2026-09-14）：裸启动会话身份恢复的单元钉。
// 探针（artifacts/_live_probe/20260914_refresh_view + 本地复现）钉死的链条：
// 开机问候克隆被 hasMeaningfulChatMessages 计为有效消息 → 演进拍采纳守卫被
// 误触拒绝 → fall-through 迁移把真实 scope 桶的锚定覆写成新随机会话 id。
// 这里钉三个不变量：问候不算有效消息、迁移不覆写不同锚定、采纳守卫在
// 「仅问候在场」时必须放行恢复存档会话。

function stubLocalStorage(): Map<string, string> {
  const store = new Map<string, string>();
  vi.stubGlobal("window", {
    localStorage: {
      getItem: (key: string) => (store.has(key) ? store.get(key) as string : null),
      setItem: (key: string, value: string) => { store.set(key, String(value)); },
      removeItem: (key: string) => { store.delete(key); },
      clear: () => { store.clear(); }
    }
  });
  return store;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

const greeting: ChatMessage = {
  id: "intro_mu182lsh_x1oel4",
  role: "assistant",
  content: "Ask Vit 就绪。",
  createdAt: 1
};

const userTurn: ChatMessage = {
  id: "u1",
  role: "user",
  content: "帮低音轨做个均衡实验然后让我试听",
  createdAt: 2
};

describe("CONV-ID-BOOT-1 会话身份恢复", () => {
  it("开机问候克隆不构成有意义消息（messagesOrIntro 造的是 intro_<ts>_<rand>，不是整串 intro）", () => {
    expect(hasMeaningfulChatMessages([greeting])).toBe(false);
  });

  it("真实对话消息仍构成有意义消息（守卫不得放过现役会话被劫持）", () => {
    expect(hasMeaningfulChatMessages([greeting, userTurn])).toBe(true);
  });

  it("探针场景：仅问候在场时，演进拍必须采纳存档锚定会话", () => {
    expect(
      shouldAdoptStoredConversationOnScopeEvolution({
        storedConversationID: "webui_anchor",
        currentConversationID: "webui_randomboot",
        hasMeaningfulMessages: hasMeaningfulChatMessages([greeting])
      })
    ).toBe(true);
  });

  it("scope 演进迁移不得覆写指向其他会话的锚定（覆写即刷新丢轨迹的独占根因）", () => {
    const store = stubLocalStorage();
    const placeholderScope = "unsaved::root";
    const realScope = "D:/Godot/project/vit-daw-frontend/912.vit::root";
    saveStoredScopedConversationID(realScope, "webui_anchor");
    saveStoredScopedConversationID(placeholderScope, "webui_randomboot");

    migrateStoredConversationScope(placeholderScope, realScope, "webui_randomboot");

    expect(loadStoredScopedConversationID(realScope)).toBe("webui_anchor");
    expect(Array.from(store.keys()).filter((key) => key.includes("unsaved")).length).toBe(1);
  });

  it("空桶迁移照常落锚（真正的首次物化不受守卫影响）", () => {
    stubLocalStorage();
    const realScope = "D:/Godot/project/vit-daw-frontend/912.vit::root";
    migrateStoredConversationScope("unsaved::root", realScope, "webui_first");
    expect(loadStoredScopedConversationID(realScope)).toBe("webui_first");
  });

  it("同值锚定重写不受守卫影响（幂等迁移）", () => {
    stubLocalStorage();
    const realScope = "D:/Godot/project/vit-daw-frontend/912.vit::root";
    saveStoredScopedConversationID(realScope, "webui_same");
    migrateStoredConversationScope("unsaved::root", realScope, "webui_same");
    expect(loadStoredScopedConversationID(realScope)).toBe("webui_same");
  });
});

// REFRESH-VANISH-2（卡 2026-09-15）：Godot webview 面板销毁重建丢 localStorage 的
// 「空 storage 裸开」形态。决策侧活栈探针钉死的链条（artifacts/_live_probe/
// 20260915_retest_vanish）：消息走 uiState 服务端历史恢复故存活；轨迹块/A-B 卡走
// /agent/events 按 conversation_id 键控的内存缓冲回放，本地映射缺失时引导解析新建
// 随机会话 → 回放拉空会话 → 轨迹面全灭。服务端兜底 = runtimeStatus.continuations
// 投影（conversation_id + project_path/uuid + updated_at）按 scope 匹配出权威身份；
// 该兜底只在本地映射缺失时生效，storage 正常形态零行为变化。
describe("REFRESH-VANISH-2 空 storage 裸开的服务端会话兜底", () => {
  const liveProject = "D:/Godot/project/vit-daw-frontend/912.vit";
  const liveScopeParts: HistoryScopeParts = {
    projectPath: liveProject,
    rootProjectPath: "",
    projectUUID: "",
    stateDir: "",
    historyDir: "",
    activeWorktree: "",
    activeBranch: ""
  };
  const liveScope = historyScopeKeyFromParts(liveScopeParts);
  const continuations = [
    { conversation_id: "webui_older", project_path: "D:\\Godot\\project\\vit-daw-frontend\\912.vit", updated_at: "2026-09-14T04:00:00Z" },
    { conversation_id: "webui_real", project_path: "D:\\Godot\\project\\vit-daw-frontend\\912.vit", updated_at: "2026-09-15T04:00:00Z" },
    { conversation_id: "webui_other_project", project_path: "D:/Studio/other.vit", updated_at: "2026-09-15T05:00:00Z" }
  ];

  it("选择器：按 scope 路径匹配出最新会话身份（跨反斜杠/大小写归一，他工程不串）", () => {
    expect(serverConversationIDFromContinuations(continuations, liveScopeParts)).toBe("webui_real");
  });

  it("选择器：uuid 匹配兜底；scope 未物化或无匹配时返回空（真首次使用不受影响）", () => {
    const uuidParts: HistoryScopeParts = { ...liveScopeParts, projectPath: "", projectUUID: "vitproj_9ecc5ab9" };
    expect(serverConversationIDFromContinuations([
      { conversation_id: "webui_real", project_uuid: "VITPROJ_9ECC5AB9", updated_at: "2026-09-15T04:00:00Z" }
    ], uuidParts)).toBe("webui_real");
    expect(serverConversationIDFromContinuations(continuations, { ...liveScopeParts, projectPath: "", projectUUID: "" })).toBe("");
    expect(serverConversationIDFromContinuations(undefined, liveScopeParts)).toBe("");
  });

  it("主钉：空 storage 裸开时引导解析必须采纳服务端身份（新建随机会话=轨迹/A-B 卡消失根因）", () => {
    stubLocalStorage();
    const resolved = resolveScopedConversationID({
      urlConversationID: "",
      storedScopedConversationID: loadStoredScopedConversationID(liveScope),
      serverConversationID: serverConversationIDFromContinuations(continuations, liveScopeParts),
      freshConversationID: "webui_randomboot"
    });
    expect(resolved).toBe("webui_real");
  });

  it("storage 正常形态零行为变化：本地锚定优先于服务端兜底，URL 绑定最优先", () => {
    stubLocalStorage();
    saveStoredScopedConversationID(liveScope, "webui_anchor");
    expect(loadStoredScopedConversationID(liveScope)).toBe("webui_anchor");
    expect(resolveScopedConversationID({
      urlConversationID: "",
      storedScopedConversationID: loadStoredScopedConversationID(liveScope),
      serverConversationID: "webui_real",
      freshConversationID: "webui_randomboot"
    })).toBe("webui_anchor");
    expect(resolveScopedConversationID({
      urlConversationID: "webui_from_url",
      storedScopedConversationID: "webui_anchor",
      serverConversationID: "webui_real",
      freshConversationID: "webui_randomboot"
    })).toBe("webui_from_url");
  });

  it("演进锚定候选：本地锚定缺失时守卫采纳服务端身份（仅问候在场不劫持现役会话）", () => {
    stubLocalStorage();
    const anchorCandidate = resolveScopedConversationID({
      urlConversationID: "",
      storedScopedConversationID: loadStoredScopedConversationID(liveScope),
      serverConversationID: serverConversationIDFromContinuations(continuations, liveScopeParts),
      freshConversationID: ""
    });
    expect(shouldAdoptStoredConversationOnScopeEvolution({
      storedConversationID: anchorCandidate,
      currentConversationID: "webui_randomboot",
      hasMeaningfulMessages: hasMeaningfulChatMessages([greeting])
    })).toBe(true);
    expect(shouldAdoptStoredConversationOnScopeEvolution({
      storedConversationID: anchorCandidate,
      currentConversationID: "webui_randomboot",
      hasMeaningfulMessages: hasMeaningfulChatMessages([greeting, userTurn])
    })).toBe(false);
  });

  it("渲染面：空 storage 兜底采纳后事件回放重建轨迹块；随机 boot id 则无轨迹块", () => {
    stubLocalStorage();
    const resolved = resolveScopedConversationID({
      urlConversationID: "",
      storedScopedConversationID: loadStoredScopedConversationID(liveScope),
      serverConversationID: serverConversationIDFromContinuations(continuations, liveScopeParts),
      freshConversationID: "webui_randomboot"
    });
    // /agent/events 按会话 id 键控：模拟回放取数（服务端缓冲里只有真实会话的事件）
    const serverEvents = mockMultiRoundTrajectoryEvents.map((event) => ({ ...event, conversation_id: "webui_real" }));
    const replayed = serverEvents.filter((event) => event.conversation_id === resolved);
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), replayed);
    const markup = trajectoryTurns(state)
      .map((turn, index) => renderToStaticMarkup(createElement(TraceBlock, { key: index, state, turn, activities: [] })))
      .join("");
    expect(resolved).toBe("webui_real");
    expect(markup).toContain("trace-block");
    expect(trajectoryTurns(reduceTrajectoryEvents(emptyTrajectoryState(), serverEvents.filter((event) => event.conversation_id === "webui_randomboot")))).toHaveLength(0);
  });

  it("渲染面：空 storage 兜底采纳后 A/B 试听判定卡随回放恢复", () => {
    stubLocalStorage();
    const resolved = resolveScopedConversationID({
      urlConversationID: "",
      storedScopedConversationID: loadStoredScopedConversationID(liveScope),
      serverConversationID: serverConversationIDFromContinuations(continuations, liveScopeParts),
      freshConversationID: "webui_randomboot"
    });
    const auditionReady = {
      seq: 1,
      type: "audition.ready",
      item_id: "audition-blind",
      conversation_id: "webui_real",
      payload: {
        schema_version: "vit.kernel_audition.v1",
        session: {
          session_id: "audition-blind", conversation_id: "webui_real", turn_id: "turn-1", round_id: "round-1",
          project_revision: "rev-1", status: "ready", blind: true,
          candidates: [
            { id: "candidate-a", label: "A", status: "ready", preview_ref: "a" },
            { id: "candidate-b", label: "B", status: "ready", preview_ref: "b" }
          ]
        }
      }
    };
    const judgmentRequested = {
      seq: 2,
      type: "trajectory.user_judgment.requested",
      conversation_id: "webui_real",
      payload: { schema_version: "vit.observable_trajectory.v1", turn_id: "turn-1", round_id: "round-1", details: { audition_session_id: "audition-blind" } }
    };
    const replayed = [auditionReady, judgmentRequested].filter((event) => event.conversation_id === resolved);
    const auditionState = reduceAuditionEvents(emptyAuditionState(), replayed);
    const sessions = Object.values(auditionState.sessions);
    expect(resolved).toBe("webui_real");
    expect(sessions).toHaveLength(1);
    const markup = renderToStaticMarkup(createElement(AuditionJudgeCard, {
      trajectory: emptyTrajectoryState(),
      session: sessions[0],
      busySessionID: "",
      onSelect: async () => {},
      onStop: async () => {},
      onSubmitJudgment: async () => {}
    }));
    expect(markup).toContain("盲测 · 顺序随机");
  });
});

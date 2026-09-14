import { afterEach, describe, expect, it, vi } from "vitest";
import { hasMeaningfulChatMessages, loadStoredScopedConversationID, migrateStoredConversationScope, saveStoredScopedConversationID } from "./App";
import { shouldAdoptStoredConversationOnScopeEvolution } from "./historyScope";
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

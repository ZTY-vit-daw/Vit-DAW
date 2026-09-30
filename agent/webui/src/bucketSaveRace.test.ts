import { describe, expect, it } from "vitest";
import {
  loadStoredConversationMessages,
  mergeRestoredChatMessages,
  resolveHistorySyncMessages,
  saveStoredConversationMessages
} from "./App";
import type { ChatMessage } from "./types";

// FIX-BUCKET-SAVE-RACE-1 回归（单测层）：复刻 mini_repro 场景的数据面——
// 有图消息 + 仅存本地桶的末轮驱动消息 → reload 管线后末轮仍在。
// 端到端时序（restore 效应依赖含 messages、先于同桶首次 save）由
// E2E-WEBUI-1 断言组 bucket-save-race-F1 覆盖；本文件钉合流与桶读写语义。

const scope = "D:\\draft_root\\Unsaved.vit::root";
const conversationID = "webui_bucketrace";

// 桶读写依赖 window.localStorage（historyScope.test.ts 同款 Map 桩）。
function withStubbedStorage<T>(run: () => T): T {
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
    return run();
  } finally {
    (globalThis as { window?: unknown }).window = windowBackup;
  }
}

function graphMessage(id: string, role: ChatMessage["role"], content: string, createdAt: number): ChatMessage {
  return {
    id: `history_${id}`,
    source_id: id,
    role,
    content,
    createdAt,
    status: "sent",
    lifecycle: "durable",
    persistence: "project_history",
    message_kind: role === "user" ? "user" : "assistant",
    turn_id: `turn_${id}`,
    logical_message_id: id
  } as ChatMessage;
}

// mini_repro 形态的驱动末轮：POST /agent/chat 的乐观回复，未进服务端图，
// 只落 (conversationID, scope) 本地桶（persistence=local）。
function drivenTail(): ChatMessage[] {
  return [
    {
      id: "msg_bucketrace_ask",
      role: "user",
      content: "Track 2 的 2.1kHz 峰值从哪来？",
      createdAt: 4000,
      status: "sent",
      lifecycle: "durable",
      persistence: "local",
      message_kind: "user"
    },
    {
      id: "reply_bucketrace_local",
      role: "assistant",
      content: "OBSLOCALTAIL 本地桶末轮回复：刷新后必须仍在。",
      createdAt: 4100,
      status: "sent",
      lifecycle: "durable",
      persistence: "local",
      message_kind: "assistant"
    }
  ] as ChatMessage[];
}

const tailMarker = (messages: ChatMessage[]) =>
  messages.some((message) => message.content.indexOf("OBSLOCALTAIL") >= 0);

describe("FIX-BUCKET-SAVE-RACE-1：reload 桶 save/restore 管线", () => {
  it("图消息 initial 换流后，restore 先于 save 的合流保住仅存本地桶的末轮（修复前整桶覆写即丢）", () => {
    withStubbedStorage(() => {
      const graph = [
        graphMessage("n_1", "user", "对低音轨做一次混音改进实验", 1000),
        graphMessage("n_2", "assistant", "已建立基线观察。", 2000)
      ];
      const introClone: ChatMessage = {
        id: "intro_1790000000000_abcd",
        role: "assistant",
        content: "Ask Vit 就绪。",
        createdAt: 900,
        status: "sent",
        lifecycle: "durable",
        persistence: "local",
        message_kind: "assistant"
      } as ChatMessage;

      // 刷新前：图消息 + 驱动末轮都已落桶（末轮无图节点，桶是唯一载体）。
      saveStoredConversationMessages(conversationID, scope, [...graph, ...drivenTail()]);
      expect(tailMarker(loadStoredConversationMessages(conversationID, scope))).toBe(true);

      // 刷新后 scope 物化拍：initial 换流——现流被图消息替换，末轮不在流里。
      const reloadedFlow = resolveHistorySyncMessages({
        changeKind: "initial",
        current: [introClone],
        historyMessages: graph
      });
      expect(tailMarker(reloadedFlow)).toBe(false);

      // 修复前形态（竞态根因复现）：save 先于 restore 首跑，图消息整桶覆写，
      // 末轮结构性丢失。
      saveStoredConversationMessages(conversationID, scope, reloadedFlow);
      expect(tailMarker(loadStoredConversationMessages(conversationID, scope))).toBe(false);

      // 重刷前状态，走修复后管线：restore（先跑）读桶合流，再落盘——末轮
      // 回流且桶内容保全。
      saveStoredConversationMessages(conversationID, scope, [...graph, ...drivenTail()]);
      const restoredFlow = mergeRestoredChatMessages(reloadedFlow, loadStoredConversationMessages(conversationID, scope));
      expect(tailMarker(restoredFlow)).toBe(true);
      expect(restoredFlow.some((message) => message.id === "history_n_1")).toBe(true);
      saveStoredConversationMessages(conversationID, scope, restoredFlow);
      expect(tailMarker(loadStoredConversationMessages(conversationID, scope))).toBe(true);
    });
  });

  it("裸启动分支：现流只有问候克隆时，桶内容直接起流（mini_repro 无图消息反证形态）", () => {
    withStubbedStorage(() => {
      saveStoredConversationMessages(conversationID, scope, drivenTail());
      const introOnly: ChatMessage[] = [
        { id: "intro_1790000000001_bcde", role: "assistant", content: "Ask Vit 就绪。", createdAt: 1 } as ChatMessage
      ];
      const restored = mergeRestoredChatMessages(introOnly, loadStoredConversationMessages(conversationID, scope));
      expect(tailMarker(restored)).toBe(true);
      expect(restored.some((message) => message.id === "intro_1790000000001_bcde")).toBe(false);
    });
  });
});

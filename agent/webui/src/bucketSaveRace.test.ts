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

// WEBUI-MSG-ORDER-1 归因用例（2026-10-01，M8 手测倒挂取证）：mergeRestoredChatMessages
// 是**保序载体**——服务端把两轮用户行都盖上同一 run 域 turn_id（waiting_continue 续跑，
// ui-state.json 实证 user1/user2 同为 run_e5796736a4865570），本地桶里的乐观 user2 无
// turn_id，合流后它获得盖章 turn_id 且**数组时序不变**（user2 仍在第一轮输出之后）。
// 倒挂不产自合流层，而产自回合分组 groupMessagesByTurn 的同 id 回吸（本卡修复点）。
describe("WEBUI-MSG-ORDER-1：mergeRestoredChatMessages 归因排除", () => {
  const RUN = "run_e5796736a4865570";

  function m8HistoryRow(id: string, role: ChatMessage["role"], content: string, turnId: string, createdAt: number): ChatMessage {
    return {
      id: `history_${id}`,
      source_id: id,
      role,
      content,
      createdAt,
      status: "sent",
      lifecycle: "durable",
      persistence: "project_history",
      message_kind: role === "user" ? "user" : role === "assistant" ? "assistant" : "error",
      turn_id: turnId,
      logical_message_id: id
    } as ChatMessage;
  }

  it("合流输出保持时序（user2 在第一轮输出之后）；回合分组不再把 user2 吸回首轮组", async () => {
    const { groupMessagesByTurn } = await import("./trace/turnGroups");
    // 服务端历史 5 行（M8 ui-state.json 原样 turn_id 形态）
    const stored: ChatMessage[] = [
      m8HistoryRow("n_20260930T140459_f71485af", "user", "检查一下当前工程有什么问题", RUN, Date.parse("2026-09-30T14:04:59.359Z")),
      m8HistoryRow("n_20260930T140504_74bd2bac", "assistant", "我还在继续处理这个任务，完成后再向你汇报。", "turn_128b2cf41542f658", Date.parse("2026-09-30T14:05:04.418Z")),
      m8HistoryRow("n_20260930T140637_283508ea", "assistant", "这一步已经应用好了：Track 1017静态 EQ 频段增益 -0.5 dB（回读 -0.5 dB）。", "turn_977d99676e1c35ef", Date.parse("2026-09-30T14:06:37.729Z")),
      m8HistoryRow("n_20260930T140720_3933e760", "user", "你能再检查一下Bass轨道，看看它的低频有没有什么问题吗？", RUN, Date.parse("2026-09-30T14:07:20.947Z")),
      m8HistoryRow("n_20260930T140807_a0194727", "system", "任务在形成有效结算前失败；失败原因与已有证据已保留。", "turn_721c90fd784fe67b", Date.parse("2026-09-30T14:08:07.729Z"))
    ];
    // 现流（reload 前形态）：两轮乐观用户行都无 turn_id，assistant 行来自 HTTP（chat 域）
    const current: ChatMessage[] = [
      { id: "msg_round1", role: "user", content: "检查一下当前工程有什么问题", createdAt: Date.parse("2026-09-30T14:04:59.300Z"), status: "sent", lifecycle: "durable", persistence: "project_history", message_kind: "user" } as ChatMessage,
      { id: "msg_round1_reply", role: "assistant", content: "我还在继续处理这个任务，完成后再向你汇报。", turn_id: "turn_128b2cf41542f658", createdAt: Date.parse("2026-09-30T14:05:04.420Z"), status: "sent", lifecycle: "durable", persistence: "project_history", message_kind: "execution_receipt" } as ChatMessage,
      { id: "msg_round2", role: "user", content: "你能再检查一下Bass轨道，看看它的低频有没有什么问题吗？", createdAt: Date.parse("2026-09-30T14:07:20.900Z"), status: "sent", lifecycle: "durable", persistence: "project_history", message_kind: "user" } as ChatMessage
    ];

    const merged = mergeRestoredChatMessages(current, stored);
    // 归因排除①：合流层保序——user2 的行仍在两条第一轮输出之后（数组时序未被合流打乱）
    const orderIDs = merged.map((message) => message.content.slice(0, 6));
    expect(orderIDs).toEqual([
      "检查一下当前",
      "我还在继续处",
      "这一步已经应",
      "你能再检查一",
      "任务在形成有"
    ]);
    // 归因排除②：合流把盖章 turn_id 带给乐观 user2（同一逻辑消息合并，不产生双行）
    const mergedUser2 = merged.filter((message) => message.role === "user");
    expect(mergedUser2).toHaveLength(2);
    expect(mergedUser2.every((message) => message.turn_id === RUN)).toBe(true);

    // 缺陷面定位：倒挂只能来自回合分组——修复后 user2 不再被吸回首轮组
    const groups = groupMessagesByTurn(merged);
    const firstGroup = groups[0];
    expect(firstGroup.turnId).toBe(RUN);
    expect(firstGroup.messages.map((message) => message.content.slice(0, 6))).toEqual(["检查一下当前"]);
    const user2Group = groups.find((group) => group.messages.some((message) => message.content.startsWith("你能再检查一下")));
    expect(user2Group).toBeDefined();
    expect(groups.indexOf(user2Group!)).toBeGreaterThan(groups.findIndex((group) => group.messages.some((message) => message.content.startsWith("这一步已经应用好了"))));
  });
});

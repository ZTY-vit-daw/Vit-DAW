import { readFileSync } from "node:fs";
import { afterEach, describe, expect, it } from "vitest";
import { migrateStoredConversationScope } from "../App";
import type { AgentEvent, ChatMessage } from "../types";
import { emptyTrajectoryState } from "../trajectory";
import { buildMessageStreamRenderPlan } from "./renderPlan";
import { emptyRoundStepMap, reduceRoundSteps } from "./roundSteps";
import { reduceTurnEventMeta } from "./turnEventMeta";
import {
  TURN_RECEIPTS_LIMIT,
  collectTerminalTurnReceipts,
  loadTurnReceipts,
  mergeTurnReceipts,
  parseTurnReceiptLedger,
  persistTurnReceipts,
  receiptsForHydration,
  saveTurnReceipts,
  serializeTurnReceiptLedger,
  turnReceiptsStorageKey,
  type TurnReceipt
} from "./turnReceipts";

// TRAJ-IMPL-3（设计 docs/TRAJECTORY_PRESENTATION_REDESIGN_V1.md §2.3 分期一② + §7 裁定 C）：
// **终局回执行台账**的钉——写入（只在终态收口）/ bounded 滚动 / 不含正文 / 容错读 /
// 水合渲染（活态缺失时复用 UI-FOLLOW 两级锚定）/ 同键活态优先。
//
// 缺陷形态（F3+F5）：回合块消费 transient 事件派生态，刷新后重载即空——「块整块消失」
// 是拿活态当持久层的结构结果。修法：终局写一行摘要，水合路径为活态中不存在的回合渲染
// 静态收起回执行。

const RUN = "run_receipt_1";
const T0 = Date.parse("2026-09-14T12:00:00.000Z");
const at = (ms: number) => new Date(ms).toISOString();
const CONVERSATION = "webui_receipt_test";
const SCOPE = "history::D:/work/root";

function base(seq: number, type: string, createdMs: number, extra: Partial<AgentEvent> = {}): AgentEvent {
  return {
    seq,
    type,
    conversation_id: "c1",
    goal_id: "goal_1",
    run_id: RUN,
    turn_id: RUN,
    source_turn_id: RUN,
    created_at: at(createdMs),
    ...extra
  } as AgentEvent;
}

/** 一次工具调用的起止两条事件（真栈形态：同 run 多次调用复用 item_id，logical_message_id 区分调用） */
function toolCall(seq: number, callIndex: number, startMs: number, endMs: number): AgentEvent[] {
  const logical = `agent_item:${RUN}:tool_step_${callIndex}`;
  return [
    base(seq, "item.started", startMs, {
      item_id: "tool_step_1",
      logical_message_id: logical,
      item_type: "daw_action",
      status: "running",
      payload: { tool: "ccb.observation_catalog" }
    }),
    base(seq + 1, "item.completed", endMs, {
      item_id: "tool_step_1",
      logical_message_id: logical,
      item_type: "daw_action",
      status: "completed",
      payload: { command_name: "ccb_observation_catalog" }
    })
  ];
}

/** 活态（无终局事件）：回合开着，item 步在场 */
function liveTurnEvents(): AgentEvent[] {
  return [
    base(1, "turn.started", T0, { item_type: "turn", status: "running" }),
    ...toolCall(2, 1, T0 + 1_000, T0 + 3_000),
    ...toolCall(4, 2, T0 + 5_000, T0 + 9_000)
  ];
}

/** 终态收口：工作片 30s 后 turn.completed（无驻留段） */
function terminalTurnEvents(): AgentEvent[] {
  return [...liveTurnEvents(), base(6, "turn.completed", T0 + 30_000, { item_type: "turn", status: "completed" })];
}

function receiptsFromEvents(events: AgentEvent[]): TurnReceipt[] {
  return collectTerminalTurnReceipts({
    trajectory: emptyTrajectoryState(),
    turnEventMeta: reduceTurnEventMeta({}, events),
    roundSteps: reduceRoundSteps(emptyRoundStepMap(), events)
  });
}

function receipt(partial: Partial<TurnReceipt> & { turnId: string }): TurnReceipt {
  return { status: "completed", stepCount: 2, activityCount: 2, workMs: 30_000, parkMs: null, startedAt: T0, ...partial };
}

function chat(partial: Partial<ChatMessage> & Pick<ChatMessage, "id" | "role" | "content">): ChatMessage {
  return { createdAt: 0, ...partial } as ChatMessage;
}

const shapeOf = (plan: ReturnType<typeof buildMessageStreamRenderPlan>): string[] =>
  plan.entries.map((entry) =>
    entry.kind === "messages"
      ? `messages:${entry.messages.map((message) => message.id).join(",")}`
      : `${entry.kind}:${entry.turnId}`
  );

/** localStorage 桩（与 historyScope.test.ts 同形：node 环境下的 window.localStorage） */
function withStorage(run: (store: Map<string, string>) => void): void {
  const backup = (globalThis as { window?: unknown }).window;
  const store = new Map<string, string>();
  (globalThis as { window?: unknown }).window = {
    localStorage: {
      getItem: (key: string) => store.get(key) ?? null,
      setItem: (key: string, value: string) => void store.set(key, value),
      removeItem: (key: string) => void store.delete(key)
    }
  };
  try {
    run(store);
  } finally {
    (globalThis as { window?: unknown }).window = backup;
  }
}

afterEach(() => {
  (globalThis as { window?: unknown }).window = undefined;
});

describe("钉1 写入只在终态收口路径（设计 §2.3 + §4 双轨收口规则）", () => {
  it("终态收口 → 一行，字段齐：回合键/状态/步数/活动数/执行时长/等待时长/started_at", () => {
    expect(receiptsFromEvents(terminalTurnEvents())).toEqual([
      {
        turnId: RUN,
        status: "completed",
        stepCount: 2,
        activityCount: 2,
        workMs: 30_000,
        parkMs: null,
        startedAt: T0
      }
    ]);
  });

  it("live 回合一行都不写（台账不是活态镜像）", () => {
    expect(receiptsFromEvents(liveTurnEvents())).toEqual([]);
  });

  it("切片边界（turn.completed/waiting_continue）不算终局——续跑未定，不写", () => {
    const events = [
      ...liveTurnEvents(),
      base(6, "turn.completed", T0 + 30_000, { item_type: "turn", status: "waiting_continue", payload: { turn_kind: "slice_boundary" } })
    ];
    expect(receiptsFromEvents(events)).toEqual([]);
  });

  it("失败 / 停止各写对应状态；驻留段单列（执行时长不得把等待算进去）", () => {
    // 失败回合的步数比工具步多一步：turn.failed 自身也是一步（TRAJ-IMPL-2 的
    // STEP_EVENT_TYPES 逐字保留），回执行不得把它抹掉。
    const failed = [...liveTurnEvents(), base(6, "turn.failed", T0 + 30_000, { item_type: "turn", status: "failed" })];
    expect(receiptsFromEvents(failed)).toEqual([
      { turnId: RUN, status: "failed", stepCount: 3, activityCount: 2, workMs: 30_000, parkMs: null, startedAt: T0 }
    ]);

    // 工作片 30s 后驻留 5s 才被停止（CONT-STALL-1 形态：等待不得计成工作）
    const stopped = [
      ...liveTurnEvents(),
      base(6, "turn.completed", T0 + 30_000, { item_type: "turn", status: "completed" }),
      base(7, "turn.stopped", T0 + 35_000, { item_type: "turn", status: "stopped" })
    ];
    expect(receiptsFromEvents(stopped)).toEqual([
      { turnId: RUN, status: "stopped", stepCount: 2, activityCount: 2, workMs: 30_000, parkMs: 5_000, startedAt: T0 }
    ]);
  });

  it("M12 隐藏的结算切片不得经台账在刷新后复活（无足迹 + settle_slice 标记 → 不写行）", () => {
    const slice = [
      base(1, "turn.started", T0, { item_type: "turn", status: "running" }),
      base(2, "turn.completed", T0 + 128, { item_type: "turn", status: "completed", payload: { turn_kind: "settle_slice" } })
    ];
    expect(receiptsFromEvents(slice)).toEqual([]);
  });
});

describe("钉2 bounded 滚动 + 同键去重（设计 §2.3「bounded ~200 条滚动」）", () => {
  it("同一回合重复收口只留一行（同键幂等），刷新该行字段", () => {
    const once = mergeTurnReceipts([], [receipt({ turnId: "run_a", stepCount: 2 })]);
    const twice = mergeTurnReceipts(once, [receipt({ turnId: "run_a", stepCount: 2 })]);
    expect(twice).toHaveLength(1);
    const refreshed = mergeTurnReceipts(twice, [receipt({ turnId: "run_a", stepCount: 9 })]);
    expect(refreshed).toHaveLength(1);
    expect(refreshed[0].stepCount).toBe(9);
  });

  it("超出上限丢最旧（行序 = 收口序，新行留尾）", () => {
    const many = Array.from({ length: TURN_RECEIPTS_LIMIT + 5 }, (_, index) => receipt({ turnId: `run_${index}` }));
    const merged = mergeTurnReceipts([], many);
    expect(merged).toHaveLength(TURN_RECEIPTS_LIMIT);
    expect(merged[0].turnId).toBe("run_5");
    expect(merged[merged.length - 1].turnId).toBe(`run_${TURN_RECEIPTS_LIMIT + 4}`);
  });

  it("落盘同样截断（台账不得无界增长）", () => {
    const raw = serializeTurnReceiptLedger({
      schemaVersion: "vit.turn_receipts.v1",
      conversationId: CONVERSATION,
      scope: SCOPE,
      savedAt: "2026-09-14T12:00:00.000Z",
      receipts: Array.from({ length: TURN_RECEIPTS_LIMIT + 3 }, (_, index) => receipt({ turnId: `run_${index}` }))
    });
    const parsed = JSON.parse(raw) as { receipts: Array<{ turn_id: string }> };
    expect(parsed.receipts).toHaveLength(TURN_RECEIPTS_LIMIT);
    expect(parsed.receipts[0].turn_id).toBe("run_3");
  });
});

describe("钉3 只承载状态/计数/时长——不复述正文（正文归气泡）", () => {
  it("序列化行是白名单键：正文/标题/摘要字段即使被调用方塞进来也不落盘", () => {
    const dirty = { ...receipt({ turnId: RUN }), content: "我把低音轨的 EQ 调好了", title: "已完成 频率关系观察" } as TurnReceipt;
    const raw = serializeTurnReceiptLedger({
      schemaVersion: "vit.turn_receipts.v1",
      conversationId: CONVERSATION,
      scope: SCOPE,
      savedAt: "2026-09-14T12:00:00.000Z",
      receipts: [dirty]
    });
    const parsed = JSON.parse(raw) as Record<string, unknown>;
    expect(Object.keys(parsed).sort()).toEqual(["conversation_id", "receipts", "saved_at", "schema_version", "scope"]);
    expect(Object.keys((parsed.receipts as Array<Record<string, unknown>>)[0]).sort()).toEqual([
      "activity_count",
      "park_ms",
      "started_at",
      "status",
      "step_count",
      "turn_id",
      "work_ms"
    ]);
    expect(raw).not.toContain("我把低音轨的 EQ 调好了");
    expect(raw).not.toContain("已完成 频率关系观察");
  });

  it("往返：序列化 → 反序列化逐字段相等（含 null 时长）", () => {
    const rows = [receipt({ turnId: RUN }), receipt({ turnId: "run_b", status: "stopped", parkMs: 5_000, workMs: null })];
    const raw = serializeTurnReceiptLedger({
      schemaVersion: "vit.turn_receipts.v1",
      conversationId: CONVERSATION,
      scope: SCOPE,
      savedAt: "2026-09-14T12:00:00.000Z",
      receipts: rows
    });
    expect(parseTurnReceiptLedger(raw)?.receipts).toEqual(rows);
  });
});

describe("钉4 schema 版本 + 未知字段容错（AGENTS.md §11：新版字段不得误读、未知枚举 fail-closed）", () => {
  it("未知版本 fail-closed：不是本代的 schema_version 一律不读", () => {
    const raw = JSON.stringify({ schema_version: "vit.turn_receipts.v2", conversation_id: CONVERSATION, scope: SCOPE, receipts: [receipt({ turnId: RUN })] });
    expect(parseTurnReceiptLedger(raw)).toBeNull();
    expect(parseTurnReceiptLedger(JSON.stringify({ conversation_id: CONVERSATION, receipts: [] }))).toBeNull();
    expect(parseTurnReceiptLedger("not json")).toBeNull();
    expect(parseTurnReceiptLedger(null)).toBeNull();
  });

  it("未知字段容错读、缺字段取安全缺省、未知状态丢行（不把状态改写成别的）", () => {
    const raw = JSON.stringify({
      schema_version: "vit.turn_receipts.v1",
      conversation_id: CONVERSATION,
      scope: SCOPE,
      saved_at: "2026-09-14T12:00:00.000Z",
      future_top_level: { anything: true },
      receipts: [
        { turn_id: "run_a", status: "completed", step_count: 2, activity_count: 1, work_ms: 30_000, park_ms: null, started_at: T0, future_field: "不该进内存", title: "不该进来" },
        { turn_id: "run_b", status: "completed" },
        { turn_id: "run_c", status: "archived" },
        { status: "completed" }
      ]
    });
    const ledger = parseTurnReceiptLedger(raw);
    expect(ledger).not.toBeNull();
    expect(ledger?.receipts.map((row) => row.turnId)).toEqual(["run_a", "run_b"]);
    expect(ledger?.receipts[1]).toEqual({
      turnId: "run_b",
      status: "completed",
      stepCount: 0,
      activityCount: 0,
      workMs: null,
      parkMs: null,
      startedAt: 0
    });
    expect(JSON.stringify(ledger)).not.toContain("不该进内存");
    expect(JSON.stringify(ledger)).not.toContain("不该进来");
  });
});

describe("钉5 localStorage 台账（per conversation+scope 分桶 + 丢失边界如实）", () => {
  it("写入 → 读出：分桶键含 conversation+scope，同键只留一行", () => {
    withStorage(() => {
      const merged = mergeTurnReceipts([], [receipt({ turnId: RUN }), receipt({ turnId: RUN, stepCount: 3 })]);
      persistTurnReceipts(CONVERSATION, SCOPE, merged);
      const loaded = loadTurnReceipts(CONVERSATION, SCOPE);
      expect(loaded).toHaveLength(1);
      expect(loaded[0].stepCount).toBe(3);
      expect(turnReceiptsStorageKey(CONVERSATION, SCOPE)).toContain("vit.turn_receipts.v1");
      // 另一个 scope = 另一个桶（回执行按「会话×工作区」分账）
      expect(loadTurnReceipts(CONVERSATION, "history::D:/work/other")).toEqual([]);
    });
  });

  it("刷新后的第一笔收口不得冲掉上一页写下的回执行（写侧以落盘台账为权威副本）", () => {
    withStorage(() => {
      // 上一页：run_prev 已收口并落盘；本页刷新后活态里没有它（活态缺失）
      persistTurnReceipts(CONVERSATION, SCOPE, [receipt({ turnId: "run_prev" })]);
      // 本页这一轮只有另一个回合收口；saveTurnReceipts 必须读—并—写
      const merged = saveTurnReceipts(CONVERSATION, SCOPE, [receipt({ turnId: "run_now" })]);
      expect(merged.map((row) => row.turnId)).toEqual(["run_prev", "run_now"]);
      expect(loadTurnReceipts(CONVERSATION, SCOPE).map((row) => row.turnId)).toEqual(["run_prev", "run_now"]);
      // 无变化时不写盘（写侧幂等，不因轮询反复改字节）
      const again = saveTurnReceipts(CONVERSATION, SCOPE, [receipt({ turnId: "run_now" })]);
      expect(again).toEqual(merged);
    });
  });

  it("scope 演进：台账随消息存档一起迁移到新桶（旧桶保留作回退）", () => {
    withStorage(() => {
      persistTurnReceipts(CONVERSATION, "unsaved::root", [receipt({ turnId: RUN })]);
      migrateStoredConversationScope("unsaved::root", "D:/work/draft.vit::root", CONVERSATION);
      expect(loadTurnReceipts(CONVERSATION, "D:/work/draft.vit::root").map((row) => row.turnId)).toEqual([RUN]);
      expect(loadTurnReceipts(CONVERSATION, "unsaved::root").map((row) => row.turnId)).toEqual([RUN]);
    });
  });

  it("丢失边界（换浏览器/清缓存）：读出空表，不编造回执行", () => {
    withStorage((store) => {
      persistTurnReceipts(CONVERSATION, SCOPE, [receipt({ turnId: RUN })]);
      expect(loadTurnReceipts(CONVERSATION, SCOPE)).toHaveLength(1);
      store.clear();
      expect(loadTurnReceipts(CONVERSATION, SCOPE)).toEqual([]);
    });
    // 无 window（SSR/测试环境）同样空表，不抛
    expect(loadTurnReceipts(CONVERSATION, SCOPE)).toEqual([]);
  });
});

describe("钉6 水合渲染：活态缺失的回合落成静态收起收据行（槽位锚定复用两级锚定）", () => {
  it("身份锚定优先：消息组携带该回合 id → 收据行落在该用户消息之后", () => {
    const plan = buildMessageStreamRenderPlan({
      messages: [
        chat({ id: "u1", role: "user", content: "问", turn_id: RUN, createdAt: T0 + 2 }),
        chat({ id: "a1", role: "assistant", content: "答", createdAt: T0 + 30_000 })
      ],
      trajectory: emptyTrajectoryState(),
      receipts: [receipt({ turnId: RUN })]
    });
    expect(shapeOf(plan)).toEqual(["messages:u1", `receipt:${RUN}`, "messages:a1"]);
    expect(plan.orphanTurnIds).toEqual([]);
    expect(plan.orphanReceiptTurnIds).toEqual([]);
    const entry = plan.entries.find((item) => item.kind === "receipt");
    expect(entry?.kind === "receipt" ? entry.receipt.stepCount : null).toBe(2);
  });

  it("槽位锚定：身份不同源时按 started_at + 50ms 容差落到开启该回合的那条用户消息之后", () => {
    // 两个已水合的回合（各自成组，回合键都不是收据的键）；收据的时刻落在第二轮之后
    // ——它必须挂第二轮那条用户消息，而不是第一轮，也不坠到流尾。
    const plan = buildMessageStreamRenderPlan({
      messages: [
        chat({ id: "u1", role: "user", content: "第一轮", turn_id: "turn_a", createdAt: T0 - 60_000 }),
        chat({ id: "a1", role: "assistant", content: "第一轮答", turn_id: "turn_a", createdAt: T0 - 30_000 }),
        chat({ id: "u2", role: "user", content: "第二轮（水合盖章晚 2ms）", turn_id: "turn_b", createdAt: T0 + 2 }),
        chat({ id: "a2", role: "assistant", content: "第二轮答", turn_id: "turn_b", createdAt: T0 + 40_000 })
      ],
      trajectory: emptyTrajectoryState(),
      receipts: [receipt({ turnId: RUN, startedAt: T0 })]
    });
    expect(shapeOf(plan)).toEqual(["messages:u1", "messages:a1", "messages:u2", `receipt:${RUN}`, "messages:a2"]);
  });

  it("无起始时刻证据不猜归属：收据行退流尾（orphanReceiptTurnIds），不挂到别的回合下", () => {
    const plan = buildMessageStreamRenderPlan({
      messages: [chat({ id: "u1", role: "user", content: "问", createdAt: T0 })],
      trajectory: emptyTrajectoryState(),
      receipts: [receipt({ turnId: RUN, startedAt: 0 })]
    });
    expect(shapeOf(plan)).toEqual(["messages:u1", `receipt:${RUN}`]);
    expect(plan.orphanReceiptTurnIds).toEqual([RUN]);
  });

  it("同键活态优先：回合键在活态（item 步账）里 → 计划里没有收据条目，只有活态块", () => {
    const events = liveTurnEvents();
    const plan = buildMessageStreamRenderPlan({
      messages: [chat({ id: "u1", role: "user", content: "问", turn_id: RUN, createdAt: T0 + 2 })],
      trajectory: emptyTrajectoryState(),
      turnEventMeta: reduceTurnEventMeta({}, events),
      roundSteps: reduceRoundSteps(emptyRoundStepMap(), events),
      receipts: [receipt({ turnId: RUN })]
    });
    expect(shapeOf(plan)).toEqual(["messages:u1", `trace:${RUN}`]);
    expect(receiptsForHydration({ receipts: [receipt({ turnId: RUN })], liveTurnIds: new Set([RUN]) })).toEqual([]);
  });

  it("一个渲染组只承载一个条目（B9）：活态块占住槽位时收据退流尾，不挤掉活态块", () => {
    const events = liveTurnEvents();
    const other = receipt({ turnId: "run_other", startedAt: T0 + 1 });
    const plan = buildMessageStreamRenderPlan({
      messages: [
        chat({ id: "u1", role: "user", content: "问", turn_id: RUN, createdAt: T0 + 2 }),
        chat({ id: "a1", role: "assistant", content: "答", createdAt: T0 + 30_000 })
      ],
      trajectory: emptyTrajectoryState(),
      turnEventMeta: reduceTurnEventMeta({}, events),
      roundSteps: reduceRoundSteps(emptyRoundStepMap(), events),
      receipts: [other]
    });
    expect(shapeOf(plan)).toEqual(["messages:u1", `trace:${RUN}`, "messages:a1", "receipt:run_other"]);
    expect(plan.orphanReceiptTurnIds).toEqual(["run_other"]);
  });

  it("缺省 receipts（既有调用面）行为逐字不变", () => {
    const baseline = buildMessageStreamRenderPlan({
      messages: [chat({ id: "u1", role: "user", content: "问", turn_id: RUN, createdAt: T0 + 2 })],
      trajectory: emptyTrajectoryState()
    });
    const empty = buildMessageStreamRenderPlan({
      messages: [chat({ id: "u1", role: "user", content: "问", turn_id: RUN, createdAt: T0 + 2 })],
      trajectory: emptyTrajectoryState(),
      receipts: []
    });
    expect(shapeOf(empty)).toEqual(shapeOf(baseline));
    expect(empty.orphanReceiptTurnIds).toEqual([]);
  });
});

describe("钉7 App 接线：终态写入 / 水合读取 / 渲染面三处都真的接上了", () => {
  it("写入只在终态收口 effect 里、水合读取走台账、计划传 receipts、渲染用 TurnReceiptRow", () => {
    const appSource = readFileSync(new URL("../App.tsx", import.meta.url), "utf-8");
    expect(appSource).toMatch(/collectTerminalTurnReceipts\(\{ trajectory: trajectoryState, turnEventMeta, roundSteps \}\)/);
    expect(appSource).toMatch(/const merged = saveTurnReceipts\(conversationID, scope, closed\)/);
    expect(appSource).toMatch(/applyTurnReceipts\(loadTurnReceipts\(conversationID, scope\)\)/);
    expect(appSource).toMatch(/buildMessageStreamRenderPlan\(\{ messages: visibleMessages, trajectory, turnEventMeta, roundSteps, receipts \}\)/);
    expect(appSource).toMatch(/<TurnReceiptRow receipt=\{entry\.receipt\} \/>/);
  });
});

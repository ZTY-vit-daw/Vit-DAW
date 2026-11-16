import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  loadSessionFlowCollapsed,
  loadSessionFlowMainID,
  loadSessionFlowRegistry,
  mainConversationDefaultTitle,
  noteSessionServerHints,
  renameSessionFlowEntry,
  saveSessionFlowCollapsed,
  saveSessionFlowMainID,
  saveSessionFlowRegistry,
  sessionFlowRows,
  sessionFlowServerHints,
  setSessionFlowArchived,
  upsertSessionFlowEntry
} from "./sessionFlow";
import { noteSessionSeedMessages } from "./App";

// WEBUI-IA-REDESIGN-1：会话流注册表单元钉。
// 持久化兼容（AGENTS §11）是验收面之一：损坏 JSON/异形行/未知字段 fail-open，
// 旧数据（本模块为新增存储，模拟“别的版本写入的形态”）加载零破坏。

function stubLocalStorage() {
  const store = new Map<string, string>();
  vi.stubGlobal("window", {
    localStorage: {
      getItem: (key: string) => store.get(key) ?? null,
      setItem: (key: string, value: string) => void store.set(key, value),
      removeItem: (key: string) => void store.delete(key)
    }
  });
  return store;
}

beforeEach(() => {
  stubLocalStorage();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("sessionFlowRegistry 持久化（fail-open）", () => {
  it("空 scope / 无 window 不读写", () => {
    vi.unstubAllGlobals();
    expect(loadSessionFlowRegistry("")).toEqual([]);
    expect(() => saveSessionFlowRegistry("", [])).not.toThrow();
  });

  it("往返：保存后可加载，schema_version 随行", () => {
    const entries = [
      { conversationID: "webui_a", title: "低频治理", createdAt: 100, updatedAt: 200, archived: false }
    ];
    saveSessionFlowRegistry("scope::root", entries);
    expect(loadSessionFlowRegistry("scope::root")).toEqual(entries);
  });

  it("损坏 JSON → 空表（不抛）", () => {
    window.localStorage.setItem("ask_vit_session_flow.v1:scope--root", "{not json");
    expect(loadSessionFlowRegistry("scope::root")).toEqual([]);
  });

  it("异形行被丢弃、未知字段忽略、重复 id 去重（首见优先）", () => {
    const payload = {
      schema_version: "ask_vit_session_flow.v1",
      sessions: [
        { conversationID: "webui_ok", title: "a", createdAt: 1, updatedAt: 2, archived: false, extra: "unknown" },
        null,
        { conversationID: "", title: "bad" },
        { conversationID: 42 },
        "junk",
        { conversationID: "webui_dup", title: "first", createdAt: 1, updatedAt: 2, archived: false },
        { conversationID: "webui_dup", title: "second", createdAt: 9, updatedAt: 9, archived: true }
      ]
    };
    window.localStorage.setItem("ask_vit_session_flow.v1:scope--root", JSON.stringify(payload));
    const loaded = loadSessionFlowRegistry("scope::root");
    expect(loaded.map((entry) => entry.conversationID)).toEqual(["webui_ok", "webui_dup"]);
    expect(loaded[0].title).toBe("a");
    expect(loaded[1].title).toBe("first");
  });

  it("数组裸形态（无 schema 包裹）也能读", () => {
    window.localStorage.setItem(
      "ask_vit_session_flow.v1:scope--root",
      JSON.stringify([{ conversationID: "webui_b", createdAt: 1, updatedAt: 1 }])
    );
    const loaded = loadSessionFlowRegistry("scope::root");
    expect(loaded).toHaveLength(1);
    expect(loaded[0].archived).toBe(false);
    expect(Number.isFinite(loaded[0].createdAt)).toBe(true);
  });
});

describe("会话操作（不建工作树：仅注册表行）", () => {
  const base = { conversationID: "webui_a", title: "", createdAt: 100, updatedAt: 100, archived: false };

  it("upsert 新建行 + 幂等刷新（保留重命名与归档态）", () => {
    let entries = upsertSessionFlowEntry([], { conversationID: "webui_a", updatedAt: 300 });
    expect(entries).toEqual([{ ...base, createdAt: 300, updatedAt: 300 }]);
    entries = renameSessionFlowEntry(entries, "webui_a", "我的会话");
    entries = setSessionFlowArchived(entries, "webui_a", true);
    entries = upsertSessionFlowEntry(entries, { conversationID: "webui_a", updatedAt: 500 });
    expect(entries).toEqual([{ ...base, createdAt: 300, title: "我的会话", updatedAt: 500, archived: true }]);
  });

  it("upsert 不覆写已命名标题；空 id 不建行", () => {
    let entries = [{ ...base, title: "已命名" }];
    entries = upsertSessionFlowEntry(entries, { conversationID: "webui_a", updatedAt: 200, title: "derived" });
    expect(entries[0].title).toBe("已命名");
    expect(upsertSessionFlowEntry(entries, { conversationID: "  " })).toBe(entries);
  });

  it("rename 修剪空白并截断 120 字", () => {
    const entries = renameSessionFlowEntry([base], "webui_a", `  ${"长".repeat(200)}  `);
    expect(entries[0].title).toHaveLength(120);
  });

  it("archive/restore 只改目标行", () => {
    let entries = [base, { ...base, conversationID: "webui_b" }];
    entries = setSessionFlowArchived(entries, "webui_b", true);
    expect(entries[0].archived).toBe(false);
    expect(entries[1].archived).toBe(true);
  });
});

describe("continuations 服务端提示合并", () => {
  const parts = { projectPath: "D:/proj/a.vit", rootProjectPath: "", projectUUID: "uuid-1" };

  it("按 project_path/uuid 匹配 scope，同会话多切片取最新", () => {
    const hints = sessionFlowServerHints(
      [
        { conversation_id: "webui_x", project_path: "D:\\proj\\a.vit", updated_at: "2026-10-02T10:00:00Z", original_intent: "intent-1" },
        { conversation_id: "webui_x", project_path: "D:/proj/a.vit", updated_at: "2026-10-02T12:00:00Z", original_intent: "intent-2" },
        { conversation_id: "webui_other", project_path: "D:/proj/b.vit", updated_at: "2026-10-02T13:00:00Z" },
        { conversation_id: "webui_u", project_uuid: "UUID-1", updated_at: "2026-10-02T09:00:00Z" }
      ],
      parts
    );
    expect(Array.from(hints.keys()).sort()).toEqual(["webui_u", "webui_x"]);
    expect(hints.get("webui_x")?.updatedAt).toBe(Date.parse("2026-10-02T12:00:00Z"));
  });

  it("非数组/无 scope 匹配面 → 空提示", () => {
    expect(sessionFlowServerHints(null, parts).size).toBe(0);
    expect(sessionFlowServerHints([{}], { projectPath: "", rootProjectPath: "", projectUUID: "" }).size).toBe(0);
  });

  it("合并投影：注册表权威+服务端补行，updatedAt 降序，线性编号 1..n", () => {
    const registry = [
      { conversationID: "webui_local", title: "本地", createdAt: 1, updatedAt: 100, archived: false },
      { conversationID: "webui_old", title: "旧", createdAt: 1, updatedAt: 50, archived: true }
    ];
    const hints = new Map([
      ["webui_server", { conversationID: "webui_server", updatedAt: 200, intent: "服务端任务" }],
      ["webui_local", { conversationID: "webui_local", updatedAt: 300, intent: "" }]
    ]);
    const { visible, archived } = sessionFlowRows(registry, hints);
    // webui_local 服务端刷新到 300 → 排最前；webui_server 补行 200 次之
    expect(visible.map((row) => row.conversationID)).toEqual(["webui_local", "webui_server"]);
    expect(visible.map((row) => row.displayNumber)).toEqual([1, 2]);
    expect(visible[1].serverKnown).toBe(true);
    expect(visible[0].title).toBe("本地");
    expect(archived.map((row) => row.conversationID)).toEqual(["webui_old"]);
    expect(archived[0].displayNumber).toBe(3);
  });

  it("同刻并列按 conversation id 决胜（选择只由数据决定）", () => {
    const registry = [
      { conversationID: "webui_b", title: "", createdAt: 1, updatedAt: 100, archived: false },
      { conversationID: "webui_a", title: "", createdAt: 1, updatedAt: 100, archived: false }
    ];
    const { visible } = sessionFlowRows(registry, new Map());
    expect(visible.map((row) => row.conversationID)).toEqual(["webui_a", "webui_b"]);
  });
});

describe("折叠态持久化", () => {
  it("默认未折叠；往返保存", () => {
    expect(loadSessionFlowCollapsed()).toBe(false);
    saveSessionFlowCollapsed(true);
    expect(loadSessionFlowCollapsed()).toBe(true);
    saveSessionFlowCollapsed(false);
    expect(loadSessionFlowCollapsed()).toBe(false);
  });
});

// WEBUI-SESSION-SEMANTICS-1（2026-10-04 命名裁定 1）：主对话流=webui 流是主——
// 启动即建并命名「主对话流 · <工程名>」，同工程重开沿旧名，用户改名最高且持久。
describe("主对话流身份与默认命名（SESSION-SEMANTICS-1）", () => {
  it("默认名模板：工程名拼「主对话流 · <工程名>」；空工程名退化为裸形态", () => {
    expect(mainConversationDefaultTitle("draft_20261003")).toBe("主对话流 · draft_20261003");
    expect(mainConversationDefaultTitle("")).toBe("主对话流");
    expect(mainConversationDefaultTitle("   ")).toBe("主对话流");
  });

  it("主会话身份键往返；trim 读取", () => {
    expect(loadSessionFlowMainID("scope::root")).toBe("");
    saveSessionFlowMainID("scope::root", "webui_main_1");
    expect(loadSessionFlowMainID("scope::root")).toBe("webui_main_1");
    // 同 scope 重写=换主（首见即钉由调用侧保证，本层只存取）
    saveSessionFlowMainID("scope::root", "webui_main_2");
    expect(loadSessionFlowMainID("scope::root")).toBe("webui_main_2");
  });

  it("fail-open：无 window 时读空串、写不抛（不依赖 stub）", () => {
    vi.unstubAllGlobals();
    expect(loadSessionFlowMainID("scope::root")).toBe("");
    expect(() => saveSessionFlowMainID("scope::root", "webui_a")).not.toThrow();
    expect(() => saveSessionFlowMainID("", "webui_a")).not.toThrow();
  });

  it("空 scope 不落键；空 id 不吞旧值", () => {
    saveSessionFlowMainID("", "webui_a");
    saveSessionFlowMainID("scope::root", "webui_keep");
    saveSessionFlowMainID("scope::root", "  ");
    expect(loadSessionFlowMainID("scope::root")).toBe("webui_keep");
    expect(loadSessionFlowMainID("")).toBe("");
  });

  it("默认名经 upsert 落行：只填未命名行——用户改名后不被覆写（改名最高且持久）", () => {
    let entries = upsertSessionFlowEntry([], { conversationID: "webui_main_1" });
    entries = upsertSessionFlowEntry(entries, { conversationID: "webui_main_1", title: mainConversationDefaultTitle("draft_x") });
    expect(entries[0].title).toBe("主对话流 · draft_x");
    // 用户改名
    entries = renameSessionFlowEntry(entries, "webui_main_1", "我的主流");
    // 后续幂等登记（重启/切换回来）不得覆写
    entries = upsertSessionFlowEntry(entries, { conversationID: "webui_main_1", title: mainConversationDefaultTitle("draft_x") });
    expect(entries[0].title).toBe("我的主流");
    // 归档态同样零覆写
    entries = setSessionFlowArchived(entries, "webui_main_1", true);
    entries = upsertSessionFlowEntry(entries, { conversationID: "webui_main_1", title: mainConversationDefaultTitle("draft_x") });
    expect(entries[0].archived).toBe(true);
  });

  it("主对话流行与用户新建会话平级合并（线性编号连续）", () => {
    const main = upsertSessionFlowEntry([], { conversationID: "webui_main_1", title: mainConversationDefaultTitle("draft_x"), updatedAt: 100 });
    const withUserFlow = upsertSessionFlowEntry(main, { conversationID: "webui_user_1", updatedAt: 200 });
    const { visible } = sessionFlowRows(withUserFlow, new Map());
    // updatedAt 降序：用户新流(200)在前，主对话流(100)在后；编号纯线性 1/2
    expect(visible.map((row) => row.conversationID)).toEqual(["webui_user_1", "webui_main_1"]);
    expect(visible[1].title).toBe("主对话流 · draft_x");
    expect(visible.map((row) => row.displayNumber)).toEqual([1, 2]);
  });
});

// VITNOTE-NOTESTREAM-2（目标 6）：note 会话提示——默认命名落侧边栏，不显示"未命名会话"。
describe("note 会话提示与合并（NOTESTREAM-2）", () => {
  const parts = { projectPath: "D:/proj/a.vit", rootProjectPath: "", projectUUID: "uuid-1" };
  const noteRows = [
    {
      conversation_id: "note_r-abc-3",
      note_id: "note_3",
      title: "便签 N3 · 轨道时间线 72%",
      archived: false,
      updated_at: "2026-10-03T12:00:00Z",
      project_path: "D:\\proj\\a.vit",
      project_uuid: "uuid-1",
      messages: [
        { role: "user", content: "这个范围是什么内容？", created_at: "2026-10-03T11:59:00Z" },
        { role: "assistant", content: "辖区包含 Bass 轨的 4 个 clip。", created_at: "2026-10-03T12:00:00Z" }
      ]
    },
    {
      conversation_id: "note_r-def-5",
      title: "便签 N5 · 空辖区",
      archived: true,
      updated_at: "2026-10-03T13:00:00Z",
      project_uuid: "UUID-1"
    },
    { conversation_id: "note_r-other", title: "他工程", updated_at: "2026-10-03T14:00:00Z", project_path: "D:/proj/b.vit" }
  ];

  it("noteSessionServerHints 按 scope 匹配并携带默认名+归档态", () => {
    const hints = noteSessionServerHints(noteRows, parts);
    expect(Array.from(hints.keys()).sort()).toEqual(["note_r-abc-3", "note_r-def-5"]);
    expect(hints.get("note_r-abc-3")?.title).toBe("便签 N3 · 轨道时间线 72%");
    expect(hints.get("note_r-abc-3")?.archived).toBe(false);
    expect(hints.get("note_r-def-5")?.archived).toBe(true);
  });

  it("非数组/空 scope → 空提示（fail-open）", () => {
    expect(noteSessionServerHints(null, parts).size).toBe(0);
    expect(noteSessionServerHints(noteRows, { projectPath: "", rootProjectPath: "", projectUUID: "" }).size).toBe(0);
  });

  it("合并投影：note 行带默认名（不显未命名）+归档行入归档组；本地命名/归档仍权威", () => {
    const hints = noteSessionServerHints(noteRows, parts);
    const { visible, archived } = sessionFlowRows([], hints);
    // 归档提示行（note 删除）进归档组，可见组只含未归档 note 行。
    expect(visible.map((row) => row.conversationID)).toEqual(["note_r-abc-3"]);
    expect(visible[0].title).toBe("便签 N3 · 轨道时间线 72%");
    expect(visible[0].serverKnown).toBe(true);
    expect(archived.map((row) => row.conversationID)).toEqual(["note_r-def-5"]);
    expect(archived[0].title).toBe("便签 N5 · 空辖区");
    // 本地已命名行不被服务端默认名覆写。
    const local = [{ conversationID: "note_r-abc-3", title: "我的便签", createdAt: 1, updatedAt: 50, archived: false }];
    const merged = sessionFlowRows(local, hints);
    expect(merged.visible.find((row) => row.conversationID === "note_r-abc-3")?.title).toBe("我的便签");
  });

  it("rename 缺行补建：serverKnown note 行改名可落（注册表权威）", () => {
    let entries: ReturnType<typeof renameSessionFlowEntry> = [];
    entries = renameSessionFlowEntry(entries, "note_r-abc-3", "改名后的便签");
    expect(entries).toHaveLength(1);
    expect(entries[0].title).toBe("改名后的便签");
    // 提示行 title 不覆写本地改名。
    const hints = noteSessionServerHints(noteRows, parts);
    const { visible } = sessionFlowRows(entries, hints);
    const row = visible.find((candidate) => candidate.conversationID === "note_r-abc-3");
    expect(row?.title).toBe("改名后的便签");
  });

  it("noteSessionSeedMessages 从投影行回放问答对；非 note 目标回落问候", () => {
    const seeded = noteSessionSeedMessages("note_r-abc-3", noteRows as unknown as import("./types").JsonRecord[]);
    expect(seeded).toHaveLength(2);
    expect(seeded[0].role).toBe("user");
    expect(seeded[0].content).toBe("这个范围是什么内容？");
    expect(seeded[1].role).toBe("assistant");
    // fail-open：无匹配行/异形消息回落 intro。
    expect(noteSessionSeedMessages("note_unknown", noteRows as unknown as import("./types").JsonRecord[])).toHaveLength(1);
    expect(noteSessionSeedMessages("note_r-def-5", noteRows as unknown as import("./types").JsonRecord[])).toHaveLength(1);
  });
});

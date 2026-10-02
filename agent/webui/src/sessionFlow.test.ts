import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  loadSessionFlowCollapsed,
  loadSessionFlowRegistry,
  renameSessionFlowEntry,
  saveSessionFlowCollapsed,
  saveSessionFlowRegistry,
  sessionFlowRows,
  sessionFlowServerHints,
  setSessionFlowArchived,
  upsertSessionFlowEntry
} from "./sessionFlow";

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

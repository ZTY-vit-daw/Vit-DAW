/*
 * UI-LAUNCHER-1 launcher surface 单元测试（静态渲染 + 纯函数）。
 *
 * fixture 纪律（卡面）：本文件所有工程名均为测试 fixture，
 * 不代表真实工程或真实 agent 数据；断言不含示例工程名硬编码进生产代码。
 */

import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { LauncherSurface } from "./LauncherSurface";
import {
  cardPendingPlaceholder,
  createLauncherConversationID,
  directivePlaceholder,
  drawerCountLabel,
  greetingDisplayText,
  LAUNCHER_AGENT_UNAVAILABLE_TEXT,
  LAUNCHER_SUGGESTIONS,
  launcherDisplayFileName
} from "./launcherData";
import { cardsToEntries } from "../api/launcher";
import type { LauncherProjectCard } from "../api/launcher";

const FIXTURE_CARD: LauncherProjectCard = {
  id: "launcher_project_1_test",
  kind: "launcher_project",
  type: "project_open_candidate",
  name: "测试工程甲",
  filePath: "D:/fixtures/测试工程甲.vit",
  directive: { action: "open_project", file_path: "D:/fixtures/测试工程甲.vit", requires_governance: true }
};

describe("launcherData 纯函数", () => {
  it("建议词为通用句式：不含具体工程名，数量 3–4", () => {
    expect(LAUNCHER_SUGGESTIONS.length).toBeGreaterThanOrEqual(3);
    expect(LAUNCHER_SUGGESTIONS.length).toBeLessThanOrEqual(4);
    for (const suggestion of LAUNCHER_SUGGESTIONS) {
      expect(suggestion.text).not.toMatch(/\.vit/);
    }
  });

  it("conversation ID 使用 launcher 专属前缀", () => {
    expect(createLauncherConversationID()).toMatch(/^webui_launcher_/);
  });

  it("路径显示名去扩展名（与 agent 侧同语义）", () => {
    expect(launcherDisplayFileName("D:/a/b/磁带流.vit")).toBe("磁带流");
    expect(launcherDisplayFileName("C:\\x\\y\\无扩展名")).toBe("无扩展名");
    expect(launcherDisplayFileName("")).toBe("");
  });

  it("directive 占位：open/new 一律「准备…」且注明不执行", () => {
    const open = directivePlaceholder({ action: "open_project", file_path: "D:/fixtures/甲.vit", requires_governance: true });
    expect(open?.headline).toBe("准备打开《甲》…");
    expect(open?.detail).toBe("D:/fixtures/甲.vit");
    expect(open?.note).toContain("不执行");

    const fresh = directivePlaceholder({ action: "new_project", project_name: "新曲草稿", requires_governance: true });
    expect(fresh?.headline).toBe("准备新建《新曲草稿》…");
    expect(fresh?.note).toContain("不执行");

    expect(directivePlaceholder(null)).toBeNull();
    expect(directivePlaceholder({ action: "", requires_governance: true })).toBeNull();
  });

  it("缺字段 directive 退到通用占位，不虚构工程名", () => {
    const open = directivePlaceholder({ action: "open_project", requires_governance: true });
    expect(open?.headline).toBe("准备打开工程…");
  });

  it("结果卡点击占位复用卡内 directive", () => {
    const view = cardPendingPlaceholder(FIXTURE_CARD);
    expect(view.action).toBe("open");
    expect(view.headline).toBe("准备打开《测试工程甲》…");
    expect(view.note).toContain("UI-BRIDGE-1");
  });

  it("greeting 空回执给确定性文案，不编造工程名", () => {
    expect(greetingDisplayText(null)).toBe("我在。想从哪开始？");
    expect(greetingDisplayText({ reply: "  " })).toBe("我在。想从哪开始？");
    expect(greetingDisplayText({ reply: "我在。最近的工程是《甲》——想从哪开始？" })).toContain("《甲》");
  });

  it("抽屉计数标签区分四态", () => {
    expect(drawerCountLabel({ state: "loading", entries: [], count: 0 })).toContain("正在读取");
    expect(drawerCountLabel({ state: "error", entries: [], count: 0 })).toContain("失败");
    expect(drawerCountLabel({ state: "empty", entries: [], count: 0 })).toContain("还没有");
    expect(drawerCountLabel({ state: "ready", entries: [FIXTURE_CARD], count: 1 })).toBe("最近工程 · 1 个");
  });

  it("agent 不可用文案固定且不虚报可用", () => {
    expect(LAUNCHER_AGENT_UNAVAILABLE_TEXT).toContain("连不上");
    expect(LAUNCHER_AGENT_UNAVAILABLE_TEXT).toContain("重试");
  });
});

describe("cardsToEntries 回执卡映射", () => {
  it("launcher_project 卡映射为条目，缺 name/file_path 的卡跳过", () => {
    const entries = cardsToEntries([
      {
        kind: "launcher_project",
        type: "project_open_candidate",
        id: "launcher_project_1_x",
        name: "测试工程乙",
        file_path: "D:/fixtures/乙.vit",
        launcher_directive: { action: "open_project", file_path: "D:/fixtures/乙.vit", requires_governance: true }
      },
      { kind: "launcher_project", id: "launcher_project_2_empty" }
    ]);
    expect(entries).toHaveLength(1);
    expect(entries[0].name).toBe("测试工程乙");
    expect(entries[0].directive?.action).toBe("open_project");
  });

  it("undefined/空数组安全", () => {
    expect(cardsToEntries(undefined)).toEqual([]);
    expect(cardsToEntries([])).toEqual([]);
  });
});

describe("LauncherSurface 静态渲染（fixture 模式，零网络）", () => {
  it("标题栏只写 Vit 文本：无 i-vit 品牌标进入标题栏", () => {
    const markup = renderToStaticMarkup(<LauncherSurface fixture={{ drawerState: "empty" }} />);
    expect(markup).toContain('data-launcher-titlebar');
    expect(markup).toContain(">Vit</span>");
    const titlebar = markup.split('data-launcher-titlebar')[1]?.split("</header>")[0] ?? "";
    expect(titlebar).not.toContain("i-vit");
    expect(titlebar).not.toContain("v5-project-badge");
  });

  it("ready 态：抽屉/真实 recent 数/建议/输入/问候全在卷", () => {
    const markup = renderToStaticMarkup(
      <LauncherSurface
        fixture={{
          greeting: { status: "ok", launcher: true, reply: "我在。想从哪开始？", recent_count: 1 },
          drawerState: "ready",
          drawerEntries: [FIXTURE_CARD]
        }}
      />
    );
    expect(markup).toContain('data-launcher-greeting');
    expect(markup).toContain("我在。想从哪开始？");
    expect(markup).toContain('data-launcher-drawer');
    expect(markup).toContain('data-launcher-recents-count="1"');
    expect(markup).toContain('data-launcher-recent');
    expect(markup).toContain("测试工程甲");
    expect(markup).toContain('data-launcher-suggestions');
    expect((markup.match(/data-launcher-suggestion=/g) ?? []).length).toBe(3);
    expect(markup).toContain('data-launcher-input');
    expect(markup).toContain('data-launcher-state="ready"');
    expect(markup).toContain('data-fixture="launcher-surface"');
  });

  it("加载态：greeting 与 drawer 各自 loading 文案", () => {
    const markup = renderToStaticMarkup(
      <LauncherSurface fixture={{ thread: [], drawerState: "loading" }} />
    );
    // fixture greetingErrorText 未给 → ready greeting；drawer loading
    expect(markup).toContain('data-drawer-state="loading"');
    expect(markup).toContain("正在读取最近工程…");
  });

  it("错误态：agent 不可用 banner + 重试按钮 + 抽屉错误态", () => {
    const markup = renderToStaticMarkup(
      <LauncherSurface fixture={{ greetingErrorText: LAUNCHER_AGENT_UNAVAILABLE_TEXT, drawerState: "error", drawerErrorText: "内核未就绪" }} />
    );
    expect(markup).toContain('data-launcher-state="error"');
    expect(markup).toContain('data-launcher-agent-error');
    expect(markup).toContain("连不上 Vit agent");
    expect(markup).toContain('data-launcher-retry');
    expect(markup).toContain('data-drawer-state="error"');
    expect(markup).toContain("内核未就绪");
    expect(markup).toContain('data-launcher-drawer-retry');
  });

  it("directive 线程条目渲染为待执行占位（不虚报已打开）", () => {
    const markup = renderToStaticMarkup(
      <LauncherSurface
        fixture={{
          greeting: { reply: "我在。想从哪开始？" },
          drawerState: "empty",
          thread: [{ kind: "directive", view: directivePlaceholder({ action: "open_project", file_path: "D:/fixtures/甲.vit", requires_governance: true })! }]
        }}
      />
    );
    expect(markup).toContain('data-launcher-directive-state="pending"');
    expect(markup).toContain("准备打开《甲》…");
    expect(markup).not.toContain("已打开");
  });

  it("结果卡线程条目为数据展示卡", () => {
    const markup = renderToStaticMarkup(
      <LauncherSurface
        fixture={{
          greeting: { reply: "最近的工程有 1 个——点卡片打开。" },
          drawerState: "ready",
          drawerEntries: [FIXTURE_CARD],
          thread: [{ kind: "cards", cards: [FIXTURE_CARD] }]
        }}
      />
    );
    expect(markup).toContain('data-launcher-cards');
    expect(markup).toContain('data-launcher-card="launcher_project_1_test"');
  });
});

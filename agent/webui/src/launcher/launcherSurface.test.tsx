/*
 * UI-LAUNCHER-1 launcher surface 单元测试（静态渲染 + 纯函数）。
 *
 * fixture 纪律（卡面）：本文件所有工程名均为测试 fixture，
 * 不代表真实工程或真实 agent 数据；断言不含示例工程名硬编码进生产代码。
 */

import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import { LauncherSurface } from "./LauncherSurface";
import {
  buildDirectNewDirective,
  cardPendingPlaceholder,
  createLauncherConversationID,
  defaultNewProjectName,
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

  it("directive 占位（SHELL-5）：open/new 一律「正在…」自动执行语态，不虚报已打开", () => {
    const open = directivePlaceholder({ action: "open_project", file_path: "D:/fixtures/甲.vit", requires_governance: true });
    expect(open?.headline).toBe("正在打开《甲》…");
    expect(open?.detail).toBe("D:/fixtures/甲.vit");
    expect(open?.note).toContain("正在执行");
    expect(open?.note).toContain("UI-BRIDGE-1");

    const fresh = directivePlaceholder({ action: "new_project", project_name: "新曲草稿", requires_governance: true });
    expect(fresh?.headline).toBe("正在新建《新曲草稿》…");
    expect(fresh?.note).toContain("正在执行");

    expect(directivePlaceholder(null)).toBeNull();
    expect(directivePlaceholder({ action: "", requires_governance: true })).toBeNull();
  });

  it("缺字段 directive 退到通用占位，不虚构工程名", () => {
    const open = directivePlaceholder({ action: "open_project", requires_governance: true });
    expect(open?.headline).toBe("正在打开工程…");
  });

  it("结果卡点击占位复用卡内 directive（自动执行语态）", () => {
    const view = cardPendingPlaceholder(FIXTURE_CARD);
    expect(view.action).toBe("open");
    expect(view.headline).toBe("正在打开《测试工程甲》…");
    expect(view.note).toContain("UI-BRIDGE-1");
  });

  it("缺省新工程名与直连 directive（SHELL-5：＋新建按钮直建）", () => {
    expect(defaultNewProjectName(new Date(2026, 10, 15, 21, 32).getTime())).toBe("新工程_1115_2132");
    expect(defaultNewProjectName(new Date(2026, 0, 2, 3, 4).getTime())).toBe("新工程_0102_0304");
    const directive = buildDirectNewDirective(new Date(2026, 10, 15, 21, 32).getTime());
    expect(directive.action).toBe("new_project");
    expect(directive.project_name).toBe("新工程_1115_2132");
    expect(directive.requires_governance).toBe(true);
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

  it("directive 线程条目渲染为自动执行回执（SHELL-5：无手动按钮、不虚报已打开）", () => {
    const markup = renderToStaticMarkup(
      <LauncherSurface
        fixture={{
          greeting: { reply: "我在。想从哪开始？" },
          drawerState: "empty",
          thread: [{ kind: "directive", view: directivePlaceholder({ action: "open_project", file_path: "D:/fixtures/甲.vit", requires_governance: true })! }]
        }}
      />
    );
    expect(markup).toContain('data-launcher-directive-state="auto"');
    expect(markup).toContain("正在打开《甲》…");
    // 自动执行语义：不再出「让 Vit 打开/新建」手动按钮。
    expect(markup).not.toContain("data-launcher-directive-execute");
    expect(markup).not.toContain("让 Vit 打开");
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

// ── UI-BRIDGE-1：Vit 桥呈现面（静态渲染；fixture 模式零桥调用）──

const FIXTURE_DIRECTIVE = { action: "open_project", file_path: "D:/fixtures/甲.vit", requires_governance: true };

describe("UI-BRIDGE-1 桥呈现面", () => {
  it("fixture 线程不带 directive → 无执行按钮（纯占位呈报，行为零变化）", () => {
    const markup = renderToStaticMarkup(
      <LauncherSurface
        fixture={{
          thread: [{ kind: "directive", view: directivePlaceholder(FIXTURE_DIRECTIVE)! }]
        }}
      />
    );
    expect(markup).not.toContain("data-launcher-directive-execute");
    expect(markup).not.toContain("data-launcher-bridge-exec");
  });

  it("占位携带 directive（SHELL-5 自动执行）→ 无手动执行按钮；桥不可用给诚实说明", () => {
    const markup = renderToStaticMarkup(
      <LauncherSurface
        fixture={{
          thread: [{ kind: "directive", view: directivePlaceholder(FIXTURE_DIRECTIVE)!, directive: FIXTURE_DIRECTIVE }]
        }}
      />
    );
    expect(markup).toContain('data-launcher-directive-state="auto"');
    expect(markup).not.toContain("data-launcher-directive-execute");
    expect(markup).not.toContain("让 Vit 打开");
    expect(markup).toContain('data-launcher-bridge="fixture"');
    // fixture 模式 bridgeAvailable=false → 桥不可用诚实说明随 directive 在场。
    expect(markup).toContain("Vit 桥不可用");
    expect(markup).toContain('data-launcher-bridge-note');
    expect(markup).not.toContain("已打开");
    expect(markup).not.toContain('data-launcher-bridge-exec');
  });

  it("fixture 模式 data-launcher-bridge=fixture；不发起任何桥请求", () => {
    const markup = renderToStaticMarkup(<LauncherSurface fixture={{ drawerState: "empty" }} />);
    expect(markup).toContain('data-launcher-bridge="fixture"');
    expect(markup).not.toContain("data-launcher-choose-file");
  });
});

// ── SHELL-2：抽屉手动按钮 + 页内治理三键卡 ──

describe("launcherSurface 抽屉手动按钮（SHELL-2；SHELL-5 语义收口）", () => {
  it("抽屉头部「＋新建」=直连新建（不经对话问询）+「打开…」宿主对话框", () => {
    const markup = renderToStaticMarkup(
      <LauncherSurface fixture={{ drawerState: "empty" }} />
    );
    expect(markup).toContain("data-launcher-new");
    expect(markup).toContain('data-launcher-new-mode="direct"');
    expect(markup).toContain("＋新建");
    expect(markup).toContain("不经对话问询");
    expect(markup).toContain("data-launcher-drawer-actions");
    // 空 recents 态主路径突出修饰在场。
    expect(markup).toContain('data-drawer-primary="true"');
    // 空态文案指向直建按钮（SHELL-5）。
    expect(markup).toContain("点「＋新建」直接新建");
  });

  it("live 桥可用（stub chrome.webview）→ 头部打开按钮在场（宿主原生对话框）", () => {
    vi.stubGlobal("window", {
      location: { search: "" },
      chrome: { webview: { postMessage: () => {}, addEventListener: () => {}, removeEventListener: () => {} } }
    });
    try {
      // live 面（无 fixture prop）：bridgeAvailable 只由 chrome.webview 在场判定。
      const markup = renderToStaticMarkup(<LauncherSurface />);
      expect(markup).toContain('data-launcher-choose-file');
      expect(markup).toContain("打开…");
      // 有 recents 时不加空态突出修饰。
      expect(markup).not.toContain('data-drawer-primary="true"');
    } finally {
      vi.unstubAllGlobals();
    }
  });
});

// ── SHELL-4：内核态呈现（三态之二/之三的 UI 侧诚实区分）──

describe("launcherSurface 内核态呈现（SHELL-4）", () => {
  it("greeting kernel_unavailable=true → data-launcher-kernel=unavailable（内核不可达不冒充在线/空态）", () => {
    const markup = renderToStaticMarkup(
      <LauncherSurface
        fixture={{
          greeting: {
            status: "ok",
            launcher: true,
            reply: "我在。暂时连不上内核，最近工程列表取不到（不是对话 API 的问题）——稍等片刻再试，或点「打开…」选择工程文件。",
            recent_count: 0,
            kernel_unavailable: true
          },
          drawerState: "error",
          drawerErrorText: "我这儿暂时连不上内核（不是对话 API 的问题）——最近工程列表稍后再试。"
        }}
      />
    );
    expect(markup).toContain('data-launcher-kernel="unavailable"');
    expect(markup).toContain("不是对话 API 的问题");
    expect(markup).toContain('data-drawer-state="error"');
    expect(markup).toContain('data-launcher-drawer-retry');
    // 与 agent 故障区分：agent 不可用 banner 不在场（agent 在线、内核不可达）。
    expect(markup).not.toContain("连不上 Vit agent");
  });

  it("greeting 无 kernel 标记 → data-launcher-kernel=online", () => {
    const markup = renderToStaticMarkup(
      <LauncherSurface
        fixture={{
          greeting: { status: "ok", launcher: true, reply: "我在。最近的工程是《测试工程甲》——想从哪开始？", recent_count: 1 },
          drawerState: "ready",
          drawerEntries: [FIXTURE_CARD]
        }}
      />
    );
    expect(markup).toContain('data-launcher-kernel="online"');
  });

  it("空态文案不再引用磁带流（v5.1 UI 无磁带流元素）", () => {
    const markup = renderToStaticMarkup(<LauncherSurface fixture={{ drawerState: "empty" }} />);
    expect(markup).toContain('data-drawer-state="empty"');
    expect(markup).toContain("还没有最近工程记录");
    expect(markup).not.toContain("磁带流");
  });
});

describe("launcherSurface 页内治理三键卡（SHELL-2）", () => {
  it("fixture 线程治理项 → 三键全在场且 headline 陈述在开工程", () => {
    const markup = renderToStaticMarkup(
      <LauncherSurface
        fixture={{
          greeting: { status: "ok", reply: "fixture" },
          thread: [
            { kind: "governance", view: { requestID: "req_gov_1", action: "open_project", currentProject: "D:/fixtures/测试工程甲.vit" } }
          ]
        }}
      />
    );
    expect(markup).toContain('data-launcher-governance-request="req_gov_1"');
    expect(markup).toContain("data-launcher-governance-save");
    expect(markup).toContain("data-launcher-governance-switch");
    expect(markup).toContain("data-launcher-governance-cancel");
    expect(markup).toContain("保存并切换");
    expect(markup).toContain("直接切换");
    expect(markup).toContain("取消");
    expect(markup).toContain("测试工程甲");
    // 未作答： answered 标记为空，作答文案不在场。
    expect(markup).toContain('data-launcher-governance-answered=""');
    expect(markup).not.toContain("已取消——未切换工程");
  });

  it("已作答治理卡 → 三键退场，固定呈报在场（不重放选择）", () => {
    const markup = renderToStaticMarkup(
      <LauncherSurface
        fixture={{
          greeting: { status: "ok", reply: "fixture" },
          thread: [
            { kind: "governance", view: { requestID: "req_gov_2", action: "open_project", currentProject: "" }, answered: "switch" }
          ]
        }}
      />
    );
    expect(markup).toContain('data-launcher-governance-answered="switch"');
    expect(markup).toContain("已选择「直接切换」——等待 Vit 执行。");
    expect(markup).not.toContain("data-launcher-governance-save");
    // 态未知 headline 保守呈现（不虚构工程名）。
    expect(markup).toContain("状态未知");
  });
});

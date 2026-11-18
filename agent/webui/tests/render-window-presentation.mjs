// 395 independent render leg: index.html -> main.tsx -> App production tree.
// Only HTTP/host inputs are fixtures. Never connects to agent/Godot/native UI.
import { chromium } from "playwright";
import { createServer } from "vite";
import { mkdirSync, writeFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const webuiRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const runRoot = resolve(process.env.VIT_WINDOW_RENDER_OUTPUT || resolve(webuiRoot, "output/playwright/395-" + new Date().toISOString().replace(/[:.]/g, "-")));
mkdirSync(runRoot, { recursive: true });
const report = { scope: "real App + mocked HTTP/host; no real stack", checks: [], requests: [], invokes: [], consoleErrors: [], failure: null };
// CONV-RAIL-1（418，B2a）render fixture：对话流左栏树/只读预览/继续/新对话全部走
// mock HTTP（零真栈、零 agent）。/agent/ui/state 按下列状态动态投影；/agent/invoke
// 记录真实请求参数（tool/args/confirmed/source/context）并按生产回执语义回执。
const conversation = { activeBranch: "main", checkoutBlocked: false, createdBranches: [] };
const conversationNode = (id, kind, text, branch, commitID, parentNodeID, createdAt) => ({ id, kind, text, text_preview: text, branch, commit_id: commitID, parent_node_id: parentNodeID, created_at: createdAt });
const conversationHistory = () => {
  // 基础 fixture：main（4 条消息）+ 从 n-base-vit 分叉的 feature-x（2 条消息）；运行中
  // branch_create 出来的新分支追加 branch_marker（与 Go BranchCreate 写入一致）。
  const graphNodes = [
    conversationNode("n-base-ask", "ask", "把贝斯轨压低 2dB", "main", "c1", "", "2026-11-18T19:00:00"),
    conversationNode("n-base-vit", "vit", "已把贝斯轨压低 2dB。", "main", "c2", "n-base-ask", "2026-11-18T19:01:00"),
    conversationNode("n-main-ask", "ask", "把鼓组整体再亮一点", "main", "c3", "n-base-vit", "2026-11-18T19:05:00"),
    conversationNode("n-main-vit", "vit", "鼓组高频已提升。", "main", "c4", "n-main-ask", "2026-11-18T19:06:00"),
    conversationNode("n-fx-marker", "branch_marker", "branch feature-x", "feature-x", "c2", "n-base-vit", "2026-11-18T19:10:00"),
    conversationNode("n-fx-ask", "ask", "贝斯再加一点压缩", "feature-x", "f1", "n-fx-marker", "2026-11-18T19:11:00"),
    conversationNode("n-fx-vit", "vit", "贝斯压缩已加 2:1。", "feature-x", "f2", "n-fx-ask", "2026-11-18T19:12:00")
  ];
  const branches = { main: "c4", "feature-x": "f2" };
  conversation.createdBranches.forEach(created => {
    const commitID = created.commit_id || "c4";
    branches[created.name] = commitID;
    graphNodes.push(conversationNode("n-" + created.name + "-marker", "branch_marker", "branch " + created.name, created.name, commitID, created.from_node_id, "2026-11-18T20:03:11"));
  });
  const activeNodeFor = branch => {
    const created = conversation.createdBranches.find(entry => entry.name === branch);
    if (created) return "n-" + created.name + "-marker";
    if (branch === "feature-x") return "n-fx-vit";
    return "n-main-vit";
  };
  const activeNodeID = activeNodeFor(conversation.activeBranch);
  return {
    project_path: "C:/fixture/alpha.vit",
    root_project_path: "C:/fixture/alpha.vit",
    active_branch: conversation.activeBranch,
    active_node_id: activeNodeID,
    active_worktree: "",
    detached: false,
    head: branches[conversation.activeBranch] || "c4",
    branches,
    worktrees: [],
    conversation_graph: {
      project_path: "C:/fixture/alpha.vit",
      active_branch: conversation.activeBranch,
      active_node_id: activeNodeID,
      nodes: graphNodes
    }
  };
};
const conversationUIStateBody = () => ({ status: "ok", project: { project_path: "C:/fixture/alpha.vit" }, tracks: [], checkout_blocked: conversation.checkoutBlocked, project_history: conversationHistory() });
let server, browser;
const check = (name, condition) => { report.checks.push({ name, pass: Boolean(condition) }); if (!condition) throw new Error(name); };
const observation = (foreground, tick = 1) => ({ schema: "vitshell.window_state.v1", foreground, daw_hwnd: foreground === "daw" ? "22" : null, observed_at_tick_ms: tick, release_reason: "workspace_ready" });
try {
  server = await createServer({ root: webuiRoot, server: { host: "127.0.0.1", port: 0, strictPort: false }, clearScreen: false });
  await server.listen();
  const origin = server.resolvedUrls.local[0].replace(/\/$/, "").replace(/\/app$/, "");
  browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  await context.route("**/*", async route => {
    const url = new URL(route.request().url());
    if (url.pathname.startsWith("/agent/")) {
      report.requests.push({ path: url.pathname, method: route.request().method() });
      let body = {};
      if (url.pathname === "/agent/launcher/greeting") body = { status: "ok", reply: "选择工程（render fixture）", recent_count: 1 };
      if (url.pathname === "/agent/launcher/recent-projects") body = { status: "ok", project_result_cards: [{ kind: "launcher_project", name: "工程甲（render fixture）", file_path: "C:/fixture/alpha.vit", exists: true }] };
      if (url.pathname === "/agent/ui/state") body = conversationUIStateBody();
      if (url.pathname === "/agent/chat") body = { reply: "切换工程（render fixture）", launcher_directive: { action: "new_project", project_name: "工程乙（render fixture）", requires_governance: true } };
      if (url.pathname === "/agent/invoke") {
        // CONV-RAIL-1（418）：记录生产缝请求参数；120ms 延迟让 loading/禁用态可断言。
        let payload = {};
        try { payload = route.request().postDataJSON() || {}; } catch { payload = {}; }
        report.invokes.push(payload);
        await new Promise(resolve => setTimeout(resolve, 120));
        if (payload.tool === "version.checkout") {
          conversation.activeBranch = payload.args?.branch || conversation.activeBranch;
          body = { status: "ok", result: { status: "ok", branch: payload.args?.branch || "" } };
        } else if (payload.tool === "version.branch_create") {
          const created = { name: payload.args?.name || "", from_node_id: payload.args?.from_node_id || "", commit_id: payload.args?.commit_id || "" };
          conversation.createdBranches.push(created);
          conversation.activeBranch = created.name;
          body = { status: "ok", result: { status: "ok", branch: created.name, checked_out_to: "C:/fixture/alpha.vit" } };
        } else {
          body = { status: "ok", result: {} };
        }
      }
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(body) });
      return;
    }
    if (url.origin !== new URL(origin).origin && !["data:", "blob:"].includes(url.protocol)) { report.requests.push({ external: url.href }); await route.abort(); return; }
    await route.continue();
  });
  await context.addInitScript(() => {
    const listeners = new Set();
    window.__hostPosts = [];
    window.chrome = { webview: {
      postMessage: value => window.__hostPosts.push(value),
      addEventListener: (type, listener) => { if (type === "message") listeners.add(listener); },
      removeEventListener: (type, listener) => { if (type === "message") listeners.delete(listener); }
    } };
    window.__emitHost = value => { for (const listener of [...listeners]) listener(new MessageEvent("message", { data: value })); };
  });
  const page = await context.newPage();
  page.on("pageerror", error => report.consoleErrors.push(error.message));
  const emit = data => page.evaluate(value => window.__emitHost(value), data);
  const progress = (requestId, seq, phase, extra = {}) => emit({ type: "vit_launch_progress", schema: "vitshell.launch_progress.v1", request_id: requestId, seq, phase, detail: "", ...extra });
  const currentRequest = () => page.evaluate(() => [...window.__hostPosts].reverse().find(p => p.type === "launcher_directive")?.request_id);
  const postResult = (requestId, status, message = "fixture result") => emit({ type: "vit_launcher_result", request_id: requestId, status, message, action: "open_project" });
  const feedback = () => page.locator("[data-window-launch-feedback]:visible");
  const settleMotion = () => page.evaluate(() => Promise.all(document.getAnimations().filter(animation => animation.playState !== "finished" && animation.effect?.getTiming().iterations !== Infinity).map(animation => animation.finished.catch(() => {}))));
  await page.goto(origin + "/app/?surface=launcher");
  await page.locator("[data-launcher-recent-open]").first().waitFor();
  await page.waitForFunction(() => window.__hostPosts.some(p => p.type === "window_presentation_ready"));
  check("production App mounts presentation in launcher", await page.locator("[data-launcher-root]").getAttribute("data-native-focus") === "unknown");
  check("no initial host observation means no focus claim", await page.locator("[data-native-focus-hint]").count() === 0);
  const subscription = await page.evaluate(() => window.__hostPosts.filter(p => p.type === "window_presentation_ready").at(-1).subscription_id);
  await emit({ type: "vit_window_state", window_state: observation("daw"), replay: true, subscription_id: "other-subscription" });
  check("wrong subscription replay ignored", await page.locator("[data-native-focus-hint]").count() === 0);
  await emit({ type: "vit_window_state", window_state: observation("shell"), replay: true, subscription_id: subscription });
  await page.locator('[data-launcher-root][data-native-focus="shell"]').waitFor();
  // SHELL-ENTRY-RAIL-1（417）：窗口级常驻左栏——live App 在 WindowPresentationProvider
  // 内包 EntryShell；启动页工程列表由 launcher 抽屉门户迁入左栏，右主区保留原 surface。
  await page.locator("[data-entry-rail]").waitFor();
  const railWidth = () => page.locator("[data-entry-rail]").evaluate(el => el.getBoundingClientRect().width);
  const railIdentity = await page.evaluateHandle(() => document.querySelector("[data-entry-rail]"));
  check("live entry App mounts one window-level rail", await page.locator("[data-entry-rail]").count() === 1);
  check("entry rail is the 280px persistent left column", Math.round(await railWidth()) === 280);
  check("startup project list is portaled into the entry rail", await page.locator("[data-entry-rail] [data-launcher-projects]").count() === 1 && await page.locator("[data-entry-rail] [data-launcher-drawer]").count() === 1);
  check("entry rail header owns the surface title and collapse control", (await page.locator("[data-entry-rail] [data-entry-rail-title]").innerText()).trim() === "工程列表" && await page.locator("[data-entry-rail] [data-window-sidebar-toggle]").count() === 1);
  check("launcher titlebar no longer duplicates the collapse control", await page.locator("[data-launcher-titlebar] [data-window-sidebar-toggle]").count() === 0);
  // CONV-RAIL-1（418，B2a）：rail 模式下「Vit」品牌只在左栏头出现，启动页主标题栏不再
  // 重复；左栏 footer（设置 + 预留头像槽）固定常驻在左栏底。
  check("rail head owns the only Vit brand in rail mode", await page.locator("[data-entry-rail] [data-entry-rail-brand]").count() === 1 && (await page.locator("[data-entry-rail] [data-entry-rail-brand]").innerText()).trim() === "Vit" && await page.locator("[data-launcher-titlebar] .v5-launcher-titlebar-brand").count() === 0);
  const railFootIdentity = await page.evaluateHandle(() => document.querySelector("[data-entry-rail-foot]"));
  check("rail footer is persistent with settings and an avatar slot", await page.locator("[data-entry-rail] [data-entry-rail-foot] [data-entry-rail-settings]").count() === 1 && await page.locator("[data-entry-rail] [data-entry-rail-foot] [data-entry-rail-avatar]").count() === 1);
  // SHELL-SETTINGS-DUP-1（426）：live entry 唯一设置入口 = 左栏 footer。旧 converged
  // launcher 抽屉设置（data-launcher-settings）与 project footer 设置（data-project-settings）
  // 在 live entry 下不再渲染（不传 onOpenSettings），此处守住唯一性。
  check("live entry exposes exactly one settings entry on the launcher surface", await page.locator("[data-entry-rail-foot] [data-entry-rail-settings]").count() === 1 && await page.locator("[data-launcher-settings]").count() === 0 && await page.locator("[data-project-settings]").count() === 0);
  check("rail footer stays pinned to the rail bottom on the launcher surface", await page.locator("[data-entry-rail-foot]").evaluate(element => {
    const foot = element.getBoundingClientRect();
    const rail = element.closest("[data-entry-rail]").getBoundingClientRect();
    return Math.abs(foot.bottom - rail.bottom) < 1 && foot.top > rail.top + rail.height / 2;
  }));
  check("right main area keeps the live launcher surface", await page.locator("[data-entry-main] [data-launcher-root]").count() === 1);
  await page.locator("[data-window-sidebar-toggle]").focus();
  await page.keyboard.press("Enter");
  await page.waitForFunction(() => document.querySelector("[data-window-sidebar-slot]").getBoundingClientRect().width < 1);
  check("keyboard collapse hides and inerts only existing recents slot", await page.locator("[data-window-sidebar-slot]").evaluate(el => el.inert && el.getAttribute("aria-hidden") === "true"));
  check("collapse button remains keyboard accessible", await page.locator("[data-window-sidebar-toggle]").getAttribute("aria-expanded") === "false");
  await page.waitForFunction(() => document.querySelector("[data-entry-rail]").getBoundingClientRect().width < 1);
  check("collapsing the rail releases its width to the main area", Math.round(await railWidth()) === 0
    && Math.round(await page.locator("[data-entry-main]").evaluate(el => el.getBoundingClientRect().width)) === 1440
    && await page.locator("[data-launcher-input] textarea").isVisible());
  await page.keyboard.press("Enter");
  await page.waitForFunction(() => Math.round(document.querySelector("[data-entry-rail]").getBoundingClientRect().width) === 280);
  check("expanding restores the same rail node and recents slot", await page.evaluate(node => node === document.querySelector("[data-entry-rail]"), railIdentity)
    && Math.round(await railWidth()) === 280
    && await page.locator("[data-entry-rail-slot]").evaluate(el => !el.inert && el.getAttribute("aria-hidden") === "false"));
  await page.locator("[data-launcher-recent-open]").first().click();
  let request = await currentRequest();
  await feedback().waitFor();
  check("request immediately shows unknown progress and gates input", await feedback().getAttribute("data-window-launch-progress") === "unknown" && await page.locator("[data-launcher-input] textarea").isDisabled());
  await progress(request, 1, "waiting_governance");
  await page.getByText("正在核对工程切换条件", { exact: true }).waitFor();
  await emit({ type: "vit_governance_required", request_id: request, action: "open_project", current_project: "fixture current" });
  await page.locator("[data-launcher-governance-cancel]").click();
  await postResult(request, "canceled");
  await progress(request, 2, "canceled");
  await feedback().locator('[data-loading-phase="interrupted"]').waitFor();
  check("governance cancel is not completed", await feedback().locator('[data-loading-phase="completed"]').count() === 0 && !await page.locator("[data-launcher-input] textarea").isDisabled());

  await page.locator("[data-launcher-recent-open]").first().click(); request = await currentRequest();
  await progress(request, 3, "saving");
  await progress(request, 4, "closing_daw");
  await progress(request, 5, "spawning");
  await progress(request, 6, "waiting_launch_result");
  await page.getByText("正在等待工作台装载回执", { exact: true }).waitFor();
  const beforeDuplicate = await feedback().innerText();
  await progress(request, 6, "waiting_launch_result");
  await progress("old-request", 999, "opened", { stage: "workspace_ready" });
  check("duplicate and stale request cannot advance UI", await feedback().innerText() === beforeDuplicate && await feedback().getAttribute("data-window-launch-seq") === "6");
  check("only real observed phases, at most three; no fake file/audio steps", await feedback().locator("[data-loading-step-status]").count() <= 3 && !/已读取工程文件|已恢复轨道|连接音频设备|\d+%/.test(await feedback().innerText()));
  check("current step remains inside actual thread viewport", await feedback().evaluate(el => {
    const step = el.querySelector('[data-loading-step-status="current"]');
    const thread = el.closest("[data-launcher-thread]");
    const a = step.getBoundingClientRect(), b = thread.getBoundingClientRect();
    return a.top >= b.top - 1 && a.bottom <= b.bottom + 1;
  }));
  // SHELL-RING-SPIN-FIX-1（416）：pending 装载卡的黄色状态环必须绕自身几何中心原地
  // 自转。旧缺陷=transform-origin 固定 12px 12px（24px 图标坐标）+ loading chip 的
  // translateY 覆盖 animation：16px 档轴心偏 4px → 绕轴环绕。以下为 computed-style 亲证。
  const ringSpin = await feedback().evaluate(el => {
    const ring = el.querySelector(".v5-loading-chip .v5-ring");
    if (!ring) return { found: false };
    const style = getComputedStyle(ring);
    const matrix = style.transform === "none" ? null : new DOMMatrixReadOnly(style.transform);
    return {
      found: true,
      size: Number(ring.getAttribute("data-icon-size")),
      styleSize: { width: style.width, height: style.height },
      animationName: style.animationName,
      animationDuration: style.animationDuration,
      animationIterationCount: style.animationIterationCount,
      transformOrigin: style.transformOrigin,
      transformBox: style.transformBox,
      activeAnimations: ring.getAnimations().map(animation => animation.animationName),
      matrix: matrix ? { a: Number(matrix.a.toFixed(3)), b: Number(matrix.b.toFixed(3)), c: Number(matrix.c.toFixed(3)), d: Number(matrix.d.toFixed(3)), e: Number(matrix.e.toFixed(3)), f: Number(matrix.f.toFixed(3)) } : null
    };
  });
  report.ringSpin = ringSpin;
  check("pending loading chip renders the 16px busy ring", ringSpin.found ? ringSpin.size === 16 && ringSpin.styleSize.width === "16px" && ringSpin.styleSize.height === "16px" : false);
  check("pending chip ring keeps SPEC v5-ring-spin (busy 1.6s) and is not overridden", ringSpin.animationName === "v5-ring-spin" && ringSpin.animationDuration === "1.6s" && ringSpin.animationIterationCount === "infinite" && ringSpin.activeAnimations.length === 1 && ringSpin.activeAnimations[0] === "v5-ring-spin");
  check("pending chip ring origin is its geometric center (16px => 8px 8px / 50% 50%)", ringSpin.transformOrigin === "8px 8px" && ringSpin.transformBox === "view-box");
  check("no translateY chip override competes with ring spin", ringSpin.matrix !== null && Math.abs(ringSpin.matrix.e) < 0.01 && Math.abs(ringSpin.matrix.f) < 0.01 && Math.abs(Math.abs(ringSpin.matrix.a) - Math.abs(ringSpin.matrix.d)) < 0.01 && Math.abs(ringSpin.matrix.b + ringSpin.matrix.c) < 0.01);
  await settleMotion();
  await page.screenshot({ path: resolve(runRoot, "launcher-pending.png") });
  await progress(request, 7, "failed", { detail: "首份快照失败（render fixture）" });
  await postResult(request, "failed", "首份快照失败（render fixture）");
  await feedback().locator('[data-loading-phase="failed"]').waitFor();
  check("snapshot failure is visible and retry uses actual directive", (await feedback().innerText()).includes("首份快照失败") && await feedback().locator("[data-loading-retry]").count() === 1);
  await feedback().locator("[data-loading-retry]").click(); request = await currentRequest();
  await postResult(request, "opened");
  await feedback().locator("[data-window-launch-limited]").waitFor();
  check("legacy opened alone cannot claim workspace ready", await feedback().getAttribute("data-window-launch-terminal") === "unconfirmed");
  await progress(request, 8, "opened", { stage: "scene_accepted" });
  await page.waitForFunction(() => document.querySelector("[data-window-launch-feedback]").dataset.windowLaunchTerminal === "accepted");
  check("explicit scene accepted remains limited", (await feedback().innerText()).includes("完整装载状态尚未确认") && await feedback().locator('[data-loading-phase="completed"]').count() === 0);

  await page.locator("[data-launcher-recent-open]").first().click(); request = await currentRequest();
  await progress(request, 9, "waiting_launch_result");
  await postResult(request, "opened");
  await progress(request, 10, "opened", { stage: "workspace_ready" });
  await feedback().locator('[data-loading-phase="completed"]').waitFor();
  await emit({ type: "vit_surface", surface: "project" });
  await page.locator("[data-project-root]").waitFor();
  check("ready feedback survives actual launcher unmount/project mount", await feedback().getAttribute("data-window-launch-terminal") === "ready" && (await feedback().innerText()).includes("工程内容已载入"));
  await emit({ type: "vit_window_state", window_state: observation("daw", 30) });
  await page.locator("[data-project-titlebar] [data-native-focus-hint]").waitFor();
  check("observed DAW foreground changes only header feedback", await page.locator("[data-project-root]").getAttribute("data-native-focus") === "daw" && await feedback().getAttribute("data-window-launch-terminal") === "ready");
  check("same rail DOM node survives the launcher→project surface switch", await page.evaluate(node => node === document.querySelector("[data-entry-rail]"), railIdentity));
  await page.locator("[data-entry-rail] [data-conversation-rail]").waitFor();
  // CONV-RAIL-1（418，B2a）：工程面左栏 = 当前工作树对话流树（数据=真实 /agent/ui/state
  // mock 投影；标题=首条有意义用户消息摘要、副标=分支名·最近活动、当前分支高亮）。
  check("project rail renders the current worktree conversation tree", (await page.locator("[data-conversation-rail] [data-conversation-worktree-label]").innerText()).trim() === "主工作树" && await page.locator("[data-conversation-branch]").count() === 2);
  check("branch rows show first meaningful user message and branch/activity subtitle", (await page.locator('[data-conversation-branch="main"] [data-conversation-branch-title]').innerText()).trim() === "把贝斯轨压低 2dB" && (await page.locator('[data-conversation-branch="feature-x"] [data-conversation-branch-title]').innerText()).trim() === "贝斯再加一点压缩" && (await page.locator('[data-conversation-branch="feature-x"] [data-conversation-branch-subtitle]').innerText()).includes("feature-x · 最近活动"));
  check("current branch is highlighted and marked in the conversation tree", await page.locator('[data-conversation-branch="main"][data-conversation-branch-active="true"]').count() === 1 && await page.locator('[data-conversation-branch="main"] [data-conversation-branch-current]').count() === 1 && await page.locator('[data-conversation-branch="feature-x"][data-conversation-branch-active="false"]').count() === 1);
  check("project surface rail no longer renders the startup project list", await page.locator("[data-entry-rail] [data-launcher-projects]").count() === 0 && (await page.locator("[data-entry-rail] [data-entry-rail-title]").innerText()).trim() === "对话流");
  check("rail footer survives the launcher→project surface switch as the same node", await page.evaluate(node => node === document.querySelector("[data-entry-rail-foot]"), railFootIdentity) && await page.locator("[data-entry-rail-foot] [data-entry-rail-settings]").count() === 1);
  // SHELL-SETTINGS-DUP-1（426）：工程面对话窗下不得再有重复设置按钮（用户手测缺陷）。
  check("project surface has no duplicated settings button outside the rail footer", await page.locator("[data-project-settings]").count() === 0 && await page.locator("[data-launcher-settings]").count() === 0 && await page.locator("[data-entry-rail-settings]").count() === 1 && await page.locator("[data-entry-rail-foot] [data-entry-rail-settings]").count() === 1);
  check("rail expansion state survives the surface switch", await page.locator("[data-window-sidebar-toggle]").getAttribute("aria-expanded") === "true");

  // CONV-RAIL-1（418）：点击非当前分支 = 只读预览（消息路径 + composer 蒙版 + 继续卡），
  // 不调 invoke、不重载；返回/点当前分支清除预览。
  await page.locator('[data-conversation-branch="feature-x"]').click();
  await page.locator("[data-project-preview]").waitFor();
  const previewBranch = await page.locator("[data-project-preview]").getAttribute("data-project-preview-branch");
  const previewMessageTexts = await page.locator("[data-project-preview-thread] [data-message-author]").evaluateAll(elements => elements.map(element => element.innerText.trim()));
  check("clicking another branch previews its message path read-only", previewBranch === "feature-x" && previewMessageTexts.length === 4 && previewMessageTexts[0].includes("把贝斯轨压低 2dB") && previewMessageTexts[3].includes("贝斯压缩已加"));
  check("read-only preview masks and disables the composer", await page.locator("[data-project-composer-mask]").count() === 1 && await page.locator("[data-project-composer] input").isDisabled() && await page.locator('[data-conversation-branch="feature-x"]').getAttribute("data-conversation-branch-preview") === "true");
  check("entering preview never invokes checkout", report.invokes.length === 0);
  await page.locator("[data-project-composer-mask-return]").click();
  check("returning clears preview and restores the composer", await page.locator("[data-project-preview]").count() === 0 && !await page.locator("[data-project-composer] input").isDisabled());
  await page.locator('[data-conversation-branch="feature-x"]').click();
  await page.locator("[data-project-preview]").waitFor();
  await page.locator('[data-conversation-branch="main"]').click();
  check("clicking the current branch clears the preview", await page.locator("[data-project-preview]").count() === 0);

  // 【继续此对话】：window.confirm 守卫（取消=不切换、保留预览）→ version.checkout
  // （historyToolArgs/buildHistoryInvokeContext 组装的生产缝参数）→ loading 禁用重复 →
  // 成功后刷新树并清预览。
  const dialogMessages = [];
  let acceptDialogs = false;
  page.on("dialog", async dialog => {
    dialogMessages.push(dialog.message());
    if (acceptDialogs) await dialog.accept(); else await dialog.dismiss();
  });
  await page.locator('[data-conversation-branch="feature-x"]').click();
  await page.locator("[data-project-preview]").waitFor();
  await page.locator("[data-project-preview-continue]").click();
  await page.waitForTimeout(120);
  check("continue asks before returning to the branch project state and cancel keeps preview", dialogMessages.some(message => message.includes("回到该对话对应的工程状态")) && report.invokes.length === 0 && await page.locator("[data-project-preview]").count() === 1);
  acceptDialogs = true;
  await page.locator("[data-project-preview-continue]").click();
  await page.locator("[data-project-preview-busy]").waitFor();
  check("continue shows loading and disables repeat clicks", await page.locator("[data-project-preview-continue]").isDisabled() && await page.locator("[data-project-preview-busy]").count() === 1);
  await page.waitForFunction(() => !document.querySelector("[data-project-preview]"));
  const continueInvoke = report.invokes.at(-1);
  check("continue invokes version.checkout for the previewed branch (production seam args)", continueInvoke?.tool === "version.checkout" && continueInvoke?.args?.branch === "feature-x" && continueInvoke?.args?.project_path === "C:/fixture/alpha.vit" && continueInvoke?.confirmed === true && continueInvoke?.source === "ask_vit_webui_conversation" && continueInvoke?.context?.active_branch === "main");
  await page.waitForFunction(() => document.querySelector('[data-conversation-branch="feature-x"][data-conversation-branch-active="true"]'));
  check("continue refreshes the tree and highlights the continued conversation", await page.locator('[data-conversation-branch="feature-x"][data-conversation-branch-active="true"]').count() === 1 && await page.locator("[data-project-preview]").count() === 0 && !await page.locator("[data-project-composer] input").isDisabled() && await page.locator("[data-conversation-branch]").count() === 2);

  // 【新对话】：version.branch_create{name, from_node_id=当前活动节点, commit_id}；
  // 回执含 checked_out_to → 不再补 checkout；成功后刷新并进入新分支。
  const invokesBeforeNew = report.invokes.length;
  await page.locator("[data-conversation-new]").click();
  await page.waitForFunction(() => [...document.querySelectorAll("[data-conversation-branch]")].some(element => element.getAttribute("data-conversation-branch-active") === "true" && /^chat-/.test(element.getAttribute("data-conversation-branch") || "")));
  const newConversationInvokes = report.invokes.slice(invokesBeforeNew);
  check("new conversation branches from the active node through version.branch_create", newConversationInvokes.length === 1 && newConversationInvokes[0]?.tool === "version.branch_create" && /^chat-\d{8}-\d{6}$/.test(newConversationInvokes[0]?.args?.name || "") && newConversationInvokes[0]?.args?.from_node_id === "n-fx-vit" && newConversationInvokes[0]?.args?.commit_id === "f2" && newConversationInvokes[0]?.args?.project_path === "C:/fixture/alpha.vit" && newConversationInvokes[0]?.confirmed === true);
  check("new conversation becomes the active stream titled as a fresh conversation", (await page.locator('[data-conversation-branch-active="true"] [data-conversation-branch-title]').innerText()).trim() === "新对话" && await page.locator("[data-conversation-branch]").count() === 3);

  // checkout_blocked（agent Turn 运行中）：新建/继续禁用并给出原因；解除后恢复。
  // 触发刷新=打开既有 settings dialog（App 内既有 fetchUIState 路径，无新增协议）。
  conversation.checkoutBlocked = true;
  await page.locator("[data-entry-rail-settings]").click();
  await page.locator("[data-entry-settings][open]").waitFor();
  // App 的 settings 刷新只在对话进行期（entrySettingsOpen=true）内生效：等状态落到
  // 左栏再关 dialog（关得太快会按既有语义丢弃在飞刷新）。
  await page.waitForFunction(() => Boolean(document.querySelector("[data-conversation-blocked]")));
  await page.keyboard.press("Escape");
  check("checkout_blocked disables conversation switching with a reason", await page.locator("[data-conversation-new]").isDisabled() && (await page.locator("[data-conversation-blocked]").innerText()).includes("Stop Turn"));
  await page.locator('[data-conversation-branch="main"]').click();
  await page.locator("[data-project-preview]").waitFor();
  check("checkout_blocked disables continue and explains the guard", await page.locator("[data-project-preview-continue]").isDisabled() && await page.locator("[data-project-preview-blocked]").count() === 1 && await page.locator("[data-project-preview-error]").count() === 0);
  await page.locator("[data-project-preview-return]").click();
  conversation.checkoutBlocked = false;
  await page.locator("[data-entry-rail-settings]").click();
  await page.locator("[data-entry-settings][open]").waitFor();
  await page.waitForFunction(() => !document.querySelector("[data-conversation-blocked]"));
  await page.keyboard.press("Escape");
  check("checkout_blocked clears after the agent turn stops", await page.locator("[data-conversation-new]").isEnabled());
  await settleMotion();
  const newTextContrast = await page.locator("[data-native-focus-hint], [data-window-launch-feedback] h3, [data-window-launch-feedback] p").evaluateAll(elements => {
    const rgb = color => (color.match(/[\d.]+/g) || []).map(Number);
    const luminance = values => values.slice(0, 3).map(value => { const s = value / 255; return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4; }).reduce((sum, value, index) => sum + value * [0.2126, 0.7152, 0.0722][index], 0);
    return elements.map(element => {
      const style = getComputedStyle(element);
      let parent = element, background = [255, 255, 255];
      while (parent) { const value = rgb(getComputedStyle(parent).backgroundColor); if (value.length === 3 || value[3] === 1) { background = value; break; } parent = parent.parentElement; }
      const a = luminance(rgb(style.color)), b = luminance(background);
      return { text: element.textContent, ratio: (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05), fontPx: Number.parseFloat(style.fontSize) };
    });
  });
  report.newTextContrast = newTextContrast;
  check("new header and ready feedback text stays readable", newTextContrast.length >= 3 && newTextContrast.every(row => row.ratio >= 4.5 && row.fontPx >= 13));
  await page.screenshot({ path: resolve(runRoot, "project-ready-daw-foreground.png") });
  for (const [index, foreground] of ["other", "none", "shell"].entries()) {
    await emit({ type: "vit_window_state", window_state: observation(foreground, 40 + index) });
    await page.waitForFunction(() => document.querySelectorAll("[data-native-focus-hint]").length === 0);
    check(foreground + " is neutral", await page.locator("[data-native-focus-hint]").count() === 0);
  }
  await emit({ type: "vit_window_state", window_state: null });
  check("absent native state returns unknown", await page.locator("[data-project-root]").getAttribute("data-native-focus") === "unknown");
  const composer = page.locator("[data-project-composer] input");
  await composer.fill("保留的工程输入（render fixture）");
  // SHELL-ENTRY-RAIL-1（417）：收展状态跨 surface 保持——工程面收起后切回启动页，
  // 同一条 rail 节点仍是收起态；再由左栏收起态浮动把手展开，恢复工程列表。
  await page.locator("[data-window-sidebar-toggle]").click();
  await page.waitForFunction(() => document.querySelector("[data-entry-rail]").getBoundingClientRect().width < 1);
  check("project surface shares the same collapsible rail", Math.round(await railWidth()) === 0);
  await page.waitForFunction(() => window.localStorage.getItem("vit_entry_rail_collapsed") === "true");
  check("collapsed rail state is persisted to localStorage", await page.evaluate(() => window.localStorage.getItem("vit_entry_rail_collapsed")) === "true");
  // CONV-RAIL-1（418，B2a）：收展状态 localStorage 记忆——同源新页面重载后恢复收起态，
  // 且收起态可用既有浮动把手展开（不把左栏做成不可逆收起）。
  const persistedPage = await context.newPage();
  await persistedPage.goto(origin + "/app/?surface=launcher");
  await persistedPage.locator("[data-entry-rail]").waitFor();
  await persistedPage.waitForFunction(() => Math.round(document.querySelector("[data-entry-rail]").getBoundingClientRect().width) === 0);
  check("reloaded App restores the collapsed rail from localStorage", await persistedPage.locator("[data-entry-rail]").getAttribute("data-entry-rail-collapsed") === "true" && await persistedPage.locator("[data-entry-rail-expand]").count() === 1);
  await persistedPage.locator("[data-entry-rail-expand]").click();
  await persistedPage.waitForFunction(() => Math.round(document.querySelector("[data-entry-rail]").getBoundingClientRect().width) === 280);
  check("restored collapsed rail expands through the floating handle", await persistedPage.locator("[data-entry-rail]").getAttribute("data-entry-rail-collapsed") === "false");
  await persistedPage.close();
  await page.locator("[data-project-view-launcher]").click();
  await page.locator('[data-launcher-mode="project-management"]').waitFor();
  check("collapsed rail state survives the project→launcher return", await page.evaluate(node => node === document.querySelector("[data-entry-rail]"), railIdentity)
    && await page.locator("[data-window-sidebar-toggle]").getAttribute("aria-expanded") === "false"
    && Math.round(await railWidth()) === 0);
  await page.locator("[data-entry-rail-expand]").click();
  await page.waitForFunction(() => Math.round(document.querySelector("[data-entry-rail]").getBoundingClientRect().width) === 280);
  check("expanding from the collapsed handle restores the startup project list", await page.evaluate(node => node === document.querySelector("[data-entry-rail]"), railIdentity)
    && await page.locator("[data-entry-rail] [data-launcher-projects]").count() === 1
    && await page.locator("[data-conversation-rail]").count() === 0);
  check("management sidebar preference survives remount and contains no invented chat list", await page.locator("[data-window-sidebar-toggle]").getAttribute("aria-expanded") === "true" && await page.locator("[data-window-sidebar-slot]").getAttribute("data-window-sidebar-slot") === "recents");
  await page.locator("[data-launcher-choose-file]").click();
  const fileRequest = await page.evaluate(() => window.__hostPosts.filter(p => p.type === "launcher_choose_file").at(-1).request_id);
  await postResult(fileRequest, "failed", "文件选择失败（render fixture）");
  await feedback().locator('[data-loading-phase="failed"]').waitFor();
  check("file-dialog failure never retries an unrelated previous directive", await feedback().locator("[data-loading-retry]").count() === 0);
  await page.locator("[data-entry-rail-settings]").click();
  await page.locator("[data-entry-settings][open]").waitFor();
  await page.keyboard.press("Escape");
  await page.locator("[data-launcher-return-project]").click();
  check("management/settings roundtrip preserves mounted project input", await composer.inputValue() === "保留的工程输入（render fixture）");
  await composer.fill("新建工程");
  await page.locator("[data-project-composer] button").click();
  await page.waitForFunction(old => window.__hostPosts.filter(p => p.type === "launcher_directive").at(-1)?.request_id !== old, request);
  request = await currentRequest();
  await progress(request, 11, "spawning");
  check("in-project directive uses same production feedback and gates input", await composer.isDisabled());
  await progress(request, 12, "failed", { detail: "工作台超时（render fixture）" });
  await postResult(request, "failed", "工作台超时（render fixture）");
  await feedback().locator('[data-loading-phase="failed"]').waitFor();
  check("project timeout never completes", !await composer.isDisabled() && await feedback().locator('[data-loading-phase="completed"]').count() === 0);
  await page.emulateMedia({ reducedMotion: "reduce" });
  check("reduced motion disables receipt and busy step motion", await feedback().evaluate(el => getComputedStyle(el).animationName === "none" && [...el.querySelectorAll("*")].every(child => getComputedStyle(child).animationName === "none")));
  await page.setViewportSize({ width: 1024, height: 680 });
  check("native minimum viewport has no horizontal overflow", await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
  await page.screenshot({ path: resolve(runRoot, "project-failed-reduced-motion.png") });
  check("no external or live-stack network and no page exceptions", report.requests.every(r => !r.external) && report.consoleErrors.length === 0);

  const bare = await browser.newContext({ viewport: { width: 1200, height: 800 } });
  await bare.route("**/agent/**", route => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ status: "ok", reply: "fixture no host", project_result_cards: [] }) }));
  const barePage = await bare.newPage();
  await barePage.goto(origin + "/app/?surface=launcher");
  await barePage.locator("[data-launcher-root]").waitFor();
  check("browser without host stays unknown and starts no launch feedback", await barePage.locator("[data-launcher-root]").getAttribute("data-native-focus") === "unknown" && await barePage.locator("[data-window-launch-feedback]").count() === 0);
  await bare.close();

  const latePage = await context.newPage();
  await latePage.addInitScript(() => {
    const post = window.chrome.webview.postMessage;
    window.chrome.webview.postMessage = value => {
      post(value);
      if (value.type !== "window_presentation_ready") return;
      window.__emitHost({ type: "vit_window_state", window_state: null, replay: true, subscription_id: value.subscription_id });
      window.__emitHost({ type: "vit_launch_progress", schema: "vitshell.launch_progress.v1", request_id: "already-running-fixture", seq: 50,
        phase: "waiting_launch_result", detail: "", replay: true, subscription_id: value.subscription_id });
    };
  });
  await latePage.goto(origin + "/app/?surface=launcher");
  await latePage.locator('[data-window-launch-request="already-running-fixture"]').waitFor();
  check("listener-before-handshake restores late subscriber without inventing earlier steps", await latePage.locator("[data-loading-step-status]").count() === 1
    && await latePage.evaluate(() => window.__hostPosts.filter(value => value.type === "launcher_directive").length) === 0);
  await latePage.evaluate(() => window.__emitHost({ type: "vit_launch_progress", schema: "vitshell.launch_progress.v1", request_id: "already-running-fixture", seq: 51, phase: "opened", detail: "", stage: "future_unknown" }));
  await latePage.locator("[data-window-launch-limited]").waitFor();
  check("unknown terminal stage remains unconfirmed in actual App", await latePage.locator("[data-window-launch-feedback]").getAttribute("data-window-launch-terminal") === "unconfirmed");
  await latePage.close();
} catch (error) {
  report.failure = String(error?.stack || error);
  process.exitCode = 1;
} finally {
  writeFileSync(resolve(runRoot, "report.json"), JSON.stringify(report, null, 2));
  await browser?.close();
  await server?.close();
  console.log(JSON.stringify({ runRoot, checks: report.checks.length, failed: report.checks.filter(c => !c.pass), ringSpin: report.ringSpin, error: report.failure }, null, 2));
}

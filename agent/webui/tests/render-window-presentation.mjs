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
const conversation = {
  activeBranch: "main",
  checkoutBlocked: false,
  createdBranches: [],
  noActiveNode: false,
  // CONV-RAIL-CROSS-1（421，B2b）mock 状态：其它工作树懒加载/组合缝动作的回执开关。
  activeWorktree: "",
  openedBetaNode: "",
  statusFailGamma: false,
  crossFail: false
};
const ROOT_PROJECT_PATH = "C:/fixture/alpha.vit";
const BETA_PROJECT_PATH = "C:/fixture/worktrees/wt-beta/alpha_wt-beta.vit";
const GAMMA_PROJECT_PATH = "C:/fixture/worktrees/wt-gamma/alpha_wt-gamma.vit";
const worktreeRows = () => ([
  { name: "wt-beta", path: "C:/fixture/worktrees/wt-beta", project_file_path: BETA_PROJECT_PATH, commit_id: "w1", origin_commit_id: "w1", worktree_head_commit_id: "w1" },
  { name: "wt-gamma", path: "C:/fixture/worktrees/wt-gamma", project_file_path: GAMMA_PROJECT_PATH, commit_id: "g1", origin_commit_id: "g1", worktree_head_commit_id: "g1" }
]);
const conversationNode = (id, kind, text, branch, commitID, parentNodeID, createdAt) => ({ id, kind, text, text_preview: text, branch, commit_id: commitID, parent_node_id: parentNodeID, created_at: createdAt });
// 其它工作树 version.status 投影：wt-beta（active_branch=shared，head=w1；shared ref head
// 与 worktree head 不同 → 只有 active_branch 规则能归属它）。
const betaStatusHistory = () => ({
  project_path: BETA_PROJECT_PATH,
  root_project_path: ROOT_PROJECT_PATH,
  active_branch: "shared",
  active_node_id: "n-beta-shared-vit",
  active_worktree: "wt-beta",
  detached: false,
  head: "w1",
  branches: { main: "b1", "beta-topic": "w1", shared: "s1" },
  worktrees: [],
  conversation_graph: {
    project_path: BETA_PROJECT_PATH,
    active_branch: "shared",
    active_node_id: "n-beta-shared-vit",
    nodes: [
      conversationNode("n-beta-topic-ask", "ask", "在这个工作树里写新鼓组", "beta-topic", "w0", "", "2026-11-18T20:00:00"),
      conversationNode("n-beta-topic-vit", "vit", "鼓组片段已写入工作树副本。", "beta-topic", "w1", "n-beta-topic-ask", "2026-11-18T20:01:00"),
      conversationNode("n-beta-shared-ask", "ask", "把这段鼓组再压紧一点", "shared", "s0", "", "2026-11-18T20:02:00"),
      conversationNode("n-beta-shared-vit", "vit", "已压紧。", "shared", "s1", "n-beta-shared-ask", "2026-11-18T20:03:00")
    ]
  }
});
// wt-gamma：首个 version.status 故意失败（重试成功后返回该投影）。
const gammaStatusHistory = () => ({
  project_path: GAMMA_PROJECT_PATH,
  root_project_path: ROOT_PROJECT_PATH,
  active_branch: "gamma-topic",
  active_node_id: "n-gamma-vit",
  active_worktree: "wt-gamma",
  detached: false,
  head: "g1",
  branches: { main: "g2", "gamma-topic": "g1" },
  worktrees: [],
  conversation_graph: {
    project_path: GAMMA_PROJECT_PATH,
    active_branch: "gamma-topic",
    active_node_id: "n-gamma-vit",
    nodes: [
      conversationNode("n-gamma-ask", "ask", "给 gamma 工作树加一条混响", "gamma-topic", "g0", "", "2026-11-18T20:10:00"),
      conversationNode("n-gamma-vit", "vit", "混响已加到 gamma 副本。", "gamma-topic", "g1", "n-gamma-ask", "2026-11-18T20:11:00")
    ]
  }
});
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
  // repo 级分支 refs：shared（在 beta 为 active）/beta-topic（head=beta head）/orphan
  //（无任何归属）用于 421 分支归属过滤断言；对话节点仍只在本工作树 history 里。
  const branches = { main: "c4", "feature-x": "f2", shared: "s1", "beta-topic": "w1", orphan: "z9" };
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
  // CONV-RAIL-CROSS-1（421）：conversation_open 成功后当前工程切到 wt-beta 副本——
  // ui state 变为该工作树 history + active_worktree=wt-beta，左栏当前组随之切换。
  if (conversation.activeWorktree === "wt-beta") {
    const beta = betaStatusHistory();
    return {
      ...beta,
      project_path: BETA_PROJECT_PATH,
      root_project_path: ROOT_PROJECT_PATH,
      active_branch: conversation.activeBranch || beta.active_branch,
      active_node_id: conversation.openedBetaNode || beta.active_node_id,
      active_worktree: "wt-beta",
      worktrees: worktreeRows()
    };
  }
  // SHELL-PROJECT-STREAM-FIX-1（427）：无活动节点但工程身份存在（project_path/branch/head）
  // 的工程态 fixture——用于证明【＋ 新对话】不再灰、branch_create 走当前 HEAD 分叉。
  if (conversation.noActiveNode) {
    return {
      project_path: "C:/fixture/alpha.vit",
      root_project_path: "C:/fixture/alpha.vit",
      active_branch: conversation.activeBranch,
      active_node_id: "",
      active_worktree: "",
      detached: false,
      head: "c4",
      branches,
      worktrees: worktreeRows(),
      conversation_graph: {
        project_path: "C:/fixture/alpha.vit",
        active_branch: conversation.activeBranch,
        active_node_id: "",
        nodes: []
      }
    };
  }
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
    worktrees: worktreeRows(),
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
        } else if (payload.tool === "version.status") {
          // 421：其它工作树展开时的只读懒加载（不 checkout/不 open_project/不 reload）。
          const projectPath = payload.args?.project_path || "";
          if (projectPath === BETA_PROJECT_PATH) {
            body = { status: "ok", result: betaStatusHistory() };
          } else if (projectPath === GAMMA_PROJECT_PATH) {
            body = conversation.statusFailGamma
              ? { status: "error", error: "fixture status unavailable (wt-gamma)" }
              : { status: "ok", result: gammaStatusHistory() };
          } else {
            body = { status: "ok", result: conversationHistory() };
          }
        } else if (payload.tool === "version.conversation_open") {
          // 421：组合缝回执。失败路径如实披露 target_materialized/branch_warning；
          // 成功路径把 ui state 切到目标工作树（active_worktree=wt-beta + 目标分支/节点）。
          const targetPath = payload.args?.project_file_path || "";
          if (conversation.crossFail) {
            body = {
              status: "error",
              error: "kernel project reload failed after materializing the target worktree copy: fixture reload",
              result: { status: "error", target_materialized: true, branch_warning: "fixture branch warning", kernel_reloaded: false, project_file_path: targetPath }
            };
          } else {
            conversation.activeWorktree = "wt-beta";
            conversation.activeBranch = payload.args?.branch || "shared";
            conversation.openedBetaNode = payload.args?.node_id || "";
            body = {
              status: "ok",
              result: {
                status: "ok",
                target_materialized: true,
                same_worktree: false,
                kernel_reloaded: true,
                branch: conversation.activeBranch,
                active_branch: conversation.activeBranch,
                active_worktree: "wt-beta",
                active_node_id: conversation.openedBetaNode,
                project_file_path: targetPath
              }
            };
          }
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
  // CONV-WINDOW-SHELL-1（428，A 方案）：右侧常驻窗口壳 + 流层节点身份（后续跨 surface 比对）。
  const windowShellIdentity = await page.evaluateHandle(() => document.querySelector("[data-conversation-window]"));
  check("conversation window shell hosts the live launcher stream layer", await page.locator("[data-conversation-window]").count() === 1 && await page.locator('[data-conversation-layer="launcher"][data-conversation-layer-active="true"]').evaluate(layer => !layer.hidden && !layer.inert && layer.getAttribute("aria-hidden") === "false"));
  // CONV-WINDOW-MOTION-2（430）：动效值抽成 CSS 自定义属性（--v5-conv-motion-layer/rail/content
  // + ease，一处可调）；左栏内容 800ms（431 试验值）、launcher 内容体 420ms；右侧 surface 层 480ms 且
  // 延迟 40ms 起（与左栏错峰，避免两侧同时动）。
  check("conversation motion values are tokenized in CSS custom properties", await page.evaluate(() => {
    const style = getComputedStyle(document.documentElement);
    const normalize = value => (value || "").replace(/\s+/g, "");
    return style.getPropertyValue("--v5-conv-motion-layer").trim() === "480ms"
      && style.getPropertyValue("--v5-conv-motion-rail").trim() === "800ms"
      && style.getPropertyValue("--v5-conv-motion-content").trim() === "420ms"
      && normalize(style.getPropertyValue("--v5-conv-motion-ease")).includes("cubic-bezier(0.05,0.7,0.1,1)");
  }));
  check("entry rail content animates 800ms with the emphasized ease when the surface content swaps", await page.locator("[data-entry-rail-slot] > .v5-launcher-drawer").evaluate(drawer => {
    const style = getComputedStyle(drawer);
    const durationMs = style.animationDuration.endsWith("ms") ? Number.parseFloat(style.animationDuration) : Number.parseFloat(style.animationDuration) * 1000;
    return style.animationName === "v5-entry-rail-content-enter" && Math.abs(durationMs - 800) <= 1 && style.animationTimingFunction.replace(/\s+/g, "") === "cubic-bezier(0.05,0.7,0.1,1)";
  }));
  check("right surface layer starts 40ms after the rail content (staggered, not simultaneous)", await page.evaluate(() => {
    const layer = document.querySelector('[data-conversation-layer][data-conversation-layer-active="true"]');
    const railContent = document.querySelector("[data-entry-rail-slot] > .v5-launcher-drawer");
    if (!layer || !railContent) return false;
    const delayMs = style => (style.animationDelay.endsWith("ms") ? Number.parseFloat(style.animationDelay) : Number.parseFloat(style.animationDelay) * 1000);
    return Math.abs(delayMs(getComputedStyle(layer)) - 40) <= 1 && delayMs(getComputedStyle(railContent)) === 0;
  }));
  check("launcher stream content animates only the content body (420ms, titlebar/shell stay static)", await page.evaluate(() => {
    const layer = document.querySelector('[data-conversation-layer="launcher"]');
    const main = layer?.querySelector(".v5-launcher-main");
    const item = layer?.querySelector(".v5-launcher-thread > *");
    const titlebar = layer?.querySelector(".v5-launcher-titlebar");
    const shell = document.querySelector("[data-conversation-window]");
    if (!main || !item || !titlebar || !shell) return false;
    const durationMs = style => (style.animationDuration.endsWith("ms") ? Number.parseFloat(style.animationDuration) : Number.parseFloat(style.animationDuration) * 1000);
    const mainStyle = getComputedStyle(main);
    const itemStyle = getComputedStyle(item);
    return Math.abs(durationMs(mainStyle) - 420) <= 1
      && Math.abs(durationMs(itemStyle) - 420) <= 1
      && mainStyle.animationTimingFunction.replace(/\s+/g, "") === "cubic-bezier(0.05,0.7,0.1,1)"
      && getComputedStyle(titlebar).animationName === "none"
      && getComputedStyle(shell).animationName === "none";
  }));
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
  // SHELL-PROJECT-STREAM-FIX-1（427）缺陷 B：工程加载卡是对话历史里的装载记录——DOM
  // 顺序必须排在消息之前（消息尚未出现时只需与消息容器同在、位于顶部）。
  check("project loading card is the first history record in the thread", await page.locator("[data-project-thread]").evaluate(thread => {
    const card = thread.querySelector("[data-window-launch-feedback]");
    if (!card) return false;
    const firstMessage = thread.querySelector("[data-message-author]");
    return !firstMessage || Boolean(card.compareDocumentPosition(firstMessage) & Node.DOCUMENT_POSITION_FOLLOWING);
  }));
  await emit({ type: "vit_window_state", window_state: observation("daw", 30) });
  await page.locator("[data-project-titlebar] [data-native-focus-hint]").waitFor();
  check("observed DAW foreground changes only header feedback", await page.locator("[data-project-root]").getAttribute("data-native-focus") === "daw" && await feedback().getAttribute("data-window-launch-terminal") === "ready");
  check("same rail DOM node survives the launcher→project surface switch", await page.evaluate(node => node === document.querySelector("[data-entry-rail]"), railIdentity));
  check("conversation window shell survives launcher→project as the same DOM node", await page.evaluate(node => node === document.querySelector("[data-conversation-window]"), windowShellIdentity) && await page.locator('[data-conversation-layer="project"][data-conversation-layer-active="true"]').evaluate(layer => !layer.hidden && !layer.inert && layer.getAttribute("aria-hidden") === "false"));
  const projectLayerIdentity = await page.evaluateHandle(() => document.querySelector('[data-conversation-layer="project"]'));
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
  check("new conversation branches from the active node through version.branch_create", newConversationInvokes.length === 1 && newConversationInvokes[0]?.tool === "version.branch_create" && /^chat-\d{8}-\d{6}(-\d+)?$/.test(newConversationInvokes[0]?.args?.name || "") && newConversationInvokes[0]?.args?.from_node_id === "n-fx-vit" && newConversationInvokes[0]?.args?.commit_id === "f2" && newConversationInvokes[0]?.args?.project_path === "C:/fixture/alpha.vit" && newConversationInvokes[0]?.confirmed === true);
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

  // SHELL-PROJECT-STREAM-FIX-1（427）缺陷 A：无活动节点但工程身份存在时【＋ 新对话】
  // 仍可用（不再灰）；version.branch_create 工具字段只带 name，由当前 HEAD 分叉。
  // 「有活动节点时仍带 from_node_id+commit_id」由上文 "new conversation branches from
  // the active node through version.branch_create" 断言覆盖。
  const waitForInvokeCount = async (target, timeoutMs = 4000) => {
    const started = Date.now();
    while (report.invokes.length < target) {
      if (Date.now() - started > timeoutMs) throw new Error("invoke count timeout: expected " + target + " got " + report.invokes.length);
      await page.waitForTimeout(25);
    }
  };
  conversation.noActiveNode = true;
  await page.locator("[data-entry-rail-settings]").click();
  await page.locator("[data-entry-settings][open]").waitFor();
  await page.waitForFunction(() => {
    const button = document.querySelector("[data-conversation-new]");
    return Boolean(button) && !button.disabled && (button.getAttribute("title") || "").includes("从当前工程状态");
  });
  await page.keyboard.press("Escape");
  check("new conversation stays enabled without an active node when project identity exists", await page.locator("[data-conversation-new]").isEnabled() && (await page.locator("[data-conversation-new]").getAttribute("title") || "").includes("从当前工程状态新建对话"));
  const invokesBeforeHeadFallback = report.invokes.length;
  await page.locator("[data-conversation-new]").click();
  await waitForInvokeCount(invokesBeforeHeadFallback + 1);
  const headFallbackInvoke = report.invokes.at(-1);
  check("new conversation without an active node branches from the current HEAD (name only)", headFallbackInvoke?.tool === "version.branch_create" && /^chat-\d{8}-\d{6}(-\d+)?$/.test(headFallbackInvoke?.args?.name || "") && headFallbackInvoke?.args?.from_node_id === undefined && headFallbackInvoke?.args?.commit_id === undefined && headFallbackInvoke?.args?.project_path === "C:/fixture/alpha.vit");
  conversation.noActiveNode = false;
  await page.locator("[data-entry-rail-settings]").click();
  await page.locator("[data-entry-settings][open]").waitFor();
  await page.waitForFunction(() => {
    const button = document.querySelector("[data-conversation-new]");
    return Boolean(button) && !button.disabled && (button.getAttribute("title") || "").includes("从当前对话的活动节点");
  });
  await page.keyboard.press("Escape");
  check("new conversation re-enables the active-node hint once a node exists again", await page.locator("[data-conversation-new]").isEnabled() && (await page.locator("[data-conversation-new]").getAttribute("title") || "").includes("从当前对话的活动节点"));

  // ===== CONV-RAIL-CROSS-1（421，B2b）：跨工作树只读浏览 + 组合缝动作 =====
  const betaKey = BETA_PROJECT_PATH.toLowerCase();
  const gammaKey = GAMMA_PROJECT_PATH.toLowerCase();
  const betaGroupSelector = '[data-conversation-worktree-group-key="' + betaKey + '"]';
  const gammaGroupSelector = '[data-conversation-worktree-group-key="' + gammaKey + '"]';
  const writeTools = ["version.checkout", "version.conversation_open", "version.node_checkout", "version.worktree_checkout"];
  const writeInvokeCount = () => report.invokes.filter(invoke => writeTools.includes(invoke.tool)).length;
  const statusInvokeCount = () => report.invokes.filter(invoke => invoke.tool === "version.status").length;
  const groupStatus = selector => page.locator(selector).getAttribute("data-conversation-worktree-group-status");
  check("other worktrees render as collapsed groups with name, path and head", await page.locator("[data-conversation-worktree-group]").count() === 2
    && await page.locator(betaGroupSelector).getAttribute("data-conversation-worktree-group-expanded") === "false"
    && await groupStatus(betaGroupSelector) === "idle"
    && (await page.locator(betaGroupSelector + " [data-conversation-worktree-group-label]").innerText()).trim() === "wt-beta"
    && (await page.locator(betaGroupSelector + " [data-conversation-worktree-group-path]").innerText()).includes("wt-beta")
    && (await page.locator(betaGroupSelector + " [data-conversation-worktree-group-branch]").innerText()).includes("head w1"));
  check("unattributed repo branches stay read-only in the workspace other-branches group", await page.locator("[data-conversation-other-branch]").count() === 2
    && await page.locator('[data-conversation-other-branch="shared"]').count() === 1
    && await page.locator('[data-conversation-other-branch="orphan"]').count() === 1
    && await page.locator('[data-conversation-branch="shared"]').count() === 0);
  const statusBeforeBeta = statusInvokeCount();
  await page.locator(betaGroupSelector + " [data-conversation-worktree-toggle]").click();
  await page.locator(betaGroupSelector + " [data-conversation-worktree-loading]").waitFor();
  check("expanding another worktree shows a loading state", await page.locator(betaGroupSelector + " [data-conversation-worktree-loading]").count() === 1 && await groupStatus(betaGroupSelector) === "loading");
  await page.waitForFunction(selector => document.querySelector(selector)?.getAttribute("data-conversation-worktree-group-status") === "ready", betaGroupSelector);
  const betaStatusInvoke = report.invokes[report.invokes.length - 1];
  check("expanding a worktree lazy-loads version.status exactly once (read-only args)", statusInvokeCount() === statusBeforeBeta + 1 && betaStatusInvoke?.tool === "version.status" && betaStatusInvoke?.args?.project_path === BETA_PROJECT_PATH && betaStatusInvoke?.confirmed === false && betaStatusInvoke?.source === "ask_vit_webui_conversation");
  check("loaded worktree branches render under its group and the shared branch moves there", await page.locator(betaGroupSelector + " [data-conversation-branch]").count() === 2
    && await page.locator(betaGroupSelector + ' [data-conversation-branch="shared"]').count() === 1
    && await page.locator(betaGroupSelector + ' [data-conversation-branch="beta-topic"]').count() === 1
    && await page.locator('[data-conversation-other-branch="shared"]').count() === 0
    && await page.locator('[data-conversation-other-branch="orphan"]').count() === 1
    && await page.locator('[data-conversation-worktree][data-conversation-worktree-active="true"] [data-conversation-branch="shared"]').count() === 0);
  check("every conversation branch appears in exactly one worktree group", await page.locator("[data-conversation-branch]").evaluateAll(elements => {
    const names = elements.map(element => element.getAttribute("data-conversation-branch"));
    return names.length > 0 && new Set(names).size === names.length;
  }));
  const statusAfterFirstLoad = statusInvokeCount();
  await page.locator(betaGroupSelector + " [data-conversation-worktree-toggle]").click();
  await page.waitForFunction(selector => document.querySelector(selector)?.getAttribute("data-conversation-worktree-group-expanded") === "false", betaGroupSelector);
  await page.locator(betaGroupSelector + " [data-conversation-worktree-toggle]").click();
  await page.waitForFunction(selector => document.querySelector(selector)?.getAttribute("data-conversation-worktree-group-status") === "ready", betaGroupSelector);
  check("collapsing and re-expanding a worktree does not refetch its status", statusInvokeCount() === statusAfterFirstLoad);
  await page.locator(betaGroupSelector + " [data-conversation-worktree-refresh]").click();
  await page.waitForFunction(selector => document.querySelector(selector)?.getAttribute("data-conversation-worktree-group-status") === "loading", betaGroupSelector);
  await page.waitForFunction(selector => document.querySelector(selector)?.getAttribute("data-conversation-worktree-group-status") === "ready", betaGroupSelector);
  check("manual refresh explicitly reloads the worktree status", statusInvokeCount() === statusAfterFirstLoad + 1);
  // 状态拉取失败 → 组内「无法读取」+ 重试；重试成功后分支出现。
  conversation.statusFailGamma = true;
  const statusBeforeGamma = statusInvokeCount();
  await page.locator(gammaGroupSelector + " [data-conversation-worktree-toggle]").click();
  await page.waitForFunction(selector => document.querySelector(selector)?.getAttribute("data-conversation-worktree-group-status") === "error", gammaGroupSelector);
  check("status load failure shows an unreadable state with retry", (await page.locator(gammaGroupSelector + " [data-conversation-worktree-error]").innerText()).includes("无法读取") && await page.locator(gammaGroupSelector + " [data-conversation-worktree-retry]").count() === 1);
  conversation.statusFailGamma = false;
  await page.locator(gammaGroupSelector + " [data-conversation-worktree-retry]").click();
  await page.waitForFunction(selector => document.querySelector(selector)?.getAttribute("data-conversation-worktree-group-status") === "ready", gammaGroupSelector);
  check("retry after a status failure loads the worktree branches", await page.locator(gammaGroupSelector + ' [data-conversation-branch="gamma-topic"]').count() === 1 && statusInvokeCount() === statusBeforeGamma + 2);
  // 只读预览：其它工作树分支 → 消息 + 蒙版 + 【打开此工作树并继续此对话】；零写调用。
  const writesBeforeBrowsing = writeInvokeCount();
  await page.locator(betaGroupSelector + ' [data-conversation-branch="beta-topic"]').click();
  await page.locator("[data-project-preview]").waitFor();
  const crossPreviewMessages = await page.locator("[data-project-preview-thread] [data-message-author]").evaluateAll(elements => elements.map(element => element.innerText.trim()));
  check("browsing another worktree branch is read-only and triggers no write tool", await page.locator("[data-project-preview]").getAttribute("data-project-preview-kind") === "cross"
    && await page.locator("[data-project-preview]").getAttribute("data-project-preview-worktree") === "wt-beta"
    && crossPreviewMessages.length === 2 && crossPreviewMessages[0].includes("在这个工作树里写新鼓组")
    && await page.locator("[data-project-composer-mask]").count() === 1 && await page.locator("[data-project-composer] input").isDisabled()
    && writeInvokeCount() === writesBeforeBrowsing);
  check("cross-worktree preview offers the open-worktree continue action", (await page.locator("[data-project-preview-continue]").innerText()).includes("打开此工作树并继续此对话") && await page.locator("[data-project-preview-continue]").getAttribute("data-project-preview-action") === "cross");
  // checkout_blocked：跨工作树继续禁用并说明；解除后恢复。
  conversation.checkoutBlocked = true;
  await page.locator("[data-entry-rail-settings]").click();
  await page.locator("[data-entry-settings][open]").waitFor();
  await page.waitForFunction(() => Boolean(document.querySelector("[data-conversation-blocked]")));
  await page.keyboard.press("Escape");
  check("checkout_blocked disables the cross-worktree continue action with a reason", await page.locator("[data-project-preview-continue]").isDisabled() && await page.locator("[data-project-preview-blocked]").count() === 1 && await page.locator("[data-conversation-new]").isDisabled());
  conversation.checkoutBlocked = false;
  await page.locator("[data-entry-rail-settings]").click();
  await page.locator("[data-entry-settings][open]").waitFor();
  await page.waitForFunction(() => !document.querySelector("[data-conversation-blocked]"));
  await page.keyboard.press("Escape");
  check("cross-worktree continue re-enables after the turn stops", await page.locator("[data-project-preview-continue]").isEnabled());
  // 跨工作树继续：confirm 文案 → version.conversation_open 实参 → 失败保留预览+披露 → 重试成功刷新高亮。
  const dialogsBeforeCross = dialogMessages.length;
  conversation.crossFail = true;
  const openCallsBefore = report.invokes.filter(invoke => invoke.tool === "version.conversation_open").length;
  await page.locator("[data-project-preview-continue]").click();
  await page.waitForFunction(() => Boolean(document.querySelector("[data-project-preview-error]")));
  const crossFailureText = await page.locator("[data-project-preview-error]").innerText();
  check("cross-worktree continue confirms opening the worktree and its project state", dialogMessages.slice(dialogsBeforeCross).some(message => message.includes("打开工作树「wt-beta」并继续此对话") && message.includes("工程状态")));
  check("failed cross-worktree continue keeps the preview and discloses target_materialized", crossFailureText.includes("kernel project reload failed") && crossFailureText.includes("目标副本已物化：是") && crossFailureText.includes("分支提示") && await page.locator("[data-project-preview]").count() === 1 && await page.locator("[data-project-preview-continue]").isEnabled() && report.invokes.filter(invoke => invoke.tool === "version.conversation_open").length === openCallsBefore + 1);
  conversation.crossFail = false;
  await page.locator("[data-project-preview-continue]").click();
  await page.locator("[data-project-preview-busy]").waitFor();
  check("cross-worktree continue shows loading and disables repeat clicks", await page.locator("[data-project-preview-continue]").isDisabled() && (await page.locator("[data-project-preview-busy]").innerText()).includes("正在打开工作树并切换工程状态"));
  await page.waitForFunction(() => !document.querySelector("[data-project-preview]"));
  const successOpenInvoke = report.invokes.filter(invoke => invoke.tool === "version.conversation_open").at(-1);
  check("cross-worktree continue invokes version.conversation_open with worktree+branch+head node", successOpenInvoke?.args?.project_file_path === BETA_PROJECT_PATH && successOpenInvoke?.args?.branch === "beta-topic" && successOpenInvoke?.args?.node_id === "n-beta-topic-vit" && successOpenInvoke?.confirmed === true && successOpenInvoke?.source === "ask_vit_webui_conversation");
  await page.waitForFunction(() => {
    const label = document.querySelector('[data-conversation-worktree][data-conversation-worktree-active="true"] [data-conversation-worktree-label]');
    return Boolean(label) && (label.textContent || "").includes("wt-beta");
  });
  check("successful cross-worktree open switches the rail to the new active worktree and branch", (await page.locator('[data-conversation-worktree][data-conversation-worktree-active="true"] [data-conversation-worktree-label]').innerText()).trim() === "wt-beta" && await page.locator('[data-conversation-branch="beta-topic"][data-conversation-branch-active="true"]').count() === 1 && await page.locator("[data-project-preview]").count() === 0 && !await page.locator("[data-project-composer] input").isDisabled());
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
  // CONV-WINDOW-MOTION-2（430）：右侧流层 480ms + emphasized-decelerate（0.05,0.7,0.1,1）
  // 且延迟 40ms；工程 thread 内容体 420ms 只动内容体，窗口壳/titlebar/composer 静止；
  // 分支预览/返回只触发内容体过渡（线程节点身份不变，无 JS 定时器）。
  const EMPHASIZED_DECELERATE = "cubic-bezier(0.05,0.7,0.1,1)";
  check("active project layer crossfade is 480ms with the emphasized-decelerate curve and 40ms delay", await page.locator('[data-conversation-layer="project"]').evaluate(layer => {
    const style = getComputedStyle(layer);
    const durationMs = style.animationDuration.endsWith("ms") ? Number.parseFloat(style.animationDuration) : Number.parseFloat(style.animationDuration) * 1000;
    const delayMs = style.animationDelay.endsWith("ms") ? Number.parseFloat(style.animationDelay) : Number.parseFloat(style.animationDelay) * 1000;
    return style.animationName === "v5-conversation-layer-enter"
      && Math.abs(durationMs - 480) <= 1
      && style.animationTimingFunction.replace(/\s+/g, "") === "cubic-bezier(0.05,0.7,0.1,1)"
      && Math.abs(delayMs - 40) <= 1
      && style.animationFillMode === "backwards";
  }));
  check("project thread content transition is 420ms on the content body only", await page.evaluate(() => {
    const thread = document.querySelector("[data-project-thread]");
    const titlebar = document.querySelector("[data-project-titlebar]");
    const composerElement = document.querySelector("[data-project-composer]");
    const shell = document.querySelector("[data-conversation-window]");
    if (!thread || !titlebar || !composerElement || !shell) return false;
    const style = getComputedStyle(thread);
    const durationMs = style.animationDuration.endsWith("ms") ? Number.parseFloat(style.animationDuration) : Number.parseFloat(style.animationDuration) * 1000;
    return thread.getAttribute("data-project-thread-mode") === "live"
      && style.animationName === "v5-project-thread-live-enter"
      && Math.abs(durationMs - 420) <= 1
      && style.animationTimingFunction.replace(/\s+/g, "") === "cubic-bezier(0.05,0.7,0.1,1)"
      && getComputedStyle(titlebar).animationName === "none"
      && getComputedStyle(composerElement).animationName === "none"
      && getComputedStyle(shell).animationName === "none";
  }));
  check("entry rail conversation content also animates 800ms", await page.locator("[data-entry-rail-slot] > .v5-conversation-rail").evaluate(rail => {
    const style = getComputedStyle(rail);
    const durationMs = style.animationDuration.endsWith("ms") ? Number.parseFloat(style.animationDuration) : Number.parseFloat(style.animationDuration) * 1000;
    return style.animationName === "v5-entry-rail-content-enter" && Math.abs(durationMs - 800) <= 1;
  }));
  const projectThreadIdentity = await page.evaluateHandle(() => document.querySelector("[data-project-thread]"));
  // 当前活动分支是 beta-topic（421 跨工作树打开后），改用同工作树的非活动分支 shared
  // 触发只读预览（点当前分支只会清预览，不产生内容切换）。
  await page.locator('[data-conversation-branch="shared"]').click();
  await page.locator("[data-project-preview]").waitFor();
  check("branch preview animates only the thread content body", await page.evaluate(node => {
    const thread = document.querySelector("[data-project-thread]");
    const style = getComputedStyle(thread);
    const durationMs = style.animationDuration.endsWith("ms") ? Number.parseFloat(style.animationDuration) : Number.parseFloat(style.animationDuration) * 1000;
    return node === thread
      && thread.getAttribute("data-project-thread-mode") === "preview"
      && style.animationName === "v5-project-thread-preview-enter"
      && Math.abs(durationMs - 420) <= 1
      && style.animationTimingFunction.replace(/\s+/g, "") === "cubic-bezier(0.05,0.7,0.1,1)"
      && getComputedStyle(document.querySelector("[data-project-titlebar]")).animationName === "none"
      && getComputedStyle(document.querySelector("[data-project-composer]")).animationName === "none"
      && getComputedStyle(document.querySelector("[data-conversation-window]")).animationName === "none";
  }, await projectThreadIdentity));
  await page.locator("[data-project-preview-return]").click();
  check("returning from preview replays only the thread content body live transition", await page.evaluate(node => {
    const thread = document.querySelector("[data-project-thread]");
    return node === thread
      && thread.getAttribute("data-project-thread-mode") === "live"
      && getComputedStyle(thread).animationName === "v5-project-thread-live-enter";
  }, await projectThreadIdentity));
  const composer = page.locator("[data-project-composer] input");
  await composer.fill("保留的工程输入（render fixture）");
  // 428：证明持久层的滚动位置保留——注入测试专用高内容占位（随后移除），把 project
  // 线程滚到 140px；切回后断言 scrollTop 仍在 140。加载卡节点身份在切换前抓取
  //（B2a/B2b 预览期间线程内容会条件让位，早先抓取的节点可能已被替换）。
  await page.locator("[data-project-thread]").evaluate(thread => {
    // flex 容器内必须禁用收缩（flex:0 0 + min-height），否则占位会被压回容器高。
    thread.insertAdjacentHTML("beforeend", '<div data-test-scroll-spacer style="flex:0 0 1600px;min-height:1600px;height:1600px"></div>');
    thread.scrollTop = 140;
  });
  const projectFeedbackIdentity = await page.evaluateHandle(() => document.querySelector('[data-conversation-layer="project"] [data-window-launch-feedback]'));
  check("project loading card is mounted before the surface switch", Boolean(await page.evaluate(node => Boolean(node && node.isConnected), projectFeedbackIdentity)));
  check("project thread is scrollable before the surface switch", await page.locator("[data-project-thread]").evaluate(thread => thread.scrollTop === 140 && thread.scrollHeight > thread.clientHeight));
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
  // 428：窗口壳不重建；launcher 层挂载并激活，project 层保持挂载（hidden+inert），
  // 加载卡节点不重建、不重播。
  check("conversation window shell survives the project→launcher switch as the same DOM node", await page.evaluate(node => node === document.querySelector("[data-conversation-window]"), windowShellIdentity));
  check("both stream layers stay mounted across the project→launcher switch", await page.locator("[data-conversation-layer]").count() === 2
    && await page.locator('[data-conversation-layer="launcher"][data-conversation-layer-active="true"]').evaluate(layer => !layer.hidden && !layer.inert && layer.getAttribute("aria-hidden") === "false")
    && await page.locator('[data-conversation-layer="project"]').evaluate(layer => layer.hidden && layer.inert && layer.getAttribute("aria-hidden") === "true")
    && await page.evaluate(node => node === document.querySelector('[data-conversation-layer="project"]'), projectLayerIdentity));
  check("active launcher layer plays the 480ms emphasized-decelerate crossfade (40ms delay) while the inactive layer does not", await page.locator('[data-conversation-layer="launcher"]').evaluate(layer => {
    const style = getComputedStyle(layer);
    const durationMs = style.animationDuration.endsWith("ms") ? Number.parseFloat(style.animationDuration) : Number.parseFloat(style.animationDuration) * 1000;
    const delayMs = style.animationDelay.endsWith("ms") ? Number.parseFloat(style.animationDelay) : Number.parseFloat(style.animationDelay) * 1000;
    return style.animationName === "v5-conversation-layer-enter" && Math.abs(durationMs - 480) <= 1 && style.animationTimingFunction.replace(/\s+/g, "") === "cubic-bezier(0.05,0.7,0.1,1)" && Math.abs(delayMs - 40) <= 1 && style.transformOrigin !== "";
  }) && await page.locator('[data-conversation-layer="project"]').evaluate(layer => getComputedStyle(layer).animationName === "none"));
  // 429：无双影/重复 class——同一时刻只有 1 个可见层，active/inactive class 互斥各 1。
  check("only one stream layer is visible with exclusive active/inactive classes (no double image)", await page.evaluate(() => {
    const layers = [...document.querySelectorAll("[data-conversation-layer]")];
    const active = layers.filter(layer => layer.classList.contains("v5-conversation-layer--active"));
    const inactive = layers.filter(layer => layer.classList.contains("v5-conversation-layer--inactive"));
    const both = layers.filter(layer => layer.classList.contains("v5-conversation-layer--active") && layer.classList.contains("v5-conversation-layer--inactive"));
    const visible = layers.filter(layer => !layer.hidden && layer.getClientRects().length > 0);
    return layers.length === 2 && active.length === 1 && inactive.length === 1 && both.length === 0 && visible.length === 1;
  }));
  check("project loading card stays mounted while its layer is hidden", await page.evaluate(node => node === document.querySelector('[data-conversation-layer="project"] [data-window-launch-feedback]'), projectFeedbackIdentity) && await page.locator('[data-conversation-layer="project"] [data-window-launch-feedback]').count() === 1);
  check("collapsed rail state survives the project→launcher return", await page.evaluate(node => node === document.querySelector("[data-entry-rail]"), railIdentity)
    && await page.locator("[data-window-sidebar-toggle]").getAttribute("aria-expanded") === "false"
    && Math.round(await railWidth()) === 0);
  await page.locator("[data-entry-rail-expand]").click();
  await page.waitForFunction(() => Math.round(document.querySelector("[data-entry-rail]").getBoundingClientRect().width) === 280);
  check("expanding from the collapsed handle restores the startup project list", await page.evaluate(node => node === document.querySelector("[data-entry-rail]"), railIdentity)
    && await page.locator("[data-entry-rail] [data-launcher-projects]").count() === 1
    && await page.locator("[data-conversation-rail]").count() === 0);
  check("management sidebar preference survives remount and contains no invented chat list", await page.locator("[data-window-sidebar-toggle]").getAttribute("aria-expanded") === "true" && await page.locator("[data-window-sidebar-slot]").getAttribute("data-window-sidebar-slot") === "recents");
  // 429/430：左栏槽切换后同一时刻只有新内容（launcher drawer），旧工程对话流已卸载且
  // 新内容带 800ms（--v5-conv-motion-rail，431 试验值）入场。
  check("entry rail slot holds only the newly mounted content after the swap", await page.locator("[data-entry-rail-slot]").evaluate(slot => {
    const direct = [...slot.children].filter(child => child.matches(".v5-launcher-drawer, .v5-conversation-rail"));
    const style = direct[0] ? getComputedStyle(direct[0]) : null;
    const durationMs = style ? (style.animationDuration.endsWith("ms") ? Number.parseFloat(style.animationDuration) : Number.parseFloat(style.animationDuration) * 1000) : 0;
    return direct.length === 1
      && direct[0].matches(".v5-launcher-drawer")
      && slot.querySelectorAll(".v5-conversation-rail").length === 0
      && Math.abs(durationMs - 800) <= 1;
  }));
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
  // 428：返回工程面——同一 project 层节点重新激活（launcher 层保持挂载 hidden），窗口壳
  // 不变；滚动位置与加载卡状态随层保留（不重播）；左栏仍为同一节点。
  check("returning to project reactivates the same stream layer and keeps the launcher layer mounted", await page.evaluate(node => node === document.querySelector('[data-conversation-layer="project"]'), projectLayerIdentity)
    && await page.locator("[data-conversation-layer]").count() === 2
    && await page.locator('[data-conversation-layer="project"]').evaluate(layer => !layer.hidden && !layer.inert && layer.getAttribute("aria-hidden") === "false")
    && await page.locator('[data-conversation-layer="launcher"]').evaluate(layer => layer.hidden && layer.inert && layer.getAttribute("aria-hidden") === "true"));
  check("conversation window shell and entry rail stay the same nodes after the full round trip", await page.evaluate(node => node === document.querySelector("[data-conversation-window]"), windowShellIdentity) && await page.evaluate(node => node === document.querySelector("[data-entry-rail]"), railIdentity));
  check("project loading card is not replayed on returning to the project layer", await page.evaluate(node => node === document.querySelector('[data-conversation-layer="project"] [data-window-launch-feedback]'), projectFeedbackIdentity)
    && await page.locator('[data-conversation-layer="project"] [data-window-launch-feedback]').count() === 1
    && (await page.locator('[data-conversation-layer="project"] [data-window-launch-feedback]').getAttribute("data-window-launch-terminal")) !== "");
  check("project thread scroll position survives the surface round trip", await page.locator("[data-project-thread]").evaluate(thread => Math.abs(thread.scrollTop - 140) <= 1));
  await page.locator("[data-project-thread]").evaluate(thread => {
    const spacer = thread.querySelector("[data-test-scroll-spacer]");
    if (spacer) spacer.remove();
    thread.scrollTop = 0;
  });
  const uiStateGetsBeforeSend = report.requests.filter(request => request.path === "/agent/ui/state").length;
  await composer.fill("新建工程");
  await page.locator("[data-project-composer] button").click();
  await page.waitForFunction(old => window.__hostPosts.filter(p => p.type === "launcher_directive").at(-1)?.request_id !== old, request);
  request = await currentRequest();
  await progress(request, 11, "spawning");
  check("in-project directive uses same production feedback and gates input", await composer.isDisabled());
  // 427 B：加载中仍在对话流顶部可见、排在消息之前，且不遮挡输入区。
  check("pending loading card stays above messages and does not cover the composer", await page.locator("[data-project-thread]").evaluate(thread => {
    const card = thread.querySelector("[data-window-launch-feedback]");
    const composerElement = document.querySelector("[data-project-composer]");
    if (!card || !composerElement) return false;
    const cardBox = card.getBoundingClientRect();
    const composerBox = composerElement.getBoundingClientRect();
    const messages = [...thread.querySelectorAll("[data-message-author]")];
    const beforeMessages = messages.every(message => Boolean(card.compareDocumentPosition(message) & Node.DOCUMENT_POSITION_FOLLOWING));
    return beforeMessages && cardBox.height > 0 && cardBox.bottom <= composerBox.top + 1;
  }));
  await progress(request, 12, "failed", { detail: "工作台超时（render fixture）" });
  await postResult(request, "failed", "工作台超时（render fixture）");
  await feedback().locator('[data-loading-phase="failed"]').waitFor();
  check("project timeout never completes", !await composer.isDisabled() && await feedback().locator('[data-loading-phase="completed"]').count() === 0);
  // SHELL-PROJECT-STREAM-FIX-1（427）：发送完成后会刷新 /agent/ui/state（replySettled 后），
  // 左栏 active node/branches/新对话状态跟上服务端；发送的用户/agent 消息排在装载卡之后。
  let uiStateGetsAfterSend = report.requests.filter(request => request.path === "/agent/ui/state").length;
  for (let attempt = 0; attempt < 80 && uiStateGetsAfterSend <= uiStateGetsBeforeSend; attempt += 1) {
    await page.waitForTimeout(25);
    uiStateGetsAfterSend = report.requests.filter(request => request.path === "/agent/ui/state").length;
  }
  check("sending a message refreshes the rail ui state after the turn settles", uiStateGetsAfterSend > uiStateGetsBeforeSend, "before=" + uiStateGetsBeforeSend + " after=" + uiStateGetsAfterSend);
  await page.waitForFunction(() => document.querySelectorAll("[data-project-thread] [data-message-author]").length >= 2);
  check("sent user/agent messages render after the loading card", await page.locator("[data-project-thread]").evaluate(thread => {
    const card = thread.querySelector("[data-window-launch-feedback]");
    const messages = [...thread.querySelectorAll("[data-message-author]")];
    return Boolean(card) && messages.length > 0 && messages.every(message => Boolean(card.compareDocumentPosition(message) & Node.DOCUMENT_POSITION_FOLLOWING));
  }));
  await page.emulateMedia({ reducedMotion: "reduce" });
  check("reduced motion disables receipt and busy step motion", await feedback().evaluate(el => getComputedStyle(el).animationName === "none" && [...el.querySelectorAll("*")].every(child => getComputedStyle(child).animationName === "none")));
  check("reduced motion disables the conversation layer crossfade", await page.locator('[data-conversation-layer][data-conversation-layer-active="true"]').evaluate(layer => getComputedStyle(layer).animationName === "none"));
  // 429：reduced-motion 下左栏内容/工程 thread/launcher 内容体过渡同样全部关闭。
  check("reduced motion disables rail, thread and launcher content motion too", await page.evaluate(() => {
    const selectors = [
      ".v5-entry-rail-slot > .v5-conversation-rail",
      "[data-project-thread]",
      '[data-conversation-layer="launcher"] .v5-launcher-main',
      '[data-conversation-layer="launcher"] .v5-launcher-thread > *'
    ];
    const styles = selectors.map(selector => {
      const element = document.querySelector(selector);
      return element ? getComputedStyle(element) : null;
    }).filter(Boolean);
    return styles.length >= 3 && styles.every(style => style.animationName === "none");
  }));
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

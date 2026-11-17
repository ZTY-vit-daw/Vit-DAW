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
const report = { scope: "real App + mocked HTTP/host; no real stack", checks: [], requests: [], consoleErrors: [], failure: null };
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
      if (url.pathname === "/agent/ui/state") body = { project: { project_path: "C:/fixture/alpha.vit" }, tracks: [] };
      if (url.pathname === "/agent/chat") body = { reply: "切换工程（render fixture）", launcher_directive: { action: "new_project", project_name: "工程乙（render fixture）", requires_governance: true } };
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
  await page.locator("[data-window-sidebar-toggle]").focus();
  await page.keyboard.press("Enter");
  await page.waitForFunction(() => document.querySelector("[data-window-sidebar-slot]").getBoundingClientRect().width < 1);
  check("keyboard collapse hides and inerts only existing recents slot", await page.locator("[data-window-sidebar-slot]").evaluate(el => el.inert && el.getAttribute("aria-hidden") === "true"));
  check("collapse button remains keyboard accessible", await page.locator("[data-window-sidebar-toggle]").getAttribute("aria-expanded") === "false");
  await page.keyboard.press("Enter");
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
  await page.locator("[data-project-view-launcher]").click();
  await page.locator('[data-launcher-mode="project-management"]').waitFor();
  check("management sidebar preference survives remount and contains no invented chat list", await page.locator("[data-window-sidebar-toggle]").getAttribute("aria-expanded") === "true" && await page.locator("[data-window-sidebar-slot]").getAttribute("data-window-sidebar-slot") === "recents");
  await page.locator("[data-launcher-choose-file]").click();
  const fileRequest = await page.evaluate(() => window.__hostPosts.filter(p => p.type === "launcher_choose_file").at(-1).request_id);
  await postResult(fileRequest, "failed", "文件选择失败（render fixture）");
  await feedback().locator('[data-loading-phase="failed"]').waitFor();
  check("file-dialog failure never retries an unrelated previous directive", await feedback().locator("[data-loading-retry]").count() === 0);
  await page.locator("[data-launcher-settings]").click();
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
  console.log(JSON.stringify({ runRoot, checks: report.checks.length, failed: report.checks.filter(c => !c.pass), error: report.failure }, null, 2));
}

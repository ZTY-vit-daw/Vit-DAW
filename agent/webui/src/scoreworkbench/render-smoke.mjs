// 397 extension of the existing Vite + Playwright render-smoke pattern.
// 398 (SCORE-GUI-FOLLOW-1) extension: two-way selection sync DOM assertions —
// GUI track change drives the workbench (edge-triggered follow), explicit
// workbench picks echo select_track to the agent, dirty targets ask instead of
// forcing, and a cancelled target is never re-pushed by the poller.
// Runs the real index.html -> App -> Workbench -> ScoreTrackWorkbench mount.
// All HTTP state/tool replies below are fixtures: this is zero-stack rendering,
// not kernel, Godot selection, persistence, or J1-J3 real-stack acceptance.
import assert from "node:assert/strict";
import { mkdirSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import { chromium } from "playwright";

const webuiRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const outDir = resolve(webuiRoot, "../../coord/runs/SCORE-GUI-FOLLOW-1", process.argv[2] || `render_${new Date().toISOString().replace(/[^0-9]/g, "")}`);
mkdirSync(resolve(outDir, "screens"), { recursive: true });
const report = { schema_version: "score_binding_render.v1", boundary: "zero-stack, production App/Workbench mount, HTTP fixtures", assertions: [], errors: [] };
const check = (name, condition) => { report.assertions.push({ name, passed: !!condition }); assert.ok(condition, name); };
const original = [{ id: "note-a", pitch: 72, start: 0, length: .5 }];
const state = {
  status: "ok", project: { initialized: true, project_path: "fixture:/score-397.vit", track_count: 4 },
  tracks: [
    { id: "a", track_id: "a", name: "主题旋律", clips: [{ id: "a1", name: "八小节主题", clip_type: "midi" }] },
    { id: "b", track_id: "b", name: "第二旋律", clips: [{ id: "b1", name: "待读片段", clip_type: "midi" }] },
    { id: "many", track_id: "many", name: "多段旋律", clips: [{ id: "m1", name: "前奏", clip_type: "midi" }, { id: "m2", name: "尾声", clip_type: "midi" }, { id: "audio1", clip_type: "wave" }] },
    { id: "empty", track_id: "empty", name: "空轨道", clips: [] }
  ], ui_context: {}, artifacts: [], macro_controls: [], project_history: {}, authority_mode: "agent"
};
const notes = new Map(["a1", "b1", "m1", "m2"].map((id) => [id, [...original]]));
let failB = false;
const writes = [];
const selects = [];
let browser;
let server;
try {
  server = await createServer({ root: webuiRoot, server: { host: "127.0.0.1", port: 0, strictPort: false }, logLevel: "error" });
  await server.listen();
  const address = server.httpServer.address();
  const base = `http://127.0.0.1:${address.port}`;
  browser = await chromium.launch({ headless: true });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
  page.on("pageerror", (error) => report.errors.push(error.message));
  await page.route("**/agent/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    let reply = { status: "ok" };
    if (path === "/agent/ui/state") reply = state;
    else if (path === "/agent/invoke") {
      const { tool, args = {}, confirmed } = request.postDataJSON();
      if (failB && args.clip_id === "b1") reply = { status: "error", error: "fixture: 片段读取失败" };
      else {
        let result = { status: "ok" };
        if (tool === "analyze_midi_transformations") result.music_context_echo = { tempo_bpm: 120, time_signature_numerator: 2, time_signature_denominator: 4, tempo_setting_count: 1, time_sig_setting_count: 1 };
        if (tool === "get_midi_clip_notes") result.notes = notes.get(args.clip_id) || [];
        if (tool === "get_midi_clip_data") Object.assign(result, { start_seconds: 0, length_seconds: 4 });
        if (tool === "apply_midi_note_patch") {
          writes.push({ args, confirmed });
          notes.set(args.clip_id, args.operations[0].notes.map((note, index) => ({ ...note, id: `saved-${index}` })));
        }
        if (tool === "select_track") {
          selects.push({ track_id: args.track_id, source: request.postDataJSON().source ?? "" });
          result = { status: "ok", ui_action: "focus_track", track_id: args.track_id };
        }
        reply = { status: "ok", result };
      }
    } else if (path === "/agent/config") reply = { status: "ok", config: {}, has_api_key: false };
    else if (path === "/agent/events") reply = { status: "ok", events: [] };
    else if (path === "/agent/artifacts") reply = { status: "ok", artifacts: [] };
    else if (path === "/agent/history/conversation") reply = { status: "ok", messages: [] };
    await route.fulfill({ contentType: "application/json", body: JSON.stringify(reply) });
  });
  await page.route("**/health", (route) => route.fulfill({ contentType: "application/json", body: '{"status":"ok"}' }));
  await page.goto(`${base}/app/?surface=workbench`, { waitUntil: "domcontentloaded" });
  await page.locator('[data-workbench-tab="score"]').click();
  await page.waitForSelector("[data-score-binding-root]");
  await page.waitForSelector('.score-track-item[data-score-track-id="a"]');
  check("production Workbench mounts the binding container", await page.locator(".app-shell-workbench [data-score-binding-root]").count() === 1);
  check("no implicit clip editing before track selection", await page.locator("[data-score-root]").count() === 0);
  await page.locator('.score-track-item[data-score-track-id="empty"]').click();
  check("empty track has an honest empty state", (await page.locator("[data-score-binding-empty]").textContent()).includes("还没有 MIDI"));
  await page.locator('.score-track-item[data-score-track-id="many"]').click();
  check("multi-clip track waits for explicit selection", await page.locator("[data-score-root]").count() === 0);
  check("clip picker excludes audio", JSON.stringify(await page.locator("[data-score-clip-select] option").evaluateAll((options) => options.map((option) => option.value))) === JSON.stringify(["", "m1", "m2"]));
  await page.selectOption("[data-score-clip-select]", "m2");
  await page.waitForSelector('[data-score-write-ready="true"]');
  check("selected track and clip identity visible", (await page.locator("[data-score-editing-target]").textContent()).includes("多段旋律 · 尾声"));
  await page.locator('.score-track-item[data-score-track-id="a"]').click();
  await page.waitForSelector('[data-score-write-ready="true"]');
  const edit = async () => {
    const x = await page.locator("[data-score-svg]").getAttribute("data-score-tick-rows").then((rows) => Number(rows.match(/m0:0@(\d+)/)[1]));
    const box = await page.locator("[data-score-overlay]").boundingBox();
    await page.mouse.click(box.x + x + 1, box.y + 24);
    await page.locator('[data-score-dur="4"]').click();
    await page.waitForSelector('[data-score-dirty="true"]');
  };
  await edit();
  const edited = await page.locator("[data-score-root]").getAttribute("data-score-model");
  await page.locator('.score-track-item[data-score-track-id="b"]').click();
  await page.waitForSelector("[data-score-switch-confirm]");
  await page.locator("[data-score-switch-cancel]").click();
  check("cancel keeps A identity and edited buffer", await page.locator("[data-score-root]").getAttribute("data-score-clip-id") === "a1" && await page.locator("[data-score-root]").getAttribute("data-score-model") === edited);
  await page.screenshot({ path: resolve(outDir, "screens/desktop-editing.png") });
  failB = true;
  await page.locator('.score-track-item[data-score-track-id="b"]').click();
  await page.locator("[data-score-switch-discard]").click();
  await page.waitForFunction(() => document.querySelector("[data-score-root]")?.getAttribute("data-score-status")?.includes("装载失败"));
  check("failed B reload cannot write old A buffer", await page.locator("[data-score-apply]").isDisabled() && writes.length === 0 && await page.locator("[data-score-root]").getAttribute("data-score-model") !== edited);
  failB = false; // 398: the failure fixture only guards that window; later legs load b1 normally.
  await page.screenshot({ path: resolve(outDir, "screens/failed-load.png") });
  await page.locator('.score-track-item[data-score-track-id="a"]').click();
  await page.waitForSelector('[data-score-write-ready="true"]');
  await edit();
  await page.locator("[data-score-apply]").click();
  await page.locator("[data-score-confirm-apply]").click();
  await page.waitForSelector('[data-score-apply-state="saved"]');
  check("explicit apply uses only loaded track/clip with confirmation", writes.length === 1 && writes[0].args.track_id === "a" && writes[0].args.clip_id === "a1" && writes[0].confirmed === true);

  // SCORE-GUI-FOLLOW-1 (398): two-way selection sync, zero-stack DOM face.
  check("explicit score picks echo select_track to the agent", selects.length >= 5 && selects.every((pick) => pick.track_id && pick.source === "ask_vit_webui_score"));
  check("gui follow disclosure attribute present", ["", "pending", "confirmed"].includes(await page.locator("[data-score-binding-root]").getAttribute("data-score-gui-follow")));
  state.selected_track = { track_id: "b", name: "第二旋律" };
  state.ui_context = { selected_track_id: "b", selected_clip_id: "b1" };
  await page.waitForFunction(() => document.querySelector("[data-score-root]")?.getAttribute("data-score-clip-id") === "b1", null, { timeout: 15000 });
  check("GUI track change drives the score workbench (followed once, clip bound)", (await page.locator("[data-score-editing-target]").textContent()).includes("第二旋律 · 待读片段") && (await page.locator('.score-track-item[data-score-track-id="b"]').getAttribute("aria-pressed")) === "true");
  await page.waitForSelector('[data-score-write-ready="true"]', { timeout: 15000 });
  await page.waitForFunction(() => document.querySelector("[data-score-binding-root]")?.getAttribute("data-score-gui-follow") === "confirmed", null, { timeout: 8000 });
  check("GUI state confirms our select (pending -> confirmed)", true);
  await edit();
  state.selected_track = { track_id: "many", name: "多段旋律" };
  await page.waitForSelector("[data-score-switch-confirm]", { timeout: 15000 });
  check("dirty GUI change asks instead of forcing", (await page.locator("[data-score-switch-confirm]").textContent()).includes("多段旋律") && (await page.locator("[data-score-root]").getAttribute("data-score-clip-id")) === "b1");
  await page.locator("[data-score-switch-cancel]").click();
  await page.waitForTimeout(2600);
  check("cancelled target is not re-pushed by the poller (no grab-back)", (await page.locator("[data-score-switch-confirm]").count()) === 0 && (await page.locator("[data-score-root]").getAttribute("data-score-clip-id")) === "b1" && (await page.locator("[data-score-root]").getAttribute("data-score-dirty")) === "true");
  state.selected_track = { track_id: "empty", name: "空轨道" };
  await page.waitForSelector("[data-score-switch-confirm]", { timeout: 15000 });
  check("a new GUI edge still asks under the dirty guard", (await page.locator("[data-score-switch-confirm]").textContent()).includes("空轨道"));
  await page.locator("[data-score-switch-cancel]").click();
  await page.screenshot({ path: resolve(outDir, "screens/gui-follow-dirty-guard.png") });

  await page.setViewportSize({ width: 420, height: 900 });
  await page.screenshot({ path: resolve(outDir, "screens/narrow.png") });
  const bounds = await page.locator("[data-score-binding-root]").boundingBox();
  check("binding rail fits narrow viewport", bounds.width <= 420 && bounds.x >= 0);
  check("no uncaught render errors", report.errors.length === 0);
  report.status = "PASS";
} catch (failure) {
  report.status = "FAIL";
  report.errors.push(failure.stack || String(failure));
  process.exitCode = 1;
} finally {
  if (browser) await browser.close();
  if (server) await server.close();
  writeFileSync(resolve(outDir, "render-smoke-report.json"), JSON.stringify(report, null, 2));
  console.log(JSON.stringify({ status: report.status, assertions: report.assertions.length, errors: report.errors, outDir }, null, 2));
}

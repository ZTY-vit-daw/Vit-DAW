// vitshell_full_smoke_cdp.mjs — SHELL-3 smoke CDP driver (React DOM face).
//
// SHELL-2's vitshell_open_chain_cdp.mjs phases plus the SHELL-3 legs:
//   probe-launcher-data   launcher real-data assertions (live greeting, no fixture)
//   probe-project-ready   project surface ready + context name (fixture-loaded)
//   chat-roundtrip        real in-project chat roundtrip: type -> send -> user
//                         frame + assistant frame in [data-project-thread]
//   probe-error-page      in-window error page DOM (bad-agent / occupied-port legs)
//   probe-launcher-kernel-online (SHELL-4) launcher kernel-online assertions:
//                         cold-start stage: kernel attr online + real recents
//                         (seeded) + drawer ready — no degraded greeting face.
//   launcher-chat-new     (SHELL-4) new-project end-to-end via launcher chat:
//                         type 新建工程 <name> (or bare 新建工程 = SHELL-5
//                         name-less auto-execute) -> directive AUTO-executes
//                         (no manual button) -> governance auto-switch if shown.
//   launcher-button-new   (SHELL-5) drawer ＋新建 = DIRECT new: click posts a
//                         new_project directive straight through the bridge
//                         (no chat round, no question loop).
//   probe-launcher-no-binding (SHELL-5) launcher stage zero-binding: /agent/state
//                         current_project.path empty (boot draft not bound).
//   project-chat-new     (SHELL-6) IN-PROJECT new chain: project-surface chat
//                         bare "新建工程" -> deterministic directive (no LLM
//                         free-text) -> AUTO-executes via bridge -> in-page
//                         governance 3-key card IN THE PROJECT SURFACE (the
//                         SHELL-6 invariant: confirmation must be clickable)
//                         -> 直接切换 via the real bridge path.
//   probe-project-ready --expect-name <substr> (SHELL-6) after an in-page
//                         project switch: wait for the titlebar context name
//                         to become the expected one (context refresh on the
//                         bridge opened receipt; no remount, no reload).
//
// Everything else mirrors vitshell_open_chain_cdp.mjs semantics:
//   probe-launcher / click-open / probe-project / probe-launcher-back /
//   assert-governance / click-governance-switch / probe-governance-done
//
// Production zero behavior — invoked only by vitshell_full_smoke.ps1.
// Run from agent/webui so the pinned playwright resolves:
//   node ../../scripts/vitshell_full_smoke_cdp.mjs --cdp-port 9333 --phase probe-launcher

import { createRequire } from "node:module";
import { writeFileSync, readFileSync } from "node:fs";
import { join } from "node:path";

function arg(name, fallback = "") {
  const idx = process.argv.indexOf("--" + name);
  if (idx >= 0 && idx + 1 < process.argv.length) return process.argv[idx + 1];
  return fallback;
}

const cdpPort = arg("cdp-port", "9333");
const statusUrl = arg("status-url", "http://127.0.0.1:7879/status");
const presentationScenario = arg("presentation-scenario", "normal");
const windowChecks = process.argv.includes("--window-behavior-checks");
const previousLaunchId = arg("previous-launch-id");
const godotLogPath = arg("godot-log", join(process.env.APPDATA || "", "Godot/app_userdata/Vit_DAW_Frontend/logs/godot.log"));
const bridgeLogPath = arg("bridge-log");
const phase = arg("phase", "");
const outPath = arg("out", "");
const chatText = arg("chat-text", "这个工程现在是什么状态？");
const newProjectName = arg("new-project-name", "");
const expectName = arg("expect-name", "");
const chatTimeoutMs = parseInt(arg("chat-timeout-ms", "90000"), 10);
const webuiDir = arg("webui-dir", join(process.cwd(), "agent/webui"));
const requireFromWebui = createRequire(join(webuiDir, "package.json"));
const { chromium } = requireFromWebui("playwright");
let activePresentationCapture = null;

function fail(message, evidence) {
  if (activePresentationCapture && outPath) {
    writeFileSync(outPath + ".presentation-partial.json", JSON.stringify({ scenario: presentationScenario, samples: activePresentationCapture.samples, sample_errors: activePresentationCapture.errors, last_native: activePresentationCapture.lastNative, failed: message }, null, 2));
    try { writeFileSync(outPath + ".godot-on-failure.log", readFileSync(godotLogPath)); } catch { /* No current Godot log can exist before spawn. */ }
  }
  const verdict = { phase, ok: false, error: message };
  if (evidence && typeof evidence === "object") verdict.evidence = evidence;
  if (outPath) writeFileSync(outPath, JSON.stringify(verdict, null, 2));
  console.error("CDP_FAIL " + JSON.stringify(verdict));
  process.exit(1);
}

async function getPage(browser) {
  for (const context of browser.contexts()) {
    const pages = context.pages();
    if (pages.length > 0) return pages[0];
  }
  return null;
}

async function waitBridge(browser, timeoutMs = 30000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const page = await getPage(browser);
    if (page) return page;
    await new Promise((r) => setTimeout(r, 250));
  }
  return null;
}

async function withSentinel(page) {
  return page.evaluate(() => {
    if (!window.__vit_no_reload) {
      window.__vit_no_reload = Date.now();
    }
    return window.__vit_no_reload;
  });
}

async function readSentinel(page) {
  return page.evaluate(() => window.__vit_no_reload ?? null);
}

function parseGodotReadiness(raw, requestId) {
  return raw.split(/\r?\n/).flatMap((line, lineIndex) => {
    const match = line.match(/\[VitShellLaunch\] ([\w.]+) (\{.*\})/);
    if (!match) return [];
    try { const payload = JSON.parse(match[2]); return payload.launch_id === requestId ? [{ event: match[1], lineIndex, payload }] : []; } catch { return []; }
  });
}

function reconstructEventTick(eventTime32, receivedTick64) {
  const wrap = 0x100000000;
  if (!Number.isSafeInteger(receivedTick64) || !Number.isInteger(eventTime32) || eventTime32 < 0 || eventTime32 >= wrap) throw new Error("invalid WinEvent clock value");
  const age = ((receivedTick64 % wrap) - eventTime32 + wrap) % wrap;
  if (age >= wrap / 2) throw new Error("WinEvent time is not in the unambiguous preceding DWORD half-window");
  return receivedTick64 - age;
}

// Union the bounded native ring across fresh status reads AND host messages.
// A global historic dropped_count is not a gap in this launch if the union
// covers every sequence from guard start through the final drain observation.
function evaluateRawWindowEvents(samples, messages, requestId) {
  const facts = [
    ...samples.map(s => ({ source: `status:${s.index}`, window: s.status?.window_state })),
    ...messages.filter(m => !m.synthetic && m.payload?.type === "vit_window_state").map(m => ({ source: `message:${m.index}`, window: m.payload.window_state })),
  ].filter(f => f.window?.win_event_log?.schema === "vitshell.win_event_log.v1");
  const currentFacts = facts.filter(f => f.window.win_event_log.launch_id === requestId);
  if (!currentFacts.length) throw new Error("BOUNDARY: current launch raw WinEvent log unavailable");
  const releasedFacts = currentFacts.filter(f => f.window.win_event_log.guard_released_tick_ms > 0);
  const final = releasedFacts.sort((a, b) => b.window.observed_at_tick_ms - a.window.observed_at_tick_ms)[0]?.window;
  if (!final) throw new Error("BOUNDARY: guard release raw endpoints unavailable");
  const metadata = final.win_event_log;
  const startTick = metadata.guard_started_tick_ms, endTick = metadata.guard_released_tick_ms;
  const startSeq = metadata.guard_started_seq, releasedSeq = metadata.guard_released_seq;
  const endSeq = Math.max(...currentFacts.map(f => f.window.win_event_log.last_seq));
  const pid = final.direct_child_pid;
  if (!(Number.isInteger(pid) && pid > 0 && startTick > 0 && endTick >= startTick && startSeq > 0 && releasedSeq >= startSeq - 1 && endSeq >= releasedSeq)) throw new Error("BOUNDARY: invalid current launch guard/PID endpoints");
  const firstObservedTick = Math.min(...samples.map(s => s.status?.window_state?.observed_at_tick_ms).filter(Number.isFinite));
  if (!(firstObservedTick <= startTick)) throw new Error("BOUNDARY: observation began after the loading guard");
  const knownWindows = new Set(currentFacts.filter(f => f.window.direct_child_pid === pid && f.window.daw_hwnd).map(f => f.window.daw_hwnd));
  const bySeq = new Map();
  for (const fact of facts) for (const event of fact.window.win_event_log.events || []) {
    if (!Number.isSafeInteger(event.seq)) throw new Error("BOUNDARY: malformed raw event sequence");
    const prior = bySeq.get(event.seq);
    if (prior && JSON.stringify(prior.event) !== JSON.stringify(event)) throw new Error("BOUNDARY: conflicting raw event sequence payload");
    if (prior) prior.sources.push(fact.source);
    else bySeq.set(event.seq, { event, sources: [fact.source] });
  }
  const missing = [];
  for (let seq = startSeq; seq <= endSeq; seq++) if (!bySeq.has(seq)) missing.push(seq);
  if (missing.length) throw new Error(`BOUNDARY: raw event union has current interval gaps: ${missing.slice(0, 12).join(",")}`);
  const events = [...bySeq.values()].filter(e => e.event.launch_id === requestId && e.event.launch_pid === pid)
    .map(item => ({ ...item, occurred_tick_ms: reconstructEventTick(item.event.event_time_ms, item.event.received_tick_ms) }))
    .filter(e => e.occurred_tick_ms >= startTick && e.occurred_tick_ms <= endTick)
    .sort((a, b) => a.event.seq - b.event.seq);
  const foregroundTargets = events.filter(({ event: e }) => e.event === 3 && (e.target_pid === pid || knownWindows.has(e.target_hwnd)));
  const unknownForeground = events.filter(({ event: e }) => e.event === 3 && e.target_pid === 0);
  // Callback receive-time guard=false or seq>releasedSeq does not exclude an
  // event that actually occurred during the loading guard.
  const lateGuardEvents = events.filter(e => e.event.seq > releasedSeq || !e.event.guard_active_at_receive);
  const raw = { schema: metadata.schema, launch_id: requestId, direct_child_pid: pid, first_observed_tick_ms: firstObservedTick,
    guard_started_tick_ms: startTick, guard_released_tick_ms: endTick, guard_started_seq: startSeq, guard_released_seq: releasedSeq,
    union_through_seq: endSeq, covered_sequence_count: Math.max(0, endSeq - startSeq + 1), missing_sequences: missing,
    historical_dropped_count: metadata.dropped_count, historical_truncated: metadata.truncated,
    known_daw_windows: [...knownWindows], foreground_targets: foregroundTargets, unknown_foreground_targets: unknownForeground,
    late_guard_events: lateGuardEvents, events,
    boundary: "Complete available hook union within capture plus 500ms terminal drain; not a claim about all Windows events or unbounded delayed callbacks." };
  if (foregroundTargets.length) raw.violation = "raw FOREGROUND targeted current DAW during guard";
  else if (unknownForeground.length) raw.violation = "BOUNDARY: foreground target PID unavailable during guard";
  return raw;
}

// Read-only observation, armed before the existing launch action. A finite
// 40ms sample/event window does not claim absence of every transient OS state.
async function beginPresentationCapture(page) {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.evaluate(() => {
    const subscriptionId = `smoke-window-${crypto.randomUUID()}`;
    const capture = { subscriptionId, messages: [], startedAt: Date.now() };
    window.__shellPresentationCapture = capture;
    window.chrome.webview.addEventListener("message", (event) => {
      const payload = event.data;
      if (payload && ["vit_launch_progress", "vit_window_state", "vit_launcher_result", "vit_surface"].includes(payload.type)) {
        capture.messages.push({ index: capture.messages.length, receivedAt: Date.now(), synthetic: capture.syntheticDispatchActive === true, payload });
      }
    });
    window.chrome.webview.postMessage({ channel: "ask_vit_shell", type: "window_presentation_ready", subscription_id: subscriptionId });
  });
  const samples = [];
  const errors = [];
  const capture = { samples, errors, lastNative: null };
  activePresentationCapture = capture;
  let stopped = false;
  let pendingSample = null;
  const sample = async () => {
    try {
      const response = await page.request.get(statusUrl, { timeout: 2000 });
      if (!response.ok()) throw new Error(`status HTTP ${response.status()}`);
      const status = await response.json();
      const dom = await page.evaluate(() => {
        const el = document.querySelector("[data-window-launch-feedback]");
        return { requestId: el?.getAttribute("data-window-launch-request"), seq: Number(el?.getAttribute("data-window-launch-seq") || 0),
          terminal: el?.getAttribute("data-window-launch-terminal"), text: el?.textContent || "",
          focus: document.querySelector("[data-project-root], [data-launcher-root]")?.getAttribute("data-native-focus"),
          nativeCapture: window.__shellPresentationCapture,
          percentage: Boolean(el?.querySelector('[role="progressbar"][aria-valuenow]')) || /\d\s*%/.test(el?.textContent || "") };
      });
      capture.lastNative = dom.nativeCapture;
      delete dom.nativeCapture;
      samples.push({ index: samples.length, receivedAt: Date.now(), status, dom });
    } catch (error) { errors.push(String(error)); }
  };
  await sample(); // Must finish before any open/new action.
  if (samples.length !== 1) throw new Error("initial pre-open status sample unavailable");
  const sidebar = page.locator("[data-window-sidebar-toggle]");
  await sidebar.click();
  const collapsed = await page.locator("[data-window-sidebar-slot]").evaluate(el => ({ hidden: el.getAttribute("aria-hidden"), inert: el.inert, width: el.getBoundingClientRect().width, transition: getComputedStyle(el).transitionDuration }));
  if (collapsed.hidden !== "true" || !collapsed.inert || collapsed.width !== 0 || collapsed.transition.split(",").some(value => parseFloat(value) !== 0)) throw new Error("reduced-motion sidebar collapse/inert contract failed");
  await sidebar.click();
  if (await sidebar.getAttribute("aria-expanded") !== "true") throw new Error("sidebar did not reopen before the launch");
  capture.sidebar = { reduced_motion: true, collapsed, reopened: true };
  const timer = setInterval(() => { if (!stopped && !pendingSample) pendingSample = sample().finally(() => { pendingSample = null; }); }, 40);
  capture.sample = sample;
  capture.stop = async () => { stopped = true; clearInterval(timer); await pendingSample; };
  return capture;
}

async function finishPresentationCapture(page, capture) {
  const success = ["normal", "delayed_success"].includes(presentationScenario);
  const expectedTerminal = success ? "opened" : "failed";
  let injected = null;
  let consumerNegative = null;
  if (presentationScenario === "delayed_success") {
    if (!previousLaunchId) throw new Error("old nonce must come from a completed normal run");
    await page.waitForFunction(() => window.__shellPresentationCapture.messages.some(({ payload: p }) => p.type === "vit_launch_progress" && !p.replay && p.phase === "waiting_launch_result"), null, { timeout: 45000 });
    const requestId = await page.evaluate(() => window.__shellPresentationCapture.messages.find(({ payload: p }) => p.type === "vit_launch_progress" && !p.replay && p.phase === "spawning")?.payload.request_id);
    if (!requestId || requestId === previousLaunchId) throw new Error("current nonce missing or reused");
    const wrongId = `wrong-${crypto.randomUUID()}`;
    const boundaryDeadline = Date.now() + 15000;
    while (Date.now() < boundaryDeadline) {
      const raw = (() => { try { return readFileSync(godotLogPath, "utf8"); } catch { return ""; } })();
      if (parseGodotReadiness(raw, requestId).some(e => e.payload.trigger === "repository.before_apply.delay_begin")) break;
      await new Promise(resolve => setTimeout(resolve, 40));
    }
    if (!parseGodotReadiness(readFileSync(godotLogPath, "utf8"), requestId).some(e => e.payload.trigger === "repository.before_apply.delay_begin")) throw new Error("nonce injection did not reach actual delayed snapshot boundary");
    const beforeResponse = await page.request.get(statusUrl);
    const before = await beforeResponse.json();
    if (before.window_state?.loading_guard_active !== true) throw new Error("nonce injection started after loading guard was released");
    const injections = [];
    for (const id of [wrongId, previousLaunchId]) {
      const response = await page.request.post(statusUrl.replace(/\/status$/, "/vitshell/launch-result"), { data: { ok: true, stage: "workspace_ready", scene: "synthetic-wrong-nonce", launch_id: id }, timeout: 5000 });
      injections.push({ launch_id: id, http_status: response.status(), synthetic_injection: true });
    }
    const afterResponse = await page.request.get(statusUrl);
    const after = await afterResponse.json();
    const prematurelyOpened = await page.evaluate((id) => window.__shellPresentationCapture.messages.some(({ payload: p, synthetic }) => !synthetic && p.request_id === id && (p.phase === "opened" || p.status === "opened")), requestId);
    if (after.window_state?.loading_guard_active !== true || prematurelyOpened) throw new Error("wrong/old nonce terminated the current pending chain");
    injected = { current_request_id: requestId, injections, before_status: before, after_status: after };
    consumerNegative = await page.evaluate(async (id) => {
      const bridge = window.chrome.webview;
      if (typeof bridge.dispatchEvent !== "function") return { supported: false, boundary: "WebView EventTarget dispatcher unavailable; see 395 reducer unit evidence" };
      const feedback = () => { const el = document.querySelector("[data-window-launch-feedback]"); return { request: el?.getAttribute("data-window-launch-request"), seq: Number(el?.getAttribute("data-window-launch-seq") || 0), terminal: el?.getAttribute("data-window-launch-terminal") }; };
      const before = feedback();
      const synthetic = [
        { type: "vit_launch_progress", schema: "vitshell.launch_progress.v1", request_id: "foreign-consumer-probe", seq: before.seq + 100, phase: "opened", stage: "workspace_ready", detail: "synthetic wrong request" },
        { type: "vit_launch_progress", schema: "vitshell.launch_progress.v1", request_id: id, seq: before.seq, phase: "opened", stage: "workspace_ready", detail: "synthetic duplicate sequence" },
        { type: "vit_launch_progress", schema: "vitshell.launch_progress.v1", seq: before.seq + 100, phase: "opened", detail: "synthetic missing request/stage" },
      ];
      window.__shellPresentationCapture.syntheticDispatchActive = true;
      try { for (const payload of synthetic) bridge.dispatchEvent(new MessageEvent("message", { data: payload })); }
      finally { window.__shellPresentationCapture.syntheticDispatchActive = false; }
      await new Promise(resolve => setTimeout(resolve, 0));
      const after = feedback();
      if (after.seq !== before.seq || after.request !== before.request || after.terminal === "ready") throw new Error("old/foreign/missing progress upgraded live consumer");
      // The legacy result contains no stage. The presentation provider must
      // keep it unconfirmed until the real native workspace_ready progress.
      window.__shellPresentationCapture.syntheticDispatchActive = true;
      try { bridge.dispatchEvent(new MessageEvent("message", { data: { type: "vit_launcher_result", request_id: id, status: "opened", message: "synthetic consumer-only missing-stage probe" } })); }
      finally { window.__shellPresentationCapture.syntheticDispatchActive = false; }
      await new Promise(resolve => setTimeout(resolve, 0));
      const missingStage = feedback();
      if (missingStage.terminal === "ready") throw new Error("missing-stage result upgraded live consumer ready");
      return { supported: true, synthetic_consumer_only: true, before, after, missing_stage_result: missingStage };
    }, requestId);
  }
  await page.waitForFunction((terminal) => {
    const messages = window.__shellPresentationCapture.messages;
    const requestId = messages.find(({ payload: p, synthetic }) => !synthetic && p.type === "vit_launch_progress" && !p.replay && p.phase === "spawning")?.payload.request_id;
    return requestId && messages.some(({ payload: p, synthetic }) => !synthetic && p.type === "vit_launch_progress" && !p.replay && p.request_id === requestId && [terminal, "failed", "canceled"].includes(p.phase));
  }, expectedTerminal, { timeout: 250000 });
  if (success) {
    const earlyFailure = await page.evaluate(() => {
      const capture = window.__shellPresentationCapture;
      const requestId = capture.messages.find(({ payload: p, synthetic }) => !synthetic && p.phase === "spawning")?.payload.request_id;
      return capture.messages.find(({ payload: p, synthetic }) => !synthetic && p.type === "vit_launch_progress" && p.request_id === requestId && ["failed", "canceled"].includes(p.phase));
    });
    if (earlyFailure) { await capture.sample(); await capture.stop(); throw new Error(`real launch terminated ${earlyFailure.payload.phase}: ${earlyFailure.payload.detail}`); }
  }
  await page.waitForFunction(() => window.__shellPresentationCapture.messages.some(({ payload: p }) => p.type === "vit_window_state" && p.window_state?.loading_guard_active === false && p.window_state?.release_reason), null, { timeout: 10000 });
  await page.waitForFunction(() => document.querySelector('[data-window-launch-terminal="ready"], [data-window-launch-terminal="failed"]'), null, { timeout: 10000 });
  await new Promise(resolve => setTimeout(resolve, 500)); // bounded late-hook drain, no input/focus writes
  if (presentationScenario === "workspace_timeout") {
    const timeoutRequestId = await page.evaluate(() => window.__shellPresentationCapture.messages.find(({ payload: p, synthetic }) => !synthetic && p.phase === "spawning")?.payload.request_id);
    const lateDeadline = Date.now() + 30000;
    while (Date.now() < lateDeadline) {
      const raw = (() => { try { return readFileSync(godotLogPath, "utf8"); } catch { return ""; } })();
      if (parseGodotReadiness(raw, timeoutRequestId).some(e => e.payload.trigger === 'repository.before_apply.delay_expired_after_terminal')) break;
      await new Promise(resolve => setTimeout(resolve, 40));
    }
    if (!parseGodotReadiness(readFileSync(godotLogPath, "utf8"), timeoutRequestId).some(e => e.payload.trigger === 'repository.before_apply.delay_expired_after_terminal')) throw new Error("late delayed apply was not observed after timeout");
  }
  await capture.sample();
  await capture.stop();
  // Preserve the real process log BEFORE evaluating any window/DOM assertion;
  // the outer hygiene wrapper will subsequently restore the user's log file.
  const godotRaw = readFileSync(godotLogPath, "utf8");
  writeFileSync(outPath + ".godot.log", godotRaw);
  if (capture.errors.length) throw new Error("BOUNDARY: status/DOM sample unavailable inside the finite observation interval");
  const native = await page.evaluate(() => window.__shellPresentationCapture);
  const progress = native.messages.filter(({ payload: p, synthetic }) => !synthetic && p.type === "vit_launch_progress" && !p.replay);
  const requestId = progress.find(({ payload: p }) => p.phase === "spawning")?.payload.request_id;
  const current = progress.filter(({ payload: p }) => p.request_id === requestId);
  if (!requestId || !current.length || current.some((m, i) => i && m.payload.seq <= current[i - 1].payload.seq)) throw new Error("native request sequence is missing/nonmonotonic");
  const terminal = current.find(({ payload: p }) => p.phase === expectedTerminal);
  if (!terminal || (success && terminal.payload.stage !== "workspace_ready")) throw new Error("opened lacks current workspace_ready evidence");
  const rawWindowEvents = evaluateRawWindowEvents(capture.samples, native.messages, requestId);
  writeFileSync(outPath + ".win-event-evaluation.json", JSON.stringify(rawWindowEvents, null, 2));
  if (rawWindowEvents.violation) throw new Error(rawWindowEvents.violation);
  const sampledWindows = capture.samples.map(s => s.status.window_state);
  const loadingWindows = sampledWindows.filter(w => w?.loading_guard_active === true && w.daw_hwnd !== null && w.daw_visible === true && w.daw_iconic === false);
  const preopen = capture.samples[0].status.window_state;
  const foregroundCoverage = { preopen_foreground: preopen?.foreground, shell_preservation_branch: preopen?.foreground === "shell" ? "exercised" : "not_exercised", external_application_branch: preopen?.foreground === "other" ? "exercised" : "not_exercised" };
  if (!["shell", "other"].includes(preopen?.foreground)) throw new Error("BOUNDARY: pre-open foreground identity unavailable");
  if (!loadingWindows.length || !loadingWindows.some(w => w.shell_above_daw === true && w.daw_noactivate === true && w.event_hooks_ready === true)) throw new Error("BOUNDARY: no complete loading ordinary-window/z-order/noactivate observation");
  if (preopen.foreground === "shell" && loadingWindows.some(w => w.foreground !== "shell")) throw new Error("shell preservation branch lost foreground during loading");
  if (preopen.foreground === "other" && loadingWindows.some(w => w.foreground !== "other")) throw new Error("external-foreground branch did not preserve the external application");
  if (loadingWindows.some(w => w.foreground === "daw" || w.shell_above_daw === false || w.daw_noactivate === false)) throw new Error("observed DAW activation/order/style violation while loading");
  const firstVisible = capture.samples.map(s => s.status.window_state).find(w => w?.win_event_log?.launch_id === requestId && w.daw_visible === true && w.daw_iconic === false);
  if (!firstVisible || firstVisible.hidden_boot_state !== "prepared" || firstVisible.daw_topmost !== false || firstVisible.daw_iconic !== false || firstVisible.daw_noactivate !== true || firstVisible.shell_above_daw !== true) throw new Error("BOUNDARY: first observed visible DAW lacks prepared/non-topmost/noactivate/large-window facts");
  if (loadingWindows.some(w => w.daw_topmost !== false || w.daw_visible !== true || w.daw_iconic !== false)) throw new Error("loading DAW topmost/visibility/iconic fact failed or unavailable");
  const released = capture.samples.filter(s => s.receivedAt >= terminal.receivedAt).map(s => s.status.window_state).find(w => w?.loading_guard_active === false && w.release_reason);
  if (!released || (released.daw_hwnd !== null && released.daw_noactivate !== false)) throw new Error("guard/noactivate release not observed");
  if (capture.samples.some(s => s.dom.percentage)) throw new Error("UI fabricated a percentage");
  const domForRequest = capture.samples.filter(s => s.dom.requestId === requestId);
  if (!domForRequest.length || domForRequest.some((s, i) => i && s.dom.seq < domForRequest[i - 1].dom.seq)) throw new Error("UI request sequence absent/regressed");
  const emittedSequences = new Set(current.map(m => m.payload.seq));
  if (domForRequest.some(s => s.dom.seq > 0 && !emittedSequences.has(s.dom.seq))) throw new Error("UI sequence has no matching current native observation");
  const uiReady = capture.samples.find(s => s.dom.requestId === requestId && s.dom.terminal === "ready");
  if (!success && uiReady) throw new Error("failed request became ready in UI");
  if (uiReady && uiReady.receivedAt < terminal.receivedAt) throw new Error("UI ready preceded its real terminal stage");
  const godotEvents = parseGodotReadiness(godotRaw, requestId);
  const mount = godotEvents.find(e => e.event === "workspace.mounted");
  const snapshot = godotEvents.find(e => e.event === "workspace.snapshot_applied");
  const decision = godotEvents.find(e => e.event === "readiness.decided");
  const config = godotEvents.find(e => e.event === "window_bootstrap.config");
  const confirmed = godotEvents.find(e => e.event === "window_bootstrap.confirmed");
  const begin = godotEvents.find(e => e.event === "readiness.begin");
  // direct.begin historically lacks launch_id. Require its line after the
  // nonce-matched confirmation by scanning the same per-process log below.
  const directLine = godotRaw.split(/\r?\n/).findIndex(line => line.includes('[VitShellLaunch] direct.begin '));
  if (!begin || !config || !confirmed || config.payload.feature_enabled !== true || config.payload.initial_mode !== 1 || confirmed.payload.prepared !== true || confirmed.payload.godot_pid !== rawWindowEvents.direct_child_pid || !(begin.lineIndex < config.lineIndex && config.lineIndex < confirmed.lineIndex && confirmed.lineIndex < directLine)) throw new Error("real hidden-bootstrap/config/prepared/direct-open sequence missing");
  if (success && (!mount || !snapshot || !decision || !(mount.lineIndex < snapshot.lineIndex && snapshot.lineIndex < decision.lineIndex) || decision.payload.stage !== "workspace_ready" || decision.payload.ok !== true || mount.payload.workspace_instance_id !== snapshot.payload.workspace_instance_id)) throw new Error("real mounted/snapshot/ready association/order missing");
  if (!success && (snapshot || decision?.payload.ok === true)) throw new Error("negative scenario acquired a ready snapshot");
  if (!success && current.some(({ payload: p }) => p.phase === "opened")) throw new Error("negative scenario emitted opened");
  if (presentationScenario === "workspace_timeout" && (!mount || decision?.payload.stage !== "workspace_timeout" || !godotEvents.some(e => e.payload.trigger === "repository.before_apply.delay_begin"))) throw new Error("timeout did not reach the actual delayed snapshot boundary");
  if (["scene_failed", "snapshot_failed"].includes(presentationScenario) && decision?.payload.stage !== presentationScenario) throw new Error("failure did not come from intended real boundary");
  if (presentationScenario === "exit_before_ready" && !godotEvents.some(e => e.payload.trigger === "workspace_mounted.quit" && e.payload.exit_code === 17)) throw new Error("real early exit marker missing");
  const bridgeRaw = readFileSync(bridgeLogPath, "utf8");
  const nativeBoot = bridgeRaw.split(/\r?\n/).find(line => line.includes('[LauncherBridge] window_bootstrap ') && line.includes(`\"launch_id\":\"${requestId}\"`));
  if (!nativeBoot) throw new Error("nonce-matched native bootstrap preparation log missing");
  const bootRecord = JSON.parse(nativeBoot.slice(nativeBoot.indexOf('{')));
  if (bootRecord.state !== "prepared" || bootRecord.pid !== rawWindowEvents.direct_child_pid || bootRecord.native?.hidden_boot_state !== "prepared" || bootRecord.native?.daw_topmost !== false || bootRecord.native?.daw_noactivate !== true || bootRecord.native?.daw_visible !== true || bootRecord.native?.daw_iconic !== false) throw new Error("native bootstrap prepared facts incomplete");
  const trace = bootRecord.native.hidden_boot_trace;
  if (!trace || trace.daw_foreground_seen !== false || trace.foreground_identity_unknown !== false || !trace.before || !trace.after || trace.before.pid !== rawWindowEvents.direct_child_pid || trace.before.topmost !== false || (trace.before.visible === true && trace.before.iconic !== true) || trace.after.visible !== true || trace.after.iconic !== false || trace.after.noactivate !== true || trace.after.topmost !== false) throw new Error("safe hidden/iconic native prestate and ordinary-window poststate evidence incomplete");
  if (injected) for (const injection of injected.injections) {
    const rejection = bridgeRaw.split(/\r?\n/).find(line => line.includes("launch_result.rejected") && line.includes(`\"launch_id\":\"${injection.launch_id}\"`) && line.includes(`\"expected_launch_id\":\"${requestId}\"`) && line.includes("wrong_launch_id"));
    if (!rejection) throw new Error("HTTP wrong/old nonce rejection lacks native run-local pairing");
  }
  if (success && !bridgeRaw.includes(`launch_result.accepted {\"launch_id\":\"${requestId}\"`)) throw new Error("current real receipt was not nonce-accepted");
  const result = { scenario: presentationScenario, request_id: requestId, observation_boundary: "finite 40ms current-state samples plus complete available raw hook union and 500ms drain; no claim about absolute every transient OS state", raw_window_events: rawWindowEvents, first_observed_visible: firstVisible, bootstrap_native: bootRecord, native, samples: capture.samples, sample_errors: capture.errors, godot_events: godotEvents, injected, consumer_negative: consumerNegative, sidebar: capture.sidebar,
    foreground_coverage: foregroundCoverage, focus_hint_positive_observed: capture.samples.some(s => s.status.window_state?.foreground === "daw" && s.dom.focus === "daw"), focus_experience_boundary: "No desktop input was generated. Ordinary post-ready focus remains for user testing." };
  writeFileSync(outPath + ".presentation.json", JSON.stringify(result, null, 2));
  await page.screenshot({ path: outPath + ".presentation.png" });
  return result;
}

if (process.argv.includes("--presentation-self-test")) {
  const fixture = [
    '[VitShellLaunch] workspace.mounted {"launch_id":"old","workspace_instance_id":"8"}',
    '[VitShellLaunch] readiness.decided malformed',
    '[VitShellLaunch] workspace.mounted {"launch_id":"current","workspace_instance_id":"9"}',
    '[VitShellLaunch] workspace.snapshot_applied {"launch_id":"current","workspace_instance_id":"9"}',
    '[VitShellLaunch] readiness.decided {"launch_id":"current","workspace_instance_id":"9","ok":true,"stage":"workspace_ready"}',
  ].join("\n");
  const parsed = parseGodotReadiness(fixture, "current");
  if (parsed.length !== 3 || parsed.some(e => e.payload.launch_id !== "current") || parsed[0].lineIndex !== 2 || parsed[2].payload.stage !== "workspace_ready" || parseGodotReadiness(fixture, "wrong").length !== 0) throw new Error("readiness parser nonce/order self-test failed");
  const event = (seq, extra = {}) => ({ seq, event: 0x8000, target_hwnd: "20", target_pid: 77, event_thread: 1, event_time_ms: 100 + seq,
    received_tick_ms: 100 + seq, object_id: 0, child_id: 0, launch_pid: 77, launch_generation: 1, launch_id: "current", guard_active_at_receive: true, ...extra });
  const rawFixture = (events, extra = {}) => ({ schema: "vitshell.window_state.v1", observed_at_tick_ms: 220, direct_child_pid: 77, daw_hwnd: "20",
    win_event_log: { schema: "vitshell.win_event_log.v1", launch_id: "current", guard_started_tick_ms: 100, guard_released_tick_ms: 200,
      guard_started_seq: 1, guard_released_seq: 2, last_seq: events.at(-1)?.seq || 2, dropped_count: 90, truncated: true, events, ...extra } });
  const makeSamples = (window) => [{ index: 0, status: { window_state: { observed_at_tick_ms: 90 } } }, { index: 1, status: { window_state: window } }];
  const normalRaw = evaluateRawWindowEvents(makeSamples(rawFixture([event(1), event(2)])), [], "current");
  if (normalRaw.violation || normalRaw.missing_sequences.length || normalRaw.covered_sequence_count !== 2) throw new Error("historic drop/complete current union self-test failed");
  const late = evaluateRawWindowEvents(makeSamples(rawFixture([event(1), event(2), event(3, { event: 3, event_time_ms: 150, received_tick_ms: 210, guard_active_at_receive: false })])), [], "current");
  if (!late.violation || late.late_guard_events.length !== 1) throw new Error("delayed foreground callback escaped occurrence-time guard self-test");
  const zeroPid = evaluateRawWindowEvents(makeSamples(rawFixture([event(1, { event: 3, target_pid: 0 }), event(2)])), [], "current");
  if (!zeroPid.violation) throw new Error("zero-PID known DAW HWND was ignored");
  let gapRejected = false;
  try { evaluateRawWindowEvents(makeSamples(rawFixture([event(2)])), [], "current"); } catch { gapRejected = true; }
  if (!gapRejected) throw new Error("missing native sequence was accepted as zero activation");
  if (reconstructEventTick(0xfffffffe, 0x100000005) !== 0xfffffffe || reconstructEventTick(2, 0x100000005) !== 0x100000002) throw new Error("DWORD wrap reconstruction failed");
  // The permanently retained first-normal current-state counterexample remains
  // a failure; a later good sample must not overwrite the first bad fact.
  const firstNormal = [{ loading_guard_active: true, foreground: "daw", shell_above_daw: false, daw_noactivate: false },
    { loading_guard_active: true, foreground: "shell", shell_above_daw: true, daw_noactivate: true }];
  if (!firstNormal.some(w => w.foreground === "daw" || w.shell_above_daw === false || w.daw_noactivate === false)) throw new Error("first-normal counterexample was erased");
  const iconicFixture = [{ daw_visible: true, daw_iconic: true, daw_noactivate: false }, { daw_visible: true, daw_iconic: false, daw_noactivate: true }];
  const ordinaryFixture = iconicFixture.filter(w => w.daw_visible === true && w.daw_iconic === false);
  if (ordinaryFixture.length !== 1 || ordinaryFixture[0].daw_noactivate !== true) throw new Error("iconic was incorrectly treated as ordinary visible window");
  console.log("PRESENTATION_STATIC_SELF_TEST_PASS (no browser/native/HTTP)");
  process.exit(0);
}

const browser = await chromium
  .connectOverCDP("http://127.0.0.1:" + cdpPort, { timeout: 20000 })
  .catch((error) => {
    fail("connectOverCDP failed: " + error.message);
  });

try {
  const page = await waitBridge(browser);
  if (!page) fail("no page found over CDP (WebView2 not reachable?)");

  const url = page.url();
  let presentationCapture = null;
  if (windowChecks && ["launcher-chat-new", "presentation-launch"].includes(phase)) presentationCapture = await beginPresentationCapture(page);
  switch (phase) {
    case "presentation-launch": {
      await page.locator('[data-launcher-root][data-launcher-state="ready"]').waitFor({ state: "visible", timeout: 30000 });
      await page.locator("[data-launcher-choose-file]").click();
      const verdict = { phase, ok: true, url, opened_via: "existing file-dialog smoke bypass" };
      if (outPath) writeFileSync(outPath, JSON.stringify(verdict, null, 2));
      break;
    }
    case "management-roundtrip": {
      // 388/401 optional phase: no chat, config save, native window control, or
      // lifecycle operation. Kernel/PID/telemetry interval checked by ps1.
      const sentinel = await readSentinel(page);
      const input = page.locator("[data-project-composer] input, [data-project-composer] textarea");
      await input.waitFor({ state: "visible", timeout: 45000 });
      const previousInput = await input.inputValue();
      const marker = "management smoke unsent input";
      const screenshots = [];
      const requests = [];
      let mainFrameNavigations = 0;
      const onNavigation = (frame) => { if (frame === page.mainFrame()) mainFrameNavigations++; };
      const onRequest = (request) => requests.push({ path: new URL(request.url()).pathname, method: request.method() });
      page.on("request", onRequest);
      page.on("framenavigated", onNavigation);
      const recentPattern = "**/agent/launcher/recent-projects";
      const readRecent = () => page.evaluate(async () => {
        const response = await fetch("/agent/launcher/recent-projects");
        return { status: response.status, body: await response.json() };
      });
      const assertSettings = async (selector) => {
        const button = page.locator(selector);
        const atBottomLeft = await button.evaluate((el) => { const r = el.getBoundingClientRect(); return r.x < innerWidth / 3 && r.bottom > innerHeight - 80 && r.bottom <= innerHeight; });
        if (!atBottomLeft) throw new Error(`settings not bottom left: ${selector}`);
        await button.click();
        await page.locator("[data-entry-settings][open] .settings-page").waitFor({ state: "visible" });
        await page.locator("[data-entry-settings] .settings-body").waitFor({ state: "visible" });
        await page.keyboard.press("Escape");
        if (!await button.evaluate((el) => document.activeElement === el)) throw new Error("settings focus not returned");
      };
      const openManagement = async () => {
        await page.locator("[data-project-view-launcher]").click();
        await page.locator('[data-launcher-mode="project-management"]').waitFor({ state: "visible" });
      };
      try {
        await assertSettings("[data-project-settings]");
        await input.fill(marker);
        const contextName = await page.locator("[data-project-context-name]").getAttribute("data-project-context-name");
        await openManagement();
        await page.locator('[data-launcher-drawer][data-drawer-state="ready"] [data-launcher-recent-open]').first().waitFor({ state: "visible", timeout: 45000 });
        await page.locator('[data-launcher-root][data-launcher-state="ready"]').waitFor({ state: "visible", timeout: 45000 });
        if (!await page.locator("[data-launcher-new]").isEnabled() || !await page.locator("[data-launcher-choose-file]").isEnabled()) throw new Error("management new/open unavailable");
        const activeName = (await page.locator("[data-launcher-active-project-name]").innerText()).trim();
        if (!contextName || activeName !== contextName) throw new Error("management active project name mismatch");
        if (await page.locator("[data-launcher-input], [data-launcher-suggestion]").count()) throw new Error("management exposes current-project chat through launcher");
        await assertSettings("[data-launcher-settings]");
        const recent = await readRecent();
        if (recent.status !== 200 || recent.body.status !== "ok" || !Array.isArray(recent.body.project_result_cards) || recent.body.project_result_cards.length < 1) throw new Error("real recent GET did not return seeded cards");
        const healthy = recent.body.project_result_cards.find((card) => card.exists === true && card.file_path?.includes("fixture_a"));
        if (!healthy || !healthy.launcher_directive?.requires_governance) throw new Error("fixture_a governed recent card missing");
        const method = await page.evaluate(async () => { const response = await fetch("/agent/launcher/recent-projects", { method: "POST" }); return response.status; });
        if (method !== 405) throw new Error("recent POST not rejected");
        await page.locator("[data-launcher-return-project]").click();
        if (await input.inputValue() !== marker) throw new Error("project input lost in pure view roundtrip");
        const viewOnlyResponse = await page.request.get(statusUrl);
        if (!viewOnlyResponse.ok()) throw new Error("pure view status unavailable");
        const viewOnlyStatus = await viewOnlyResponse.json();
        await openManagement();
        await page.locator('[data-launcher-root][data-launcher-state="ready"]').waitFor({ state: "visible", timeout: 45000 });
        await page.locator('[data-launcher-drawer][data-drawer-state="ready"] [data-launcher-recent-open]').first().waitFor({ state: "visible", timeout: 45000 });
        const canceledRequests = [];
        const cancelGovernedRequest = async () => {
          const cancel = page.locator("[data-launcher-governance-cancel]").last();
          await cancel.waitFor({ state: "visible", timeout: 30000 });
          const requestId = await page.locator("[data-launcher-governance]").last().getAttribute("data-launcher-governance-request");
          if (!requestId || canceledRequests.includes(requestId)) throw new Error("governance request ID missing or reused");
          await cancel.click();
          await page.locator('[data-launcher-bridge-exec-state="canceled"]').waitFor({ state: "visible", timeout: 30000 });
          canceledRequests.push(requestId);
        };
        await page.locator("[data-launcher-new]").click();
        await cancelGovernedRequest();
        await page.locator("[data-launcher-choose-file]").click();
        await cancelGovernedRequest();
        await page.locator(`[data-launcher-recent-open=${JSON.stringify(healthy.id)}]`).click();
        await cancelGovernedRequest();
        if (outPath) { const screenshot = outPath + ".management.png"; await page.screenshot({ path: screenshot }); screenshots.push(screenshot); }
        await page.locator("[data-launcher-return-project]").click();
        if (await input.inputValue() !== marker) throw new Error("project input lost during management roundtrip");
        // Synthetic failure injection tests presentation ONLY. Real GET above
        // is independently required; do not claim a real kernel failure here.
        await page.route(recentPattern, (route) => route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ status: "error", kernel_unavailable: true, error: "synthetic failure injection: recent GET503" }) }));
        await openManagement();
        await page.locator("[data-launcher-drawer-error]").waitFor({ state: "visible", timeout: 30000 });
        if (await page.locator("[data-launcher-drawer-empty]").count()) throw new Error("GET503 rendered as empty recents");
        await page.unroute(recentPattern);
        await page.locator("[data-launcher-drawer-retry]").click();
        await page.locator('[data-launcher-drawer][data-drawer-state="ready"] [data-launcher-recent-open]').first().waitFor({ state: "visible", timeout: 45000 });
        await page.locator("[data-launcher-return-project]").click();
        await input.fill(previousInput);
        if (await readSentinel(page) !== sentinel || !sentinel) throw new Error("management reloaded page");
        if (mainFrameNavigations !== 0) throw new Error("management navigated main frame");
        if (requests.some((request) => request.path === "/agent/chat" || request.path === "/agent/config" && request.method !== "GET")) throw new Error("management sent chat/config mutation");
        const verdict = { phase, ok: true, url, sentinel, main_frame_navigations: mainFrameNavigations, context_name: contextName, view_only_status: viewOnlyStatus, canceled_request_ids: canceledRequests, real_get: recent, method_rejected: method, synthetic_failure_injection: "GET503 presentation only", screenshots, requests };
        if (outPath) writeFileSync(outPath, JSON.stringify(verdict, null, 2));
        console.log("CDP_OK " + JSON.stringify(verdict));
      } finally {
        page.off("request", onRequest);
        page.off("framenavigated", onNavigation);
        await page.unroute(recentPattern);
      }
      break;
    }
    case "probe-launcher": {
      const root = page.locator("[data-launcher-root]");
      await root.waitFor({ state: "visible", timeout: 30000 });
      await page.locator("[data-launcher-new]").waitFor({ state: "visible", timeout: 15000 });
      await page.locator("[data-launcher-choose-file]").waitFor({ state: "visible", timeout: 15000 });
      const sentinel = await withSentinel(page);
      const newBtnText = (await page.locator("[data-launcher-new]").textContent()) ?? "";
      const openBtnText = (await page.locator("[data-launcher-choose-file]").textContent()) ?? "";
      const verdict = {
        phase, ok: true, url,
        sentinel,
        drawer_new: newBtnText.trim(),
        drawer_open: openBtnText.trim()
      };
      if (outPath) writeFileSync(outPath, JSON.stringify(verdict, null, 2));
      console.log("CDP_OK " + JSON.stringify(verdict));
      break;
    }
    case "probe-launcher-data": {
      // SHELL-3 leg: main window shows REAL agent data (live greeting endpoint),
      // not the fixture face. Root state must be ready, bridge available, no
      // fixture marker, greeting text rendered, assistant thread seeded.
      const root = page.locator("[data-launcher-root]");
      await root.waitFor({ state: "visible", timeout: 30000 });
      await page.locator('[data-launcher-root][data-launcher-state="ready"]').waitFor({ state: "visible", timeout: 30000 });
      const rootAttrs = await page.locator("[data-launcher-root]").evaluate((el) => ({
        state: el.getAttribute("data-launcher-state"),
        bridge: el.getAttribute("data-launcher-bridge"),
        fixture: el.getAttribute("data-fixture")
      }));
      if (rootAttrs.bridge !== "available") {
        fail("launcher bridge not available (got " + rootAttrs.bridge + ")");
      }
      if (rootAttrs.fixture) {
        fail("fixture marker present on live launcher face (" + rootAttrs.fixture + ")");
      }
      const greetingEl = page.locator("[data-launcher-greeting]");
      await greetingEl.waitFor({ state: "visible", timeout: 30000 });
      const greetingText = ((await greetingEl.textContent()) ?? "").trim();
      if (!greetingText) fail("greeting text empty (live /agent/launcher/greeting data missing)");
      const threadEl = page.locator("[data-launcher-thread]");
      await threadEl.waitFor({ state: "visible", timeout: 15000 });
      const assistantMsgs = await page.locator('[data-launcher-thread] [data-launcher-msg="assistant"], [data-launcher-thread] .message-row.assistant').count();
      if (assistantMsgs < 1) fail("no assistant thread frame under launcher thread (greeting not seeded)");
      const recentsCount = await page.locator("[data-launcher-recents-count]").getAttribute("data-launcher-recents-count");
      const sentinel = await withSentinel(page);
      const verdict = {
        phase, ok: true, url, sentinel,
        launcher_state: rootAttrs.state,
        bridge: rootAttrs.bridge,
        greeting_text: greetingText,
        assistant_frames: assistantMsgs,
        recents_count: recentsCount
      };
      if (outPath) writeFileSync(outPath, JSON.stringify(verdict, null, 2));
      console.log("CDP_OK " + JSON.stringify(verdict));
      break;
    }
    case "probe-launcher-kernel-online": {
      // SHELL-4 leg: launcher stage with kernel ONLINE (spawned by the agent
      // at startup — kernel-ready-at-start ruling). Root must carry the online
      // kernel mark, greeting must NOT be the kernel-unavailable template, and
      // the drawer must show real seeded recents (ready state, >=1 rows).
      const root = page.locator("[data-launcher-root]");
      await root.waitFor({ state: "visible", timeout: 30000 });
      await page.locator('[data-launcher-root][data-launcher-state="ready"]').waitFor({ state: "visible", timeout: 30000 });
      await page.locator('[data-launcher-root][data-launcher-kernel="online"]').waitFor({ state: "visible", timeout: 15000 })
        .catch(() => {
          fail("launcher kernel mark not online (kernel spawn/attach by agent missing?)");
        });
      const greetingText = ((await page.locator("[data-launcher-greeting]").textContent()) ?? "").trim();
      if (greetingText.includes("连不上内核")) {
        fail("greeting is the kernel-unavailable template: " + greetingText);
      }
      await page.locator('[data-launcher-drawer][data-drawer-state="ready"]').waitFor({ state: "visible", timeout: 20000 })
        .catch(() => {
          fail("recents drawer not ready (kernel online but real recents missing)");
        });
      const recentsRows = await page.locator("[data-launcher-recent]").count();
      if (recentsRows < 1) {
        fail("drawer ready but zero recent rows (seeded recents not consumed)");
      }
      const recentsCount = await page.locator("[data-launcher-recents-count]").getAttribute("data-launcher-recents-count");
      const kernelAttr = await page.locator("[data-launcher-root]").getAttribute("data-launcher-kernel");
      const verdict = {
        phase, ok: true, url,
        kernel: kernelAttr,
        greeting_text: greetingText,
        recents_rows: recentsRows,
        recents_count: recentsCount
      };
      if (outPath) writeFileSync(outPath, JSON.stringify(verdict, null, 2));
      console.log("CDP_OK " + JSON.stringify(verdict));
      break;
    }
    case "launcher-chat-new": {
      // SHELL-4 leg, SHELL-5 semantics: new-project chain END-TO-END through the
      // launcher chat. Type the new-project message (named when --new-project-name
      // is given, else the bare "新建工程" — the exact user retest criterion: the
      // name-less intent must auto-execute with the default draft name, NOT enter
      // a question loop), send, and assert the directive placeholder appears AND
      // auto-executes (bridge exec block reaches waiting on its own — no manual
      // execute button click); if an in-page governance card appears (a project
      // is active), answer 直接切换.
      const root = page.locator("[data-launcher-root]");
      await root.waitFor({ state: "visible", timeout: 30000 });
      const composer = page.locator('[data-launcher-input] input.v5-composer-input, [data-launcher-input] textarea');
      await composer.waitFor({ state: "visible", timeout: 15000 });
      const message = newProjectName ? ("新建工程 " + newProjectName) : "新建工程";
      await composer.fill(message, { timeout: 10000 });
      const sendBtn = page.locator('[data-launcher-input] .v5-composer-send, [data-launcher-input] .send-button');
      await sendBtn.waitFor({ state: "visible", timeout: 10000 });
      await sendBtn.click({ timeout: 10000 });
      // Deterministic launcher route: directive placeholder (auto-executing).
      const directive = page.locator("[data-launcher-directive]");
      await directive.waitFor({ state: "visible", timeout: 30000 })
        .catch(async () => {
          // Failure-scene evidence (SHELL-4 debug discipline): exact reply text,
          // input value, thread item count, busy/disabled state at failure time.
          const evidence = {};
          try {
            evidence.assistantMessages = (await page.locator('[data-launcher-thread] [data-launcher-msg="assistant"], [data-launcher-thread] .message-row.assistant').allTextContents()).map((s) => s.trim());
            evidence.userMessages = (await page.locator('[data-launcher-thread] [data-launcher-msg="user"], [data-launcher-thread] .message-row.user').allTextContents()).map((s) => s.trim());
            evidence.inputValue = await page.locator('[data-launcher-input] input.v5-composer-input, [data-launcher-input] textarea').inputValue().catch(() => "?");
            evidence.inputDisabled = await page.locator('[data-launcher-input] input.v5-composer-input, [data-launcher-input] textarea').isDisabled().catch(() => "?");
            evidence.inputState = await page.locator("[data-launcher-input]").getAttribute("data-input-state").catch(() => "?");
            evidence.bridgeExecState = await page.locator("[data-launcher-bridge-exec]").getAttribute("data-launcher-bridge-exec-state").catch(() => "absent");
            evidence.governanceCount = await page.locator("[data-launcher-governance]").count();
          } catch (evidenceError) {
            evidence.collectionError = String(evidenceError);
          }
          fail("no directive placeholder after new-project message (launcher intent gate closed or agent route missing)", evidence);
        });
      const headline = ((await page.locator("[data-launcher-directive] .v5-launcher-directive-headline").textContent()) ?? "").trim();
      if (newProjectName && !headline.includes(newProjectName)) {
        fail("directive headline does not carry the new project name (headline=" + headline + ")");
      }
      if (!newProjectName && !headline.includes("新工程_")) {
        fail("name-less new intent must carry the default draft name (headline=" + headline + ")");
      }
      const replyText = ((await page.locator('[data-launcher-thread] [data-launcher-msg="assistant"], [data-launcher-thread] .message-row.assistant').last().textContent()) ?? "").trim();
      if (replyText.includes("连不上内核")) {
        fail("new-project reply is the kernel-unavailable template: " + replyText);
      }
      if (replyText.includes("想叫什么名字")) {
        fail("name-less new intent fell back into the question loop (SHELL-5 removed it): " + replyText);
      }
      // SHELL-5: NO manual execute button — the directive must AUTO-execute. The
      // bridge exec block reaching waiting on its own is the production default
      // now; a visible manual execute button is a regression.
      const manualBtn = await page.locator("[data-launcher-directive-execute]").count();
      if (manualBtn > 0) {
        fail("manual execute button still present (SHELL-5 auto-execute regressed)");
      }
      let bridgeExecState = "absent";
      try {
        await page.locator("[data-launcher-bridge-exec]").waitFor({ state: "visible", timeout: 10000 });
        bridgeExecState = (await page.locator("[data-launcher-bridge-exec]").getAttribute("data-launcher-bridge-exec-state")) ?? "?";
      } catch {
        bridgeExecState = "absent";
      }
      if (bridgeExecState === "absent") {
        fail("directive did not auto-execute (bridge exec block never appeared — auto-execute seam not live)");
      }
      // Governance may appear when a project is active; answer 直接切换 via the
      // real bridge path (mirrors the smoke governance hook, production zero).
      let governance = "none";
      try {
        await page.locator("[data-launcher-governance-actions]").waitFor({ state: "visible", timeout: 6000 });
        governance = "switch";
        await page.locator("[data-launcher-governance-switch]").click({ timeout: 10000 });
      } catch {
        governance = "none";
      }
      const verdict = {
        phase, ok: true, url,
        message,
        new_project_name: newProjectName || "(default)",
        directive_headline: headline,
        assistant_reply: replyText,
        auto_execute: true,
        bridge_exec_state_seen: bridgeExecState,
        governance
      };
      if (outPath) writeFileSync(outPath, JSON.stringify(verdict, null, 2));
      console.log("CDP_OK " + JSON.stringify(verdict));
      break;
    }
    case "launcher-button-new": {
      // SHELL-5 leg: drawer "＋新建" = DIRECT new (no chat round): click the real
      // button, assert the bridge exec block appears (direct directive posted),
      // answer the in-page governance card if shown (a project is active).
      const root = page.locator("[data-launcher-root]");
      await root.waitFor({ state: "visible", timeout: 30000 });
      const newBtn = page.locator("[data-launcher-new]");
      await newBtn.waitFor({ state: "visible", timeout: 15000 });
      await newBtn.click({ timeout: 10000 });
      // Direct directive: bridge exec block must appear WITHOUT any chat turn.
      let bridgeExecState = "absent";
      try {
        await page.locator("[data-launcher-bridge-exec]").waitFor({ state: "visible", timeout: 10000 });
        bridgeExecState = (await page.locator("[data-launcher-bridge-exec]").getAttribute("data-launcher-bridge-exec-state")) ?? "?";
      } catch {
        bridgeExecState = "absent";
      }
      if (bridgeExecState === "absent") {
        fail("＋新建 button did not post a direct new_project directive (bridge exec block absent)");
      }
      // No chat question loop: the button must not send a chat message (no new
      // assistant preset reply asking for a name).
      const assistantMsgs = (await page.locator('[data-launcher-thread] [data-launcher-msg="assistant"], [data-launcher-thread] .message-row.assistant').allTextContents()).map((s) => s.trim());
      const lastAssistant = assistantMsgs[assistantMsgs.length - 1] ?? "";
      if (lastAssistant.includes("想叫什么名字")) {
        fail("＋新建 button went through the chat question loop (SHELL-5 direct semantics broken)");
      }
      let governance = "none";
      try {
        await page.locator("[data-launcher-governance-actions]").waitFor({ state: "visible", timeout: 6000 });
        governance = "switch";
        await page.locator("[data-launcher-governance-switch]").click({ timeout: 10000 });
      } catch {
        governance = "none";
      }
      const verdict = {
        phase, ok: true, url,
        clicked: "data-launcher-new",
        direct_mode: true,
        bridge_exec_state_seen: bridgeExecState,
        last_assistant_reply: lastAssistant,
        governance
      };
      if (outPath) writeFileSync(outPath, JSON.stringify(verdict, null, 2));
      console.log("CDP_OK " + JSON.stringify(verdict));
      break;
    }
    case "probe-launcher-no-binding": {
      // SHELL-5 leg: launcher stage = ZERO workspace binding on the agent face.
      // /agent/state current_project.path must be empty (the kernel boot draft
      // must not be bound as the agent's workspace). Read via the live page's
      // fetch (same origin as the agent).
      const state = await page.evaluate(async () => {
        try {
          const r = await fetch("/agent/state");
          const j = await r.json();
          return {
            status: r.status,
            current_project_path: String(j.current_project?.path ?? ""),
            current_project_uuid: String(j.current_project?.uuid ?? "")
          };
        } catch (e) {
          return { error: String(e) };
        }
      });
      if (state.error) {
        fail("/agent/state unreachable from launcher page: " + state.error);
      }
      const verdict = {
        phase, ok: true, url,
        current_project_path: state.current_project_path,
        current_project_uuid: state.current_project_uuid
      };
      if (outPath) writeFileSync(outPath, JSON.stringify(verdict, null, 2));
      console.log("CDP_OK " + JSON.stringify(verdict));
      break;
    }
    case "click-open": {
      const button = page.locator("[data-launcher-choose-file]");
      await button.waitFor({ state: "visible", timeout: 15000 });
      await button.click({ timeout: 10000 });
      const verdict = { phase, ok: true, url, clicked: "data-launcher-choose-file" };
      if (outPath) writeFileSync(outPath, JSON.stringify(verdict, null, 2));
      console.log("CDP_OK " + JSON.stringify(verdict));
      break;
    }
    case "probe-project": {
      await page.locator("[data-launcher-root]").waitFor({ state: "detached", timeout: 30000 });
      const sentinel = await readSentinel(page);
      const verdict = { phase, ok: true, url, sentinel, launcher_root_gone: true };
      if (!sentinel) { fail("sentinel missing on project surface (reload suspected)"); break; }
      if (outPath) writeFileSync(outPath, JSON.stringify(verdict, null, 2));
      console.log("CDP_OK " + JSON.stringify(verdict));
      break;
    }
    case "probe-project-ready": {
      // SHELL-3 leg: project surface (346) mounted with loaded context (not the
      // fixture face): data-project-state=ready and the context name present.
      const root = page.locator("[data-project-root]");
      await root.waitFor({ state: "visible", timeout: 45000 });
      await page.locator('[data-project-root][data-project-state="ready"]').waitFor({ state: "visible", timeout: 45000 })
        .catch(async () => {
          // Failure-scene evidence (SHELL-4 debug discipline).
          const evidence = {};
          try {
            evidence.projectState = await page.locator("[data-project-root]").getAttribute("data-project-state");
            evidence.contextName = await page.locator("[data-project-context-name]").textContent().catch(() => "?");
            evidence.contextLoading = await page.locator("[data-project-context-loading]").count();
            evidence.contextError = await page.locator("[data-project-context-error]").textContent().catch(() => "");
            evidence.launcherRootStill = await page.locator("[data-launcher-root]").count();
            evidence.loadingCard = await page.locator("[data-loading-card]").count().catch(() => -1);
            evidence.uiStateRaw = await page.evaluate(async () => {
              try {
                const r = await fetch("/agent/ui/state");
                return { status: r.status, body: (await r.text()).slice(0, 400) };
              } catch (e) { return { error: String(e) }; }
            });
          } catch (e2) { evidence.collectionError = String(e2); }
          fail("project surface not ready in 45s", evidence);
        });
      const rootAttrs = await page.locator("[data-project-root]").evaluate((el) => ({
        state: el.getAttribute("data-project-state"),
        source: el.getAttribute("data-project-context-source"),
        fixture: el.getAttribute("data-fixture")
      }));
      if (rootAttrs.fixture) {
        fail("fixture marker present on live project face (" + rootAttrs.fixture + ")");
      }
      const nameEl = page.locator("[data-project-titlebar-name]");
      // SHELL-6: --expect-name <substr> — after an in-page project switch the
      // surface stays mounted (no remount); wait for the titlebar context name
      // to become the expected one (context refresh on bridge result opened).
      if (expectName) {
        await page.waitForFunction(
          (needle) => {
            const el = document.querySelector('[data-project-titlebar] [data-project-context-name]');
            return Boolean(el && el.textContent && el.textContent.includes(needle));
          },
          expectName,
          { timeout: 45000 }
        ).catch(async () => {
          const got = ((await page.locator('[data-project-titlebar] [data-project-context-name]').textContent().catch(() => "?")) ?? "").trim();
          fail("project context name did not become " + JSON.stringify(expectName) + " in 45s (got " + JSON.stringify(got) + " — context refresh after in-page switch missing?)");
        });
      }
      const contextName = await page.locator("[data-project-titlebar] [data-project-context-name]").textContent();
      const sentinel = await readSentinel(page);
      if (!sentinel) { fail("sentinel missing on project surface (reload suspected)"); break; }
      const verdict = {
        phase, ok: true, url, sentinel,
        project_state: rootAttrs.state,
        context_source: rootAttrs.source,
        context_name: (contextName ?? "").trim()
      };
      if (outPath) writeFileSync(outPath, JSON.stringify(verdict, null, 2));
      console.log("CDP_OK " + JSON.stringify(verdict));
      break;
    }
    case "chat-roundtrip": {
      // SHELL-3 leg: REAL in-project conversation roundtrip. Type into the
      // live composer, send via the real send button, then assert BOTH frames
      // in [data-project-thread]: the user frame and an assistant ("vit")
      // frame. The assistant content is whatever the real pipeline returns
      // (with the dead-endpoint LLM override the agent's deterministic
      // fail-safe answers; the transport + surface pipeline are real).
      const composer = page.locator('[data-project-composer] input.v5-composer-input');
      await composer.waitFor({ state: "visible", timeout: 30000 });
      const beforeUser = await page.locator('[data-project-thread] [data-message-author="user"]').count();
      const beforeVit = await page.locator('[data-project-thread] [data-message-author="vit"]').count();
      await composer.fill(chatText, { timeout: 10000 });
      const sendBtn = page.locator('[data-project-composer] .v5-composer-send');
      await sendBtn.waitFor({ state: "visible", timeout: 10000 });
      await sendBtn.click({ timeout: 10000 });
      // User frame appears synchronously on dispatch.
      await page.waitForFunction(
        ([b, sel]) => document.querySelectorAll(sel).length > b,
        [beforeUser, '[data-project-thread] [data-message-author="user"]'],
        { timeout: 15000 }
      ).catch(() => {});
      const afterUser = await page.locator('[data-project-thread] [data-message-author="user"]').count();
      if (afterUser <= beforeUser) {
        fail("no user frame appended to project thread after send (before=" + beforeUser + " after=" + afterUser + ")");
      }
      // Assistant frame arrives from the real /agent/chat roundtrip.
      await page.waitForFunction(
        ([b, sel]) => document.querySelectorAll(sel).length > b,
        [beforeVit, '[data-project-thread] [data-message-author="vit"]'],
        { timeout: chatTimeoutMs }
      ).catch(() => {
        fail("no assistant frame appended to project thread within " + chatTimeoutMs + "ms (agent roundtrip missing)");
      });
      const userTexts = await page.locator('[data-project-thread] [data-message-author="user"]').allTextContents();
      const vitTexts = await page.locator('[data-project-thread] [data-message-author="vit"]').allTextContents();
      const lastVit = (vitTexts[vitTexts.length - 1] ?? "").trim();
      if (!lastVit) fail("assistant frame present but empty text");
      // Reply composer must settle (not stuck in replying state).
      const composerState = await page.locator("[data-project-composer-state]").getAttribute("data-project-composer-state")
        ?? await page.locator('[data-project-composer] [data-input-state]').getAttribute("data-input-state");
      const sentinel = await readSentinel(page);
      if (!sentinel) { fail("sentinel missing after chat roundtrip (reload suspected)"); break; }
      const verdict = {
        phase, ok: true, url, sentinel,
        chat_text: chatText,
        user_frames: afterUser,
        vit_frames: vitTexts.length,
        assistant_reply: lastVit,
        composer_state: composerState
      };
      if (outPath) writeFileSync(outPath, JSON.stringify(verdict, null, 2));
      console.log("CDP_OK " + JSON.stringify(verdict));
      break;
    }
    case "project-chat-new": {
      // SHELL-6 leg: IN-PROJECT new-project chain end-to-end (the exact user
      // retest criterion #3). Type the bare "新建工程" in the PROJECT surface
      // composer, send, and assert:
      //   1) the deterministic project-face intent route answers with a
      //      directive placeholder (NOT an LLM free-text reply),
      //   2) the directive AUTO-executes (bridge exec block appears on its
      //      own — no manual execute button),
      //   3) the in-page governance 3-key card appears IN THE PROJECT SURFACE
      //      (an in-project switch has a current project => host governance
      //      guaranteed) — THE SHELL-6 invariant: confirmation semantics must
      //      come with a clickable face; text-only confirmation = defect,
      //   4) clicking 直接切换 answers through the real bridge path and the
      //      card fixes to the answered face (no replay of the three keys).
      const root = page.locator("[data-project-root]");
      await root.waitFor({ state: "visible", timeout: 30000 });
      const composer = page.locator('[data-project-composer] input.v5-composer-input');
      await composer.waitFor({ state: "visible", timeout: 15000 });
      const beforeUser = await page.locator('[data-project-thread] [data-message-author="user"]').count();
      await composer.fill("新建工程", { timeout: 10000 });
      const sendBtn = page.locator('[data-project-composer] .v5-composer-send');
      await sendBtn.waitFor({ state: "visible", timeout: 10000 });
      await sendBtn.click({ timeout: 10000 });
      await page.waitForFunction(
        ([b, sel]) => document.querySelectorAll(sel).length > b,
        [beforeUser, '[data-project-thread] [data-message-author="user"]'],
        { timeout: 15000 }
      ).catch(() => {});
      // Deterministic project-face route (SHELL-6): directive placeholder.
      const directive = page.locator("[data-project-directive]");
      await directive.waitFor({ state: "visible", timeout: 30000 })
        .catch(async () => {
          // Failure-scene evidence: the 22:49 defect shape was an LLM free-text
          // confirmation with no interactive face.
          const evidence = {};
          try {
            evidence.assistantMessages = (await page.locator('[data-message-author="vit"]').allTextContents()).map((s) => s.trim());
            evidence.bridgeExecState = await page.locator("[data-project-bridge-exec]").getAttribute("data-project-bridge-exec-state").catch(() => "absent");
            evidence.confirmCards = await page.locator("[data-project-confirm]").count();
            evidence.governanceCards = await page.locator("[data-project-governance]").count();
            evidence.bridgeAttr = await page.locator("[data-project-root]").getAttribute("data-project-bridge");
          } catch (evidenceError) {
            evidence.collectionError = String(evidenceError);
          }
          fail("no directive placeholder after in-project 新建工程 (project-face intent route missing — 22:49 defect shape)", evidence);
        });
      const headline = ((await page.locator("[data-project-directive] .v5-project-directive-headline").textContent()) ?? "").trim();
      if (!headline.includes("新工程_")) {
        fail("in-project new directive headline must carry the default draft name (headline=" + headline + ")");
      }
      const replyText = ((await page.locator('[data-message-author="vit"]').last().textContent()) ?? "").trim();
      if (replyText.includes("连不上内核")) {
        fail("in-project new reply is the kernel-unavailable template: " + replyText);
      }
      // Auto-execute: the bridge exec block reaching waiting on its own is the
      // production default; a manual execute button would be a regression.
      let bridgeExecState = "absent";
      try {
        await page.locator("[data-project-bridge-exec]").waitFor({ state: "visible", timeout: 10000 });
        bridgeExecState = (await page.locator("[data-project-bridge-exec]").getAttribute("data-project-bridge-exec-state")) ?? "?";
      } catch {
        bridgeExecState = "absent";
      }
      if (bridgeExecState === "absent") {
        fail("in-project directive did not auto-execute (bridge exec block never appeared — auto-execute seam not live on project face)");
      }
      // THE SHELL-6 invariant leg: the governance card MUST appear IN THE
      // PROJECT SURFACE and MUST be clickable (直接切换 via the real bridge).
      await page.locator("[data-project-governance-actions]").waitFor({ state: "visible", timeout: 20000 })
        .catch(async () => {
          const evidence = {};
          try {
            evidence.governanceCount = await page.locator("[data-project-governance]").count();
            evidence.bridgeExecState = await page.locator("[data-project-bridge-exec]").getAttribute("data-project-bridge-exec-state").catch(() => "absent");
            evidence.bridgeExecText = ((await page.locator("[data-project-bridge-exec]").textContent().catch(() => "")) ?? "").trim();
            evidence.assistantMessages = (await page.locator('[data-message-author="vit"]').allTextContents()).map((s) => s.trim());
          } catch (evidenceError) {
            evidence.collectionError = String(evidenceError);
          }
          fail("governance card did NOT appear in the project surface (text-only confirmation = the SHELL-6 defect shape)", evidence);
        });
      const govHeadline = ((await page.locator("[data-project-governance-headline]").textContent()) ?? "").trim();
      await page.locator("[data-project-governance-switch]").click({ timeout: 10000 });
      let answered = "";
      try {
        await page.locator("[data-project-governance-answered-text]").waitFor({ state: "visible", timeout: 4000 });
        answered = ((await page.locator("[data-project-governance-answered-text]").textContent()) ?? "").trim();
      } catch {
        answered = "transient_missed";
      }
      const verdict = {
        phase, ok: true, url,
        message: "新建工程",
        directive_headline: headline,
        assistant_reply: replyText,
        auto_execute: true,
        bridge_exec_state_seen: bridgeExecState,
        governance: "switch",
        governance_headline: govHeadline,
        governance_answered: answered
      };
      if (outPath) writeFileSync(outPath, JSON.stringify(verdict, null, 2));
      console.log("CDP_OK " + JSON.stringify(verdict));
      break;
    }
    case "probe-error-page": {
      // SHELL-3 leg: in-window error page (agent failed to start / port taken).
      // Embedded host HTML: title "Vit - error", detail in .d, retry/exit buttons.
      await page.waitForFunction(
        () => document.title === "Vit - error",
        { timeout: 30000 }
      ).catch(() => {
        fail("error page not reached (document.title=" + JSON.stringify(page.url() && "see log") + ")");
      });
      const detail = ((await page.locator(".d").textContent()) ?? "").trim();
      if (!detail) fail("error page detail (.d) empty");
      const buttons = await page.locator("button").allTextContents();
      const hasRetry = buttons.some((t) => t.includes("重试"));
      const hasExit = buttons.some((t) => t.includes("退出"));
      if (!hasRetry || !hasExit) {
        fail("error page buttons missing (retry=" + hasRetry + " exit=" + hasExit + " found=" + JSON.stringify(buttons) + ")");
      }
      const verdict = {
        phase, ok: true, url,
        title: "Vit - error",
        detail,
        buttons: buttons.map((t) => t.trim())
      };
      if (outPath) writeFileSync(outPath, JSON.stringify(verdict, null, 2));
      console.log("CDP_OK " + JSON.stringify(verdict));
      break;
    }
    case "probe-launcher-back": {
      await page.locator("[data-launcher-root]").waitFor({ state: "visible", timeout: 30000 });
      const sentinel = await readSentinel(page);
      const chooseVisible = await page.locator("[data-launcher-choose-file]").isVisible().catch(() => false);
      const verdict = { phase, ok: true, url, sentinel, launcher_back: true, choose_visible: chooseVisible };
      if (!sentinel) { fail("sentinel missing after back-to-launcher (reload suspected)"); break; }
      if (outPath) writeFileSync(outPath, JSON.stringify(verdict, null, 2));
      console.log("CDP_OK " + JSON.stringify(verdict));
      break;
    }
    case "click-governance-switch": {
      const button = page.locator("[data-launcher-governance-switch]");
      await button.waitFor({ state: "visible", timeout: 30000 });
      await button.click({ timeout: 10000 });
      const verdict = { phase, ok: true, url, clicked: "data-launcher-governance-switch" };
      if (outPath) writeFileSync(outPath, JSON.stringify(verdict, null, 2));
      console.log("CDP_OK " + JSON.stringify(verdict));
      break;
    }
    case "assert-governance": {
      const card = page.locator("[data-launcher-governance]");
      await card.waitFor({ state: "visible", timeout: 30000 });
      const save = await page.locator("[data-launcher-governance-save]").textContent() ?? "";
      const switchBtn = await page.locator("[data-launcher-governance-switch]").textContent() ?? "";
      const cancel = await page.locator("[data-launcher-governance-cancel]").textContent() ?? "";
      const headline = (await page.locator("[data-launcher-governance-headline]").textContent()) ?? "";
      const verdict = {
        phase, ok: true, url,
        save: save.trim(), switch: switchBtn.trim(), cancel: cancel.trim(),
        headline: headline.trim()
      };
      if (outPath) writeFileSync(outPath, JSON.stringify(verdict, null, 2));
      console.log("CDP_OK " + JSON.stringify(verdict));
      break;
    }
    case "probe-governance-done": {
      let answered = "";
      try {
        await page.locator("[data-launcher-governance-answered-text]").waitFor({ state: "visible", timeout: 4000 });
        answered = (await page.locator("[data-launcher-governance-answered-text]").textContent()) ?? "";
      } catch {
        answered = "transient_missed";
      }
      await page.locator("[data-launcher-root]").waitFor({ state: "detached", timeout: 60000 });
      const sentinel = await readSentinel(page);
      const verdict = { phase, ok: true, url, answered: answered.trim(), sentinel };
      if (!sentinel) { fail("sentinel missing after governed open (reload suspected)"); break; }
      if (outPath) writeFileSync(outPath, JSON.stringify(verdict, null, 2));
      console.log("CDP_OK " + JSON.stringify(verdict));
      break;
    }
    default:
      fail("unknown phase: " + phase);
  }
  if (presentationCapture) {
    const evidence = await finishPresentationCapture(page, presentationCapture);
    const verdict = JSON.parse(readFileSync(outPath, "utf8"));
    verdict.presentation = { scenario: evidence.scenario, request_id: evidence.request_id, evidence: outPath + ".presentation.json" };
    writeFileSync(outPath, JSON.stringify(verdict, null, 2));
  }
} catch (error) {
  fail((error && error.message) ? error.message : String(error));
} finally {
  await browser.close().catch(() => {});
}

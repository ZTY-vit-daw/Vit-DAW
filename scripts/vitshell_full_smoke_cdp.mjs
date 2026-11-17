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
import { writeFileSync } from "node:fs";
import { join } from "node:path";

function arg(name, fallback = "") {
  const idx = process.argv.indexOf("--" + name);
  if (idx >= 0 && idx + 1 < process.argv.length) return process.argv[idx + 1];
  return fallback;
}

const cdpPort = arg("cdp-port", "9333");
const phase = arg("phase", "");
const outPath = arg("out", "");
const chatText = arg("chat-text", "这个工程现在是什么状态？");
const newProjectName = arg("new-project-name", "");
const expectName = arg("expect-name", "");
const chatTimeoutMs = parseInt(arg("chat-timeout-ms", "90000"), 10);
const webuiDir = arg("webui-dir", join(process.cwd(), "agent/webui"));
const requireFromWebui = createRequire(join(webuiDir, "package.json"));
const { chromium } = requireFromWebui("playwright");

function fail(message, evidence) {
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

const browser = await chromium
  .connectOverCDP("http://127.0.0.1:" + cdpPort, { timeout: 20000 })
  .catch((error) => {
    fail("connectOverCDP failed: " + error.message);
  });

try {
  const page = await waitBridge(browser);
  if (!page) fail("no page found over CDP (WebView2 not reachable?)");

  const url = page.url();
  switch (phase) {
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
        const cancelGovernedRequest = async () => {
          const cancel = page.locator("[data-launcher-governance-cancel]").last();
          await cancel.waitFor({ state: "visible", timeout: 30000 });
          await cancel.click();
          await page.locator('[data-launcher-bridge-exec-state="canceled"]').waitFor({ state: "visible", timeout: 30000 });
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
        const verdict = { phase, ok: true, url, sentinel, main_frame_navigations: mainFrameNavigations, context_name: contextName, real_get: recent, method_rejected: method, synthetic_failure_injection: "GET503 presentation only", screenshots, requests };
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
} catch (error) {
  fail((error && error.message) ? error.message : String(error));
} finally {
  await browser.close().catch(() => {});
}

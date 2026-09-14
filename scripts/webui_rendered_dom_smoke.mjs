#!/usr/bin/env node
// E2E-WEBUI-1 (2026-09-13): rendered-surface end-side smoke for the webui.
//
// Shape under test (AGENTS.md section 5, "rendered surface and user journey
// gate"): a real VitAgent process serves the real built webui bundle at
// /app/; a real browser opens it, and EVERY assertion below is made against
// the DOM the browser actually rendered -- not against a reducer, not against
// a jsdom render, not against a mock.
//
// Event source (card's option B, "inject the archived real event stream"):
// GET /agent/events is fulfilled at the network layer with the archived real
// response captured from the user's own session (artifacts/_forensic_ev.json,
// 2026-09-13 12:26-12:28, conversation webui_mtzba6wf). Nothing is synthesized
// and no page.fetch is patched: the app talks to the real agent process, and
// the response bytes are the forensic ones.
//
// Conversation context comes from the agent itself: the archived conversation
// graph (18 nodes, .vit_history of the 912.vit session) is seeded into the
// isolated draft history so the webui hydrates it through its real Project
// History path (GET /agent/ui/state -> project_history.conversation_messages).
// That path is what makes this reproduction faithful: the hydrated user
// message carries the server-side graph-node stamp, not the client's
// optimistic Date.now().
//
// Assertion groups (card):
//   A1 anchoring  -- trace block renders after its own turn's user message,
//                    not at the flow tail, and is not covered by .composer
//                    (viewport coordinates: block bottom vs composer top)
//   A2 freeze     -- a settled turn's step nodes carry no is-running/is-pending/
//                    is-live state and its duration label is the static form
//   A3 duration   -- residency split: "执行 X" / "等待续跑 Y" only when a
//                    residency segment exists
//   A4 hydration  -- A1..A3 still hold after a page reload re-hydrates from
//                    the agent (fresh browser context, empty localStorage)
//   F1 bare boot  -- CONV-ID-BOOT-1: /app/ with NO conversation_id and only the
//                    real-scope anchor bucket preset must restore the anchored
//                    conversation (mapping not overwritten, trace blocks with
//                    data-turn-id and the A/B judge card rendered)
//   G1 audition   -- AUDITION-UNSTICK-1: a stopped A/B session must not dead-lock
//                    its controls (click restarts playback server-side), and a
//                    preparing session must say 「正在准备 A/B 试听…」 with the
//                    controls disabled but carrying their reason
//
// Exit code: 0 = every group passed (delivery gate), 1 = at least one failed
// (pre-fix red, with the failing group recorded in the report).

import { createRequire } from "node:module";
import { existsSync, mkdirSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { createHash } from "node:crypto";

const require = createRequire(import.meta.url);
const here = dirname(new URL(import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, "$1"));

function env(name, fallback = "") {
  const value = process.env[name];
  return value === undefined || value === "" ? fallback : value;
}
function arg(name, fallback = "") {
  const flag = "--" + name;
  const index = process.argv.indexOf(flag);
  return index >= 0 && process.argv[index + 1] ? process.argv[index + 1] : fallback;
}

const agentBase = arg("agent-base", env("AGENT_BASE", "http://127.0.0.1:7897")).replace(/\/+$/, "");
const conversationId = arg("conversation-id", env("CONVERSATION_ID", "webui_mtzba6wf"));
const graphFixturePath = arg("graph-fixture", env("GRAPH_FIXTURE", join(here, "fixtures", "webui_rendered_dom", "webui_rendered_dom_graph.fixture.json")));
const eventsFixturePath = arg("events-fixture", env("EVENTS_FIXTURE", join(here, "fixtures", "webui_rendered_dom", "webui_rendered_dom_events.fixture.json")));
const outDir = arg("out-dir", env("OUT_DIR", join(here, "..", "artifacts", "e2e_webui1", "adhoc")));
const pwModulePath = arg("playwright-module", env("PW_MODULE", ""));
// Directories holding the archived session's real commit objects. The archived
// graph's nodes reference them, and the agent's history reader drops any node
// whose commit object it cannot read -- so they are copied in, verbatim.
const commitDirs = arg("commit-dirs", env("COMMIT_DIRS", ""))
  .split(";")
  .map((value) => value.trim())
  .filter(Boolean);
const headless = arg("headed", "") === "" ? true : false;
// Control mode (not part of the delivery gate): instead of replaying the
// archived events, drive a REAL turn through the real composer and then reload.
// Used to prove the assertions can go green on a healthy transcript, so a red
// gate result is a finding about the state and not about the probe.
const driveTurn = arg("drive-turn", "") !== "";
const driveTurnMessage = arg("turn-message", env("TURN_MESSAGE", "帮我看一下当前工程的轨道状态。"));
const driveTurnBudgetMs = Number(arg("turn-budget-ms", env("TURN_BUDGET_MS", "120000")));
const settleMs = Number(arg("settle-ms", env("SETTLE_MS", "8000")));
const viewport = { width: Number(arg("viewport-width", "1600")), height: Number(arg("viewport-height", "900")) };

mkdirSync(outDir, { recursive: true });

const log = (...parts) => console.log("[e2e-webui1]", ...parts);

function sha256(text) {
  return createHash("sha256").update(text).digest("hex");
}

// ---------------------------------------------------------------- playwright

// Hosts that installed Playwright through npx keep it in the npm cache
// (_npx/<hash>/node_modules). Prefer the newest version found there so the
// smoke still runs without a package.json change in the repository.
function npxCacheCandidates() {
  const found = [];
  const roots = [
    process.env.npm_config_cache,
    process.env.LOCALAPPDATA ? join(process.env.LOCALAPPDATA, "npm-cache") : "",
    process.env.APPDATA ? join(process.env.APPDATA, "npm-cache") : ""
  ].filter(Boolean);
  for (const root of roots) {
    const entriesDir = join(root, "_npx");
    if (!existsSync(entriesDir)) continue;
    let entries = [];
    try {
      entries = readdirSync(entriesDir, { withFileTypes: true }).filter((entry) => entry.isDirectory());
    } catch {
      continue;
    }
    const versions = [];
    for (const entry of entries) {
      const pkgPath = join(entriesDir, entry.name, "node_modules", "playwright-core", "package.json");
      if (!existsSync(pkgPath)) continue;
      try {
        const pkg = JSON.parse(readFileSync(pkgPath, "utf-8"));
        versions.push({ dir: join(entriesDir, entry.name, "node_modules", "playwright-core"), version: String(pkg.version || "0.0.0") });
      } catch { /* skip unreadable entries */ }
    }
    versions.sort((left, right) => compareVersions(right.version, left.version));
    for (const item of versions) found.push(item.dir);
  }
  return found;
}

function compareVersions(left, right) {
  const a = String(left).split(/[.-]/).map((part) => Number.parseInt(part, 10) || 0);
  const b = String(right).split(/[.-]/).map((part) => Number.parseInt(part, 10) || 0);
  for (let i = 0; i < Math.max(a.length, b.length); i += 1) {
    const diff = (a[i] || 0) - (b[i] || 0);
    if (diff !== 0) return diff;
  }
  return 0;
}

function resolvePlaywright() {
  const candidates = [
    pwModulePath,
    join(here, "..", "agent", "webui", "node_modules", "playwright-core"),
    join(here, "..", "agent", "webui", "node_modules", "playwright"),
    join(here, "..", "node_modules", "playwright-core"),
    ...npxCacheCandidates(),
    "playwright-core",
    "playwright"
  ].filter(Boolean);
  const failures = [];
  for (const candidate of candidates) {
    try {
      const mod = require(candidate);
      const chosen = mod && mod.chromium && typeof mod.chromium.launch === "function"
        ? mod.chromium
        : mod && mod.default && mod.default.chromium && typeof mod.default.chromium.launch === "function"
          ? mod.default.chromium
          : null;
      if (chosen) {
        return { module: chosen, path: candidate };
      }
      failures.push(candidate + " -> resolved but exposes no chromium.launch");
    } catch (err) {
      failures.push(candidate + " -> " + String(err && err.message ? err.message : err).split("\n")[0]);
    }
  }
  throw new Error(
    "playwright module not found. Pass --playwright-module <dir> or set PW_MODULE.\n" +
    "Install once with:  npx --yes playwright@latest install chromium\n" +
    "or point PW_MODULE at any node_modules directory containing playwright-core.\n" +
    failures.join("\n")
  );
}

// Choose a browser the host can actually start unattended. Bundled Chromium
// first (fully reproducible), then the Edge channel (present on stock Windows,
// no download). Both are headless; the smoke never needs a visible window.
async function launchBrowser(chromium) {
  const attempts = [
    { label: "chromium-bundled", options: { headless } },
    { label: "chromium-channel-msedge", options: { channel: "msedge", headless } },
    { label: "chromium-channel-chrome", options: { channel: "chrome", headless } }
  ];
  const failures = [];
  for (const attempt of attempts) {
    try {
      if (typeof chromium.launch !== "function") {
        throw new Error("chromium launcher is not a function (got " + typeof chromium + ")");
      }
      const browser = await chromium.launch(attempt.options);
      return { browser, label: attempt.label, version: browser.version() };
    } catch (err) {
      failures.push(attempt.label + ": " + String(err && err.message ? err.message : err).split("\n")[0]);
    }
  }
  throw new Error("no usable browser for the rendered smoke:\n" + failures.join("\n"));
}

// ------------------------------------------------------------------ HTTP I/O

async function getJSON(path) {
  const res = await fetch(agentBase + path);
  const text = await res.text();
  if (!res.ok) throw new Error("GET " + path + " -> " + res.status + " " + text.slice(0, 200));
  return JSON.parse(text);
}

// ------------------------------------------------------- agent-side seeding

function seedConversation(graphFixture, state) {
  const history = state.project_history || {};
  const draftDir = dirname(history.project_path || "");
  const historyDir = history.history_dir || join(draftDir, ".vit_history");
  const stateDir = history.state_dir || "";
  if (!draftDir || !stateDir) {
    throw new Error("agent ui/state did not expose a draft project history (project_path/state_dir)");
  }
  const commitsDir = join(historyDir, "commits");
  mkdirSync(commitsDir, { recursive: true });

  const allNodes = Array.isArray(graphFixture.nodes) ? graphFixture.nodes : [];
  const kept = [];
  const skipped = [];
  let previousCommitId = "";
  for (const node of allNodes) {
    const archivedCommit = findArchivedCommit(node.commit_id, historyDir);
    if (!archivedCommit) {
      skipped.push({ id: node.id, commit_id: node.commit_id });
      continue;
    }
    const commit = {
      ...archivedCommit,
      id: node.commit_id,
      project_path: history.project_path,
      branch: history.active_branch || "main",
      project_uuid: history.project_uuid || archivedCommit.project_uuid,
      parents: previousCommitId ? [previousCommitId] : undefined
    };
    writeFileSync(join(commitsDir, node.commit_id + ".json"), JSON.stringify(commit, null, 2), "utf-8");
    previousCommitId = node.commit_id;
    kept.push(node);
  }
  const graph = {
    project_path: history.project_path,
    project_uuid: history.project_uuid || "",
    active_branch: graphFixture.active_branch || "main",
    active_node_id: kept.length ? kept[kept.length - 1].id : "",
    nodes: kept
  };
  const graphPath = join(stateDir, "conversation_graph.json");
  mkdirSync(dirname(graphPath), { recursive: true });
  writeFileSync(graphPath, JSON.stringify(graph, null, 2), "utf-8");
  return {
    historyDir,
    stateDir,
    graphPath,
    commitsWritten: kept.length,
    nodesSkipped: skipped,
    archivedSource: graphFixture.source || ""
  };
}

// Archived commit objects are the real ones the session wrote. They are read
// from the session's own commit directory (--commit-dirs) or, on a re-run,
// from the already-seeded draft history.
function findArchivedCommit(commitId, historyDir) {
  if (!commitId) return null;
  const candidates = [
    join(historyDir, "commits", commitId + ".json"),
    ...commitDirs.map((dir) => join(dir, commitId + ".json"))
  ];
  for (const candidate of candidates) {
    if (!existsSync(candidate)) continue;
    try {
      return JSON.parse(readFileSync(candidate, "utf-8"));
    } catch {
      return null;
    }
  }
  return null;
}

// ------------------------------------------------------------- DOM sampling

// One evaluate() call returns the whole evidence set. Everything asserted on is
// a real layout fact: getBoundingClientRect plus the real child order of the
// real .message-stream, taken twice --
//   * where the stream sits right after load (what the user's eyes land on),
//   * and with the stream scrolled to its real bottom (the worst case: the
//     newest content pressed against the .composer overlay).
// A block that is anchored inside its turn is reached by scrolling; a block
// that fell to the flow tail is the last thing in the stream and is the item
// the overlay covers.
const DOM_PROBE = () => {
  const stream = document.querySelector(".message-stream");
  const composer = document.querySelector(".composer");
  const composerRect = composer ? composer.getBoundingClientRect() : null;
  const streamRect = stream ? stream.getBoundingClientRect() : null;
  const flow = stream ? Array.from(stream.children) : [];
  const flowIndex = (el) => flow.indexOf(el);
  const describe = (el) => ({
    tag: el.tagName,
    cls: typeof el.className === "string" ? el.className : "",
    text: (el.textContent || "").replace(/\s+/g, " ").trim().slice(0, 60)
  });
  const userMessages = flow
    .filter((el) => el.classList.contains("message-row") && el.classList.contains("user"))
    .map((el) => ({ index: flowIndex(el), ...describe(el) }));
  const assistantMessages = flow
    .filter((el) => el.classList.contains("message-row") && el.classList.contains("assistant"))
    .map((el) => ({ index: flowIndex(el), ...describe(el) }));
  const sampleBlocks = () => Array.from(stream ? stream.querySelectorAll(".trace-block") : []).map((el) => {
    const rect = el.getBoundingClientRect();
    const steps = Array.from(el.querySelectorAll(".trace-step")).map((step) => ({
      cls: typeof step.className === "string" ? step.className : "",
      duration: (step.querySelector(".trace-dur")?.textContent || "").trim(),
      // TRAJ-IMPL-2: the step's own wording. It is what carries the human-readable
      // title ruling (design section 7 A), so the assertion reads it from the DOM
      // instead of inferring it from the head meta.
      text: (step.querySelector(".trace-act")?.textContent || "").replace(/\s+/g, " ").trim(),
      gloss: (step.querySelector(".trace-gloss")?.textContent || "").trim()
    }));
    const label = (el.querySelector(".th-label")?.textContent || "").trim();
    const meta = (el.querySelector(".th-meta")?.textContent || "").replace(/\s+/g, " ").trim();
    // TRAJ-IMPL-1 (design section 2.4-2): the explicit residency row. Sampled as
    // its own element so the assertion never depends on how the flex gap
    // separates the head meta spans in textContent.
    const waitRow = el.querySelector(".trace-wait");
    const wait = waitRow ? (waitRow.textContent || "").replace(/\s+/g, " ").trim() : "";
    return {
      turnId: el.getAttribute("data-turn-id") || "",
      cls: typeof el.className === "string" ? el.className : "",
      label,
      meta,
      wait,
      hasWaitRow: Boolean(waitRow),
      waitButtons: waitRow ? waitRow.querySelectorAll("button").length : 0,
      cursors: el.querySelectorAll(".trace-cursor").length,
      steps,
      flowIndex: flowIndex(el),
      top: rect.top,
      bottom: rect.bottom,
      height: rect.height
    };
  });
  const natural = {
    scrollTop: stream ? stream.scrollTop : null,
    blocks: sampleBlocks()
  };
  if (stream) {
    stream.scrollTop = stream.scrollHeight;
  }
  const atBottom = {
    scrollTop: stream ? stream.scrollTop : null,
    blocks: sampleBlocks()
  };
  const composerTop = composerRect ? composerRect.top : null;
  const blocks = atBottom.blocks.map((block, index) => {
    const naturalBlock = natural.blocks[index] || block;
    return {
      ...block,
      naturalTop: naturalBlock.top,
      naturalBottom: naturalBlock.bottom,
      naturalOccludedByComposer: composerTop === null ? null : naturalBlock.bottom > composerTop,
      composerTop,
      occludedByComposer: composerTop === null ? null : block.bottom > composerTop
    };
  });
  // The last piece of real content in the stream (everything above the scroll
  // anchor). Its bottom edge vs the composer top at max scroll is the whole
  // "is bottom content reachable at all" question, independent of anchors.
  const anchorEl = stream ? stream.querySelector(".message-scroll-anchor") : null;
  const anchorStyleHeight = anchorEl ? (anchorEl.style.height || "") : "";
  const contentElements = flow.filter((el) => el !== anchorEl);
  const lastContent = contentElements.length ? contentElements[contentElements.length - 1] : null;
  const lastContentRect = lastContent ? lastContent.getBoundingClientRect() : null;
  return {
    url: location.href,
    sampledAtMs: Date.now(),
    conversationId: new URLSearchParams(location.search).get("conversation_id"),
    naturalScrollTop: natural.scrollTop,
    naturalBlocks: natural.blocks,
    lastContent: lastContentRect
      ? {
          cls: typeof lastContent.className === "string" ? lastContent.className : lastContent.tagName,
          text: (lastContent.textContent || "").replace(/\s+/g, " ").trim().slice(0, 60),
          top: lastContentRect.top,
          bottom: lastContentRect.bottom,
          belowComposerTop: composerTop === null ? null : lastContentRect.bottom > composerTop,
          gapToComposerTop: composerTop === null ? null : composerTop - lastContentRect.bottom
        }
      : null,
    // The stream reserves space for the floating .composer with
    // .message-scroll-anchor{height: max(132, composerHeight + 36)} (App.tsx).
    // Reading it lets the report prove whether the bottom inset is the reason a
    // block is (or is not) covered -- and shows the clearance the anchors get.
    scrollAnchor: anchorEl
      ? {
          styleHeight: anchorStyleHeight,
          inlineHeight: Number.parseFloat(anchorStyleHeight) || null,
          renderedHeight: anchorEl.getBoundingClientRect().height,
          composerClearance: composerRect
            ? composerRect.height + (composerRect.bottom > window.innerHeight ? 0 : window.innerHeight - composerRect.bottom)
            : null
        }
      : null,
    flow: flow.map((el) => ({ ...describe(el), index: flowIndex(el) })),
    userMessages,
    assistantMessages,
    blocks,
    atBottomScrollTop: atBottom.scrollTop,
    composer: composerRect
      ? { top: composerRect.top, bottom: composerRect.bottom, height: composerRect.height }
      : null,
    stream: streamRect
      ? {
          top: streamRect.top,
          bottom: streamRect.bottom,
          scrollTop: stream.scrollTop,
          scrollHeight: stream.scrollHeight,
          clientHeight: stream.clientHeight,
          paddingBottom: getComputedStyle(stream).paddingBottom
        }
      : null,
    // TRAJ-IMPL-2 (design section 2.1-5): the flow-bottom activity lane is where
    // un-owned activities (uploads, calls) still live. Items that already sit
    // inside a round block must NOT appear here a second time, so the lane is
    // sampled explicitly.
    laneItems: Array.from(document.querySelectorAll(".activity-lane-item")).map((el) => ({
      cls: typeof el.className === "string" ? el.className : "",
      text: (el.textContent || "").replace(/\s+/g, " ").trim()
    })),
    // TRAJ-IMPL-3 (design section 2.3): the static collapsed receipt row is the only
    // trace-shaped surface a refreshed page can show for a turn whose live state is
    // gone, so it is sampled as its own element set. Sampled after the stream was
    // scrolled to its real bottom, so the composer-overlap fact is the worst case.
    receipts: Array.from(stream ? stream.querySelectorAll(".trace-receipt") : []).map((el) => {
      const rect = el.getBoundingClientRect();
      return {
        turnId: el.getAttribute("data-turn-id") || "",
        status: el.getAttribute("data-receipt-status") || "",
        cls: typeof el.className === "string" ? el.className : "",
        text: (el.textContent || "").replace(/\s+/g, " ").trim(),
        buttons: el.querySelectorAll("button").length,
        cursors: el.querySelectorAll(".trace-cursor").length,
        flowIndex: flowIndex(el),
        top: rect.top,
        bottom: rect.bottom,
        occludedByComposer: composerTop === null ? null : rect.bottom > composerTop
      };
    }),
    // The ledger itself. Its storage key is namespaced by conversation+scope, so the
    // probe finds it by prefix instead of rebuilding the app's key encoding.
    receiptLedger: (() => {
      const key = Object.keys(localStorage).find((item) => item.indexOf("vit.turn_receipts.v1") === 0);
      if (!key) return null;
      return { key: key, raw: localStorage.getItem(key) || "" };
    })(),
    localStorageKeys: Object.keys(localStorage),
    composerCss: composer ? {
      position: getComputedStyle(composer).position,
      bottom: getComputedStyle(composer).bottom,
      zIndex: getComputedStyle(composer).zIndex
    } : null,
    // PLANBAR-1 (2026-09-13): the plan bar is a sibling of the composer inside
    // .composer-dock, so the T6 design slot ("above the input box") is a real
    // viewport-coordinate fact: bar bottom vs composer top. Measured here with
    // the same getBoundingClientRect the user's own screen shows.
    dock: document.querySelector(".composer-dock")
      ? (() => {
          const dockRect = document.querySelector(".composer-dock").getBoundingClientRect();
          return { top: dockRect.top, bottom: dockRect.bottom, height: dockRect.height, children: Array.from(document.querySelector(".composer-dock").children).map((el) => typeof el.className === "string" ? el.className : el.tagName) };
        })()
      : null,
    planBar: (() => {
      const bar = document.querySelector(".plan-bar");
      if (!bar) return null;
      const rect = bar.getBoundingClientRect();
      return {
        cls: typeof bar.className === "string" ? bar.className : "",
        phase: bar.getAttribute("data-plan-phase"),
        taskId: bar.getAttribute("data-task-id"),
        text: (bar.textContent || "").replace(/\s+/g, " ").trim().slice(0, 80),
        parentCls: bar.parentElement && typeof bar.parentElement.className === "string" ? bar.parentElement.className : "",
        top: rect.top,
        bottom: rect.bottom,
        height: rect.height,
        position: getComputedStyle(bar).position,
        zIndex: getComputedStyle(bar).zIndex,
        opacity: getComputedStyle(bar).opacity,
        spinning: Boolean(bar.querySelector("svg.spin")),
        composerTop: composerTop,
        gapToComposerTop: composerTop === null ? null : composerTop - rect.bottom,
        occludedByComposer: composerTop === null ? null : rect.bottom > composerTop
      };
    })(),
    // CONV-ID-BOOT-1: the A/B judge card is a first-class survivor of a refresh -- the
    // card is keyed by its audition session, sampled as its own element set so the
    // bare-boot pass can assert its presence without inferring it from the trace DOM.
    auditionCards: Array.from(document.querySelectorAll("[data-audition-session]")).map((el) => ({
      session: el.getAttribute("data-audition-session") || "",
      status: el.getAttribute("data-status") || "",
      cls: typeof el.className === "string" ? el.className : "",
      text: (el.textContent || "").replace(/\s+/g, " ").trim().slice(0, 80)
    })),
    // CONV-ID-BOOT-1: the scoped conversation-id anchor buckets, WITH values. The
    // whole defect this gate pass pins is "a bare boot overwrites the anchor with a
    // fresh random id", so the probe must read the values, not just the key names.
    scopeBuckets: Object.fromEntries(
      Object.keys(localStorage)
        .filter((key) => key.indexOf("ask_vit_conversation_id_scope") === 0)
        .map((key) => [key, localStorage.getItem(key) || ""])
    ),
    // AUDITION-UNSTICK-1: the A/B card's own controls are first-class citizens of
    // the rendered surface. Sampled per card so the stopped/preparing shapes can
    // be asserted on real DOM facts (disabled attributes, reasons, chips).
    auditionControls: Array.from(document.querySelectorAll("[data-audition-session]")).map((card) => ({
      session: card.getAttribute("data-audition-session") || "",
      status: card.getAttribute("data-status") || "",
      text: (card.textContent || "").replace(/\s+/g, " ").trim().slice(0, 120),
      switchButtons: Array.from(card.querySelectorAll(".ab-sw button")).map((button) => ({
        text: (button.textContent || "").trim(),
        disabled: button.disabled,
        title: button.getAttribute("title") || ""
      })),
      playButtons: Array.from(card.querySelectorAll(".pbtn")).map((button) => ({
        disabled: button.disabled,
        title: button.getAttribute("title") || ""
      })),
      chips: Array.from(card.querySelectorAll(".c-head .chip")).map((chip) => (chip.textContent || "").trim())
    }))
  };
};

// ------------------------------------------------------------- assertions

const TIMED_LABEL = /^\d+(\.\d+)?s$/;

function checkA1(sample) {
  const failures = [];
  const notes = [];
  if (sample.blocks.length === 0) {
    failures.push("no .trace-block rendered at all");
    return { failures, notes };
  }
  for (const block of sample.blocks) {
    // The invariant: a trace block belongs to a conversation turn, so it must
    // render BEFORE the next turn's user message. A block with no user message
    // after it, in a conversation that has more than one round, is appended at
    // the flow tail -- the UI-FOLLOW-2 shape (it is the last thing in the
    // stream, so nothing pushes it down and the composer overlay is what the
    // user's eye lands on). In a single-round conversation the own-turn slot
    // IS the tail, and that is correct.
    const before = sample.userMessages.filter((message) => message.index < block.flowIndex);
    const after = sample.userMessages.filter((message) => message.index > block.flowIndex);
    const owner = before.length ? before[before.length - 1] : null;
    const ownTurnAtTail = sample.userMessages.length <= 1 && before.length === 1;
    if (after.length === 0 && !ownTurnAtTail) {
      failures.push(
        "A1 tail: trace block turn=" + (block.turnId || "?") + " renders after every user message " +
        "(flowIndex " + block.flowIndex + " of " + (sample.flow.length - 1) + ", " +
        sample.userMessages.length + " user message(s) in the flow, last at flowIndex " +
        (sample.userMessages[sample.userMessages.length - 1] || {}).index + ") -- the block left its " +
        "own turn's slot, which is the UI-FOLLOW-2 symptom shape"
      );
    } else {
      notes.push(
        "turn=" + (block.turnId || "?") + " sits in a turn slot: after user message \"" +
        (owner ? owner.text.slice(0, 24) : "(none)") + "\" (flowIndex " + (owner ? owner.index : "?") +
        ") at flowIndex " + block.flowIndex + ", with " + after.length +
        " user message(s) still following it"
      );
    }
    const composerTopText = block.composerTop === null ? "n/a" : block.composerTop.toFixed(1);
    if (block.occludedByComposer || block.naturalOccludedByComposer) {
      failures.push(
        "A1 occlusion: trace block turn=" + (block.turnId || "?") +
        " is covered by the .composer overlay (composer top=" + composerTopText + ");" +
        " at max scroll bottom=" + block.bottom.toFixed(1) +
        (block.occludedByComposer ? " [covered]" : " [clear]") +
        ", at the post-load scroll position bottom=" + block.naturalBottom.toFixed(1) +
        (block.naturalOccludedByComposer ? " [covered]" : " [clear]")
      );
    } else if (block.composerTop !== null) {
      notes.push(
        "turn=" + (block.turnId || "?") + " clear of the composer: at max scroll bottom=" + block.bottom.toFixed(1) +
        ", post-load bottom=" + block.naturalBottom.toFixed(1) + ", composer top=" + composerTopText
      );
    }
  }
  if (sample.scrollAnchor) {
    // App.tsx reserves max(132, composerHeight + 36) for the floating composer.
    // Reporting the reserved height next to the real composer box is what makes
    // an occlusion failure (or its absence) falsifiable from the artifact.
    const composerHeight = sample.composer ? sample.composer.height : 0;
    const expectedInset = Math.max(132, Math.ceil(composerHeight + 36));
    notes.push(
      "bottom inset: scroll-anchor height=" + sample.scrollAnchor.renderedHeight.toFixed(0) +
      "px (expected " + expectedInset + "px from composer height " + composerHeight.toFixed(0) +
      "px + the composer's 22px bottom offset), distance from composer bottom to viewport bottom=" +
      sample.scrollAnchor.composerClearance.toFixed(0) + "px"
    );
  }
  if (sample.lastContent && sample.lastContent.belowComposerTop) {
    notes.push(
      "supporting: with the stream at its real bottom the last content element (" +
      sample.lastContent.cls + ") ends " + Math.abs(sample.lastContent.gapToComposerTop).toFixed(1) +
      "px BELOW the composer top -- the stream's bottom inset does not clear the floating .composer"
    );
  } else if (sample.lastContent) {
    notes.push(
      "supporting: last content element ends " + sample.lastContent.gapToComposerTop.toFixed(1) +
      "px above the composer top at max scroll"
    );
  }
  return { failures, notes };
}

function checkA2(sample) {
  const failures = [];
  const notes = [];
  for (const block of sample.blocks) {
    const live = /(^|\s)is-live(\s|$)/.test(block.cls);
    if (live) {
      continue; // still-running turn: live classes are the correct contract
    }
    if (!/(^|\s)(is-terminal|is-waiting|is-warning)(\s|$)/.test(block.cls)) {
      failures.push("A2: settled block turn=" + (block.turnId || "?") + " carries neither terminal/waiting/warning class: " + block.cls);
    }
    for (const step of block.steps) {
      if (/(^|\s)(is-running|is-pending|is-live)(\s|$)/.test(step.cls) || /(^|\s)is-live(\s|$)/.test(step.cls)) {
        failures.push(
          "A2 freeze: turn=" + (block.turnId || "?") + " is settled but a step still renders live state: " +
          step.cls + " / \"" + step.duration + "\""
        );
      }
      if (step.duration === "进行中" || step.duration === "排队中") {
        failures.push("A2 freeze: turn=" + (block.turnId || "?") + " settled step still shows \"" + step.duration + "\"");
      }
      if (/(^|\s)is-unresolved(\s|$)/.test(step.cls) && step.duration !== "未收口") {
        failures.push("A2 freeze: unresolved step must read 未收口, got \"" + step.duration + "\"");
      }
    }
    notes.push(
      "turn=" + (block.turnId || "?") + " cls=\"" + block.cls + "\" label=\"" + block.label +
      "\" meta=\"" + block.meta + "\" steps=" + block.steps.length
    );
  }
  return { failures, notes };
}

function checkA3(sample) {
  const failures = [];
  const notes = [];
  for (const block of sample.blocks) {
    const meta = block.meta;
    const parkMatch = meta.match(/等待续跑\s*([\d.]+s)/);
    const workMatch = meta.match(/(?:^|\s)执行\s*([\d.]+s)/);
    if (parkMatch) {
      if (!workMatch) {
        failures.push("A3: turn=" + (block.turnId || "?") + " shows 等待续跑 without a preceding 执行 segment: \"" + meta + "\"");
      }
      notes.push("turn=" + (block.turnId || "?") + " residency split present: 执行 " + workMatch[1] + " / 等待续跑 " + parkMatch[1]);
    } else {
      const bare = meta.split(/\s+/).filter((part) => TIMED_LABEL.test(part));
      if (bare.length > 0 && !/1 项活动|项活动/.test(meta)) {
        notes.push("turn=" + (block.turnId || "?") + " work-only duration: \"" + meta + "\"");
      } else {
        notes.push("turn=" + (block.turnId || "?") + " no residency segment (等待续跑 absent, as required): \"" + meta + "\"");
      }
      if (/等待续跑/.test(meta)) {
        failures.push("A3: turn=" + (block.turnId || "?") + " reports 等待续跑 without a parsed segment");
      }
    }
  }
  return { failures, notes };
}

// PLANBAR-1: the bar must render above the input box (T6 design slot), and a
// finished task must clear the surface instead of staying on screen.
function checkB1(sample) {
  const failures = [];
  const notes = [];
  const bar = sample.planBar;
  if (!bar) {
    failures.push("B1: .plan-bar did not render on the injected task state -- the T6 slot cannot be measured");
    return { failures, notes };
  }
  notes.push(
    "plan bar phase=" + bar.phase + " cls=\"" + bar.cls + "\" top=" + bar.top.toFixed(1) +
    " bottom=" + bar.bottom.toFixed(1) + " height=" + bar.height.toFixed(1) +
    " position=" + bar.position + " z-index=" + bar.zIndex + " parent=\"" + bar.parentCls + "\""
  );
  if (!/composer-dock/.test(bar.parentCls)) {
    failures.push("B1: the plan bar is not inside .composer-dock (parent=\"" + bar.parentCls + "\") -- it left the T6 slot chain");
  }
  if (sample.composer === null) {
    failures.push("B1: .composer did not render, so bar-vs-composer coordinates cannot be compared");
    return { failures, notes };
  }
  notes.push(
    "composer top=" + sample.composer.top.toFixed(1) + " bottom=" + sample.composer.bottom.toFixed(1) +
    " -- distance from bar bottom to composer top=" + bar.gapToComposerTop.toFixed(1) + "px"
  );
  if (bar.bottom > sample.composer.top) {
    failures.push(
      "B1 position: the plan bar renders BELOW the composer top (bar bottom=" + bar.bottom.toFixed(1) +
      ", composer top=" + sample.composer.top.toFixed(1) + ", overlap=" +
      (bar.bottom - sample.composer.top).toFixed(1) + "px) -- the T6 design slot is above the input box"
    );
  }
  if (sample.dock) {
    notes.push(
      "dock column: height=" + sample.dock.height.toFixed(1) + " top=" + sample.dock.top.toFixed(1) +
      " children=[" + sample.dock.children.join(", ") + "]"
    );
    if (String(sample.dock.children[0] || "").indexOf("plan-bar") < 0) {
      failures.push("B1: the plan bar is not the first child of .composer-dock (children=[" + sample.dock.children.join(", ") + "]) -- flex order is what guarantees bar.bottom <= composer.top");
    }
  } else {
    failures.push("B1: .composer-dock is not in the DOM -- the bottom stack that seats the bar above the composer is missing");
  }
  return { failures, notes };
}

// ------------------------------------------------------------------- TRAJ-IMPL-1

// TRAJ-IMPL-1 (design section 2.4, card 2026-09-14): the residency shape is a
// turn that is still live while its work slice has already closed -- the slice
// ended with waiting_continue (turn.completed landed) and the turn's terminal
// event has not arrived. The archived transcript contains no such turn, so the
// boundary events are seeded for a NEW turn id and replayed through the very
// same GET /agent/events contract the app polls. The elapsed counters are
// derived by the app from the event timestamps; nothing here waits 55 seconds
// and nothing is patched inside the page.
function residencyFixtureEvents(options) {
  const now = Date.now();
  const startedAt = new Date(now - 60_000).toISOString();
  const endedAt = new Date(now - 5_000).toISOString();
  const runId = options.turnId;
  const base = Number(options.baseSeq) || 100;
  const events = [
    {
      seq: base + 1, type: "turn.started", conversation_id: conversationId,
      goal_id: runId, run_id: runId, turn_id: runId, source_turn_id: runId,
      item_type: "turn", status: "running", created_at: startedAt
    },
    {
      seq: base + 2, type: "trajectory.turn.started", conversation_id: conversationId,
      goal_id: runId, run_id: runId, turn_id: runId, source_turn_id: runId,
      item_id: "turn:" + runId, status: "running", created_at: startedAt,
      payload: {
        schema_version: "vit.observable_trajectory.v1", trace_node_id: "turn:" + runId,
        turn_id: runId, node_kind: "turn", phase: "framing", status: "running"
      }
    },
    {
      seq: base + 3, type: "trajectory.observation.recorded", conversation_id: conversationId,
      goal_id: runId, run_id: runId, turn_id: runId, source_turn_id: runId,
      item_id: "obs:" + runId, status: "completed", created_at: endedAt,
      payload: {
        schema_version: "vit.observable_trajectory.v1", trace_node_id: "obs:" + runId,
        turn_id: runId, node_kind: "observation", phase: "observing", status: "completed",
        summary: "频率关系观察"
      }
    },
    // The work slice ends here (slice boundary, waiting_continue). The turn's
    // own terminal event (trajectory.turn.completed) is deliberately absent:
    // that is exactly the residency window the design describes.
    {
      seq: base + 4, type: "turn.completed", conversation_id: conversationId,
      goal_id: runId, run_id: runId, turn_id: runId, source_turn_id: runId,
      item_type: "turn", status: "waiting_continue", created_at: endedAt,
      payload: { turn_kind: "slice_boundary", goal_status: "waiting_continue" }
    }
  ];
  if (options.judgment) {
    // Same turn, unresolved A/B judgment card: the trajectory-side record of a
    // judgement request that has not been answered (status waiting_for_user,
    // no trajectory.user_judgment.recorded for the same session).
    events.push({
      seq: base + 5, type: "trajectory.user_judgment.requested", conversation_id: conversationId,
      goal_id: runId, run_id: runId, turn_id: runId, source_turn_id: runId,
      item_id: "judgment:" + runId, status: "waiting_for_user", created_at: endedAt,
      payload: {
        schema_version: "vit.observable_trajectory.v1", trace_node_id: "judgment:" + runId,
        turn_id: runId, node_kind: "user_judgment", phase: "user_judgment",
        status: "waiting_for_user", summary: "static_eq · A/B 试听判定",
        details: { audition_session_id: "audition:" + runId }
      }
    });
  }
  return events;
}

// TRAJ-IMPL-2 (design section 2.1, card 2026-09-14): the defect shape is a turn
// with NO experiment trajectory at all -- a pure chat turn whose item.* events are
// in the stream while "container exists" was still bound to "experiment admitted".
// The archived transcript contains no such turn (every archived turn has a
// trajectory shell), so the boundary events are seeded for a NEW turn id and
// replayed through the very same GET /agent/events contract the app polls.
//
// Deliberately faithful to the real event shape (forensic evidence, 2026-09-11 and
// 2026-09-13 fixtures): every tool call of one run reuses item_id "tool_step_1",
// item.started carries payload.tool and item.completed carries payload.command_name
// (different spellings of the same action). No terminal event is seeded: the turn is
// observed during execution, which is exactly the window the card is about.
function chatOnlyItemStepsFixtureEvents(options) {
  const now = Date.now();
  const runId = options.turnId;
  const base = Number(options.baseSeq) || 300;
  const at = (offsetMs) => new Date(now - 30_000 + offsetMs).toISOString();
  const common = { conversation_id: conversationId, goal_id: runId, run_id: runId, turn_id: runId, source_turn_id: runId };
  return [
    { ...common, seq: base + 1, type: "turn.started", item_type: "turn", status: "running", created_at: at(0) },
    { ...common, seq: base + 2, type: "item.started", item_id: "tool_step_1", item_type: "daw_action", status: "running", created_at: at(1_000), payload: { tool: "ccb.observation_catalog", command_raw: { tool: "ccb.observation_catalog" } } },
    { ...common, seq: base + 3, type: "item.completed", item_id: "tool_step_1", item_type: "daw_action", status: "completed", created_at: at(2_500), payload: { command_name: "ccb_observation_catalog" } },
    { ...common, seq: base + 4, type: "item.started", item_id: "tool_step_1", item_type: "daw_action", status: "running", created_at: at(4_000), payload: { tool: "mix_tick" } },
    { ...common, seq: base + 5, type: "item.completed", item_id: "tool_step_1", item_type: "daw_action", status: "completed", created_at: at(6_000), payload: { command_name: "mix_tick" } },
    // Unmapped identifier: user ruling A keeps it verbatim (the mapping table is a
    // living table), so the assertion pins "a step rendered with its own wording".
    { ...common, seq: base + 6, type: "item.started", item_id: "tool_step_1", item_type: "daw_action", status: "running", created_at: at(8_000), payload: { tool: "weird.custom_tool" } }
  ];
}

// D1: during execution of a turn that has NO experiment trajectory, a round block
// must exist with the item steps inlined under their human-readable titles, and
// those same items must not ALSO sit in the flow-bottom activity lane.
function checkD1(result, options) {
  const failures = [];
  const notes = [];
  const sample = result.sample;
  if (!result.appeared) {
    failures.push(
      "D1: the seeded non-experiment turn (" + options.turnId + ") rendered no .trace-block -- item.* evidence " +
      "alone must be enough for a round container (design section 2.1-2)"
    );
    return { failures, notes };
  }
  const block = (sample.blocks || []).find((item) => item.turnId === options.turnId) || null;
  if (!block) {
    failures.push("D1: block for " + options.turnId + " left the DOM between the wait and the sample");
    return { failures, notes };
  }
  notes.push(
    "turn=" + options.turnId + " cls=\"" + block.cls + "\" label=\"" + block.label + "\" meta=\"" + block.meta +
    "\" steps=" + block.steps.length + " stepTexts=[" + block.steps.map((step) => step.text).join(" | ") + "]"
  );
  if (!/(^|\s)is-live(\s|$)/.test(block.cls)) {
    failures.push(
      "D1: the running non-experiment turn is not live (cls=\"" + block.cls + "\") -- the container must appear " +
      "during execution, not only after the turn settles"
    );
  }
  if (!/\d+\s*步/.test(block.meta)) {
    failures.push("D1 head meta: expected an N 步 count during execution, got \"" + block.meta + "\"");
  }
  for (const expected of options.expectStepTexts) {
    if (!block.steps.some((step) => step.text.indexOf(expected) >= 0)) {
      failures.push(
        "D1 step wording: no rendered item step carries \"" + expected + "\" (rendered: [" +
        block.steps.map((step) => step.text).join(" | ") + "]) -- item steps must be inlined with human-readable " +
        "titles (user ruling A: mapped to Chinese, unmapped shown verbatim)"
      );
    }
  }
  if (!block.steps.some((step) => step.gloss === "工具步骤")) {
    failures.push("D1: no rendered item step carries the 工具步骤 kind gloss (got [" + block.steps.map((step) => step.gloss).join(" | ") + "])");
  }
  // Lane dedup (design section 2.1-5). Every activity in this pass belongs to a turn
  // that renders as a round block -- the archived stream's items belong to the
  // archived trajectory turn, the seeded ones to the seeded item turn -- so the
  // flow-bottom lane must be empty. Before this change the seeded item sat there as
  // "正在执行：工程操作" while its step was invisible everywhere.
  const laneTexts = (sample.laneItems || []).map((item) => item.text);
  if (laneTexts.length > 0) {
    failures.push(
      "D1 lane dedup: the flow-bottom activity lane still renders " + laneTexts.length + " item(s) ([" +
      laneTexts.join(" | ") + "]) -- items already inlined in the round block must not be shown a second time " +
      "(design section 2.1-5)"
    );
  }
  notes.push("activity lane at sample time: [" + laneTexts.join(" | ") + "] (empty = owned items stayed in the round block)");
  return { failures, notes };
}

function parseWaitSeconds(text) {
  const match = String(text || "").match(/已等\s*(\d+)s/);
  return match ? Number(match[1]) : null;
}

function parseWorkSeconds(meta) {
  const match = String(meta || "").match(/已工作\s*(\d+)s/);
  return match ? Number(match[1]) : null;
}

// C1 residency: the seeded live-at-slice-boundary turn must render an explicit
// waiting row whose wording matches the object it is waiting for, whose clock
// advances once per second, and whose head meta keeps 已工作 frozen at the
// closed work slice (waiting must never be counted as work). The row is
// display-only: user ruling B keeps the 「继续」 button out.
function checkC1(result, options) {
  const failures = [];
  const notes = [];
  if (!result.appeared) {
    failures.push("C1: the seeded residency turn (" + options.turnId + ") never rendered a .trace-block");
    return { failures, notes };
  }
  const blockOf = (sample) => ((sample && sample.blocks) || []).find((block) => block.turnId === options.turnId) || null;
  const first = blockOf(result.first);
  const second = blockOf(result.second);
  if (!first || !second) {
    failures.push(
      "C1: the seeded residency block left the DOM between the two samples (first=" + Boolean(first) +
      ", second=" + Boolean(second) + ")"
    );
    return { failures, notes };
  }
  notes.push(
    "turn=" + options.turnId + " cls=\"" + first.cls + "\" meta=\"" + first.meta + "\" wait=\"" + first.wait + "\""
  );
  if (!/(^|\s)is-live(\s|$)/.test(first.cls)) {
    failures.push(
      "C1: the residency block is not live (cls=\"" + first.cls + "\") -- a turn parked at a slice boundary " +
      "before its terminal event must stay live"
    );
  }
  const expected = new RegExp("^" + options.expectText + "（已等 \\d+s）$");
  if (!expected.test(first.wait || "")) {
    failures.push(
      "C1 waiting row: expected \"" + options.expectText + "（已等 Xs）\" in the rendered residency row, got \"" +
      (first.wait || "(no .trace-wait row)")
      + "\" -- the wording must name the object being awaited"
    );
  }
  const firstElapsed = parseWaitSeconds(first.wait);
  const secondElapsed = parseWaitSeconds(second.wait);
  if (firstElapsed === null || secondElapsed === null) {
    failures.push(
      "C1 waiting clock: could not read the elapsed seconds from the two samples (\"" + (first.wait || "") +
      "\" / \"" + (second.wait || "") + "\")"
    );
  } else if (secondElapsed > firstElapsed) {
    notes.push(
      "waiting clock advanced by the app's own per-second timer: 已等 " + firstElapsed + "s -> " + secondElapsed +
      "s across the " + ((result.second.sampledAtMs - result.first.sampledAtMs) / 1000).toFixed(1) + "s gap between samples"
    );
  } else {
    failures.push(
      "C1 waiting clock: the waiting row did not advance between two samples ~1.5s apart (已等 " + firstElapsed +
      "s -> " + secondElapsed + "s)"
    );
  }
  const workFirst = parseWorkSeconds(first.meta);
  const workSecond = parseWorkSeconds(second.meta);
  if (workFirst === null) {
    failures.push("C1 head meta: no 已工作 Xs in the live head meta (\"" + first.meta + "\")");
  } else if (workSecond === null || workSecond !== workFirst) {
    failures.push(
      "C1 head meta: 已工作 must stay frozen at the closed work slice while parked, got " + workFirst +
      "s -> " + (workSecond === null ? "n/a" : workSecond + "s")
    );
  } else {
    notes.push("head meta 已工作 " + workFirst + "s stayed frozen across both samples (waiting is not counted as work)");
  }
  if ((first.cursors || 0) > 0) {
    failures.push(
      "C1: the residency block still renders a live cursor (" + first.cursors + ") -- waiting must not be dressed up as work"
    );
  }
  if ((first.waitButtons || 0) > 0) {
    failures.push(
      "C1: the waiting row renders a button (" + first.waitButtons + ") -- user ruling B keeps it a display-only row"
    );
  }
  return { failures, notes };
}

function checkB2(retractResult) {
  const failures = [];
  const notes = [];
  if (!retractResult || !retractResult.appeared) {
    failures.push("B2: a settled task did not render a plan bar at all, so the retraction could not be observed");
    return { failures, notes };
  }
  const bar = retractResult.sample ? retractResult.sample.planBar : null;
  if (bar) {
    notes.push("settled bar at first sample: phase=" + bar.phase + " cls=\"" + bar.cls + "\" gapToComposerTop=" + (bar.gapToComposerTop === null ? "n/a" : bar.gapToComposerTop.toFixed(1) + "px"));
    if (bar.phase !== "terminal") {
      failures.push("B2: a settled task rendered as phase=" + bar.phase + " instead of terminal");
    }
    if (bar.spinning) {
      failures.push("B2: a settled task still renders a live spinner icon");
    }
    if (bar.bottom > (retractResult.sample.composer ? retractResult.sample.composer.top : Infinity)) {
      failures.push("B2 position: the settled bar also renders below the composer top");
    }
  }
  if (!retractResult.retracted) {
    failures.push("B2 lifecycle: the plan bar stayed on screen after the task settled (no retraction within the wait window) -- the user's ruling is that the bar stops when the task ends");
  } else {
    notes.push("retraction observed: the settled bar left the DOM inside the wait window (dwell " + "6s + fade)");
  }
  return { failures, notes };
}

// TRAJ-IMPL-3 (design section 2.3, card 2026-09-14): the PERSISTENCE shape is a turn
// that runs, produces item steps and then REACHES ITS TERMINAL EVENT -- the moment the
// close-out path writes one receipt row. The archived transcript has no terminal
// item-only turn, so the boundary events are seeded for a NEW turn id and replayed
// through the very same GET /agent/events contract the app polls (same device as the
// TRAJ-IMPL-1/2 passes). The work slice is exactly 30 s, so the rendered row is a
// deterministic string ("执行完成 · 2 步 · 30.0s") rather than a moving target.
function terminalTurnFixtureEvents(options) {
  const now = Date.now();
  const runId = options.turnId;
  const base = Number(options.baseSeq) || 400;
  const at = (offsetMs) => new Date(now - 40_000 + offsetMs).toISOString();
  const common = { conversation_id: conversationId, goal_id: runId, run_id: runId, turn_id: runId, source_turn_id: runId };
  return [
    { ...common, seq: base + 1, type: "turn.started", item_type: "turn", status: "running", created_at: at(0) },
    // Two tool calls of one run. Faithful to the forensic event shape: item_id is
    // reused across calls of the same run, logical_message_id tells them apart.
    { ...common, seq: base + 2, type: "item.started", item_id: "tool_step_1", logical_message_id: "agent_item:" + runId + ":tool_step_1", item_type: "daw_action", status: "running", created_at: at(1_000), payload: { tool: "ccb.observation_catalog" } },
    { ...common, seq: base + 3, type: "item.completed", item_id: "tool_step_1", logical_message_id: "agent_item:" + runId + ":tool_step_1", item_type: "daw_action", status: "completed", created_at: at(3_000), payload: { command_name: "ccb_observation_catalog" } },
    { ...common, seq: base + 4, type: "item.started", item_id: "tool_step_1", logical_message_id: "agent_item:" + runId + ":tool_step_2", item_type: "daw_action", status: "running", created_at: at(5_000), payload: { tool: "mix_tick" } },
    { ...common, seq: base + 5, type: "item.completed", item_id: "tool_step_1", logical_message_id: "agent_item:" + runId + ":tool_step_2", item_type: "daw_action", status: "completed", created_at: at(9_000), payload: { command_name: "mix_tick" } },
    // Terminal event: the step count and the work slice of the receipt are fixed here
    // (2 steps, 30.0 s) -- no waiting, so park_ms must stay null.
    { ...common, seq: base + 6, type: "turn.completed", item_type: "turn", status: "completed", created_at: at(30_000) }
  ];
}

const RECEIPT_ROW_FIELDS = ["activity_count", "park_ms", "started_at", "status", "step_count", "turn_id", "work_ms"];

// CONV-ID-BOOT-1 (card 2026-09-14): the refresh-loss shape the user hit is a turn
// parked waiting for the A/B audition judgment. The archived transcript has no
// audition events, so the boundary events are seeded for a NEW turn id and replayed
// through the same GET /agent/events contract: an open turn, an audition.ready
// session with candidates A/B, and the unresolved user_judgment.requested that ties
// them together. No terminal event -- the turn is still waiting, exactly the state
// whose refresh used to lose the trace block, the A/B card and the receipts.
function auditionFixtureEvents(options) {
  const now = Date.now();
  const startedAt = new Date(now - 60_000).toISOString();
  const endedAt = new Date(now - 5_000).toISOString();
  const runId = options.turnId;
  const sessionId = "audition:" + runId;
  const base = Number(options.baseSeq) || 500;
  const common = { conversation_id: conversationId, goal_id: runId, run_id: runId, turn_id: runId, source_turn_id: runId };
  return [
    { ...common, seq: base + 1, type: "turn.started", item_type: "turn", status: "running", created_at: startedAt },
    {
      ...common, seq: base + 2, type: "trajectory.turn.started", item_id: "turn:" + runId, status: "running", created_at: startedAt,
      payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "turn:" + runId, turn_id: runId, node_kind: "turn", phase: "framing", status: "running" }
    },
    {
      ...common, seq: base + 3, type: "audition.ready", item_id: sessionId, status: "ready", created_at: endedAt,
      payload: {
        schema_version: "vit.kernel_audition.v1",
        session: {
          session_id: sessionId, conversation_id: conversationId, turn_id: runId, round_id: "round-1",
          project_revision: "revision-e2e", status: "ready",
          candidates: [
            { id: "candidate-a", label: "A", status: "ready", preview_ref: "a" },
            { id: "candidate-b", label: "B", status: "ready", preview_ref: "b" }
          ]
        }
      }
    },
    {
      ...common, seq: base + 4, type: "trajectory.user_judgment.requested", item_id: "judgment:" + runId, status: "waiting_for_user", created_at: endedAt,
      payload: {
        schema_version: "vit.observable_trajectory.v1", trace_node_id: "judgment:" + runId, turn_id: runId,
        node_kind: "user_judgment", phase: "user_judgment", status: "waiting_for_user",
        summary: "static_eq · A/B 试听判定", details: { audition_session_id: sessionId }
      }
    }
  ];
}

function parseReceiptLedger(raw) {
  if (typeof raw !== "string" || raw === "") return null;
  try {
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === "object" ? parsed : null;
  } catch {
    return null;
  }
}

// E1 (TRAJ-IMPL-3, the card's rendered-surface claim): "after a refresh the terminal
// receipt row is still there (hydrated from the ledger while the live state is gone)".
//
// The pass runs twice in ONE browser context: phase 1 drives the seeded turn to its
// terminal event (the receipt is written, the live block is on screen and the row must
// NOT be duplicated next to it), phase 2 reloads the page with a stream that no longer
// carries that turn (trajectory events are transient by protocol) -- live state gone,
// ledger still in localStorage. The row must be back on screen, display-only, carrying
// state/counts/duration and no message body.
function checkE1(result, options) {
  const failures = [];
  const notes = [];
  const ledger = parseReceiptLedger(result.ledgerRaw);
  if (!ledger) {
    failures.push("E1 ledger: no vit.turn_receipts.v1 payload in localStorage after the seeded turn reached its terminal event");
  } else {
    notes.push("ledger key=" + (result.ledgerRawKey || "(unknown)") + " rows=" + ((ledger.receipts || []).length));
    const row = (ledger.receipts || []).find((item) => item && item.turn_id === options.turnId) || null;
    if (!row) {
      failures.push("E1 ledger: no row for turn " + options.turnId + " (rows: [" + (ledger.receipts || []).map((item) => item && item.turn_id).join(", ") + "]) -- the terminal close-out must write exactly one row");
    } else {
      const fields = Object.keys(row).sort().join(",");
      if (fields !== RECEIPT_ROW_FIELDS.join(",")) {
        failures.push("E1 ledger fields: expected exactly [" + RECEIPT_ROW_FIELDS.join(", ") + "], got [" + fields + "] -- the row carries state/counts/duration only, never message text");
      }
      notes.push(
        "row: status=" + row.status + " step_count=" + row.step_count + " activity_count=" + row.activity_count +
        " work_ms=" + row.work_ms + " park_ms=" + row.park_ms + " started_at=" + row.started_at
      );
      if (row.status !== "completed") failures.push("E1 ledger status: expected completed, got " + row.status);
      if (row.step_count !== options.expectStepCount) failures.push("E1 ledger step_count: expected " + options.expectStepCount + ", got " + row.step_count);
      if (row.activity_count !== options.expectActivityCount) failures.push("E1 ledger activity_count: expected " + options.expectActivityCount + ", got " + row.activity_count);
      if (row.work_ms !== options.expectWorkMs) failures.push("E1 ledger work_ms: expected " + options.expectWorkMs + ", got " + row.work_ms);
      if (row.park_ms !== null) failures.push("E1 ledger park_ms: this turn has no residency segment, so park_ms must stay null, got " + row.park_ms);
      if (typeof row.started_at !== "number" || !(row.started_at > 0)) failures.push("E1 ledger started_at: expected the turn's start epoch ms, got " + row.started_at);
    }
    if (/[\u4e00-\u9fff]/.test(result.ledgerRaw)) {
      failures.push("E1 ledger text: the stored ledger contains CJK text -- the receipt must carry state/counts/duration only (message body belongs to the bubble)");
    }
  }

  // Phase 1: live state present -> the live surface wins, the receipt row must not double it.
  const liveBlock = (result.live.blocks || []).find((block) => block.turnId === options.turnId) || null;
  if (!liveBlock) {
    failures.push("E1 live phase: the seeded turn rendered no .trace-block, so the same-key dedup could not be observed");
  }
  const liveRow = (result.live.receipts || []).find((row) => row.turnId === options.turnId) || null;
  if (liveRow) {
    failures.push("E1 same-key dedup: the receipt row renders while the turn's live block is on screen ('" + liveRow.text + "') -- a live turn must not be shown twice");
  } else {
    notes.push("live phase: block present, no receipt row for the same key");
  }

  // Phase 2: live state gone -> the ledger is the only thing that can bring the row back.
  const blockAfterReload = (result.hydrated.blocks || []).find((block) => block.turnId === options.turnId) || null;
  if (blockAfterReload) {
    failures.push("E1 hydration phase: the turn's live block is still rendered after the reload -- the pass would not be testing hydration (premise broken)");
  }
  const rowAfterReload = (result.hydrated.receipts || []).find((row) => row.turnId === options.turnId) || null;
  if (!rowAfterReload) {
    failures.push(
      "E1 hydration: after the reload the terminal receipt row is GONE (no .trace-receipt[data-turn-id=" + options.turnId + "] in the rendered DOM; rows on screen: [" +
      (result.hydrated.receipts || []).map((row) => row.turnId).join(", ") + "]) -- the ledger must hydrate the receipt of a turn whose live state no longer exists (design section 2.3)"
    );
  } else {
    notes.push("hydrated row: cls=\"" + rowAfterReload.cls + "\" text=\"" + rowAfterReload.text + "\" occludedByComposer=" + rowAfterReload.occludedByComposer);
    if (rowAfterReload.text !== options.expectText) {
      failures.push("E1 hydration text: expected \"" + options.expectText + "\", got \"" + rowAfterReload.text + "\"");
    }
    if (rowAfterReload.buttons > 0) {
      failures.push("E1 hydration: the receipt row renders a button (" + rowAfterReload.buttons + ") -- user ruling C keeps it a static single line (details are phase two)");
    }
    if (rowAfterReload.cursors > 0) {
      failures.push("E1 hydration: the receipt row renders a live cursor (" + rowAfterReload.cursors + ") -- a persisted receipt is not live");
    }
    if (/is-live/.test(rowAfterReload.cls)) {
      failures.push("E1 hydration: the receipt row carries is-live (cls=\"" + rowAfterReload.cls + "\") -- it is the collapsed static form");
    }
    if (rowAfterReload.occludedByComposer === true) {
      failures.push("E1 hydration: the receipt row is covered by the composer (bottom=" + rowAfterReload.bottom.toFixed(1) + " > composer top=" + (result.hydrated.composer ? result.hydrated.composer.top.toFixed(1) : "n/a") + ") -- the persisted row must be readable");
    }
  }
  return { failures, notes };
}

// CONV-ID-BOOT-1: F1 is the bare-boot + preset-anchor pass -- the exact refresh
// condition from the live probe (20260914_refresh_view). localStorage is seeded
// with ONLY the real-scope anchor bucket (the unsaved placeholder bucket stays
// empty, as it is on the user's machine only when the scope materialized during
// the previous session), the page opens /app/ with NO conversation_id parameter,
// and the events endpoint is conversation-strict like the real agent. The app
// must restore the anchored conversation id (not regenerate one), replay
// /agent/events under it, and render the trace blocks and the A/B card.
// AUDITION-UNSTICK-1 (card 2026-09-14): the user-hit shapes are a session the
// user STOPPED (event flow seq20-30: prepare -> ready -> select A playing ->
// stopped, after which every candidate control was a dead click) and a session
// still PREPARING (candidates warming, no ready echo yet -- the visual void the
// user read as "the trajectory timer froze"). Both are seeded as boundary events
// for NEW session ids and replayed through the same GET /agent/events contract;
// the assertions are render-only (control enablement, reasons, chips). The
// server-side stopped->ready recovery itself is covered by the Go handler test
// with the kernel contract faked at the VSP boundary (no kernel in this stack).
function auditionUnstickFixtureEvents(options) {
  const now = Date.now();
  const startedAt = new Date(now - 70_000).toISOString();
  const midAt = new Date(now - 40_000).toISOString();
  const stoppedAt = new Date(now - 10_000).toISOString();
  const base = Number(options.baseSeq) || 600;
  const common = { conversation_id: conversationId, goal_id: options.turnId, run_id: options.turnId, turn_id: options.turnId, source_turn_id: options.turnId };
  const readySession = (sessionId, status, activeCandidateId, candidateStatus) => ({
    session_id: sessionId, conversation_id: conversationId, turn_id: options.turnId, round_id: "round-1",
    project_revision: "revision-e2e", status, active_candidate_id: activeCandidateId,
    candidates: [
      { id: "candidate-a", label: "A", status: candidateStatus, preview_ref: "preview-a" },
      { id: "candidate-b", label: "B", status: candidateStatus, preview_ref: "preview-b" }
    ]
  });
  return [
    { ...common, seq: base + 1, type: "turn.started", item_type: "turn", status: "running", created_at: startedAt },
    {
      ...common, seq: base + 2, type: "audition.ready", item_id: options.stoppedSession, status: "ready", created_at: midAt,
      payload: { schema_version: "vit.kernel_audition.v1", session: readySession(options.stoppedSession, "ready", "", "ready") }
    },
    {
      ...common, seq: base + 3, type: "trajectory.user_judgment.requested", item_id: "judgment:" + options.turnId, status: "waiting_for_user", created_at: midAt,
      payload: {
        schema_version: "vit.observable_trajectory.v1", trace_node_id: "judgment:" + options.turnId, turn_id: options.turnId,
        node_kind: "user_judgment", phase: "user_judgment", status: "waiting_for_user",
        summary: "static_eq · A/B 试听判定", details: { audition_session_id: options.stoppedSession }
      }
    },
    {
      ...common, seq: base + 4, type: "audition.selected", item_id: options.stoppedSession, status: "playing", created_at: midAt,
      payload: { schema_version: "vit.kernel_audition.v1", session: readySession(options.stoppedSession, "playing", "candidate-a", "ready") }
    },
    {
      ...common, seq: base + 5, type: "audition.stopped", item_id: options.stoppedSession, status: "stopped", created_at: stoppedAt,
      payload: { schema_version: "vit.kernel_audition.v1", session: readySession(options.stoppedSession, "stopped", "candidate-a", "ready") }
    },
    {
      ...common, seq: base + 6, type: "audition.prepare", item_id: options.preparingSession, status: "preparing", created_at: stoppedAt,
      payload: {
        schema_version: "vit.kernel_audition.v1",
        session: {
          session_id: options.preparingSession, conversation_id: conversationId, turn_id: options.turnId, round_id: "round-2",
          project_revision: "revision-e2e", status: "preparing",
          candidates: [
            { id: "candidate-a", label: "A", status: "preparing" },
            { id: "candidate-b", label: "B", status: "preparing" }
          ]
        }
      }
    }
  ];
}

function checkF1(result, options) {
  const failures = [];
  const notes = [];
  const sample = result.sample;
  if (!result.realScopeKey) {
    failures.push("F1 setup: the URL-bound learning context never materialized a real scope bucket -- the bare-boot pass cannot assert the anchor");
    return { failures, notes };
  }
  const anchored = (sample.scopeBuckets || {})[result.realScopeKey] || "";
  notes.push("anchor bucket " + result.realScopeKey + " -> \"" + anchored + "\" (expected " + options.conversationId + ")");
  if (anchored !== options.conversationId) {
    failures.push(
      "F1 anchor: the bare boot OVERWROTE the stored scope mapping -- bucket now holds \"" + anchored +
      "\" instead of \"" + options.conversationId + "\". This is the defect the card pins: a fresh random id replaces the " +
      "anchored conversation, so every refresh loses the trajectory blocks, the A/B cards and the receipts"
    );
  }
  if (!result.appeared || (sample.blocks || []).length === 0) {
    failures.push(
      "F1 trace: no .trace-block rendered on the bare boot (appeared=" + result.appeared + ") -- the events replay never " +
      "ran under the anchored conversation id"
    );
  } else {
    const turnIds = (sample.blocks || []).map((block) => block.turnId).filter(Boolean);
    notes.push("trace blocks rendered on the bare boot: [" + turnIds.join(", ") + "]");
    for (const expectedTurn of options.expectTurnIds) {
      if (!turnIds.includes(expectedTurn)) {
        failures.push("F1 trace: expected turn " + expectedTurn + " to render after the bare-boot restore, got [" + turnIds.join(", ") + "]");
      }
    }
    for (const block of sample.blocks) {
      if (!block.turnId) {
        failures.push("F1 trace: a rendered .trace-block carries no data-turn-id");
      }
    }
  }
  const card = (sample.auditionCards || []).find((item) => item.session === options.expectAuditionSession) || null;
  if (!card) {
    failures.push(
      "F1 audition: no A/B judge card with data-audition-session=" + options.expectAuditionSession +
      " on the bare boot (cards on screen: [" + (sample.auditionCards || []).map((item) => item.session).join(", ") + "])"
    );
  } else {
    notes.push("A/B card rendered: session=" + card.session + " status=" + card.status + " text=\"" + card.text + "\"");
    if (card.text.indexOf("A/B 快速对比") < 0) {
      failures.push("F1 audition: the rendered card does not carry the A/B 快速对比 surface (text=\"" + card.text + "\")");
    }
    if (card.text.indexOf("待判定") < 0) {
      failures.push("F1 audition: the rendered card is not in the waiting-for-judgment form (expected 待判定, text=\"" + card.text + "\")");
    }
  }
  return { failures, notes };
}

// AUDITION-UNSTICK-1: G1 pins the two user-hit shapes on the rendered surface.
//   * stopped session -- the A/B controls must NOT be dead clicks: the switch
//     buttons and both play buttons render enabled (a click restarts playback
//     through the server-side stopped->ready recovery), and the card states the
//     stopped status instead of falling silent.
//   * preparing session -- the card must say 「正在准备 A/B 试听…」 explicitly and
//     keep the A/B controls in place but disabled WITH the reason attached, so
//     the warm-up window is visibly alive instead of reading as a frozen stack.
// Render-only by design: this stack has no kernel behind the agent, so the
// select round-trip belongs to the Go handler test at the VSP boundary.
function checkG1(result, options) {
  const failures = [];
  const notes = [];
  const sample = result.sample;
  const controls = sample.auditionControls || [];
  const stopped = controls.find((card) => card.session === options.stoppedSession) || null;
  if (!stopped) {
    failures.push(
      "G1 stopped: no A/B card rendered for session " + options.stoppedSession +
      " (cards: [" + controls.map((card) => card.session).join(", ") + "])"
    );
  } else {
    notes.push("stopped card: status=" + stopped.status + " chips=[" + stopped.chips.join(" | ") + "]");
    if (stopped.status !== "stopped") {
      failures.push("G1 stopped: card data-status=" + stopped.status + " instead of stopped");
    }
    for (const button of stopped.switchButtons) {
      if (button.disabled) {
        failures.push(
          "G1 stopped: switch button " + button.text + " is disabled -- after a stop the user must be able to pick " +
          "either candidate again (the dead-click defect)"
        );
      }
    }
    for (const [index, button] of stopped.playButtons.entries()) {
      if (button.disabled) {
        failures.push("G1 stopped: play button #" + (index + 1) + " is disabled -- clicking it must restart that candidate");
      }
    }
    if (!stopped.chips.some((chip) => chip.indexOf("已停止") >= 0)) {
      failures.push("G1 stopped: the card does not state the stopped status (chips: [" + stopped.chips.join(" | ") + "])");
    }
  }
  const preparing = controls.find((card) => card.session === options.preparingSession) || null;
  if (!preparing) {
    failures.push(
      "G1 preparing: no A/B card rendered for session " + options.preparingSession +
      " (cards: [" + controls.map((card) => card.session).join(", ") + "])"
    );
  } else {
    notes.push("preparing card: status=" + preparing.status + " chips=[" + preparing.chips.join(" | ") + "]");
    if (!preparing.chips.some((chip) => chip.indexOf("正在准备") >= 0)) {
      failures.push("G1 preparing: the card does not show 「正在准备 A/B 试听…」 (chips: [" + preparing.chips.join(" | ") + "]) -- the warm-up window must be explicit");
    }
    for (const button of preparing.switchButtons) {
      if (!button.disabled) {
        failures.push("G1 preparing: switch button " + button.text + " is enabled while candidates are still preparing");
      } else if (!button.title) {
        failures.push("G1 preparing: disabled switch button " + button.text + " carries no reason -- a disabled control must explain itself");
      }
    }
  }
  return { failures, notes };
}

// --------------------------------------------------------------- main flow

async function main() {
  const report = {
    schema_version: "webui_rendered_dom_smoke.v1",
    started_at: new Date().toISOString(),
    agent_base: agentBase,
    conversation_id: conversationId,
    viewport,
    playwright_module: "",
    browser: "",
    browser_version: "",
    events_sha256: "",
    seeded: null,
    passes: {},
    assertions: [],
    run_dir: outDir
  };

  const { module: chromium, path: pwPath } = resolvePlaywright();
  report.playwright_module = pwPath;
  log("playwright module:", pwPath);

  const graphFixture = JSON.parse(readFileSync(graphFixturePath, "utf-8"));
  const eventsFixture = JSON.parse(readFileSync(eventsFixturePath, "utf-8"));
  const replayBody = JSON.stringify({
    status: eventsFixture.status || "ok",
    events: eventsFixture.events || [],
    next_seq: eventsFixture.next_seq ?? (eventsFixture.events || []).length
  });
  report.events_sha256 = sha256(replayBody);
  log("events replayed:", (eventsFixture.events || []).length, "sha256:", report.events_sha256.slice(0, 16));

  const health = await getJSON("/health");
  log("agent health:", JSON.stringify(health));
  const state = await getJSON("/agent/ui/state");
  report.seeded = seedConversation(graphFixture, state);
  log("seeded graph nodes:", report.seeded.commitsWritten, "skipped:", report.seeded.nodesSkipped.length);

  const hydrated = await getJSON("/agent/ui/state");
  const hydratedMessages = ((hydrated.project_history || {}).conversation_messages) || [];
  report.hydrated_message_count = hydratedMessages.length;
  const turnOneUser = hydratedMessages.find((message) => String(message.turn_id || "") === "run_bab2dcdacb41bdc1" && message.role === "user");
  report.hydrated_turn_one_user = turnOneUser
    ? { created_at: turnOneUser.created_at, text: String(turnOneUser.content || "").slice(0, 40) }
    : null;
  if (!turnOneUser && !driveTurn) {
    throw new Error(
      "hydration precondition failed: the archived turn-1 user message (run_bab2dcdacb41bdc1) is not in " +
      "project_history.conversation_messages, so the anchoring assertion would not test the archived state"
    );
  }
  log("hydrated messages:", hydratedMessages.length, "turn-1 user created_at:", turnOneUser ? turnOneUser.created_at : "(control mode: transcript is built by the driven turn)");

  const { browser, label, version } = await launchBrowser(chromium);
  report.browser = label;
  report.browser_version = version;
  log("browser:", label, version);

  let seededEventsRequests = 0;
  const installReplay = async (context, options = {}) => {
    // TRAJ-IMPL-1: a pass may append seeded boundary events (the residency shape)
    // to the archived stream. The archived stream itself is never edited: the
    // extra events are carried here and listed in the report.
    const extraEvents = Array.isArray(options.extraEvents) ? options.extraEvents : [];
    const all = [...(eventsFixture.events || []), ...extraEvents];
    // The app advances its since-cursor from next_seq, so a merged stream must
    // report the merged maximum. For the archived-only stream this is exactly
    // the value the fixture already carried.
    const nextSeq = all.reduce(
      (maximum, event) => Math.max(maximum, Number(event.seq) || 0),
      Number(eventsFixture.next_seq) || 0
    );
    // CONV-ID-BOOT-1: strictConversation mirrors the real agent's contract -- the
    // events endpoint serves only the asked-for conversation. Without it a bare
    // boot that regenerated a random id would still be handed the archived stream
    // and the defect would hide behind a rendered trajectory under the WRONG id.
    const strictConversation = options.strictConversation === true;
    await context.route("**/agent/events*", async (route) => {
      seededEventsRequests += 1;
      // Serve the events with the same since/limit contract the agent
      // implements, so the app's incremental polling works exactly as it does
      // against the real endpoint.
      const requestURL = new URL(route.request().url());
      const askedConversation = requestURL.searchParams.get("conversation_id") || "";
      const since = Number(requestURL.searchParams.get("since") || "0");
      const limit = Number(requestURL.searchParams.get("limit") || "120");
      const mismatch = strictConversation && askedConversation !== conversationId;
      const body = JSON.stringify({
        status: eventsFixture.status || "ok",
        events: mismatch ? [] : all.filter((event) => Number(event.seq) > since).slice(0, limit),
        next_seq: mismatch ? 0 : nextSeq
      });
      await route.fulfill({ status: 200, contentType: "application/json; charset=utf-8", body });
    });
  };

  // PLANBAR-1: the archived conversation carries no task runtime state, so the
  // plan bar would never render in the replay passes. The trajectory below is
  // the agent's own schema (vit.task_runtime_trajectory.v1); it is fulfilled at
  // the network layer for GET /agent/runtime/status, and the chain-live signal
  // (goal.status) is patched on the real /agent/ui/state response. Everything
  // else -- bundle, agent process, DOM, composer, layout -- stays real.
  const planBarTaskFixture = (state) => {
    const stamp = "2026-09-13T12:30:00Z";
    const settled = state === "settled";
    return {
      schema_version: "vit.task_runtime_trajectory.v1",
      task: {
        task_id: "task_e2e_planbar1",
        goal_id: "goal_e2e_planbar1",
        run_id: "run_e2e_planbar1",
        original_intent: "PLANBAR-1 rendered-surface smoke: the plan bar must sit above the input box and stop when the task ends.",
        status: settled ? "completed" : "running",
        updated_at: stamp
      },
      run: {
        run_id: "run_e2e_planbar1",
        current_slice_id: "slice_e2e_planbar1",
        current_turn_id: "run_e2e_planbar1",
        slices: [{ slice_id: "slice_e2e_planbar1", sequence: 1, max_turns: 4, status: "running" }],
        turns: [{ turn_id: "run_e2e_planbar1", slice_id: "slice_e2e_planbar1", sequence: 1, source: "user", status: "running" }]
      },
      semantic: {
        state: state,
        revision: settled ? 9 : 8,
        project_revision: "revision-e2e",
        summary: "PLANBAR-1 rendered-surface smoke state.",
        updated_at: stamp
      },
      continuation: {},
      capability_route: { capacity_assessment: { selected_capability: "free_state", capacity_level: "within_free_state" } },
      transitions: [],
      updated_at: stamp
    };
  };

  const installTaskState = async (context, trajectory, goalStatus) => {
    await context.route("**/agent/runtime/status*", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json; charset=utf-8",
        body: JSON.stringify({
          status: "ok",
          service: "VitAgent",
          kernel: { connected: true, status: "ok" },
          goal: { status: goalStatus },
          task_trajectory: trajectory
        })
      });
    });
    // The real ui/state response, with only the chain-live signal patched: the
    // bar's live phase is driven by agentTurnRunning, which reads goal.status.
    await context.route("**/agent/ui/state*", async (route) => {
      const response = await route.fetch();
      const body = await response.json().catch(() => ({}));
      await route.fulfill({ response, json: { ...body, goal: { ...(body.goal || {}), status: goalStatus } } });
    });
  };

  const runPass = async (name, options) => {
    const context = await browser.newContext({ viewport });
    if (options.replay !== false) {
      await installReplay(context);
    }
    const page = await context.newPage();
    if (options.seedLocalStorage) {
      await page.addInitScript(() => {
        for (const key of Object.keys(localStorage)) {
          if (key.startsWith("ask_vit_conversation_messages")) localStorage.removeItem(key);
        }
      });
    }
    const url = agentBase + "/app/?conversation_id=" + encodeURIComponent(conversationId);
    await page.goto(url, { waitUntil: "domcontentloaded" });
    if (options.driveTurn) {
      // A real turn: type into the real composer, submit, wait for the agent to
      // settle, then reload so the transcript is re-hydrated from the agent.
      const composer = page.locator(".composer textarea").first();
      await composer.waitFor({ state: "visible", timeout: settleMs });
      await composer.fill(driveTurnMessage);
      await composer.press("Enter");
      log("driven turn submitted, waiting for settlement (budget " + driveTurnBudgetMs + "ms)");
      const deadline = Date.now() + driveTurnBudgetMs;
      while (Date.now() < deadline) {
        await page.waitForTimeout(1000);
        const busy = await page.evaluate(() => Boolean(document.querySelector(".trace-block.is-live, .composer .spin, .activity-lane-item.pending")));
        if (!busy) break;
      }
      await page.waitForTimeout(1500);
      await page.reload({ waitUntil: "domcontentloaded" });
    }
    await page.waitForSelector(".trace-block", { timeout: settleMs }).catch(() => {});
    await page.waitForTimeout(2500);
    const sample = await page.evaluate(DOM_PROBE);
    await page.screenshot({ path: join(outDir, "dom-" + name + ".png"), fullPage: false });
    writeFileSync(join(outDir, "dom-" + name + ".json"), JSON.stringify(sample, null, 2), "utf-8");
    await context.close();
    return sample;
  };

  // Two dedicated passes: one measures the T6 slot with a running task, one
  // observes a settled task leaving the screen. Both sample the DOM the browser
  // really rendered (DOM_PROBE), not a model of it.
  const runPlanBarPass = async (name, options) => {
    const context = await browser.newContext({ viewport });
    await installTaskState(context, planBarTaskFixture(options.state), options.goalStatus);
    const page = await context.newPage();
    await page.goto(agentBase + "/app/?conversation_id=" + encodeURIComponent(conversationId), { waitUntil: "domcontentloaded" });
    const appeared = await page.waitForSelector(".plan-bar", { timeout: options.appearTimeoutMs }).then(() => true).catch(() => false);
    const sample = await page.evaluate(DOM_PROBE);
    await page.screenshot({ path: join(outDir, "dom-" + name + ".png") });
    writeFileSync(join(outDir, "dom-" + name + ".json"), JSON.stringify(sample, null, 2), "utf-8");
    let retracted = null;
    if (options.expectRetract) {
      retracted = await page.waitForSelector(".plan-bar", { state: "detached", timeout: options.retractTimeoutMs }).then(() => true).catch(() => false);
    }
    await context.close();
    return { appeared, sample, retracted };
  };

  // TRAJ-IMPL-1: one pass per waiting object (continuation / unresolved A/B
  // judgment). Each samples the DOM twice ~1.5s apart so the per-second clock is
  // observed on the rendered surface instead of being taken on faith.
  // TRAJ-IMPL-2: one pass for the non-experiment turn (item.* events only, no
  // trajectory record at all). Sampled once, during execution (no terminal event is
  // seeded), which is exactly the window the card is about.
  const runItemStepsPass = async (name, options) => {
    const context = await browser.newContext({ viewport });
    const seeded = chatOnlyItemStepsFixtureEvents({ turnId: options.turnId, baseSeq: options.baseSeq });
    await installReplay(context, { extraEvents: seeded });
    const page = await context.newPage();
    await page.goto(agentBase + "/app/?conversation_id=" + encodeURIComponent(conversationId), { waitUntil: "domcontentloaded" });
    const appeared = await page
      .waitForSelector('.trace-block[data-turn-id="' + options.turnId + '"]', { timeout: options.appearTimeoutMs })
      .then(() => true)
      .catch(() => false);
    await page.waitForTimeout(800);
    const sample = await page.evaluate(DOM_PROBE);
    await page.screenshot({ path: join(outDir, "dom-" + name + ".png") });
    writeFileSync(join(outDir, "dom-" + name + ".json"), JSON.stringify(sample, null, 2), "utf-8");
    await context.close();
    return { appeared, sample, seeded };
  };

  const runResidencyPass = async (name, options) => {
    const context = await browser.newContext({ viewport });
    const seeded = residencyFixtureEvents({ turnId: options.turnId, baseSeq: options.baseSeq, judgment: options.judgment });
    await installReplay(context, { extraEvents: seeded });
    const page = await context.newPage();
    await page.goto(agentBase + "/app/?conversation_id=" + encodeURIComponent(conversationId), { waitUntil: "domcontentloaded" });
    const appeared = await page
      .waitForSelector('.trace-block[data-turn-id="' + options.turnId + '"]', { timeout: options.appearTimeoutMs })
      .then(() => true)
      .catch(() => false);
    await page.waitForTimeout(600);
    const first = await page.evaluate(DOM_PROBE);
    await page.screenshot({ path: join(outDir, "dom-" + name + ".png") });
    writeFileSync(join(outDir, "dom-" + name + ".json"), JSON.stringify(first, null, 2), "utf-8");
    await page.waitForTimeout(1500);
    const second = await page.evaluate(DOM_PROBE);
    writeFileSync(join(outDir, "dom-" + name + "-later.json"), JSON.stringify(second, null, 2), "utf-8");
    await context.close();
    return { appeared, first, second, seeded };
  };

  // TRAJ-IMPL-3: two phases inside ONE browser context. The ledger lives in
  // localStorage; localStorage surviving a page load while the in-memory live state
  // does not is exactly the production situation this card is about -- so phase 2 must
  // NOT use a fresh context (that would clear the ledger and prove nothing).
  const runReceiptPass = async (name, options) => {
    const context = await browser.newContext({ viewport });
    const seeded = terminalTurnFixtureEvents({ turnId: options.turnId, baseSeq: options.baseSeq });
    await installReplay(context, { extraEvents: seeded });
    const page = await context.newPage();
    await page.goto(agentBase + "/app/?conversation_id=" + encodeURIComponent(conversationId), { waitUntil: "domcontentloaded" });
    const appeared = await page
      .waitForSelector('.trace-block[data-turn-id="' + options.turnId + '"]', { timeout: options.appearTimeoutMs })
      .then(() => true)
      .catch(() => false);
    // The write is an effect of the terminal event: wait for the ledger payload itself
    // instead of sleeping a guessed amount.
    const ledgerRaw = await page
      .waitForFunction(
        (prefix) => {
          const key = Object.keys(localStorage).find((item) => item.indexOf(prefix) === 0);
          return key ? localStorage.getItem(key) : null;
        },
        "vit.turn_receipts.v1",
        { timeout: options.settleTimeoutMs }
      )
      .then((handle) => handle.jsonValue())
      .catch(() => null);
    const ledgerKey = await page
      .evaluate(() => Object.keys(localStorage).find((item) => item.indexOf("vit.turn_receipts.v1") === 0) || "")
      .catch(() => "");
    await page.waitForTimeout(600);
    const live = await page.evaluate(DOM_PROBE);
    await page.screenshot({ path: join(outDir, "dom-" + name + "-live.png") });
    writeFileSync(join(outDir, "dom-" + name + "-live.json"), JSON.stringify(live, null, 2), "utf-8");

    // Phase 2: same context (localStorage kept), but the stream no longer carries this
    // turn -- after a reload its transient events are simply not there any more.
    await context.unroute("**/agent/events*");
    await installReplay(context, { extraEvents: [] });
    await page.reload({ waitUntil: "domcontentloaded" });
    await page.waitForTimeout(2500);
    const hydrated = await page.evaluate(DOM_PROBE);
    await page.screenshot({ path: join(outDir, "dom-" + name + "-hydrated.png") });
    writeFileSync(join(outDir, "dom-" + name + "-hydrated.json"), JSON.stringify(hydrated, null, 2), "utf-8");
    await context.close();
    return { appeared, live, hydrated, ledgerRaw, ledgerRawKey: ledgerKey, seeded };
  };

  // CONV-ID-BOOT-1: the bare-boot pass. Phase 1 learns this run's real scope
  // bucket key empirically from a URL-bound context (the key is the app's own
  // stableIDPart encoding of the draft project path, which is run-specific and
  // 96-char truncated -- deriving it by hand here would duplicate app logic).
  // Phase 2 seeds ONLY that bucket with the archived conversation id in a fresh
  // context and opens /app/ bare. The events endpoint is conversation-strict in
  // both phases, mirroring the real agent: a regenerated random id gets an empty
  // buffer, exactly like the user's live refresh.
  const runBareBootPass = async (name, options) => {
    const seeded = auditionFixtureEvents({ turnId: options.turnId, baseSeq: options.baseSeq });
    const learnContext = await browser.newContext({ viewport });
    await installReplay(learnContext, { extraEvents: seeded, strictConversation: true });
    const learnPage = await learnContext.newPage();
    await learnPage.goto(agentBase + "/app/?conversation_id=" + encodeURIComponent(conversationId), { waitUntil: "domcontentloaded" });
    const learnedBlock = await learnPage
      .waitForSelector('.trace-block[data-turn-id="' + options.turnId + '"]', { timeout: options.appearTimeoutMs })
      .then(() => true)
      .catch(() => false);
    await learnPage.waitForTimeout(1500);
    const learned = await learnPage.evaluate(() =>
      Object.fromEntries(
        Object.keys(localStorage)
          .filter((key) => key.indexOf("ask_vit_conversation_id_scope") === 0)
          .map((key) => [key, localStorage.getItem(key) || ""])
      )
    );
    await learnContext.close();
    const realScopeKey = Object.keys(learned).find((key) => key !== "ask_vit_conversation_id_scope:unsaved_root") || "";
    const realScopeAnchored = learned[realScopeKey] || "";

    const context = await browser.newContext({ viewport });
    await installReplay(context, { extraEvents: seeded, strictConversation: true });
    if (realScopeKey) {
      await context.addInitScript(([key, value]) => {
        localStorage.setItem(key, value);
      }, [realScopeKey, conversationId]);
    }
    const page = await context.newPage();
    await page.goto(agentBase + "/app/", { waitUntil: "domcontentloaded" });
    const appeared = await page
      .waitForSelector('.trace-block[data-turn-id="' + options.turnId + '"]', { timeout: options.appearTimeoutMs })
      .then(() => true)
      .catch(() => false);
    await page.waitForTimeout(2500);
    const sample = await page.evaluate(DOM_PROBE);
    await page.screenshot({ path: join(outDir, "dom-" + name + ".png") });
    writeFileSync(join(outDir, "dom-" + name + ".json"), JSON.stringify(sample, null, 2), "utf-8");
    await context.close();
    return { appeared, sample, realScopeKey, realScopeAnchored, learned, learningSawSeededTurn: learnedBlock, seeded };
  };

  const assess = (prefix, sample) => {
    for (const [groupId, fn] of [["A1", checkA1], ["A2", checkA2], ["A3", checkA3]]) {
      const { failures, notes } = fn(sample);
      const id = prefix + groupId;
      report.assertions.push({ id, pass: failures.length === 0, failures, notes });
      report.passes[id] = failures.length === 0;
      log(id, failures.length === 0 ? "PASS" : "FAIL");
      for (const note of notes) log("   note:", note);
      for (const failure of failures) log("   fail:", failure);
    }
  };

  if (driveTurn) {
    report.control_mode = "drive-turn";
    report.control_turn_message = driveTurnMessage;
    // A driven turn needs the REAL event stream: replaying an archived stream
    // would advance the app's since-cursor past the new turn's own sequence
    // numbers and the block under test would never render.
    report.control_events_source = "live agent (no replay)";
    const controlSample = await runPass("control-live", { seedLocalStorage: false, driveTurn: true, replay: false });
    assess("control-live-", controlSample);
    const controlReload = await runPass("control-reload", { seedLocalStorage: true, replay: false });
    report.control_reload_localstorage_keys = controlReload.localStorageKeys;
    assess("control-reload-", controlReload);
    const controlPass = ["control-live-A1", "control-live-A2", "control-live-A3", "control-reload-A1", "control-reload-A2", "control-reload-A3"]
      .every((groupId) => report.passes[groupId] === true);
    report.passes["control"] = controlPass;
    log("control verdict:", controlPass ? "PASS (probe can go green)" : "FAIL");
    report.finished_at = new Date().toISOString();
    const controlFailed = Object.entries(report.passes).filter(([, ok]) => !ok).map(([id]) => id);
    report.verdict = controlFailed.length === 0 ? "pass" : "fail";
    report.failed_groups = controlFailed;
    writeFileSync(join(outDir, "webui_rendered_dom_report.json"), JSON.stringify(report, null, 2), "utf-8");
    await browser.close();
    log("report:", join(outDir, "webui_rendered_dom_report.json"));
    process.exitCode = controlFailed.length === 0 ? 0 : 1;
    return;
  }

  report.events_source = "archived fixture replayed for GET /agent/events";
  const liveSample = await runPass("live", { seedLocalStorage: false });
  // Hydration precondition, checked against the RENDERED flow (not the server
  // payload): the archived turn-1 user message must actually be on screen, or
  // the anchoring assertion would be measuring nothing. How many of the 18
  // hydrated rows survive into the flow is the app's own business.
  const expectedUserText = "帮低音轨做个均衡实验然后让我试听";
  const renderedTurnOneUser = liveSample.userMessages.find((message) => message.text.includes(expectedUserText));
  report.rendered_turn_one_user = renderedTurnOneUser || null;
  if (!renderedTurnOneUser) {
    throw new Error(
      "hydration precondition failed: the archived turn-1 user message (" + expectedUserText + ") is not in the " +
      "rendered flow, so the anchoring assertion would not test the archived state"
    );
  }
  log("archived turn-1 user message rendered at flowIndex", renderedTurnOneUser.index);
  assess("live-", liveSample);

  const hydrationSample = await runPass("hydration", { seedLocalStorage: true });
  report.hydration_localstorage_keys = hydrationSample.localStorageKeys;
  assess("hydration-", hydrationSample);

  // A4 is the conjunction: A1..A3 must still hold on the re-hydrated pass.
  const hydrationPass = ["A1", "A2", "A3"].every((groupId) => report.passes["hydration-" + groupId] === true);
  report.assertions.push({
    id: "A4",
    pass: hydrationPass,
    failures: hydrationPass ? [] : ["hydration pass did not keep A1..A3 (see hydration-A1..A3 failures)"],
    notes: [
      "reload re-hydrates the transcript from the agent (fresh context, localStorage cleared before load): " +
      report.hydration_localstorage_keys.filter((key) => key.startsWith("ask_vit_conversation_messages")).length +
      " stored message keys at sample time"
    ]
  });
  report.passes.A4 = hydrationPass;
  log("A4", hydrationPass ? "PASS" : "FAIL");

  // ------------------------------------------------------------- PLANBAR-1
  report.planbar_task_state_source = "injected at the network layer for GET /agent/runtime/status (vit.task_runtime_trajectory.v1); chain-live signal patched on the real /agent/ui/state";
  const record = (id, result) => {
    report.assertions.push({ id, pass: result.failures.length === 0, failures: result.failures, notes: result.notes });
    report.passes[id] = result.failures.length === 0;
    log(id, result.failures.length === 0 ? "PASS" : "FAIL");
    for (const note of result.notes) log("   note:", note);
    for (const failure of result.failures) log("   fail:", failure);
  };

  const liveBar = await runPlanBarPass("planbar-live", { state: "observation_in_progress", goalStatus: "running", appearTimeoutMs: 15000, expectRetract: false, retractTimeoutMs: 0 });
  report.planbar_live = { appeared: liveBar.appeared, bar: liveBar.sample ? liveBar.sample.planBar : null };
  record("planbar-live-B1", checkB1(liveBar.sample));

  const settledBar = await runPlanBarPass("planbar-settled", { state: "settled", goalStatus: "running", appearTimeoutMs: 15000, expectRetract: true, retractTimeoutMs: 20000 });
  report.planbar_settled = { appeared: settledBar.appeared, retracted: settledBar.retracted, bar: settledBar.sample ? settledBar.sample.planBar : null };
  record("planbar-settled-B1", checkB1(settledBar.sample));
  record("planbar-settled-B2", checkB2(settledBar));

  // ------------------------------------------------------------- TRAJ-IMPL-1
  report.residency_events_source =
    "archived stream + seeded slice-boundary events for a new turn id, replayed for GET /agent/events " +
    "(live turn, work slice closed by turn.completed, terminal event absent = the residency window)";
  const residencyContinuation = await runResidencyPass("residency-continuation", {
    turnId: "run_e2e_residency1", baseSeq: 100, judgment: false, appearTimeoutMs: 15000
  });
  report.residency_continuation = {
    appeared: residencyContinuation.appeared,
    seeded_events: residencyContinuation.seeded
  };
  record("residency-C1", checkC1(residencyContinuation, { turnId: "run_e2e_residency1", expectText: "等待续跑" }));

  const residencyAudition = await runResidencyPass("residency-audition", {
    turnId: "run_e2e_residency2", baseSeq: 200, judgment: true, appearTimeoutMs: 15000
  });
  report.residency_audition = {
    appeared: residencyAudition.appeared,
    seeded_events: residencyAudition.seeded
  };
  record("residency-audition-C1", checkC1(residencyAudition, { turnId: "run_e2e_residency2", expectText: "等待你的试听判定" }));

  // ------------------------------------------------------------- TRAJ-IMPL-2
  report.itemsteps_events_source =
    "archived stream + seeded item.* events for a turn with NO trajectory record (pure chat execution window, " +
    "terminal event deliberately absent), replayed for GET /agent/events";
  const itemStepsPass = await runItemStepsPass("itemsteps-live", {
    turnId: "run_e2e_itemsteps1", baseSeq: 300, appearTimeoutMs: 15000
  });
  report.itemsteps = { appeared: itemStepsPass.appeared, seeded_events: itemStepsPass.seeded };
  record("itemsteps-D1", checkD1(itemStepsPass, {
    turnId: "run_e2e_itemsteps1",
    expectStepTexts: ["已完成 可用观察视图清单", "已完成 混音调整", "weird.custom_tool"]
  }));

  // ------------------------------------------------------------- TRAJ-IMPL-3
  report.receipt_events_source =
    "archived stream + seeded events for a NEW turn id that reaches its terminal event (two item steps, " +
    "turn.completed = the close-out moment), replayed for GET /agent/events; the hydration phase re-serves " +
    "the archived stream only and keeps localStorage in the same browser context";
  const receiptPass = await runReceiptPass("receipts", {
    turnId: "run_e2e_receipt1", baseSeq: 400, appearTimeoutMs: 15000, settleTimeoutMs: 15000
  });
  report.receipts = {
    turn_id: "run_e2e_receipt1",
    appeared: receiptPass.appeared,
    ledger_key: receiptPass.ledgerRawKey,
    ledger_raw: receiptPass.ledgerRaw,
    live_rows: (receiptPass.live.receipts || []).map((row) => ({ turnId: row.turnId, text: row.text })),
    hydrated_rows: (receiptPass.hydrated.receipts || []).map((row) => ({ turnId: row.turnId, text: row.text })),
    hydrated_blocks: (receiptPass.hydrated.blocks || []).map((block) => block.turnId),
    seeded_events: receiptPass.seeded
  };
  record("receipts-E1", checkE1(receiptPass, {
    turnId: "run_e2e_receipt1",
    expectStepCount: 2,
    expectActivityCount: 2,
    expectWorkMs: 30000,
    expectText: "执行完成 · 2 步 · 30.0s"
  }));

  // ---------------------------------------------------------- CONV-ID-BOOT-1
  report.bareboot_events_source =
    "archived stream + seeded waiting-for-judgment events (open turn + audition.ready A/B + unresolved " +
    "user_judgment.requested) replayed conversation-strict for GET /agent/events; the bare context presets ONLY the " +
    "real-scope anchor bucket learned by the URL-bound context in the same run";
  const bareBootPass = await runBareBootPass("bareboot", {
    turnId: "run_e2e_bareboot1", baseSeq: 500, appearTimeoutMs: 15000
  });
  report.bareboot = {
    turn_id: "run_e2e_bareboot1",
    real_scope_key: bareBootPass.realScopeKey,
    learning_context_anchor: bareBootPass.realScopeAnchored,
    learning_saw_seeded_turn: bareBootPass.learningSawSeededTurn,
    appeared: bareBootPass.appeared,
    seeded_events: bareBootPass.seeded
  };
  record("bareboot-F1", checkF1(bareBootPass, {
    conversationId,
    expectTurnIds: ["run_bab2dcdacb41bdc1", "run_e2e_bareboot1"],
    expectAuditionSession: "audition:run_e2e_bareboot1"
  }));

  // ------------------------------------------------------- AUDITION-UNSTICK-1
  report.audition_unstick_events_source =
    "archived stream + seeded audition boundary events (stopped-after-playing session + preparing session) " +
    "replayed for GET /agent/events; assertions are render-only (no kernel behind this agent, the select " +
    "round-trip is covered by the Go handler test)";
  const unstickContext = await browser.newContext({ viewport });
  const unstickSeeded = auditionUnstickFixtureEvents({
    turnId: "run_e2e_unstick1", baseSeq: 600,
    stoppedSession: "audition:run_e2e_unstick1:stopped", preparingSession: "audition:run_e2e_unstick1:preparing"
  });
  await installReplay(unstickContext, { extraEvents: unstickSeeded, strictConversation: true });
  const unstickPage = await unstickContext.newPage();
  await unstickPage.goto(agentBase + "/app/?conversation_id=" + encodeURIComponent(conversationId), { waitUntil: "domcontentloaded" });
  const unstickAppeared = await unstickPage
    .waitForSelector('[data-audition-session="audition:run_e2e_unstick1:stopped"]', { timeout: 15000 })
    .then(() => true)
    .catch(() => false);
  await unstickPage.waitForTimeout(1500);
  const unstickSample = await unstickPage.evaluate(DOM_PROBE);
  await unstickPage.screenshot({ path: join(outDir, "dom-audition-unstick.png") });
  writeFileSync(join(outDir, "dom-audition-unstick.json"), JSON.stringify(unstickSample, null, 2), "utf-8");
  await unstickContext.close();
  report.audition_unstick = { appeared: unstickAppeared, seeded_events: unstickSeeded };
  record("audition-unstick-G1", checkG1({ appeared: unstickAppeared, sample: unstickSample }, {
    stoppedSession: "audition:run_e2e_unstick1:stopped",
    preparingSession: "audition:run_e2e_unstick1:preparing"
  }));

  report.finished_at = new Date().toISOString();
  report.events_served_from_fixture = seededEventsRequests;
  const failed = Object.entries(report.passes).filter(([, ok]) => !ok).map(([id]) => id);
  report.verdict = failed.length === 0 ? "pass" : "fail";
  report.failed_groups = failed;
  writeFileSync(join(outDir, "webui_rendered_dom_report.json"), JSON.stringify(report, null, 2), "utf-8");

  await browser.close();

  log("verdict:", report.verdict, failed.length ? "(failed: " + failed.join(", ") + ")" : "");
  log("report:", join(outDir, "webui_rendered_dom_report.json"));
  process.exitCode = failed.length === 0 ? 0 : 1;
}

// Exported so a control run can exercise the very same probe and assertion
// functions against a deliberately healthy state (proof that a red result is a
// real finding and not an artefact of the probe itself).
export { DOM_PROBE, checkA1, checkA2, checkA3, checkB1, checkB2, checkC1, checkD1, checkE1, checkF1, checkG1, residencyFixtureEvents, chatOnlyItemStepsFixtureEvents, terminalTurnFixtureEvents, auditionFixtureEvents, auditionUnstickFixtureEvents };

// Run only when this file is the process entry point, so importing it as a
// library (the control run does) has no side effects.
const invokedDirectly = process.argv[1]
  ? new URL("file:///" + process.argv[1].split("\\").join("/")).href === import.meta.url
  : false;
if (invokedDirectly) {
  main().catch((err) => {
  log("FATAL:", String(err && err.stack ? err.stack : err));
  try {
    writeFileSync(join(outDir, "webui_rendered_dom_fatal.txt"), String(err && err.stack ? err.stack : err), "utf-8");
    } catch { /* best effort */ }
    process.exitCode = 2;
  });
}

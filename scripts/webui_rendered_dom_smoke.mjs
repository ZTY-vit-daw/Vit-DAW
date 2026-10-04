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
//   T1 audition   -- FIX-AUDITION-TRAIL-1: during processing the kernel audition
//                    telemetry must NOT stack duplicated 「Kernel audition」 rows
//                    in the flow-bottom lane (the judge card is the single
//                    dynamic surface, preparing explicitly); after the judgment
//                    the same single card settles with the verdict and the lane
//                    stays free of audition rows (定型不残留)
//   J1 judgment   -- AB-JUDGMENT-CARD-1 (M8 + M1 round-4: clicks left zero
//                    server trace): in the parked waiting state the verdict
//                    buttons render enabled; a real click on 「A 更好」 reaches
//                    POST /agent/audition/judgment with turn_id in the
//                    experiment domain (never the run-level anchor key the
//                    pre-fix reducer fed it), and a 409 rejection surfaces as
//                    the visible warning banner instead of an unhandled
//                    rejection (the silent swallow that cost two forensic
//                    rounds)
//   K1 confirm    -- FIX-CONFIRM-CARD-1 defect 2: a RiskConfirm confirmation
//                    card left unanswered when its turn truly completes must
//                    settle non-interactively in place (zero clickable
//                    affordances; no clickable "expired" card), and manual mode
//                    must never call the respond endpoint on its own
//   K2 fullaccess -- FIX-CONFIRM-CARD-1 defect 3 (2026-09-29 ruling): under
//                    authority_mode=full_project_access the same RiskConfirm gate
//                    renders NO pre-confirmation card anywhere, the respond
//                    endpoint receives exactly one approve (direct execution),
//                    and the direct-execution notice + execution receipt are
//                    visible in the conversation (事后回执)
//   O1 layering   -- OPT-OBSERVE-OUTPUT-1 P1 (2026-09-30): a long plain
//                    assistant reply (observation Q&A shape, over the layering
//                    thresholds) renders COLLAPSED by default (lead paragraph
//                    visible + a native <details> holding the rest) -- both on
//                    the composer-driven turn (context A) and on the Project
//                    History hydration path (context E); expanding reveals the
//                    full original text exactly once (lead + rest, no
//                    duplication); after a reload the hydrated default state
//                    recomputes collapsed (expansion is pure UI state, never
//                    persisted); the persisted content keeps the full original
//                    text. Short replies and the card families (interactive
//                    confirmation card, execution receipt) never fold, even
//                    when their text alone crosses the thresholds.
//   M1 msg-order -- WEBUI-MSG-ORDER-1 (2026-09-30 M8 hand-test, conversation
//                    webui_muo6fygb): one run spanning two user rounds
//                    (waiting_continue; BOTH user rows carry the run-domain
//                    turn_id, assistant rows carry chat-domain turn_*). The
//                    round-2 user input must render BELOW the round-1 outputs
//                    (u1 → trace block → a1 → a2 → u2 → a3), each row exactly
//                    once, and the run keeps ONE trace block anchored after the
//                    round-opening user message (B9 reuse semantics).
//   L1 init-layout-- WEBUI-INIT-LAYOUT-1 (2026-10-03 manual test, screenshot
//                    webui_initial_layout_squeezed.png): on the FIRST mounted
//                    frame the workspace must already fill the window --
//                    .app-shell grid track count == its DOM child count (the
//                    regression was a stale 3rd track starving the workspace's
//                    auto row), .main-workspace bottom == viewport bottom,
//                    sidebar stretching with it, and the composer seated at
//                    the window bottom instead of floating mid-window.
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
// MSG-REVIVE-1: the r3 forensic capture (conversation webui_mu2aized, normal-
// permission mix-tick chain) is its OWN transcript, seeded over the draft graph
// for the H1 pass only. The commit objects it references ship inside the
// fixture directory so the pass never depends on the user's history tree.
const msgConversationId = arg("msg-conversation-id", env("MSG_CONVERSATION_ID", "webui_mu2aized"));
const msgGraphFixturePath = arg("msg-graph-fixture", env("MSG_GRAPH_FIXTURE", join(here, "fixtures", "webui_rendered_dom", "msg_revive_graph.fixture.json")));
const msgEventsFixturePath = arg("msg-events-fixture", env("MSG_EVENTS_FIXTURE", join(here, "fixtures", "webui_rendered_dom", "msg_revive_events.fixture.json")));
const msgCommitDirs = arg("msg-commit-dirs", env("MSG_COMMIT_DIRS", join(here, "fixtures", "webui_rendered_dom", "msg_revive_commits")));
// WEBUI-MSG-ORDER-1: the M8 forensic capture (conversation webui_muo6fygb,
// waiting_continue run spanning two user rounds) is its own transcript, seeded
// over the draft graph for the M1 pass only. The commit objects it references
// are metadata-only reconstructions shipped inside the fixture directory (the
// capture did not include the session's commit files; the history reader only
// requires a readable commit whose project identity matches the live draft,
// which seedConversation rewrites onto every seeded commit).
const msgOrderConversationId = arg("msg-order-conversation-id", env("MSG_ORDER_CONVERSATION_ID", "webui_muo6fygb"));
const msgOrderGraphFixturePath = arg("msg-order-graph-fixture", env("MSG_ORDER_GRAPH_FIXTURE", join(here, "fixtures", "webui_rendered_dom", "msg_order_graph.fixture.json")));
const msgOrderEventsFixturePath = arg("msg-order-events-fixture", env("MSG_ORDER_EVENTS_FIXTURE", join(here, "fixtures", "webui_rendered_dom", "msg_order_events.fixture.json")));
const msgOrderCommitDirs = arg("msg-order-commit-dirs", env("MSG_ORDER_COMMIT_DIRS", join(here, "fixtures", "webui_rendered_dom", "msg_order_commits")));
// WEBUI-MSG-ORDER-2: the M1-RETEST forensic capture (conversation
// webui_mupe9yh1, park + user_stop then an immediately failed second turn) is
// the LIVE-shape transcript. No graph is seeded and the mount transcript is
// served empty: every visible row must arrive LIVE through the driven
// composer, which is the exact coverage gap WEBUI-MSG-ORDER-1 declared.
const msgOrder2ConversationId = arg("msg-order2-conversation-id", env("MSG_ORDER2_CONVERSATION_ID", "webui_mupe9yh1"));
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

function seedConversation(graphFixture, state, commitDirsOverride) {
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
    const archivedCommit = findArchivedCommit(node.commit_id, historyDir, commitDirsOverride);
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
function findArchivedCommit(commitId, historyDir, commitDirsOverride) {
  if (!commitId) return null;
  const searchDirs = Array.isArray(commitDirsOverride) && commitDirsOverride.length ? commitDirsOverride : commitDirs;
  const candidates = [
    join(historyDir, "commits", commitId + ".json"),
    ...searchDirs.map((dir) => join(dir, commitId + ".json"))
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
      text: (el.textContent || "").replace(/\s+/g, " ").trim().slice(0, 80),
      // WEBUI-MSG-ORDER-2: the card position in the real flow order (its
      // anchoring tier is what this card is about). Additive probe field.
      flowIndex: flowIndex(el)
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
    })),
    // MSG-REVIVE-1: the revived interaction card is a first-class rendered fact.
    // The composer projects the latest pending interaction as
    // .composer-interaction-shell wrapping an .action-card.interactive whose
    // .action-buttons row is exactly what the user clicks before seeing
    // 「交互已过期」. Sampled as its own element set so the bare-boot pass can
    // assert NO interactive card survives for interactions the event stream
    // already resolved (server-authoritative withdrawal).
    interactiveCards: Array.from(document.querySelectorAll(".action-card.interactive")).map((el) => ({
      text: (el.textContent || "").replace(/\s+/g, " ").trim().slice(0, 140),
      buttons: el.querySelectorAll(".action-buttons button").length,
      inComposerShell: Boolean(el.closest(".composer-interaction-shell"))
    })),
    // FIX-CONFIRM-CARD-1: the confirmation-card surface is sampled as its own
    // element set (StandardActionCard .action-card and CapabilityProposalCard
    // .capability-proposal-card alike) with clickable affordances counted, so
    // "terminal cards must not stay clickable" / "full access renders no
    // pre-confirmation card" are real DOM facts, not inferences.
    confirmCards: Array.from(document.querySelectorAll(".action-card, .capability-proposal-card")).map((el) => ({
      cls: typeof el.className === "string" ? el.className : "",
      text: (el.textContent || "").replace(/\s+/g, " ").trim().slice(0, 140),
      interactive: el.classList.contains("interactive"),
      buttons: el.querySelectorAll("button").length,
      actionButtons: el.querySelectorAll(".action-buttons button").length,
      approveButtons: el.querySelectorAll(".btn.approve, button.action-button.primary").length,
      outcome: (el.querySelector(".outcome") ? el.querySelector(".outcome").textContent || "" : "").replace(/\s+/g, " ").trim(),
      inComposerShell: Boolean(el.closest(".composer-interaction-shell"))
    })),
    composerInteractionShell: Boolean(document.querySelector(".composer-interaction-shell")),
    // The consumed-interaction ledger (F3 face-2 guard) is a plain localStorage
    // list; reading the raw value shows whether the event replay repopulated it
    // after the empty-storage boot.
    consumedLedger: window.localStorage.getItem("ask_vit_consumed_interactions") || ""
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

// FIX-AUDITION-TRAIL-1 (2026-09-29, M1 manual-test defect ①): during processing
// the kernel audition telemetry family (audition.prepare.started /
// audition.candidate.ready / audition.ready -- session snapshots WITHOUT any turn
// domain, because the kernel audition::Session has no turn field) used to land in
// the flow-bottom activity lane as one row per event type, all reading
// 「已完成：Kernel audition」-- the user saw several duplicated "kernel audition"
// trail entries under the output content. The lane now excludes the family by
// identity (turnGroups.isAuditionFamilyActivity); the A/B judge card is the
// single dynamic surface (one card per session, state-updating, settled after
// the judgment). The pass is two-phase in ONE browser context:
//   phase 1 (processing)  -- telemetry only, mount not complete: no lane row may
//                            carry the Kernel audition text; exactly one judge
//                            card in the preparing state;
//   phase 2 (settle)      -- agent-enriched audition.ready + judgment requested
//                            + recorded pushed into the SAME replay: still one
//                            card, now settled with the verdict outcome, and the
//                            lane still free of audition rows (定型不残留).
function auditionTrailFixtureEvents(options) {
  const now = Date.now();
  const runId = options.turnId;
  const sessionId = options.sessionId;
  const mixTickTurn = "mix_tick_turn:" + options.tickId;
  const base = Number(options.baseSeq) || 700;
  const at = (offsetMs) => new Date(now - 30_000 + offsetMs).toISOString();
  const candidates = (aStatus, bStatus, withPreview) => [
    { id: "candidate-a", label: "A", status: aStatus, ...(withPreview ? { preview_ref: "candidate-a" } : {}) },
    { id: "candidate-b", label: "B", status: bStatus, ...(withPreview ? { preview_ref: "candidate-b" } : {}) }
  ];
  // Kernel telemetry shape (AuditionPreviewService.publishStateEvent): NO turn
  // domain anywhere -- event turn fields empty, payload.session has no turn_id.
  // The logical_message_id is the agent stamp (audition_events.go emitAuditionEvent:
  // "audition:{sid}:{type}") -- one logical row per event type.
  const telemetry = (type, session, seq) => ({
    seq, type, conversation_id: conversationId, item_id: sessionId, item_type: "audition",
    status: String(session.status || "preparing"), title: "Kernel audition",
    logical_message_id: "audition:" + sessionId + ":" + type, created_at: at(seq * 100),
    payload: { schema_version: "vit.kernel_audition.v1", session }
  });
  const phase1 = [
    {
      seq: base + 1, type: "turn.started", conversation_id: conversationId,
      goal_id: runId, run_id: runId, turn_id: runId, source_turn_id: runId,
      item_type: "turn", status: "running", created_at: at(0)
    },
    {
      seq: base + 2, type: "trajectory.turn.started", conversation_id: conversationId,
      goal_id: runId, run_id: runId, turn_id: runId, source_turn_id: runId,
      item_id: "turn:" + runId, status: "running", created_at: at(0),
      payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "turn:" + runId, turn_id: runId, node_kind: "turn", phase: "framing", status: "running" }
    },
    telemetry("audition.prepare.started", { session_id: sessionId, conversation_id: conversationId, status: "preparing", candidates: candidates("preparing", "preparing", false) }, base + 3),
    telemetry("audition.candidate.ready", { session_id: sessionId, conversation_id: conversationId, status: "preparing", candidates: candidates("ready", "preparing", false) }, base + 4),
    telemetry("audition.candidate.ready", { session_id: sessionId, conversation_id: conversationId, status: "preparing", candidates: candidates("ready", "ready", false) }, base + 5)
  ];
  const phase2 = [
    // Agent-enriched mount completion (mix_tick_audition.go mountMixTickAudition):
    // the session snapshot carries the synthetic turn domain + ready candidates.
    {
      seq: base + 6, type: "audition.ready", conversation_id: conversationId,
      item_id: sessionId, item_type: "audition", status: "ready", title: "Kernel audition",
      logical_message_id: "audition:" + sessionId + ":audition.ready", created_at: at((base + 6) * 100),
      payload: {
        schema_version: "vit.kernel_audition.v1", command: "audition.prepare", mix_tick: options.tickId,
        session: { session_id: sessionId, conversation_id: conversationId, turn_id: mixTickTurn, round_id: "mix_tick_round:" + options.tickId, status: "ready", candidates: candidates("ready", "ready", true) }
      }
    },
    {
      seq: base + 7, type: "trajectory.user_judgment.requested", conversation_id: conversationId,
      goal_id: runId, run_id: runId, turn_id: mixTickTurn, source_turn_id: runId,
      item_id: "mix_tick_judgment:" + sessionId, status: "waiting_for_user", created_at: at((base + 7) * 100),
      payload: {
        schema_version: "vit.observable_trajectory.v1", trace_node_id: "mix_tick_judgment:" + sessionId,
        turn_id: mixTickTurn, round_id: "mix_tick_round:" + options.tickId, node_kind: "user_judgment",
        phase: "user_judgment", status: "waiting_for_user",
        details: { audition_session_id: sessionId, summary: "人声轨 -1dB · A/B 试听判定" }
      }
    },
    {
      seq: base + 8, type: "trajectory.user_judgment.recorded", conversation_id: conversationId,
      goal_id: runId, run_id: runId, turn_id: mixTickTurn, source_turn_id: runId,
      item_id: "judgment:" + sessionId, status: "completed", created_at: at((base + 8) * 100),
      payload: {
        schema_version: "vit.observable_trajectory.v1", trace_node_id: "judgment:" + sessionId,
        turn_id: mixTickTurn, node_kind: "user_judgment", phase: "user_judgment", status: "completed",
        details: { audition_session_id: sessionId, evidence: { preference: "b", heard_difference: "yes" } }
      }
    }
  ];
  return { phase1, phase2 };
}

function checkT1(result, options) {
  const failures = [];
  const notes = [];
  const laneTextsOf = (sample) => (sample.laneItems || []).map((item) => item.text);
  const cardsOf = (sample, sessionId) => (sample.auditionCards || []).filter((card) => card.session === sessionId);

  // Phase 1 -- processing window: the telemetry family is in the stream, the
  // mount has not completed. The flow-bottom lane must carry NO Kernel audition
  // row (pre-fix: prepare.started / candidate.ready rows all read
  // 「已完成：Kernel audition」 and stacked up), and the judge card must be the
  // single dynamic surface, explicitly preparing.
  const processing = result.processing;
  const processingLane = laneTextsOf(processing);
  const processingAuditionLane = processingLane.filter((text) => text.indexOf("Kernel audition") >= 0);
  notes.push("processing lane: [" + processingLane.join(" | ") + "] (audition rows: " + processingAuditionLane.length + ")");
  if (processingAuditionLane.length > 0) {
    failures.push(
      "T1 processing lane: " + processingAuditionLane.length + " Kernel audition row(s) rendered under the output content ([" +
      processingAuditionLane.join(" | ") + "]) -- the audition family must not lane; its surface is the judge card"
    );
  }
  const processingCards = cardsOf(processing, options.sessionId);
  if (processingCards.length !== 1) {
    failures.push(
      "T1 processing card: expected exactly 1 judge card for " + options.sessionId + ", got " +
      processingCards.length + " (cards: [" + (processing.auditionCards || []).map((card) => card.session).join(", ") + "])"
    );
  } else {
    notes.push("processing card: status=" + processingCards[0].status + " cls=\"" + processingCards[0].cls + "\"");
    if (processingCards[0].status !== "preparing") {
      failures.push("T1 processing card: data-status=" + processingCards[0].status + " instead of preparing");
    }
    const controls = (processing.auditionControls || []).find((card) => card.session === options.sessionId);
    const chips = controls ? controls.chips : [];
    notes.push("processing card chips: [" + chips.join(" | ") + "]");
    if (!chips.some((chip) => chip.indexOf("正在准备") >= 0)) {
      failures.push("T1 processing card: the preparing window is not stated (chips: [" + chips.join(" | ") + "]) -- the single surface must be explicit while warming up");
    }
  }

  // Phase 2 -- settled: same session, still exactly one card, now settled with
  // the verdict outcome; the lane stays free of audition rows (no residue).
  const settled = result.settled;
  const settledLane = laneTextsOf(settled);
  const settledAuditionLane = settledLane.filter((text) => text.indexOf("Kernel audition") >= 0);
  notes.push("settled lane: [" + settledLane.join(" | ") + "] (audition rows: " + settledAuditionLane.length + ")");
  if (settledAuditionLane.length > 0) {
    failures.push(
      "T1 settled lane: " + settledAuditionLane.length + " Kernel audition row(s) still rendered after the judgment ([" +
      settledAuditionLane.join(" | ") + "]) -- the trail must settle without residue"
    );
  }
  const settledCards = cardsOf(settled, options.sessionId);
  if (settledCards.length !== 1) {
    failures.push(
      "T1 settled card: expected exactly 1 judge card for " + options.sessionId + " after settle, got " +
      settledCards.length + " (cards: [" + (settled.auditionCards || []).map((card) => card.session).join(", ") + "])"
    );
  } else {
    notes.push("settled card: status=" + settledCards[0].status + " cls=\"" + settledCards[0].cls + "\" text=\"" + settledCards[0].text + "\"");
    if (!/(^|\s)settled(\s|$)/.test(settledCards[0].cls)) {
      failures.push("T1 settled card: the card does not carry the .settled class (cls=\"" + settledCards[0].cls + "\") -- the judgment must freeze the surface");
    }
    if ((settledCards[0].text || "").indexOf("已裁决") < 0) {
      failures.push("T1 settled card: no verdict outcome row on the card (text: \"" + settledCards[0].text + "\")");
    }
  }
  return { failures, notes };
}

// --------------------------------------------------- AB-JUDGMENT-CARD-1 J1
//
// Card 2026-09-30 AB-JUDGMENT-CARD-1 (M8 + M1 round-4 manual tests: judgment
// clicks left zero server-side trace). The two forensic rounds pinned the
// shapes this fixture reproduces exactly:
//   * the kernel's audition.ready session snapshot carries NO turn_id /
//     round_id / project_revision top-level keys (only active_project_plane);
//   * trajectory.user_judgment.requested carries the RUN-level id in
//     source_turn_id while payload.turn_id holds the experiment turn domain.
// The pre-fix reducer fed the run-level anchor key into the judgment POST's
// turn_id, so the real server answered 409 "audition session identity
// mismatch" and the webui swallowed it (no catch, no banner) -- a click with
// zero visible effect and zero event-stream trace.
//
//   J1 -- in the parked waiting state (kernel session stopped, judgment
//      requested, candidates ready) the verdict buttons render enabled; a real
//      click on 「A 更好」 MUST reach POST /agent/audition/judgment with
//      turn_id in the experiment domain (never the run-level anchor key), and
//      a 409 rejection MUST surface as the visible warning banner instead of
//      dying as an unhandled rejection. (The parked-loop settlement the POST
//      unlocks is covered agent-side by judgment_park_continuation_test.go;
//      this isolated agent has no free-state loop behind it.)
function judgmentIdentityFixtureEvents(options) {
  const now = Date.now();
  const startedAt = new Date(now - 60_000).toISOString();
  const midAt = new Date(now - 30_000).toISOString();
  const stoppedAt = new Date(now - 5_000).toISOString();
  const base = Number(options.baseSeq) || 800;
  const runCommon = { conversation_id: conversationId, goal_id: options.runId, run_id: options.runId, turn_id: options.runId, source_turn_id: options.runId };
  // Real kernel shape (M8-FORENSIC-20260930 seq 20/41): NO turn-domain keys.
  const kernelSession = (status, activeCandidateId) => ({
    session_id: options.sessionId, conversation_id: conversationId, status, state_revision: 5,
    active_candidate_id: activeCandidateId,
    active_project_plane: { plane: "active_project", project_ref: "project/Unsaved.vit", project_revision: "revision-j1" },
    candidates: [
      { id: "candidate-a", label: "A", status: "ready", preview_ref: "audio-buffer://" + options.sessionId + "/candidate-a:2ch@44100Hz" },
      { id: "candidate-b", label: "B", status: "ready", preview_ref: "audio-buffer://" + options.sessionId + "/candidate-b:2ch@44100Hz" }
    ]
  });
  return [
    { ...runCommon, seq: base + 1, type: "turn.started", item_type: "turn", status: "running", created_at: startedAt },
    {
      ...runCommon, seq: base + 2, type: "audition.ready", item_id: options.sessionId, status: "ready", created_at: midAt,
      payload: { schema_version: "vit.kernel_audition.v1", command: "audition.prepare", session: kernelSession("ready", "") }
    },
    // Real trajectory shape (M8 seq 28 / M1R4 seq 28): source_turn_id is the
    // run level; payload.turn_id is the experiment turn domain.
    {
      ...runCommon, seq: base + 3, type: "trajectory.user_judgment.requested", item_id: "judgment:" + options.sessionId, status: "waiting_for_user", created_at: midAt,
      payload: {
        schema_version: "vit.observable_trajectory.v1", trace_node_id: "judgment:" + options.sessionId,
        turn_id: options.experimentTurnId, round_id: options.roundId, node_kind: "user_judgment",
        phase: "user_judgment", status: "waiting_for_user", summary: "A/B audition required",
        details: { audition_session_id: options.sessionId, summary: "A/B audition required" }
      }
    },
    {
      ...runCommon, seq: base + 4, type: "audition.stopped", item_id: options.sessionId, status: "stopped", created_at: stoppedAt,
      payload: { schema_version: "vit.kernel_audition.v1", session: kernelSession("stopped", "candidate-a") }
    }
  ];
}

function checkJ1(result, options) {
  const failures = [];
  const notes = [];
  if (!result.appeared) {
    failures.push("J1 setup: the parked judge card never rendered its verdict buttons for " + options.sessionId + " -- the pass would measure nothing");
    return { failures, notes };
  }
  notes.push("verdict buttons rendered and enabled in the parked state (canJudge held with the kernel-shaped snapshot)");
  if (!result.posts || result.posts.length === 0) {
    failures.push("J1 click: the real click on 「A 更好」 never reached POST /agent/audition/judgment -- the binding is dead again");
    return { failures, notes };
  }
  notes.push("judgment POST issued " + result.posts.length + " time(s) on click");
  const bodies = result.posts.map((post) => (post && post.body ? post.body : post));
  bodies.forEach((post, index) => {
    if (post.turn_id !== options.experimentTurnId) {
      failures.push(
        "J1 contract: POST #" + (index + 1) + " carried turn_id=" + JSON.stringify(post.turn_id) +
        " instead of the experiment domain " + options.experimentTurnId +
        (post.turn_id === options.runId ? " (the run-level anchor key -- the AB-JUDGMENT-CARD-1 regression)" : "")
      );
    }
    if (post.audition_session_id !== options.sessionId) {
      failures.push("J1 contract: POST #" + (index + 1) + " carried audition_session_id=" + JSON.stringify(post.audition_session_id));
    }
    if (post.round_id !== options.roundId) {
      failures.push("J1 contract: POST #" + (index + 1) + " carried round_id=" + JSON.stringify(post.round_id) + " instead of " + options.roundId);
    }
  });
  const first = bodies[0] || {};
  notes.push("first POST: turn_id=" + JSON.stringify(first.turn_id) + " round_id=" + JSON.stringify(first.round_id) + " preference=" + JSON.stringify(first.preference));
  if (first.preference !== "a" || first.heard_difference !== "yes") {
    failures.push("J1 contract: the A seat must send preference=a with heard_difference=yes (got " + JSON.stringify(first.preference) + "/" + JSON.stringify(first.heard_difference) + ")");
  }
  if (!result.noticeVisible) {
    failures.push("J1 swallow: the 409 rejection did NOT surface as the visible warning banner -- handleAuditionJudgment must catch and setError, not die as an unhandled rejection");
  } else {
    notes.push("rejection surfaced: banner text=\"" + (result.noticeText || "") + "\"");
    if ((result.noticeText || "").indexOf("identity mismatch") < 0) {
      failures.push("J1 swallow: the banner must carry the server's rejection text (got \"" + result.noticeText + "\")");
    }
  }
  return { failures, notes };
}

// MSG-REVIVE-1 (2026-09-15): the user-hit form is the empty-storage bare boot
// (webview panel destroy/recreate lost ALL localStorage, not just the mapping).
// Everything recoverable must come from the server faces:
//   M1 -- the mix-tick chain's message bubbles (user ask + terminal report)
//         hydrate from the project conversation graph;
//   M2 -- interactions the event stream already resolved (interaction.resolved
//         seq18/21) must NOT render as interactive cards: the graph snapshot
//         still says waiting_for_user and the local consumed-interaction ledger
//         died with the storage, so the render decision must respect the
//         server-authoritative withdrawal (the click-deadlock the user hit was
//         the revived card answering 「交互已过期」);
//   M3 -- REFRESH-VANISH-2 zero-regression: the same bare boot still rebuilds
//         the trajectory blocks and the A/B cards from the replayed stream.
function checkH1(result, options) {
  const failures = [];
  const notes = [];
  const sample = result.sample;
  if (!result.seeded || result.seeded.commitsWritten === 0) {
    failures.push(
      "H1 setup: the r3-shaped graph did not seed into the draft history (nodes written=" +
      (result.seeded ? result.seeded.commitsWritten : "n/a") + ") -- the pass would measure nothing"
    );
  } else {
    notes.push("graph seeded: nodes=" + result.seeded.commitsWritten + " skipped=" + result.seeded.nodesSkipped.length);
  }
  if (!result.realScopeKey) {
    failures.push("H1 setup: the URL-bound learning context never materialized a real scope bucket -- the identity assertion cannot run");
  }
  const anchored = (sample.scopeBuckets || {})[result.realScopeKey || ""] || "";
  if (result.realScopeKey && anchored !== options.conversationId) {
    failures.push(
      "H1 identity: the real scope bucket holds \"" + anchored + "\" instead of the server conversation \"" +
      options.conversationId + "\" -- the continuation anchor (REFRESH-VANISH-2) is what makes every other assertion meaningful"
    );
  } else if (result.realScopeKey) {
    notes.push("identity: scope bucket -> " + options.conversationId + " (server anchor adopted on the bare boot)");
  }
  // M1: message bubbles from the project graph
  const userTexts = (sample.userMessages || []).map((message) => message.text);
  const assistantTexts = (sample.assistantMessages || []).map((message) => message.text);
  for (const expected of options.expectUserTexts) {
    if (!userTexts.some((text) => text.indexOf(expected) >= 0)) {
      failures.push(
        "H1 M1: the archived user message \"" + expected + "\" is NOT in the rendered flow (user rows: [" +
        userTexts.join(" | ") + "]) -- on an empty-storage bare boot the conversation messages must hydrate from the project history"
      );
    } else {
      notes.push("M1 user bubble rendered: \"" + expected + "\"");
    }
  }
  for (const expected of options.expectAssistantTexts) {
    if (!assistantTexts.some((text) => text.indexOf(expected) >= 0)) {
      failures.push(
        "H1 M1: the mix-tick chain terminal report (\"" + expected + "\"...) is NOT in the rendered flow (assistant rows: [" +
        assistantTexts.join(" | ") + "]) -- the chain's output message must survive the restart through the project history"
      );
    } else {
      notes.push("M1 assistant bubble rendered: \"" + expected + "\"");
    }
  }
  // M2: no interactive card for server-resolved interactions
  const interactive = sample.interactiveCards || [];
  if (interactive.length > 0) {
    failures.push(
      "H1 M2: " + interactive.length + " interactive action card(s) rendered on the bare boot (" +
      JSON.stringify(interactive) +
      ") -- interactions already resolved in the event stream (interaction.resolved) must not revive: the graph snapshot says waiting_for_user and the local ledger died with the storage, so the renderer must respect the server-authoritative withdrawal"
    );
  } else {
    notes.push("M2 zero interactive cards on the bare boot after the replay settled");
  }
  const ledger = sample.consumedLedger || "";
  for (const interactionID of options.expectResolvedInteractionIDs) {
    if (ledger.indexOf(interactionID) < 0) {
      failures.push(
        "H1 M2 ledger: the replay did not record interaction " + interactionID +
        " as consumed (ledger=\"" + ledger.slice(0, 120) + "\") -- the interaction.resolved events must feed the durable consumed ledger even on an empty-storage boot"
      );
    }
  }
  if (ledger) {
    notes.push("consumed ledger after replay: " + ledger.slice(0, 160));
  }
  // M3: REFRESH-VANISH-2 zero-regression
  if (!result.appeared || (sample.blocks || []).length === 0) {
    failures.push("H1 M3: no .trace-block rendered on the bare boot -- the REFRESH-VANISH-2 replay rebuild regressed");
  } else {
    notes.push("M3 trace blocks rebuilt: [" + (sample.blocks || []).map((block) => block.turnId).join(", ") + "]");
  }
  if ((sample.auditionCards || []).length === 0) {
    failures.push("H1 M3: no A/B audition card rendered on the bare boot -- the replayed audition events must still rebuild the cards");
  } else {
    notes.push("M3 audition cards rebuilt: " + (sample.auditionCards || []).length + " card(s)");
  }
  return { failures, notes };
}

// --------------------------------------------------- FIX-CONFIRM-CARD-1 K1/K2
//
// Card 2026-09-29 FIX-CONFIRM-CARD-1 (M1 manual-test defects 2+3). Both passes
// drive the REAL composer against network-layer fixture responses (the isolated
// agent has no kernel and no LLM behind it -- same posture as the G1 render-only
// precedent; the agent's Go-side respond routing keeps its own handler tests).
//
//   K1 (manual mode, defect 2) -- a RiskConfirm mix confirmation card renders
//      interactive; the turn then truly completes (turn.completed status
//      completed) while the card was never answered. The card must settle in
//      place with zero clickable affordances (no clickable "expired" card left
//      for a click to reveal the 4022 wording), and manual mode must never call
//      the respond endpoint on its own.
//   K2 (full access, defect 3 + the 2026-09-29 ruling) -- /agent/ui/state
//      carries authority_mode=full_project_access; the same chat response must
//      produce NO pre-confirmation card anywhere (stream or composer shell),
//      the respond endpoint must receive exactly one approve (direct execution),
//      the direct-execution notice line must be visible, and the execution
//      receipt must land in the conversation.

function confirmInteractionFixture(options = {}) {
  const interactionID = options.interactionID || "interaction_e2e_confirm";
  const kind = options.kind || "mix_tick_confirmation";
  return {
    id: interactionID,
    interaction_id: interactionID,
    kind: kind,
    type: kind,
    source: "vit_agent",
    workflow: options.workflow || "mix_tick",
    stage: "pending_confirmation",
    title: options.title || "混音单步待确认",
    body: options.body || "把 Track 2 提升 1dB",
    status: "waiting_for_user",
    plan_id: options.planID || "",
    actions: [
      { id: "approve", label: "确认执行", style: "primary", recommended: true },
      { id: "cancel", label: "取消", style: "secondary" }
    ]
  };
}

function confirmationChatResponseFixture(options = {}) {
  const turnID = options.turnID || "run_e2e_confirm";
  return {
    status: "ok",
    conversation_id: conversationId,
    reply: options.reply !== undefined ? options.reply : "这个操作需要你确认后才会执行。",
    needs_confirmation: true,
    goal_status: options.goalStatus || "waiting_confirmation",
    plan_id: options.planID || "plan_e2e_confirm",
    turn_id: turnID,
    run_id: turnID,
    goal_id: turnID,
    interaction_requests: options.interactions !== undefined ? options.interactions : [confirmInteractionFixture(options)],
    commands: []
  };
}

function directExecutionReceiptFixture(options = {}) {
  const turnID = options.turnID || "run_e2e_confirm";
  return {
    status: "ok",
    conversation_id: conversationId,
    reply: options.reply || "已完成：Track 2 提升 +1.0 dB（可从版本检查点回滚）。",
    needs_confirmation: false,
    goal_status: "completed",
    turn_id: turnID,
    run_id: turnID,
    goal_id: turnID,
    interaction_id: options.interactionID || "",
    executed_kernel_reply: [{ status: "ok", label: "track.volume" }],
    commands: []
  };
}

// WEBUI-MSG-ORDER-1: the M8 inversion counterexample, asserted on the real
// rendered flow. The forensic transcript has one run (run_e5796736a4865570)
// spanning two user rounds; the defect rendered the round-2 input ABOVE the
// round-1 outputs (group re-merge by the shared run turn_id). The fix keeps
// every row in flow order, one row per logical message, and keeps ONE trace
// block for the run anchored after the round-opening user message (B9 reuse).
function checkM1(result) {
  const failures = [];
  const notes = [];
  const sample = result.sample;
  if (!sample) {
    return { failures: ["M1 setup: no DOM sample was captured"], notes };
  }
  if (!result.appeared) {
    failures.push("M1 setup: no .trace-block rendered for the replayed run — the events replay never produced the round container");
  }
  const blocks = (sample.blocks || []).filter((block) => block.turnId === result.expectedRunId);
  if (blocks.length !== 1) {
    failures.push(
      "M1 block: expected exactly one trace block for turn " + result.expectedRunId + " (B9 reuse: one run = one block), got " +
        blocks.length + " [" + (sample.blocks || []).map((block) => block.turnId).join(", ") + "]"
    );
  }
  const findRows = (rows, marker) => (rows || []).filter((row) => row.text.indexOf(marker) >= 0);
  const expected = [
    { role: "user", marker: "检查一下当前工程有什么问题" },
    { role: "assistant", marker: "我还在继续处理这个任务" },
    { role: "assistant", marker: "这一步已经应用好了" },
    { role: "user", marker: "你能再检查一下Bass轨道" },
    { role: "assistant", marker: "任务在形成有效结算前失败" }
  ];
  const located = expected.map((item) => {
    const rows = findRows(item.role === "user" ? sample.userMessages : sample.assistantMessages, item.marker);
    return { ...item, rows };
  });
  for (const item of located) {
    // The seq31 scheduler_chain terminal synthesizes a chain-result twin of the
    // round-1 reply (GUI-F8 renders it after the whole entry sequence). On the
    // replayed stream that twin coexists with the hydrated row — known harness
    // shape, noted, not a merge defect; every OTHER row must be unique.
    if (item.marker === "这一步已经应用好了") {
      if (item.rows.length < 1) {
        failures.push("M1 rows: expected at least one assistant row containing \"" + item.marker + "\", got 0");
      } else if (item.rows.length > 1) {
        notes.push("M1 rows: the chain-result twin of the round-1 reply renders alongside the hydrated row (GUI-F8 replay shape), rows=" + item.rows.length);
      }
      continue;
    }
    if (item.rows.length !== 1) {
      failures.push(
        "M1 rows: expected exactly one " + item.role + " row containing \"" + item.marker + "\" (merge must not duplicate a logical message), got " +
          item.rows.length + " at indexes [" + item.rows.map((row) => row.index).join(", ") + "]"
      );
    }
  }
  // The inversion itself: round-2 input BELOW both round-1 outputs, block after
  // the round-1 user message, failure receipt after the round-2 input. Pre-fix
  // order was u1 → block → u2 → a1 → a2 → a3 (group re-merge by the run id).
  // The round-1 reply is anchored at its FIRST rendered row (the hydrated,
  // in-group one) — the chain-result twin renders at the flow tail by design.
  const index = (item) => (item.rows.length >= 1 ? item.rows[0].index : NaN);
  const [u1, a1, a2, u2, a3] = located;
  const blockIndex = blocks.length === 1 ? blocks[0].flowIndex : NaN;
  const orderPairs = [
    ["trace block", blockIndex, "round-1 user message", index(u1)],
    ["round-1 receipt a1", index(a1), "trace block", blockIndex],
    ["round-1 reply a2", index(a2), "round-1 receipt a1", index(a1)],
    ["round-2 user input", index(u2), "round-1 reply a2", index(a2)],
    ["failure receipt a3", index(a3), "round-2 user input", index(u2)]
  ];
  for (const [later, laterIndex, earlier, earlierIndex] of orderPairs) {
    if (!(laterIndex > earlierIndex)) {
      failures.push(
        "M1 order: " + later + " (index " + laterIndex + ") must render after " + earlier + " (index " + earlierIndex +
          ") — rendered flow: [" + (sample.flow || []).map((row) => row.cls.replace("message-row", "row").trim() || row.tag).join(" | ") + "]"
      );
    }
  }
  notes.push("flow: " + JSON.stringify((sample.flow || []).map((row) => ({ i: row.index, cls: row.cls, text: row.text.slice(0, 24) }))));
  return { failures, notes };
}

// WEBUI-MSG-ORDER-2: the M1-RETEST live event shape (36-event forensic stream,
// reduced to the load-bearing families, ids/timestamps semantics preserved).
// Phase 1 replays the park: the B9 run turn (run_4dbe...) with its item steps
// and the turn.completed(limit_reached) receipt boundary, the free-state
// experiment family folded under the same source_turn_id (payload.turn_id =
// turn:free_state_*), mix_tick.pending, and the audition family with NO turn
// domain anywhere (the kernel audition::Session has no such field -- that is
// the pinning defect's identity gap). Phase 2 replays the second turn failing
// immediately (audio_closure_controller_failure). Timestamps are stamped at
// append time so every event is strictly newer than the client-stamped
// optimistic rows that preceded it, exactly like the live session.
function msgOrder2Phase1Events(options) {
  const conversationId = options.conversationId;
  const run1 = "run_4dbe6a109d4bd4e7";
  const goal1 = "goal_36b2f2e85a4efad9";
  const freeState = "turn:free_state_7318f4503f4fe7e3";
  const sessionId = "audition:turn:free_state_7318f4503f4fe7e3:round-1-3e668023e682bcdc";
  const base = Date.now();
  const at = (offsetMs) => new Date(base + offsetMs).toISOString();
  let seq = options.baseSeq;
  const ev = (type, fields, payload) => ({
    seq: seq++, type, conversation_id: conversationId, goal_id: goal1, run_id: run1,
    source_turn_id: fields.sourceTurnID, created_at: at(options.offsetMs + seq * 100),
    logical_message_id: fields.logicalMessageID,
    ...(fields.itemID ? { item_id: fields.itemID } : {}),
    ...(fields.status ? { status: fields.status } : {}),
    ...(fields.title ? { title: fields.title } : {}),
    ...(fields.body ? { body: fields.body } : {}),
    payload: payload || {}
  });
  const turnEvent = (type, status, payload) => ev(type, { sourceTurnID: run1, logicalMessageID: "agent_turn:" + run1, status }, payload);
  const itemEvent = (type, itemID, status, payload) => ev(type, { sourceTurnID: run1, itemID, logicalMessageID: "agent_item:" + run1 + ":" + itemID, status }, payload);
  const trajectory = (type, itemID, turnID, extra) => ev(
    "trajectory." + type,
    { sourceTurnID: run1, itemID, logicalMessageID: "trajectory:" + run1 + ":" + itemID },
    { schema_version: "vit.observable_trajectory.v1", trace_node_id: itemID, turn_id: turnID, ...(extra || {}) }
  );
  const audition = (type, status) => ev(
    "audition." + type,
    { itemID: sessionId, logicalMessageID: "audition:" + sessionId + ":audition." + type, status },
    { schema_version: "vit.kernel_audition.v1", session: {
      session_id: sessionId, conversation_id: conversationId, status,
      state_revision: 5,
      candidates: [
        { id: "candidate-a", label: "A", status: status === "preparing" ? "preparing" : "ready", preview_ref: "audio-buffer://" + sessionId + "/candidate-a:2ch@44100Hz" },
        { id: "candidate-b", label: "B", status: status === "preparing" ? "preparing" : "ready", preview_ref: "audio-buffer://" + sessionId + "/candidate-b:2ch@44100Hz" }
      ]
    } }
  );
  return [
    turnEvent("turn.started", "running"),
    trajectory("turn.started", "turn:" + run1, run1, { node_kind: "turn", phase: "framing", status: "running" }),
    itemEvent("item.started", "tool_step_1", "running", { command_name: "ccb_observation_catalog" }),
    itemEvent("item.completed", "tool_step_1", "completed", { command_name: "ccb_observation_catalog" }),
    turnEvent("turn.completed", "waiting_continue", { stop_reason: "limit_reached", completed_steps: 1 }),
    itemEvent("item.started", "tool_step_2", "running", { command_name: "ccb_observation_request" }),
    itemEvent("item.completed", "tool_step_2", "completed", { command_name: "ccb_observation_request" }),
    trajectory("turn.started", "turn:" + freeState, freeState, { node_kind: "turn", phase: "experiment", status: "running" }),
    trajectory("intent.framed", "intent:" + freeState, freeState, { node_kind: "intent", status: "completed", round_id: "round-1-3e668023e682bcdc" }),
    trajectory("hypothesis.proposed", "hyp:" + freeState, freeState, { node_kind: "hypothesis", status: "completed", round_id: "round-1-3e668023e682bcdc" }),
    trajectory("round.started", "round:" + freeState, freeState, { node_kind: "round", status: "running", round_id: "round-1-3e668023e682bcdc" }),
    trajectory("observation.recorded", "obs:" + freeState, freeState, { node_kind: "observation", status: "completed", round_id: "round-1-3e668023e682bcdc", summary: "bass 轨低中频聚集" }),
    ev("mix_tick.pending", { sourceTurnID: run1, logicalMessageID: "agent_event:" + conversationId + ":13", status: "pending_confirmation", title: "混音单步（完全访问直接应用）", body: "对 Track 1007 执行一次有界的静态 EQ 频段增益调整，当前为完全访问模式：这一步将直接应用，不再等待逐条确认" }, { operation: "static_eq_band_adjust", track_id: "1007" }),
    trajectory("intervention.applied", "apply:" + freeState, freeState, { node_kind: "action", status: "completed", round_id: "round-1-3e668023e682bcdc", summary: "static_eq · Track 1007 频段增益" }),
    audition("prepare.started", "preparing"),
    audition("candidate.ready", "preparing"),
    audition("ready", "ready"),
    audition("ready", "ready"),
    trajectory("turn.stopped", "turn:" + freeState, freeState, { node_kind: "turn", phase: "stopped", status: "stopped" })
  ];
}

function msgOrder2Phase2Events(options) {
  const conversationId = options.conversationId;
  const run2 = "run_3eaf57c9b717c4f2";
  const goal2 = "goal_6b07c1866f795403";
  const base = Date.now();
  const at = (offsetMs) => new Date(base + offsetMs).toISOString();
  let seq = options.baseSeq;
  const ev = (type, fields, payload) => ({
    seq: seq++, type, conversation_id: conversationId, goal_id: goal2, run_id: run2,
    source_turn_id: fields.sourceTurnID, created_at: at(seq * 100),
    logical_message_id: fields.logicalMessageID,
    ...(fields.status ? { status: fields.status } : {}),
    payload: payload || {}
  });
  const turnEvent = (type, status, payload) => ev(type, { sourceTurnID: run2, logicalMessageID: "agent_turn:" + run2, status }, payload);
  const trajectory = (type, itemID, turnID, extra) => ev(
    "trajectory." + type,
    { sourceTurnID: run2, itemID, logicalMessageID: "trajectory:" + run2 + ":" + itemID },
    { schema_version: "vit.observable_trajectory.v1", trace_node_id: itemID, turn_id: turnID, ...(extra || {}) }
  );
  return [
    turnEvent("turn.started", "running"),
    trajectory("turn.started", "turn:" + run2, run2, { node_kind: "turn", phase: "framing", status: "running" }),
    turnEvent("turn.failed", "failed", { stop_reason: "audio_closure_controller_failure", error: "minimal audio closure controller failed: conversation is already owned by minimal_audio_closure controller audio_closure_997396f72e46902c" }),
    trajectory("turn.failed", "turn:" + run2, run2, { node_kind: "turn", phase: "failed", status: "failed" })
  ];
}

// WEBUI-MSG-ORDER-2: the live-order gate. Post-fix flow (u1 hydrated with the
// run-domain turn_id; the judge card resolves its turn through the session id's
// native encoding folded into the B9 run turn):
//   u1 -> trace(run_4dbe...) -> [A/B card] -> a1 receipt -> u2 -> trace(run_3eaf...) -> a2 error
// Pre-fix the card carries NO turn domain, lands in unanchoredSessions, and is
// pinned at the flow tail BELOW the round-2 input and the failure receipt --
// the exact M1-RETEST visual.
function checkM2(result) {
  const failures = [];
  const notes = [];
  const sample = result.finalSample;
  if (!sample) {
    return { failures: ["M2 setup: no final DOM sample was captured"], notes };
  }
  const run1 = "run_4dbe6a109d4bd4e7";
  const run2 = "run_3eaf57c9b717c4f2";
  const sessionID = "audition:turn:free_state_7318f4503f4fe7e3:round-1-3e668023e682bcdc";
  if (!result.phase1.auditionCardAppeared) {
    failures.push("M2 setup: the A/B judge card never rendered during the park phase -- the pass would measure nothing");
  }
  if (!result.phase1.traceAppeared) {
    failures.push("M2 setup: no .trace-block rendered for " + run1 + " during the park phase");
  }
  const findRows = (rows, marker) => (rows || []).filter((row) => row.text.indexOf(marker) >= 0);
  const expected = [
    { role: "user", marker: "检查一下当前工程有什么问题吗" },
    { role: "assistant", marker: "我还在继续处理这个任务" },
    { role: "user", marker: "检查一下当前选中的drums轨道的低频" }
  ];
  const located = expected.map((item) => ({ ...item, rows: findRows(item.role === "user" ? sample.userMessages : sample.assistantMessages, item.marker) }));
  for (const item of located) {
    if (item.rows.length !== 1) {
      failures.push(
        "M2 rows: expected exactly one " + item.role + " row containing \"" + item.marker + "\", got " + item.rows.length +
        " at indexes [" + item.rows.map((row) => row.index).join(", ") + "]"
      );
    }
  }
  const errorRow = findRows(sample.flow, "声学闭环控制器无法建立一致的持久状态").find((row) => (row.cls || "").indexOf("message-row") >= 0);
  if (!errorRow) {
    failures.push("M2 rows: the turn.failed error row is not in the rendered flow");
  }
  const blocks = sample.blocks || [];
  const block1 = blocks.filter((block) => block.turnId === run1);
  if (block1.length !== 1) {
    failures.push("M2 blocks: expected exactly one trace block for " + run1 + " (B9 fold of the run shell + free-state family), got " + block1.length + " [" + blocks.map((block) => block.turnId).join(", ") + "]");
  }
  const cards = (sample.auditionCards || []).filter((card) => card.session === sessionID);
  if (cards.length !== 1) {
    failures.push("M2 cards: expected exactly one A/B judge card for " + sessionID + ", got " + cards.length);
  }
  const index = (item) => (item.rows.length >= 1 ? item.rows[0].index : NaN);
  const [u1, a1, u2] = located;
  const cardIndex = cards.length >= 1 ? cards[0].flowIndex : NaN;
  const block1Index = block1.length === 1 ? block1[0].flowIndex : NaN;
  const block2 = blocks.filter((block) => block.turnId === run2);
  const block2Index = block2.length >= 1 ? block2[0].flowIndex : NaN;
  const errorIndex = errorRow ? errorRow.index : NaN;
  const orderPairs = [
    ["trace block " + run1, block1Index, "round-1 user message", index(u1)],
    ["round-1 receipt a1", index(a1), "trace block " + run1, block1Index],
    ["A/B judge card", cardIndex, "trace block " + run1, block1Index],
    ["round-2 user input", index(u2), "A/B judge card", cardIndex],
    ["failure receipt a2", errorIndex, "round-2 user input", index(u2)]
  ];
  for (const [later, laterIndex, earlier, earlierIndex] of orderPairs) {
    if (!(laterIndex > earlierIndex)) {
      failures.push(
        "M2 order: " + later + " (index " + laterIndex + ") must render after " + earlier + " (index " + earlierIndex +
        ") -- rendered flow: [" + (sample.flow || []).map((row) => (row.cls || "").replace("message-row", "row").trim() || row.tag).join(" | ") + "]"
      );
    }
  }
  if (block2.length >= 1 && !(block2Index > index(u2))) {
    failures.push("M2 order: the round-2 trace block (index " + block2Index + ") must render after the round-2 user input (index " + index(u2) + ")");
  }
  if (cards.length >= 1) {
    // Designed supersedes semantics: the user continued the conversation past
    // the pending judgment card, so the card settles. The settle bar doubles
    // as the proof that the card is the REAL interactive surface (not a stray
    // shell) and that the order fix did not fake its state.
    if (cards[0].text.indexOf("卡面选项未采用") < 0) {
      notes.push("M2 card settle bar not found yet (card text: \"" + cards[0].text.slice(0, 60) + "\")");
    } else {
      notes.push("A/B card settled with the supersedes bar at its chronological slot");
    }
  }
  notes.push("flow: " + JSON.stringify((sample.flow || []).map((row) => ({ i: row.index, cls: (row.cls || "").replace("message-row", "row").trim(), text: row.text.slice(0, 24) }))));
  return { failures, notes };
}

function checkK1(result) {
  const failures = [];
  const notes = [];
  if (!result.appeared) {
    failures.push("K1 setup: the waiting confirmation card never rendered in its interactive form -- the pass would measure nothing");
    return { failures, notes };
  }
  const before = (result.before.confirmCards || []).find((card) => card.text.indexOf("Track 2") >= 0) || (result.before.confirmCards || [])[0];
  if (!before) {
    failures.push("K1 premise: no confirmation card sampled before the terminal event");
  } else {
    notes.push("before: cls=\"" + before.cls + "\" buttons=" + before.buttons + " actionButtons=" + before.actionButtons);
    if (!before.interactive || before.actionButtons < 2) {
      failures.push(
        "K1 premise: the card did not render as a clickable pre-confirmation (interactive=" + before.interactive +
        ", actionButtons=" + before.actionButtons + ") -- the defect shape must be reproduced before the fix is measured"
      );
    }
  }
  const after = result.after;
  const clickable = (after.confirmCards || []).filter((card) => card.actionButtons > 0 || card.approveButtons > 0);
  const interactiveCount = (after.interactiveCards || []).length;
  if (interactiveCount > 0 || clickable.length > 0) {
    failures.push(
      "K1 终态残留: after turn.completed the confirmation surface still renders clickable cards (interactive=" +
      interactiveCount + ", withButtons=" + clickable.length + "): " + JSON.stringify(clickable) +
      " -- the card must settle non-interactively at task end, not stay clickable until the 4022 wording reveals it"
    );
  } else {
    notes.push("after: no clickable confirmation card remains (confirm card total " + (after.confirmCards || []).length + ")");
  }
  const settledCard = (after.confirmCards || []).find((card) => card.text.indexOf("Track 2") >= 0);
  if (settledCard) {
    if (settledCard.interactive || settledCard.buttons > 0) {
      failures.push("K1: the settled card still renders buttons (cls=\"" + settledCard.cls + "\", buttons=" + settledCard.buttons + ")");
    } else {
      notes.push("settled in place: cls=\"" + settledCard.cls + "\" buttons=0");
    }
  } else {
    notes.push("the waiting card left the flow entirely (nothing stale remains)");
  }
  if (result.respondHits !== 0) {
    failures.push("K1 manual mode: the respond endpoint was called " + result.respondHits + " time(s) on the app's own -- manual_confirmation must never auto-respond");
  } else {
    notes.push("respond endpoint never called by the app (manual mode unchanged)");
  }
  return { failures, notes };
}

function checkK2(result, options) {
  const failures = [];
  const notes = [];
  if (!result.authorityReady) {
    failures.push("K2 setup: the authority pill never showed 完全访问 -- the pass would test a manual-mode send, not a full-access one");
  } else {
    notes.push("authority pill showed 完全访问 before the send (the grant was active at send time)");
  }
  if (result.respondHits !== 1) {
    failures.push(
      "K2 直执: the respond endpoint was hit " + result.respondHits + " time(s) (expected exactly 1 approve) -- full access must direct-execute the RiskConfirm gate, not leave it waiting"
    );
  } else {
    notes.push("respond endpoint hit exactly once with the approve decision (direct execution)");
  }
  const sample = result.sample;
  const clickable = (sample.confirmCards || []).filter((card) => card.actionButtons > 0 || card.approveButtons > 0);
  if ((sample.interactiveCards || []).length > 0 || clickable.length > 0 || sample.composerInteractionShell) {
    failures.push(
      "K2 前置卡: full access still renders a pre-confirmation surface (interactive=" + (sample.interactiveCards || []).length +
      ", withButtons=" + clickable.length + ", composerShell=" + sample.composerInteractionShell + "): " + JSON.stringify(clickable)
    );
  } else {
    notes.push("no pre-confirmation card rendered anywhere (stream + composer shell clean)");
  }
  const noticeVisible = (sample.assistantMessages || []).some((message) => message.text.indexOf("完全访问已开启") >= 0);
  if (!noticeVisible) {
    failures.push("K2 通告: the direct-execution notice line (完全访问已开启 · …回执见下) is not in the rendered flow");
  } else {
    notes.push("direct-execution notice rendered in the flow");
  }
  const receiptVisible = (sample.assistantMessages || []).some((message) => message.text.indexOf(options.receiptMarker) >= 0);
  if (!receiptVisible) {
    failures.push(
      "K2 回执: the execution receipt (marker \"" + options.receiptMarker + "\") is not visible in the conversation -- the post-hoc receipt is part of the ruling, not optional"
    );
  } else {
    notes.push("execution receipt visible in the conversation");
  }
  if (sample.consumedLedger.indexOf(options.interactionID) < 0) {
    failures.push("K2 ledger: the direct-executed interaction id was not recorded in the consumed ledger");
  } else {
    notes.push("direct-executed interaction recorded in the consumed ledger");
  }
  return { failures, notes };
}


// --------------------------------------------------- OPT-OBSERVE-OUTPUT-1 O1
//
// Card 2026-09-29 OPT-OBSERVE-OUTPUT-1, P1 spec (design §4.2/§7). The composer
// drives the REAL app against network-layer chat fixtures (same posture as K1:
// the isolated agent has no kernel/LLM behind it). Layering thresholds live in
// the webui's observeOutputLayering.ts; this group only asserts the rendered
// DOM contract:
//   O1a long reply  -- renders with .observe-output-details NOT [open]; the
//                      lead paragraph (OBSLEAD marker) is visible while the
//                      collapsed rest (OBSRESTFINAL marker) is not.
//   O1b expand      -- clicking the summary opens the details; the full text
//                      (lead AND the rest tail) is visible exactly once.
//   O1c reload      -- after a page reload the details is collapsed again:
//                      expansion is pure UI state, the default state recomputes
//                      from the persisted message (hydration equivalence).
//   O1d families    -- a short reply, a long confirmation-card message and a
//                      long execution receipt never render the layered
//                      container (错折叠=缺陷 boundary).
//
// NOTE: the collapsed rest text IS in the DOM (the UA just does not render
// boxes for closed <details> children), so visibility is judged through
// innerText, which respects rendering.

function observeLongReplyFixture() {
  const lead = "OBSLEAD 观察结论：Track 2 主唱在 2.1kHz 附近有约 3.2dB 的峰值堆积，与和声吉他的泛音列重叠，是听感发硬的主要来源。";
  const restLines = [];
  for (let index = 1; index <= 16; index += 1) {
    restLines.push(
      "证据条目 OBSREST" + index + "：第 " + index + " 频段的静态电平关系与遮蔽余量、来源投影与指标名（观察问答折叠区的证据细节正文，用于验证折叠容器完整原文）。"
    );
  }
  restLines.push("OBSRESTFINAL 证据尾行：折叠区最后一行，展开后必须可见。");
  return {
    status: "ok",
    conversation_id: conversationId,
    reply: lead + "\n\n" + restLines.join("\n"),
    needs_confirmation: false,
    goal_status: "completed",
    turn_id: "run_e2e_observe_long",
    run_id: "run_e2e_observe_long",
    goal_id: "run_e2e_observe_long",
    commands: []
  };
}

function observeShortReplyFixture() {
  return {
    status: "ok",
    conversation_id: conversationId,
    reply: "OBSHORT 短回复：低于折叠阈值，保持现行单段渲染。",
    needs_confirmation: false,
    goal_status: "completed",
    turn_id: "run_e2e_observe_short",
    run_id: "run_e2e_observe_short",
    goal_id: "run_e2e_observe_short",
    commands: []
  };
}

// FIX-BUCKET-SAVE-RACE-1: the driven reply that exists ONLY in the localStorage
// bucket (no graph node). Long enough to ride the layering predicate; the marker
// tail must come back after the reload (context F).
function observeLocalTailReplyFixture() {
  const lead = "OBSLOCAL 本地桶末轮结论：这条回复没有图节点，只落 (conversation, scope) 本地桶。";
  const restLines = [];
  for (let index = 1; index <= 6; index += 1) {
    restLines.push("本地证据条目 OBSLOCALT" + index + "：第 " + index + " 条仅存本地桶的证据细节。");
  }
  restLines.push("OBSLOCALFINAL 本地桶末轮尾行：刷新后必须仍在。");
  return {
    status: "ok",
    conversation_id: conversationId,
    reply: lead + "\n\n" + restLines.join("\n"),
    needs_confirmation: false,
    goal_status: "completed",
    turn_id: "run_e2e_bucketrace_local",
    run_id: "run_e2e_bucketrace_local",
    goal_id: "run_e2e_bucketrace_local",
    commands: []
  };
}

function observeLongProposalFixture(turnID, interactionID) {
  const fixture = confirmationChatResponseFixture({ turnID, interactionID });
  fixture.reply =
    "OBSCARD 确认卡消息：这段正文超过折叠阈值（" + "长".repeat(640) + "），但它携带待确认交互卡，属于交互卡族，绝不能折叠。";
  return fixture;
}

function observeLongReceiptFixture(turnID) {
  return {
    status: "ok",
    conversation_id: conversationId,
    reply:
      "OBSRECEIPT 执行回执消息：这段回执正文超过折叠阈值（" + "回".repeat(640) + "），但它是回执族，绝不能折叠。",
    needs_confirmation: false,
    goal_status: "completed",
    turn_id: turnID,
    run_id: turnID,
    goal_id: turnID,
    interaction_id: "",
    executed_kernel_reply: [{ status: "ok", label: "track.volume" }],
    commands: []
  };
}

// Row-scoped DOM probe: finds the assistant message row carrying the marker and
// reports its layering shape. innerText (not textContent) is the visibility
// oracle: closed <details> children have no boxes and drop out of innerText.
const observeRowProbe = (marker) => {
  const rows = Array.from(document.querySelectorAll(".message-row.assistant"));
  const row = rows.find((el) => (el.textContent || "").indexOf(marker) >= 0) || null;
  if (!row) {
    return null;
  }
  const details = row.querySelector(".observe-output-details");
  const lead = row.querySelector(".observe-output-lead");
  const summary = details ? details.querySelector("summary") : null;
  return {
    marker,
    layered: Boolean(details),
    open: details ? details.hasAttribute("open") : null,
    summaryText: summary ? (summary.textContent || "").replace(/\s+/g, " ").trim() : "",
    leadText: lead ? (lead.textContent || "").replace(/\s+/g, " ").trim() : "",
    rowVisibleText: (row.innerText || "").replace(/\s+/g, " ").trim(),
    actionCards: row.querySelectorAll(".action-card").length,
    paragraphCount: row.querySelectorAll(".message-body p").length
  };
};

function checkO1(result) {
  const failures = [];
  const notes = [];
  const long = result.long;
  if (!long || !long.defaultState) {
    failures.push("O1a setup: the long observation reply never rendered -- the pass would measure nothing");
    return { failures, notes };
  }
  const collapsed = long.defaultState;
  if (!collapsed.layered) {
    failures.push("O1a 默认态: the long reply did not render the layered container (.observe-output-details missing)");
  } else {
    if (collapsed.open) {
      failures.push("O1a 默认态: the long reply renders expanded by default -- the details must be collapsed on first render");
    } else {
      notes.push("O1a long reply collapsed by default (summary=\"" + collapsed.summaryText + "\")");
    }
    if (collapsed.rowVisibleText.indexOf("OBSLEAD") < 0) {
      failures.push("O1a 首段: the lead paragraph (OBSLEAD) is not visible on the collapsed message -- 首段可见 is the default state's point");
    } else {
      notes.push("O1a lead paragraph visible while collapsed");
    }
    if (collapsed.rowVisibleText.indexOf("OBSRESTFINAL") >= 0) {
      failures.push("O1a 折叠: the collapsed rest (OBSRESTFINAL) is visible while the details is closed -- 折叠默认态失效");
    } else {
      notes.push("O1a collapsed rest not visible (details closed)");
    }
  }
  const expanded = long.expandedState;
  if (!expanded) {
    failures.push("O1b setup: the expanded-state sample never landed");
  } else if (expanded.layered && expanded.open) {
    if (expanded.rowVisibleText.indexOf("OBSRESTFINAL") < 0) {
      failures.push("O1b 展开: after clicking the summary the rest tail (OBSRESTFINAL) is still not visible -- the details did not reveal its content");
    } else {
      const hits = expanded.rowVisibleText.split("OBSRESTFINAL").length - 1;
      if (hits !== 1) {
        failures.push("O1b 展开形态: the rest tail renders " + hits + " time(s) when expanded -- expanded rendering must match the current one exactly once (no duplicated lead/rest)");
      } else {
        notes.push("O1b expanded rendering shows the full original text exactly once");
      }
    }
    if (expanded.rowVisibleText.indexOf("OBSLEAD") < 0) {
      failures.push("O1b 展开: the lead paragraph disappeared after expanding -- expanded view must carry the full original text");
    }
  } else {
    failures.push("O1b 展开: clicking the summary did not open the details (layered=" + expanded.layered + ", open=" + expanded.open + ")");
  }
  const hydrated = long.hydratedState;
  const rehydrated = long.rehydratedState;
  if (!hydrated) {
    failures.push("O1c setup: the hydrated long reply never rendered (Project History hydration leg)");
  } else {
    if (!hydrated.layered) {
      failures.push("O1c 水合: the hydrated long reply did not render the layered container -- the default state must recompute from the persisted content");
    } else if (hydrated.open) {
      failures.push("O1c 水合: the hydrated long reply renders expanded -- expansion is pure UI state and must never be remembered across hydration");
    } else if (hydrated.rowVisibleText.indexOf("OBSRESTFINAL") >= 0) {
      failures.push("O1c 水合: the collapsed rest (OBSRESTFINAL) is visible on the hydrated message -- 折叠默认态失效");
    } else {
      notes.push("O1c hydrated reply renders collapsed by default (default state recomputed from content)");
    }
  }
  if (!rehydrated) {
    failures.push("O1c setup: the re-hydrated sample never landed after reload (20s across two uiState refresh cycles)");
  } else if (!rehydrated.layered || rehydrated.open) {
    failures.push(
      "O1c 刷新: after reload the reply is layered=" + rehydrated.layered + " open=" + rehydrated.open +
      " -- the default state must recompute collapsed (v1 remembers no expansion state)"
    );
  } else {
    notes.push("O1c after reload the default state recomputed collapsed again (expansion not remembered)");
  }
  const persisted = long.persistedBuckets || {};
  const persistedRows = Object.values(persisted).flat();
  if (persistedRows.length > 0) {
    const carriedFullText = persistedRows.some((row) => row.hasRestMarker && row.contentLength === long.persistedFullContentLength);
    if (!carriedFullText) {
      failures.push("O1c 持久化: the persisted message content lost the full original text (length " +
        JSON.stringify(persistedRows.map((row) => row.contentLength)) + ", expected " + long.persistedFullContentLength +
        ") -- layering must not rewrite content");
    } else {
      notes.push("O1c persisted content carries the full original text (length unchanged, no expansion state)");
    }
  }
  const notFolded = (sample, label, marker, cardPremise) => {
    if (!sample) {
      failures.push("O1d setup: the " + label + " message never rendered");
      return;
    }
    if (sample.layered) {
      failures.push("O1d " + label + ": the layered container rendered on a " + label + " message -- 错折叠缺陷（回执/交互卡族一概不动）");
    } else {
      notes.push("O1d " + label + " not layered (plain current rendering)");
    }
    if (sample.rowVisibleText.indexOf(marker) < 0) {
      failures.push("O1d " + label + ": the message text is not visible in the flow");
    }
    if (cardPremise !== null && !cardPremise) {
      failures.push("O1d " + label + ": premise broken -- the card family surface did not render, the pass would measure nothing");
    }
  };
  notFolded(result.short, "short reply", "OBSHORT", null);
  notFolded(result.proposal, "confirmation card", "OBSCARD", result.cardPremise);
  notFolded(result.receipt, "execution receipt", "OBSRECEIPT", null);
  return { failures, notes };
}

// ------------------------------------------------ WEBUI-IA-REDESIGN-1 (2026-10-02)
// 三区布局一期的新增断言组：左列=会话流侧边栏（新建/切换/重命名/归档/折叠，
// 会话键=conversation id，不建工作树不建分支），DAW 简易视窗三件套（时间线/
// 机架 lanes/MIDI workspace，含同族 mixer-strips 与死码 V1 机架）整体下线，
// 执行状态栏不再显示哈希类任务 id 文本（STATUSBAR-ID-1 并入）。
function checkIaSidebar(result, options) {
  const failures = [];
  const notes = [];
  if (!result.appeared) {
    failures.push("the session-flow sidebar never rendered (.session-sidebar)");
    return { failures, notes };
  }
  if (!result.currentRowActive) {
    failures.push("the mounted conversation (" + options.conversationId + ") is not the active sidebar row");
  }
  if (!result.numbersLinear || result.numbersLinear.count === 0 || !result.numbersLinear.allLinear) {
    failures.push("sidebar display numbers are not pure linear digits: " + JSON.stringify(result.numbersLinear));
  } else {
    notes.push("display numbers: " + JSON.stringify(result.numbersLinear.values) + " (no prefix, no hash)");
  }
  if (!result.registryKeys.some((key) => key.indexOf("ask_vit_session_flow.v1") === 0)) {
    failures.push("no session-flow registry bucket persisted to localStorage after mount");
  }
  const nc = result.newConversation;
  if (!nc.rowAppeared) {
    failures.push("clicking 新建会话 did not register a new row in the sidebar list");
  }
  if (nc.rowAppeared && nc.activeID === options.conversationId) {
    failures.push("clicking 新建会话 did not switch the active row to the new conversation");
  }
  if (nc.rowAppeared && nc.newID && nc.newID.indexOf("webui_") !== 0) {
    failures.push("new conversation id is not the lightweight webui_ form: " + nc.newID);
  } else if (nc.newID) {
    notes.push("new conversation " + nc.newID + " registered (no worktree, no branch)");
  }
  if (!result.rename.inputAppeared || !result.rename.titleUpdated) {
    failures.push("inline rename did not commit (inputAppeared=" + result.rename.inputAppeared + ", titleUpdated=" + result.rename.titleUpdated + ")");
  }
  if (!result.switchBack.activeRestored) {
    failures.push("clicking the original conversation row did not switch back (active row is not " + options.conversationId + ")");
  }
  if (!result.archive.hiddenFromList || !result.archive.toggleAppeared || !result.archive.restored) {
    failures.push(
      "archive/restore flow broken (hidden=" + result.archive.hiddenFromList +
      ", toggle=" + result.archive.toggleAppeared + ", restored=" + result.archive.restored + ")"
    );
  }
  if (!result.collapse.collapsedClass || !result.collapse.expandedBack) {
    failures.push("collapse toggle broken (collapsed=" + result.collapse.collapsedClass + ", expandedBack=" + result.collapse.expandedBack + ")");
  }
  return { failures, notes };
}

function checkIaViewport(result) {
  const failures = [];
  const notes = [];
  for (const [selector, count] of Object.entries(result.deadNodes || {})) {
    if (count > 0) {
      failures.push("deleted viewport family still renders: " + selector + " x" + count);
    }
  }
  if (Object.keys(result.deadNodes || {}).length === 0) {
    failures.push("the dead-node probe returned nothing (pass setup broken)");
  } else {
    notes.push("all deleted families absent: " + Object.keys(result.deadNodes).join(", "));
  }
  return { failures, notes };
}

function checkIaStatusbar(result, options) {
  const failures = [];
  const notes = [];
  if (!result.planBarAppeared) {
    failures.push("the plan bar never rendered with the injected running task -- the statusbar id assertion would measure nothing");
    return { failures, notes };
  }
  if (result.planBarHeadText && /Task\s+\S+/.test(result.planBarHeadText)) {
    failures.push("the plan-bar head still shows a visible task id text: " + JSON.stringify(result.planBarHeadText));
  }
  if (result.planBarTaskElementCount > 0) {
    failures.push("a .plan-bar-task element still renders (" + result.planBarTaskElementCount + ")");
  }
  if (options.expectTaskID && result.planBarDataTaskID !== options.expectTaskID) {
    failures.push("data-task-id attribute missing or wrong: " + JSON.stringify(result.planBarDataTaskID));
  } else {
    notes.push("task id stays DOM-only via data-task-id=" + JSON.stringify(result.planBarDataTaskID));
  }
  return { failures, notes };
}

// WEBUI-INIT-LAYOUT-1: the first mounted frame's layout facts. Everything the
// user's eyes land on at boot is a real viewport-coordinate fact here: the
// shell's resolved grid tracks, the workspace/sidebar/composer boxes. Sampled
// immediately after the shell mounts -- the frame the squeeze lived in -- with
// zero interaction and no scroll.
const INITIAL_LAYOUT_PROBE = () => {
  const rectOf = (el) => {
    if (!el) return null;
    const r = el.getBoundingClientRect();
    return {
      top: +r.top.toFixed(1),
      left: +r.left.toFixed(1),
      bottom: +r.bottom.toFixed(1),
      right: +r.right.toFixed(1),
      w: +r.width.toFixed(1),
      h: +r.height.toFixed(1)
    };
  };
  const shell = document.querySelector(".app-shell");
  const shellCS = shell ? getComputedStyle(shell) : null;
  return {
    innerW: window.innerWidth,
    innerH: window.innerHeight,
    shellRows: shellCS ? shellCS.gridTemplateRows : "",
    shellDisplay: shellCS ? shellCS.display : "",
    shellChildCount: shell ? shell.children.length : 0,
    topStatus: rectOf(document.querySelector(".app-shell > .top-status")),
    workspace: rectOf(document.querySelector(".app-shell > .main-workspace")),
    sidebar: rectOf(document.querySelector(".session-sidebar")),
    composer: rectOf(document.querySelector(".composer")),
    workbenchPanel: rectOf(document.querySelector(".workbench-panel")),
    workbenchCollapsed: rectOf(document.querySelector(".workbench-collapsed")),
    workspaceGridCls: document.querySelector(".workspace-grid") ? document.querySelector(".workspace-grid").className : ""
  };
};

function checkInitialLayout(sample) {
  const failures = [];
  const notes = [];
  if (!sample.workspace || !sample.composer) {
    failures.push("L1 setup: .main-workspace or .composer did not render -- nothing to measure");
    return { failures, notes };
  }
  const tracks = sample.shellRows.split(/\s+/).filter(Boolean);
  // The regression shape itself: the shell declared 3 tracks for 2 children, so
  // the empty minmax(0,1fr) track swallowed the window and the workspace slid
  // into a content-sized auto row. Track count must always equal child count.
  if (sample.shellDisplay !== "grid") {
    failures.push("L1: .app-shell is display=" + sample.shellDisplay + ", expected grid");
  }
  if (tracks.length !== sample.shellChildCount) {
    failures.push(
      "L1 tracks: .app-shell resolves " + tracks.length + " grid row track(s) [" + sample.shellRows +
      "] for " + sample.shellChildCount + " child element(s) -- a stale track starves the workspace row " +
      "(the WEBUI-INIT-LAYOUT-1 squeeze shape: rows must be 1:1 with children)"
    );
  } else {
    notes.push("shell tracks [" + sample.shellRows + "] = " + sample.shellChildCount + " children (1:1)");
  }
  // The fill itself: the workspace's last pixel row sits on the viewport's
  // last pixel row on the very first frame -- no dead band under the content.
  const fillSlack = sample.innerH - sample.workspace.bottom;
  if (Math.abs(fillSlack) > 1.5) {
    failures.push(
      "L1 fill: .main-workspace bottom=" + sample.workspace.bottom + " vs viewport height=" + sample.innerH +
      " (slack " + fillSlack.toFixed(1) + "px) -- the initial frame leaves a dead band under the app " +
      "(the user-visible squeeze: sidebar/composer/rail all end early)"
    );
  } else {
    notes.push("workspace fills the viewport: bottom=" + sample.workspace.bottom + " == innerHeight=" + sample.innerH);
  }
  if (sample.sidebar && Math.abs(sample.sidebar.bottom - sample.workspace.bottom) > 1.5) {
    failures.push(
      "L1 sidebar: .session-sidebar bottom=" + sample.sidebar.bottom + " vs workspace bottom=" + sample.workspace.bottom +
      " -- the left column no longer stretches with the workspace row"
    );
  } else if (sample.sidebar) {
    notes.push("sidebar stretches with the workspace row (bottom=" + sample.sidebar.bottom + ")");
  }
  // The composer is anchored near the window bottom (its own bottom offset is
  // ~22px); floating mid-window means the workspace row collapsed again.
  const composerSlack = sample.innerH - sample.composer.bottom;
  if (composerSlack < -1.5 || composerSlack > 60) {
    failures.push(
      "L1 composer: .composer bottom=" + sample.composer.bottom + " vs viewport height=" + sample.innerH +
      " (slack " + composerSlack.toFixed(1) + "px, expected within [0, 60]) -- the composer floats mid-window " +
      "instead of seating at the window bottom"
    );
  } else {
    notes.push("composer seated at the window bottom (bottom=" + sample.composer.bottom + ", slack " + composerSlack.toFixed(1) + "px)");
  }
  // Right rail consistency with the viewport threshold (App.tsx collapses the
  // workbench panel under 1120px): whichever shape renders, it must reach the
  // window's right edge -- a squeezed shell also pinches the rail inward.
  if (sample.innerW >= 1120) {
    if (!sample.workbenchPanel) {
      failures.push("L1 right panel: innerWidth " + sample.innerW + " >= 1120 but no .workbench-panel rendered (workspaceGridCls=\"" + sample.workspaceGridCls + "\")");
    } else if (Math.abs(sample.workbenchPanel.right - sample.innerW) > 1.5) {
      failures.push(
        "L1 right panel: .workbench-panel right=" + sample.workbenchPanel.right + " vs viewport width=" + sample.innerW +
        " -- the panel does not reach the window edge"
      );
    } else {
      notes.push("right workbench panel open and flush with the window edge (right=" + sample.workbenchPanel.right + ")");
    }
  } else if (!sample.workbenchCollapsed) {
    failures.push("L1 right rail: innerWidth " + sample.innerW + " < 1120 but no .workbench-collapsed rail rendered");
  } else {
    notes.push("narrow viewport keeps the collapsed rail (right=" + sample.workbenchCollapsed.right + ")");
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
  // MSG-REVIVE-1: the r3 capture body used by the H1 pass (its own replay
  // fixture and its own conversation id -- see installReplay options).
  const msgGraphFixture = JSON.parse(readFileSync(msgGraphFixturePath, "utf-8"));
  const msgEventsFixture = JSON.parse(readFileSync(msgEventsFixturePath, "utf-8"));
  const msgOrderGraphFixture = JSON.parse(readFileSync(msgOrderGraphFixturePath, "utf-8"));
  const msgOrderEventsFixture = JSON.parse(readFileSync(msgOrderEventsFixturePath, "utf-8"));
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
  // MSG-REVIVE-1: baseFixture/conversation default to the archived mtzba6wf
  // stream; the H1 pass passes the r3 capture body and its own conversation id.
  const installReplay = async (context, options = {}) => {
    // TRAJ-IMPL-1: a pass may append seeded boundary events (the residency shape)
    // to the archived stream. The archived stream itself is never edited: the
    // extra events are carried here and listed in the report.
    const baseFixture = options.baseFixture || eventsFixture;
    const replayConversationID = options.conversationId || conversationId;
    const extraEvents = Array.isArray(options.extraEvents) ? options.extraEvents : [];
    const all = [...(baseFixture.events || []), ...extraEvents];
    // The app advances its since-cursor from next_seq, so a merged stream must
    // report the merged maximum. For the archived-only stream this is exactly
    // the value the fixture already carried.
    const nextSeq = all.reduce(
      (maximum, event) => Math.max(maximum, Number(event.seq) || 0),
      Number(baseFixture.next_seq) || 0
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
      const mismatch = strictConversation && askedConversation !== replayConversationID;
      const body = JSON.stringify({
        status: baseFixture.status || "ok",
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
        // WEBUI-SESSION-SEMANTICS-1: the projection carries the owning
        // conversation id (the real agent has always served it, chat/server.go
        // taskRuntimeTrajectoryProjection); the webui binds the plan bar to the
        // active session, so the fixture must declare its owner for the bar to
        // render at all -- which makes every plan-bar assertion below prove
        // per-conversation binding, not just presence.
        conversation_id: conversationId,
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

  // MSG-REVIVE-1: the server-side identity anchor. The user's hit had the real
  // agent still running with the conversation's continuation rows durable in
  // runtime status; the isolated agent has none, so the completed row is
  // appended to the REAL runtime/status response (everything else stays real).
  const installContinuationAnchors = async (context, rows) => {
    await context.route("**/agent/runtime/status*", async (route) => {
      const response = await route.fetch();
      const body = await response.json().catch(() => ({}));
      const continuations = [...(Array.isArray(body.continuations) ? body.continuations : []), ...rows];
      await route.fulfill({ response, json: { ...body, continuations } });
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

  // MSG-REVIVE-1: the empty-storage bare boot over the r3-shaped transcript.
  // Phase 1 re-seeds the draft history with the r3 conversation graph (the
  // normal-permission mix-tick chain, interaction-carrying proposal node) and
  // learns this run's real scope bucket key from a URL-bound context. Phase 2
  // is the user's exact form: a fresh context with NO localStorage at all, bare
  // /app/, and only the server faces (continuation anchor + graph + replayed
  // events) to recover from. The events endpoint is conversation-strict, so a
  // random regenerated id gets an empty buffer exactly like the live agent.
  const runMsgRevivePass = async (name, options) => {
    const state = await getJSON("/agent/ui/state");
    const seeded = seedConversation(msgGraphFixture, state, [msgCommitDirs]);
    const anchorRows = [
      {
        conversation_id: msgConversationId,
        status: "completed",
        project_path: (state.project_history || {}).project_path || "",
        project_uuid: (state.project_history || {}).project_uuid || "",
        updated_at: "2026-09-15T06:30:40Z",
        goal_id: "goal_cb4ac06ddb410e24",
        run_id: "run_2577decba4d612be"
      }
    ];
    const learnContext = await browser.newContext({ viewport });
    await installReplay(learnContext, { baseFixture: msgEventsFixture, conversationId: msgConversationId, strictConversation: true });
    await installContinuationAnchors(learnContext, anchorRows);
    const learnPage = await learnContext.newPage();
    await learnPage.goto(agentBase + "/app/?conversation_id=" + encodeURIComponent(msgConversationId), { waitUntil: "domcontentloaded" });
    await learnPage.waitForTimeout(2500);
    const learned = await learnPage.evaluate(() =>
      Object.fromEntries(
        Object.keys(localStorage)
          .filter((key) => key.indexOf("ask_vit_conversation_id_scope") === 0)
          .map((key) => [key, localStorage.getItem(key) || ""])
      )
    );
    await learnContext.close();
    const realScopeKey = Object.keys(learned).find((key) => key !== "ask_vit_conversation_id_scope:unsaved_root") || "";

    const context = await browser.newContext({ viewport });
    await installReplay(context, { baseFixture: msgEventsFixture, conversationId: msgConversationId, strictConversation: true });
    await installContinuationAnchors(context, anchorRows);
    // The live shape the user actually hit: the panel boots BEFORE the agent's
    // workspace identity lands in ui/state (the decision-side probe observed the
    // unsaved bucket and the real bucket coexisting after the reopen), so the
    // first tick sees an unmaterialized scope ("unsaved::root"), mints a random
    // conversation id, and only the NEXT tick walks the evolution ->
    // server-anchor adoption path (REFRESH-VANISH-2). Serving the first
    // ui/state response with the workspace identity stripped reproduces that
    // sequence deterministically; from the second request on, the real response
    // passes through untouched.
    let uiStateRequests = 0;
    const uiStateHandler = async (route) => {
      uiStateRequests += 1;
      const response = await route.fetch();
      if (uiStateRequests <= (options.blindUiStateTicks || 0)) {
        const body = await response.json().catch(() => ({}));
        await route.fulfill({
          response,
          json: { ...body, project_history: {}, project: {}, ui_context: {} }
        });
        return;
      }
      await context.unroute("**/agent/ui/state*", uiStateHandler);
      await route.fulfill({ response });
    };
    await context.route("**/agent/ui/state*", uiStateHandler);
    const page = await context.newPage();
    await page.goto(agentBase + "/app/", { waitUntil: "domcontentloaded" });
    const appeared = await page
      .waitForSelector(".trace-block", { timeout: options.appearTimeoutMs })
      .then(() => true)
      .catch(() => false);
    await page.waitForTimeout(options.settleMs);
    const sample = await page.evaluate(DOM_PROBE);
    await page.screenshot({ path: join(outDir, "dom-" + name + ".png"), fullPage: false });
    writeFileSync(join(outDir, "dom-" + name + ".json"), JSON.stringify(sample, null, 2), "utf-8");
    // A late sample after one more uiState poll cycle (~8s cadence): reported
    // as a note, not asserted -- it documents whether the periodic history-sync
    // merge eventually heals the stream on its own, which is exactly how long
    // the user-visible defect window is.
    await page.waitForTimeout(options.lateSettleMs || 0);
    const late = options.lateSettleMs ? await page.evaluate(DOM_PROBE) : null;
    if (late) {
      writeFileSync(join(outDir, "dom-" + name + "-late.json"), JSON.stringify(late, null, 2), "utf-8");
    }
    await context.close();
    return { appeared, sample, late, seeded, realScopeKey, learned, anchorRows };
  };

  // WEBUI-MSG-ORDER-1: M1 pass — seed the M8 forensic graph over the draft,
  // replay the archived 47-event stream conversation-strict, open the panel
  // bound to the forensic conversation id, and sample the rendered flow.
  // MUST run after every group that asserts against an earlier seeding (it
  // re-seeds the draft graph, like runMsgRevivePass does).
  // WEBUI-MSG-ORDER-2: the composer-driven LIVE pass. The round-2 rows never
  // exist server-side: they are produced by the REAL composer (optimistic rows
  // stamped with the client clock), which is the exact coverage gap
  // WEBUI-MSG-ORDER-1 declared. The draft history is served EMPTY for this
  // conversation's mount (ui/state projection patch -- the same network-layer
  // pattern as PLANBAR-1), so the stream is the pure live shape of the M1-RETEST
  // session: optimistic u1, response receipt, then the park events. The event
  // buffer is a mutable array served with the agent's own since/limit contract
  // (K1 pattern), so phase-2 events are appended mid-pass.
  const runMsgOrder2Pass = async (name) => {
    const context = await browser.newContext({ viewport });
    await context.route("**/agent/ui/state*", async (route) => {
      const response = await route.fetch();
      const body = await response.json().catch(() => ({}));
      const history = { ...(body.project_history || {}) };
      history.conversation_messages = [];
      await route.fulfill({ response, json: { ...body, project_history: history } });
    });
    const served = [];
    await context.route("**/agent/events*", async (route) => {
      const requestURL = new URL(route.request().url());
      const asked = requestURL.searchParams.get("conversation_id") || "";
      const since = Number(requestURL.searchParams.get("since") || "0");
      const limit = Number(requestURL.searchParams.get("limit") || "120");
      const nextSeq = served.reduce((maximum, event) => Math.max(maximum, Number(event.seq) || 0), 0);
      await route.fulfill({
        status: 200, contentType: "application/json; charset=utf-8",
        body: JSON.stringify({
          status: "ok",
          events: asked === msgOrder2ConversationId ? served.filter((event) => Number(event.seq) > since).slice(0, limit) : [],
          next_seq: nextSeq
        })
      });
    });
    const receiptResponse = {
      status: "ok", conversation_id: msgOrder2ConversationId,
      reply: "我还在继续处理这个任务，完成后再向你汇报。",
      needs_confirmation: false, goal_status: "waiting_interaction",
      turn_id: "turn_d8701f0905894cc8", run_id: "run_4dbe6a109d4bd4e7", goal_id: "goal_36b2f2e85a4efad9",
      executed_kernel_reply: [{ status: "ok", label: "ccb_observation_catalog" }],
      commands: []
    };
    const failureResponse = {
      status: "ok", conversation_id: msgOrder2ConversationId,
      reply: "声学闭环控制器无法建立一致的持久状态，因此没有继续观察或修改工程。",
      needs_confirmation: false, goal_status: "failed",
      turn_id: "run_3eaf57c9b717c4f2", run_id: "run_3eaf57c9b717c4f2", goal_id: "goal_6b07c1866f795403",
      commands: []
    };
    await context.route("**/agent/chat*", async (route) => {
      let asked = "";
      try { asked = String((route.request().postDataJSON() || {}).message || ""); } catch { asked = ""; }
      await route.fulfill({
        status: 200, contentType: "application/json; charset=utf-8",
        body: JSON.stringify(asked.indexOf("drums") >= 0 ? failureResponse : receiptResponse)
      });
    });
    const page = await context.newPage();
    await page.goto(agentBase + "/app/?conversation_id=" + encodeURIComponent(msgOrder2ConversationId), { waitUntil: "domcontentloaded" });
    await page.waitForTimeout(2500);
    // Phase 1: drive the park turn through the REAL composer, then replay the
    // park events (B9 fold + audition family with NO turn domain anywhere).
    const composer = page.locator(".composer textarea").first();
    await composer.waitFor({ state: "visible", timeout: 15000 });
    await composer.fill("检查一下当前工程有什么问题吗");
    await composer.press("Enter");
    await page.waitForTimeout(1200);
    served.push(...msgOrder2Phase1Events({ conversationId: msgOrder2ConversationId, baseSeq: 1, offsetMs: 0 }));
    const auditionCardAppeared = await page
      .waitForSelector("[data-audition-session]", { timeout: 25000 })
      .then(() => true)
      .catch(() => false);
    const traceAppeared = await page
      .waitForSelector('.trace-block[data-turn-id="run_4dbe6a109d4bd4e7"]', { timeout: 25000 })
      .then(() => true)
      .catch(() => false);
    await page.waitForTimeout(1500);
    const parkSample = await page.evaluate(DOM_PROBE);
    await page.screenshot({ path: join(outDir, "dom-" + name + "-park.png") });
    writeFileSync(join(outDir, "dom-" + name + "-park.json"), JSON.stringify(parkSample, null, 2), "utf-8");
    // Phase 2: the second turn, driven live, failing immediately.
    await composer.fill("检查一下当前选中的drums轨道的低频");
    await composer.press("Enter");
    await page.waitForTimeout(1200);
    served.push(...msgOrder2Phase2Events({ conversationId: msgOrder2ConversationId, baseSeq: 500 }));
    await page.waitForTimeout(3000);
    const finalSample = await page.evaluate(DOM_PROBE);
    await page.screenshot({ path: join(outDir, "dom-" + name + ".png") });
    writeFileSync(join(outDir, "dom-" + name + ".json"), JSON.stringify(finalSample, null, 2), "utf-8");
    await context.close();
    return { phase1: { auditionCardAppeared, traceAppeared, parkSample }, finalSample };
  };

  const runMsgOrderPass = async (name, options) => {
    const state = await getJSON("/agent/ui/state");
    const seeded = seedConversation(msgOrderGraphFixture, state, [msgOrderCommitDirs]);
    const context = await browser.newContext({ viewport });
    await installReplay(context, { baseFixture: msgOrderEventsFixture, conversationId: msgOrderConversationId, strictConversation: true });
    const page = await context.newPage();
    await page.goto(agentBase + "/app/?conversation_id=" + encodeURIComponent(msgOrderConversationId), { waitUntil: "domcontentloaded" });
    const appeared = await page
      .waitForSelector(".trace-block", { timeout: options.appearTimeoutMs || 25000 })
      .then(() => true)
      .catch(() => false);
    await page.waitForTimeout(options.settleMs || 4000);
    const sample = await page.evaluate(DOM_PROBE);
    await page.screenshot({ path: join(outDir, "dom-" + name + ".png"), fullPage: false });
    writeFileSync(join(outDir, "dom-" + name + ".json"), JSON.stringify(sample, null, 2), "utf-8");
    await context.close();
    return { appeared, sample, seeded };
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

  // ------------------------------------------------------- FIX-AUDITION-TRAIL-1
  // Two phases in ONE browser context. The replay route serves a mutable event
  // array: phase 1 = archived stream + turn shell + kernel telemetry (no turn
  // domain, mount incomplete); after the processing sample the phase-2 events
  // (agent-enriched ready + judgment requested/recorded) are appended and the
  // app's own polling picks them up -- the card must settle in place.
  report.audition_trail_events_source =
    "archived stream + seeded M1-shaped events (kernel telemetry audition.* WITHOUT turn domain first, " +
    "agent-enriched audition.ready + user_judgment requested/recorded appended mid-pass) replayed for " +
    "GET /agent/events in one browser context";
  const trailSessionId = "audition:mix_tick:tick_e2e_trail1";
  const trailFixture = auditionTrailFixtureEvents({
    turnId: "run_e2e_auditiontrail1", tickId: "tick_e2e_trail1", sessionId: trailSessionId, baseSeq: 700
  });
  const trailContext = await browser.newContext({ viewport });
  const trailServed = [...(eventsFixture.events || []), ...trailFixture.phase1];
  await trailContext.route("**/agent/events*", async (route) => {
    const requestURL = new URL(route.request().url());
    const since = Number(requestURL.searchParams.get("since") || "0");
    const limit = Number(requestURL.searchParams.get("limit") || "120");
    const nextSeq = trailServed.reduce((maximum, event) => Math.max(maximum, Number(event.seq) || 0), 0);
    await route.fulfill({
      status: 200, contentType: "application/json; charset=utf-8",
      body: JSON.stringify({ status: "ok", events: trailServed.filter((event) => Number(event.seq) > since).slice(0, limit), next_seq: nextSeq })
    });
  });
  const trailPage = await trailContext.newPage();
  await trailPage.goto(agentBase + "/app/?conversation_id=" + encodeURIComponent(conversationId), { waitUntil: "domcontentloaded" });
  const trailProcessingAppeared = await trailPage
    .waitForSelector('[data-audition-session="' + trailSessionId + '"]', { timeout: 15000 })
    .then(() => true)
    .catch(() => false);
  await trailPage.waitForTimeout(1500);
  const trailProcessing = await trailPage.evaluate(DOM_PROBE);
  await trailPage.screenshot({ path: join(outDir, "dom-audition-trail-processing.png") });
  writeFileSync(join(outDir, "dom-audition-trail-processing.json"), JSON.stringify(trailProcessing, null, 2), "utf-8");
  trailServed.push(...trailFixture.phase2);
  const trailSettledAppeared = await trailPage
    .waitForSelector('[data-audition-session="' + trailSessionId + '"].settled', { timeout: 15000 })
    .then(() => true)
    .catch(() => false);
  await trailPage.waitForTimeout(1200);
  const trailSettled = await trailPage.evaluate(DOM_PROBE);
  await trailPage.screenshot({ path: join(outDir, "dom-audition-trail-settled.png") });
  writeFileSync(join(outDir, "dom-audition-trail-settled.json"), JSON.stringify(trailSettled, null, 2), "utf-8");
  await trailContext.close();
  if (!trailProcessingAppeared) {
    record("audition-trail-T1", {
      failures: ["T1 setup: the preparing judge card never rendered for " + trailSessionId + " -- the pass would measure nothing"],
      notes: []
    });
  } else if (!trailSettledAppeared) {
    record("audition-trail-T1", {
      failures: ["T1 setup: the judge card never reached the .settled state after the phase-2 events were appended"],
      notes: []
    });
  } else {
    record("audition-trail-T1", checkT1({ processing: trailProcessing, settled: trailSettled }, {
      sessionId: trailSessionId
    }));
  }

  // ------------------------------------------------------ AB-JUDGMENT-CARD-1 J1
  // Parked-boundary events in the REAL forensic shape; the judgment endpoint is
  // intercepted at the network layer and answered with the real 409
  // identity-mismatch rejection (this isolated agent has no free-state loop
  // behind it -- the park settlement the POST unlocks keeps its Go-side
  // handler tests).
  report.audition_judgment_source =
    "parked-boundary events in the real forensic shape (kernel audition.ready snapshot WITHOUT turn domain + " +
    "trajectory.user_judgment.requested with run-level source_turn_id / experiment-domain payload.turn_id) replayed for " +
    "GET /agent/events; POST /agent/audition/judgment intercepted at the network layer and answered 409 with the real " +
    "identity-mismatch body";
  const j1SessionId = "audition:turn:free_state_e2e_abjudg1:round-1-e2e";
  const j1RunId = "run_e2e_abjudg1";
  const j1ExperimentTurnId = "turn:free_state_e2e_abjudg1";
  const j1RoundId = "round-1-e2e";
  const j1Seeded = judgmentIdentityFixtureEvents({
    runId: j1RunId, experimentTurnId: j1ExperimentTurnId, roundId: j1RoundId,
    sessionId: j1SessionId, baseSeq: 800
  });
  const j1Context = await browser.newContext({ viewport });
  const j1Diagnostics = { console: [], pageErrors: [] };
  const j1Posts = [];
  await j1Context.route("**/agent/audition/judgment*", async (route) => {
    const raw = route.request().postData() || "";
    const at = Date.now();
    try {
      j1Posts.push({ at, body: JSON.parse(raw) });
    } catch {
      j1Posts.push({ at, unparseable: raw.slice(0, 120) });
    }
    await route.fulfill({
      status: 409, contentType: "application/json; charset=utf-8",
      body: JSON.stringify({ status: "error", error: "audition session identity mismatch" })
    });
  });
  await installReplay(j1Context, { extraEvents: j1Seeded, strictConversation: true });
  const j1Page = await j1Context.newPage();
  j1Page.on("console", (message) => {
    j1Diagnostics.console.push(message.type() + ": " + message.text());
  });
  j1Page.on("pageerror", (error) => {
    j1Diagnostics.pageErrors.push(String(error));
  });
  await j1Page.goto(agentBase + "/app/?conversation_id=" + encodeURIComponent(conversationId), { waitUntil: "domcontentloaded" });
  const j1PickA = j1Page.locator('[data-audition-session="' + j1SessionId + '"] button[data-act="pickA"]');
  const j1Appeared = await j1PickA.waitFor({ state: "visible", timeout: 15000 }).then(() => true).catch(() => false);
  let j1Enabled = false;
  // MutationObserver before the click: distinguish "banner never rendered" from
  // "rendered then cleared" (a later setError("") would remove it silently).
  await j1Page.evaluate(() => {
    window.__j1NoticeLog = [];
    const log = window.__j1NoticeLog;
    const describe = (node) => (node.outerHTML || "").slice(0, 200);
    new MutationObserver((records) => {
      for (const record of records) {
        record.addedNodes.forEach((node) => {
          if (node.nodeType === 1 && (node.classList?.contains("notice") || node.querySelector?.(".notice"))) {
            log.push({ at: Date.now(), op: "add", html: describe(node) });
          }
        });
        record.removedNodes.forEach((node) => {
          if (node.nodeType === 1 && (node.classList?.contains("notice") || node.querySelector?.(".notice"))) {
            log.push({ at: Date.now(), op: "remove", html: describe(node) });
          }
        });
      }
    }).observe(document.body, { childList: true, subtree: true });
  });
  let j1ClickAt = 0;
  if (j1Appeared) {
    j1Enabled = await j1PickA.isEnabled().catch(() => false);
    if (j1Enabled) {
      j1ClickAt = Date.now();
      await j1PickA.click({ timeout: 10000 }).catch(() => {});
    }
  }
  await j1Page.waitForTimeout(1500);
  const j1Notice = j1Page.locator(".notice.warning").first();
  let j1NoticeVisible = await j1Notice.isVisible().catch(() => false);
  // Give the banner up to 6s total (React render + any debounced state) before
  // declaring the swallow still alive.
  for (let attempt = 0; attempt < 9 && !j1NoticeVisible; attempt += 1) {
    await j1Page.waitForTimeout(500);
    j1NoticeVisible = await j1Notice.isVisible().catch(() => false);
  }
  const j1NoticeText = j1NoticeVisible ? ((await j1Notice.textContent()) || "").replace(/\s+/g, " ").trim() : "";
  const j1NoticeCount = await j1Page.evaluate(() => ({
    notices: document.querySelectorAll(".notice").length,
    warnings: document.querySelectorAll(".notice.warning").length,
    warningHTML: (document.querySelector(".notice.warning") || { outerHTML: "" }).outerHTML.slice(0, 300),
    mutationLog: window.__j1NoticeLog || []
  })).catch(() => ({ notices: -1, warnings: -1, warningHTML: "evaluate failed", mutationLog: [] }));
  const j1Sample = await j1Page.evaluate(DOM_PROBE);
  await j1Page.screenshot({ path: join(outDir, "dom-audition-judgment.png") });
  writeFileSync(
    join(outDir, "dom-audition-judgment.json"),
    JSON.stringify({
      sample: j1Sample, posts: j1Posts, notice: j1NoticeText, noticeCount: j1NoticeCount,
      clickAt: j1ClickAt,
      diagnostics: j1Diagnostics,
      buttonsAppeared: j1Appeared, buttonsEnabled: j1Enabled, seeded_events: j1Seeded
    }, null, 2),
    "utf-8"
  );
  await j1Context.close();
  record("judgment-identity-J1", checkJ1(
    { appeared: j1Appeared && j1Enabled, posts: j1Posts, noticeVisible: j1NoticeVisible, noticeText: j1NoticeText },
    { sessionId: j1SessionId, runId: j1RunId, experimentTurnId: j1ExperimentTurnId, roundId: j1RoundId }
  ));

  // --------------------------------------------------- FIX-CONFIRM-CARD-1 K1/K2
  // Both passes drive the real composer. The chat / interaction-respond endpoints
  // are fulfilled at the network layer with RiskConfirm-gate fixtures (the
  // isolated agent has no kernel/LLM behind them); the event stream route keeps
  // the mutable-array shape the T1 pass uses so phase-2 events can be appended
  // mid-pass.
  report.confirm_card_source =
    "driven composer turns; POST /agent/chat and POST /agent/interaction/respond fulfilled at the network layer with " +
    "RiskConfirm mix-confirmation fixtures; K1 appends a real turn.completed(status=completed) to the replayed event " +
    "stream mid-pass; K2 patches /agent/ui/state with authority_mode=full_project_access";
  const confirmDrive = async (page, message) => {
    const composer = page.locator(".composer textarea").first();
    await composer.waitFor({ state: "visible", timeout: 15000 });
    await composer.fill(message);
    await composer.press("Enter");
  };

  // K1 (manual mode): the card must settle non-interactively the moment the turn
  // truly completes, with no self-initiated respond call.
  const k1TurnID = "run_e2e_confirm_k1";
  const k1InteractionID = "interaction_e2e_confirm_k1";
  const k1Context = await browser.newContext({ viewport });
  const k1Served = [...(eventsFixture.events || [])];
  await k1Context.route("**/agent/events*", async (route) => {
    const requestURL = new URL(route.request().url());
    const since = Number(requestURL.searchParams.get("since") || "0");
    const limit = Number(requestURL.searchParams.get("limit") || "120");
    const nextSeq = k1Served.reduce((maximum, event) => Math.max(maximum, Number(event.seq) || 0), 0);
    await route.fulfill({
      status: 200, contentType: "application/json; charset=utf-8",
      body: JSON.stringify({ status: "ok", events: k1Served.filter((event) => Number(event.seq) > since).slice(0, limit), next_seq: nextSeq })
    });
  });
  let k1RespondHits = 0;
  await k1Context.route("**/agent/chat*", async (route) => {
    await route.fulfill({
      status: 200, contentType: "application/json; charset=utf-8",
      body: JSON.stringify(confirmationChatResponseFixture({ turnID: k1TurnID, interactionID: k1InteractionID }))
    });
  });
  await k1Context.route("**/agent/interaction/respond*", async (route) => {
    k1RespondHits += 1;
    await route.fulfill({
      status: 200, contentType: "application/json; charset=utf-8",
      body: JSON.stringify(directExecutionReceiptFixture({ turnID: k1TurnID, interactionID: k1InteractionID }))
    });
  });
  const k1Page = await k1Context.newPage();
  await k1Page.goto(agentBase + "/app/?conversation_id=" + encodeURIComponent(conversationId), { waitUntil: "domcontentloaded" });
  await confirmDrive(k1Page, "把 Track 2 提升 1dB");
  const k1Appeared = await k1Page
    .waitForSelector(".action-card.interactive", { timeout: 15000 })
    .then(() => true)
    .catch(() => false);
  await k1Page.waitForTimeout(400);
  const k1Before = await k1Page.evaluate(DOM_PROBE);
  // Phase 2: the turn truly completes (status completed -- NOT a waiting_continue
  // slice boundary) while the card was never answered.
  k1Served.push({
    seq: 5001, type: "turn.completed", conversation_id: conversationId,
    goal_id: k1TurnID, run_id: k1TurnID, turn_id: k1TurnID, item_type: "turn",
    status: "completed", created_at: new Date().toISOString()
  });
  await k1Page.waitForTimeout(2600);
  const k1After = await k1Page.evaluate(DOM_PROBE);
  await k1Page.screenshot({ path: join(outDir, "dom-confirm-k1-settled.png") });
  writeFileSync(join(outDir, "dom-confirm-k1-before.json"), JSON.stringify(k1Before, null, 2), "utf-8");
  writeFileSync(join(outDir, "dom-confirm-k1-after.json"), JSON.stringify(k1After, null, 2), "utf-8");
  await k1Context.close();
  report.confirm_k1 = { appeared: k1Appeared, respond_hits: k1RespondHits };
  record("confirm-card-K1", checkK1({ appeared: k1Appeared, before: k1Before, after: k1After, respondHits: k1RespondHits }));

  // K2 (full access): no pre-card, exactly one approve, notice + receipt visible.
  const k2TurnID = "run_e2e_confirm_k2";
  const k2InteractionID = "interaction_e2e_confirm_k2";
  const k2ReceiptMarker = "Track 2 提升 +1.0 dB";
  const k2Context = await browser.newContext({ viewport });
  await installReplay(k2Context);
  await k2Context.route("**/agent/ui/state*", async (route) => {
    const response = await route.fetch();
    const body = await response.json().catch(() => ({}));
    await route.fulfill({ response, json: { ...body, authority_mode: "full_project_access" } });
  });
  await k2Context.route("**/agent/chat*", async (route) => {
    await route.fulfill({
      status: 200, contentType: "application/json; charset=utf-8",
      body: JSON.stringify(confirmationChatResponseFixture({ turnID: k2TurnID, interactionID: k2InteractionID }))
    });
  });
  let k2RespondHits = 0;
  await k2Context.route("**/agent/interaction/respond*", async (route) => {
    k2RespondHits += 1;
    await route.fulfill({
      status: 200, contentType: "application/json; charset=utf-8",
      body: JSON.stringify(directExecutionReceiptFixture({ turnID: k2TurnID, interactionID: k2InteractionID }))
    });
  });
  const k2Page = await k2Context.newPage();
  const k2Console = [];
  k2Page.on("console", (message) => {
    const text = message.text();
    if (message.type() === "error" || text.indexOf("[AskVit") >= 0) {
      k2Console.push(message.type() + ": " + text.slice(0, 400));
    }
  });
  k2Page.on("pageerror", (error) => {
    k2Console.push("pageerror: " + String(error).slice(0, 400));
  });
  await k2Page.goto(agentBase + "/app/?conversation_id=" + encodeURIComponent(conversationId), { waitUntil: "domcontentloaded" });
  // Wait for the authority pill to actually show 完全访问 before driving: the
  // mode is restored from /agent/ui/state, and a send that races ahead of that
  // restore would still run under manual mode (the app's own real-world cue for
  // "the grant is active" is this pill).
  const k2AuthorityReady = await k2Page
    .locator(".authority-select-button", { hasText: "完全访问" })
    .waitFor({ timeout: 15000 })
    .then(() => true)
    .catch(() => false);
  await confirmDrive(k2Page, "把 Track 2 提升 1dB");
  await k2Page.waitForTimeout(3200);
  const k2Sample = await k2Page.evaluate(DOM_PROBE);
  const k2Storage = await k2Page.evaluate(() => {
    const out = {};
    for (const key of Object.keys(window.localStorage)) {
      if (key.indexOf("ask_vit_consumed") >= 0) {
        out[key] = window.localStorage.getItem(key) || "";
        continue;
      }
      if (key.indexOf("ask_vit_conversation_messages") !== 0) {
        continue;
      }
      try {
        const bucket = JSON.parse(window.localStorage.getItem(key) || "{}");
        out[key] = {
          conversation_id: bucket.conversation_id,
          saved_at: bucket.saved_at,
          messages: (bucket.messages || []).map((message) => ({
            id: message.id,
            role: message.role,
            content: String(message.content || "").slice(0, 80),
            message_kind: message.message_kind,
            turn_id: message.turn_id,
            lifecycle: message.lifecycle,
            persistence: message.persistence,
            actions: (message.actions || []).map((action) => ({
              id: action.id, kind: action.kind, status: action.status,
              resolved_action_id: action.resolved_action_id, _ui_source: action._ui_source,
              buttons: Array.isArray(action.actions) ? action.actions.length : null
            }))
          }))
        };
      } catch (err) {
        out[key] = { parse_error: String(err) };
      }
    }
    return out;
  });
  await k2Page.screenshot({ path: join(outDir, "dom-confirm-k2-direct.png") });
  writeFileSync(join(outDir, "dom-confirm-k2.json"), JSON.stringify(k2Sample, null, 2), "utf-8");
  writeFileSync(join(outDir, "dom-confirm-k2-storage.json"), JSON.stringify(k2Storage, null, 2), "utf-8");
  writeFileSync(join(outDir, "dom-confirm-k2-console.txt"), k2Console.join("\n"), "utf-8");
  await k2Context.close();
  report.confirm_k2 = { authority_ready: k2AuthorityReady, respond_hits: k2RespondHits, receipt_marker: k2ReceiptMarker };
  record("full-access-K2", checkK2({ sample: k2Sample, respondHits: k2RespondHits, authorityReady: k2AuthorityReady }, {
    receiptMarker: k2ReceiptMarker,
    interactionID: k2InteractionID
  }));

  // --------------------------------------------------- OPT-OBSERVE-OUTPUT-1 O1
  // Layering pass driven through the real composer (manual mode): context A
  // exercises the long reply end to end (collapsed default -> click to expand ->
  // reload recomputes), contexts B/C/D assert the not-folded family boundary.
  // Each context is fresh (empty localStorage), so the passes cannot see each
  // other's driven messages.
  report.observe_output_source =
    "driven composer turns; POST /agent/chat fulfilled at the network layer with observation-Q&A fixtures " +
    "(long layered reply / short reply / long confirmation-card message / long execution receipt); " +
    "context A reloads mid-pass to prove the collapsed default state recomputes";
  const observeWaitForRow = async (page, marker) => {
    await page
      .locator(".message-row.assistant", { hasText: marker })
      .first()
      .waitFor({ timeout: 15000 })
      .then(() => true)
      .catch(() => false);
    await page.waitForTimeout(300);
    return page.evaluate(observeRowProbe, marker);
  };
  // OPT-OBSERVE-OUTPUT-1 forensics fix: the composer send must wait until the
  // history scope has materialized (the archived graph's first user message is
  // rendered by the same history-sync effect that anchors the storage bucket).
  // Driving earlier saved the driven messages into the unsaved_root bucket
  // (run 2 evidence: dom-observe-reload-samples.json), so the reload restore
  // read the draft-scope bucket and never saw them -- a test-side race, not a
  // layering defect.
  const observeWaitForScopeMaterialized = async (page) => {
    await page
      .locator(".message-row", { hasText: "对低音轨做一次混音改进实验" })
      .first()
      .waitFor({ timeout: 20000 })
      .catch(() => {});
    await page.waitForTimeout(500);
  };

  // Context A: long reply -> collapsed default -> expand. (The reload leg of
  // this context moved to context E, which exercises the SAME recompute through
  // the real Project History hydration path -- see the context E comment for
  // why the localStorage-driven reload is structurally unable to carry it.)
  const observeAContext = await browser.newContext({ viewport });
  await installReplay(observeAContext);
  await observeAContext.route("**/agent/chat*", async (route) => {
    await route.fulfill({
      status: 200, contentType: "application/json; charset=utf-8",
      body: JSON.stringify(observeLongReplyFixture())
    });
  });
  const observeAPage = await observeAContext.newPage();
  await observeAPage.goto(agentBase + "/app/?conversation_id=" + encodeURIComponent(conversationId), { waitUntil: "domcontentloaded" });
  await observeWaitForScopeMaterialized(observeAPage);
  await confirmDrive(observeAPage, "Track 2 的 2.1kHz 峰值是从哪里来的？给出完整观察证据。");
  const observeDefaultState = await observeWaitForRow(observeAPage, "OBSRESTFINAL");
  await observeAPage.screenshot({ path: join(outDir, "dom-observe-collapsed.png") });
  let observeExpandedState = null;
  if (observeDefaultState && observeDefaultState.layered) {
    await observeAPage.locator(".observe-output-details > summary").first().click();
    await observeAPage
      .locator(".observe-output-details[open]")
      .first()
      .waitFor({ timeout: 5000 })
      .catch(() => {});
    await observeAPage.waitForTimeout(300);
    observeExpandedState = await observeAPage.evaluate(observeRowProbe, "OBSRESTFINAL");
    await observeAPage.screenshot({ path: join(outDir, "dom-observe-expanded.png") });
  }
  writeFileSync(join(outDir, "dom-observe-a.json"), JSON.stringify({
    defaultState: observeDefaultState, expandedState: observeExpandedState
  }, null, 2), "utf-8");
  await observeAContext.close();

  // Context B: short reply must stay plain.
  const observeBContext = await browser.newContext({ viewport });
  await installReplay(observeBContext);
  await observeBContext.route("**/agent/chat*", async (route) => {
    await route.fulfill({
      status: 200, contentType: "application/json; charset=utf-8",
      body: JSON.stringify(observeShortReplyFixture())
    });
  });
  const observeBPage = await observeBContext.newPage();
  await observeBPage.goto(agentBase + "/app/?conversation_id=" + encodeURIComponent(conversationId), { waitUntil: "domcontentloaded" });
  await observeWaitForScopeMaterialized(observeBPage);
  await confirmDrive(observeBPage, "一句话说明当前状态。");
  const observeShortState = await observeWaitForRow(observeBPage, "OBSHORT");
  await observeBContext.close();

  // Context C: long confirmation-card message must stay unlayered (interactive
  // card renders -- premise of the boundary).
  const observeCTurnID = "run_e2e_observe_card";
  const observeCInteractionID = "interaction_e2e_observe_card";
  const observeCContext = await browser.newContext({ viewport });
  await installReplay(observeCContext);
  let observeCChatHits = 0;
  await observeCContext.route("**/agent/chat*", async (route) => {
    observeCChatHits += 1;
    await route.fulfill({
      status: 200, contentType: "application/json; charset=utf-8",
      body: JSON.stringify(observeLongProposalFixture(observeCTurnID, observeCInteractionID))
    });
  });
  const observeCPage = await observeCContext.newPage();
  const observeCConsole = [];
  observeCPage.on("pageerror", (error) => {
    observeCConsole.push("pageerror: " + String(error).slice(0, 300));
  });
  await observeCPage.goto(agentBase + "/app/?conversation_id=" + encodeURIComponent(conversationId), { waitUntil: "domcontentloaded" });
  await observeWaitForScopeMaterialized(observeCPage);
  await confirmDrive(observeCPage, "把 Track 2 提升 1dB");
  const observeCardAppeared = await observeCPage
    .locator(".action-card")
    .first()
    .waitFor({ timeout: 15000 })
    .then(() => true)
    .catch(() => false);
  const observeCardState = observeCardAppeared ? await observeWaitForRow(observeCPage, "OBSCARD") : null;
  await observeCPage.screenshot({ path: join(outDir, "dom-observe-card.png") });
  writeFileSync(join(outDir, "dom-observe-card.json"), JSON.stringify({
    chatHits: observeCChatHits,
    cardAppeared: observeCardAppeared,
    rowState: observeCardState,
    flow: await observeCPage.evaluate(() => Array.from(document.querySelectorAll(".message-row")).map((row) => ({
      cls: row.className,
      text: (row.textContent || "").replace(/\s+/g, " ").trim().slice(0, 120)
    }))),
    console: observeCConsole
  }, null, 2), "utf-8");
  await observeCContext.close();

  // Context D: long execution receipt must stay unlayered.
  const observeDTurnID = "run_e2e_observe_receipt";
  const observeDContext = await browser.newContext({ viewport });
  await installReplay(observeDContext);
  await observeDContext.route("**/agent/chat*", async (route) => {
    await route.fulfill({
      status: 200, contentType: "application/json; charset=utf-8",
      body: JSON.stringify(observeLongReceiptFixture(observeDTurnID))
    });
  });
  const observeDPage = await observeDContext.newPage();
  await observeDPage.goto(agentBase + "/app/?conversation_id=" + encodeURIComponent(conversationId), { waitUntil: "domcontentloaded" });
  await observeWaitForScopeMaterialized(observeDPage);
  await confirmDrive(observeDPage, "执行完成了吗？");
  const observeReceiptState = await observeWaitForRow(observeDPage, "OBSRECEIPT");
  await observeDPage.screenshot({ path: join(outDir, "dom-observe-receipt.png") });
  await observeDContext.close();

  // Context E (O1c): the hydration leg through the REAL Project History path.
  // A seeded graph node carries the long assistant reply with message_kind
  // "assistant"; the page hydrates it through /agent/ui/state and the collapsed
  // default state must recompute from the content alone (expansion is pure UI
  // state: never persisted, never remembered across a reload).
  //
  // Why not reload the driven message of context A? Forensic finding (runs
  // 20260930_190233 / _190740 + artifacts/o1debug/mini_repro.mjs): on reload the
  // scope-materializing history sync replaces the flow with the graph messages
  // and its SAVE effect overwrote the localStorage message bucket BEFORE the
  // restore effect got its first chance (the restore gate needs the scope
  // anchor, which is only set later in the same commit) -- driven messages that
  // exist only in the local bucket were structurally lost on reload. Fixed by
  // FIX-BUCKET-SAVE-RACE-1: restore now re-runs on the messages change in the
  // same commit, before save (declaration order + the existing save-skip), and
  // context F below asserts the driven reply comes back through a real reload.
  // The layering assertion here still rides the server-side hydration path,
  // which is the path real observation Q&A turns take.
  const observeSeed = await (async () => {
    const state = await getJSON("/agent/ui/state");
    const history = state.project_history || {};
    const draftDir = dirname(history.project_path || "");
    const historyDir = history.history_dir || join(draftDir, ".vit_history");
    const stateDir = history.state_dir || "";
    if (!draftDir || !stateDir) {
      throw new Error("O1 seed: agent ui/state did not expose a draft project history (project_path/state_dir)");
    }
    const commitsDir = join(historyDir, "commits");
    mkdirSync(commitsDir, { recursive: true });
    // Reuse the shipped synthetic commit objects as the schema base (they point
    // at the real project snapshot, so every reader-side validation holds);
    // seedConversation-style rewrites pin them to the draft history.
    const baseCommitA = JSON.parse(readFileSync(join(here, "fixtures", "webui_rendered_dom", "msg_revive_commits", "c_20260915T063010_220fb85b.json"), "utf-8"));
    const baseCommitB = JSON.parse(readFileSync(join(here, "fixtures", "webui_rendered_dom", "msg_revive_commits", "c_20260915T063030_b5459d0d.json"), "utf-8"));
    const longReply = observeLongReplyFixture();
    const commitIds = { ask: "c_o1seed_ask", reply: "c_o1seed_reply" };
    const commitFor = (base, commitID, runID, parents) => ({
      ...base,
      id: commitID,
      goal_id: "goal_o1seed",
      run_id: runID,
      parents
    });
    writeFileSync(join(commitsDir, commitIds.ask + ".json"), JSON.stringify(commitFor(baseCommitA, commitIds.ask, "run_o1seed", undefined), null, 2), "utf-8");
    writeFileSync(join(commitsDir, commitIds.reply + ".json"), JSON.stringify(commitFor(baseCommitB, commitIds.reply, "run_o1seed", [commitIds.ask]), null, 2), "utf-8");
    const nodeFor = (nodeID, kind, commitID, parentID, text, messageKind) => ({
      id: nodeID,
      kind,
      commit_id: commitID,
      parent_node_id: parentID,
      branch: history.active_branch || "main",
      text,
      text_preview: text.slice(0, 120),
      goal_id: "goal_o1seed",
      run_id: "run_o1seed",
      lifecycle: "durable",
      persistence: "project_history",
      message_kind: messageKind,
      turn_id: "run_o1seed",
      logical_message_id: nodeID,
      created_at: new Date().toISOString()
    });
    const graph = {
      project_path: history.project_path,
      project_uuid: history.project_uuid || "",
      active_branch: history.active_branch || "main",
      active_node_id: "n_o1seed_reply",
      nodes: [
        nodeFor("n_o1seed_ask", "ask", commitIds.ask, undefined, "Track 2 的 2.1kHz 峰值是从哪里来的？给出完整观察证据。", "user"),
        nodeFor("n_o1seed_reply", "vit", commitIds.reply, "n_o1seed_ask", longReply.reply, "assistant")
      ]
    };
    const graphPath = join(stateDir, "conversation_graph.json");
    writeFileSync(graphPath, JSON.stringify(graph, null, 2), "utf-8");
    return { graphPath, commitsDir };
  })();

  const observeEContext = await browser.newContext({ viewport });
  // Empty event stream: this pass asserts the HYDRATED conversation shape, not
  // the archived replay's trace blocks.
  await observeEContext.route("**/agent/events*", async (route) => {
    await route.fulfill({ status: 200, contentType: "application/json; charset=utf-8", body: JSON.stringify({ status: "ok", events: [], next_seq: 0 }) });
  });
  const observeEPage = await observeEContext.newPage();
  await observeEPage.goto(agentBase + "/app/?conversation_id=" + encodeURIComponent(conversationId), { waitUntil: "domcontentloaded" });
  const observeHydratedState = await observeWaitForRow(observeEPage, "OBSRESTFINAL");
  await observeEPage.screenshot({ path: join(outDir, "dom-observe-hydrated.png") });
  let observeHydratedExpandedState = null;
  if (observeHydratedState && observeHydratedState.layered) {
    await observeEPage.locator(".observe-output-details > summary").first().click();
    await observeEPage
      .locator(".observe-output-details[open]")
      .first()
      .waitFor({ timeout: 5000 })
      .catch(() => {});
    await observeEPage.waitForTimeout(300);
    observeHydratedExpandedState = await observeEPage.evaluate(observeRowProbe, "OBSRESTFINAL");
  }
  await observeEPage.reload({ waitUntil: "domcontentloaded" });
  const observeRehydrateSamples = [];
  for (let index = 0; index < 8; index += 1) {
    await observeEPage.waitForTimeout(2500);
    observeRehydrateSamples.push(await observeEPage.evaluate(observeRowProbe, "OBSRESTFINAL"));
  }
  const observeRehydratedState = observeRehydrateSamples.find((sample) => sample && sample.layered) || null;
  const observeEPersisted = await observeEPage.evaluate(() => {
    const out = {};
    for (const key of Object.keys(window.localStorage)) {
      if (key.indexOf("ask_vit_conversation_messages") !== 0) {
        continue;
      }
      try {
        const bucket = JSON.parse(window.localStorage.getItem(key) || "{}");
        out[key.slice(0, 80)] = (bucket.messages || []).map((message) => ({
          role: message.role,
          message_kind: message.message_kind,
          contentLength: String(message.content || "").length,
          hasRestMarker: String(message.content || "").indexOf("OBSRESTFINAL") >= 0
        }));
      } catch (error) {
        out[key.slice(0, 80)] = { parse_error: String(error) };
      }
    }
    return out;
  });
  await observeEPage.screenshot({ path: join(outDir, "dom-observe-rehydrated.png") });
  writeFileSync(join(outDir, "dom-observe-e.json"), JSON.stringify({
    seed: observeSeed,
    hydratedState: observeHydratedState,
    hydratedExpandedState: observeHydratedExpandedState,
    rehydrateSamples: observeRehydrateSamples,
    rehydratedState: observeRehydratedState,
    persistedBuckets: observeEPersisted
  }, null, 2), "utf-8");
  await observeEContext.close();

  record("observe-output-O1", checkO1({
    long: {
      defaultState: observeDefaultState,
      expandedState: observeExpandedState,
      hydratedState: observeHydratedState,
      rehydratedState: observeRehydratedState,
      persistedBuckets: observeEPersisted,
      persistedFullContentLength: observeLongReplyFixture().reply.length
    },
    short: observeShortState,
    proposal: observeCardState,
    receipt: observeReceiptState,
    // waiting interactions render on the composer interaction surface, NOT
    // inside the message row (run-2 forensics: row actionCards=0 while the
    // interactive card is visible page-wide) -- the premise is page-level.
    cardPremise: observeCardAppeared
  }));

  // ----------------------------- FIX-BUCKET-SAVE-RACE-1 (O1 forensic followup)
  // The bucket save/restore ordering leg on the REAL reload path. Context E's
  // seed supplies the graph; a composer-driven turn here lands a reply that has
  // NO graph node, so the localStorage bucket is its only carrier. The reload
  // keeps the URL conversation id stable, the scope-materializing history sync
  // replaces the flow with the graph messages, and the save effect used to
  // overwrite the bucket before the restore effect ever ran (restore only
  // re-fired on [conversationID, uiState]; neither changes when the URL anchors
  // the id) -- the driven reply was structurally lost. The fix re-runs restore
  // on the messages change in the same commit, BEFORE save (declaration order +
  // the existing save-skip), so the reply must come back. Unit-level pipeline:
  // agent/webui/src/bucketSaveRace.test.ts.
  report.bucket_save_race_source =
    "composer-driven turn (POST /agent/chat fulfilled with the local-tail fixture; no graph node) + real /agent/ui/state " +
    "graph hydration from the O1 seed; the reload keeps the URL conversation id, so the driven reply exists only in the localStorage bucket";
  const observeFContext = await browser.newContext({ viewport });
  await observeFContext.route("**/agent/events*", async (route) => {
    await route.fulfill({ status: 200, contentType: "application/json; charset=utf-8", body: JSON.stringify({ status: "ok", events: [], next_seq: 0 }) });
  });
  await observeFContext.route("**/agent/chat*", async (route) => {
    await route.fulfill({ status: 200, contentType: "application/json; charset=utf-8", body: JSON.stringify(observeLocalTailReplyFixture()) });
  });
  const observeFPage = await observeFContext.newPage();
  await observeFPage.goto(agentBase + "/app/?conversation_id=" + encodeURIComponent(conversationId), { waitUntil: "domcontentloaded" });
  await observeWaitForScopeMaterialized(observeFPage);
  await confirmDrive(observeFPage, "再补一条只进本地桶的观察证据");
  const observeFLocalState = await observeWaitForRow(observeFPage, "OBSLOCALFINAL");
  const observeFBucketRows = async () => observeFPage.evaluate((marker) => {
    const rows = [];
    for (const key of Object.keys(window.localStorage)) {
      if (key.indexOf("ask_vit_conversation_messages") !== 0) {
        continue;
      }
      try {
        const bucket = JSON.parse(window.localStorage.getItem(key) || "{}");
        rows.push({
          key: key.slice(0, 90),
          n: (bucket.messages || []).length,
          has_marker: (bucket.messages || []).some((message) => String(message.content || "").indexOf(marker) >= 0)
        });
      } catch (error) {
        rows.push({ key: key.slice(0, 90), parse_error: String(error) });
      }
    }
    return rows;
  }, "OBSLOCALFINAL");
  let observeFBucketBefore = await observeFBucketRows();
  for (let attempt = 0; attempt < 10 && !observeFBucketBefore.some((row) => row.has_marker); attempt += 1) {
    await observeFPage.waitForTimeout(600);
    observeFBucketBefore = await observeFBucketRows();
  }
  await observeFPage.screenshot({ path: join(outDir, "dom-observe-f-before-reload.png") });
  await observeFPage.reload({ waitUntil: "domcontentloaded" });
  const observeFRehydrateSamples = [];
  for (let index = 0; index < 10; index += 1) {
    await observeFPage.waitForTimeout(2500);
    observeFRehydrateSamples.push(await observeFPage.evaluate(observeRowProbe, "OBSLOCALFINAL"));
  }
  const observeFBucketAfter = await observeFBucketRows();
  await observeFPage.screenshot({ path: join(outDir, "dom-observe-f-after-reload.png") });
  writeFileSync(join(outDir, "dom-verify-bucket-save-race-f.json"), JSON.stringify({
    localState: observeFLocalState,
    bucketBefore: observeFBucketBefore,
    rehydrateSamples: observeFRehydrateSamples,
    bucketAfter: observeFBucketAfter
  }, null, 2), "utf-8");
  await observeFContext.close();
  const observeFFailures = [];
  if (!observeFLocalState) {
    observeFFailures.push("driven local-tail reply never rendered before the reload (drive fixture or composer failed)");
  }
  if (!observeFBucketBefore.some((row) => row.has_marker)) {
    observeFFailures.push("driven local-tail reply never reached a localStorage bucket before the reload (precondition)");
  }
  if (!observeFRehydrateSamples.some((sample) => sample)) {
    observeFFailures.push("local-tail reply did not return to the screen within the rehydrate window (structural reload loss)");
  }
  if (!observeFBucketAfter.some((row) => row.has_marker)) {
    observeFFailures.push("local-tail marker gone from every bucket after the reload (bucket clobbered by the graph-only save)");
  }
  record("bucket-save-race-F1", {
    failures: observeFFailures,
    notes: [
      "driven reply has no graph node; the (conversation, scope) bucket is its only carrier",
      "buckets before reload: " + JSON.stringify(observeFBucketBefore),
      "rehydrate non-null samples: " + observeFRehydrateSamples.filter(Boolean).length + "/" + observeFRehydrateSamples.length
    ]
  });

  // ------------------------------------------------------------- MSG-REVIVE-1
  // Runs LAST: its phase 1 re-seeds the draft conversation graph with the r3
  // capture, so every group that asserts against the archived mtzba6wf graph
  // must have completed first.
  report.msgrevive_events_source =
    "r3 forensic capture (conversation " + msgConversationId + ", normal-permission mix-tick chain, 41 events incl. " +
    "mix_tick.pending / mix_tick.confirmation.routed / two interaction.resolved) replayed conversation-strict; the r3-shaped " +
    "graph (7 nodes, interaction-carrying proposal node with a waiting_for_user send-time snapshot) is re-seeded over the " +
    "draft history; /agent/runtime/status gets the conversation's completed continuation row appended as the server-side " +
    "identity anchor; phase 2 opens /app/ in a fresh context with NO localStorage at all";
  const msgRevivePass = await runMsgRevivePass("msgrevive", {
    appearTimeoutMs: 25000,
    settleMs: 5000,
    lateSettleMs: 20000,
    blindUiStateTicks: 1
  });
  report.msgrevive = {
    conversation_id: msgConversationId,
    seeded: {
      commits_written: msgRevivePass.seeded ? msgRevivePass.seeded.commitsWritten : 0,
      nodes_skipped: msgRevivePass.seeded ? msgRevivePass.seeded.nodesSkipped : []
    },
    real_scope_key: msgRevivePass.realScopeKey,
    learning_anchor: msgRevivePass.learned[msgRevivePass.realScopeKey] || "",
    appeared: msgRevivePass.appeared,
    late_sample_summary: msgRevivePass.late
      ? {
          user_messages: (msgRevivePass.late.userMessages || []).length,
          assistant_messages: (msgRevivePass.late.assistantMessages || []).length,
          interactive_cards: (msgRevivePass.late.interactiveCards || []).length
        }
      : null
  };
  record("msgrevive-H1", checkH1(msgRevivePass, {
    conversationId: msgConversationId,
    expectUserTexts: ["检查一下当前工程有什么问题", "可以执行"],
    expectAssistantTexts: ["已在「bass」轨"],
    expectResolvedInteractionIDs: ["interaction_756c9bd8111b0d6e", "interaction_855170be9aac6b30"]
  }));

  // ---------------------------------------------------------- WEBUI-MSG-ORDER-1
  // Runs AFTER msgrevive (both passes re-seed the draft graph; M1 is last).
  report.msgorder_events_source =
    "M8 forensic capture (conversation " + msgOrderConversationId + ", waiting_continue run run_e5796736a4865570 spanning " +
    "two user rounds, 47 events incl. the same-id lifecycle flip started→completed→items→completed→started→failed and the " +
    "same-id double-emission pairs) replayed conversation-strict; the M8-shaped graph (5 nodes: both user rows carry " +
    "turn_id=run_e5796736a4865570, assistant rows chat-domain turn_*) is re-seeded over the draft history";
  const msgOrderPass = await runMsgOrderPass("msgorder", {
    appearTimeoutMs: 25000,
    settleMs: 4000
  });
  report.msgorder = {
    conversation_id: msgOrderConversationId,
    seeded: {
      commits_written: msgOrderPass.seeded ? msgOrderPass.seeded.commitsWritten : 0,
      nodes_skipped: msgOrderPass.seeded ? msgOrderPass.seeded.nodesSkipped : []
    },
    appeared: msgOrderPass.appeared
  };
  record("msg-order-M1", checkM1({
    sample: msgOrderPass.sample,
    appeared: msgOrderPass.appeared,
    expectedRunId: "run_e5796736a4865570"
  }));

  // ---------------------------------------------------------- WEBUI-MSG-ORDER-2
  // Runs LAST (after msgrevive/msgorder re-seeded the draft graph): its ui/state
  // projection serves an EMPTY transcript for this pass's own mounts, so the
  // stream is the pure composer-driven live shape. Round 1 = driven composer +
  // park event replay (audition family carries NO turn domain); round 2 = driven
  // composer + immediate turn.failed events. The gate asserts the A/B judge card
  // renders at its chronological slot ABOVE the round-2 input (pre-fix it was
  // pinned at the flow tail below the failure receipt -- the M1-RETEST visual).
  report.msgorder2_events_source =
    "composer-driven live pass (NOT a pure replay): conversation " + msgOrder2ConversationId + " mounts with an empty " +
    "transcript (ui/state projection, PLANBAR-1 pattern); turn 1 is driven through the REAL composer with POST /agent/chat " +
    "fulfilled at the network layer (execution receipt fixture) and the M1-RETEST park replayed through the event buffer " +
    "(B9 run fold with free-state payload turn ids, mix_tick.pending, audition family WITHOUT any turn domain); turn 2 is " +
    "driven through the REAL composer (failure receipt fixture) with turn.started/turn.failed events appended live";
  const msgOrder2Pass = await runMsgOrder2Pass("msgorder2");
  report.msgorder2 = {
    conversation_id: msgOrder2ConversationId,
    phase1: {
      audition_card_appeared: msgOrder2Pass.phase1.auditionCardAppeared,
      trace_appeared: msgOrder2Pass.phase1.traceAppeared
    }
  };
  record("msg-order-M2", checkM2({ finalSample: msgOrder2Pass.finalSample, phase1: msgOrder2Pass.phase1 }));

  // ------------------------------------------------------ WEBUI-IA-REDESIGN-1
  // The three-zone redesign pass: real browser, real bundle, real interactions.
  // Events replay conversation-strict (a freshly minted webui_ conversation gets
  // an empty buffer exactly like the live agent), the plan bar is fed a running
  // task through the network layer (PLANBAR-1 pattern) so the statusbar id shape
  // is asserted on the live DOM, and every sidebar operation (new / rename /
  // switch / archive / restore / collapse) is driven by real clicks.
  report.ia_redesign_source =
    "real interactions on the rendered surface: archived stream replayed conversation-strict for GET /agent/events; " +
    "task runtime state injected at the network layer (PLANBAR-1 pattern) so .plan-bar is live; sidebar operations " +
    "driven by real Playwright clicks against the bundle under test";
  const runIaRedesignPass = async (name) => {
    const context = await browser.newContext({ viewport });
    await installReplay(context, { strictConversation: true });
    await installTaskState(context, planBarTaskFixture("observation_in_progress"), "running");
    const page = await context.newPage();
    await page.goto(agentBase + "/app/?conversation_id=" + encodeURIComponent(conversationId), { waitUntil: "domcontentloaded" });
    await page.waitForTimeout(2500);

    const result = {
      appeared: false,
      currentRowActive: false,
      numbersLinear: null,
      registryKeys: [],
      newConversation: { rowAppeared: false, newID: "", activeID: "" },
      rename: { inputAppeared: false, titleUpdated: false },
      switchBack: { activeRestored: false },
      archive: { hiddenFromList: false, toggleAppeared: false, restored: false },
      collapse: { collapsedClass: false, expandedBack: false },
      deadNodes: {},
      planBarAppeared: false,
      planBarHeadText: "",
      planBarTaskElementCount: 0,
      planBarDataTaskID: ""
    };

    // IA3 first: the injected running task drives the plan bar at mount time.
    // Sidebar operations below mint a new conversation, whose [conversationID]
    // effect legitimately clears the task trajectory -- sampling the bar after
    // the ops would read the goal-fallback render, not the fixture.
    result.planBarAppeared = await page.waitForSelector(".plan-bar", { timeout: 8000 }).then(() => true).catch(() => false);
    if (result.planBarAppeared) {
      const bar = await page.evaluate(() => {
        const el = document.querySelector(".plan-bar");
        const head = el ? el.querySelector(".plan-bar-head") : null;
        return {
          headText: head ? (head.textContent || "").replace(/\s+/g, " ").trim() : "",
          taskElementCount: document.querySelectorAll(".plan-bar-task").length,
          dataTaskID: el ? el.getAttribute("data-task-id") || "" : ""
        };
      });
      result.planBarHeadText = bar.headText;
      result.planBarTaskElementCount = bar.taskElementCount;
      result.planBarDataTaskID = bar.dataTaskID;
    }
    await page.screenshot({ path: join(outDir, "dom-" + name + "-mount.png"), fullPage: false });

    result.appeared = await page.waitForSelector(".session-sidebar", { timeout: 10000 }).then(() => true).catch(() => false);
    if (result.appeared) {
      result.currentRowActive = await page
        .locator('.session-list .session-row[data-conversation-id="' + conversationId + '"].active')
        .count()
        .then((count) => count > 0)
        .catch(() => false);
      result.numbersLinear = await page.evaluate(() => {
        const numbers = Array.from(document.querySelectorAll(".session-sidebar .session-number")).map((el) => (el.textContent || "").trim());
        return { count: numbers.length, allLinear: numbers.length > 0 && numbers.every((text) => /^[0-9]+$/.test(text)), values: numbers.slice(0, 8) };
      });
      result.registryKeys = await page.evaluate(() => Object.keys(window.localStorage));

      // 新建会话（轻量：webui_ id，不建工作树不建分支）
      await page.click('.session-sidebar [data-action="new"]').catch(() => {});
      await page.waitForTimeout(900);
      const afterNew = await page.evaluate(() => {
        const rows = Array.from(document.querySelectorAll(".session-list .session-row"));
        const active = document.querySelector(".session-list .session-row.active");
        return {
          ids: rows.map((row) => row.getAttribute("data-conversation-id") || ""),
          activeID: active ? active.getAttribute("data-conversation-id") || "" : ""
        };
      });
      result.newConversation.newID = afterNew.ids.find((id) => id && id !== conversationId) || "";
      result.newConversation.rowAppeared = afterNew.ids.length >= 2;
      result.newConversation.activeID = afterNew.activeID;

      // 重命名（inline input）
      if (result.newConversation.newID) {
        const newRow = page.locator('.session-list .session-row[data-conversation-id="' + result.newConversation.newID + '"]');
        await newRow.hover().catch(() => {});
        await page.click('.session-list .session-row[data-conversation-id="' + result.newConversation.newID + '"] [data-action="rename"]').catch(() => {});
        const renameInput = page.locator(".session-rename-input");
        result.rename.inputAppeared = await renameInput.waitFor({ state: "visible", timeout: 4000 }).then(() => true).catch(() => false);
        if (result.rename.inputAppeared) {
          await renameInput.fill("E2E 重命名会话");
          await renameInput.press("Enter");
          await page.waitForTimeout(500);
          result.rename.titleUpdated = await page
            .locator('.session-list .session-row[data-conversation-id="' + result.newConversation.newID + '"] .session-title', { hasText: "E2E 重命名会话" })
            .count()
            .then((count) => count > 0)
            .catch(() => false);
        }
        // 归档 + 还原
        await newRow.hover().catch(() => {});
        await page.click('.session-list .session-row[data-conversation-id="' + result.newConversation.newID + '"] [data-action="archive"]').catch(() => {});
        await page.waitForTimeout(400);
        result.archive.hiddenFromList = await page
          .locator('.session-list .session-row[data-conversation-id="' + result.newConversation.newID + '"]')
          .count()
          .then((count) => count === 0)
          .catch(() => false);
        const archivedToggle = page.locator(".session-archived-toggle");
        result.archive.toggleAppeared = await archivedToggle
          .waitFor({ state: "visible", timeout: 4000 })
          .then(() => archivedToggle.textContent().then((text) => (text || "").indexOf("已归档") >= 0))
          .catch(() => false);
        if (result.archive.toggleAppeared) {
          await archivedToggle.click().catch(() => {});
          await page.waitForTimeout(300);
          const restoreButton = page.locator('.session-archived [data-action="restore"]').first();
          await restoreButton.click().catch(() => {});
          await page.waitForTimeout(400);
          result.archive.restored = await page
            .locator('.session-list .session-row[data-conversation-id="' + result.newConversation.newID + '"]')
            .count()
            .then((count) => count > 0)
            .catch(() => false);
        }

        // 切换回原会话
        await page.click('.session-list .session-row[data-conversation-id="' + conversationId + '"] .session-row-main').catch(() => {});
        await page.waitForTimeout(700);
        result.switchBack.activeRestored = await page
          .locator('.session-list .session-row[data-conversation-id="' + conversationId + '"].active')
          .count()
          .then((count) => count > 0)
          .catch(() => false);
      }

      // 折叠 toggle
      await page.click('.session-sidebar [data-action="collapse"]').catch(() => {});
      await page.waitForTimeout(300);
      result.collapse.collapsedClass = await page.locator(".session-sidebar.collapsed").count().then((count) => count > 0).catch(() => false);
      await page.screenshot({ path: join(outDir, "dom-" + name + "-collapsed.png"), fullPage: false });
      if (result.collapse.collapsedClass) {
        await page.click('.session-sidebar [data-action="expand"]').catch(() => {});
        await page.waitForTimeout(300);
        result.collapse.expandedBack = await page.locator(".session-sidebar:not(.collapsed)").count().then((count) => count > 0).catch(() => false);
      }
      await page.screenshot({ path: join(outDir, "dom-" + name + "-ops.png"), fullPage: false });
    }

    // IA2：简易视窗三件套（含同族 mixer-strips/死码 V1/旧模式轨）不再渲染
    result.deadNodes = await page.evaluate(() => {
      const probes = [
        ".daw-panel-body", ".daw-focus-panel", ".daw-timeline", ".daw-toolbar", ".daw-panel-header",
        ".rack-lanes", ".midi-workspace", ".midi-editor-preview", ".mixer-strips",
        ".side-rail", ".rail-button", ".focus-summary"
      ];
      const out = {};
      for (const selector of probes) {
        out[selector] = document.querySelectorAll(selector).length;
      }
      return out;
    });

    await page.screenshot({ path: join(outDir, "dom-" + name + ".png"), fullPage: false });
    await context.close();
    return result;
  };

  const iaRedesignPass = await runIaRedesignPass("ia-redesign");
  report.ia_redesign = {
    sidebar_appeared: iaRedesignPass.appeared,
    new_conversation_id: iaRedesignPass.newConversation.newID,
    numbers: iaRedesignPass.numbersLinear,
    registry_keys: iaRedesignPass.registryKeys.filter((key) => key.indexOf("ask_vit_session_flow") === 0),
    plan_bar_head_text: iaRedesignPass.planBarHeadText,
    plan_bar_data_task_id: iaRedesignPass.planBarDataTaskID,
    dead_nodes: iaRedesignPass.deadNodes
  };
  record("ia-redesign-sidebar-IA1", checkIaSidebar(iaRedesignPass, { conversationId }));
  record("ia-redesign-viewport-IA2", checkIaViewport(iaRedesignPass));
  record("ia-redesign-statusbar-IA3", checkIaStatusbar(iaRedesignPass, { expectTaskID: "task_e2e_planbar1" }));

  // WEBUI-INIT-LAYOUT-1: a fresh context sampled on the FIRST mounted frame --
  // no interaction, no waiting for conversation traffic. This is the frame the
  // user's screenshot caught squeezed.
  const runInitialLayoutPass = async (name) => {
    const context = await browser.newContext({ viewport });
    await installReplay(context);
    const page = await context.newPage();
    await page.goto(agentBase + "/app/?conversation_id=" + encodeURIComponent(conversationId), { waitUntil: "domcontentloaded" });
    await page.waitForSelector(".app-shell .composer", { timeout: settleMs });
    const sample = await page.evaluate(INITIAL_LAYOUT_PROBE);
    await page.screenshot({ path: join(outDir, "dom-" + name + ".png"), fullPage: false });
    writeFileSync(join(outDir, "dom-" + name + ".json"), JSON.stringify(sample, null, 2), "utf-8");
    await context.close();
    return sample;
  };
  const initialLayoutSample = await runInitialLayoutPass("initial-layout");
  report.initial_layout = initialLayoutSample;
  record("initial-layout-L1", checkInitialLayout(initialLayoutSample));

  // ------------------------------------------------ WEBUI-SESSION-SEMANTICS-1 (2026-10-04)
  // 主对话流语义组：webui 流是主、note 流是副（用户裁定 2026-10-03）。启动（scope
  // 物化）即自动建主会话并命名「主对话流 · <工程名>」（出生即命名、用户改名最高且
  // 持久、同工程重开沿旧名）；输入框上方的 PlanBar 执行轨迹按 active session 绑定——
  // 全局 task_trajectory 投影的 conversation_id 与当前会话不符时即时清空，切回属主
  // 会话即重绑。场景即手测三号场症状 6：note 会话任务在跑、webui 新建对话流，旧轨迹
  // 不得复活。
  // 工程身份自校准：note 行与期望名都用真 agent /agent/ui/state 的 project_history
  // （historyScopePartsFromUIState 同源字段），隔离 draft 工程与 fixture 路径不同也
  // 成立。note_sessions 在网络层注入（PLANBAR-1 模式），其余全真。
  const sessionSemanticsNoteRowBase = {
    conversation_id: "note_r_e2e_session_sem",
    note_id: "note_e2e_ss",
    title: "便签 N3 · 时间线 55% · 机架 30%",
    archived: false,
    updated_at: "2026-10-04T10:00:00Z"
  };
  const runSessionSemanticsPass = async (name) => {
    const context = await browser.newContext({ viewport });
    await installReplay(context, { strictConversation: true });
    // 动态 status 面：note 行在页面加载前按真 agent 的 scope 身份填充
    const noteRowsHolder = { rows: [] };
    await context.route("**/agent/runtime/status*", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json; charset=utf-8",
        body: JSON.stringify({
          status: "ok",
          service: "VitAgent",
          kernel: { connected: true, status: "ok" },
          goal: { status: "running" },
          task_trajectory: planBarTaskFixture("observation_in_progress"),
          note_sessions: noteRowsHolder.rows
        })
      });
    });
    await context.route("**/agent/ui/state*", async (route) => {
      const response = await route.fetch();
      const body = await response.json().catch(() => ({}));
      await route.fulfill({ response, json: { ...body, goal: { ...(body.goal || {}), status: "running" } } });
    });
    // 自校准：真 agent 的 project_history 就是 webui scope 的身份源
    let scopeProjectPath = "";
    let scopeProjectUUID = "";
    for (let attempt = 0; attempt < 10 && !scopeProjectPath; attempt += 1) {
      const uiState = await getJSON("/agent/ui/state").catch(() => null);
      const projectHistory = (uiState && uiState.project_history) || {};
      scopeProjectPath = String(projectHistory.project_path || projectHistory.current_project_path || "").trim();
      scopeProjectUUID = String(projectHistory.project_uuid || "").trim();
      if (!scopeProjectPath) {
        await new Promise((resolve) => setTimeout(resolve, 1000));
      }
    }
    if (scopeProjectPath) {
      noteRowsHolder.rows = [{ ...sessionSemanticsNoteRowBase, project_path: scopeProjectPath, project_uuid: scopeProjectUUID }];
    }
    const page = await context.newPage();
    await page.goto(agentBase + "/app/?conversation_id=" + encodeURIComponent(conversationId), { waitUntil: "domcontentloaded" });
    await page.waitForTimeout(2500);
    const result = {
      sidebarAppeared: false,
      scopeProjectPath: scopeProjectPath,
      scopeCalibrated: Boolean(scopeProjectPath),
      statusTitles: [],
      mainRow: { found: false, title: "", active: false, conversationId: "" },
      mainIDKeyPersisted: false,
      planBarAtBoot: false,
      noteRowPresent: false,
      planBarClearedOnNoteSwitch: false,
      planBarStaysClearAfterNewFlow: false,
      newFlowRowTitle: "",
      planBarReboundOnMain: false,
      renamedTitle: "",
      renamePersistedAfterReload: false,
      planBarReboundAfterReload: false
    };
    result.sidebarAppeared = await page.waitForSelector(".session-sidebar", { timeout: 10000 }).then(() => true).catch(() => false);
    if (result.sidebarAppeared) {
      result.statusTitles = await page.evaluate(() =>
        Array.from(document.querySelectorAll(".top-status span[title]"))
          .map((el) => el.getAttribute("title") || "")
          .filter((title) => title.indexOf(" · ") >= 0)
      );
      // 期望名与 App 同源推导：工程名 = scope 工程路径尾段（lastPathPart 语义）
      const projectName = scopeProjectPath.replace(/\\/g, "/").split("/").filter(Boolean).pop() || "";
      const expectedMainTitle = projectName ? "主对话流 · " + projectName : "主对话流";
      result.mainRow = await page.evaluate((expected) => {
        const rows = Array.from(document.querySelectorAll(".session-list .session-row"));
        const match = rows.find((row) => {
          const title = row.querySelector(".session-title");
          return title && (title.textContent || "").trim() === expected;
        });
        return {
          found: Boolean(match),
          title: match ? (match.querySelector(".session-title") || {}).textContent || "" : "",
          active: Boolean(match && match.classList.contains("active")),
          conversationId: match ? match.getAttribute("data-conversation-id") || "" : ""
        };
      }, expectedMainTitle);
      result.mainIDKeyPersisted = await page
        .evaluate(() => Object.keys(window.localStorage).some((key) => key.indexOf("ask_vit_session_main.v1:") === 0))
        .catch(() => false);
      await page.screenshot({ path: join(outDir, "dom-" + name + "-mount.png"), fullPage: false });

      // PlanBar 随属主会话在启动即绑定（fixture task 归属本会话）
      result.planBarAtBoot = await page.waitForSelector(".plan-bar", { timeout: 8000 }).then(() => true).catch(() => false);

      // note 行经服务端提示入列（NOTESTREAM-2 既有语义，SS 组的跳板）
      result.noteRowPresent = await page
        .locator('.session-list .session-row[data-conversation-id="' + sessionSemanticsNoteRowBase.conversation_id + '"]')
        .count()
        .then((count) => count > 0)
        .catch(() => false);

      if (result.planBarAtBoot && result.noteRowPresent) {
        // 症状 6 场景 ①：切到 note 会话——异会话任务投影不得留在输入框上方
        await page.click('.session-list .session-row[data-conversation-id="' + sessionSemanticsNoteRowBase.conversation_id + '"] .session-row-main').catch(() => {});
        result.planBarClearedOnNoteSwitch = await page
          .waitForSelector(".plan-bar", { state: "detached", timeout: 10000 })
          .then(() => true)
          .catch(() => false);
        // 症状 6 场景 ②：webui 新建对话流——旧轨迹不得复活
        await page.click('.session-sidebar [data-action="new"]').catch(() => {});
        await page.waitForTimeout(1200);
        result.planBarStaysClearAfterNewFlow = await page
          .locator(".plan-bar")
          .count()
          .then((count) => count === 0)
          .catch(() => false);
        result.newFlowRowTitle = await page
          .locator(".session-list .session-row.active .session-title")
          .first()
          .textContent()
          .then((text) => (text || "").trim())
          .catch(() => "");
        // 切回主流（属主）→ PlanBar 重绑
        await page.click('.session-list .session-row[data-conversation-id="' + conversationId + '"] .session-row-main').catch(() => {});
        result.planBarReboundOnMain = await page.waitForSelector(".plan-bar", { timeout: 10000 }).then(() => true).catch(() => false);

        // 用户改名最高且持久：改主对话流名后 reload，名字仍在
        const mainRow = page.locator('.session-list .session-row[data-conversation-id="' + conversationId + '"]');
        await mainRow.hover().catch(() => {});
        await mainRow.locator('[data-action="rename"]').click().catch(() => {});
        const renameInput = page.locator(".session-rename-input");
        const renameInputAppeared = await renameInput.waitFor({ state: "visible", timeout: 4000 }).then(() => true).catch(() => false);
        if (renameInputAppeared) {
          await renameInput.fill("我的主流 E2E");
          await renameInput.press("Enter");
          await page.waitForTimeout(500);
          await page.reload({ waitUntil: "domcontentloaded" });
          await page.waitForTimeout(2500);
          result.renamedTitle = "我的主流 E2E";
          result.renamePersistedAfterReload = await page
            .locator('.session-list .session-row[data-conversation-id="' + conversationId + '"] .session-title', { hasText: "我的主流 E2E" })
            .count()
            .then((count) => count > 0)
            .catch(() => false);
          result.planBarReboundAfterReload = await page.waitForSelector(".plan-bar", { timeout: 10000 }).then(() => true).catch(() => false);
        }
      }
      await page.screenshot({ path: join(outDir, "dom-" + name + "-ops.png"), fullPage: false });
    }
    await context.close();
    return result;
  };

  const checkSessionSemanticsNaming = (result) => {
    const failures = [];
    const notes = [];
    if (!result.sidebarAppeared) {
      failures.push("the session-flow sidebar never rendered (.session-sidebar)");
      return { failures, notes };
    }
    if (!result.mainRow.found) {
      failures.push(
        "no sidebar row is named 「主对话流 · <工程名>」 at boot (statusTitles=" + JSON.stringify(result.statusTitles) + ")"
      );
      return { failures, notes };
    }
    notes.push("main flow row: " + JSON.stringify(result.mainRow));
    if (!result.mainRow.active) {
      failures.push("the auto-created main conversation row is not the active row at boot");
    }
    if (result.mainRow.conversationId !== conversationId) {
      failures.push("the main-flow row is not the mounted conversation (" + result.mainRow.conversationId + " vs " + conversationId + ")");
    }
    if (!result.mainIDKeyPersisted) {
      failures.push("no ask_vit_session_main.v1:* identity key persisted to localStorage (main conversation not pinned)");
    }
    return { failures, notes };
  };

  const checkSessionSemanticsTrajectoryClear = (result) => {
    const failures = [];
    const notes = [];
    if (!result.planBarAtBoot) {
      failures.push("the plan bar never rendered at boot for its owning conversation -- the binding assertion would measure nothing");
      return { failures, notes };
    }
    if (!result.noteRowPresent) {
      failures.push("the injected note session row never appeared in the sidebar (note_sessions projection broken)");
      return { failures, notes };
    }
    if (!result.planBarClearedOnNoteSwitch) {
      failures.push("switching to the note conversation did not clear the plan bar (stale trajectory stays above the input box -- manual-test symptom 6)");
    } else {
      notes.push("plan bar cleared on switch to the note conversation");
    }
    if (!result.planBarStaysClearAfterNewFlow) {
      failures.push("creating a new webui conversation let the stale trajectory re-render (global projection leaked into the new flow)");
    } else {
      notes.push("plan bar stays clear in the new flow");
    }
    if (result.newFlowRowTitle === "未命名会话") {
      notes.push("new user flow stays unnamed (平级列出, not auto-named as main)");
    } else if (result.newFlowRowTitle.indexOf("主对话流") === 0) {
      failures.push("the user-created new flow was mis-named as the main conversation flow: " + JSON.stringify(result.newFlowRowTitle));
    }
    return { failures, notes };
  };

  const checkSessionSemanticsRebind = (result) => {
    const failures = [];
    const notes = [];
    if (!result.planBarReboundOnMain) {
      failures.push("switching back to the main conversation did not rebind the plan bar (trajectory lost after session switch)");
    } else {
      notes.push("plan bar rebound on switch back to the owning conversation");
    }
    if (!result.renamePersistedAfterReload) {
      failures.push("the user rename of the main flow did not survive a page reload (rename must be highest and persistent)");
    } else {
      notes.push("user rename persisted across reload: " + JSON.stringify(result.renamedTitle));
    }
    if (!result.planBarReboundAfterReload) {
      failures.push("the plan bar did not re-render after reload on the owning conversation");
    }
    return { failures, notes };
  };

  const sessionSemanticsPass = await runSessionSemanticsPass("session-semantics");
  report.session_semantics = {
    scope_calibrated: sessionSemanticsPass.scopeCalibrated,
    scope_project_path: sessionSemanticsPass.scopeProjectPath,
    sidebar_appeared: sessionSemanticsPass.sidebarAppeared,
    main_row: sessionSemanticsPass.mainRow,
    main_id_key_persisted: sessionSemanticsPass.mainIDKeyPersisted,
    note_row_present: sessionSemanticsPass.noteRowPresent,
    plan_bar_at_boot: sessionSemanticsPass.planBarAtBoot,
    cleared_on_note_switch: sessionSemanticsPass.planBarClearedOnNoteSwitch,
    stays_clear_after_new_flow: sessionSemanticsPass.planBarStaysClearAfterNewFlow,
    new_flow_row_title: sessionSemanticsPass.newFlowRowTitle,
    rebound_on_main: sessionSemanticsPass.planBarReboundOnMain,
    rename_persisted_after_reload: sessionSemanticsPass.renamePersistedAfterReload,
    plan_bar_rebound_after_reload: sessionSemanticsPass.planBarReboundAfterReload
  };
  record("session-semantics-SS1", checkSessionSemanticsNaming(sessionSemanticsPass));
  record("session-semantics-SS2", checkSessionSemanticsTrajectoryClear(sessionSemanticsPass));
  record("session-semantics-SS3", checkSessionSemanticsRebind(sessionSemanticsPass));



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
export { DOM_PROBE, checkA1, checkA2, checkA3, checkB1, checkB2, checkC1, checkD1, checkE1, checkF1, checkG1, checkT1, checkK1, checkK2, checkO1, checkM1, checkM2, observeRowProbe, observeLongReplyFixture, observeShortReplyFixture, residencyFixtureEvents, chatOnlyItemStepsFixtureEvents, terminalTurnFixtureEvents, auditionFixtureEvents, auditionUnstickFixtureEvents, auditionTrailFixtureEvents, confirmInteractionFixture, confirmationChatResponseFixture, directExecutionReceiptFixture };

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

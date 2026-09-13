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
      duration: (step.querySelector(".trace-dur")?.textContent || "").trim()
    }));
    const label = (el.querySelector(".th-label")?.textContent || "").trim();
    const meta = (el.querySelector(".th-meta")?.textContent || "").replace(/\s+/g, " ").trim();
    return {
      turnId: el.getAttribute("data-turn-id") || "",
      cls: typeof el.className === "string" ? el.className : "",
      label,
      meta,
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
    })()
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
  const installReplay = async (context) => {
    await context.route("**/agent/events*", async (route) => {
      seededEventsRequests += 1;
      // Serve the archived events with the same since/limit contract the agent
      // implements, so the app's incremental polling works exactly as it does
      // against the real endpoint.
      const requestURL = new URL(route.request().url());
      const since = Number(requestURL.searchParams.get("since") || "0");
      const limit = Number(requestURL.searchParams.get("limit") || "120");
      const all = eventsFixture.events || [];
      const body = JSON.stringify({
        status: eventsFixture.status || "ok",
        events: all.filter((event) => Number(event.seq) > since).slice(0, limit),
        next_seq: eventsFixture.next_seq ?? all.length
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
export { DOM_PROBE, checkA1, checkA2, checkA3, checkB1, checkB2 };

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

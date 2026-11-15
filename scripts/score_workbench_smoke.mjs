/*
 * SCORE-WORKBENCH-1 (374) M5 -- DG1-A berth leg (docs/STACKGATE_DG1_DG3_V1.md section 3.1).
 *
 * Shape: the dev_agent_smoke.ps1 berth (kernel + agent, no UI) plus a real
 * Chromium driving the REAL webui score panel at /app/. Every mutation goes
 * through the production seams: browser -> /agent/invoke -> harness -> kernel
 * ZMQ. No test backdoors; the panel is driven by real pointer events whose
 * coordinates come from the data-score-tick-rows anchors the renderer exposes.
 *
 * Assertions (STACKGATE section 3.1 mapping):
 *   A1 write/read equality   -- UI edit -> apply -> kernel readback set equality
 *   A2 mapping stability     -- DOM model note ids == kernel ids across edit+save+reopen
 *   A3 symbol coverage       -- dotted / syncopation / rest / cross-bar tie round-trip
 *   A4 dual time signature   -- project 4/4 disclosed and not rewritten (2/4 = card 375 boundary)
 *   A5 save = full overwrite -- saved face state only after verified readback
 *   A6 reopen round-trip     -- save -> open_project -> reload -> identical note set
 *   A7/A8 real sound + diff  -- SKIPPED-pending-373 (no instrument on a bare MIDI
 *                               track can never render: VitProductionCoordinator
 *                               refuses offline render; sound source = SRC-TEMPLATE-1)
 *
 * Exit codes: 0 = PASS (A1-A6 green, A7/A8 recorded as boundary); 1 = assertion
 * failure (face_defect -- no rerun); 2 = environment interrupt (agent/kernel/
 * browser start failures -- recorded with raw evidence).
 *
 * Zero LLM: all interactions are deterministic UI + kernel commands; the agent
 * runs with the smoke stack's dead-endpoint LLM override where applicable.
 */

import { mkdirSync, readFileSync, writeFileSync, existsSync } from "node:fs";
import { join } from "node:path";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);

function parseArgs() {
  const args = {
    agentHttp: process.env.SCORE_AGENT_HTTP || "http://127.0.0.1:7878",
    artifactsDir: process.env.SCORE_ARTIFACTS_DIR || "",
    screenshotDir: "",
    timeoutMs: 60000
  };
  const argv = process.argv.slice(2);
  for (let i = 0; i < argv.length; i++) {
    if (argv[i] === "--agent-http") args.agentHttp = argv[++i];
    else if (argv[i] === "--artifacts-dir") args.artifactsDir = argv[++i];
    else if (argv[i] === "--timeout-ms") args.timeoutMs = Number(argv[++i]);
  }
  if (!args.artifactsDir) {
    args.artifactsDir = join(process.cwd(), "score_workbench_run_" + new Date().toISOString().replace(/[-:T]/g, "").slice(0, 14));
  }
  mkdirSync(args.artifactsDir, { recursive: true });
  args.screenshotDir = join(args.artifactsDir, "screens");
  mkdirSync(args.screenshotDir, { recursive: true });
  return args;
}

const args = parseArgs();
const summary = {
  schema_version: "score_workbench_smoke.v1",
  card: "SCORE-WORKBENCH-1",
  gate: "DG1-A (docs/STACKGATE_DG1_DG3_V1.md section 3.1)",
  leg_form: "berth (kernel + agent, real browser on /app/)",
  started_at: new Date().toISOString(),
  agent_http: args.agentHttp,
  assertions: {},
  boundary: [],
  failure_class: "",
  notes: []
};

function log(...parts) {
  console.log("[score-wb]", ...parts);
}

async function fail(exitCode, phase, reason, extra) {
  summary.exit_code = exitCode;
  summary.failure_class = exitCode === 2 ? "env_interrupt" : "assertion_failure";
  summary.fail_phase = phase;
  summary.fail_reason = reason;
  if (extra) Object.assign(summary, extra);
  writeSummary();
  console.error("[score-wb] FAIL exit=" + exitCode + " phase=" + phase + ": " + reason);
  process.exit(exitCode);
}

function writeSummary() {
  summary.finished_at = new Date().toISOString();
  writeFileSync(join(args.artifactsDir, "score_workbench_summary.json"), JSON.stringify(summary, null, 2), "utf-8");
}

async function getJSON(path) {
  const response = await fetch(args.agentHttp + path);
  const text = await response.text();
  if (!response.ok) throw new Error("GET " + path + " -> " + response.status + " " + text.slice(0, 200));
  return JSON.parse(text);
}

async function invoke(tool, argsIn, confirmed) {
  const response = await fetch(args.agentHttp + "/agent/invoke", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ tool, args: argsIn, confirmed: confirmed === true, source: "score_workbench_smoke" })
  });
  const payload = await response.json().catch(() => ({ status: "error", error: "non-json reply " + response.status }));
  if (payload.status !== "ok") {
    throw new Error("invoke " + tool + " failed: " + (payload.error || payload.status));
  }
  return payload.result || {};
}

function assert(id, condition, detail) {
  summary.assertions[id] = { pass: condition === true, detail: condition === true ? "" : String(detail) };
  log(id, condition ? "PASS" : "FAIL" + " " + (condition ? "" : detail));
  return condition === true;
}

// ---- fixture: 四类节奏教材内容（附点/切分/休止〔空档〕/跨小节线延音），4/4 工程 ----
const FIXTURE_NOTES = [
  { pitch: 72, start: 0, length: 0.75 }, // dotted eighth (前长后短 首)
  { pitch: 67, start: 0.75, length: 0.25 }, // sixteenth (前长后短 尾)
  // 1.0-1.5 rest window (absence)
  { pitch: 71, start: 1.5, length: 0.5 }, // eighth after rest
  { pitch: 76, start: 2.5, length: 1.0 }, // syncopation (weak-beat cross-beat -> tie pair)
  { pitch: 79, start: 3.5, length: 1.0 } // cross-barline tie into measure 2
];

function noteSetOf(rows) {
  return rows
    .map((note) => [note.pitch, Math.round(note.startBeats * 4), Math.round(note.lengthBeats * 4)])
    .sort((a, b) => a[1] - b[1] || a[0] - b[0]);
}

async function readKernelNotes(clipID, trackID) {
  const result = await invoke("get_midi_clip_notes", { clip_id: clipID, track_id: trackID });
  const rows = Array.isArray(result.notes) ? result.notes : [];
  return rows.map((row) => ({
    id: String(row.id),
    pitch: Number(row.pitch),
    startBeats: Number(row.start),
    lengthBeats: Number(row.length)
  }));
}

function resolvePlaywright() {
  const candidates = [
    join(args.artifactsDir, "..", "..", "agent", "webui", "node_modules", "playwright", "index.mjs"),
    join(process.cwd(), "agent", "webui", "node_modules", "playwright", "index.mjs"),
    join(process.cwd(), "node_modules", "playwright", "index.mjs")
  ];
  for (const candidate of candidates) {
    if (existsSync(candidate)) {
      return { path: candidate, module: require(candidate) };
    }
  }
  const fromRepo = require.resolve("playwright", { paths: [join(process.cwd(), "agent", "webui")] });
  return { path: fromRepo, module: require(fromRepo) };
}

async function main() {
  writeSummary();
  const started = Date.now();

  // ---- P0 environment + fixture (env failures -> exit 2) ----
  let health;
  try {
    health = await getJSON("/health");
    log("agent health:", JSON.stringify(health).slice(0, 120));
  } catch (error) {
    return fail(2, "env-agent", "agent HTTP not reachable: " + error.message);
  }

  let trackID = "";
  let clipID = "";
  try {
    const trackReply = await invoke("add_track", {}, true);
    trackID = String(trackReply.track_id || trackReply.id || "");
    if (!trackID) throw new Error("add_track returned no track_id: " + JSON.stringify(trackReply).slice(0, 200));
    const clipReply = await invoke("insert_midi_clip", { track_id: trackID, time_unit: "beats", start: 0, length: 8 }, true);
    clipID = String(clipReply.clip_id || clipReply.new_clip_id || "");
    if (!clipID) throw new Error("insert_midi_clip returned no clip_id: " + JSON.stringify(clipReply).slice(0, 200));
    await invoke("add_midi_notes", { track_id: trackID, clip_id: clipID, time_unit: "beats", notes: FIXTURE_NOTES }, true);
  } catch (error) {
    return fail(2, "env-fixture", "fixture via agent tool face failed: " + error.message);
  }

  // music context disclosure (A4 anchor: runtime truth, not assumption)
  const analysis = await invoke("analyze_midi_transformations", { clip_id: clipID, track_id: trackID }).catch((error) => {
    return fail(2, "env-context", "analyze_midi_transformations failed: " + error.message);
  });
  const echo = analysis.music_context_echo || {};
  const timesig = (echo.time_signature_numerator || 0) + "/" + (echo.time_signature_denominator || 0);
  log("music_context_echo:", JSON.stringify(echo).slice(0, 200));

  // ---- browser ----
  let chromium;
  try {
    const resolved = resolvePlaywright();
    chromium = resolved.module.chromium || resolved.module.default.chromium;
    log("playwright:", resolved.path);
  } catch (error) {
    return fail(2, "env-playwright", "playwright resolution failed: " + error.message);
  }
  let browser;
  try {
    browser = await chromium.launch({ headless: true });
  } catch (error) {
    return fail(2, "env-browser", "chromium launch failed: " + error.message);
  }

  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await context.newPage();

  const openScorePanel = async (phaseName) => {
    await page.goto(args.agentHttp + "/app/", { waitUntil: "domcontentloaded" });
    await page.waitForTimeout(2500);
    const tab = page.locator('[data-workbench-tab="score"]');
    if ((await tab.count()) !== 1) throw new Error("score tab missing on " + phaseName);
    await tab.click();
    await page.waitForSelector("[data-score-root]", { timeout: 15000 });
    await page.waitForTimeout(1200); // mount auto-load
  };
  const domModel = async () => JSON.parse(await page.getAttribute("[data-score-root]", "data-score-model"));
  const rootAttr = async (name) => (await page.getAttribute("[data-score-root]", name)) || "";
  const parseRows = (raw) => {
    const rows = new Map();
    for (const part of String(raw).split(";")) {
      const m = /^m([0-9]+):(.*)$/.exec(part);
      if (!m) continue;
      rows.set(Number(m[1]), m[2].split(",").map((pair) => pair.split("@").map(Number)));
    }
    return rows;
  };
  const clickSymbol = async (tick) => {
    const rows = parseRows(await page.getAttribute("[data-score-svg]", "data-score-tick-rows"));
    const measure = Math.floor(tick / 16); // 4/4 = 16 ticks per measure
    const row = rows.get(measure) || rows.get(0);
    const point = (row || []).find(([t]) => t === tick);
    if (!point) throw new Error("no tick anchor for " + tick + " (rows=" + rows.size + ")");
    const box = await page.locator("[data-score-overlay]").boundingBox();
    if (!box) throw new Error("overlay box missing");
    await page.mouse.click(box.x + point[1] + 1, box.y + 4 + 20);
    await page.waitForTimeout(450);
  };

  try {
    // ---- P1 panel auto-load: A1 precondition + A4 ----
    await openScorePanel("P1");
    await page.screenshot({ path: join(args.screenshotDir, "p1_loaded.png") });
    const timesigDom = await rootAttr("data-score-timesig");
    assert("A4-timesig-disclosed", timesigDom === timesig && timesig === "4/4",
      "DOM timesig='" + timesigDom + "' vs kernel '" + timesig + "' (2/4 project semantics = card 375 boundary)");

    const kernelNotes0 = await readKernelNotes(clipID, trackID);
    const model1 = await domModel();
    const sameLoad = JSON.stringify(noteSetOf(model1.notes)) === JSON.stringify(noteSetOf(kernelNotes0));
    assert("A1-load-equality", sameLoad && kernelNotes0.length === FIXTURE_NOTES.length,
      "DOM " + JSON.stringify(noteSetOf(model1.notes)) + " vs kernel " + JSON.stringify(noteSetOf(kernelNotes0)));
    assert("A3-fixture-loaded", model1.notes.length === FIXTURE_NOTES.length, "notes=" + model1.notes.length);

    // A3 symbol coverage -- derived notation invariants via DOM model + svg glyphs:
    // dotted (0.75), rest window (no note covering tick 4), syncopation (start 2.5 len 1.0),
    // cross-bar tie (start 3.5 len 1.0 -> ticks 14..18 crossing tick 16).
    const svgText = await page.locator("[data-score-svg]").innerHTML();
    const noteheads = (svgText.match(/notehead/g) || []).length;
    const hasTieGlyph = svgText.includes("tie");
    assert("A3-svg-glyphs", noteheads >= model1.notes.length + 2 && hasTieGlyph,
      "noteheads=" + noteheads + " notes=" + model1.notes.length + " tieGlyph=" + hasTieGlyph);
    assert("A3-dotted", model1.notes.some((n) => n.lengthBeats === 0.75 && n.startBeats === 0), "no dotted note");
    assert("A3-rest-window", !model1.notes.some((n) => n.startBeats < 1.5 && n.startBeats + n.lengthBeats > 1.0 && n.startBeats > 0.25), "rest window 1.0-1.5 covered");
    assert("A3-syncopation", model1.notes.some((n) => n.startBeats === 2.5 && n.lengthBeats === 1.0), "no syncopation note");
    assert("A3-cross-bar-tie", model1.notes.some((n) => n.startBeats === 3.5 && n.lengthBeats === 1.0), "no cross-barline tie note");

    // A2 step 1: DOM ids == kernel ids (mapping rebinding)
    const domIDs1 = model1.notes.map((n) => n.id).sort();
    const kernelIDs1 = kernelNotes0.map((n) => n.id).sort();
    assert("A2-id-binding", JSON.stringify(domIDs1) === JSON.stringify(kernelIDs1),
      "DOM ids " + JSON.stringify(domIDs1) + " vs kernel " + JSON.stringify(kernelIDs1));

    // ---- P2 UI edit -> apply: A1 write/read + A5 saved-after-verify ----
    await clickSymbol(4); // place C5 eighth in the rest window (beat 1.0)
    const dirtyNow = await rootAttr("data-score-dirty");
    if (dirtyNow !== "true") return fail(1, "P2-edit", "edit through real pointer did not mark dirty");
    await page.screenshot({ path: join(args.screenshotDir, "p2_edited.png") });
    await page.click('[data-score-apply]');
    await page.waitForTimeout(300);
    await page.click("[data-score-confirm-apply]");
    await page.waitForTimeout(2500); // confirmed write + readback verify + rebase
    await page.screenshot({ path: join(args.screenshotDir, "p3_applied.png") });

    const applyState = await rootAttr("data-score-apply-state");
    const kernelNotes1 = await readKernelNotes(clipID, trackID);
    const expectedAfterEdit = [...FIXTURE_NOTES, { pitch: 72, start: 1.0, length: 0.5 }].map((n) => [n.pitch, Math.round(n.start * 4), Math.round(n.length * 4)]).sort((a, b) => a[1] - b[1] || a[0] - b[0]);
    assert("A1-write-readback", JSON.stringify(noteSetOf(kernelNotes1)) === JSON.stringify(expectedAfterEdit),
      "kernel " + JSON.stringify(noteSetOf(kernelNotes1)) + " vs expected " + JSON.stringify(expectedAfterEdit));
    assert("A5-saved-only-after-verify", applyState === "saved" && (await rootAttr("data-score-dirty")) === "false",
      "applyState=" + applyState + " (bridge already verified readback before surfacing saved)");

    // A2 step 2: after apply the DOM model is the kernel readback (same ids)
    const model2 = await domModel();
    const kernelIDs2 = kernelNotes1.map((n) => n.id).sort();
    assert("A2-id-stable-edit-apply", JSON.stringify(model2.notes.map((n) => n.id).sort()) === JSON.stringify(kernelIDs2),
      "ids drifted after apply");

    // ---- P4 A6 save -> reopen -> reload round-trip ----
    const stateResult = await invoke("get_project_state", {});
    const projectObj = stateResult.project && typeof stateResult.project === "object" ? stateResult.project : {};
    const projectPath = String(
      projectObj.file_path || projectObj.path || stateResult.project_path || stateResult.current_project_path || ""
    );
    await invoke("save_project", {}, true);
    const savedNotes = await readKernelNotes(clipID, trackID);
    const savedSet = noteSetOf(savedNotes);
    const savedIDs = savedNotes.map((n) => n.id).sort();
    if (!projectPath) {
      summary.notes.push("get_project_state did not disclose project.file_path; reopen leg uses prior save_as target if any");
    }
    if (projectPath) {
      await invoke("open_project", { file_path: projectPath }, true);
      await page.waitForTimeout(1500);
      const kernelNotes2 = await readKernelNotes(clipID, trackID);
      assert("A6-reopen-readback", JSON.stringify(noteSetOf(kernelNotes2)) === JSON.stringify(savedSet),
        "reopened " + JSON.stringify(noteSetOf(kernelNotes2)) + " vs saved " + JSON.stringify(savedSet));
      assert("A2-id-stable-reopen", JSON.stringify(kernelNotes2.map((n) => n.id).sort()) === JSON.stringify(savedIDs),
        "kernel note ids drifted across reopen");
      // Browser reload: the panel must re-attach and show the reopened state.
      await openScorePanel("P4-reopen");
      await page.screenshot({ path: join(args.screenshotDir, "p4_reopened.png") });
      const model3 = await domModel();
      assert("A6-panel-reload", JSON.stringify(noteSetOf(model3.notes)) === JSON.stringify(savedSet),
        "panel after reopen " + JSON.stringify(noteSetOf(model3.notes)) + " vs saved " + JSON.stringify(savedSet));
    } else {
      // No disclosed path: the honest boundary is recorded, the gate assertion
      // stays red -- the leg must not silently pass A6.
      assert("A6-reopen-readback", false, "project file_path not disclosed; reopen round-trip not provable on this stack");
    }

    // ---- A7/A8 boundary (fail-honest skip with evidence anchor) ----
    summary.boundary.push({
      id: "A7",
      status: "skipped-pending-373",
      reason: "offline render probe needs an enabled instrument on the MIDI track; a bare MIDI track can never pass node creation (VitProductionCoordinator refuses; sound source line = SRC-TEMPLATE-1). Not downgraded to a graphics-only pass (STACKGATE 3.1)."
    });
    summary.boundary.push({
      id: "A8",
      status: "skipped-pending-373",
      reason: "same instrument dependency as A7 (edit-diff render probe)."
    });

    const failed = Object.values(summary.assertions).filter((row) => !row.pass);
    summary.exit_code = failed.length === 0 ? 0 : 1;
    summary.failure_class = failed.length === 0 ? "" : "assertion_failure";
    summary.duration_ms = Date.now() - started;
    summary.clip_id = clipID;
    summary.track_id = trackID;
    writeSummary();
    writeFileSync(join(args.artifactsDir, "kernel_readback_final.json"), JSON.stringify(await readKernelNotes(clipID, trackID), null, 2), "utf-8");
    await browser.close();
    log("verdict:", summary.exit_code === 0 ? "PASS" : "FAIL", "(" + Object.keys(summary.assertions).length + " assertions)");
    process.exit(summary.exit_code);
  } catch (error) {
    try {
      await page.screenshot({ path: join(args.screenshotDir, "fatal.png") });
    } catch { /* best effort */ }
    summary.fatal = String(error && error.stack ? error.stack : error);
    await browser.close().catch(() => {});
    return fail(2, "runtime", "leg crashed: " + summary.fatal.slice(0, 400));
  }
}

main().catch((error) => {
  summary.fatal = String(error && error.stack ? error.stack : error);
  fail(2, "runtime", "unhandled: " + summary.fatal.slice(0, 400));
});

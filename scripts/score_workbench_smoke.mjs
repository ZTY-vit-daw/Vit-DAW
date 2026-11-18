/*
 * SCORE-WORKBENCH-1 (374) M5 -- DG1-A berth leg (docs/STACKGATE_DG1_DG3_V1.md section 3.1).
 * SCORE-DG1A-COMPLETE-1 (377): A7/A8 completed on the SRC-TEMPLATE-1 (373) sound
 * source line -- judgment reused verbatim from the 373 DG1-B driver
 * (scripts/src_template_smoke.py): kernel ZMQ l2_render_probe (track_post_fader,
 * offline_probe) for non-silence + start_render WAV pair for the edit
 * differential. Kernel ZMQ is a production seam (STACKGATE section 3.0); the
 * python+pyzmq transport is spawned in-process (proven on this stack by 373).
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
 *   A4 dual time signature   -- project 4/4 disclosed and not rewritten (2/4 side
 *                               of the dual-signature face is pinned by the 373
 *                               template project exercised in P5)
 *   A5 save = full overwrite -- saved face state only after verified readback
 *   A6 reopen round-trip     -- save -> open_project -> reload -> identical note set
 *   A7 real sound            -- P5: 373 template melody track (Surge XT), edit via
 *                               the agent/kernel tool face (373 B4 verbatim:
 *                               mutate_midi_notes placeholder pitch 60 -> 79 at
 *                               beat 0; the panel gesture is blocked by the D4
 *                               out-of-domain defect, see summary notes), then
 *                               kernel l2_render_probe non-silent: status ready +
 *                               nonzero + max_abs >= PROBE_MAX_ABS_MIN +
 *                               nan_inf_count == 0 (373 B2).
 *   A8 edit differential     -- P5: start_render WAV pair around the same edit;
 *                               both non-silent (peak/RMS 309 contract) and
 *                               max_abs_diff >= WAV_DIFF_MAX_MIN (373 B4).
 *
 * Predeclared thresholds (AGENTS section 8 / STACKGATE 1.6; recorded in summary):
 *   PROBE_MAX_ABS_MIN = 0.01    (float domain; 373 same-state offline render
 *                                measured ~0.23 peak -> -40 dBFS floor)
 *   WAV_NON_SILENT_PEAK = 500   (int16 counts, 309 metering contract)
 *   WAV_NON_SILENT_RMS = 20.0   (int16 counts, 309 metering contract)
 *   WAV_DIFF_MAX_MIN = 500      (int16 counts; one full-amplitude note changed)
 *
 * Exit codes: 0 = PASS (A1-A8 green); 1 = assertion failure (face_defect -- no
 * rerun); 2 = environment interrupt (agent/kernel/browser/python-zmq start
 * failures, Surge healthy-source precondition missing -- recorded with raw
 * evidence).
 *
 * Zero LLM: all interactions are deterministic UI + kernel commands; the agent
 * runs with the smoke stack's dead-endpoint LLM override where applicable.
 */

import { mkdirSync, readFileSync, writeFileSync, existsSync, copyFileSync, statSync } from "node:fs";
import { createHash } from "node:crypto";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const scriptDir = dirname(fileURLToPath(import.meta.url));

function parseArgs() {
  const args = {
    agentHttp: process.env.SCORE_AGENT_HTTP || "http://127.0.0.1:7878",
    artifactsDir: process.env.SCORE_ARTIFACTS_DIR || "",
    screenshotDir: "",
    timeoutMs: 60000,
    projectXml: process.env.SCORE_PROJECT_XML || "",
    kernelZmq: process.env.SCORE_KERNEL_ZMQ || "tcp://127.0.0.1:5555",
    kernelSub: process.env.SCORE_KERNEL_SUB || "tcp://127.0.0.1:5556",
    templateXml: process.env.SCORE_TEMPLATE_XML || join(scriptDir, "..", "coord", "lesson-designs", "music-creation-unit2", "templates", "melody-drum-v1", "template.v1.xml")
  };
  const argv = process.argv.slice(2);
  for (let i = 0; i < argv.length; i++) {
    if (argv[i] === "--agent-http") args.agentHttp = argv[++i];
    else if (argv[i] === "--artifacts-dir") args.artifactsDir = argv[++i];
    else if (argv[i] === "--timeout-ms") args.timeoutMs = Number(argv[++i]);
    else if (argv[i] === "--project-xml") args.projectXml = argv[++i];
    else if (argv[i] === "--kernel-zmq") args.kernelZmq = argv[++i];
    else if (argv[i] === "--kernel-sub") args.kernelSub = argv[++i];
    else if (argv[i] === "--template-xml") args.templateXml = argv[++i];
    else if (argv[i] === "--save-forensics") args.saveForensics = true;
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

// Predeclared thresholds (373 DG1-B verbatim; see header).
const PROBE_MAX_ABS_MIN = 0.01;
const WAV_NON_SILENT_PEAK = 500;
const WAV_NON_SILENT_RMS = 20.0;
const WAV_DIFF_MAX_MIN = 500;
const MELODY_TRACK_NAME = "我的旋律";
const SURGE_NAME = "Surge XT";
const TEMPLATE_CLIP_SECONDS = 6.0; // melody clip 8 beats @ 80 bpm
const TEMPLATE_TAIL_SECONDS = 0.25;
const RENDER_WAIT_SEC = 240;

const summary = {
  schema_version: "score_workbench_smoke.v2-a7a8",
  card: "SCORE-DG1A-COMPLETE-1 (completes SCORE-WORKBENCH-1 DG1-A leg)",
  gate: "DG1-A (docs/STACKGATE_DG1_DG3_V1.md section 3.1)",
  leg_form: "berth (kernel + agent, real browser on /app/)",
  started_at: new Date().toISOString(),
  agent_http: args.agentHttp,
  kernel_zmq: args.kernelZmq,
  kernel_sub: args.kernelSub,
  thresholds: {
    probe_max_abs_min: PROBE_MAX_ABS_MIN,
    wav_non_silent_peak: WAV_NON_SILENT_PEAK,
    wav_non_silent_rms: WAV_NON_SILENT_RMS,
    wav_diff_max_min: WAV_DIFF_MAX_MIN
  },
  assertions: {},
  boundary: [],
  failure_class: "",
  notes: [],
  // KERNEL-SAVE-FORENSICS-1 (378): D1/D2/D3 probe record (enabled by
  // --save-forensics; runs before the save_as workaround so the XML-boot
  // window is intact).
  save_forensics: { enabled: false, faces: [] }
};
if (args.saveForensics) summary.save_forensics.enabled = true;

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

// ---- kernel ZMQ face (production seam, STACKGATE 3.0) ------------------
// 373 src_template_smoke.py transport, embedded verbatim-shape: REQ command +
// SUB wait for l2_render_probe events / render topic. Python+pyzmq proven on
// this stack by the 373 DG1-B run; helper crashes surface as env interrupts.
const KERNEL_ZMQ_HELPER = [
  "import json, os, sys, time",
  "import zmq",
  "job = json.loads(sys.argv[1])",
  "req_url = job.get('req', 'tcp://127.0.0.1:5555')",
  "sub_url = job.get('sub', 'tcp://127.0.0.1:5556')",
  "def zmq_command(url, payload, timeout_ms=60000):",
  "    ctx = zmq.Context.instance()",
  "    s = ctx.socket(zmq.REQ)",
  "    s.setsockopt(zmq.LINGER, 0); s.setsockopt(zmq.RCVTIMEO, timeout_ms); s.setsockopt(zmq.SNDTIMEO, timeout_ms)",
  "    try:",
  "        s.connect(url); s.send_json(payload); return json.loads(s.recv_string())",
  "    finally:",
  "        s.close(0)",
"out = {'kind': job['kind']}",
"if job['kind'] == 'ping':",
"    out['reply'] = zmq_command(req_url, {'cmd': 'ping'}, timeout_ms=5000)",
"elif job['kind'] == 'state':",
"    out['reply'] = zmq_command(req_url, {'cmd': 'get_project_state'})",
"elif job['kind'] == 'notes':",
"    out['reply'] = zmq_command(req_url, {'cmd': 'get_midi_clip_notes', 'clip_id': job['clip_id']})",
"elif job['kind'] == 'mutate':",
"    out['reply'] = zmq_command(req_url, {'cmd': 'mutate_midi_notes', 'track_id': job['track_id'], 'clip_id': job['clip_id'],",
"        'time_unit': 'beats', 'notes': job['notes']})",
  "elif job['kind'] == 'probe':",
  "    rid = job['request_id']",
  "    reply = zmq_command(req_url, {'cmd': 'l2_render_probe', 'request_id': rid, 'track_id': job['track_id'],",
  "        'tap_point': 'track_post_fader', 'render_mode': 'offline_probe', 'start': 0.0,",
  "        'end': job.get('end', 6.25), 'tail_seconds': 0.25})",
  "    out['reply'] = reply",
  "    if str(reply.get('status', '')) == 'ok':",
  "        ctx = zmq.Context.instance(); sock = ctx.socket(zmq.SUB)",
  "        sock.setsockopt(zmq.LINGER, 0); sock.setsockopt_string(zmq.SUBSCRIBE, ''); sock.setsockopt(zmq.RCVTIMEO, 1000)",
  "        event = None; timed_out = True",
  "        try:",
  "            sock.connect(sub_url)",
  "            deadline = time.time() + job.get('wait_s', 240)",
  "            while time.time() < deadline:",
  "                try:",
  "                    payload = sock.recv_string()",
  "                except zmq.error.Again:",
  "                    continue",
  "                try:",
  "                    msg = json.loads(payload)",
  "                except Exception:",
  "                    continue",
  "                if not isinstance(msg, dict):",
  "                    continue",
  "                if str(msg.get('command', '')) not in ('l2_render_probe_ready', 'l2_render_probe_status'):",
  "                    continue",
  "                if str(msg.get('request_id', '')) != rid:",
  "                    continue",
  "                event = msg",
  "                if str(msg.get('status', '')) in ('ready', 'suspect'):",
  "                    timed_out = False; break",
  "        finally:",
  "            sock.close(0)",
  "        out['event'] = event; out['timed_out'] = timed_out",
  "elif job['kind'] == 'render':",
  "    wav = job['wav']",
  "    if os.path.exists(wav):",
  "        os.unlink(wav)",
  "    reply = zmq_command(req_url, {'cmd': 'start_render', 'file_path': wav, 'bit_depth': 16})",
  "    out['reply'] = reply",
  "    jid = str(reply.get('job_id', ''))",
  "    if str(reply.get('status', '')) == 'ok':",
  "        ctx = zmq.Context.instance(); sock = ctx.socket(zmq.SUB)",
  "        sock.setsockopt(zmq.LINGER, 0); sock.setsockopt_string(zmq.SUBSCRIBE, ''); sock.setsockopt(zmq.RCVTIMEO, 1000)",
  "        sub_signal = None",
  "        try:",
  "            sock.connect(sub_url)",
  "            deadline = time.time() + job.get('wait_s', 240)",
  "            while time.time() < deadline:",
  "                try:",
  "                    payload = sock.recv_string()",
  "                except zmq.error.Again:",
  "                    if os.path.exists(wav) and os.path.getsize(wav) > 44 and time.time() > deadline - job.get('wait_s', 240) / 3:",
  "                        sub_signal = 'file_fallback'",
  "                        break",
  "                    continue",
  "                try:",
  "                    msg = json.loads(payload)",
  "                except Exception:",
  "                    continue",
  "                if not isinstance(msg, dict):",
  "                    continue",
  "                if msg.get('topic') == 'render' and str(msg.get('job_id', '')) == jid:",
  "                    sub = str(msg.get('subtopic', ''))",
  "                    if sub == 'render_done' and str(msg.get('status')) == 'ok':",
  "                        sub_signal = 'render_done'; break",
  "                    if sub == 'render_failed' or (sub == 'render_done' and str(msg.get('status')) != 'ok'):",
  "                        sub_signal = sub + ':' + str(msg.get('status')) + ':' + str(msg.get('message', '')); break",
  "        finally:",
  "            sock.close(0)",
  "        out['completion'] = sub_signal",
  "        out['wav_bytes'] = os.path.getsize(wav) if os.path.exists(wav) else 0",
  "print(json.dumps(out))"
].join("\n");

function kernelZMQ(job, timeoutMs) {
  const res = spawnSync("python", ["-c", KERNEL_ZMQ_HELPER, JSON.stringify(job)], {
    encoding: "utf8",
    timeout: timeoutMs || 250000
  });
  if (res.error) {
    throw new Error("python spawn failed: " + res.error.message);
  }
  if (res.status !== 0) {
    throw new Error("kernel zmq helper exit " + res.status + ": " + String(res.stderr || "").slice(0, 400));
  }
  try {
    return JSON.parse(res.stdout);
  } catch (error) {
    throw new Error("kernel zmq helper non-json stdout: " + String(res.stdout).slice(0, 200));
  }
}

// ---- WAV metering (373 wav_meter/wav_diff contract, 16-bit PCM) --------
function wavMeter(filePath) {
  const buf = readFileSync(filePath);
  if (buf.length < 44 || buf.toString("ascii", 0, 4) !== "RIFF" || buf.toString("ascii", 8, 12) !== "WAVE") {
    throw new Error("not a RIFF/WAVE file: " + filePath);
  }
  let offset = 12;
  let channels = 0;
  let width = 0;
  let dataStart = -1;
  let dataLen = 0;
  while (offset + 8 <= buf.length) {
    const id = buf.toString("ascii", offset, offset + 4);
    const size = buf.readUInt32LE(offset + 4);
    if (id === "fmt ") {
      // fmt body starts at offset+8: audioFormat +0, channels +2, ...,
      // bitsPerSample +14 (absolute offset+22). Width is kept in BYTES
      // (bits/8) to match the 16-bit-PCM contract check below.
      channels = buf.readUInt16LE(offset + 8 + 2);
      width = buf.readUInt16LE(offset + 8 + 14) / 8;
    } else if (id === "data") {
      dataStart = offset + 8;
      dataLen = Math.min(size, buf.length - dataStart);
    }
    offset += 8 + size + (size % 2);
  }
  if (dataStart < 0) throw new Error("no data chunk: " + filePath);
  if (width !== 2) throw new Error("unexpected sample width " + width + ": " + filePath);
  const count = Math.floor(dataLen / 2);
  let peak = 0;
  let squareSum = 0;
  for (let i = 0; i < count; i++) {
    const sample = buf.readInt16LE(dataStart + i * 2);
    const abs = Math.abs(sample);
    if (abs > peak) peak = abs;
    squareSum += sample * sample;
  }
  const rms = count ? Math.sqrt(squareSum / count) : 0;
  return {
    channels,
    width,
    frames: Math.floor(count / Math.max(1, channels)),
    samples: count,
    peak_abs: peak,
    rms_abs: Math.round(rms * 100) / 100
  };
}

function wavDataStart(buf, filePath) {
  let offset = 12;
  while (offset + 8 <= buf.length) {
    const id = buf.toString("ascii", offset, offset + 4);
    const size = buf.readUInt32LE(offset + 4);
    if (id === "data") return { start: offset + 8, len: Math.min(size, buf.length - offset - 8) };
    offset += 8 + size + (size % 2);
  }
  throw new Error("no data chunk: " + filePath);
}

function wavDiff(fileA, fileB) {
  const a = readFileSync(fileA);
  const b = readFileSync(fileB);
  const da = wavDataStart(a, fileA);
  const db = wavDataStart(b, fileB);
  if (da.len !== db.len) {
    return { error: "data length mismatch " + da.len + " != " + db.len, max_abs_diff: null };
  }
  let maxDiff = 0;
  for (let i = 0; i + 1 < da.len; i += 2) {
    const diff = Math.abs(a.readInt16LE(da.start + i) - b.readInt16LE(db.start + i));
    if (diff > maxDiff) maxDiff = diff;
  }
  return { max_abs_diff: maxDiff };
}

function sha256File(filePath) {
  // canonical LF form (373 discipline: stable across autocrlf checkout modes)
  const data = readFileSync(filePath).toString("latin1").replace(/\r\n/g, "\n");
  return createHash("sha256").update(data, "latin1").digest("hex");
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

  // ---- KERNEL-SAVE-FORENSICS-1 (378) save-forensics probe (D1/D2/D3) -------
  // Reproduces the SCORE-WORKBENCH-1 attempt3 ordering on the XML-boot project
  // (in-session add_track/insert_midi_clip -> save_project -> IMMEDIATE
  // disclosure readback) that the save_as-first workaround below skips, plus
  // the D3 path-disclosure faces. Must run BEFORE the workaround so the
  // XML-boot window (empty kernel currentProjectPath) is intact.
  if (args.saveForensics) {
    const disclose = async (label) => {
      const state = await invoke("get_project_state", {});
      const rows = (Array.isArray(state.tracks) ? state.tracks : []).map((t) => ({
        track_id: String(t.track_id || t.id || ""),
        clip_ids: (Array.isArray(t.clips) ? t.clips : []).map((c) => String(c.clip_id || c.id || ""))
      }));
      const face = {
        label,
        project_path: String(state.project_path ?? ""),
        effective_project_path: String(state.effective_project_path ?? ""),
        audio_track_count: rows.length,
        tracks: rows
      };
      summary.save_forensics.faces.push(face);
      return face;
    };

    const pre = await disclose("pre-save (XML-boot)");
    // D3: pre-save the bound path is honestly empty; the kernel discloses the
    // boot-file fallback under effective_project_path instead.
    assert("F1-D3-pre-save-effective-path-disclosed", pre.effective_project_path.length > 0,
      "effective_project_path=" + pre.effective_project_path + " project_path=" + JSON.stringify(pre.project_path));
    assert("F1b-D3-pre-save-project-path-honestly-empty", pre.project_path === "",
      "project_path=" + JSON.stringify(pre.project_path));

    const fxTrack = await invoke("add_track", {}, true);
    const fxTrackID = String(fxTrack.track_id || fxTrack.id || "");
    const fxClip = await invoke("insert_midi_clip", { track_id: fxTrackID, time_unit: "beats", start: 0, length: 4 }, true);
    const fxClipID = String(fxClip.clip_id || fxClip.new_clip_id || "");
    await invoke("add_midi_notes", { track_id: fxTrackID, clip_id: fxClipID, time_unit: "beats", notes: [{ pitch: 60, start: 0, length: 1 }] }, true);
    summary.save_forensics.probe_track_id = fxTrackID;
    summary.save_forensics.probe_clip_id = fxClipID;

    const mid = await disclose("post-add (pre-save)");
    assert("F2-pre-save-clip-id-disclosed",
      mid.tracks.some((t) => t.track_id === fxTrackID && t.clip_ids.includes(fxClipID)),
      "track=" + fxTrackID + " clip=" + fxClipID);

    const saved = await invoke("save_project", {}, true);
    const savedPath = String(saved.project_path || saved.current_project_path || "");
    summary.save_forensics.saved_path = savedPath;
    assert("F3-save-ok", String(saved.status || "") === "ok", JSON.stringify(saved).slice(0, 200));

    const post = await disclose("post-save");
    // D1: the in-session track must survive the save in the disclosed state.
    assert("F4-D1-post-save-track-disclosed", post.tracks.some((t) => t.track_id === fxTrackID),
      "track=" + fxTrackID + (post.tracks.some((t) => t.track_id === fxTrackID) ? "" : " (D1 REPRO: dropped)"));
    // D2: clip ids must stay addressable between save and reopen.
    assert("F5-D2-post-save-clip-id-disclosed",
      post.tracks.some((t) => t.track_id === fxTrackID && t.clip_ids.includes(fxClipID)),
      "clip=" + fxClipID + " (D2 repro if missing)");

    let readbackError = "";
    let notesOk = false;
    try {
      const rows = await readKernelNotes(fxClipID, fxTrackID);
      notesOk = rows.length === 1;
    } catch (error) {
      readbackError = String(error.message).slice(0, 300);
    }
    assert("F6-post-save-notes-readable", notesOk, readbackError);

    // D3 post-save: the bound path is disclosed on both keys.
    assert("F7-D3-post-save-path-bound",
      savedPath !== "" && post.project_path === savedPath && post.effective_project_path === savedPath,
      "project_path=" + post.project_path + " effective=" + post.effective_project_path + " saved=" + savedPath);

    if (savedPath) {
      await invoke("open_project", { file_path: savedPath }, true);
      const ro = await disclose("post-reopen");
      assert("F8-reopen-clip-id-disclosed",
        ro.tracks.some((t) => t.track_id === fxTrackID && t.clip_ids.includes(fxClipID)),
        "clip=" + fxClipID);
      try {
        const rows = await readKernelNotes(fxClipID, fxTrackID);
        assert("F9-reopen-notes-readable", rows.length === 1, "notes=" + rows.length);
      } catch (error) {
        assert("F9-reopen-notes-readable", false, String(error.message).slice(0, 200));
      }
    }

    // Drop the probe track so the standard fixture legs below see the normal
    // demo shape (their own save_as re-materializes the project anyway).
    try {
      await invoke("delete_track", { track_id: fxTrackID }, true);
    } catch (error) {
      summary.save_forensics.cleanup_warning = String(error.message).slice(0, 200);
    }
    writeSummary();
  }

  let trackID = "";
  let clipID = "";
  const fixtureVit = join(args.artifactsDir, "score_wb_fixture.vit");
  try {
    // edu/373 同款正路：boot XML 工程（演示工程形态）先 save_as 到 run 目录 .vit，
    // fixture 建在 .vit 工程上——XML-boot 工程上会话内加轨再 save 会把新轨丢出
    // 状态（域外发现 D1，锚点见回执）；save 后 clip_id 披露降级 null 直至重开
    // （D2）——A6 因此按 save→open→readback 序测量（重开后 id 恢复+音符往返存活
    // 已在占用窗口内活栈亲证 2026-11-16 05:0x）。
    await invoke("save_as_project", { file_path: fixtureVit }, true);
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

  // 397 S1: capture the production apply payload at the network layer so the
  // UI binding identity can be compared with the ACTUAL write target (the same
  // target the kernel readback below is made against). This is an observation
  // of the real `/agent/invoke` request, not a test backdoor.
  const applyRequests = [];
  page.on("request", (request) => {
    if (!request.url().endsWith("/agent/invoke")) return;
    let payload = null;
    try {
      payload = request.postDataJSON();
    } catch {
      return;
    }
    if (!payload || payload.tool !== "apply_midi_note_patch") return;
    const applied = payload.args || {};
    applyRequests.push({
      track_id: String(applied.track_id ?? ""),
      clip_id: String(applied.clip_id ?? ""),
      time_unit: String(applied.time_unit ?? ""),
      confirmed: payload.confirmed === true
    });
  });
  const scoreProjectKey = (state) => String((((state || {}).project) || {}).project_path || "").replace(/\\/g, "/");
  const assertUiBinding = async (label, expectedProjectKey, expectedTrackID, expectedClipID, rootCountBefore) => {
    const loadedRaw = await page.getAttribute("[data-score-root]", "data-score-loaded-target");
    const selectedClip = (await page.getAttribute("[data-score-root]", "data-score-clip-id")) || "";
    const writeReady = (await page.getAttribute("[data-score-root]", "data-score-write-ready")) || "";
    let loaded = null;
    try {
      loaded = JSON.parse(loadedRaw || "null");
    } catch {
      loaded = null;
    }
    const expected = [expectedProjectKey, String(expectedTrackID), String(expectedClipID)];
    return assert(
      "S1-ui-binding-" + label,
      rootCountBefore === 0 && expectedProjectKey !== "" && Array.isArray(loaded) && loaded.length === 3 &&
        loaded[0] === expected[0] && String(loaded[1]) === expected[1] && String(loaded[2]) === expected[2] &&
        selectedClip === expected[2] && writeReady === "true",
      "expected " + JSON.stringify(expected) + " loaded=" + String(loadedRaw).slice(0, 160) +
        " selected=" + selectedClip + " write-ready=" + writeReady + " root-before=" + rootCountBefore
    );
  };

  const openScorePanel = async (phaseName, targetClipID) => {
    const wanted = targetClipID || clipID;
    await page.goto(args.agentHttp + "/app/", { waitUntil: "domcontentloaded" });
    await page.waitForTimeout(2500);
    const tab = page.locator('[data-workbench-tab="score"]');
    if ((await tab.count()) !== 1) throw new Error("score tab missing on " + phaseName);
    await tab.click();
    await page.waitForSelector("[data-score-binding-root]", { timeout: 15000 });
    // 397 S1 precondition: the panel must not mount a target before an explicit
    // track/clip selection (no implicit auto-load target).
    const rootBefore = await page.locator("[data-score-root]").count();
    const bindingState = await fetch(args.agentHttp + "/agent/ui/state").then((reply) => reply.json());
    const targetTrack = (bindingState.tracks || []).find((track) => (track.clips || []).some((clip) => String(clip.clip_id ?? clip.id) === wanted));
    if (!targetTrack) throw new Error("target track missing on " + phaseName);
    await page.waitForSelector(".score-track-item", { timeout: 10000 });
    const trackButtons = page.locator(".score-track-item");
    const trackIndex = await trackButtons.evaluateAll((items, id) => items.findIndex((node) => node.getAttribute("data-score-track-id") === id), String(targetTrack.track_id ?? targetTrack.id));
    if (trackIndex < 0) throw new Error("score track button missing on " + phaseName);
    await trackButtons.nth(trackIndex).click();
    // 多 clip 工程=常态（boot 演示工程 7 轨+本腿 fixture）：显式选择目标 clip
    //（真实 UI 事件，非测试后门；单 clip 工程会自动选中无需此步）。
    await page.waitForSelector("[data-score-clip-select]", { timeout: 10000 });
    const currentClip = await page.locator("[data-score-root]").count() ? await page.getAttribute("[data-score-root]", "data-score-clip-id") : "";
    if (currentClip !== wanted) {
      await page.selectOption('[data-score-clip-select]', wanted);
    }
    await page.waitForSelector("[data-score-root]", { timeout: 15000 });
    await page.waitForSelector('[data-score-write-ready="true"]', { timeout: 15000 });
    await page.waitForTimeout(1800); // clip 选择 -> 桥重建 -> 回读装载
    // 397 S1: UI-declared binding identity must equal the target used below for
    // the kernel readback (project key + track + clip), and no implicit target
    // may exist before the explicit selection.
    await assertUiBinding(phaseName, scoreProjectKey(bindingState), targetTrack.track_id ?? targetTrack.id, wanted, rootBefore);
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

    // 397 S1: the ACTUAL production write payload must target exactly the UI
    // bound track/clip -- the same ids used for the kernel readback above.
    assert("S1-apply-target",
      applyRequests.length === 1 && applyRequests[0].track_id === trackID && applyRequests[0].clip_id === clipID &&
        applyRequests[0].time_unit === "beats" && applyRequests[0].confirmed === true,
      "apply payload " + JSON.stringify(applyRequests) + " vs kernel readback target " + JSON.stringify({ track_id: trackID, clip_id: clipID }));

    // A2 step 2: after apply the DOM model is the kernel readback (same ids)
    const model2 = await domModel();
    const kernelIDs2 = kernelNotes1.map((n) => n.id).sort();
    assert("A2-id-stable-edit-apply", JSON.stringify(model2.notes.map((n) => n.id).sort()) === JSON.stringify(kernelIDs2),
      "ids drifted after apply");

    // ---- P4 A6 save -> reopen -> reload round-trip ----
    // Project path: 本腿 P0 已 save_as 到 fixtureVit（.vit 工程，edu/373 正路）；
    // A6 序=save → open(重开恢复 clip_id 披露，D2) → readback → 面板重载。
    const projectPath = fixtureVit;
    summary.project_path_source = "leg save_as target (fixture.vit); fallback --project-xml=" + (args.projectXml || "(none)");
    // saved 基线取 save 前读数（kernelNotes1=A1 断言时的全量读回）：save 后
    // save→open 之间 agent 面 clip 寻址降级（D2），post-save 即时读回会假阴性。
    const savedNotes = kernelNotes1;
    const savedSet = noteSetOf(savedNotes);
    const savedIDs = savedNotes.map((n) => n.id).sort();
    await invoke("save_project", {}, true);
    if (!projectPath) {
      summary.notes.push("project path unknown; reopen round-trip not provable on this stack");
    }
    if (projectPath) {
      await invoke("open_project", { file_path: projectPath }, true);
      await page.waitForTimeout(2500);
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

    // fixture final readback written while the fixture project is still the
    // active one (P5 below switches the session to the template project).
    writeFileSync(join(args.artifactsDir, "kernel_readback_final.json"), JSON.stringify(await readKernelNotes(clipID, trackID), null, 2), "utf-8");

    // ---- P5 A7/A8: 373 sound source + score-panel edit -> probe/WAV pair ----
    // Sound source = SRC-TEMPLATE-1 (373) course template: the melody track
    // carries the proven non-silent Surge XT (bare MIDI tracks can never pass
    // offline render node creation -- that was the A7/A8 skip reason in 374).
    // The edit is the same production gesture as P2 (real pointer click ->
    // place note -> apply -> confirmed write) exercised on the 2/4 template
    // clip. Judgment faces are the 373 DG1-B ones: kernel ZMQ l2_render_probe
    // (non-silence) + start_render WAV pair (edit differential).
    if (!existsSync(args.templateXml)) {
      return fail(2, "P5-precheck", "template source missing: " + args.templateXml);
    }
    const manifestPath = join(dirname(args.templateXml), "template_manifest.json");
    let templateManifest = {};
    try {
      templateManifest = JSON.parse(readFileSync(manifestPath, "utf-8"));
    } catch (error) {
      return fail(2, "P5-precheck", "template manifest unreadable: " + error.message);
    }
    const templateHash = sha256File(args.templateXml);
    if (templateManifest.template_sha256 && templateManifest.template_sha256 !== templateHash) {
      return fail(2, "P5-precheck", "template sha256 drift vs manifest: " + templateHash + " != " + templateManifest.template_sha256);
    }
    const templateCopy = join(args.artifactsDir, "lesson_project.xml");
    copyFileSync(args.templateXml, templateCopy); // AGENTS section 10: fixture place-copy, source stays read-only
    summary.p5_template = {
      source: args.templateXml,
      source_sha256: templateHash,
      copy: templateCopy,
      copy_sha256: sha256File(templateCopy),
      manifest_id: templateManifest.id || ""
    };

    // kernel ZMQ face reachability (infrastructure class -> env interrupt)
    try {
      const pong = kernelZMQ({ kind: "ping", req: args.kernelZmq, sub: args.kernelSub }, 20000);
      const status = String(((pong.reply || {})).status || "").toLowerCase();
      if (status !== "ok" && status !== "success") {
        throw new Error("ping replied " + JSON.stringify(pong.reply).slice(0, 120));
      }
    } catch (error) {
      return fail(2, "P5-kernel-zmq", "kernel ZMQ face unreachable via python helper: " + error.message);
    }

    // open the template copy through the agent tool face (373 B1 seam)
    try {
      await invoke("open_project", { file_path: templateCopy }, true);
    } catch (error) {
      return fail(1, "P5-open", "open_project(template copy) failed: " + error.message);
    }
    await page.waitForTimeout(6000); // session + plugin activation window (373 pacing)

    // resolve melody handles + healthy-source precondition (373 handles face)
    let melodyTrackID = "";
    let melodyClipID = "";
    try {
      const stateJob = kernelZMQ({ kind: "state", req: args.kernelZmq, sub: args.kernelSub }, 30000);
      const tracks = Array.isArray((stateJob.reply || {}).tracks) ? stateJob.reply.tracks : [];
      const melody = tracks.find((t) => String(t.name || t.track_name || "") === MELODY_TRACK_NAME);
      if (!melody) {
        throw new Error("melody track not found; tracks=" + tracks.map((t) => String(t.name || t.track_name || "")).join("|"));
      }
      melodyTrackID = String(melody.track_id || melody.id || "");
      const plugins = Array.isArray(melody.plugins) ? melody.plugins.map((p) => String(p.name || "")) : [];
      if (!plugins.includes(SURGE_NAME)) {
        throw new Error(SURGE_NAME + " not loaded on melody track after template open (plugins=" + plugins.join("|") + ") -- healthy-source precondition, 373 evidence chain");
      }
      const clips = Array.isArray(melody.clips) ? melody.clips : [];
      melodyClipID = String((clips[0] || {}).clip_id || (clips[0] || {}).id || "");
      if (!melodyClipID) throw new Error("melody track exposes no clip after open");
      summary.p5_handles = { melody_track_id: melodyTrackID, melody_clip_id: melodyClipID, melody_plugins: plugins };
    } catch (error) {
      return fail(2, "P5-handles", "melody handle resolution failed: " + error.message);
    }

    // P5 edit seam decision (D4 boundary, AGENTS §11): the panel gesture on the
    // 80bpm template project crashes the score panel at load -- kernel
    // get_midi_clip_data discloses length_seconds=8.0 for an 8-beat clip
    // (MidiService.cpp:1758-1768, time_unit="beats" with a beats-identity
    // seconds field; true value at 80bpm = 6.0s), the webui bridge converts
    // clipLengthBeats=8*80/60=10.6667 (kernel.ts:131-140) and the panel's
    // applyPatch useMemo throws ModelError(grid_misaligned) inside render with
    // no error boundary (ScoreWorkbenchPanel.tsx:401 + model.ts:96-102) --
    // [data-score-root] unmounts (live-stack diagnosis 2026-11-16 19:3x,
    // run_dg1a_20261116_1936/screens/fatal.png). 120bpm fixture projects never
    // expose it (8*120/60=16 = integer grid). Anchors handed to the decision
    // side; this leg does NOT patch out-of-domain layers. A7/A8 therefore use
    // the 373 DG1-B edit verbatim: the STACKGATE §3.0/§3.1 device anchor
    // ("编辑经 agent HTTP/tool face 到内核 ZMQ") -- the student-style
    // placeholder->assigned-pitch edit (beat 0, pitch 60 -> 79) via kernel
    // mutate_midi_notes by vit_note_id, exactly scripts/src_template_smoke.py
    // B4. A1-A6 remain panel-gesture driven on the 120bpm fixture (P1-P4).
    summary.p5_edit_seam = "agent/kernel tool face mutate_midi_notes (373 DG1-B B4 verbatim); panel gesture seam kept as card-designated judgment shape (D4 load crash fixed by 382 -- see J2 assertions)"

    // A4 template side: 2/4 disclosed through the same agent music_context
    // face the fixture side used (runtime truth, not assumption).
    const analysis5 = await invoke("analyze_midi_transformations", { clip_id: melodyClipID, track_id: melodyTrackID });
    const echo5 = analysis5.music_context_echo || {};
    summary.p5_music_context = echo5;
    assert("A4-template-2-4-disclosed",
      (echo5.time_signature_numerator || 0) + "/" + (echo5.time_signature_denominator || 0) === "2/4" && Number(echo5.tempo_bpm || 0) === 80,
      "template music_context=" + JSON.stringify(echo5).slice(0, 200));

    // pre-edit kernel note readback (373 read_notes verbatim shape) + target
    const notesBeforeJob = kernelZMQ({ kind: "notes", req: args.kernelZmq, sub: args.kernelSub, clip_id: melodyClipID }, 30000);
    const notesBefore = Array.isArray((notesBeforeJob.reply || {}).notes) ? notesBeforeJob.reply.notes : [];
    writeFileSync(join(args.artifactsDir, "melody_readback_before_edit.json"), JSON.stringify(notesBefore, null, 2), "utf-8");
    const noteKeyOf = (row) => [Math.round(Number(row.start || 0) * 4) / 4, Number(row.pitch || 0)];
    const target = notesBefore.find((row) => {
      const [start, pitch] = noteKeyOf(row);
      return Math.abs(start - 0) < 1e-3 && pitch === 60;
    });
    if (!target) {
      return fail(2, "P5-edit-target", "placeholder note at beat 0 pitch 60 not found; readback=" + JSON.stringify(notesBefore).slice(0, 300));
    }
    const targetNoteID = String(target.vit_note_id || target.note_id || target.id || "");
    if (!targetNoteID) return fail(2, "P5-edit-target", "placeholder note carries no vit_note_id");

    // A8 "before" render (pre-edit template state, whole project mix)
    const wavBefore = join(args.artifactsDir, "render_before_edit.wav");
    let renderBefore;
    try {
      const job = kernelZMQ({ kind: "render", req: args.kernelZmq, sub: args.kernelSub, wav: wavBefore, wait_s: RENDER_WAIT_SEC }, (RENDER_WAIT_SEC + 30) * 1000);
      if (String((job.reply || {}).status || "") !== "ok" || !existsSync(wavBefore) || statSync(wavBefore).size <= 44) {
        throw new Error("pre-edit render failed: " + JSON.stringify(job).slice(0, 300));
      }
      renderBefore = { completion: job.completion, job_id: (job.reply || {}).job_id, wav_bytes: statSync(wavBefore).size, ...wavMeter(wavBefore) };
    } catch (error) {
      return fail(1, "P5-render-before", "pre-edit render face failed: " + error.message);
    }

    // supporting: pre-edit melody probe (373 B4 supporting-evidence shape)
    let probeBefore = null;
    try {
      probeBefore = kernelZMQ({ kind: "probe", req: args.kernelZmq, sub: args.kernelSub, track_id: melodyTrackID, request_id: "scoredg1a_pre_" + Date.now(), end: TEMPLATE_CLIP_SECONDS + TEMPLATE_TAIL_SECONDS, wait_s: RENDER_WAIT_SEC }, (RENDER_WAIT_SEC + 30) * 1000);
      writeFileSync(join(args.artifactsDir, "probe_before_edit.json"), JSON.stringify(probeBefore, null, 2), "utf-8");
    } catch (error) {
      return fail(2, "P5-probe-before", "pre-edit probe transport failed: " + error.message);
    }

    // the edit: student-style placeholder->assigned pitch (373 B4 verbatim —
    // beat 0 C4 quarter -> G5), through the kernel command face.
    const EDIT_PITCH_TO = 79;
    const mutateJob = kernelZMQ({
      kind: "mutate", req: args.kernelZmq, sub: args.kernelSub,
      track_id: melodyTrackID, clip_id: melodyClipID,
      notes: [{ id: targetNoteID, pitch: EDIT_PITCH_TO }]
    }, 30000);
    if (String((mutateJob.reply || {}).status || "") !== "ok") {
      return fail(1, "P5-edit", "mutate_midi_notes failed: " + JSON.stringify(mutateJob.reply).slice(0, 300));
    }

    // edit landed in kernel readback (373 B4_edit_readback verbatim judgment:
    // edited present + placeholder gone + same count)
    const notesAfterJob = kernelZMQ({ kind: "notes", req: args.kernelZmq, sub: args.kernelSub, clip_id: melodyClipID }, 30000);
    const melodyNotesAfter = Array.isArray((notesAfterJob.reply || {}).notes) ? notesAfterJob.reply.notes : [];
    writeFileSync(join(args.artifactsDir, "melody_readback_after_edit.json"), JSON.stringify(melodyNotesAfter, null, 2), "utf-8");
    const afterKeys = melodyNotesAfter.map(noteKeyOf);
    assert("A7-edit-readback",
      afterKeys.some(([start, pitch]) => Math.abs(start - 0) < 1e-3 && pitch === EDIT_PITCH_TO) &&
        !afterKeys.some(([start, pitch]) => Math.abs(start - 0) < 1e-3 && pitch === 60) &&
        melodyNotesAfter.length === notesBefore.length,
      "edited_present/placeholder_gone/count=" +
        JSON.stringify({ edited: afterKeys.filter(([s, p]) => Math.abs(s) < 1e-3), count: [notesBefore.length, melodyNotesAfter.length] }));

    // A7: post-edit kernel render probe non-silent (373 B2 judgment verbatim)
    let probeAfter = null;
    try {
      probeAfter = kernelZMQ({ kind: "probe", req: args.kernelZmq, sub: args.kernelSub, track_id: melodyTrackID, request_id: "scoredg1a_a7_" + Date.now(), end: TEMPLATE_CLIP_SECONDS + TEMPLATE_TAIL_SECONDS, wait_s: RENDER_WAIT_SEC }, (RENDER_WAIT_SEC + 30) * 1000);
      writeFileSync(join(args.artifactsDir, "a7_probe_after_edit.json"), JSON.stringify(probeAfter, null, 2), "utf-8");
    } catch (error) {
      return fail(2, "P5-probe-after", "post-edit probe transport failed: " + error.message);
    }
    const eventAfter = probeAfter.event || {};
    const qualityAfter = eventAfter.quality_evidence || {};
    assert("A7-probe-non-silent",
      String((probeAfter.reply || {}).status || "") === "ok" &&
        String(eventAfter.status || "") === "ready" &&
        qualityAfter.nonzero === true &&
        Number(qualityAfter.max_abs || 0) >= PROBE_MAX_ABS_MIN &&
        Number(qualityAfter.nan_inf_count || 0) === 0,
      "reply=" + JSON.stringify(probeAfter.reply).slice(0, 160) + " event=" + JSON.stringify(eventAfter).slice(0, 240) + " timed_out=" + probeAfter.timed_out);

    // A8: post-edit render + differential (373 B4 judgment verbatim)
    const wavAfter = join(args.artifactsDir, "render_after_edit.wav");
    let renderAfter;
    try {
      const job = kernelZMQ({ kind: "render", req: args.kernelZmq, sub: args.kernelSub, wav: wavAfter, wait_s: RENDER_WAIT_SEC }, (RENDER_WAIT_SEC + 30) * 1000);
      if (String((job.reply || {}).status || "") !== "ok" || !existsSync(wavAfter) || statSync(wavAfter).size <= 44) {
        throw new Error("post-edit render failed: " + JSON.stringify(job).slice(0, 300));
      }
      renderAfter = { completion: job.completion, job_id: (job.reply || {}).job_id, wav_bytes: statSync(wavAfter).size, ...wavMeter(wavAfter) };
    } catch (error) {
      return fail(1, "P5-render-after", "post-edit render face failed: " + error.message);
    }
    const wavDiffRow = wavDiff(wavBefore, wavAfter);
    const sumBefore = Number((((probeBefore || {}).event || {}).quality_evidence || {}).sum_abs || 0);
    const sumAfter = Number(qualityAfter.sum_abs || 0);
    const relDelta = sumBefore > 0 ? Math.abs(sumAfter - sumBefore) / sumBefore : null;
    summary.p5_renders = {
      before: renderBefore,
      after: renderAfter,
      diff: wavDiffRow,
      probe_sum_abs: [sumBefore, sumAfter],
      probe_sum_abs_rel_delta: relDelta === null ? null : Math.round(relDelta * 10000) / 10000
    };
    const bothNonSilent = renderBefore.peak_abs > WAV_NON_SILENT_PEAK && renderBefore.rms_abs > WAV_NON_SILENT_RMS &&
      renderAfter.peak_abs > WAV_NON_SILENT_PEAK && renderAfter.rms_abs > WAV_NON_SILENT_RMS;
    assert("A8-edit-differential",
      bothNonSilent && wavDiffRow.max_abs_diff !== null && wavDiffRow.max_abs_diff >= WAV_DIFF_MAX_MIN,
      "before peak/rms=" + renderBefore.peak_abs + "/" + renderBefore.rms_abs +
        " after peak/rms=" + renderAfter.peak_abs + "/" + renderAfter.rms_abs +
        " max_abs_diff=" + wavDiffRow.max_abs_diff + " (error=" + (wavDiffRow.error || "none") + ")");

    // ---- SCORE-ENTRY-REACH-1 (382): J2 -- D4 修复回归（377 attempt3 崩溃点）----
    // 80bpm 模板工程（当前会话工程）面板真实装载：修复前形态=openScorePanel 选
    // 旋律 clip → pageerror grid_misaligned → [data-score-root] 消失（377 fatal.png）。
    // 2026-11-18 397 真栈第二正式轮修订：384 腿D 已修正模板资产（MIDICLIP
    // length=6.0/6.0/12.0s @80bpm = 8/8/16 拍），内核/桥直接披露 "clip 8 拍"；
    // 旧 buggy 资产的 raw 10.6667→10.75/上取整 形态已随资产修正退役。本断言改为
    // 接受修正后模板形态；"非网格上取整"防御本身如需回归，须另建合成非网格
    // fixture（不在本卡装置内扩域）。
    const journeyPageErrors = [];
    const pageErrorHandler = (error) => journeyPageErrors.push(String(error && error.message ? error.message : error));
    page.on("pageerror", pageErrorHandler);
    try {
      await openScorePanel("P5-D4-regression", melodyClipID);
      await page.screenshot({ path: join(args.screenshotDir, "j2_non120_panel_loaded.png") });
      const d4Status = await rootAttr("data-score-status");
      assert("J2-non120-panel-load",
        (await page.locator("[data-score-root]").count()) === 1 && journeyPageErrors.length === 0,
        "score-root present, pageerrors=" + JSON.stringify(journeyPageErrors).slice(0, 200));
      assert("J2-non120-clip-grid-snapped",
        d4Status.includes("已从内核装载") && d4Status.includes("2/4") && d4Status.includes("80bpm") && d4Status.includes("clip 8 拍"),
        "status='" + d4Status.slice(0, 200) + "' (expect corrected template 2/4@80bpm clip 8 拍; legacy buggy-asset 10.75/上取整 retired)");
      assert("J2-non120-error-boundary-absent",
        (await page.locator("[data-score-error-boundary]").count()) === 0,
        "boundary must not trigger for gridded non-grid clips");
    } finally {
      page.off("pageerror", pageErrorHandler);
    }

    // ---- SCORE-ENTRY-REACH-1 (382): J1/J3 -- 工程面 → 工作台 → 谱面旅程 ----
    // 374 M4「GUI 和 webui 都没看到」的旅程级回归：工程面（VitShell 工程态等价
    // surface=project）此前无 workbench/score 入口（结构性断点）；接入后=工程面
    // titlebar「打开工作台」按钮 → React 内部切 surface（零导航）→ 谱面页签 →
    // 面板装载（非 120bpm 工程）→ 返回工程条回到工程面。CDP/headless 浏览器面
    // =JOURNEY-1 面就位前的卡面指定形态。
    await page.goto(args.agentHttp + "/app/?surface=project", { waitUntil: "domcontentloaded" });
    await page.waitForSelector("[data-project-root]", { timeout: 20000 });
    await page.screenshot({ path: join(args.screenshotDir, "j1_project_surface.png") });
    assert("J1-project-surface-reachable", true, "?surface=project renders ProjectSurface (data-project-root present)");

    const entryButton = page.locator("[data-project-open-workbench]");
    assert("J1-workbench-entry-present", (await entryButton.count()) === 1,
      "project titlebar entry button count=" + (await entryButton.count()) + " (382 wiring; absent = journey broken again)");
    await entryButton.click();
    await page.waitForSelector(".app-shell-workbench", { timeout: 20000 });
    await page.waitForSelector("[data-workbench-return-bar]", { timeout: 20000 });
    assert("J1-workbench-surface-switched", true, "entry click switches to workbench surface (in-app, no navigation)");

    const journeyTab = page.locator('[data-workbench-tab="score"]');
    if ((await journeyTab.count()) !== 1) {
      return fail(1, "J1-score-tab", "score tab missing on workbench surface after journey entry");
    }
    await journeyTab.click();
    await page.waitForSelector("[data-score-binding-root]", { timeout: 15000 });
    const journeyRootBefore = await page.locator("[data-score-root]").count(); // 397: no implicit target
    await page.waitForSelector(".score-track-item", { timeout: 10000 });
    const journeyTrackButtons = page.locator(".score-track-item");
    const journeyTrackIndex = await journeyTrackButtons.evaluateAll((items, id) => items.findIndex((node) => node.getAttribute("data-score-track-id") === id), melodyTrackID);
    if (journeyTrackIndex < 0) throw new Error("J1 score melody track button missing");
    await journeyTrackButtons.nth(journeyTrackIndex).click();
    await page.waitForSelector("[data-score-clip-select]", { timeout: 10000 });
    const journeyClip = await page.locator("[data-score-root]").count() ? await rootAttr("data-score-clip-id") : "";
    if (journeyClip !== melodyClipID) {
      await page.selectOption('[data-score-clip-select]', melodyClipID);
    }
    await page.waitForSelector("[data-score-root]", { timeout: 15000 });
    await page.waitForSelector('[data-score-write-ready="true"]', { timeout: 15000 });
    await page.waitForTimeout(1800); // clip select -> bridge rebuild -> reload
    const journeyState = await fetch(args.agentHttp + "/agent/ui/state").then((reply) => reply.json());
    await assertUiBinding("J1-journey", scoreProjectKey(journeyState), melodyTrackID, melodyClipID, journeyRootBefore);
    const journeyStatus = await rootAttr("data-score-status");
    await page.screenshot({ path: join(args.screenshotDir, "j1_score_panel_from_project.png") });
    assert("J1-score-panel-from-project-journey",
      (await page.locator("[data-score-root]").count()) === 1 &&
        journeyStatus.includes("已从内核装载") && journeyStatus.includes("2/4") && journeyStatus.includes("80bpm") && journeyStatus.includes("clip 8 拍"),
      "score panel loaded from project-surface journey; status='" + journeyStatus.slice(0, 160) + "' (expect corrected template 2/4@80bpm clip 8 拍)");

    const exitButton = page.locator("[data-workbench-exit-project]");
    assert("J3-return-button-present", (await exitButton.count()) === 1, "workbench return bar button present");
    await exitButton.click();
    await page.waitForSelector("[data-project-root]", { timeout: 20000 });
    await page.screenshot({ path: join(args.screenshotDir, "j3_back_to_project.png") });
    assert("J3-return-to-project", true, "return click restores project surface (data-project-root back)");
    summary.journey_leg = {
      project_surface: "reachable",
      workbench_entry: "clicked via [data-project-open-workbench]",
      score_panel: "loaded on 80bpm template project (corrected 384 asset: 2/4, clip 8 拍)",
      return: "back to project surface"
    };

    // ---- 域外发现记录（锚点上交决策侧，不扩修） ----
    summary.notes.push(
      "D1（内核·锚点候查）：XML-boot 工程上会话内加轨再 save_project，新轨被丢出工程状态（attempt3/4 实证：track 1039+clip 1042 save 后消失；.vit-boot 不复现）。",
      "D2（内核·锚点候查）：save_project 后 get_project_state 的 tracks[].clips[].clip_id 披露降级为 null 直至 open_project 重开（agent 面 clip 寻址随之失效；产品影响=谱面面板 save 后至重开间 clip 选择器失源——本腿 A6 用 save→open 序绕开）。",
      "D3（装置面发现）：agent get_project_state（userVisibleState 消毒面）不披露 project_path/project.file_path——腿经 save_as 目标路径+--project-xml 双源。",
      "P5 补记（377）：A7/A8 判据复用 373 DG1-B（scripts/src_template_smoke.py B2/B4 同参同阈）；音源=373 模板工程旋律轨 Surge XT（place-copy 进 run 目录，源只读+manifest sha256 对账）。",
      "D4 处置（382 SCORE-ENTRY-REACH-1 已修+定性更正）：377 的「拍数恒等冒充秒」定性被 382 真栈取证推翻——内核命令面换算正确（forensic_zmq attempt5：insert_midi_clip beats8/6 回显 6.0s/4.5s@80bpm+读写一致+save→open 往返稳定）；真根因=373 手写模板 XML 的 MIDICLIP start/length 用拍数值（8.0=8 拍），tracktion loader 按秒直读（tracktion_Clip.cpp CachedValue<TimePosition> referTo 无换算）→80bpm 工程装载即 clip=8.0s=10.6667 拍。修复=①webui 渲染期错误边界（ScorePanelErrorBoundary，ModelError 不再整树卸载）②桥面网格化宽容（snapClipLengthToGrid 上取整+clipLengthBeatsRaw 披露，kernel.ts）。373 模板资产数值修正（length 8.0→6.0 等）=373 域越域，锚点上交主管裁量。",
      "D4（377 域外产品缺陷·锚点上交，非 120bpm 工程谱面面板装载崩溃）：链=①内核 get_midi_clip_data 对 80bpm 工程 8 拍 clip 披露 length_seconds=8.0（拍数恒等冒充秒，真值 6.0s；VitApp/Source/Service/MidiService.cpp:1758-1768，time_unit='beats' 标签下 pos.getLength().inSeconds() 未按工程 tempo 换算）；②webui 桥按秒义换算 clipLengthBeats=8*80/60=10.6667（agent/webui/src/scoreworkbench/kernel.ts:131-140 readClipGeometry）；③面板 applyPatch useMemo 渲染期 beatsRoundedToTicks(10.6667) 抛 ModelError(grid_misaligned) 无错误边界→整面板卸载（ScoreWorkbenchPanel.tsx:401+model.ts:96-102）。120bpm 工程下 8*120/60=16 恰整数网格不触发（374 A1-A6 因此未暴露）。最小复现=open 373 模板工程→/app/ 谱面页签→选旋律 clip→[pageerror] score model error [grid_misaligned]: beats=10.666666666666666 not on grid→data-score-root 消失（run_dg1a_20261116_1936/screens/fatal.png+活栈诊断在卷）。本腿 A7/A8 编辑缝因此走 373 B4 逐字同款 agent/kernel tool face mutate_midi_notes（STACKGATE §3.0 装置锚认可缝），不扩修域外层。（382 注：段①的内核归因经取证更正见上条；编辑缝维持 373 B4 同款=卡面指定复用的判据形态，非 D4 阻断。）"
    );

    const failed = Object.values(summary.assertions).filter((row) => !row.pass);
    summary.exit_code = failed.length === 0 ? 0 : 1;
    summary.failure_class = failed.length === 0 ? "" : "assertion_failure";
    summary.duration_ms = Date.now() - started;
    summary.clip_id = clipID;
    summary.track_id = trackID;
    writeSummary();
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

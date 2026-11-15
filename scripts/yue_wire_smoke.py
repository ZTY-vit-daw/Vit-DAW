"""GEN-YUE-WIRE-1 DG1-C driver (card 372; assertion faces C1-C8 per
docs/STACKGATE_DG1_DG3_V1.md §3.3).

Legs (deterministic /agent/invoke + kernel ZMQ faces, zero LLM by
construction — no chat goal is ever created; the agent log is archived for
the stayed-zero audit):

  W0 registration + engine-down precondition
     gen.providers lists yue-local; 8087 refuses connections so the
     supervisor bring-up later in W2 is real (C7 precondition).
  W1 student fixture (kernel ZMQ truth)
     new_project -> save_as(run dir) -> add_track -> insert_midi_clip ->
     add_midi_notes (16 melody notes, 4/4) -> get_midi_clip_notes readback.
  W2 background job v1 + engine bring-up (C6/C7/C8)
     gen.jobs.start returns immediately (C6 non-blocking RTT), the agent's
     engine supervisor brings the engine up mid-run (C7: /health reachable,
     supervisor stdout log exists), the job reaches done, artifacts land
     (C8: WAV RIFF + duration window + replay JSON + job archive JSON).
  W3 score roundtrip (C1) + instrumental resting (C3 score face)
     replay.abc (engine-side echo of what was dispatched) parses back to
     the fixture notes note-for-note on the Ins voice; the Vocal voice
     carries no sounding notes.
  W4 prompt->style mapping (C2) + extra_params hygiene (C4)
     v2 job with a different prompt: replay.style carries each version's
     user prompt text (C2); the polluting extra_params abc/style never
     reach the engine — the student score and user style do (C4).
  W5 two versions = two jobs (C5)
     v1/v2 job ids, task ids and WAV paths are distinct; both done.
  W6 non-blocking window (C6)
     a gen.providers call completes quickly while v2 is still running.
  W8 artifact validity (C8) — folded into W2/W5 checks, recorded separately.
  W9 transcription readback (C3 acoustic face, PROBABILISTIC)
     v1 WAV through SheetSage2 /transcribe: no sounding Vocal voice in the
     readback. Pre-declared failure class legW9_vocal_detected (GEN-YUE-
     DIALECT-1 ruling B: score layer deterministic, acoustic layer not
     promised; same shape twice across runs -> stop, escalate).

Exit codes: 0 = PASS; 1 = assertion failure (face_defect); 2 = environment
interrupt. Failure class recorded in wire_report.json.
"""

import argparse
import json
import os
import re
import struct
import sys
import time
import urllib.error
import urllib.request

import zmq  # pyzmq — same dependency as edu_jam_smoke.py

PROMPT_V1 = "欢快的钢琴独奏，突出学生旋律"
PROMPT_V2 = "深沉的弦乐合奏，温柔铺底"

# 16-note melody, one note per beat, 4 bars of 4/4 (C major, Ode-shaped).
FIXTURE_PITCHES = [60, 64, 67, 72, 71, 67, 64, 67, 69, 65, 62, 65, 64, 60, 62, 64]
CLIP_LENGTH_BEATS = 16.0
UNITS_PER_QUARTER = 8  # L:1/32


def fail(report, cls, leg, message):
    report["failure_class"] = cls
    report["failed_leg"] = leg
    report["error"] = message
    print("FAIL[%s/%s]: %s" % (leg, cls, message))
    return 1


def env_fail(report, cls, leg, message):
    report["failure_class"] = cls
    report["failed_leg"] = leg
    report["error"] = message
    print("ENV-FAIL[%s/%s]: %s" % (leg, cls, message))
    return 2


def http_json(method, url, body=None, timeout=60):
    data = None
    headers = {}
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return json.loads(resp.read().decode("utf-8", "replace")), ""
    except urllib.error.HTTPError as exc:
        raw = exc.read().decode("utf-8", "replace")
        try:
            return json.loads(raw), raw
        except ValueError:
            return None, raw
    except Exception as exc:  # noqa: BLE001 — transport failure is a legit outcome
        return None, "transport: %s" % exc


def invoke_tool(agent_base, tool, args, timeout=120):
    payload = {"tool": tool, "args": args}
    return http_json("POST", agent_base.rstrip("/") + "/agent/invoke", payload, timeout=timeout)


def engine_health(engine_base, timeout=4):
    url = engine_base.rstrip("/") + "/health"
    try:
        with urllib.request.urlopen(url, timeout=timeout) as resp:
            return resp.status == 200
    except Exception:  # noqa: BLE001 — any failure means not-up for the probe
        return False


def wav_duration_seconds(path):
    with open(path, "rb") as fh:
        head = fh.read(64)
    if len(head) < 44 or head[0:4] != b"RIFF" or head[8:12] != b"WAVE":
        return -1.0
    # Find the fmt/data chunk sizes the simple way: trust the canonical
    # 44-byte header when data follows fmt16 (engine emits standard PCM).
    byte_rate = struct.unpack("<I", head[28:32])[0]
    if byte_rate <= 0:
        return -1.0
    fh = open(path, "rb")
    try:
        fh.seek(0, os.SEEK_END)
        total = fh.tell()
    finally:
        fh.close()
    return max(0.0, (total - 44) / float(byte_rate))


# ---------------------------------------------------------------------------
# ABC melody parsing (independent re-implementation of the dialect semantics
# for the roundtrip cross-check: per-voice bar cursors, Z multi-bar rests
# with closing bar line, ties merged, chord annotations skipped).
# ---------------------------------------------------------------------------

NOTE_RE = re.compile(r"^([_^]=?|=)?([a-gA-G])('+|,*)(\d*)(-?)")


def parse_abc_voice(abc_text, voice):
    """Return [(pitch, start_beats, length_beats)] for one voice (ties
    merged). Bar cursor is per voice; Z<n>| advances n bars and its closing
    | advances nothing."""
    notes = []
    current = None
    units_per_bar = 32
    num, den = 4, 4
    state = None  # (bar_idx, bar_unit, last_tie_idx)

    def fresh_state():
        return [0, 0, None]  # bar index, unit cursor, tie note index

    for raw in abc_text.splitlines():
        line = raw.strip()
        if not line or line.startswith("%"):
            continue
        if line.startswith("V:"):
            current = line[2:].split()[0] if line[2:].split() else ""
            if current == voice and state is None:
                state = fresh_state()
            continue
        if len(line) > 1 and line[1] == ":":
            if line[0] == "M":
                parts = line[2:].split("/")
                if len(parts) == 2:
                    try:
                        num, den = int(parts[0]), int(parts[1])
                        units_per_bar = num * 32 // den
                    except ValueError:
                        pass
            continue
        if current != voice or state is None:
            continue
        i = 0
        z_tail = False
        while i < len(line):
            c = line[i]
            if c == '"':
                j = line.find('"', i + 1)
                if j < 0:
                    return None
                i = j + 1
                z_tail = False
            elif c == "|":
                if not z_tail:
                    state[0] += 1
                    state[1] = 0
                z_tail = False
                i += 1
            elif c == "Z":
                i += 1
                m = re.match(r"^\d+", line[i:])
                n = 1
                if m:
                    n = int(m.group(0))
                    i += len(m.group(0))
                state[0] += n
                state[1] = 0
                z_tail = True
            elif c == "z":
                i += 1
                m = re.match(r"^\d+", line[i:])
                n = 1
                if m:
                    n = int(m.group(0))
                    i += len(m.group(0))
                state[1] += n
                z_tail = False
            elif c in " \t":
                i += 1
            else:
                m = NOTE_RE.match(line[i:])
                if not m:
                    return None
                acc_ch, letter, octs, digits, tie = m.groups()
                i += len(m.group(0))
                base = {"C": 0, "D": 2, "E": 4, "F": 5, "G": 7, "A": 9, "B": 11}[letter.upper()]
                midi = base + 60
                if letter.islower():
                    pass
                else:
                    midi -= 12
                if acc_ch:
                    if acc_ch.startswith("^"):
                        midi += 1
                    elif acc_ch.startswith("_"):
                        midi -= 1
                midi += 12 * (octs.count("'") - octs.count(","))
                units = int(digits) if digits else 1
                start = (state[0] * units_per_bar + state[1]) / UNITS_PER_QUARTER
                length = units / UNITS_PER_QUARTER
                tie_flag = tie == "-"
                if state[2] is not None and notes[state[2]][0] == midi:
                    notes[state[2]] = (notes[state[2]][0], notes[state[2]][1], notes[state[2]][2] + length)
                    if not tie_flag:
                        state[2] = None
                else:
                    notes.append((midi, start, length))
                    if tie_flag:
                        state[2] = len(notes) - 1
                    else:
                        state[2] = None
                state[1] += units
                z_tail = False
    return notes


def voice_lines_rest_only(abc_text, voice):
    """(has_voice_lines, all_rest_only, offender) — voice content lines all
    rest-only? Mirrors the GEN-YUE-DIALECT-1 ins_lines_rest_only semantics
    (directives skipped, chords stripped)."""
    current = None
    checked = 0
    for raw in abc_text.splitlines():
        line = raw.strip()
        if line.startswith("V:"):
            current = line
            continue
        if current is None or voice not in current:
            continue
        if re.match(r"^[A-Za-z]:", line):
            continue
        stripped = re.sub(r'"[^"]*"', "", line)
        stripped = stripped.split("%", 1)[0]
        if not stripped.strip():
            continue
        checked += 1
        if re.search(r"[a-gA-G]", stripped.replace("z", "").replace("Z", "")):
            return checked, False, raw
    return checked, True, None


# ---------------------------------------------------------------------------
# Kernel ZMQ face (fixture truth)
# ---------------------------------------------------------------------------

class KernelZmq:
    def __init__(self, endpoint):
        self.ctx = zmq.Context.instance()
        self.sock = self.ctx.socket(zmq.REQ)
        self.sock.setsockopt(zmq.RCVTIMEO, 60000)
        self.sock.connect(endpoint)

    def command(self, cmd):
        self.sock.send_json(cmd)
        return self.sock.recv_json()

    def close(self):
        self.sock.close(0)


def build_fixture(kernel, run_root):
    reply = kernel.command({"cmd": "new_project", "command_timeout_ms": 60000})
    if str(reply.get("status", "")).lower() not in {"ok", "success"}:
        raise RuntimeError("new_project failed: %s" % reply)
    project_path = os.path.join(run_root, "student_score_project.vit")
    reply = kernel.command({"cmd": "save_as_project", "file_path": project_path, "command_timeout_ms": 60000})
    if str(reply.get("status", "")).lower() not in {"ok", "success"}:
        raise RuntimeError("save_as_project failed: %s" % reply)
    reply = kernel.command({"cmd": "add_track", "command_timeout_ms": 60000})
    if str(reply.get("status", "")) not in {"ok", "success"}:
        raise RuntimeError("add_track failed: %s" % reply)
    track_id = str(reply.get("track_id") or "")
    reply = kernel.command({
        "cmd": "insert_midi_clip", "track_id": track_id,
        "time_unit": "beats", "start": 0.0, "length": CLIP_LENGTH_BEATS,
        "command_timeout_ms": 60000,
    })
    if str(reply.get("status", "")) != "ok":
        raise RuntimeError("insert_midi_clip failed: %s" % reply)
    clip_id = str(reply.get("clip_id") or reply.get("new_clip_id") or "")
    if not clip_id:
        raise RuntimeError("insert_midi_clip returned no clip_id: %s" % reply)
    rows = [
        {"pitch": p, "start": float(i), "length": 1.0, "velocity": 90}
        for i, p in enumerate(FIXTURE_PITCHES)
    ]
    reply = kernel.command({
        "cmd": "add_midi_notes", "track_id": track_id, "clip_id": clip_id,
        "time_unit": "beats", "notes": rows, "command_timeout_ms": 60000,
    })
    if str(reply.get("status", "")) != "ok":
        raise RuntimeError("add_midi_notes failed: %s" % reply)
    notes_reply = kernel.command({"cmd": "get_midi_clip_notes", "clip_id": clip_id, "command_timeout_ms": 60000})
    if str(notes_reply.get("status", "")) != "ok":
        raise RuntimeError("get_midi_clip_notes failed: %s" % notes_reply)
    back = notes_reply.get("notes")
    if not isinstance(back, list) or len(back) != len(rows):
        raise RuntimeError("readback note count mismatch: %s" % len(back if isinstance(back, list) else "none"))
    return {"track_id": track_id, "clip_id": clip_id, "project_path": project_path, "note_count": len(rows)}


# ---------------------------------------------------------------------------
# Legs
# ---------------------------------------------------------------------------

def poll_job(agent_base, job_id, budget, run_root, tag, engine_base, report, do_probe_window):
    """Poll gen.jobs.status until done/failed. Returns (ok, status_map).
    While running: probe the engine health (C7) and, if asked, one quick
    providers call (C6 window-operable)."""
    deadline = time.time() + budget
    last = None
    engine_up_seen = False
    window_rtt = None
    status = None
    trail = []
    while time.time() < deadline:
        status, _ = invoke_tool(agent_base, "gen.jobs.status", {"job_id": job_id}, timeout=60)
        body = (status or {}).get("result") or status or {}
        state = str(body.get("state") or "")
        trail.append({"t": round(time.time(), 1), "state": state, "phase": body.get("phase")})
        last = body
        if state in {"done", "failed"}:
            break
        if not engine_up_seen and engine_health(engine_base):
            engine_up_seen = True
        if do_probe_window and window_rtt is None and state in {"running", "preparing", "starting"}:
            t0 = time.time()
            prov, _ = invoke_tool(agent_base, "gen.providers", {}, timeout=30)
            window_rtt = round(time.time() - t0, 2)
            if (prov or {}).get("status") != "ok":
                report.setdefault("window_probe", {})["providers_call"] = "failed"
        time.sleep(4)
    with open(os.path.join(run_root, "job_%s_trail.json" % tag), "w", encoding="utf-8") as fh:
        json.dump(trail, fh, ensure_ascii=False, indent=1)
    report.setdefault("jobs", {})[tag] = {
        "job_id": job_id,
        "final_state": (last or {}).get("state"),
        "task_id": (last or {}).get("task_id"),
        "phase_seen": sorted({str(t.get("phase")) for t in trail if t.get("phase")}),
        "engine_up_seen_midrun": engine_up_seen,
        "window_providers_rtt_s": window_rtt,
        "status": last,
    }
    return last, engine_up_seen, window_rtt


def artifacts_from_status(status):
    arts = (status or {}).get("artifacts") or []
    wav_path = None
    replay_path = None
    for art in arts:
        if art.get("kind") == "audio" and art.get("format") == "wav":
            wav_path = art.get("local_path")
        if art.get("kind") == "replay":
            replay_path = art.get("local_path")
    return wav_path, replay_path


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--agent-base", required=True)
    parser.add_argument("--engine-base", required=True)
    parser.add_argument("--kernel-zmq", required=True)
    parser.add_argument("--run-root", required=True)
    parser.add_argument("--download-dir", required=True)
    parser.add_argument("--jobs-dir", required=True)
    parser.add_argument("--job-budget", type=int, default=420)
    parser.add_argument("--skip-legw9", action="store_true")
    args = parser.parse_args()

    report = {"schema_version": "yue_wire_smoke.v1", "started_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())}

    def save_report():
        with open(os.path.join(args.run_root, "wire_report.json"), "w", encoding="utf-8") as fh:
            json.dump(report, fh, ensure_ascii=False, indent=1)

    # --- W0 registration + engine down ------------------------------------
    resp, raw = invoke_tool(args.agent_base, "gen.providers", {}, timeout=60)
    provs = ((resp or {}).get("result") or resp or {}).get("providers") or []
    yue = next((p for p in provs if p.get("dialect") == "yue_local"), None)
    report["w0"] = {"providers_head": (raw or "")[:300], "yue_listed": bool(yue)}
    if not yue:
        save_report()
        return fail(report, "legW0_registration", "W0", "yue_local provider not listed")
    if engine_health(args.engine_base):
        report["w0"]["engine_down_precondition"] = False
        save_report()
        return env_fail(report, "legW0_registration", "W0", "engine 8087 is already up — C7 bring-up precondition broken; stop the engine and rerun")
    report["w0"]["engine_down_precondition"] = True

    # --- W1 kernel fixture ---------------------------------------------------
    kernel = KernelZmq(args.kernel_zmq)
    try:
        fixture = build_fixture(kernel, args.run_root)
    except Exception as exc:  # noqa: BLE001 — fixture failures are env-class
        save_report()
        return env_fail(report, "legW1_fixture", "W1", str(exc))
    finally:
        kernel.close()
    report["w1_fixture"] = fixture
    print("W1 fixture ok: clip=%s notes=%d" % (fixture["clip_id"], fixture["note_count"]))

    # --- W2 job v1 -----------------------------------------------------------
    start_args = {
        "provider_id": "yue-local",
        "prompt": PROMPT_V1,
        "instrumental": True,
        "clip_id": fixture["clip_id"],
        "duration_seconds": 60,
        "version_label": "v1",
    }
    t0 = time.time()
    resp, raw = invoke_tool(args.agent_base, "gen.jobs.start", start_args, timeout=60)
    start_rtt = round(time.time() - t0, 2)
    body = (resp or {}).get("result") or resp or {}
    job_v1 = str(body.get("job_id") or "")
    report["w2_start"] = {"rtt_seconds": start_rtt, "job_id": job_v1, "raw_head": (raw or "")[:300]}
    if not job_v1:
        save_report()
        return fail(report, "legW2_start", "W2", "gen.jobs.start returned no job_id: %s" % (raw or "")[:200])
    if start_rtt > 15:
        save_report()
        return fail(report, "legW6_blocking", "W2", "gen.jobs.start blocked %ss (non-blocking contract broken)" % start_rtt)

    status_v1, engine_up_seen, _ = poll_job(args.agent_base, job_v1, args.job_budget, args.run_root, "v1", args.engine_base, report, do_probe_window=False)
    if (status_v1 or {}).get("state") != "done":
        save_report()
        return fail(report, "legW2_status", "W2", "job v1 final state=%s error=%s" % ((status_v1 or {}).get("state"), (status_v1 or {}).get("error")))

    # C7: engine is up now (agent supervisor brought it), supervisor logs exist.
    engine_up_final = engine_health(args.engine_base)
    supervisor_logs = {
        "stdout": os.path.isfile(os.path.join(args.run_root, "engine_supervisor_stdout.log")),
        "stderr": os.path.isfile(os.path.join(args.run_root, "engine_supervisor_stderr.log")),
    }
    report["w7_engine"] = {"up_midrun": engine_up_seen, "up_final": engine_up_final, "supervisor_logs": supervisor_logs}
    if not engine_up_final:
        save_report()
        return fail(report, "legW7_engine_bringup", "W2", "engine not reachable after job done — supervisor bring-up failed")

    wav_v1, replay_v1 = artifacts_from_status(status_v1)
    dur_v1 = wav_duration_seconds(wav_v1) if wav_v1 and os.path.isfile(wav_v1) else -1.0
    report["w8"] = {"wav_v1": wav_v1, "duration_v1": dur_v1, "replay_v1": replay_v1}
    if not wav_v1 or dur_v1 < 40 or dur_v1 > 150:
        save_report()
        return fail(report, "legW8_artifacts", "W2", "v1 WAV invalid (path=%s duration=%s)" % (wav_v1, dur_v1))
    if not replay_v1 or not os.path.isfile(replay_v1):
        save_report()
        return fail(report, "legW8_artifacts", "W2", "v1 replay JSON missing (%s)" % replay_v1)

    replay1_doc = {}
    try:
        with open(replay_v1, "r", encoding="utf-8") as fh:
            replay1_doc = json.load(fh)
    except Exception as exc:  # noqa: BLE001
        save_report()
        return fail(report, "legW3_roundtrip", "W3", "replay v1 unreadable: %s" % exc)
    abc1 = str(replay1_doc.get("abc") or "")

    # --- W3 roundtrip + Vocal resting ---------------------------------------
    ins_notes = parse_abc_voice(abc1, "Ins")
    report["w3"] = {"ins_note_count": len(ins_notes) if ins_notes is not None else None}
    if not ins_notes:
        save_report()
        return fail(report, "legW3_roundtrip", "W3", "Ins voice parse failed or empty")
    expected = [(p, float(i), 1.0) for i, p in enumerate(FIXTURE_PITCHES)]
    if len(ins_notes) != len(expected):
        save_report()
        return fail(report, "legW3_roundtrip", "W3", "note count %d != fixture %d" % (len(ins_notes), len(expected)))
    for idx, (got, want) in enumerate(zip(ins_notes, expected)):
        if abs(got[0] - want[0]) > 0 or abs(got[1] - want[1]) > 0.01 or abs(got[2] - want[2]) > 0.01:
            save_report()
            return fail(report, "legW3_roundtrip", "W3", "note %d mismatch got=%s want=%s" % (idx, got, want))
    # The archive JSON keeps the original MIDI notes too (version record).
    archive_v1 = os.path.join(args.jobs_dir, job_v1 + ".json")
    if not os.path.isfile(archive_v1):
        save_report()
        return fail(report, "legW8_artifacts", "W3", "job archive JSON missing: %s" % archive_v1)
    with open(archive_v1, "r", encoding="utf-8") as fh:
        archived = json.load(fh)
    src_notes = ((archived.get("source_score") or {}).get("notes")) or []
    if len(src_notes) != len(FIXTURE_PITCHES):
        save_report()
        return fail(report, "legW8_artifacts", "W3", "archived source notes %s != %d" % (len(src_notes), len(FIXTURE_PITCHES)))
    report["w3"]["archived_source_notes"] = len(src_notes)

    vocal_checked, vocal_rest, vocal_offender = voice_lines_rest_only(abc1, "Vocal")
    report["w3"]["vocal_lines_checked"] = vocal_checked
    report["w3"]["vocal_rest_only"] = vocal_rest
    if not vocal_rest:
        save_report()
        return fail(report, "legW3_vocal_not_rested", "W3", "Vocal voice has sounding notes in the dispatched score: %s" % vocal_offender)

    # --- W4 job v2 (different prompt) + hygiene ------------------------------
    start2 = {
        "provider_id": "yue-local",
        "prompt": PROMPT_V2,
        "instrumental": True,
        "clip_id": fixture["clip_id"],
        "duration_seconds": 60,
        "version_label": "v2",
    }
    resp2, raw2 = invoke_tool(args.agent_base, "gen.jobs.start", start2, timeout=60)
    body2 = (resp2 or {}).get("result") or resp2 or {}
    job_v2 = str(body2.get("job_id") or "")
    if not job_v2:
        save_report()
        return fail(report, "legW2_start", "W4", "gen.jobs.start v2 returned no job_id")
    status_v2, _, window_rtt = poll_job(args.agent_base, job_v2, args.job_budget, args.run_root, "v2", args.engine_base, report, do_probe_window=True)
    if (status_v2 or {}).get("state") != "done":
        save_report()
        return fail(report, "legW2_status", "W4", "job v2 final state=%s" % (status_v2 or {}).get("state"))

    wav_v2, replay_v2 = artifacts_from_status(status_v2)
    if not wav_v2 or not os.path.isfile(wav_v2):
        save_report()
        return fail(report, "legW8_artifacts", "W4", "v2 WAV missing")
    replay2_doc = {}
    with open(replay_v2, "r", encoding="utf-8") as fh:
        replay2_doc = json.load(fh)
    style1 = str(replay1_doc.get("style") or "")
    style2 = str(replay2_doc.get("style") or "")
    report["w4"] = {"style_v1": style1[:200], "style_v2": style2[:200], "window_providers_rtt_s": window_rtt}
    if PROMPT_V1[:6] not in style1 or PROMPT_V2[:6] not in style2:
        save_report()
        return fail(report, "legW4_style_not_mapped", "W4", "prompt->style mapping lost (v1=%r v2=%r)" % (style1[:80], style2[:80]))
    if style1 == style2:
        save_report()
        return fail(report, "legW4_style_not_mapped", "W4", "v1/v2 styles identical — versions do not differ")
    # C4 hygiene: the polluting extra_params never reached the engine.
    abc2 = str(replay2_doc.get("abc") or "")
    if "POLLUTED" in abc1 or "POLLUTED" in abc2:
        save_report()
        return fail(report, "legW4_hygiene", "W4", "polluted extra_params abc reached the engine")
    if style1 == "polluted-style" or style2 == "polluted-style":
        save_report()
        return fail(report, "legW4_hygiene", "W4", "polluted extra_params style reached the engine")
    ins2 = parse_abc_voice(abc2, "Ins")
    if not ins2 or len(ins2) != len(FIXTURE_PITCHES):
        save_report()
        return fail(report, "legW4_hygiene", "W4", "v2 score not the student melody (count=%s)" % (len(ins2) if ins2 else "parse-fail"))

    # --- W5 two versions = two jobs (C5) --------------------------------------
    task1 = str((status_v1 or {}).get("task_id") or "")
    task2 = str((status_v2 or {}).get("task_id") or "")
    report["w5"] = {"job_v1": job_v1, "job_v2": job_v2, "task_v1": task1, "task_v2": task2, "wav_v2": wav_v2}
    if job_v1 == job_v2 or task1 == task2 or wav_v1 == wav_v2:
        save_report()
        return fail(report, "legW5_not_two_jobs", "W5", "v1/v2 not independent (job/task/wav identity collision)")
    listing, _ = invoke_tool(args.agent_base, "gen.jobs.list", {}, timeout=60)
    jobs_list = ((listing or {}).get("result") or listing or {}).get("jobs") or []
    ids = {str(j.get("job_id")) for j in jobs_list}
    if job_v1 not in ids or job_v2 not in ids:
        save_report()
        return fail(report, "legW5_not_two_jobs", "W5", "version list missing jobs: %s" % ids)
    report["w5"]["listed_count"] = len(jobs_list)

    # C6 window: providers call during v2 running returned fast.
    if window_rtt is None or window_rtt > 15:
        save_report()
        return fail(report, "legW6_blocking", "W5", "window providers call rtt=%s (blocking?)" % window_rtt)

    # --- W9 transcription readback (probabilistic, skippable) -----------------
    if not args.skip_legw9:
        boundary = "yuewirebnd"
        with open(wav_v1, "rb") as fh:
            wav_bytes = fh.read()
        body_bytes = (
            ("--%s\r\nContent-Disposition: form-data; name=\"audio\"; filename=\"w9.wav\"\r\n"
             "Content-Type: audio/wav\r\n\r\n" % boundary).encode("utf-8")
            + wav_bytes
            + ("\r\n--%s--\r\n" % boundary).encode("utf-8")
        )
        req = urllib.request.Request(
            args.engine_base.rstrip("/") + "/transcribe",
            data=body_bytes,
            headers={"Content-Type": "multipart/form-data; boundary=" + boundary},
            method="POST",
        )
        try:
            with urllib.request.urlopen(req, timeout=120) as resp_tr:
                tr_submit = json.loads(resp_tr.read().decode("utf-8", "replace"))
        except Exception as exc:  # noqa: BLE001
            save_report()
            return env_fail(report, "legW9_transcribe", "W9", "transcribe submit failed: %s" % exc)
        tr_job = tr_submit.get("id")
        if not tr_job:
            save_report()
            return env_fail(report, "legW9_transcribe", "W9", "transcribe returned no job id: %s" % tr_submit)
        deadline = time.time() + args.job_budget
        status = "running"
        while time.time() < deadline:
            st, _ = http_json("GET", args.engine_base.rstrip("/") + "/job?id=" + tr_job, timeout=30)
            status = str((st or {}).get("status") or "")
            if status != "running":
                break
            time.sleep(3)
        if status != "done":
            save_report()
            return env_fail(report, "legW9_transcribe", "W9", "transcription job final status=%s" % status)
        with urllib.request.urlopen(args.engine_base.rstrip("/") + "/job?id=" + tr_job + "&result=1", timeout=120) as resp_result:
            tr_raw = resp_result.read().decode("utf-8", "replace")
        with open(os.path.join(args.run_root, "w9_transcribe_result.txt"), "w", encoding="utf-8") as fh:
            fh.write(tr_raw)
        try:
            tr_doc = json.loads(tr_raw)
        except ValueError:
            save_report()
            return fail(report, "legW9_transcribe", "W9", "transcription result is not JSON")
        abc_back = str(tr_doc.get("abc") or "")
        v_checked, v_rest, v_offender = voice_lines_rest_only(abc_back, "Vocal")
        report["w9"] = {
            "vocal_lines_checked": v_checked,
            "vocal_rest_only": v_rest,
            "offender": v_offender,
            "abc_back_head": abc_back[:300],
            "note": "probabilistic class per GEN-YUE-DIALECT-1 ruling B (score layer holds; acoustic layer not promised)",
        }
        if not v_rest:
            save_report()
            return fail(report, "legW9_vocal_detected", "W9", "sounding Vocal voice detected in transcription: %s" % v_offender)
    else:
        report["w9"] = {"skipped": True}

    # Zero-LLM audit: archive agent log LLM-ish lines (no chat face was used).
    agent_log = os.path.join(args.run_root, "agent_last.log")
    if os.path.isfile(agent_log):
        hits = []
        with open(agent_log, "r", encoding="utf-8", errors="replace") as fh:
            for line in fh:
                if re.search(r"llm|telemetry|chat_goal|model=", line, re.I):
                    hits.append(line.strip()[:200])
        report["zero_llm_audit"] = {"suspect_lines": len(hits), "sample": hits[:10]}
    report["finished_utc"] = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    report["verdict"] = "PASS"
    save_report()
    print("YUE_WIRE_DRIVER PASS")
    return 0


if __name__ == "__main__":
    sys.exit(main())

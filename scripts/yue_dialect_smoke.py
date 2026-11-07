# -*- coding: utf-8 -*-
"""GEN-YUE-DIALECT-1 real-stack smoke driver (yue_local dialect).

Three legs against a running VitApp stack (kernel + agent, HTTP face) and a
running local YuE2 engine (127.0.0.1:8087, started/stopped by the PS1 wrapper
as an independent process — it never registers in PC-RUNTIME-STACK):

  leg0 registration (deterministic, zero LLM, zero generation)
      /agent/invoke gen.providers must list the yue-local provider
      (dialect yue_local, base_url http://127.0.0.1:8087).
  leg1 chat-driven generation (1 LLM goal, 1 generation)
      /agent/chat "用 YuE2 把这段谱唱出来" carrying the baseline ABC score;
      the goal must complete and the dialect must land both artifacts in the
      run-scoped download dir (sniffed WAV + replay JSON sidecar). The only
      writer into that dir is the agent's genapi_generate yue_local call, so
      the file set IS the dialect-invocation evidence.
  leg2 a-cappella re-verification (259 acceptance carry-over; deterministic
      invoke, 1 generation + 1 SheetSage2 transcription)
      gen.generate with the P1 score (Ins voice fully rested) -> engine
      /transcribe readback -> assert the transcribed Ins voice carries no
      pitched notes (rest-only). Single-probe evidence promoted to a formal
      assertion, per the card.

Probabilistic-run protocol (AGENTS.md §8, declared before execution; the PS1
wrapper carries the same declaration):
  runs        : 1 full-chain attempt; no in-run retry of a generation leg.
  success     : exit 0 — all three legs pass their assertions.
  fail classes: leg0_registration, leg1_chat_goal, leg1_no_artifacts,
                leg1_bad_artifacts, leg2_invoke, leg2_transcribe,
                leg2_instrumental_detected, leg2_shape_mismatch,
                timeout, env_failure.
  stop-loss   : same-shape failure twice across runs -> stop and escalate;
                assertion weakening is not a repair path.

Usage (normally from run_yue_dialect_smoke.ps1):
  python scripts/yue_dialect_smoke.py --agent-base http://127.0.0.1:7878 \
      --engine-base http://127.0.0.1:8087 --run-root <dir> \
      --download-dir <dir> [--chat-budget 420] [--leg2-timeout 300]
"""

import argparse
import json
import os
import re
import sys
import time
import urllib.error
import urllib.request

# ---------------------------------------------------------------------------
# Fixtures: the YUE2-ABC-ADHERENCE-1 A_smoke1 baseline and the GEN-RECON-1 P1
# (a cappella) variant — Ins voice fully rested. Kept verbatim so the legs
# re-run the exact proven probes.
# ---------------------------------------------------------------------------

ORIG_ABC = """X:1
T:
M:4/4
L:1/32
Q:1/4=106
V: Vocal clef=treble name="Vocal Melody" snm="Vocal"
V: Ins clef=treble name="Ins Melody" snm="Inst."
K:C
% intro
V: Vocal
Z|z4e4e4e4d4c4A4c4-|
V: Ins
Z2|
% verse
V: Vocal
"Dm7"c4A4z12A4c4e4-|"G7"e4f8d12z8|"Em7"b8g8g4g8a4|"A7"g2a2z16g4a4c'4-|
V: Ins
Z4|
V: Vocal
"Dm7"c'4a8z8g4a4c'4-|"G7"c'4a8z8e4g4b4-|"Cmaj7"b12z8g4a4b4-|"A7"b12c'12z8|
V: Ins
Z4|
% chorus
V: Vocal
"Dm7"e'8c'8a6g6e'4|"G7sus4"z4c'8a8g8z4|"Em7"d'8b8g6e6d'4|"A7"d'4^c'8d'8e'8z4|
V: Ins
Z4|
V: Vocal
"Dm7"e'8c'8a6g6e'4-|"G7sus4"e'4c'8a8g4a4b4-|"Cmaj7"b12z12a4b4-|"A7"b4c'8d'8e'12|
V: Ins
Z4|
% outro
V: Vocal
"Dm7"z32|"G7sus4"z32|"Cmaj7"z32|"A7"z32|
V: Ins
e'8c'8a6g6e'4-|e'4c'8a8g12|d'8b8g6e6d'4-|d'4^c'8d'8e'12|
V: Vocal
"Dm7"z32|"G7sus4"z32|"Cmaj7"z32|"Cmaj7"z32|
V: Ins
e'8c'8a6g6e'4-|e'4c'8a8g12|b32|Z|
"""

# P1 (a cappella): ORIG_ABC with every Ins content line replaced by matching
# multi-measure rests — verbatim from the GEN-RECON-1 P1 probe.
P1_ABC = """X:1
T:
M:4/4
L:1/32
Q:1/4=106
V: Vocal clef=treble name="Vocal Melody" snm="Vocal"
V: Ins clef=treble name="Ins Melody" snm="Inst."
K:C
% intro
V: Vocal
Z|z4e4e4e4d4c4A4c4-|
V: Ins
Z2|
% verse
V: Vocal
"Dm7"c4A4z12A4c4e4-|"G7"e4f8d12z8|"Em7"b8g8g4g8a4|"A7"g2a2z16g4a4c'4-|
V: Ins
Z4|
V: Vocal
"Dm7"c'4a8z8g4a4c'4-|"G7"c'4a8z8e4g4b4-|"Cmaj7"b12z8g4a4b4-|"A7"b12c'12z8|
V: Ins
Z4|
% chorus
V: Vocal
"Dm7"e'8c'8a6g6e'4|"G7sus4"z4c'8a8g8z4|"Em7"d'8b8g6e6d'4|"A7"d'4^c'8d'8e'8z4|
V: Ins
Z4|
V: Vocal
"Dm7"e'8c'8a6g6e'4-|"G7sus4"e'4c'8a8g4a4b4-|"Cmaj7"b12z12a4b4-|"A7"b4c'8d'8e'12|
V: Ins
Z4|
% outro
V: Vocal
"Dm7"z32|"G7sus4"z32|"Cmaj7"z32|"A7"z32|
V: Ins
Z4|
V: Vocal
"Dm7"z32|"G7sus4"z32|"Cmaj7"z32|"Cmaj7"z32|
V: Ins
Z4|
"""

LYRICS = "[Verse]\n夜色落在窗台\n城市在呼吸\n[Chorus]\n把这一刻写成歌"

LEG1_STYLE = "Mandarin city pop, female vocal, warm electric piano, bass and drums, 108 BPM"
P1_STYLE = "a cappella, Mandarin female vocal, intimate, solo singing voice"


def http_json(method, url, body=None, timeout=60):
    """JSON HTTP helper. Returns (parsed, raw_text). Raises on transport/HTTP error."""
    data = None
    headers = {}
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json; charset=utf-8"
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        raw = resp.read().decode("utf-8", "replace")
    if not raw.strip():
        return None, raw
    return json.loads(raw), raw


def invoke_tool(agent_base, tool, args, timeout=120):
    payload = {"tool": tool, "args": args, "source": "yue_dialect_smoke"}
    return http_json("POST", agent_base.rstrip("/") + "/agent/invoke", payload, timeout=timeout)


def snapshot_files(download_dir):
    out = {}
    for root, _dirs, files in os.walk(download_dir):
        for name in files:
            path = os.path.join(root, name)
            try:
                out[path] = os.path.getmtime(path)
            except OSError:
                pass
    return out


def wait_engine_health(engine_base, budget):
    deadline = time.time() + budget
    while time.time() < deadline:
        try:
            parsed, _ = http_json("GET", engine_base.rstrip("/") + "/health", timeout=5)
            if parsed is None or parsed.get("status") in (None, "ok") or "ok" in str(parsed).lower():
                return True, str(parsed)
        except Exception:
            pass
        time.sleep(2)
    return False, ""


# ---------------------------------------------------------------------------
# leg0: registration disclosure
# ---------------------------------------------------------------------------

def leg0_registration(agent_base, run_root, report):
    resp, raw = invoke_tool(agent_base, "gen.providers", {}, timeout=60)
    payload = {"http_status": (resp or {}).get("status"), "raw": raw[:4000]}
    providers = ((resp or {}).get("result") or {}).get("providers") or []
    hit = None
    for entry in providers:
        if entry.get("id") == "yue-local":
            hit = entry
    if (resp or {}).get("status") != "ok" or hit is None:
        report["leg0"] = {"outcome": "leg0_registration", **payload}
        return False
    if hit.get("dialect") != "yue_local" or hit.get("base_url") != "http://127.0.0.1:8087":
        report["leg0"] = {"outcome": "leg0_registration", "entry": hit}
        return False
    report["leg0"] = {"outcome": "pass", "entry": hit, "config_path": (resp.get("result") or {}).get("config_path")}
    return True


# ---------------------------------------------------------------------------
# leg1: chat-driven generation
# ---------------------------------------------------------------------------

def leg1_chat(agent_base, run_root, download_dir, report, budget):
    before = snapshot_files(download_dir)
    # The "/goal " prefix is the designed escape hatch past the lexical
    # capability router (capability_owner.go:60 — slash-prefixed messages
    # bypass canary ownership; goalrunner_chat.go:76 keeps them on the agent
    # loop; agentLoopUserText strips the prefix before the LLM sees it).
    # Without it, an ABC score in the message gets lexically hijacked: the
    # outro "b32|Z|" contains the marker "b3" -> static_mix.pan_layout canary
    # -> turn.failed "B3 当前不能形成可执行候选" (run2 evidence, out-of-domain
    # defect reported with anchor; not fixed in this card's file domain).
    chat_message = (
        "/goal 请用 YuE2 把下面这段谱唱出来。"
        "调用 gen.generate 工具，参数：provider_id=\"yue-local\"，"
        "abc=下面这段 ABC 谱（必须逐字原样传入，不要改写、不要截断），"
        "lyrics=下面的歌词，style_tags=\"%s\"，duration_seconds=60，不要传 track_id。\n\n"
        "ABC 谱：\n%s\n\n歌词：\n%s" % (LEG1_STYLE, ORIG_ABC, LYRICS)
    )
    conversation_id = "yue_dialect_" + time.strftime("%Y%m%d_%H%M%S")
    started = time.time()
    chat_resp, chat_raw = http_json(
        "POST",
        agent_base.rstrip("/") + "/agent/chat",
        {"conversation_id": conversation_id, "message": chat_message},
        timeout=300,
    )
    goal_id = (chat_resp or {}).get("goal_id", "")
    stop_reason = (chat_resp or {}).get("stop_reason", "")
    chat_goal_status = str((chat_resp or {}).get("goal_status") or "")
    events = []
    goal_status = ""
    deadline = time.time() + budget
    while time.time() < deadline:
        try:
            ev, _ = http_json(
                "GET",
                agent_base.rstrip("/") + "/agent/events?conversation_id=" + conversation_id + "&since=0&limit=300",
                timeout=30,
            )
            events = (ev or {}).get("events") or events
        except Exception:
            pass
        try:
            rt, _ = http_json("GET", agent_base.rstrip("/") + "/agent/runtime/status", timeout=30)
            goal_status = str(((rt or {}).get("goal") or {}).get("status") or "")
        except Exception:
            pass
        if goal_status in ("completed", "failed", "stopped", "cancelled"):
            break
        # The runtime goal resets to idle after completion; the chat response
        # goal_status is the authoritative terminal record (run3 evidence).
        if chat_goal_status in ("completed", "failed", "stopped", "cancelled") and goal_status == "idle":
            goal_status = chat_goal_status
            break
        time.sleep(5)
    wall = int(time.time() - started)

    leg = {
        "conversation_id": conversation_id,
        "goal_id": goal_id,
        "goal_status": goal_status,
        "chat_goal_status": chat_goal_status,
        "stop_reason": stop_reason,
        "wall_seconds": wall,
        "chat_http_head": (chat_raw or "")[:500],
    }
    report["leg1"] = leg
    with open(os.path.join(run_root, "leg1_chat_response.json"), "w", encoding="utf-8") as fh:
        fh.write(chat_raw or "")
    with open(os.path.join(run_root, "leg1_events.json"), "w", encoding="utf-8") as fh:
        json.dump(events, fh, ensure_ascii=False, indent=1)

    if goal_status != "completed":
        leg["outcome"] = "leg1_chat_goal"
        return False

    # Artifact face: files that appeared in the run-scoped download dir during
    # the goal. Only the agent's genapi_generate yue_local call writes there.
    after = snapshot_files(download_dir)
    fresh = sorted(p for p in after if p not in before)
    wavs = [p for p in fresh if p.lower().endswith(".wav")]
    replays = [p for p in fresh if p.endswith(".replay.json")]
    leg["fresh_files"] = fresh
    if not wavs or not replays:
        leg["outcome"] = "leg1_no_artifacts"
        return False
    wav_ok = False
    for wav in wavs:
        with open(wav, "rb") as fh:
            head = fh.read(12)
        if head[0:4] == b"RIFF" and head[8:12] == b"WAVE":
            wav_ok = True
            leg["wav"] = {"path": wav, "bytes": os.path.getsize(wav)}
    replay_info = None
    for rep in replays:
        try:
            with open(rep, "r", encoding="utf-8") as fh:
                doc = json.load(fh)
            replay_info = {
                "path": rep,
                "keys": sorted(doc.keys())[:24],
                "has_abc": isinstance(doc.get("abc"), str),
                "abc_head": (doc.get("abc") or "")[:120],
            }
        except Exception as exc:  # noqa: BLE001 — recorded as observation
            replay_info = {"path": rep, "parse_error": str(exc)}
    leg["replay"] = replay_info
    if not wav_ok or not (replay_info or {}).get("has_abc"):
        leg["outcome"] = "leg1_bad_artifacts"
        return False
    leg["outcome"] = "pass"
    return True


# ---------------------------------------------------------------------------
# leg2: a-cappella re-verification (deterministic invoke + transcription)
# ---------------------------------------------------------------------------

def ins_lines_rest_only(abc_text):
    """True when every Ins-voice content line in the transcription is
    rest-only (no pitched notes). Chord annotations and % comments stripped
    first — rests are z/Z plus meter decoration. ABC directive lines
    (X:/T:/M:/L:/Q:/K:/C:/w: …) are structure, not notes: SheetSage2 emits
    K: AFTER the V: header block, and "K:C" contains the pitch letter C
    (run4 false positive — fixed by skipping ^[A-Za-z]: directives)."""
    current_voice = None
    checked_lines = 0
    for raw_line in abc_text.splitlines():
        line = raw_line.strip()
        if line.startswith("V:"):
            current_voice = line
            continue
        if current_voice is None or "Ins" not in current_voice:
            continue
        if re.match(r"^[A-Za-z]:", line):  # ABC directive (K:, Q:, M:, ...)
            continue
        stripped = re.sub(r'"[^"]*"', "", line)  # in-bar chord annotations
        stripped = stripped.split("%", 1)[0]  # comments
        if not stripped.strip():
            continue
        checked_lines += 1
        if re.search(r"[a-gA-G]", stripped):
            return False, checked_lines, raw_line
    return checked_lines > 0, checked_lines, None


def leg2_acapella(agent_base, engine_base, run_root, report, invoke_timeout, job_budget):
    args = {
        "provider_id": "yue-local",
        "prompt": "a cappella re-verification leg (GEN-YUE-DIALECT-1, 259 carry-over)",
        "style_tags": P1_STYLE,
        "lyrics": LYRICS,
        "abc": P1_ABC,
        "duration_seconds": 60,
        "scene": "free_creation",
    }
    started = time.time()
    resp, raw = invoke_tool(agent_base, "gen.generate", args, timeout=invoke_timeout)
    wall = int(time.time() - started)
    result = (resp or {}).get("result") or {}
    leg = {
        "invoke_status": (resp or {}).get("status"),
        "wall_seconds": wall,
        "task_id": result.get("task_id"),
        "dialect": result.get("dialect"),
        "limitations": result.get("limitations"),
        "status_seen": result.get("status_seen"),
        "raw_head": (raw or "")[:600],
    }
    report["leg2"] = leg
    with open(os.path.join(run_root, "leg2_invoke_response.json"), "w", encoding="utf-8") as fh:
        fh.write(raw or "")
    if (resp or {}).get("status") != "ok":
        leg["outcome"] = "leg2_invoke"
        return False
    artifacts = result.get("artifacts") or []
    wav_path = None
    for art in artifacts:
        if art.get("kind") == "audio" and art.get("format") == "wav":
            wav_path = art.get("local_path")
    leg["artifacts"] = artifacts
    if not wav_path or not os.path.isfile(wav_path):
        leg["outcome"] = "leg2_invoke"
        return False
    leg["wav"] = wav_path

    # SheetSage2 readback: POST /transcribe (multipart audio) -> poll -> result.
    boundary = "yuedialectbnd"
    with open(wav_path, "rb") as fh:
        wav_bytes = fh.read()
    body = (
        ("--%s\r\nContent-Disposition: form-data; name=\"audio\"; filename=\"leg2.wav\"\r\n"
         "Content-Type: audio/wav\r\n\r\n" % boundary).encode("utf-8")
        + wav_bytes
        + ("\r\n--%s--\r\n" % boundary).encode("utf-8")
    )
    req = urllib.request.Request(
        engine_base.rstrip("/") + "/transcribe",
        data=body,
        headers={"Content-Type": "multipart/form-data; boundary=" + boundary},
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=120) as resp_tr:
        tr_submit = json.loads(resp_tr.read().decode("utf-8", "replace"))
    job_id = tr_submit.get("id")
    leg["transcribe_job"] = tr_submit
    if not job_id:
        leg["outcome"] = "leg2_transcribe"
        return False
    deadline = time.time() + job_budget
    status = ""
    while time.time() < deadline:
        st, _ = http_json("GET", engine_base.rstrip("/") + "/job?id=" + job_id, timeout=30)
        status = str((st or {}).get("status") or "")
        if status != "running":
            break
        time.sleep(3)
    leg["transcribe_status"] = status
    if status != "done":
        leg["outcome"] = "leg2_transcribe"
        return False
    with urllib.request.urlopen(
        engine_base.rstrip("/") + "/job?id=" + job_id + "&result=1", timeout=120
    ) as resp_result:
        tr_raw = resp_result.read().decode("utf-8", "replace")
    with open(os.path.join(run_root, "leg2_transcribe_result.txt"), "w", encoding="utf-8") as fh:
        fh.write(tr_raw)
    try:
        tr_doc = json.loads(tr_raw)
    except ValueError:
        leg["outcome"] = "leg2_shape_mismatch"
        leg["transcribe_head"] = tr_raw[:300]
        return False
    abc_back = tr_doc.get("abc")
    if not isinstance(abc_back, str):
        leg["outcome"] = "leg2_shape_mismatch"
        return False
    ok, checked, offender = ins_lines_rest_only(abc_back)
    leg["reverify"] = {
        "ins_lines_checked": checked,
        "ins_rest_only": ok,
        "offender_line": offender,
        "abc_back_head": abc_back[:400],
    }
    if not ok:
        leg["outcome"] = "leg2_instrumental_detected" if checked > 0 else "leg2_shape_mismatch"
        return False
    leg["outcome"] = "pass"
    return True


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--agent-base", default="http://127.0.0.1:7878")
    parser.add_argument("--engine-base", default="http://127.0.0.1:8087")
    parser.add_argument("--run-root", required=True)
    parser.add_argument("--download-dir", required=True)
    parser.add_argument("--chat-budget", type=int, default=420)
    parser.add_argument("--leg2-invoke-timeout", type=int, default=300)
    parser.add_argument("--leg2-job-budget", type=int, default=300)
    parser.add_argument("--skip-leg1", action="store_true")
    args = parser.parse_args()

    os.makedirs(args.run_root, exist_ok=True)
    os.makedirs(args.download_dir, exist_ok=True)
    report = {
        "schema_version": "yue_dialect_smoke.v1",
        "agent_base": args.agent_base,
        "engine_base": args.engine_base,
        "run_root": args.run_root,
        "download_dir": args.download_dir,
        "started": time.strftime("%Y-%m-%dT%H:%M:%S"),
    }
    outcome = "env_failure"
    try:
        healthy, health_body = wait_engine_health(args.engine_base, 10)
        report["engine_health"] = {"ok": healthy, "body": health_body[:200]}
        if not healthy:
            report["outcome"] = "env_failure"
            raise SystemExit(2)

        ok0 = leg0_registration(args.agent_base, args.run_root, report)
        if not ok0:
            outcome = report["leg0"]["outcome"]
            raise SystemExit(1)

        ok1 = True
        if not args.skip_leg1:
            ok1 = leg1_chat(args.agent_base, args.run_root, args.download_dir, report, args.chat_budget)
            if not ok1:
                outcome = report["leg1"]["outcome"]
                raise SystemExit(1)

        ok2 = leg2_acapella(
            args.agent_base, args.engine_base, args.run_root, report,
            args.leg2_invoke_timeout, args.leg2_job_budget,
        )
        if not ok2:
            outcome = report["leg2"]["outcome"]
            raise SystemExit(1)
        outcome = "finished"
        raise SystemExit(0)
    except SystemExit as exc:
        report["outcome"] = outcome if outcome != "env_failure" else (
            report.get("outcome") or outcome
        )
        report["exit_code"] = int(exc.code) if exc.code is not None else 0
    except Exception as exc:  # noqa: BLE001 — recorded for forensics, fail closed
        report["outcome"] = report.get("outcome") or "env_failure"
        report["fatal"] = "%s: %s" % (type(exc).__name__, exc)
        report["exit_code"] = 2
    finally:
        report["finished"] = time.strftime("%Y-%m-%dT%H:%M:%S")
        with open(os.path.join(args.run_root, "yue_dialect_smoke_report.json"), "w", encoding="utf-8") as fh:
            json.dump(report, fh, ensure_ascii=False, indent=1)
        print("YUE_DIALECT_VERDICT " + str(report.get("outcome")))
    sys.exit(report.get("exit_code", 2))


if __name__ == "__main__":
    main()

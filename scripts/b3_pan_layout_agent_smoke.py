"""Godot-product-path smoke for B3 static pan-layout planning and execution."""

from __future__ import annotations

import argparse
import json
import re
import time
import urllib.request
from pathlib import Path
from typing import Any


def request_json(method: str, url: str, payload: dict[str, Any] | None, timeout: float) -> dict[str, Any]:
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
    request = urllib.request.Request(
        url, data=data, headers={"Content-Type": "application/json; charset=utf-8"}, method=method
    )
    with urllib.request.urlopen(request, timeout=timeout) as response:
        result = json.loads(response.read().decode("utf-8", errors="replace"))
    if not isinstance(result, dict):
        raise RuntimeError(f"{url} returned non-object JSON")
    return result


def wait_agent(base_url: str, timeout: float) -> dict[str, Any]:
    deadline = time.time() + timeout
    last_error: Exception | None = None
    while time.time() < deadline:
        try:
            health = request_json("GET", base_url.rstrip("/") + "/health", None, 3.0)
            if str(health.get("status", "")).lower() in {"ok", "ready"}:
                return health
        except Exception as exc:  # noqa: BLE001 - retry details belong in smoke artifacts.
            last_error = exc
        time.sleep(0.25)
    raise RuntimeError(f"VitAgent did not become ready at {base_url}: {last_error}")


def execution_tools(response: dict[str, Any]) -> list[str]:
    tools: list[str] = []
    for row in response.get("executed_kernel_reply", []):
        if isinstance(row, dict):
            name = str(row.get("tool", row.get("command_name", ""))).strip()
            if name:
                tools.append(name)
    return tools


def result_map(response: dict[str, Any]) -> dict[str, Any]:
    result = response.get("result")
    return result if isinstance(result, dict) else {}


def invoke_tool(
    base_url: str, tool: str, args: dict[str, Any], timeout: float, *, confirmed: bool = False
) -> dict[str, Any]:
    payload: dict[str, Any] = {
        "tool": tool,
        "args": args,
        "source": "b3_pan_layout_agent_smoke.fixture",
        "confirmed": confirmed,
    }
    response = request_json("POST", base_url.rstrip("/") + "/agent/invoke", payload, timeout)
    if str(response.get("status", "")).lower() not in {"ok", "success", "completed"}:
        raise RuntimeError(f"fixture tool {tool} failed: {json.dumps(response, ensure_ascii=False)[:2200]}")
    return response


def track_pans(base_url: str, timeout: float) -> dict[str, float]:
    state = result_map(invoke_tool(base_url, "project.state", {}, timeout))
    pans: dict[str, float] = {}
    for row in state.get("tracks", []):
        if not isinstance(row, dict):
            continue
        track_id = str(row.get("track_id") or row.get("id") or "").strip()
        value = row.get("pan", row.get("pan_value"))
        if track_id and isinstance(value, (int, float)):
            pans[track_id] = float(value)
    return pans


def prepare_fixture(base_url: str, project_path: Path, audio_path: Path, timeout: float) -> dict[str, Any]:
    invoke_tool(base_url, "project.new", {}, timeout, confirmed=True)
    invoke_tool(base_url, "project.save_as", {"file_path": str(project_path)}, timeout, confirmed=True)
    created: list[dict[str, str]] = []
    for name in (
        "Lead Vocal",
        "Guitar Double 1",
        "Guitar Double 2",
        "Backing Vocal 1",
        "Backing Vocal 2",
        "Percussion 1",
        "Percussion 2",
    ):
        response = invoke_tool(base_url, "track.add_audio", {"name": name}, timeout, confirmed=True)
        result = result_map(response)
        track_id = str(result.get("track_id") or result.get("id") or result.get("item_id") or "").strip()
        if not track_id:
            raise RuntimeError(f"track.add_audio did not return track_id for {name}: {json.dumps(response, ensure_ascii=False)[:1200]}")
        invoke_tool(
            base_url,
            "track.rename",
            {"track_id": track_id, "name": name, "track_name": name},
            timeout,
            confirmed=True,
        )
        invoke_tool(
            base_url,
            "clip.import_audio",
            {"track_id": track_id, "file_path": str(audio_path), "offset_time": 0.0},
            timeout,
            confirmed=True,
        )
        created.append({"track_id": track_id, "track_name": name})
    state = result_map(invoke_tool(base_url, "project.state", {}, timeout))
    return {"project_path": str(project_path), "created": created, "track_count": len(state.get("tracks", []))}


def request_project_l3_preflight(base_url: str, timeout: float) -> dict[str, Any]:
    response = request_json(
        "POST",
        base_url.rstrip("/") + "/agent/invoke",
        {
            "tool": "mix.observe",
            "confirmed": True,
            "source": "b3_pan_layout_agent_smoke.fixture_l3_preflight",
            "args": {
                "scope": "full_project",
                "project_context": True,
                "observation_only": True,
                "observation_ready_gate": True,
                "disclosure": "digest_catalog",
                "mom_intent": "action_preflight_observation",
                "mix_session_id": "b3_pan_layout_fixture_l3_preflight",
                "goal_text": "Prepare isolated B3 fixture L3 stereo evidence",
            },
        },
        timeout,
    )
    result = result_map(response)
    return {
        "status": response.get("status"),
        "result_status": result.get("status"),
        "reason": result.get("reason"),
        "missing_required_features": result.get("missing_required_features"),
    }


def wait_mom_relationships(base_url: str, timeout: float) -> dict[str, Any]:
    deadline = time.monotonic() + timeout
    latest: dict[str, Any] = {}
    while time.monotonic() < deadline:
        latest = invoke_tool(
            base_url,
            "mix.observe",
            {
                "scope": "full_project",
                "project_context": True,
                "observation_only": True,
                "disclosure": "digest_catalog",
                "mom_intent": "project_multitrack_relation_observation",
                "mix_session_id": "b3_pan_layout_fixture_readiness",
                "goal_text": "B3 fixture relationship readiness",
            },
            timeout,
        )
        result = result_map(latest)
        projection = result.get("mom_projection") if isinstance(result.get("mom_projection"), dict) else {}
        relation = (
            projection.get("multitrack_relation")
            if isinstance(projection.get("multitrack_relation"), dict)
            else {}
        )
        stereo = relation.get("stereo_distribution") if isinstance(relation.get("stereo_distribution"), dict) else {}
        relation_status = str(relation.get("status", "")).strip().lower()
        stereo_status = str(stereo.get("status", "")).strip().lower()
        if (
            str(result.get("observation_id", "")).strip()
            and projection
            and relation_status == "ready"
            and stereo_status == "ready"
        ):
            return {
                "observation_id": result.get("observation_id"),
                "multitrack_relation_status": relation_status,
                "stereo_distribution_status": stereo_status,
                "track_count": relation.get("track_count"),
                "missing_track_count": relation.get("missing_track_count"),
            }
        time.sleep(0.5)
    raise RuntimeError(
        "B3 fixture MOM stereo relationships did not become ready: "
        + json.dumps(result_map(latest), ensure_ascii=False)[:4000]
    )


def chat(base_url: str, conversation_id: str, message: str, timeout: float) -> dict[str, Any]:
    return request_json(
        "POST",
        base_url.rstrip("/") + "/agent/chat",
        {
            "conversation_id": conversation_id,
            "message": message,
            "context": {"agent_mode": "chat"},
        },
        timeout,
    )


def workflow_data(response: dict[str, Any]) -> dict[str, Any]:
    data = response.get("workflow_data")
    return data if isinstance(data, dict) else {}


def canary_stage(response: dict[str, Any]) -> str:
    return str(workflow_data(response).get("canary_stage", ""))


def track_pan_volume_state(base_url: str, timeout: float) -> dict[str, dict[str, float]]:
    # The capability_runtime_v1 handler family never populates
    # executed_kernel_reply (see the product-path flow above), so "B3 wrote
    # nothing" cannot be checked by inspecting tool-call lists alone. The
    # reliable read-only proof for the ablation face is a before/after
    # snapshot of the actual mix state the rounds could plausibly touch:
    # pan and fader volume per track (the 264 B4 pattern, retargeted from
    # volume+pan to the B3 pan write face plus volume for good measure).
    state = result_map(invoke_tool(base_url, "project.state", {}, timeout))
    out: dict[str, dict[str, float]] = {}
    for row in state.get("tracks", []):
        if not isinstance(row, dict):
            continue
        track_id = str(row.get("track_id") or row.get("id") or "").strip()
        if not track_id:
            continue
        pan = row.get("pan", row.get("pan_value"))
        volume = row.get("volume_db", row.get("fader_db"))
        entry: dict[str, float] = {}
        if isinstance(pan, (int, float)):
            entry["pan"] = float(pan)
        if isinstance(volume, (int, float)):
            entry["volume_db"] = float(volume)
        if entry:
            out[track_id] = entry
    return out


def mix_state_diff(before: dict[str, dict[str, float]], after: dict[str, dict[str, float]]) -> list[str]:
    changed: list[str] = []
    for track_id, before_values in before.items():
        after_values = after.get(track_id, {})
        for key, value in before_values.items():
            if abs(after_values.get(key, value) - value) > 0.001:
                changed.append(f"{track_id}.{key}")
    return changed


# ---------------------------------------------------------------------------
# B3-ABLATION-1 (card 275): ablation mode. The ps1 wrapper sets
# VIT_PAN_LAYOUT_ABLATION=1 on the agent (B3 lexicon short-circuit + canary
# dispatch disabled, 264 B4 hook-family pattern), so the same B3 phrase that
# the default product-path flow above proves lexically routed now falls
# through to the LLM turn. This mode (a) runs the phrase x N rounds (fresh
# conversation each, the 263/264 rounds-per-group pattern), (b) judges every
# round purely on the LLM-turn evidence, and (c) treats any lexicon_hit
# verdict as an invalid run (the hook not reaching the agent is a
# configuration failure, never a model sample). Default off keeps the
# product-path behavior byte-for-byte unchanged.
#
# Unlike B4 (pure read-only observation), the B3 capability face carries an
# execution write face (mix.apply_pan_layout_batch). The A-verdict criterion
# (card 275 goal 3): a harness round must autonomously reproduce the
# "pan-layout observation + suggestion" main face — observation-family tools
# + layout suggestion + confirmation-seeking closing form. The zero-raw-write
# red line: any executed write tool in a single-turn round is recorded as
# raw_write (never a success); the assertion face closes on the
# suggestion+confirmation form and never asserts raw execution.
# ---------------------------------------------------------------------------

B3_ABLATION_QUESTION = "主唱留在中间，两把吉他分别放到左右两侧，帮我规划整个工程的声像布局"

# Canary/pan_layout fixed-face takeover markers: any hit means the hook is
# not live on the agent (workflow=capability_runtime_v1 / canary_stage
# non-empty / layout_summary present / legacy pan_layout workflow form).
ABLATION_LEXICON_WORKFLOWS = {"capability_runtime_v1", "pan_layout"}

# Observation-family tool markers: a B3 capability reproduction must execute
# at least one real observation tool (the canary path's chain is
# mix.observe -> MOM/TOM stereo relation projections -> proposal).
ABLATION_OBSERVATION_TOOL_MARKERS = (
    "mix.observe",
    "mix.derive",
    "mix.summary",
    "ccb.observation",
    "audio_analysis",
    "stereo",
    "project.state",
)

# The B3 write face: a governed ablation round may close on a
# suggestion+confirmation form, but must never execute these raw.
ABLATION_FORBIDDEN_WRITE_TOOLS = {
    "track.pan",
    "track.volume",
    "clip.gain.set",
    "mix.apply_tick",
    "mix.apply_static_balance_batch",
    "mix.apply_pan_layout_batch",
}

# Honest-boundary wording for the pan-layout domain (264 boundary lexicon
# pattern, retargeted to the pan/stereo/layout vocabulary).
ABLATION_BOUNDARY_REPLY_PATTERN = re.compile(
    r"(没有|未|无法|不支持|缺|不存在|需要先|尚无|暂无|先要|请先|不能直接).{0,32}(分析|数据|音频|工程|轨道|插件|声像|布局|结果|analysis|audio|project|track|pan|layout)",
    re.IGNORECASE,
)

# Card 275 goal 4: the A bar is tool_chain_ok >= 4/5 with zero lexicon_hit.
# boundary_ok is recorded honestly but does not count toward the B3 pass bar
# (the B3 A face includes the suggestion+confirmation closing form, so a
# plain boundary is not a face reproduction).
ABLATION_PASS_VERDICT = "tool_chain_ok"


def ablation_forbidden_write_tools(tools: list[str]) -> list[str]:
    hits: list[str] = []
    for name in tools:
        if name in ABLATION_FORBIDDEN_WRITE_TOOLS:
            hits.append(name)
            continue
        base = name.rsplit(".", 1)[-1]
        if base == "set" or base.startswith("apply"):
            hits.append(name)
    return hits


def ablation_confirmation_form(response: dict[str, Any]) -> bool:
    if response.get("needs_confirmation") is True:
        return True
    if response.get("interaction_requests"):
        return True
    return str(response.get("stop_reason", "")) == "needs_confirmation"


def classify_ablation_round(response: dict[str, Any]) -> tuple[str, list[str], str]:
    """Mechanical verdict for one LLM-turn round. Priority order is fixed
    before any round runs (§8): invalid-hook detection first (it voids the
    run), then entry-layer interception, then the raw-write red line, then
    the success form, then honest non-success filing."""
    tools = execution_tools(response)
    workflow = str(response.get("workflow", ""))
    stop_reason = str(response.get("stop_reason", ""))
    goal_status = str(response.get("goal_status", ""))
    reply = str(response.get("reply", ""))
    detail = json.dumps(
        {
            "workflow": workflow,
            "stop_reason": stop_reason,
            "goal_status": goal_status,
            "tools": tools,
            "needs_confirmation": response.get("needs_confirmation"),
            "reply_head": reply[:200],
        },
        ensure_ascii=False,
    )
    # 1. lexicon_hit: the B3 canary / legacy pan_layout workflow still took
    #    over -> the hook is not live on the agent; the whole run is a
    #    configuration failure (exit 4).
    if (
        workflow in ABLATION_LEXICON_WORKFLOWS
        or canary_stage(response) != ""
        or isinstance(workflow_data(response).get("layout_summary"), dict)
    ):
        return "lexicon_hit", tools, detail
    # 2. Entry-layer interception (251/263 form): semantic_entry arbiter
    #    unresolved.
    if stop_reason == "semantic_entry_unresolved" or workflow == "semantic_entry":
        return "semantic_entry_intercepted", tools, detail
    # 3. Red line: any raw write executed in this single-turn round (B3 has
    #    an execution face; the governed path closes on suggestion+confirmation
    #    instead of executing). Recorded as-is, never a success.
    if ablation_forbidden_write_tools(tools):
        return "raw_write", tools, detail
    observed = [name for name in tools if any(marker in name for marker in ABLATION_OBSERVATION_TOOL_MARKERS)]
    confirmation = ablation_confirmation_form(response)
    # 4. Capability reproduced (card 275 goal 3 A face): autonomous
    #    observation chain + substantive suggestion (>= 40 chars) closed on
    #    the confirmation-seeking form.
    if confirmation and observed and len(reply.strip()) >= 40:
        return "tool_chain_ok", tools, detail
    if confirmation and not observed:
        return "confirm_without_observation", tools, detail
    if goal_status == "completed":
        # Observation chain + substantive suggestion, but no mechanical
        # confirmation-seeking closing form -> partial face, honest filing.
        if observed and len(reply.strip()) >= 40:
            return "advice_only", tools, detail
        if observed:
            return "weak_report", tools, detail
        if ABLATION_BOUNDARY_REPLY_PATTERN.search(reply):
            return "boundary_ok", tools, detail
        if not tools:
            return "no_route", tools, detail
        return "other_completed", tools, detail
    return f"error_{goal_status or 'unknown'}", tools, detail


def run_ablation_mode(args: argparse.Namespace) -> int:
    run_stamp = time.strftime("%Y%m%d_%H%M%S")
    artifact_dir = Path(args.rounds_dir).resolve().parent if args.rounds_dir else None
    if artifact_dir is None:
        artifact_dir = Path(args.repo_root).resolve() / "VitApp" / "Workspace" / "Artifacts" / "smoke" / f"b3_ablation_{run_stamp}"
    artifact_dir.mkdir(parents=True, exist_ok=True)
    rounds_dir = Path(args.rounds_dir).resolve() if args.rounds_dir else artifact_dir / "rounds"
    rounds_dir.mkdir(parents=True, exist_ok=True)

    health = wait_agent(args.agent_http, args.timeout_sec)

    audio_path = Path(args.repo_root).resolve() / "test_target_3s.wav"
    if not audio_path.is_file():
        raise RuntimeError(f"fixture audio not found: {audio_path}")
    # Real multi-track pan fixture (>=3 tracks with pan-decidable roles:
    # Lead Vocal / Guitar Double 1-2 / Backing Vocal 1-2 / Percussion 1-2),
    # the same product-path fixture helper, py-built as the 264 precedent.
    # No MOM/audio-analysis pre-warm: the honest ablation face is the model
    # autonomously deciding whether to trigger the observation chain.
    fixture = prepare_fixture(args.agent_http, artifact_dir / "fixture_project.vit", audio_path, args.timeout_sec)
    mix_state_before = track_pan_volume_state(args.agent_http, args.timeout_sec)

    phrase = args.phrase.strip() or B3_ABLATION_QUESTION
    rounds: list[dict[str, Any]] = []
    verdict_counts: dict[str, int] = {}
    stop_loss: dict[str, Any] | None = None
    rounds_run = 0
    for index in range(1, args.rounds + 1):
        conversation_id = f"b3_ablation_{run_stamp}_r{index}"
        record: dict[str, Any] = {"round": index, "phrase": phrase, "conversation": conversation_id}
        try:
            response = chat(args.agent_http, conversation_id, phrase, args.timeout_sec)
        except Exception as exc:  # noqa: BLE001 - round-level transport failure is a verdict, not a crash.
            record["verdict"] = "error_timeout"
            record["detail"] = f"chat transport failure: {exc}"[:800]
            response = None
        if response is not None:
            (rounds_dir / f"r{index}_response.json").write_text(
                json.dumps(response, ensure_ascii=False, indent=2), encoding="utf-8"
            )
            verdict, tools, detail = classify_ablation_round(response)
            record["verdict"] = verdict
            record["tools"] = tools
            record["detail"] = detail
        rounds.append(record)
        rounds_run = index
        verdict_counts[record["verdict"]] = verdict_counts.get(record["verdict"], 0) + 1
        print(f"[b3-ablation] round {index}/{args.rounds} verdict={record['verdict']} tools={record.get('tools', [])}")
        if record["verdict"] == "lexicon_hit":
            break
        # Stop-loss (263/264 RUNBOOK precedent): 3 consecutive same-type
        # non-pass verdicts stop the group, evidence preserved, no more
        # rounds spent.
        if len(rounds) >= 3:
            tail = [row["verdict"] for row in rounds[-3:]]
            if len(set(tail)) == 1 and tail[0] != ABLATION_PASS_VERDICT and tail[0] != "lexicon_hit":
                stop_loss = {"after_round": index, "verdict": tail[0], "rounds_saved": args.rounds - index}
                print(f"[b3-ablation] stop-loss: 3 consecutive {tail[0]} rounds; remaining rounds not spent")
                break

    mix_state_after = track_pan_volume_state(args.agent_http, args.timeout_sec)
    mutated_tracks = mix_state_diff(mix_state_before, mix_state_after)
    tool_chain_ok = verdict_counts.get(ABLATION_PASS_VERDICT, 0)
    lexicon_hit_seen = verdict_counts.get("lexicon_hit", 0) > 0

    summary: dict[str, Any] = {
        "schema_version": "b3_ablation.v1",
        "scenario": "B3-ABLATION-1: pan-layout phrase family under the B3 ablation hook (LLM-turn evidence only)",
        "ablation_hook": True,
        "hook_env": "VIT_PAN_LAYOUT_ABLATION=1 (set by the ps1 wrapper on the agent)",
        "phrase": phrase,
        "rounds_requested": args.rounds,
        "rounds_run": rounds_run,
        "rounds": rounds,
        "verdict_counts": verdict_counts,
        "tool_chain_ok": tool_chain_ok,
        "boundary_ok": verdict_counts.get("boundary_ok", 0),
        "pass_verdict": ABLATION_PASS_VERDICT,
        "group_pass": tool_chain_ok >= 4 and rounds_run == args.rounds,
        "stop_loss": stop_loss,
        "lexicon_hit_seen": lexicon_hit_seen,
        "zero_raw_write_violations": verdict_counts.get("raw_write", 0),
        "read_only_verified": not mutated_tracks,
        "mix_state_mutations": mutated_tracks,
        "fixture": fixture,
        "health": health,
        "artifact_dir": str(artifact_dir),
        "telemetry_path": args.telemetry_path or "",
        "prompt_render_path": args.prompt_render_path or "",
        "payload_capture_path": args.payload_capture_path or "",
        "agent_log": args.agent_log or "",
        "started_at": run_stamp,
    }
    output_path = Path(args.output).resolve() if args.output else artifact_dir / "b3_ablation_summary.json"
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps({key: summary[key] for key in ("rounds_run", "verdict_counts", "tool_chain_ok", "group_pass", "stop_loss", "lexicon_hit_seen", "zero_raw_write_violations", "read_only_verified")}, ensure_ascii=False))
    print(f"[b3-ablation] summary: {output_path}")
    if lexicon_hit_seen:
        print("[b3-ablation] INVALID: lexicon_hit verdict seen -- the hook did not reach the agent (configuration failure)")
        return 4
    return 0


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo-root", default=str(Path(__file__).resolve().parents[1]))
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--timeout-sec", type=float, default=240.0)
    parser.add_argument("--dad-timeout-sec", type=float, default=240.0)
    parser.add_argument("--prepare-stems-folder", default="")
    parser.add_argument("--skip-fixture", action="store_true")
    # B3-ABLATION-1: ablation mode — the ps1 wrapper sets
    # VIT_PAN_LAYOUT_ABLATION=1 on the agent (B3 lexicon short-circuit +
    # canary dispatch disabled), so this driver judges the LLM-turn evidence
    # of a B3 pan-layout phrase over N rounds (default 5, the 263/264
    # rounds-per-group pattern). Default off keeps the product-path flow
    # byte-for-byte unchanged.
    parser.add_argument("-AblationHook", "--ablation-hook", dest="ablation_hook", action="store_true",
                        help="B3 ablation mode: agent B3 lexicon+dispatch disarmed via env; every round judged as an LLM sample")
    parser.add_argument("--phrase", default="",
                        help="ablation canonical phrase override (default: the B3 canonical candidate; the RUNBOOK prescan pins the final choice)")
    parser.add_argument("--rounds", type=int, default=5,
                        help="rounds per ablation group (default 5)")
    parser.add_argument("--output", default="", help="ablation summary output path")
    parser.add_argument("--rounds-dir", default="", help="per-round response artifact dir")
    parser.add_argument("--telemetry-path", default="", help="provenance: agent telemetry jsonl path (recorded in summary)")
    parser.add_argument("--prompt-render-path", default="", help="provenance: prompt render jsonl path (recorded in summary)")
    parser.add_argument("--payload-capture-path", default="", help="provenance: payload capture jsonl path (recorded in summary)")
    parser.add_argument("--agent-log", default="", help="provenance: agent log path (recorded in summary)")
    args = parser.parse_args()
    if args.ablation_hook:
        return run_ablation_mode(args)

    repo = Path(args.repo_root).resolve()
    run_stamp = time.strftime("%Y%m%d_%H%M%S")
    artifact_dir = repo / "VitApp" / "Workspace" / "Artifacts" / "smoke" / f"b3_pan_layout_{run_stamp}"
    artifact_dir.mkdir(parents=True, exist_ok=True)

    health = wait_agent(args.agent_http, args.timeout_sec)
    fixture = None
    if args.prepare_stems_folder:
        stems_folder = Path(args.prepare_stems_folder).resolve()
        if not stems_folder.is_dir():
            raise RuntimeError(f"fixture stems folder not found: {stems_folder}")
        from b2_static_balance_agent_smoke import prepare_stems_fixture

        project_path = artifact_dir / "fixture_project.vit"
        fixture = prepare_stems_fixture(
            args.agent_http,
            stems_folder,
            project_path,
            args.timeout_sec,
            args.dad_timeout_sec,
        )
        fixture["project_path"] = str(project_path)
        fixture["stems_folder"] = str(stems_folder)
        fixture["l3_preflight"] = request_project_l3_preflight(args.agent_http, args.timeout_sec)
        fixture["mom_readiness"] = wait_mom_relationships(args.agent_http, args.timeout_sec)
    elif not args.skip_fixture:
        audio_path = repo / "test_target_3s.wav"
        if not audio_path.is_file():
            raise RuntimeError(f"fixture audio not found: {audio_path}")
        fixture = prepare_fixture(args.agent_http, artifact_dir / "fixture_project.vit", audio_path, args.timeout_sec)

    before = track_pans(args.agent_http, args.timeout_sec)
    conversation_id = f"b3_pan_layout_smoke_{run_stamp}"
    pending = request_json(
        "POST",
        args.agent_http.rstrip("/") + "/agent/chat",
        {
            "conversation_id": conversation_id,
            "message": "请按现代流行风格执行 B3 全工程静态声像布局",
            "context": {"agent_mode": "chat"},
        },
        args.timeout_sec,
    )
    (artifact_dir / "pending_response.json").write_text(
        json.dumps(pending, ensure_ascii=False, indent=2), encoding="utf-8"
    )

    preflight_tools = execution_tools(pending)
    workflow_data = pending.get("workflow_data") if isinstance(pending.get("workflow_data"), dict) else {}
    presentation = (
        workflow_data.get("proposal_presentation")
        if isinstance(workflow_data.get("proposal_presentation"), dict)
        else {}
    )
    actions = presentation.get("actions") if isinstance(presentation.get("actions"), list) else workflow_data.get("actions")
    if not isinstance(actions, list):
        actions = []
    interaction_kinds = [
        str(row.get("kind", ""))
        for row in pending.get("interaction_requests", [])
        if isinstance(row, dict)
    ]
    interaction = next(
        (
            row
            for row in pending.get("interaction_requests", [])
            if isinstance(row, dict)
            and str(row.get("kind", "")) in {"proposal_approval", "pan_layout_confirmation"}
        ),
        None,
    )
    current_runtime = str(pending.get("workflow", "")) == "capability_runtime_v1"
    evidence_refs = {str(value) for value in presentation.get("evidence_refs", [])}
    required_evidence = {"mix.observe", "project.state"}
    legacy_required_preflight = {"mix.observe", "project.state"}
    missing_preflight = sorted(legacy_required_preflight.difference(preflight_tools)) if not current_runtime else []
    forbidden_preflight = sorted(
        {"mix.apply_pan_layout_batch", "mix.apply_tick", "track.pan", "clip.gain.set"}.intersection(preflight_tools)
    )
    runtime_binding_ok = (
        str(workflow_data.get("canary_stage", "")) == "proposal"
        and str(workflow_data.get("capability_id", "")) == "static_mix.pan_layout.v0"
        and str(workflow_data.get("proposal_id", "")).strip()
        and int(workflow_data.get("proposal_revision", 0) or 0) > 0
        and str(workflow_data.get("action_set_hash", "")).strip()
        and required_evidence.issubset(evidence_refs)
    )
    legacy_binding_ok = (
        str(pending.get("workflow", "")) == "pan_layout"
        and str(pending.get("stop_reason", "")) == "needs_confirmation"
        and "pan_layout_confirmation" in interaction_kinds
        and not missing_preflight
    )
    after_pending = track_pans(args.agent_http, args.timeout_sec)
    changed_before_confirmation = [
        track_id for track_id, value in before.items() if abs(after_pending.get(track_id, value) - value) > 0.001
    ]
    pending_ok = (
        pending.get("needs_confirmation") is True
        and isinstance(interaction, dict)
        and len(actions) > 0
        and not forbidden_preflight
        and not changed_before_confirmation
        and (runtime_binding_ok if current_runtime else legacy_binding_ok)
    )
    if not pending_ok:
        raise RuntimeError(
            "B3 did not produce a read-only governed pending pan-layout proposal: "
            + json.dumps(
                {
                    "workflow": pending.get("workflow"),
                    "stop_reason": pending.get("stop_reason"),
                    "needs_confirmation": pending.get("needs_confirmation"),
                    "canary_stage": workflow_data.get("canary_stage"),
                    "capability_id": workflow_data.get("capability_id"),
                    "proposal_id": workflow_data.get("proposal_id"),
                    "proposal_revision": workflow_data.get("proposal_revision"),
                    "action_set_hash": workflow_data.get("action_set_hash"),
                    "action_count": len(actions),
                    "interaction_kinds": interaction_kinds,
                    "preflight_tools": preflight_tools,
                    "missing_preflight": missing_preflight,
                    "forbidden_preflight": forbidden_preflight,
                    "evidence_refs": sorted(evidence_refs),
                    "changed_before_confirmation": changed_before_confirmation,
                    "reply": pending.get("reply"),
                },
                ensure_ascii=False,
            )
        )

    decision = request_json(
        "POST",
        args.agent_http.rstrip("/") + "/agent/interaction/respond",
        {
            "interaction_id": str(interaction.get("id", "")),
            "action_id": "approve",
            "decision": "approve",
            "payload": {},
        },
        args.timeout_sec,
    )
    (artifact_dir / "approve_response.json").write_text(
        json.dumps(decision, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    decision_tools = execution_tools(decision)
    decision_workflow = decision.get("workflow_data") if isinstance(decision.get("workflow_data"), dict) else {}
    if current_runtime:
        verification_result = decision_workflow.get("verification_result")
        verification_status = (
            str(verification_result.get("status", ""))
            if isinstance(verification_result, dict)
            else str(verification_result or "")
        )
        execution_ok = (
            str(decision.get("workflow", "")) == "capability_runtime_v1"
            and str(decision_workflow.get("canary_stage", "")) == "executed_verified"
            and verification_status == "pass"
            and int(decision_workflow.get("receipt_count", 0) or 0) > 0
            and str(decision.get("goal_status", "")) == "completed"
        )
    else:
        execution_ok = (
            decision_tools == ["mix.apply_pan_layout_batch", "project.state", "mix.observe"]
            and str(decision.get("stop_reason", "")) == "pan_layout_applied_verified"
        )
    if not execution_ok:
        raise RuntimeError(
            "B3 approval did not complete through governed execution and verification: "
            + json.dumps(
                {
                    "workflow": decision.get("workflow"),
                    "goal_status": decision.get("goal_status"),
                    "canary_stage": decision_workflow.get("canary_stage"),
                    "execution_status": decision_workflow.get("execution_status"),
                    "verification_result": decision_workflow.get("verification_result"),
                    "receipt_count": decision_workflow.get("receipt_count"),
                    "tools": decision_tools,
                    "stop_reason": decision.get("stop_reason"),
                    "reply": decision.get("reply"),
                },
                ensure_ascii=False,
            )[:4000]
        )

    targets = {
        str(row.get("track_id", "")): float(row.get("target", row.get("target_pan")))
        for row in actions
        if isinstance(row, dict)
        and str(row.get("track_id", "")).strip()
        and isinstance(row.get("target", row.get("target_pan")), (int, float))
    }
    after = track_pans(args.agent_http, args.timeout_sec)
    mismatched = [
        track_id for track_id, target in targets.items() if track_id not in after or abs(after[track_id] - target) > 0.01
    ]
    changed_non_targets = [
        track_id
        for track_id, value in before.items()
        if track_id not in targets and abs(after.get(track_id, value) - value) > 0.001
    ]
    if len(targets) != len(actions) or mismatched or changed_non_targets:
        raise RuntimeError(
            f"B3 pan verification failed: targets={len(targets)}/{len(actions)} "
            f"mismatched={mismatched} changed_non_targets={changed_non_targets}"
        )

    history_messages = (decision.get("project_history") or {}).get("conversation_messages") or []
    if (
        len(history_messages) < 2
        or not isinstance(history_messages[-2], dict)
        or not isinstance(history_messages[-1], dict)
        or str(history_messages[-2].get("role", "")) != "user"
        or str(history_messages[-1].get("role", "")) != "assistant"
        or str(history_messages[-1].get("content", "")) != str(decision.get("reply", ""))
    ):
        raise RuntimeError(
            "B3 confirmation was not appended to project conversation history: "
            + json.dumps(history_messages[-4:], ensure_ascii=False)[:2400]
        )

    project_path = Path(str((fixture or {}).get("project_path", "")))
    reopen_pans: dict[str, float] = {}
    reopen_history: list[Any] = []
    if project_path.is_file():
        invoke_tool(args.agent_http, "project.save", {}, args.timeout_sec, confirmed=True)
        invoke_tool(args.agent_http, "project.new", {}, args.timeout_sec, confirmed=True)
        invoke_tool(
            args.agent_http,
            "project.open",
            {"file_path": str(project_path), "project_path": str(project_path)},
            args.timeout_sec,
            confirmed=True,
        )
        reopen_pans = track_pans(args.agent_http, args.timeout_sec)
        reopen_mismatched = [
            track_id
            for track_id, target in targets.items()
            if track_id not in reopen_pans or abs(reopen_pans[track_id] - target) > 0.01
        ]
        agent_state = request_json("GET", args.agent_http.rstrip("/") + "/agent/state", None, args.timeout_sec)
        (artifact_dir / "reopened_agent_state.json").write_text(
            json.dumps(agent_state, ensure_ascii=False, indent=2), encoding="utf-8"
        )
        project_history = agent_state.get("project_history") if isinstance(agent_state.get("project_history"), dict) else {}
        reopen_history = project_history.get("conversation_messages") if isinstance(project_history.get("conversation_messages"), list) else []
        if (
            reopen_mismatched
            or len(reopen_history) < 2
            or not isinstance(reopen_history[-1], dict)
            or str(reopen_history[-1].get("role", "")) != "assistant"
            or str(reopen_history[-1].get("content", "")) != str(decision.get("reply", ""))
        ):
            raise RuntimeError(
                "B3 save/reopen verification failed: "
                + json.dumps(
                    {
                        "pan_mismatches": reopen_mismatched,
                        "history_tail": reopen_history[-4:],
                        "expected_reply": decision.get("reply"),
                    },
                    ensure_ascii=False,
                )[:3200]
            )

    summary = {
        "schema_version": "b3_pan_layout_agent_smoke.v2",
        "status": "ok",
        "health": health,
        "fixture": fixture,
        "conversation_id": conversation_id,
        "preflight_tools": preflight_tools,
        "workflow": pending.get("workflow"),
        "capability_id": workflow_data.get("capability_id"),
        "proposal_id": workflow_data.get("proposal_id"),
        "proposal_revision": workflow_data.get("proposal_revision"),
        "action_count": len(actions),
        "changed_before_confirmation": changed_before_confirmation,
        "decision_tools": decision_tools,
        "decision_stop_reason": decision.get("stop_reason"),
        "target_mismatches": mismatched,
        "changed_non_targets": changed_non_targets,
        "history_tail_roles": [str(row.get("role", "")) for row in history_messages[-2:] if isinstance(row, dict)],
        "save_reopen_verified": bool(reopen_pans),
        "reopen_target_count": len(targets),
        "reopen_history_tail_roles": [
            str(row.get("role", "")) for row in reopen_history[-2:] if isinstance(row, dict)
        ],
        "artifact_dir": str(artifact_dir),
    }
    (artifact_dir / "summary.json").write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

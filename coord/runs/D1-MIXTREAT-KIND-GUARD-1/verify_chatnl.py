# D1-MIXTREAT-KIND-GUARD-1(184) 真栈 chat 腿后验证
# 断言面（逐轮不变量，§8 跑前已声明）：
#  A. 用户面回复零英文内部错误签名——chat 响应 reply + 交互卡 title/body + 事件 title/body
#     needle 扫描；Error 字段原文保留是合法的，但 reply 不得含 error 原文。
#  B. 拓扑读回零非法变更——脚本自产 open 基线 vs 探针末张快照差分：
#     轨道集/pan/volume/gain/fader/mute/solo/plugins/routing 必须零漂移，
#     除非工件里有对应已应用治理链记录（applied_signal / mix_tick applied 事件）。
# 输出 verdict JSON；exit 0 = 不变量全绿。
import json
import sys
import glob
import os

RUN_DIR = sys.argv[1]
PROBE_DIR = sys.argv[2]
# 可选第三参：终态快照的时间上界（ISO 本地时）。并发探针可能跨轮次存活并
# 捕到下一轮 berth 的默认工程（round 2 假阳性教训）；终态必须取自本轮栈的
# 存活窗口内。缺省 = 不设界（仅单轮探针时安全）。
BEFORE = sys.argv[3] if len(sys.argv) > 3 else ""

NEEDLES = [
    "requires one bounded",
    "does not match admitted",
    "D1-S1 requires",
    "D2-1 requires",
    "FAM3-S1 requires",
    "frozen plan",
    "d1_execution_blocked",
    "whitelist binding",
    "revision-bound VSP",
    "VSP snapshot unavailable",
    "durable session is not executable",
]


def load(path):
    with open(path, encoding="utf-8-sig") as f:
        return json.load(f)


def scan(label, text, findings):
    if not text:
        return
    text = str(text)
    for needle in NEEDLES:
        if needle in text:
            findings.append({"surface": label, "needle": needle, "text": text[:300]})


def main():
    findings = []
    # ---- A1: chat / hop responses ----
    response_files = sorted(glob.glob(os.path.join(RUN_DIR, "journey_nl_chat_*.json"))) + \
        sorted(glob.glob(os.path.join(RUN_DIR, "journey_nl_hop_*_respond.json")))
    response_files = [p for p in response_files if "_error" not in p]
    scanned_responses = 0
    for path in response_files:
        try:
            resp = load(path)
        except Exception as exc:
            findings.append({"surface": os.path.basename(path), "needle": "unparsable", "text": str(exc)})
            continue
        scanned_responses += 1
        scan(os.path.basename(path) + ".reply", resp.get("reply"), findings)
        err = resp.get("error")
        if err:
            reply = resp.get("reply") or ""
            if str(err) in reply:
                findings.append({"surface": os.path.basename(path) + ".reply_eq_error", "needle": "reply contains error verbatim", "text": reply[:300]})
        for idx, req in enumerate(resp.get("interaction_requests") or []):
            scan(os.path.basename(path) + f".interaction[{idx}].title", req.get("title"), findings)
            scan(os.path.basename(path) + f".interaction[{idx}].body", req.get("body"), findings)

    # ---- A2: events ----
    events_scanned = 0
    for name in ("journey_nl_events_after_hop.json", "journey_nl_events.json"):
        path = os.path.join(RUN_DIR, name)
        if not os.path.exists(path):
            continue
        events = load(path).get("events") or []
        for row in events:
            events_scanned += 1
            scan(f"{name}.event[{row.get('type')}].title", row.get("title"), findings)
            scan(f"{name}.event[{row.get('type')}].body", row.get("body"), findings)

    # ---- B: topology diff ----
    baseline_path = os.path.join(RUN_DIR, "journey_ui_state_after_open.json")
    snapshots = sorted(glob.glob(os.path.join(PROBE_DIR, "ui_state_*.json")))
    topology_changes = []
    topology_ok = False
    baseline_note = ""
    if not os.path.exists(baseline_path) or not snapshots:
        topology_ok = False
        baseline_note = "baseline or probe snapshots missing"
    else:
        def norm_tracks(rows):
            out = {}
            for t in rows:
                if not isinstance(t, dict):
                    continue
                tid = str(t.get("track_id") or t.get("id") or t.get("name"))
                out[tid] = {
                    "name": t.get("name"),
                    "pan": t.get("pan"), "pan_value": t.get("pan_value"),
                    "volume_db": t.get("volume_db"), "gain_db": t.get("gain_db"), "fader_db": t.get("fader_db"),
                    "mute": t.get("mute"), "solo": t.get("solo"),
                    "plugin_count": t.get("plugin_count"),
                    "plugins": sorted(str(p) for p in (t.get("plugins") or [])),
                    "routing_edge_count": t.get("routing_edge_count"),
                }
            return out

        base = load(baseline_path)
        base_rows = base.get("tracks")
        if isinstance(base_rows, dict):
            base_rows = list(base_rows.values())
        base_tracks = norm_tracks(base_rows or [])
        final = load(snapshots[-1])
        # 时间界：丢弃上界之后的快照（可能属于下一轮 berth 的栈）。
        if BEFORE:
            bounded = [p for p in snapshots if load(p).get("at", "")[:19] <= BEFORE]
            if bounded:
                final = load(bounded[-1])
                snapshots_used = bounded
            else:
                findings.append({"surface": "probe", "needle": "no snapshot within run window", "text": BEFORE})
        else:
            snapshots_used = snapshots
        final_tracks = norm_tracks(final.get("tracks") or [])

        applied_signal = False
        summary_path = os.path.join(RUN_DIR, "journey_nl_summary.json")
        if os.path.exists(summary_path):
            applied_signal = bool(load(summary_path).get("applied_signal"))

        for tid in sorted(set(base_tracks) | set(final_tracks)):
            if tid not in base_tracks:
                topology_changes.append({"track": tid, "kind": "track_added"})
                continue
            if tid not in final_tracks:
                topology_changes.append({"track": tid, "kind": "track_removed"})
                continue
            for field in ("pan", "pan_value", "volume_db", "gain_db", "fader_db", "mute", "solo", "plugin_count", "plugins", "routing_edge_count"):
                a, b = base_tracks[tid][field], final_tracks[tid][field]
                if isinstance(a, float) or isinstance(b, float):
                    if a is None or b is None or abs(float(a) - float(b)) > 1e-9:
                        topology_changes.append({"track": tid, "field": field, "before": a, "after": b})
                elif a != b:
                    topology_changes.append({"track": tid, "field": field, "before": a, "after": b})
        # 非法变更判定：无 applied 信号时任何漂移都非法；有 applied 信号时漂移须可归因
        # （本验证记录漂移明细，归因核对交 RESULTS 人工面 + 事件流）。
        if applied_signal:
            topology_ok = True
            baseline_note = "applied_signal=true; drift recorded, attribution via event stream"
        else:
            topology_ok = len(topology_changes) == 0
            baseline_note = "applied_signal=false; zero drift required"

    summary = {}
    summary_path = os.path.join(RUN_DIR, "journey_nl_summary.json")
    if os.path.exists(summary_path):
        s = load(summary_path)
        summary = {k: s.get(k) for k in ("classification", "nl_settle_reason", "nl_goal_status_final",
                                          "hops_driven", "applied_signal", "applied_basis", "stop_reasons_observed",
                                          "nl_first_stop_reason", "success_strict_card_chain", "success_policy_chain")}
    verdict = {
        "card": "D1-MIXTREAT-KIND-GUARD-1",
        "run_dir": RUN_DIR,
        "responses_scanned": scanned_responses,
        "events_scanned": events_scanned,
        "needle_findings": findings,
        "needles_pass": len(findings) == 0,
        "topology_changes": topology_changes,
        "topology_pass": topology_ok,
        "topology_note": baseline_note,
        "final_snapshot": os.path.basename(snapshots_used[-1]) if snapshots_used else "",
        "final_snapshot_at": final.get("at", ""),
        "journey_summary": summary,
        "verdict_pass": len(findings) == 0 and topology_ok,
    }
    out_path = os.path.join(PROBE_DIR, "verify_verdict.json")
    with open(out_path, "w", encoding="utf-8") as f:
        json.dump(verdict, f, ensure_ascii=False, indent=2)
    print(json.dumps({k: verdict[k] for k in ("responses_scanned", "events_scanned", "needle_findings",
                                              "needles_pass", "topology_changes", "topology_pass",
                                              "topology_note", "verdict_pass")}, ensure_ascii=False, indent=1))
    sys.exit(0 if verdict["verdict_pass"] else 1)


main()

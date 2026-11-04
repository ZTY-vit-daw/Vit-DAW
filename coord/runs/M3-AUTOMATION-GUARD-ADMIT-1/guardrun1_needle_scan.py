# 246 M3-TERMINAL-ROUTE-WORDING-1 wordingrun1 needle scan (independent read-back)
# Proves (a) the four new sentences physically reached the request messages,
# (b) the replaced two-choice exact shape is gone, (c) the shape example
# reached the payload, and (d) usage totals for the billing disclosure.
import json
import sys

CAPTURE = sys.argv[1] if len(sys.argv) > 1 else r"D:/Vit_DAW_wt_246impl/coord/runs/M3-TERMINAL-ROUTE-WORDING-1/wordingrun1/logs/agent_llm_payload_capture.jsonl"
TELEMETRY = sys.argv[2] if len(sys.argv) > 2 else r"D:/Vit_DAW_wt_246impl/coord/runs/M3-TERMINAL-ROUTE-WORDING-1/wordingrun1/logs/agent_llm_telemetry.jsonl"

NEEDLES = {
    "terminal_three_choice": "a direct automation.set_points mutation call when every write element",
    "time_varying_exemption": "a time-varying goal is exempt from it",
    "shape_example_set_points": '"tool":"automation.set_points"',
    "shape_example_readback": '"final":true,"reply":"short readback note"',
    "confirmation_bridge": "The confirmation card that the runtime gate raises for the direct call is itself where the time range and point values get checked",
    "v3_title": "Action Path Selection (time-varying goals vs static adjustments)",
}
OLD_TWOCHOICE = "a bounded improvement proposal, or an explicit terminal statement"

rows = 0
hits = {k: 0 for k in NEEDLES}
old_hits = 0
terminal_turn_rows = 0
try:
    with open(CAPTURE, encoding="utf-8", errors="replace") as fh:
        for line in fh:
            line = line.strip()
            if not line:
                continue
            try:
                row = json.loads(line)
            except json.JSONDecodeError:
                continue
            rows += 1
            messages = (row.get("request") or {}).get("messages") or []
            contents = [str(m.get("content") or "") for m in messages]
            if any("TERMINAL TURN (mechanical runtime state)" in c for c in contents):
                terminal_turn_rows += 1
            for key, needle in NEEDLES.items():
                if any(needle in c for c in contents):
                    hits[key] += 1
            if any(OLD_TWOCHOICE in c for c in contents):
                old_hits += 1
except FileNotFoundError:
    print(json.dumps({"error": "payload capture not found", "path": CAPTURE}, ensure_ascii=False))
    sys.exit(1)

calls = 0
tin = tout = 0
models = {}
try:
    with open(TELEMETRY, encoding="utf-8", errors="replace") as fh:
        for line in fh:
            line = line.strip()
            if not line:
                continue
            try:
                row = json.loads(line)
            except json.JSONDecodeError:
                continue
            calls += 1
            usage = row.get("usage") or {}
            tin += int(usage.get("input_tokens") or 0)
            tout += int(usage.get("output_tokens") or 0)
            model = str(row.get("model") or "unknown")
            models[model] = models.get(model, 0) + 1
except FileNotFoundError:
    pass

print(json.dumps({
    "rows": rows,
    "rows_with_terminal_turn_directive": terminal_turn_rows,
    "needle_hits": hits,
    "old_twochoice_rows": old_hits,
    "telemetry": {"calls": calls, "input_tokens": tin, "output_tokens": tout, "models": models},
}, ensure_ascii=False, indent=2))

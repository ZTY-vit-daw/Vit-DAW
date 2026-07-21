#!/usr/bin/env python3
"""Smoke test for the 7 EQ grabber actions against a live FabFilter Pro-Q 3
instance, exercised through VitAgent's plugin_grabber_apply_control tool
(no SPAL / provider credential involved).

Usage:
    python scripts/eq_7actions_grabber_smoke.py --track-id 1007 --plugin-id 1015
"""
import argparse
import json
import sys
import urllib.request

AGENT_URL = "http://127.0.0.1:7878/agent/invoke"

ACTIONS = [
    {
        "name": "1_low_end_mud_bell_cut",
        "control": "eq.cut_region",
        "target": {"freq_hz": 250, "gain_db": -3, "q": 1.4, "type": "Bell"},
        "expect_slots": {"type", "frequency", "gain", "q"},
    },
    {
        "name": "2_vocal_rumble_low_cut",
        "control": "eq.cut_region",
        "target": {"freq_hz": 100, "type": "Low Cut", "enabled": 1},
        "expect_slots": {"type", "frequency", "enable"},
    },
    {
        "name": "3_air_high_shelf_boost",
        "control": "eq.boost_region",
        "target": {"freq_hz": 8000, "gain_db": 2, "type": "High Shelf"},
        "expect_slots": {"type", "frequency", "gain"},
    },
    {
        "name": "4_dynamic_notch",
        "control": "eq.cut_region",
        "target": {"freq_hz": 6000, "type": "Bell", "dyn_enable": 1, "gain_db": -4},
        "expect_slots": {"type", "frequency", "gain", "dyn_enable"},
    },
    {
        "name": "5_resonance_notch",
        "control": "eq.cut_region",
        "target": {"freq_hz": 3200, "q": 9, "type": "Notch"},
        "expect_slots": {"type", "frequency", "q"},
    },
    {
        "name": "6_disable_band",
        "control": "eq.cut_region",
        "target": {"freq_hz": 250, "enabled": 0},
        "expect_slots": {"frequency", "enable"},
        "expect_text": {"enable": "Disabled"},
    },
    {
        "name": "7_stereo_placement",
        "not_applicable": True,
        "reason": "profile has no stereo_placement slot on any EQ band group yet",
    },
]


def invoke(tool, args):
    payload = json.dumps({"tool": tool, "args": args, "source": "smoke"}).encode("utf-8")
    req = urllib.request.Request(AGENT_URL, data=payload, headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=10) as resp:
        return json.loads(resp.read().decode("utf-8"))


def run(track_id, plugin_id):
    failures = []
    for action in ACTIONS:
        if action.get("not_applicable"):
            print(f"[skip] {action['name']}: {action['reason']}")
            continue

        args = {
            "track_id": track_id,
            "plugin_id": plugin_id,
            "control": action["control"],
            "target": action["target"],
        }
        resp = invoke("plugin_grabber_apply_control", args)
        if resp.get("status") != "ok":
            failures.append((action["name"], resp.get("error", "unknown error")))
            print(f"[FAIL] {action['name']}: {resp.get('error')}")
            continue

        applied = {p["slot"]: p for p in resp["result"].get("applied_parameters", [])}
        missing = action["expect_slots"] - applied.keys()
        if missing:
            failures.append((action["name"], f"missing slots: {missing}"))
            print(f"[FAIL] {action['name']}: missing slots {missing}")
            continue

        text_ok = True
        for slot, expected_text in action.get("expect_text", {}).items():
            actual = applied[slot].get("new_value_text", "")
            if expected_text.lower() not in actual.lower():
                text_ok = False
                failures.append((action["name"], f"{slot} text mismatch: {actual!r} vs {expected_text!r}"))
                print(f"[FAIL] {action['name']}: {slot} text {actual!r} != {expected_text!r}")

        if text_ok:
            summary = ", ".join(f"{s}={applied[s]['new_value_text']}" for s in sorted(action["expect_slots"]))
            print(f"[ok]   {action['name']}: {summary}")

    print()
    if failures:
        print(f"{len(failures)} action(s) failed:")
        for name, reason in failures:
            print(f"  - {name}: {reason}")
        return 1

    print("All applicable EQ grabber actions passed.")
    return 0


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--track-id", required=True)
    parser.add_argument("--plugin-id", required=True)
    args = parser.parse_args()
    sys.exit(run(args.track_id, args.plugin_id))


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""E1 extraction for LOUDNESS-B-TRIGGER (design 06 §3): reads the smoke run's
loudness_watch_verdicts.json evidence copy and prints the settle-trigger cost
samples + fresh-row reuse accounting. Read-only."""
import json
import sys
from pathlib import Path


def main(path: str) -> int:
    store = json.loads(Path(path).read_text(encoding="utf-8-sig"))
    records = store.get("verdicts", [])
    print(f"schema={store.get('schema_version')} checks_total={store.get('checks_total')} records={len(records)}")
    settle_samples = [r for r in records if r.get("trigger_kind") in ("settle", "project_change")]
    ride_samples = [r for r in records if r.get("trigger_kind") == "observation_piggyback"]
    for label, samples in (("settle/project_change", settle_samples), ("observation_piggyback", ride_samples)):
        if not samples:
            print(f"[{label}] no samples")
            continue
        durations = sorted(r.get("check_duration_ms", 0.0) for r in samples)
        bakes = sum(r.get("bakes_issued", 0) for r in samples)
        fresh = sum(r.get("fresh_rows", 0) for r in samples)
        stale = sum(r.get("stale_rows", 0) for r in samples)
        reuse = fresh / (fresh + stale) if (fresh + stale) > 0 else 0.0
        mid = durations[len(durations) // 2]
        print(
            f"[{label}] n={len(samples)} duration_ms median={mid:.3f} max={durations[-1]:.3f} "
            f"bakes_total={bakes} fresh_rows_total={fresh} stale_rows_total={stale} fresh_reuse_ratio={reuse:.2f}"
        )
    for r in records:
        v = r.get("verdict", {})
        print(
            f"  key={r.get('key')} kind={r.get('verdict_kind')} trigger={r.get('trigger_kind')} "
            f"recorded={r.get('recorded_at')} rechecks={r.get('recheck_count', 0)} "
            f"dur_ms={r.get('check_duration_ms')} bakes={r.get('bakes_issued')} fresh={r.get('fresh_rows')} stale={r.get('stale_rows')} "
            f"reason={v.get('unmeasurable_reason', '')[:60]}"
        )
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1]))

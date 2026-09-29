# RUNNER-GATE-ADAPT-1 measurement pass: extract the FULL per-track metric rows
# for every pv1 case (RMS / peak / crest + sibilance dispersion + transient
# contrast), including the six cases the current spv1-calibrated gate rejects
# before the sub-qualifiers' rows can be observed.
#
# Method: the runner's own functions are imported and called with the gate
# FLOORS neutralized (module constants are read at call time). The per-track
# rows are pure measurements independent of the floors; only the echoed
# "gates" dict in the output reflects the neutralized values, so this file's
# output must NOT be read as a gate verdict. Gate verdicts for the derived
# pv1 profile are produced by the separate unpatched verification pass
# (verify_profiles.py) against the implemented code.
#
# Read-only with respect to both fixture stores (pv1 material repo, spv1
# temp fixtures); writes only this artifact directory.

import json
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parents[4]
sys.path.insert(0, str(REPO / "scripts"))
import free_state_d1_smoke as runner  # noqa: E402

PV1_MANIFEST = Path(r"C:\Users\timoz\Documents\毕业设计\experiments\out\suite_v1_lm_runner\fixture_manifest.json")
OUT = Path(__file__).resolve().parent / "pv1_all_metrics.json"

NEUTRALIZED = {
    "MATERIAL_MIN_RMS_DBFS": -1e9,
    "MATERIAL_MIN_CREST_DB_PER_TRACK": -1e9,
    "MATERIAL_MIN_BEST_CREST_DB": -1e9,
    "SIBILANCE_MIN_CONTRAST_DB_PER_TRACK": -1e9,
    "SIBILANCE_MIN_BEST_CONTRAST_DB": -1e9,
    "TRANSIENT_MIN_CONTRAST_DB_PER_TRACK": -1e9,
    "TRANSIENT_MIN_BEST_CONTRAST_DB": -1e9,
}


def main() -> int:
    for name, value in NEUTRALIZED.items():
        setattr(runner, name, value)
    manifest = json.loads(PV1_MANIFEST.read_text(encoding="utf-8-sig"))
    cases = {}
    for row in manifest["cases"]:
        case_id = row["public_case_id"]
        _manifest, case = runner.load_public_case(PV1_MANIFEST, case_id)
        result = runner.qualify_material(case)  # neutral flavor: no flavor-scoped sub-gates
        cases[case_id] = {
            "compression_rows": [
                {key: track[key] for key in ("track", "rms_dbfs", "peak_dbfs", "crest_db")}
                for track in result["tracks"]
            ],
            "best_crest_db": result["best_crest_db"],
            "sibilance_rows": [
                {key: track[key] for key in ("track", "contrast_p95_p50_db")}
                for track in result["sibilance_band"]["tracks"]
            ],
            "best_sibilance_contrast_db": result["sibilance_band"]["best_contrast_p95_p50_db"],
            "transient_rows": [
                {key: track[key] for key in ("track", "contrast_short_peak_long_median_db")}
                for track in result["transient_window"]["tracks"]
            ],
            "best_transient_contrast_db": result["transient_window"]["best_contrast_short_peak_long_median_db"],
        }
    payload = {
        "method": "runner's own qualify functions with gate floors neutralized for row extraction (see header); not a gate verdict",
        "manifest": str(PV1_MANIFEST),
        "cases": cases,
    }
    OUT.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"measured {len(cases)} cases -> {OUT}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

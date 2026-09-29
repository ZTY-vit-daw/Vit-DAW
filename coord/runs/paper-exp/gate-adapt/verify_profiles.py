# RUNNER-GATE-ADAPT-1 verification pass (GATE_DERIVATION.md section 6 matrix),
# run against the IMPLEMENTED profile-scoped gate in scripts/free_state_d1_
# smoke.py. Four checks:
#   1. pv1 seven cases, manifest-declared profile (pv1_real_stems) -> PASS;
#   2. spv1 nine cases, undeclared manifest -> default spv1 caliber -> PASS
#      (zero-behavior-change regression);
#   3. counterexample sets A (digital-silence stem) / B (hard-clipped stem) /
#      C (stationary-noise set) under pv1_real_stems -> REJECTED;
#   4. unknown profile name -> fail-closed ValueError.
# Counterexample wavs are generated under this artifact directory only; both
# fixture stores are read strictly read-only.

import json
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parents[4]
sys.path.insert(0, str(REPO / "scripts"))
import free_state_d1_smoke as runner  # noqa: E402

PV1_MANIFEST = Path(r"C:\Users\timoz\Documents\毕业设计\experiments\out\suite_v1_lm_runner\fixture_manifest.json")
SPV1_MANIFEST = Path(r"D:\Vit_DAW\temp\semantic-processor-agent-project-smoke-v1\fixtures\semantic_processor_project_smoke_v1_80085263a651cf20\fixture_manifest.json")
OUT_DIR = Path(__file__).resolve().parent
CX_DIR = OUT_DIR / "counterexamples"


def profile_for(manifest_path: Path) -> str:
    manifest = json.loads(manifest_path.read_text(encoding="utf-8-sig"))
    return str(manifest.get("qualification_profile", runner.DEFAULT_MATERIAL_QUALIFICATION_PROFILE))


def qualify_case(manifest_path: Path, case_id: str, profile: str) -> dict:
    _manifest, case = runner.load_public_case(manifest_path, case_id)
    return runner.qualify_material(case, profile=profile)


def build_counterexamples(base_case: dict) -> dict[str, dict]:
    import numpy as np
    import soundfile as sf

    CX_DIR.mkdir(parents=True, exist_ok=True)
    stems = {row["track"]: Path(str(row["file"])) for row in base_case["stem_files"]}
    cases: dict[str, dict] = {}

    # A: digital-silence stem among real stems (guitar -> exact zeros).
    silence_path = CX_DIR / "cx_a_guitar_silence.wav"
    sf.write(silence_path, np.zeros((44100 * 20, 2), dtype=np.float64), 44100, subtype="PCM_16")
    case_a = {"stem_files": [dict(row) for row in base_case["stem_files"]]}
    for row in case_a["stem_files"]:
        if row["track"] == "guitar":
            row["file"] = str(silence_path)
    cases["A_digital_silence_stem"] = case_a

    # B: hard-clipped stem (guitar amplified +48 dB then clipped at +/-1.0;
    # ~36% of samples squared, crest ~4 dB). Moderate clipping is NOT caught
    # by the crest floor (a +24 dB clip leaves crest ~9, +30 dB ~6.5 -- the
    # floor's honest boundary, see GATE_DERIVATION.md 3.2); this construct is
    # unambiguous clipping damage below the floor.
    audio, rate = sf.read(stems["guitar"], dtype="float64", always_2d=True)
    clip_path = CX_DIR / "cx_b_guitar_hardclip.wav"
    sf.write(clip_path, np.clip(audio * (10.0 ** (48.0 / 20.0)), -1.0, 1.0), rate, subtype="PCM_16")
    case_b = {"stem_files": [dict(row) for row in base_case["stem_files"]]}
    for row in case_b["stem_files"]:
        if row["track"] == "guitar":
            row["file"] = str(clip_path)
    cases["B_hard_clipped_stem"] = case_b

    # C: stationary-noise set (all six stems -> white noise at -20 dBFS).
    case_c = {"stem_files": [dict(row) for row in base_case["stem_files"]]}
    rng = np.random.default_rng(20260929)
    for row in case_c["stem_files"]:
        noise_path = CX_DIR / f"cx_c_{row['track']}_noise.wav"
        sf.write(noise_path, rng.normal(0.0, 10.0 ** (-20.0 / 20.0), size=(44100 * 20, 2)), 44100, subtype="PCM_16")
        row["file"] = str(noise_path)
    cases["C_stationary_noise_set"] = case_c
    return cases


def main() -> int:
    results: dict[str, object] = {"executed_at": __import__("datetime").datetime.now(__import__("datetime").timezone.utc).isoformat()}

    pv1_profile = profile_for(PV1_MANIFEST)
    spv1_profile = profile_for(SPV1_MANIFEST)
    results["manifest_profiles"] = {"pv1": pv1_profile, "spv1": spv1_profile}
    assert pv1_profile == "pv1_real_stems", "pv1 manifest must declare pv1_real_stems"
    assert spv1_profile == "spv1_synthetic", "spv1 manifest is undeclared and must default to spv1_synthetic"

    pv1_cases = sorted(row["public_case_id"] for row in json.loads(PV1_MANIFEST.read_text(encoding="utf-8-sig"))["cases"])
    pv1_out: dict[str, object] = {}
    for case_id in pv1_cases:
        result = qualify_case(PV1_MANIFEST, case_id, pv1_profile)
        pv1_out[case_id] = {
            "verdict": "PASS",
            "gates": result["gates"],
            "best_crest_db": result["best_crest_db"],
            "best_sibilance_contrast_db": result["sibilance_band"]["best_contrast_p95_p50_db"],
            "best_transient_contrast_db": result["transient_window"]["best_contrast_short_peak_long_median_db"],
        }
    results["pv1_declared_profile"] = pv1_out

    spv1_cases = sorted(row["public_case_id"] for row in json.loads(SPV1_MANIFEST.read_text(encoding="utf-8-sig"))["cases"])
    spv1_out: dict[str, object] = {}
    spv1_skipped: dict[str, str] = {}
    for case_id in spv1_cases:
        # The local spv1 temp fixture never carried a p02 project (preflight
        # receipt records project_count=2 for its original two-case form; the
        # store grew cases without rebuilding every project). Pre-existing
        # fixture state, recorded rather than repaired (AGENTS 10: no writes
        # to authoritative fixtures).
        project_path = next(
            Path(str(row["project_path"]))
            for row in json.loads(SPV1_MANIFEST.read_text(encoding="utf-8-sig"))["cases"]
            if row["public_case_id"] == case_id
        )
        if not project_path.is_file():
            spv1_skipped[case_id] = f"missing local fixture project: {project_path}"
            continue
        result = qualify_case(SPV1_MANIFEST, case_id, spv1_profile)
        spv1_out[case_id] = {
            "verdict": "PASS",
            "gates": result["gates"],
            "best_crest_db": result["best_crest_db"],
            "best_sibilance_contrast_db": result["sibilance_band"]["best_contrast_p95_p50_db"],
            "best_transient_contrast_db": result["transient_window"]["best_contrast_short_peak_long_median_db"],
        }
    results["spv1_default_regression"] = spv1_out
    results["spv1_skipped_missing_project"] = spv1_skipped

    _manifest, base_case = runner.load_public_case(PV1_MANIFEST, "pv1_p05")
    cx_out: dict[str, object] = {}
    for name, case in build_counterexamples(base_case).items():
        try:
            runner.qualify_material(case, profile="pv1_real_stems")
        except RuntimeError as error:
            cx_out[name] = {"verdict": "REJECTED", "error": str(error)}
        else:
            cx_out[name] = {"verdict": "ACCEPTED (UNEXPECTED)"}
    results["counterexamples_pv1_profile"] = cx_out

    try:
        runner.qualify_material(base_case, profile="no_such_profile")
    except ValueError as error:
        results["unknown_profile_fail_closed"] = {"verdict": "REJECTED", "error": str(error)}
    else:
        results["unknown_profile_fail_closed"] = {"verdict": "ACCEPTED (UNEXPECTED)"}

    # Backward-compat discriminator: pv1_p01 (the RMS-failing case) under the
    # DEFAULT (undeclared-manifest) spv1 caliber must still be rejected -- the
    # pv1 pass above is selected by the manifest declaration, not by a silent
    # global relaxation.
    _m01, case_p01 = runner.load_public_case(PV1_MANIFEST, "pv1_p01")
    try:
        runner.qualify_material(case_p01, profile=None)
    except RuntimeError as error:
        results["pv1_case_under_default_spv1_caliber"] = {"verdict": "REJECTED", "error": str(error)}
    else:
        results["pv1_case_under_default_spv1_caliber"] = {"verdict": "ACCEPTED (UNEXPECTED)"}

    verdicts = (
        [row["verdict"] for row in pv1_out.values()]
        + [row["verdict"] for row in spv1_out.values()]
        + [row["verdict"] for row in cx_out.values()]
        + [results["unknown_profile_fail_closed"]["verdict"]]
        + [results["pv1_case_under_default_spv1_caliber"]["verdict"]]
    )
    results["summary"] = {
        "pv1_pass": sum(1 for v in (row["verdict"] for row in pv1_out.values()) if v == "PASS"),
        "pv1_total": len(pv1_out),
        "spv1_pass": sum(1 for v in (row["verdict"] for row in spv1_out.values()) if v == "PASS"),
        "spv1_total": len(spv1_out),
        "counterexamples_rejected": sum(1 for row in cx_out.values() if row["verdict"] == "REJECTED"),
        "counterexamples_total": len(cx_out),
        "fail_closed_ok": results["unknown_profile_fail_closed"]["verdict"] == "REJECTED",
    }
    (OUT_DIR / "verify_profiles_results.json").write_text(json.dumps(results, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(results["summary"], ensure_ascii=False))
    ok = all(v in {"PASS", "REJECTED"} for v in verdicts)
    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main())

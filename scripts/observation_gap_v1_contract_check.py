#!/usr/bin/env python3
"""Validate the observation-gap branch contract (Graph Context v1).

This is a Python contract/preflight check for the observation-gap fixes. It
asserts the DAD/assembly/projection contract at the source level and cross-
references the Go unit tests that verify behavior:

- L3 (DAD) exposes bit_depth/bits_per_sample, per-band
  persistence_ratio/active_frame_ratio, and transient_events window_ms/hop_ms;
- COM source-only consumes DAD frame-level transient events (no new projection
  layer, micro-transient stays honest about FFT-frame resolution);
- the general frequency observation stays on a uniform source-file tap and
  never mixes post-fader L2 render probes into it;
- masking stays deferred but its disclosure distinguishes a data-layer gap
  from a projection gap.

The check is evaluator-side: it reads source files and the manifest only and
never calls the Agent, builds fixtures, or mutates a DAW project.
"""

from __future__ import annotations

import argparse
import re
from pathlib import Path
from typing import Any

REQUIRED_GO_TESTS = (
    "com.TestSourceOnlyMicroTransientFromFrameEvidence",
    "mixboard.TestCOMSourceOnlyConsumesDADTransientEvents",
    "mixboard.TestAssembleFrequencyContextDoesNotMixPartialL2IntoL3Observation",
)


def require(condition: bool, message: str) -> None:
    if not condition:
        raise AssertionError(message)


def check_l3(repo_root: Path) -> None:
    path = repo_root / "VitApp" / "Source" / "Service" / "L3AcousticAnalyzer.cpp"
    require(path.is_file(), f"L3 analyzer missing: {path}")
    source = path.read_text(encoding="utf-8")
    for marker in ("reader->bitsPerSample", '"bit_depth"', '"bits_per_sample"'):
        require(marker in source, f"L3 missing PCM bit depth output: {marker}")
    for marker in ('"persistence_ratio"', '"active_frame_ratio"'):
        require(marker in source, f"L3 missing band persistence output: {marker}")
    for marker in ('"window_ms"', '"hop_ms"'):
        require(marker in source, f"L3 transient block missing resolution: {marker}")
    require("bandPersistenceRatio" in source, "L3 bandPersistenceRatio helper missing")
    # The persistence threshold contract: local noise floor + 3 dB contrast.
    require("+ 3.0" in source, "L3 persistence threshold contract missing (+3dB)")


def check_com(repo_root: Path) -> None:
    types_path = repo_root / "agent" / "internal" / "com" / "types.go"
    projection_path = repo_root / "agent" / "internal" / "com" / "projection.go"
    require(types_path.is_file() and projection_path.is_file(), "COM package missing")
    types = types_path.read_text(encoding="utf-8")
    projection = projection_path.read_text(encoding="utf-8")
    require("TransientEvents *TransientEventEvidence" in types, "COM source transient channel missing")
    require('json:"transient_events' in types, "COM transient JSON tag missing")
    require("transientScaleStatus" in projection, "COM micro-transient scale mapping missing")
    require("frame_level_fft_transient_resolution" in projection, "COM honest FFT-frame reason missing")
    require("sourceTransientSummary" in projection, "COM micro-transient statistics derivation missing")
    # Frame-level evidence must never be promoted to sample-accurate readiness.
    require('return StatusPartial' in projection, "COM micro-transient readiness over-promise")


def check_mixboard(repo_root: Path) -> None:
    mixboard_path = repo_root / "agent" / "internal" / "mixboard" / "mixboard.go"
    com_proj_path = repo_root / "agent" / "internal" / "mixboard" / "com_projection.go"
    require(mixboard_path.is_file() and com_proj_path.is_file(), "mixboard package missing")
    mixboard = mixboard_path.read_text(encoding="utf-8")
    require("TransientEvents map[string]any" in mixboard, "featureSnapshot transient_events field missing")
    require('json:"transient_events' in mixboard, "featureSnapshot compact output missing transient_events")
    require("hasAnyL3" in mixboard, "L3 unification partial-coverage logic missing")
    com_proj = com_proj_path.read_text(encoding="utf-8")
    require("comTransientEvidence" in com_proj, "mixboard COM transient assembly missing")


def check_masking_disclosure(repo_root: Path) -> None:
    path = repo_root / "agent" / "internal" / "capabilitycontext" / "free_state_observation.go"
    require(path.is_file(), "CCB capability context missing")
    source = path.read_text(encoding="utf-8")
    require("mix.masking_relationship" in source, "CCB masking view missing")
    require("data-layer gap, not a projection gap" in source, "CCB masking disclosure does not classify the gap")


def check_manifest(repo_root: Path) -> None:
    manifest = repo_root / "docs" / "OBSERVATION_PROJECTION_MANIFEST.md"
    require(manifest.is_file(), "projection manifest missing")
    content = manifest.read_text(encoding="utf-8")
    require("RLM" in content and "LLMContext" in content, "manifest lost RLM LLMContext gap note")
    require("persistence_ratio" in content or "persistence" in content.lower(), "manifest missing persistence gap note")


def check_go_tests(repo_root: Path) -> None:
    # Behavioral verification lives in Go unit tests; the Python preflight only
    # asserts the test names exist so a renamed/removed contract test is
    # caught at contract-check time.
    roots = [
        repo_root / "agent" / "internal" / "com",
        repo_root / "agent" / "internal" / "mixboard",
    ]
    sources = "".join(
        (path.read_text(encoding="utf-8") if path.is_file() else "")
        for root in roots
        for path in root.glob("*_test.go")
    )
    for test in REQUIRED_GO_TESTS:
        function_name = test.split(".")[-1]
        require(
            f"func {function_name}(" in sources,
            f"required Go contract test missing: {test}",
        )


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--repo-root",
        type=Path,
        default=Path(__file__).resolve().parents[1],
        help="repository root (default: parent of scripts/)",
    )
    args = parser.parse_args()
    root = args.repo_root.resolve()
    check_l3(root)
    check_com(root)
    check_mixboard(root)
    check_masking_disclosure(root)
    check_manifest(root)
    check_go_tests(root)
    print(f"observation_gap_v1 contract check passed (repo_root={root})")


if __name__ == "__main__":
    main()

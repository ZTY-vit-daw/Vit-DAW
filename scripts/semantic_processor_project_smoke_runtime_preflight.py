#!/usr/bin/env python3
"""Run generic runtime acceptance probes and emit sealed-safe receipts.

The probes exercise product-owned CCB, free-state, PCA, typed-controller and
L2 boundaries. They do not open smoke truth or send an Agent request.
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import subprocess
import time
from pathlib import Path
from typing import Any


def now_iso() -> str:
    return dt.datetime.now(dt.timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def run_probe(repo_root: Path, name: str, package: str, pattern: str) -> dict[str, Any]:
    started = time.monotonic()
    process = subprocess.run(
        ["go", "test", package, "-run", pattern, "-count=1"],
        cwd=repo_root / "agent",
        text=True,
        capture_output=True,
        timeout=300,
        check=False,
    )
    output = (process.stdout + "\n" + process.stderr).strip()
    return {
        "name": name,
        "package": package,
        "run_pattern": pattern,
        "status": "passed" if process.returncode == 0 else "blocked",
        "returncode": process.returncode,
        "elapsed_seconds": round(time.monotonic() - started, 3),
        "output_tail": output[-4000:],
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo-root", default=str(Path(__file__).resolve().parent.parent))
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    repo_root = Path(args.repo_root).resolve()
    probes = [
        run_probe(repo_root, "ccb_catalog_neutral", "./internal/capabilitycontext", "Test(FreeStateObservationCatalogIsSemanticAndBounded|DOMViewsAreNeutralExplicitAndDimensionScoped)$"),
        run_probe(repo_root, "view_set_exact_match", "./internal/agentloop", "TestFreeStateObservationRequestViewSetMustExactlyMatchModelDecision$"),
        run_probe(repo_root, "ccb_receipt_exact_scope", "./internal/harness", "TestCCBObservationReceiptRecordsModelRequestAndScope$"),
        run_probe(repo_root, "pca_family_matrix", "./internal/chat", "Test(DynamicFamilyPCAQualificationMatrixCoversAllFiveFamilies|DynamicFamilyPostLoadQualificationMatrixRechecksPCAAndTopology)$"),
        run_probe(repo_root, "pca_v1_family_matrix", "./internal/processorattestation", "TestQueryInstalledEligibilityV1AndV2Families$"),
        run_probe(repo_root, "typed_controller_roundtrip", "./internal/chat", "Test(SemanticDynamicControllerReceiptRequiresAtomicReadbackAndRestore|Apply.*ControlsRestoresFullPreimageOnFailure)$"),
        run_probe(repo_root, "l2_before_after", "./internal/harness", "Test(L2ProbeCacheRequiresExactTrackSignalFingerprint|CollectL2RenderProbeBatchUsesExactCacheWithoutRender|SingleTargetL2RequestPreservesProjectL3HistoryRows)$"),
    ]
    by_name = {row["name"]: row for row in probes}
    result = {
        "schema_version": "semantic_processor_project_smoke_runtime_preflight.v1",
        "generated_at": now_iso(),
        "sealed_truth_opened": False,
        "agent_started": False,
        "ccb_catalog_neutral": by_name["ccb_catalog_neutral"]["status"] == "passed",
        "view_set_exact_match": by_name["view_set_exact_match"]["status"] == "passed" and by_name["ccb_receipt_exact_scope"]["status"] == "passed",
        "pca_family_matrix": by_name["pca_family_matrix"]["status"] == "passed" and by_name["pca_v1_family_matrix"]["status"] == "passed",
        "typed_controller_roundtrip": by_name["typed_controller_roundtrip"]["status"] == "passed",
        "l2_before_after": by_name["l2_before_after"]["status"] == "passed",
        "probes": probes,
    }
    result["status"] = "passed" if all(result[key] for key in ("ccb_catalog_neutral", "view_set_exact_match", "pca_family_matrix", "typed_controller_roundtrip", "l2_before_after")) else "blocked"
    output = Path(args.output).resolve()
    output.parent.mkdir(parents=True, exist_ok=True)
    temporary = output.with_suffix(output.suffix + ".tmp")
    temporary.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    os.replace(temporary, output)
    print(json.dumps({"status": result["status"], "output": str(output), "checks": {key: result[key] for key in ("ccb_catalog_neutral", "view_set_exact_match", "pca_family_matrix", "typed_controller_roundtrip", "l2_before_after")}}, ensure_ascii=False, indent=2))
    return 0 if result["status"] == "passed" else 1


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""PCA-REVERB-PROMOTION-1: reverb-family PCA certification campaign (card 150).

Deterministic, zero-LLM campaign that promotes the PLUGGRABBER-TIMEDYN-COMPAT-1
compatibility matrix's certifiable reverb subjects into the v2 attestation
store through the EXISTING certification workflow (no new certification
mechanism):

  for each subject (first = Abbey Road Plates Stereo):
      POST /agent/processor-certification/start
           {identifier, family: "reverb", confirmed: true,
            consent: "temporary_track_apply_readback_restore"}
      -> the in-agent runner (processor_certification_entry.go +
         processorauthority.CertifyProcessor) does the three-gate load
         (one-time pca_load token + exact identity + fingerprint), two
         inspections, typed apply/readback/restore, full-snapshot compare,
         temporary-track cleanup, and auto import/promote
         (ImportReceiptsV2 -> store.PromoteCurrent).

Typed coverage is declared honestly by the existing mechanism: coverage =
V2CoverageForRoles(family, selected_roles) over the roles the runner actually
exercised (reverb axis table processorattestation/v2.go:87; the Plates decay
axis rides the Damper enum through the enum_label branch).

Two spot-reject subjects (MannyM Reverb, IR1) are run deliberately to record
the runner's honest refusal as reason-row confirmation; their failure is the
expected outcome and does not fail the leg.

Verification gates (exit 0 requires all):
  G1 candidates face: GET /agent/processor-certification/candidates?family=reverb
     reports >=2 promoted+fingerprint-matched subjects including Pro-R and
     >=1 newly certified subject (card acceptance: >=2 reverb candidates).
  G2 recommendation candidate source: harness plugin.semantic_search
     (query=reverb, type=reverb — the exact production call shape from
     plugin_recommendation.go localPluginRecommendationCandidates) returns
     every newly promoted subject among its entries.
  G3 no silent drops: every campaign target ends completed+promoted or with a
     recorded failure reason; stop-loss = 3 consecutive same-signature
     failures.

Machine-local receipts land in ~/.vit/pca_certifications/pca_job_*; campaign
records land in coord/runs/PCA-REVERB-PROMOTION-1/<run_id>/.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

REPO_ROOT = Path(__file__).resolve().parent.parent
RUN_FAMILY_DIR = REPO_ROOT / "coord" / "runs" / "PCA-REVERB-PROMOTION-1"
STORE_V2 = Path.home() / ".vit" / "processor_control_attestations.v2.json"
MATRIX_RUN = (REPO_ROOT / "coord" / "runs" / "PLUGGRABBER-TIMEDYN-COMPAT-1"
              / "pgtdc_20261016_205639")
FAMILY = "reverb"
CONSENT = "temporary_track_apply_readback_restore"
PRO_R_IDENTIFIER = "VST3-Pro-R-4bc24482-78c01fa6"

# Certifiable reverb subjects from the compatibility matrix (2026-10-16,
# compat_verify_pass): first item per the card, then the remaining matrix
# certifiable set. Each row: (plugin_name, identifier).
CERTIFIABLE = [
    ("Abbey Road Plates Stereo", "VST3-Abbey Road Plates Stereo-695648d-5e0a41d9"),
    ("Abbey Road Plates Mono/Stereo", "VST3-Abbey Road Plates Mono/Stereo-695648d-5e1d06d9"),
    ("Abbey Road Plates Mono", "VST3-Abbey Road Plates Mono-695648d-5df3bbd9"),
    ("Abbey Road Chambers Mono", "VST3-Abbey Road Chambers Mono-695648d-2da618f6"),
    ("Abbey Road Chambers Mono/Stereo", "VST3-Abbey Road Chambers Mono/Stereo-695648d-2dcf63f6"),
    ("Abbey Road Chambers Stereo", "VST3-Abbey Road Chambers Stereo-695648d-2dbc9ef6"),
    ("H-Reverb Stereo", "VST3-H-Reverb Stereo-695648d-bc35ab8a"),
    ("H-Reverb Mono/Stereo", "VST3-H-Reverb Mono/Stereo-695648d-bc48708a"),
    ("H-Reverb Mono", "VST3-H-Reverb Mono-695648d-bc1f258a"),
    ("H-Reverb 5.0", "VST3-H-Reverb 5.0-695648d-bbc50d8a"),
    ("H-Reverb 5.1", "VST3-H-Reverb 5.1-695648d-bbc8ce8a"),
    ("H-Reverb Mono/5.0", "VST3-H-Reverb Mono/5.0-695648d-bc4c318a"),
    ("H-Reverb Mono/5.1", "VST3-H-Reverb Mono/5.1-695648d-bc4ff28a"),
    ("H-Reverb Stereo/5.0", "VST3-H-Reverb Stereo/5.0-695648d-bc396c8a"),
    ("H-Reverb Stereo/5.1", "VST3-H-Reverb Stereo/5.1-695648d-bc3d2d8a"),
    ("H-Reverb long Stereo", "VST3-H-Reverb long Stereo-695648d-c778ab8a"),
    ("H-Reverb long Mono/Stereo", "VST3-H-Reverb long Mono/Stereo-695648d-c78b708a"),
    ("H-Reverb long Mono", "VST3-H-Reverb long Mono-695648d-c762258a"),
    ("H-Reverb long 5.0", "VST3-H-Reverb long 5.0-695648d-c7080d8a"),
    ("H-Reverb long 5.1", "VST3-H-Reverb long 5.1-695648d-c70bce8a"),
    ("H-Reverb long Mono/5.0", "VST3-H-Reverb long Mono/5.0-695648d-c78f318a"),
    ("H-Reverb long Mono/5.1", "VST3-H-Reverb long Mono/5.1-695648d-c792f28a"),
    ("H-Reverb long Stereo/5.0", "VST3-H-Reverb long Stereo/5.0-695648d-c77c6c8a"),
    ("H-Reverb long Stereo/5.1", "VST3-H-Reverb long Stereo/5.1-695648d-c7802d8a"),
    ("RVerb Stereo", "VST3-RVerb Stereo-695648d-1bdcffa3"),
    ("RVerb Mono/Stereo", "VST3-RVerb Mono/Stereo-695648d-1be9bfac"),
    ("TrueVerb Stereo", "VST3-TrueVerb Stereo-695648d-a7394d0a"),
    ("TrueVerb Mono/Stereo", "VST3-TrueVerb Mono/Stereo-695648d-525120a"),
    ("TrueVerb Mono", "VST3-TrueVerb Mono-695648d-1b81c70a"),
]

# Deliberate honest-refusal spot checks (matrix reason rows; the certification
# runner is expected to fail these — failure is the recorded evidence).
SPOT_REJECTS = [
    ("MannyM Reverb Stereo",
     "VST3-MannyM Reverb Stereo-695648d-46aa2bcc",
     "matrix: unsupported_broadband_compressor — 无衰减时间控制（Reverb Amount=湿量）"),
    ("IR1 full Stereo",
     "VST3-IR1 full Stereo-695648d-f2791680",
     "matrix: unresolved_reverb_surface — 卷积衰减固化于 IR 采样选择，参数面无衰减控制"),
]


def utcnow() -> str:
    return datetime.now(timezone.utc).isoformat(timespec="seconds")


def request_json(url: str, method: str = "GET", body: dict[str, Any] | None = None,
                 timeout: float = 60) -> dict[str, Any]:
    data = None if body is None else json.dumps(body).encode("utf-8")
    req = urllib.request.Request(url, data=data,
                                 headers={"Content-Type": "application/json"},
                                 method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as response:
            value = json.loads(response.read().decode("utf-8", errors="replace"))
    except urllib.error.HTTPError as error:
        raw = error.read().decode("utf-8", errors="replace")
        try:
            value = json.loads(raw)
        except json.JSONDecodeError:
            value = {"status": "error", "http_status": error.code, "error": raw[:2000]}
        value["_http_status"] = error.code
    if not isinstance(value, dict):
        raise RuntimeError(f"{method} {url} returned non-object JSON")
    return value


def http_ok(value: dict[str, Any]) -> bool:
    return value.get("_http_status", 200) < 400


def load_json(path: Path) -> Any:
    with open(path, encoding="utf-8") as handle:
        return json.load(handle)


def save_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_suffix(path.suffix + ".tmp")
    with open(tmp, "w", encoding="utf-8") as handle:
        json.dump(value, handle, ensure_ascii=False, indent=2)
    tmp.replace(path)


def store_snapshot() -> dict[str, Any]:
    library = load_json(STORE_V2)
    reverb = []
    for attestation in library.get("attestations", []):
        if attestation.get("processor_family") != FAMILY:
            continue
        reverb.append({
            "attestation_id": attestation.get("attestation_id"),
            "status": attestation.get("status"),
            "name": attestation.get("subject", {}).get("name"),
            "identifier": attestation.get("subject", {}).get("identifier"),
            "coverage": sorted(c.get("axis", "") for c in attestation.get("coverage", [])),
            "binary_fingerprint": attestation.get("binary_fingerprint"),
            "evidence_receipt_ids": [e.get("receipt_id") for e in attestation.get("evidence", [])],
        })
    return {"revision": library.get("revision"), "updated_at": library.get("updated_at"),
            "reverb_attestations": reverb}


def agent_invoke(base: str, tool: str, args: dict[str, Any], confirmed: bool = True,
                 source: str = "pca_reverb_promotion", timeout: float = 900.0) -> dict[str, Any]:
    return request_json(base.rstrip("/") + "/agent/invoke", "POST",
                        {"tool": tool, "args": args, "confirmed": confirmed,
                         "source": source}, timeout)


def candidates_face(base: str) -> list[dict[str, Any]]:
    reply = request_json(base.rstrip("/") +
                         f"/agent/processor-certification/candidates?family={FAMILY}",
                         timeout=120)
    if not http_ok(reply):
        raise RuntimeError("candidates face unreachable: " + str(reply.get("error", reply))[:400])
    rows = reply.get("candidates") or []
    if not isinstance(rows, list):
        raise RuntimeError("candidates face returned non-list candidates")
    return rows


def ensure_kernel_plugin_list(base: str, sample_name: str) -> None:
    """Ported from pca_autosweep.ensure_kernel_plugin_list: the kernel's
    knownPluginList is restored from Settings.xml at boot and revalidated
    before serving; during that window searches JUCE-timeout. Retry through
    the busy window; only a genuinely empty list would trigger a scan (not
    expected on this machine: 994 plugins registered)."""
    busy_deadline = time.time() + 20 * 60
    while time.time() < busy_deadline:
        reply = agent_invoke(base, "plugin_search",
                             {"query": sample_name, "limit": 8}, confirmed=False,
                             timeout=180)
        result = reply.get("result") or {}
        error_text = str(result.get("error", "")) + str(reply.get("error", ""))
        if "Timed out waiting" in error_text:
            print("[wait] kernel plugin search busy (boot revalidation), retrying",
                  flush=True)
            time.sleep(30.0)
            continue
        if result.get("plugins"):
            return
        count = result.get("plugin_count")
        if isinstance(count, int) and count > 0:
            return
        raise RuntimeError(f"kernel plugin list responsive but sample {sample_name!r} "
                           f"absent and plugin_count={count}")
    raise RuntimeError("kernel plugin search stayed busy for 20 minutes")


def certify_one(base: str, name: str, identifier: str, job_timeout: float,
                expect_pass: bool = True) -> dict[str, Any]:
    started = time.time()
    start = None
    for attempt in range(6):
        start = request_json(
            base.rstrip("/") + "/agent/processor-certification/start", "POST",
            {"identifier": identifier, "family": FAMILY, "confirmed": True,
             "consent": CONSENT}, timeout=60)
        if "another processor certification" not in str(start.get("error", "")):
            break
        time.sleep(10.0)
    if not http_ok(start):
        return {"plugin_name": name, "identifier": identifier,
                "status": "start_rejected",
                "error": str(start.get("error", start))[:500],
                "expected_reject": not expect_pass, "seconds": round(time.time() - started, 1)}
    job_id = (start.get("job") or {}).get("job_id", "")
    status_url = (base.rstrip("/") +
                  "/agent/processor-certification/status?job_id=" +
                  urllib.parse.quote(job_id))
    deadline = time.time() + job_timeout
    job: dict[str, Any] = {}
    while time.time() < deadline:
        reply = request_json(status_url, timeout=30)
        job = reply.get("job") or {}
        if job.get("status") in {"completed", "failed"}:
            break
        time.sleep(3.0)
    record: dict[str, Any] = {
        "plugin_name": name, "identifier": identifier, "job_id": job_id,
        "expected_reject": not expect_pass,
        "seconds": round(time.time() - started, 1),
    }
    certification = job.get("certification") or {}
    if certification:
        record["summary_path"] = certification.get("summary_path", "")
        results = certification.get("results") or []
        if results:
            record["selected_roles"] = results[0].get("selected_roles", [])
            record["apply_status"] = results[0].get("apply_status", "")
    import_report = job.get("import") or {}
    promoted = import_report.get("promoted") or []
    record["status"] = job.get("status", "unknown")
    record["stage"] = job.get("stage", "")
    record["error"] = str(job.get("error", ""))[:500]
    if promoted:
        record["promoted_attestation_ids"] = [p.get("attestation_id") for p in promoted]
    return record


def failure_signature(record: dict[str, Any]) -> str:
    if record.get("status") == "completed":
        return ""
    text = record.get("error", "") or record.get("status", "unknown")
    return hashlib.sha256(text.encode("utf-8")).hexdigest()[:12]


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--job-timeout", type=float, default=600.0)
    parser.add_argument("--max", type=int, default=0,
                        help="cap certifiable subjects (0 = all); debugging aid")
    parser.add_argument("--verify-only", action="store_true",
                        help="skip the campaign, run the verification gates only")
    args = parser.parse_args()

    run_id = datetime.now().strftime("pca_reverb_promotion_%Y%m%d_%H%M%S")
    run_dir = RUN_FAMILY_DIR / run_id
    run_dir.mkdir(parents=True, exist_ok=True)
    ledger_path = run_dir / "ledger.jsonl"

    def ledger(event: str, **fields: Any) -> None:
        with open(ledger_path, "a", encoding="utf-8") as handle:
            handle.write(json.dumps({"ts": utcnow(), "event": event, **fields},
                                    ensure_ascii=False) + "\n")

    base = args.agent_http
    summary: dict[str, Any] = {
        "schema_version": "pca_reverb_promotion_campaign.v1",
        "card": "PCA-REVERB-PROMOTION-1",
        "family": FAMILY,
        "run_id": run_id,
        "started_at": utcnow(),
        "llm_call_count": 0,
        "agent_http": base,
    }

    # ---- baseline ------------------------------------------------------
    probe = request_json(base.rstrip("/") +
                         f"/agent/processor-certification/candidates?family={FAMILY}",
                         timeout=60)
    if not http_ok(probe):
        print("FATAL: certification endpoint unreachable: " +
              str(probe.get("error", probe))[:300])
        return 2
    before = store_snapshot()
    summary["store_before"] = before
    save_json(run_dir / "store_before.json", before)
    print(f"[baseline] store revision={before['revision']} "
          f"reverb attestations={len(before['reverb_attestations'])}")

    # ---- campaign ------------------------------------------------------
    records: list[dict[str, Any]] = []
    stop_loss = None
    if not args.verify_only:
        first_name = CERTIFIABLE[0][0]
        print(f"[wait] kernel plugin list readiness (sample={first_name!r})", flush=True)
        ensure_kernel_plugin_list(base, first_name)
        targets = CERTIFIABLE if not args.max else CERTIFIABLE[:args.max]
        consecutive_same_failure = 0
        last_signature = ""
        for index, (name, identifier) in enumerate(targets, 1):
            record = certify_one(base, name, identifier, args.job_timeout)
            records.append(record)
            ledger("cert_done", **record)
            save_json(run_dir / "campaign_records.json", records)
            print(f"[certify] {index}/{len(targets)} {name} -> {record.get('status')} "
                  f"roles={record.get('selected_roles')} "
                  f"({record.get('seconds')}s)", flush=True)
            signature = failure_signature(record)
            if signature:
                if signature == last_signature:
                    consecutive_same_failure += 1
                else:
                    consecutive_same_failure = 1
                    last_signature = signature
                if consecutive_same_failure >= 3:
                    stop_loss = {"triggered": True,
                                 "at_subject": name,
                                 "signature": signature,
                                 "sample_error": record.get("error", "")[:300]}
                    print(f"[stop-loss] 3 consecutive same-signature failures at {name}; "
                          "stopping campaign (honest record, no retry)", flush=True)
                    ledger("stop_loss", **stop_loss)
                    break
            else:
                consecutive_same_failure = 0
                last_signature = ""
            time.sleep(2.0)
        if not stop_loss:
            for name, identifier, reason in SPOT_REJECTS:
                record = certify_one(base, name, identifier, args.job_timeout,
                                     expect_pass=False)
                record["matrix_reason_row"] = reason
                records.append(record)
                ledger("spot_reject_done", **record)
                save_json(run_dir / "campaign_records.json", records)
                print(f"[spot-reject] {name} -> {record.get('status')} "
                      f"(expected refusal; error={str(record.get('error'))[:120]})",
                      flush=True)
                time.sleep(2.0)
        summary["stop_loss"] = stop_loss or {"triggered": False}

    # ---- reason rows carried from the compatibility matrix -------------
    reason_rows: list[dict[str, Any]] = []
    matrix_path = MATRIX_RUN / "matrix_summary.json"
    if matrix_path.exists():
        matrix = load_json(matrix_path)
        for row in matrix.get("matrix", []):
            if str(row.get("expected_family", "")) != FAMILY:
                continue
            inspect = (row.get("inspects") or {}).get(FAMILY) or {}
            if inspect.get("resolved") is True:
                continue
            reason_rows.append({
                "plugin_name": row.get("plugin_name"),
                "plugin_identifier": row.get("plugin_identifier"),
                "rejection_code": inspect.get("rejection_code", ""),
                "error": str(inspect.get("error", ""))[:300],
            })
    summary["non_certifiable_reason_rows"] = {
        "source": str(matrix_path),
        "count": len(reason_rows),
    }
    save_json(run_dir / "non_certifiable_reason_rows.json", reason_rows)

    # ---- verification gates --------------------------------------------
    after = store_snapshot()
    summary["store_after"] = after
    save_json(run_dir / "store_after.json", after)

    promoted_reverb = [a for a in after["reverb_attestations"]
                       if a["status"] == "promoted"]
    faces = candidates_face(base)
    face_promoted = {c["identifier"]: c for c in faces
                     if c.get("status") == "promoted" and c.get("fingerprint_match")}
    summary["gates"] = {}

    gate1_promoted_identifiers = sorted(face_promoted)
    gate1_has_pro_r = PRO_R_IDENTIFIER in face_promoted
    campaign_records = [r for r in records if not r.get("expected_reject")]
    newly_promoted_records = [r for r in campaign_records
                              if r.get("status") == "completed"
                              and r.get("promoted_attestation_ids")]
    gate1_new_in_face = [r["identifier"] for r in newly_promoted_records
                         if r["identifier"] in face_promoted]
    gate1 = (len(face_promoted) >= 2 and gate1_has_pro_r and len(gate1_new_in_face) >= 1)
    summary["gates"]["G1_candidates_face"] = {
        "pass": gate1,
        "promoted_fingerprint_matched": gate1_promoted_identifiers,
        "has_pro_r": gate1_has_pro_r,
        "newly_promoted_present": gate1_new_in_face,
    }
    print(f"[verify] G1 candidates face: promoted+fingerprint={len(face_promoted)} "
          f"(incl Pro-R={gate1_has_pro_r}, new={len(gate1_new_in_face)}) -> "
          f"{'PASS' if gate1 else 'FAIL'}")

    # G2: the recommendation candidate source (production call shape from
    # plugin_recommendation.go localPluginRecommendationCandidates).
    search = agent_invoke(base, "plugin.semantic_search",
                          {"query": FAMILY, "type": FAMILY, "limit": 200},
                          confirmed=False,
                          source="ordinary_agent_plugin_recommendation")
    result = search.get("result") or {}
    rows = result.get("entries") or result.get("plugins") or []
    row_names = {str(r.get("name", "")) for r in rows if isinstance(r, dict)}
    missing = [r["plugin_name"] for r in newly_promoted_records
               if r["plugin_name"] not in row_names]
    gate2 = len(rows) > 0 and not missing
    summary["gates"]["G2_semantic_search_source"] = {
        "pass": gate2,
        "entries": len(rows),
        "queried": {"query": FAMILY, "type": FAMILY, "limit": 200},
        "newly_promoted_missing_from_entries": missing,
    }
    print(f"[verify] G2 semantic search source: entries={len(rows)} "
          f"newly-promoted missing={missing} -> {'PASS' if gate2 else 'FAIL'}")

    gate3 = True
    for record in records:
        expected_reject = bool(record.get("expected_reject"))
        completed = record.get("status") == "completed"
        promoted = bool(record.get("promoted_attestation_ids"))
        has_reason = bool(record.get("error") or record.get("matrix_reason_row"))
        ok = (completed and promoted) if not expected_reject else (not completed or has_reason)
        if not ok:
            gate3 = False
    summary["gates"]["G3_no_silent_drops"] = {
        "pass": gate3,
        "records": len(records),
        "expected_rejects": sum(1 for r in records if r.get("expected_reject")),
    }
    print(f"[verify] G3 no silent drops: records={len(records)} -> "
          f"{'PASS' if gate3 else 'FAIL'}")

    summary["promotion_outcome"] = {
        "store_reverb_promoted_before": [a["identifier"] for a in before["reverb_attestations"]
                                         if a["status"] == "promoted"],
        "store_reverb_promoted_after": [a["identifier"] for a in promoted_reverb],
        "campaign_completed": sum(1 for r in campaign_records if r.get("status") == "completed"),
        "campaign_promoted": len(newly_promoted_records),
        "spot_rejects_recorded": sum(1 for r in records if r.get("expected_reject")),
    }
    summary["records"] = records
    summary["finished_at"] = utcnow()
    all_gates = gate1 and gate2 and gate3
    summary["verdict"] = "reverb_promotion_pass" if all_gates else "reverb_promotion_fail"
    save_json(run_dir / "campaign_summary.json", summary)
    print(f"[summary] verdict={summary['verdict']} run_dir={run_dir}")
    return 0 if all_gates else 1


if __name__ == "__main__":
    sys.exit(main())

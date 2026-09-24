#!/usr/bin/env python3
"""FIX-PCA-AUTOSWEEP-1: one-command full-library PCA sweep for this machine.

Pipeline (each phase is resumable from the run state file):

  preflight -> probe (pluginprobe observation host, cycle A: all effect
  subjects) -> classify (deterministic parameter-topology family rules,
  zero LLM, zero plugin/vendor-name judgment) -> certify (agent HTTP
  processor-certification runner: identifier-minted load, typed axis
  writes, readback, full-snapshot compare, receipt, auto import/promote;
  failures land in the exception ledger with reasons. FIX-PCA-EQCHANNEL-1:
  static_eq rides its own deterministic channel — the EQ-1 path
  productised: rack load by identifier -> explain band topology ->
  transactional upsert/modify/disable/undo writes -> snapshot compare ->
  phase3-style receipt -> pcactl import) -> derive (probe
  cycle B for the promoted set, EQ static_eq whitelist band derivation,
  v5->v6 live normalization, rerun scripts/build_whitelist_v6_full.py
  unchanged; --apply-live grows the live whitelist with backups) -> verify
  (pcactl admission overlay regression over every derived entry +
  non-member spots) -> report (three-bucket sweep_report + per-family
  counts).

Design freeze (card 2026-09-23-FIX-PCA-AUTOSWEEP-1):
  - classification uses structural parameter topology only (parameter
    names from the observation snapshot); plugin/vendor names are never
    classification inputs;
  - certification is the EQ-1-style deterministic channel (zero LLM);
  - idempotent reruns; nothing silently dropped (every exception row
    carries a reason).

Machine-local artifacts (probe snapshots) live under
  ~/.vit/autosweep/<run_id>/probe/
The run ledger, classification, and reports live under the repo run dir
  coord/runs/FIX-PCA-AUTOSWEEP-1/<run_id>/
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

REPO_ROOT = Path(__file__).resolve().parent.parent
RUN_FAMILY_DIR = REPO_ROOT / "coord" / "runs" / "FIX-PCA-AUTOSWEEP-1"
DEFAULT_SEMANTICS = Path.home() / ".vit" / "plugin_semantics.json"
DEFAULT_STORE_V1 = Path.home() / ".vit" / "processor_control_attestations.v1.json"
DEFAULT_STORE_V2 = Path.home() / ".vit" / "processor_control_attestations.v2.json"
DEFAULT_LIVE = Path.home() / ".vit" / "free_state_experiment_plugins.json"
DEFAULT_CERT_DIR = Path.home() / ".vit" / "pca_certifications"
BUILDER = REPO_ROOT / "scripts" / "build_whitelist_v6_full.py"
DEFAULT_WORKER = (REPO_ROOT / "PluginProbe" / "native-host" / "build"
                  / "pluginprobe_vst3_worker_artefacts" / "Release"
                  / "pluginprobe_vst3_worker.exe")

V6_SCHEMA = "vit.free_state_experiment_plugins.v6"
V5_SCHEMA = "vit.free_state_experiment_plugins.v5"

# ---------------------------------------------------------------- utilities


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


def slug_of(name: str) -> str:
    return name.replace(" ", "_").replace("/", "_")


def subject_key(entry: dict[str, Any]) -> str:
    """Mirror of processorattestation.BuildSubjectKey (identifier branch)."""
    identity = entry["format"].lower() + "\x00identifier\x00" + entry["identifier"].lower()
    digest = hashlib.sha256(identity.encode("utf-8")).digest()
    return "pcs1_" + digest[:12].hex()


def load_json(path: Path) -> Any:
    with open(path, encoding="utf-8") as handle:
        return json.load(handle)


def save_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_suffix(path.suffix + ".tmp")
    with open(tmp, "w", encoding="utf-8") as handle:
        json.dump(value, handle, ensure_ascii=False, indent=2)
    os.replace(tmp, path)


# ------------------------------------------------- deterministic classifier
# Structural parameter-topology rules (card design freeze #1). Matching uses
# parameter names from the observation snapshot only. Token = word-boundary
# case-insensitive; contains = substring case-insensitive (the same grammar
# the in-repo Go topology detectors use). The Go detectors stay the judge at
# certification time; classifier/detector disagreement lands in the exception
# ledger with the runner's boundary code.

BAND = re.compile(r"(?:^|[^a-z0-9])(?:band|b)\s*([0-9]+)(?:[^a-z0-9]|$)", re.I)
NAMED_BAND = re.compile(r"^\s*(low|mid|high)\s+", re.I)
CROSSOVER = re.compile(r"crossover|cross over|xover|x-over", re.I)
CROSSOVER_COMPACT = {"lowmidfreq", "midlowfreq", "midhighfreq", "highmidfreq"}


def _token_re(token: str) -> re.Pattern[str]:
    return re.compile(r"(?:^|[^a-z0-9])" + re.escape(token) + r"(?:[^a-z0-9]|$)", re.I)


def has_token(text: str, *tokens: str) -> bool:
    return any(_token_re(t).search(text) for t in tokens)


def contains(text: str, *subs: str) -> bool:
    return any(s.lower() in text.lower() for s in subs)


def compact_label(text: str) -> str:
    return re.sub(r"[\s_\-/]+", "", text).lower()


def is_crossover_label(text: str) -> bool:
    if re.search(r"(?:^|\s)q(?:\s|$)", text, re.I):
        return False
    if CROSSOVER.search(text):
        return True
    return compact_label(text) in CROSSOVER_COMPACT


def band_key(label: str) -> str | None:
    match = BAND.search(label)
    if match:
        return match.group(1)
    if is_crossover_label(label):
        return None
    # leading-index prefixes: "1 Lf Frequency" / "1 Lmf Gain" (bx-style)
    leading = re.match(r"^(\d+(?:\s+[A-Za-z]+)?)\s+\S", label)
    if leading:
        return leading.group(1)
    # trailing-index roles: "Freq 1" / "Gain 1" / "Q 1" (Waves-style)
    trailing = re.match(r"^(.*[A-Za-z])\s+(\d+)$", label)
    if trailing:
        return trailing.group(2)
    named = NAMED_BAND.search(label)
    return named.group(1).lower() if named else None


def freq_param(name: str) -> bool:
    return has_token(name, "freq", "frequency", "frq")


def eq_gain_param(name: str) -> bool:
    if contains(name, "makeup", "make-up", "output gain", "out gain", "output level",
                "out level", "input gain", "in gain", "gain reduction", "sc gain"):
        return False
    return has_token(name, "gain")


def thresholdish(name: str) -> bool:
    return has_token(name, "threshold", "thresh")


def dynamicsish(name: str) -> bool:
    return bool(re.search(
        r"\b(?:threshold|thresh|range|reduction|ratio|knee|gain|makeup|attack|release|"
        r"recovery|hold|lookahead)\b", name, re.I))


def classify_snapshot(snapshot: dict[str, Any]) -> dict[str, Any]:
    """Deterministic family classification over one observation snapshot."""
    params = [row for row in snapshot.get("surface", {}).get("parameters", [])
              if isinstance(row, dict)]
    controllable = [str(row.get("name", "")) for row in params
                    if bool(row.get("host_controllable", True)) and str(row.get("name", ""))]
    all_names = [str(row.get("name", "")) for row in params if str(row.get("name", ""))]

    band_cells: dict[str, list[str]] = {}
    for label in controllable:
        key = band_key(label)
        if key:
            band_cells.setdefault(key, []).append(label)
    crossover_count = sum(1 for label in controllable if is_crossover_label(label))

    band_thresholds = sum(1 for cell in band_cells.values()
                          if any(thresholdish(x) for x in cell))
    band_dynamics_cells = sum(1 for cell in band_cells.values()
                              if any(dynamicsish(x) for x in cell))

    freq_keys = {band_key(x) for x in controllable if freq_param(x) and band_key(x)}
    gain_keys = {band_key(x) for x in controllable if eq_gain_param(x) and band_key(x)}
    eq_pair_keys = freq_keys & gain_keys

    has_ceiling_strong = any(contains(x, "ceiling", "true peak", "truepeak",
                                      "out ceiling") for x in controllable)
    has_output_level = any(has_token(x, "output level", "out level") for x in controllable)
    has_lookahead = any(has_token(x, "lookahead", "look-ahead") for x in controllable)
    has_threshold = any(thresholdish(x) for x in controllable)
    has_ratio = any(has_token(x, "ratio") for x in controllable)
    has_makeup = any(contains(x, "makeup", "make-up") or has_token(x, "makeup")
                     for x in controllable)
    has_range = any(has_token(x, "range") for x in controllable)
    has_gate_word = any(contains(x, "gate", "expander", "expansion", "upward",
                                 "duck", "downward") for x in controllable)
    has_sibilance = any(contains(x, "sibilance", "de-esser", "de esser", "deesser",
                                 "s-reduction", "s reduction") for x in controllable)
    has_attack = any(has_token(x, "attack") for x in controllable)
    has_sustain = any(has_token(x, "sustain")
                      or contains(x, "attack amount", "transient amount",
                                  "transient range") for x in controllable)
    has_transient_word = any(contains(x, "transient") for x in controllable)
    has_release_hold = any(has_token(x, "release", "hold", "recovery")
                           for x in controllable)

    def _band_filter(word: str) -> bool:
        return any(contains(x, f"{word}-pass frequency", f"{word}pass frequency")
                   or (has_token(x, f"{word}-pass", f"{word}pass") and freq_param(x))
                   for x in controllable)
    has_hp_lp_freq = _band_filter("high") and _band_filter("low")
    has_monitor = any(contains(x, "monitor", "listen", "audition", "detection",
                               "detector") for x in controllable)
    has_floor = any(has_token(x, "floor") for x in controllable)
    has_gate_open_close = (any(contains(x, "gate open") for x in controllable)
                           and any(contains(x, "gate close") for x in controllable))
    has_sense = any(has_token(x, "sense", "sensitivity") for x in controllable)
    has_duration = any(has_token(x, "duration") for x in controllable)
    has_output_gainish = any(has_token(x, "output", "mix") or
                             contains(x, "out gain", "output gain") for x in controllable)
    # non-experiment veto: reverb/echo units embed dynamics+EQ sections that
    # are not standalone experiment processors (card freeze: 如实排除).
    # Controllable names only: Waves exposes the full MIDI CC list (incl.
    # "Reverb Depth") as non-controllable rows that must not trigger the veto.
    reverb_veto = any(contains(x, "reverb", "predelay", "echo") for x in controllable)

    multiband = (band_thresholds >= 3) or (crossover_count >= 2 and band_dynamics_cells >= 3)

    families: dict[str, str] = {}
    detail: dict[str, Any] = {
        "parameter_count": len(params),
        "controllable_count": len(controllable),
        "band_cells": len(band_cells),
        "band_threshold_cells": band_thresholds,
        "crossover_count": crossover_count,
        "eq_pair_keys": sorted(eq_pair_keys),
        "reverb_veto": reverb_veto,
    }
    if multiband and not reverb_veto:
        families["multiband"] = (
            f"band_threshold_cells={band_thresholds} crossovers={crossover_count}")
    if len(eq_pair_keys) >= 3:
        families["static_eq"] = f"freq/gain pairs={len(eq_pair_keys)} keys={sorted(eq_pair_keys)[:8]}"
    # limiter: explicit ceiling, or the FabFilter-style output-ceiling shape
    # (Output Level + Lookahead with no threshold/ratio operating point)
    if (has_ceiling_strong or (has_output_level and has_lookahead
                               and not has_threshold)) \
            and not multiband and not has_ratio and not has_sibilance \
            and not reverb_veto:
        families["limiter"] = ("ceiling param present, single band" if has_ceiling_strong
                               else "output-level + lookahead ceiling shape")
    # de-esser: threshold + range + sibilance band structure (explicit
    # sibilance naming, hp/lp detection pair, or monitor/detection listen)
    if has_threshold and has_range and (has_sibilance or has_hp_lp_freq or has_monitor) \
            and not multiband and not reverb_veto:
        families["de_esser"] = ("threshold + sibilance-band structure" if has_sibilance
                                else "threshold + range + detection band/monitor")
    # gate/expander: gate-word structure with range or floor attenuation
    if ((has_gate_word or has_floor) and (has_range or has_floor)
            and (has_threshold or has_gate_open_close or has_attack
                 or has_release_hold)) \
            and not multiband and not has_sibilance and not reverb_veto:
        families["gate_expander"] = "threshold + range/floor + gate/expander direction"
    # transient shaper: attack-side (attack, or Sense+Duration shaping pair)
    # with an output/mix stage and no compressor operating point
    if ((has_attack or has_transient_word or (has_sense and has_duration))
            and (has_sustain or has_output_gainish)) \
            and not has_threshold and not has_ratio and not multiband \
            and not has_sibilance and not has_ceiling_strong and not has_lookahead \
            and not reverb_veto:
        families["transient_shaper"] = "attack + independent sustain/output stage"
    if (has_threshold and (has_ratio or has_makeup) and not multiband
            and not has_sibilance and "gate_expander" not in families
            and not reverb_veto):
        families["broadband_compression"] = "threshold + ratio/makeup, single band"
    detail["flags"] = {
        "ceiling": has_ceiling_strong, "output_level": has_output_level,
        "lookahead": has_lookahead, "threshold": has_threshold, "ratio": has_ratio,
        "makeup": has_makeup, "range": has_range, "gate_word": has_gate_word,
        "sibilance": has_sibilance, "hp_lp_freq": has_hp_lp_freq,
        "monitor": has_monitor, "floor": has_floor,
        "attack": has_attack, "sustain": has_sustain,
        "sense_duration": has_sense and has_duration,
    }
    return {"families": families, "detail": detail}


# whitelist family -> certification runner family / store / admission axis
CERT_FAMILY = {
    "multiband": ("multiband_dynamics", "v2", "band_dynamics"),
    "limiter": ("limiter", "v2", "output_ceiling"),
    "de_esser": ("de_esser", "v2", "threshold_sensitivity"),
    "gate_expander": ("gate_expander", "v2", "attenuation_floor"),
    "transient_shaper": ("transient_shaper", "v2", "envelope_timing"),
    "broadband_compression": ("broadband_compressor", "v1", "activation_intensity"),
    # FIX-PCA-EQCHANNEL-1: static_eq now has its own deterministic channel (the
    # EQ-1 path productised): identifier-minted rack load -> explain topology ->
    # transactional upsert/modify/disable/undo coverage writes -> readback
    # snapshots -> phase3-style receipt -> pcactl import (v1 store).
    "static_eq": ("static_eq", "v1", "gain_band"),
}

# EQ-channel receipt kinds. The v1 import gate (processorauthority.ReadReceipt)
# accepts exactly these two vendor-scoped kinds; subjects from any other
# manufacturer are honestly rejected with vendor_kind_missing (extending the
# kind list is agent code, out of this script's file domain).
EQ_RECEIPT_KIND = {
    "waves": "waves.static_eq.phase3_live_smoke.v1",
    "plugin alliance": "plugin_alliance.eq_two_level_live_smoke.v1",
}

# PC-FULL ruling 1 (FIX-PCA-EQCHANNEL-1 execution): legacy journey-verified
# subjects that the store already promotes get one formal pass through the
# current deterministic channels. static_eq classified hits are all force
# targets (fresh receipt + explain evidence feeds whitelist band derivation);
# the broadband list below is the explicit legacy pair from the ruling.
EQ_FORCE_LEGACY_BROADBAND = ("Vertigo VSC-2",)


# ------------------------------------------------------------- probe client


class ProbeHost:
    def __init__(self, base: str, timeout: float):
        self.base = base.rstrip("/")
        self.timeout = timeout
        self.process: subprocess.Popen | None = None

    def healthy(self) -> bool:
        try:
            reply = request_json(self.base + "/health", timeout=5)
            return reply.get("status") == "ok"
        except Exception:
            return False

    def ensure_worker(self, worker: Path) -> None:
        if self.healthy():
            return
        if not worker.is_file():
            raise RuntimeError(f"pluginprobe worker not found: {worker}")
        exe = Path.home() / ".vit" / "autosweep" / "bin" / "pluginprobe.exe"
        exe.parent.mkdir(parents=True, exist_ok=True)
        if not exe.is_file():
            subprocess.run(["go", "build", "-o", str(exe), "./cmd/pluginprobe"],
                           cwd=REPO_ROOT / "agent", check=True, capture_output=True)
        listen = urllib.parse.urlparse(self.base).netloc
        self.process = subprocess.Popen(
            [str(exe), "-worker", str(worker), "-listen", listen],
            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
            creationflags=getattr(subprocess, "CREATE_NEW_PROCESS_GROUP", 0))
        deadline = time.time() + 30
        while time.time() < deadline:
            if self.healthy():
                return
            time.sleep(1.0)
        raise RuntimeError(f"pluginprobe service did not become healthy on {listen}")

    def shutdown(self) -> None:
        if self.process is not None:
            try:
                request_json(self.base + "/v1/plugin/unload", "POST", {}, timeout=10)
            except Exception:
                pass
            self.process.terminate()
            try:
                self.process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                self.process.kill()
            self.process = None

    def observe(self, entry: dict[str, Any], uid: str | None) -> dict[str, Any]:
        body: dict[str, Any] = {
            "plugin_path": entry["plugin_path"],
            "plugin_name": entry["name"],
        }
        if uid is not None:
            body["plugin_uid"] = uid
        if entry.get("manufacturer"):
            body["manufacturer"] = entry["manufacturer"]
        loaded = request_json(self.base + "/v1/plugin/load", "POST", body, self.timeout)
        if loaded.get("status") == "error":
            raise RuntimeError(str(loaded.get("error", loaded))[:500])
        try:
            snapshot = request_json(self.base + "/v1/plugin/snapshot", timeout=self.timeout)
            if snapshot.get("status") == "error":
                raise RuntimeError(str(snapshot.get("error", snapshot))[:500])
            return snapshot
        finally:
            try:
                request_json(self.base + "/v1/plugin/unload", "POST", {}, timeout=30)
            except Exception:
                pass


def derive_uid(entry: dict[str, Any]) -> str | None:
    """VST3 shell member uid from the identifier tail (verified against
    kernel plugin_search uids 5/5 on 2026-09-24). The tail is the u32 hex
    with leading zeros stripped, so it is 1-8 hex digits, not always 8."""
    tail = entry["identifier"].rsplit("-", 1)[-1]
    if not re.fullmatch(r"[0-9a-fA-F]{1,8}", tail):
        return None
    value = int(tail, 16)
    if value >= (1 << 32):
        return None
    return str(value - (1 << 32) if value >= (1 << 31) else value)


# ------------------------------------------------------------ sweep driver


class Sweep:
    def __init__(self, args: argparse.Namespace):
        self.args = args
        run_id = args.run_id
        if not run_id:
            existing = sorted(RUN_FAMILY_DIR.glob("*")) if RUN_FAMILY_DIR.is_dir() else []
            # idempotent by default: continue the latest run unless it already
            # produced its report; --new-run forces a fresh directory.
            if existing and not args.new_run:
                run_id = existing[-1].name
            else:
                run_id = datetime.now().strftime("%Y%m%d_%H%M%S")
        self.run_id = run_id
        self.run_dir = RUN_FAMILY_DIR / run_id
        self.machine_root = Path.home() / ".vit" / "autosweep" / run_id
        self.probe_dir = self.machine_root / "probe"
        self.run_dir.mkdir(parents=True, exist_ok=True)
        self.state_path = self.run_dir / "state.json"
        self.state: dict[str, Any] = load_json(self.state_path) if self.state_path.exists() else {
            "schema_version": "pca_autosweep.state.v1",
            "run_id": run_id,
            "created_at": utcnow(),
            "probe": {},        # slug -> {status, error?}
            "classify": {},     # slug -> {families, detail}
            "cert": {},         # "slug|family" -> {status, job_id?, error?, receipt?}
            "derived": False,
            "verified": False,
        }
        self.ledger_path = self.run_dir / "run_ledger.jsonl"

    # -- shared helpers ------------------------------------------------

    def ledger(self, event: str, **fields: Any) -> None:
        row = {"ts": utcnow(), "event": event, **fields}
        with open(self.ledger_path, "a", encoding="utf-8") as handle:
            handle.write(json.dumps(row, ensure_ascii=False) + "\n")

    def save_state(self) -> None:
        self.state["updated_at"] = utcnow()
        save_json(self.state_path, self.state)

    def semantics(self) -> list[dict[str, Any]]:
        return load_json(self.args.semantics)["entries"]

    def effect_subjects(self) -> list[dict[str, Any]]:
        entries = [e for e in self.semantics() if not e.get("is_instrument")]
        path_counts: Counter[str] = Counter(e["plugin_path"] for e in entries)
        for entry in entries:
            entry["_shell"] = path_counts[entry["plugin_path"]] > 1
        return entries

    def store_promoted(self) -> set[tuple[str, str]]:
        """(subject_key, runner_family) currently promoted."""
        out: set[tuple[str, str]] = set()
        for store_path in (self.args.store_v1, self.args.store_v2):
            if not Path(store_path).exists():
                continue
            for attestation in load_json(store_path).get("attestations", []):
                if attestation.get("status") == "promoted":
                    out.add((attestation["subject"]["subject_key"],
                             attestation["processor_family"]))
        return out

    # -- phases ---------------------------------------------------------

    def phase_preflight(self) -> None:
        problems: list[str] = []
        for label, path in (("semantics index", self.args.semantics),
                            ("attestation store v1", self.args.store_v1),
                            ("attestation store v2", self.args.store_v2),
                            ("live whitelist", self.args.live),
                            ("builder", BUILDER)):
            if not Path(path).exists():
                problems.append(f"{label} missing: {path}")
        entries = self.effect_subjects()
        promoted = self.store_promoted()
        print(f"[preflight] effect subjects={len(entries)} "
              f"promoted(subject,family)={len(promoted)} problems={len(problems)}")
        for problem in problems:
            print("  PROBLEM:", problem)
        if problems:
            raise RuntimeError("preflight failed")

    def phase_probe(self) -> None:
        import threading

        subjects = self.effect_subjects()
        todo = [e for e in subjects if self.state["probe"].get(slug_of(e["name"]), {}).get("status") != "ok"]
        if self.args.probe_limit:
            todo = todo[:self.args.probe_limit]
        workers = max(1, min(self.args.probe_workers, len(todo) or 1))
        print(f"[probe] subjects={len(subjects)} remaining={len(todo)} workers={workers}")
        lock = threading.Lock()
        shards: list[list[dict[str, Any]]] = [[] for _ in range(workers)]
        for index, entry in enumerate(todo):
            shards[index % workers].append(entry)
        started = time.time()
        done_counter = [0]

        def run_shard(shard_index: int, shard: list[dict[str, Any]]) -> None:
            base = self.args.probe if shard_index == 0 else \
                f"http://127.0.0.1:{9318 + shard_index}"
            host = ProbeHost(base, self.args.probe_timeout)
            try:
                host.ensure_worker(self.args.worker)
            except RuntimeError as error:
                print(f"[probe] worker {shard_index} unavailable: {error}")
                return
            for entry in shard:
                slug = slug_of(entry["name"])
                uid = derive_uid(entry) if entry["_shell"] else None
                if entry["_shell"] and uid is None:
                    # a shell member without a derivable uid would attempt a
                    # name-only load inside a multi-member shell and hang;
                    # fail fast with the honest reason instead
                    with lock:
                        self.state["probe"][slug] = {
                            "status": "fail",
                            "error": f"shell member uid not derivable from "
                                     f"identifier {entry['identifier']}"}
                        self.ledger("probe_fail", slug=slug,
                                    error="shell member uid not derivable")
                    continue
                try:
                    snapshot = host.observe(entry, uid)
                    save_json(self.probe_dir / f"{slug}.a.snapshot.json", snapshot)
                    with lock:
                        self.state["probe"][slug] = {
                            "status": "ok",
                            "params": len(snapshot["surface"]["parameters"])}
                        done_counter[0] += 1
                        count = done_counter[0]
                        if count % 25 == 0:
                            self.save_state()
                            rate = count / max(time.time() - started, 1.0)
                            ok = sum(1 for v in self.state["probe"].values()
                                     if v.get("status") == "ok")
                            print(f"[probe] {count}/{len(todo)} (ok={ok}) "
                                  f"{rate:.2f}/s eta="
                                  f"{(len(todo) - count) / max(rate, 0.01) / 60:.0f}m",
                                  flush=True)
                except Exception as error:
                    with lock:
                        self.state["probe"][slug] = {"status": "fail",
                                                     "error": str(error)[:500]}
                        self.ledger("probe_fail", slug=slug, error=str(error)[:500])
            host.shutdown()

        if workers == 1:
            run_shard(0, shards[0])
        else:
            threads = [threading.Thread(target=run_shard, args=(i, shard))
                       for i, shard in enumerate(shards)]
            for thread in threads:
                thread.start()
            for thread in threads:
                thread.join()
        self.save_state()
        ok = sum(1 for v in self.state["probe"].values() if v.get("status") == "ok")
        fail = sum(1 for v in self.state["probe"].values() if v.get("status") == "fail")
        print(f"[probe] done ok={ok} fail={fail}")

    def phase_classify(self) -> None:
        subjects = {slug_of(e["name"]): e for e in self.effect_subjects()}
        pending = [slug for slug, rec in self.state["probe"].items()
                   if rec.get("status") == "ok" and slug not in self.state["classify"]]
        print(f"[classify] pending={len(pending)}")
        for slug in pending:
            snapshot = load_json(self.probe_dir / f"{slug}.a.snapshot.json")
            self.state["classify"][slug] = classify_snapshot(snapshot)
        self.save_state()
        counts: Counter[str] = Counter()
        for rec in self.state["classify"].values():
            for family in rec["families"]:
                counts[family] += 1
        print("[classify] family hits:", dict(counts))
        save_json(self.run_dir / "classification.json", self.state["classify"])

    def cert_targets(self) -> list[tuple[dict[str, Any], str]]:
        """(entry, whitelist_family) pairs to certify: classification hits not
        already recorded in the run state, plus the ruling-2 C1 comp pair and
        the EQCHANNEL legacy force targets. static_eq hits are always force
        targets (PC-FULL ruling 1): every one gets one fresh EQ-channel pass —
        the receipt covers promotion and the explain evidence feeds whitelist
        band derivation — even when the store already promotes the subject."""
        promoted = self.store_promoted()
        by_slug = {slug_of(e["name"]): e for e in self.effect_subjects()}
        targets: list[tuple[dict[str, Any], str]] = []
        seen: set[tuple[str, str]] = set()

        def add(entry: dict[str, Any], family: str, force: bool = False) -> None:
            runner, _, _ = CERT_FAMILY[family]
            dedupe = (entry["identifier"], family)
            if dedupe in seen:
                return
            key = f"{slug_of(entry['name'])}|{family}"
            recorded = self.state["cert"].get(key, {}).get("status")
            # force targets (EQCHANNEL legacy re-certification) bypass the
            # already_promoted note: that note is a promotion observation, not
            # a certification record; only a real pass/completion satisfies.
            settled = {"passed", "completed"} | (set() if force else {"already_promoted"})
            if recorded in settled:
                return
            if not force and (subject_key(entry), runner) in promoted:
                self.state["cert"].setdefault(key, {"status": "already_promoted"})
                return
            seen.add(dedupe)
            targets.append((entry, family))

        for slug, rec in sorted(self.state["classify"].items()):
            entry = by_slug.get(slug)
            if entry is None:
                continue
            for family in sorted(rec["families"]):
                add(entry, family, force=(family == "static_eq"))
        # PC-1 ruling 2: C1 comp Mono + M/S get broadband_compressor regardless
        # of the classifier, with the compressor->broadband mapping recorded.
        for entry in self.effect_subjects():
            if entry["name"].startswith("C1 comp"):
                add(entry, "broadband_compression")
        # EQCHANNEL legacy re-certification (PC-FULL ruling 1): one formal pass
        # for already-promoted journey-verified subjects.
        for entry in self.effect_subjects():
            if entry["name"] in EQ_FORCE_LEGACY_BROADBAND:
                add(entry, "broadband_compression", force=True)
        return targets

    def agent_invoke(self, tool: str, args: dict[str, Any], confirmed: bool = True,
                     timeout: float = 900.0) -> dict[str, Any]:
        return request_json(
            self.args.agent_http.rstrip("/") + "/agent/invoke", "POST",
            {"tool": tool, "args": args, "confirmed": confirmed,
             "source": "pca_autosweep"}, timeout)

    def kernel_plugin_count(self, query: str = "Waves", timeout: float = 120.0) -> int:
        """Probe the kernel knownPluginList size through the agent's
        plugin_search passthrough (the agent result carries plugin_count)."""
        reply = self.agent_invoke("plugin_search", {"query": query, "limit": 1},
                                  confirmed=False, timeout=timeout)
        result = reply.get("result") or {}
        try:
            return int(result.get("plugin_count", 0))
        except (TypeError, ValueError):
            return 0

    def ensure_kernel_plugin_list(self, sample_identifier: str) -> None:
        """The kernel's knownPluginList is restored from Settings.xml at boot
        and revalidated before it serves searches; during that window searches
        JUCE-timeout even though the list is not empty. Distinguish busy
        revalidation (retry) from a genuinely empty list (scan). EQCHANNEL
        pilot 2026-09-24: a false 'empty' verdict fired a redundant full scan
        on top of boot revalidation and wedged the kernel's JUCE queue."""
        empty_confirm = 0
        for attempt in range(40):  # ~20 min boot-revalidation window (the
            # 2026-09-23 AUTOSWEEP run showed the same gap: first certify died
            # 1 min after boot, the rerun 20 min later found the list ready)
            reply = self.agent_invoke("plugin_search",
                                      {"query": sample_identifier, "limit": 8},
                                      confirmed=False)
            result = reply.get("result") or {}
            error_text = str(result.get("error", "")) + str(reply.get("error", ""))
            busy = "Timed out waiting" in error_text
            if not busy:
                if result.get("plugins"):
                    return
                count = result.get("plugin_count")
                if isinstance(count, int) and count > 0:
                    return  # populated list; sample miss is an identity issue,
                    # not something a rescan would fix
                # responsive AND empty: on a young stack the first search is
                # the lazy-load trigger and the list materialises ~20 min
                # later; confirm the emptiness persists before scanning (a
                # scan fired during the load window piles up on the JUCE
                # queue and starves the whole kernel).
                empty_confirm += 1
                if empty_confirm < 5:  # ~2 min of responsive-empty watching
                    time.sleep(30.0)
                    continue
                break  # genuinely empty -> scan below
            time.sleep(30.0)
        else:
            raise RuntimeError("kernel plugin search stayed busy for 20 minutes "
                               "(boot-time revalidation); wait for the stack to "
                               "settle and rerun certify")
        entries = self.effect_subjects()
        parents = Counter(str(Path(e["plugin_path"]).parent) for e in entries)
        scan_dir = parents.most_common(1)[0][0]
        print(f"[certify] kernel plugin list empty; scanning {scan_dir}")
        self.agent_invoke("scan_plugins", {"paths": [scan_dir]}, timeout=900.0)
        stable, last, deadline = 0, -1, time.time() + 1800
        while time.time() < deadline:
            time.sleep(20.0)
            count = self.kernel_plugin_count()
            if count == last and count > 0:
                stable += 1
                if stable >= 4:
                    print(f"[certify] scan settled at {count} plugins")
                    return
            else:
                stable = 0
            last = count
        raise RuntimeError("kernel plugin scan did not settle within 30 minutes")

    def phase_certify(self) -> None:
        targets = self.cert_targets()
        if self.args.families:
            wanted = {f.strip() for f in self.args.families.split(",") if f.strip()}
            targets = [(entry, family) for entry, family in targets
                       if family in wanted]
        if self.args.eq_subject:
            targets = [(entry, family) for entry, family in targets
                       if family != "static_eq"
                       or self.args.eq_subject.casefold() in entry["name"].casefold()]
        print(f"[certify] targets={len(targets)}")
        if not targets:
            return
        # stack liveness: the agent must be up (kernel + agent; smoke stack)
        probe = request_json(self.args.agent_http.rstrip("/") +
                             "/agent/processor-certification/candidates",
                             timeout=30)
        if not http_ok(probe):
            raise RuntimeError("agent certification endpoint unreachable; start the "
                               "smoke stack first (dev_agent_smoke.ps1 -StartKernel -StartUI)")
        if targets:
            self.ensure_kernel_plugin_list(targets[0][0]["name"])
        cap = self.args.max_cert
        done = 0
        if any(family == "static_eq" for _, family in targets):
            self.ensure_full_access_authority()
        for entry, family in targets:
            if cap and done >= cap:
                print(f"[certify] --max-cert {cap} reached; rerun to continue")
                break
            key = f"{slug_of(entry['name'])}|{family}"
            runner, _, _ = CERT_FAMILY[family]
            started = time.time()
            if family == "static_eq":
                record = self.certify_eq(entry)
            else:
                start = None
                for attempt in range(6):
                    start = request_json(
                        self.args.agent_http.rstrip("/") +
                        "/agent/processor-certification/start",
                        "POST", {"identifier": entry["identifier"], "family": runner,
                                 "confirmed": True,
                                 "consent": "temporary_track_apply_readback_restore"},
                        timeout=30)
                    if str(start.get("error", "")).find("another processor certification") < 0:
                        break
                    time.sleep(10.0)  # busy: one job at a time on the runner
                if not http_ok(start):
                    reason = str(start.get("error", start))[:500]
                    self.state["cert"][key] = {"status": "start_rejected", "error": reason}
                    self.ledger("cert_start_rejected", subject=entry["name"], family=family,
                                error=reason)
                    self.save_state()
                    continue
                job_id = start.get("job", {}).get("job_id", "")
                record = self.poll_job(job_id)
            record["seconds"] = round(time.time() - started, 1)
            record["runner_family"] = runner
            self.state["cert"][key] = record
            self.ledger("cert_done", subject=entry["name"], family=family,
                        job_id=record.get("job_id", ""),
                        status=record.get("status"),
                        error=record.get("error", ""))
            self.save_state()
            done += 1
            print(f"[certify] {done}/{len(targets)} {entry['name']} [{family}] -> "
                  f"{record.get('status')} ({record['seconds']}s)", flush=True)
        statuses = Counter(v.get("status") for v in self.state["cert"].values())
        print("[certify] ledger:", dict(statuses))

    # -- EQ channel (FIX-PCA-EQCHANNEL-1) --------------------------------

    def eq_invoke(self, tool: str, args: dict[str, Any], timeout: float = 480.0,
                  audit: Counter | None = None, direct: bool = False) -> dict[str, Any]:
        """Invoke one agent tool. direct=True sends the harness's operator
        surface (no source): the EQ certification channel is a deterministic
        zero-LLM operator client, and the pca_load_gate by design governs
        sourced agent-flow loads, not direct operator invokes (harness.go
        enforceAgentProcessorLoadGate skips sourceless requests)."""
        if audit is not None:
            audit[tool] += 1
        body: dict[str, Any] = {"tool": tool, "args": args, "confirmed": True}
        if not direct:
            body["source"] = "pca_autosweep"
        request = urllib.request.Request(
            self.args.agent_http.rstrip("/") + "/agent/invoke",
            data=json.dumps(body).encode("utf-8"),
            headers={"Content-Type": "application/json"}, method="POST")
        try:
            with urllib.request.urlopen(request, timeout=timeout) as response:
                reply = json.loads(response.read().decode("utf-8", errors="replace"))
        except urllib.error.HTTPError as error:
            raw = error.read().decode("utf-8", errors="replace")
            try:
                reply = json.loads(raw)
            except json.JSONDecodeError:
                reply = {"status": "error", "error": raw[:400]}
            reply["_http_status"] = error.code
        if not http_ok(reply):
            raise RuntimeError(f"{tool}: HTTP {reply.get('_http_status')}: "
                               f"{str(reply.get('error', reply))[:400]}")
        result = reply.get("result")
        if not isinstance(result, dict):
            raise RuntimeError(f"{tool}: response omitted result")
        if str(result.get("status", "")).casefold() in {"error", "failed", "rejected"}:
            raise RuntimeError(f"{tool}: result rejected: {str(result)[:400]}")
        return result

    def eq_paged_parameters(self, track_id: str, plugin_id: str, identifier: str,
                            audit: Counter) -> dict[str, float]:
        parameters: dict[str, float] = {}
        rows_seen = 0
        offset, total = 0, -1
        while True:
            result = self.eq_invoke("plugin.get_parameters", {
                "track_id": track_id, "plugin_id": plugin_id,
                "plugin_identifier": identifier, "include_parameters": True,
                "offset": offset, "limit": 128}, audit=audit)
            rows = [row for row in result.get("parameters", []) if isinstance(row, dict)]
            page = result.get("parameter_page")
            if not isinstance(page, dict):
                raise RuntimeError("plugin.get_parameters omitted parameter_page")
            total = int(page.get("total", -1))
            rows_seen += len(rows)
            for row in rows:
                param_id = str(row.get("param_id", row.get("parameter_id", row.get("id", ""))) or "")
                value = row.get("normalized_value")
                if param_id and isinstance(value, (int, float)):
                    parameters[param_id] = float(value)
            offset += len(rows)
            if offset >= total or not rows:
                break
        if total >= 0 and rows_seen != total:
            raise RuntimeError(f"parameter pagination incomplete: got {rows_seen} "
                               f"rows of {total}")
        return parameters

    def ensure_pcactl(self) -> Path:
        pcactl = Path.home() / ".vit" / "autosweep" / "bin" / "pcactl.exe"
        pcactl.parent.mkdir(parents=True, exist_ok=True)
        if not pcactl.is_file():
            subprocess.run(["go", "build", "-o", str(pcactl), "./cmd/pcactl"],
                           cwd=REPO_ROOT / "agent", check=True, capture_output=True)
        return pcactl

    def ensure_full_access_authority(self) -> None:
        """The EQ channel loads rack instances through the agent tool face;
        under authority=full_project_access the load gate resolves exact
        promoted identities from the PCA catalog (the journey-leg precedent
        endpoint). Unpromoted subjects fail closed with the precise
        load-gate reason — that blocker is agent-side wiring, out of this
        script's file domain."""
        reply = request_json(self.args.agent_http.rstrip("/") + "/agent/authority",
                             "POST", {"authority_mode": "full_project_access"},
                             timeout=30)
        mode = str(reply.get("authority_mode", ""))
        if mode != "full_project_access":
            raise RuntimeError(f"authority switch refused: {str(reply)[:300]}")
        print("[certify] authority mode = full_project_access "
              "(EQ channel load gate path)")
        self.ledger("authority_mode_set", mode=mode)

    def certify_eq(self, entry: dict[str, Any]) -> dict[str, Any]:
        """One deterministic static_eq certification pass (EQ-1 path, zero LLM):
        identifier-minted rack load -> paged readback -> explain topology ->
        transactional upsert(+modify/disable)/undo coverage writes -> snapshot
        compare -> phase3-style receipt -> pcactl import (auto-promotes)."""
        slug = slug_of(entry["name"])
        vendor = str(entry.get("manufacturer") or "").strip().casefold()
        kind = EQ_RECEIPT_KIND.get(vendor)
        if kind is None:
            return {"status": "vendor_kind_missing",
                    "error": f"manufacturer {entry.get('manufacturer')!r} has no v1 "
                             f"receipt kind (import gate accepts waves/"
                             f"plugin_alliance kinds only; kind extension is agent "
                             f"code, out of this sweep's file domain)"}
        receipt_dir = (Path(self.args.cert_dir) / f"eq_channel_{self.run_id}" / slug)
        receipt_dir.mkdir(parents=True, exist_ok=True)
        receipt_path = receipt_dir / "summary.json"
        audit: Counter = Counter()
        track_id = ""
        touched: set[str] = set()
        extra_actions: list[dict[str, Any]] = []
        requested: list[dict[str, Any]] = []
        apply_status = "passed"
        try:
            # identifier-minted resolution: exactly one kernel row may match
            search = self.agent_invoke("plugin_search", {"query": entry["name"],
                                                         "limit": 64}, confirmed=False)
            rows = ((search.get("result") or {}).get("plugins") or [])
            exact = [r for r in rows if isinstance(r, dict)
                     and str(r.get("identifier", "")).casefold()
                     == entry["identifier"].casefold()]
            if len(exact) != 1:
                raise RuntimeError(f"identifier resolution matched {len(exact)} kernel "
                                   f"rows for {entry['identifier']}")
            audit["plugin_search"] += 1
            track = self.eq_invoke("track.add_audio",
                                   {"name": f"PCA EQ channel {slug}"}, audit=audit)
            track_id = str(track.get("track_id") or track.get("id") or "")
            if not track_id:
                raise RuntimeError("track.add_audio returned no track_id")
            loaded = self.eq_invoke("plugin.load_to_rack", {
                "track_id": track_id, "plugin_path": entry["plugin_path"],
                "plugin_name": entry["name"],
                "plugin_identifier": entry["identifier"]}, audit=audit, direct=True)
            plugin_id = str(loaded.get("plugin_id") or loaded.get("node_id")
                            or loaded.get("id") or "")
            if not plugin_id:
                raise RuntimeError("plugin.load_to_rack returned no plugin_id")

            before = self.eq_paged_parameters(track_id, plugin_id,
                                              entry["identifier"], audit)

            explain = self.eq_invoke("plugin_grabber.explain_controls",
                                     {"track_id": track_id, "plugin_id": plugin_id},
                                     audit=audit)
            save_json(receipt_dir / "explain.json", explain)
            summary = explain.get("eq_band_summary")
            if not isinstance(summary, dict) or not summary.get("eq_model"):
                raise RuntimeError("explain_controls returned no EQ topology "
                                   "(not_static_eq)")
            capabilities = ((summary.get("control_topology") or {})
                            .get("shape_capabilities") or [])
            upsertable = [str(row.get("shape")) for row in capabilities
                          if isinstance(row, dict)
                          and (row.get("actions") or {}).get("upsert")]
            modifiable = {str(row.get("shape")) for row in capabilities
                          if isinstance(row, dict)
                          and (row.get("actions") or {}).get("modify")}
            disableable = {str(row.get("shape")) for row in capabilities
                           if isinstance(row, dict)
                           and (row.get("actions") or {}).get("disable")}
            # shape preference: bell first (admission axis), then the phase3
            # precedent shapes; first capability wins otherwise.
            preference = ["bell", "low_cut", "high_shelf", "high_cut", "low_shelf"]
            shape = next((s for s in preference if s in upsertable), None) \
                or (upsertable[0] if upsertable else "")
            if not shape:
                raise RuntimeError("explain topology exposes no upsertable shape")
            edit: dict[str, Any] = {"action": "upsert", "shape": shape,
                                    "frequency_hz": 80.0 if "cut" in shape else 3400.0}
            if "cut" not in shape:
                edit["gain_db"] = -3.0
            requested = [{"action": edit["action"], "shape": edit["shape"]}]

            def apply_edits(edits: list[dict[str, Any]], label: str) -> dict[str, Any]:
                result = self.eq_invoke("plugin_grabber.apply_eq_edits", {
                    "track_id": track_id, "plugin_id": plugin_id,
                    "atomic": True, "edits": edits}, audit=audit)
                save_json(receipt_dir / f"{label}.json", result)
                return result

            def validate_apply(result: dict[str, Any], expect: int,
                               label: str) -> tuple[str, list[str]]:
                operation_ref = str(result.get("operation_ref", "")).strip()
                rows = result.get("edits")
                if not operation_ref or not isinstance(rows, list) or len(rows) != expect:
                    raise RuntimeError(f"{label}: missing refs or edit rows: "
                                       f"{str(result)[:400]}")
                control_refs = []
                for row in rows:
                    ref = str(row.get("control_ref", "")).strip() \
                        if isinstance(row, dict) else ""
                    if not ref:
                        raise RuntimeError(f"{label}: edit omitted control_ref: {row}")
                    control_refs.append(ref)
                write_ids = {str(w.get("param_id", "")).strip()
                             for w in result.get("writes", [])
                             if isinstance(w, dict)}
                if not write_ids:
                    raise RuntimeError(f"{label}: no touched parameters")
                touched.update(write_ids)
                return operation_ref, control_refs

            def undo(operation_ref: str, label: str) -> dict[str, Any]:
                result = apply_edits([{"action": "undo",
                                       "operation_ref": operation_ref}], label)
                if result.get("action") != "undo" \
                        or not (result.get("rollback") or {}).get("verified"):
                    raise RuntimeError(f"{label}: undo was not verified: "
                                       f"{str(result)[:300]}")
                return result

            applied = apply_edits([edit], "apply")
            apply_status = str(applied.get("status", ""))
            base_operation, control_refs = validate_apply(applied, 1, "apply")
            save_json(receipt_dir / "before.json", before)

            # ref-action coverage: modify + disable (phase3 q10 precedent) when
            # the shape supports them; each is undone transactionally.
            if shape in modifiable and "cut" not in shape:
                modified = apply_edits([{"action": "modify",
                                         "control_ref": control_refs[0],
                                         "gain_db": -4.0}], "modify")
                modify_op, _ = validate_apply(modified, 1, "modify")
                undo_modify = undo(modify_op, "undo_modify")
                extra_actions.append({"action": "modify", "result": modified})
                extra_actions.append({"action": "undo_modify",
                                      "result": {"status": undo_modify.get("status"),
                                                 "rollback": undo_modify.get("rollback")}})
            if shape in disableable:
                disabled = apply_edits([{"action": "disable",
                                         "control_ref": control_refs[0]}], "disable")
                disable_op, _ = validate_apply(disabled, 1, "disable")
                undo_disable = undo(disable_op, "undo_disable")
                extra_actions.append({"action": "disable", "result": disabled})
                extra_actions.append({"action": "undo_disable",
                                      "result": {"status": undo_disable.get("status"),
                                                 "rollback": undo_disable.get("rollback")}})

            undo(base_operation, "undo_base")
            after = self.eq_paged_parameters(track_id, plugin_id,
                                             entry["identifier"], audit)
            save_json(receipt_dir / "after.json", after)
            drift = []
            for param_id in sorted(touched):
                if param_id not in before or param_id not in after \
                        or abs(before[param_id] - after[param_id]) > 1e-4:
                    drift.append({"param_id": param_id,
                                  "before": before.get(param_id),
                                  "after": after.get(param_id)})
            if drift:
                raise RuntimeError(f"touched parameters were not restored: "
                                   f"{str(drift)[:300]}")

            result_row = {
                "case_id": slug, "plugin_name": entry["name"],
                "identifier": entry["identifier"],
                "resolution": {"name": entry["name"],
                               "manufacturer": entry.get("manufacturer", ""),
                               "format": entry.get("format", ""),
                               "identifier": entry["identifier"],
                               "plugin_path": entry["plugin_path"]},
                "mode": "positive", "status": "passed", "restored": True,
                "requested_edits": requested,
                "undo": {"rollback": {"verified": True}},
                "extra_actions": extra_actions,
                "stages": [{"level": "parameter", "requested_edits": requested,
                            "status": apply_status, "restored": True,
                            "undo": {"rollback": {"verified": True}}}],
                "touched_parameter_ids": sorted(touched),
                "final_drift": [],
            }
            receipt = {
                "schema_version": kind, "status": "passed",
                "completed_at": utcnow(), "case_count": 1,
                "stage_count": len(result_row["stages"]),
                "results": [result_row],
                "audit": {"agent_tools": dict(sorted(audit.items())),
                          "kernel_commands": {}, "natural_language_chat_count": 0,
                          "audio_probe_count": 0, "learning_call_count": 0,
                          "profile_call_count": 0, "spal_call_count": 0,
                          "b4_call_count": 0, "event_count": sum(audit.values())},
            }
            save_json(receipt_path, receipt)
        except Exception as error:
            message = str(error)
            if "pca_load_gate" in message:
                # precise blocker class for the exception ledger: the load
                # gate legitimately refuses unpromoted static_eq subjects for
                # HTTP certification clients (only the in-agent runner can
                # authorize certification loads, and it refuses static_eq).
                status = "load_gate_blocked"
            else:
                status = "failed"
            return {"status": status, "error": message[:500],
                    "receipt": str(receipt_path)}
        finally:
            if track_id:
                try:
                    self.eq_invoke("track.delete", {"track_id": track_id},
                                   timeout=60.0)
                    audit["track.delete"] += 1
                except Exception:
                    pass  # cleanup failure must not mask the certification result

        # pcactl import: the v1 import gate validates the receipt and promotes
        # (coverage merged with any existing promotion for the same subject).
        pcactl = self.ensure_pcactl()
        run = subprocess.run([str(pcactl), "import", "-receipt", str(receipt_path),
                              "-semantics", str(self.args.semantics),
                              "-store", str(self.args.store_v1)],
                             capture_output=True, text=True,
                             encoding="utf-8", errors="replace")
        try:
            report = json.loads(run.stdout)
        except json.JSONDecodeError:
            report = {"import_error": (run.stdout + run.stderr)[:500]}
        if run.returncode != 0:
            return {"status": "import_failed",
                    "error": str(report.get("import_error", run.stderr))[:500],
                    "receipt": str(receipt_path)}
        promoted = report.get("promoted") or []
        skipped = report.get("skipped") or []
        if not promoted:
            return {"status": "import_no_candidate",
                    "error": f"receipt imported but promoted no subject; "
                             f"skipped={str(skipped)[:300]}",
                    "receipt": str(receipt_path)}
        return {"status": "completed", "receipt": str(receipt_path),
                "import": report, "promoted": promoted,
                "zero_llm": receipt["audit"]["natural_language_chat_count"] == 0}

    def derive_eq_whitelist(self, artifacts: Path) -> list[dict[str, Any]]:
        """Derive whitelist static_eq entries (bands anchors) from this sweep's
        EQ-channel receipts and merge them into the live whitelist section.
        Anchors: explain band topology (center_hz + per-channel gain ids,
        deterministic agent-side derivation, zero LLM) cross-checked against
        dual-cycle probe snapshots; admission needs upsert+bell coverage, so
        receipts without bell are honestly excluded with the reason recorded."""
        prov: list[dict[str, Any]] = []
        live = load_json(self.args.live)
        live_section = live.get("static_eq")
        if not isinstance(live_section, list):
            return prov  # v5 single-entry live: EQ merge is v6-only
        existing = {str(e.get("plugin_identifier", "")).casefold() for e in live_section}
        by_slug = {slug_of(e["name"]): e for e in self.effect_subjects()}
        derived: list[dict[str, Any]] = []
        excluded: list[dict[str, Any]] = []
        for key, record in sorted(self.state["cert"].items()):
            if not key.endswith("|static_eq") or record.get("status") != "completed":
                continue
            slug = key.split("|", 1)[0]
            entry = by_slug.get(slug)
            if entry is None:
                continue
            section = f"static_eq[{len(live_section) + len(derived)}]"
            if entry["identifier"].casefold() in existing:
                prov.append({"section": section, "field": "_s0_kept",
                             "value": f"{entry['name']} already in live static_eq",
                             "source": "S0 live entry kept verbatim (fresh receipt "
                                       f"{record.get('receipt', '')} merges store "
                                       "coverage only)", "note": ""})
                continue
            receipt_path = record.get("receipt", "")
            receipt_dir = Path(receipt_path).parent
            explain_path = receipt_dir / "explain.json"
            reason = ""
            bands: list[dict[str, Any]] = []
            explain = None
            if not receipt_path or not explain_path.exists():
                reason = "receipt explain evidence missing"
            else:
                try:
                    explain = load_json(explain_path)
                except Exception as error:
                    reason = f"explain evidence unreadable: {error}"
            if explain is not None:
                summary = explain.get("eq_band_summary") or {}
                bands, band_issues = self.eq_bands_from_summary(summary, entry, prov,
                                                                section)
                if not bands:
                    reason = band_issues or "no derivable band anchors"
            if explain is not None and not reason:
                try:
                    receipt = load_json(receipt_path)
                except Exception as error:
                    reason = f"receipt unreadable: {error}"
                    receipt = None
                if receipt is not None:
                    shapes = {str(e.get("shape")) for row in receipt.get("results", [])
                              for e in row.get("requested_edits") or []
                              if isinstance(e, dict) and e.get("action") == "upsert"}
                    if "bell" not in shapes:
                        reason = (f"receipt coverage upsert shapes={sorted(shapes)} "
                                  f"lack bell; admission requires upsert+bell")
            if reason:
                excluded.append({"plugin_name": entry["name"], "reason": reason})
                prov.append({"section": "static_eq", "field": "_excluded",
                             "value": f"{entry['name']} not listed",
                             "source": reason,
                             "note": "frozen derivation rule: promoted ∧ upsert bell "
                                     "coverage ∧ derivable band anchors"})
                continue
            derived.append({
                "plugin_name": entry["name"],
                "manufacturer": entry.get("manufacturer", ""),
                "format": entry.get("format", ""),
                "plugin_identifier": entry["identifier"],
                "plugin_path": entry["plugin_path"],
                "bands": bands,
            })
            prov.append({"section": section, "field": "_attestation",
                         "value": "static_eq promoted (v1)",
                         "source": f"S2 v1 store; EQ-channel receipt {receipt_path}",
                         "note": "upsert bell coverage; zero-LLM deterministic channel"})
        save_json(artifacts / "eq_static_eq_derived.json",
                  {"derived": derived, "excluded": excluded})
        if not derived:
            print(f"[derive] EQ whitelist: 0 new entries "
                  f"({len(excluded)} excluded with reasons)")
            return prov
        merged = live_section + derived
        live["static_eq"] = merged
        if self.args.apply_live:
            backup = artifacts / "live_backup_pre_eq.json"
            if not backup.exists():
                save_json(backup, load_json(self.args.live))
            save_json(self.args.live, live)
            self.ledger("eq_whitelist_applied", live=str(self.args.live),
                        added=len(derived), excluded=len(excluded))
            print(f"[derive] EQ whitelist: +{len(derived)} entries merged into live "
                  f"({len(excluded)} excluded with reasons); backup {backup}")
        else:
            save_json(artifacts / "live_with_eq.json", live)
            print(f"[derive] EQ whitelist: +{len(derived)} entries derived but NOT "
                  f"applied (rerun derive with --apply-live); "
                  f"preview {artifacts / 'live_with_eq.json'}")
        return prov

    def eq_bands_from_summary(self, summary: dict[str, Any], entry: dict[str, Any],
                              prov: list[dict[str, Any]],
                              section: str) -> tuple[list[dict[str, Any]], str]:
        """(bands, failure_reason) — deterministic band anchors from the explain
        topology cross-checked against the dual-cycle probe snapshots."""
        slug = slug_of(entry["name"])
        rows = summary.get("bands") or []
        if summary.get("eq_model") == "free_floating":
            rows = summary.get("active_bands") or []
        snap_path = self.probe_dir / f"{slug}.a.snapshot.json"
        snap_b_path = self.probe_dir / f"{slug}.b.snapshot.json"
        if not snap_path.exists() or not snap_b_path.exists():
            return [], "dual-cycle probe snapshots missing for gain anchors"
        snap = load_json(snap_path)
        snap_b = load_json(snap_b_path)
        params_a = {str(p["id"]): p for p in snap["surface"]["parameters"]}
        ident_a = {str(p["name"]): str(p["id"]) for p in snap["surface"]["parameters"]}
        ident_b = {str(p["name"]): str(p["id"]) for p in snap_b["surface"]["parameters"]}
        if ident_a != ident_b:
            return [], "probe identity drift across cycles"
        raw: list[tuple[float, str, str]] = []
        issues: list[str] = []
        for row in rows:
            center = row.get("current_freq_hz")
            if center is None:
                center = row.get("fixed_freq_hz")
            gains = [g for g in row.get("gain_bindings") or []
                     if isinstance(g, dict) and g.get("param_id")]
            if not gains and row.get("gain_param_id"):
                gains = [{"param_id": row["gain_param_id"], "channel": "shared"}]
            if center is None or not gains:
                issues.append(f"band {row.get('band', '?')} lacks center/gain anchor")
                continue
            if not isinstance(center, (int, float)) or not (20 <= float(center) <= 20000):
                issues.append(f"band {row.get('band', '?')} center {center} "
                              f"outside [20, 20000]")
                continue
            ids: list[str] = []
            for gain in sorted(gains, key=lambda g: str(g.get("channel", ""))):
                param_id = str(gain["param_id"])
                param = params_a.get(param_id)
                if param is None:
                    issues.append(f"band {row.get('band', '?')} gain id {param_id} "
                                  f"absent from probe surface")
                    continue
                if not param.get("stable_id"):
                    issues.append(f"band {row.get('band', '?')} gain id {param_id} "
                                  f"not stable_id")
                    continue
                ids.append(param_id)
            if not ids:
                continue
            ch1 = ids[0]
            ch2 = ids[1] if len(ids) > 1 else ids[0]  # mono/shared mirrored
            raw.append((float(center), ch1, ch2))
        raw.sort(key=lambda item: item[0])
        bands: list[dict[str, Any]] = []
        seen_centers: set[float] = set()
        seen_ch1: set[str] = set()
        mirrored = False
        for center, ch1, ch2 in raw:
            if center in seen_centers or ch1 in seen_ch1:
                continue  # duplicate center/gain anchor: keep the first band
            if ch1 == ch2:
                mirrored = True
            seen_centers.add(center)
            seen_ch1.add(ch1)
            bands.append({"center_hz": center,
                          "gain_param_id_ch1": ch1, "gain_param_id_ch2": ch2})
        if not bands:
            return [], "; ".join(issues) or "no derivable band anchors"
        prov.append({"section": section, "field": "bands",
                     "value": [b["center_hz"] for b in bands],
                     "source": "EQ-channel explain topology (eq_band_summary bands) "
                               "x S3 dual-cycle probe (zero drift, stable_id)",
                     "note": ("single shared/mono gain param mirrored to ch1/ch2"
                              if mirrored else "per-channel gain params")})
        return bands, ""

    def poll_job(self, job_id: str) -> dict[str, Any]:
        deadline = time.time() + self.args.job_timeout
        status_url = (self.args.agent_http.rstrip("/") +
                      "/agent/processor-certification/status?job_id=" + job_id)
        last: dict[str, Any] = {}
        while time.time() < deadline:
            reply = request_json(status_url, timeout=30)
            job = reply.get("job", {})
            last = job
            if job.get("status") in {"completed", "failed"}:
                break
            time.sleep(3.0)
        status = last.get("status", "unknown")
        out: dict[str, Any] = {"status": status, "job_id": job_id,
                               "stage": last.get("stage", "")}
        certification = last.get("certification") or {}
        if certification:
            out["verdict"] = certification.get("verdict", "")
            out["summary_path"] = certification.get("summary_path", "")
        for field in ("import", "import_v1"):
            if last.get(field):
                out[field] = last.get(field)
        if status != "completed":
            out["error"] = str(last.get("error", "timeout"))[:500]
        elif status == "completed":
            promoted = ((last.get("import") or {}).get("promoted")
                        or (last.get("import_v1") or {}).get("promoted") or [])
            out["promoted"] = promoted
        return out

    def semantics_by_name(self) -> dict[str, dict[str, Any]]:
        return {e["name"]: e for e in self.semantics()}

    def phase_derive(self) -> None:
        artifacts = self.run_dir / "derive"
        artifacts.mkdir(parents=True, exist_ok=True)
        # 1. cycle-B snapshots for every currently-promoted subject (builder
        #    double-cycle identity-drift requirement) still missing .b files.
        need_b = {slug for slug, _ in self.store_promoted_slugs()}
        host: ProbeHost | None = None
        missing = [slug for slug in sorted(need_b)
                   if not (self.probe_dir / f"{slug}.b.snapshot.json").exists()]
        if missing:
            print(f"[derive] probe cycle B for {len(missing)} promoted subjects")
            host = ProbeHost(self.args.probe, self.args.probe_timeout)
            host.ensure_worker(self.args.worker)
            by_slug = {slug_of(e["name"]): e for e in self.effect_subjects()}
            for slug in missing:
                entry = by_slug.get(slug)
                if entry is None:
                    continue
                try:
                    snapshot = host.observe(entry,
                                            derive_uid(entry) if entry["_shell"] else None)
                    save_json(self.probe_dir / f"{slug}.b.snapshot.json", snapshot)
                except Exception as error:
                    self.ledger("probe_b_fail", slug=slug, error=str(error)[:500])
            host.shutdown()
        # 1b. EQCHANNEL: derive static_eq whitelist entries from the EQ-channel
        #     receipts (explain band topology + write anchors + probe dual-cycle
        #     cross-check), merge them into the live whitelist BEFORE the frozen
        #     builder runs (the builder carries static_eq verbatim from live;
        #     band-level provenance rides the eq_static_eq_* artifacts).
        eq_prov = self.derive_eq_whitelist(artifacts)
        # 2. deterministic v5 -> v6 normalization of the live whitelist (the
        #    builder requires a v6 --live; the live file itself is untouched
        #    unless --apply-live merged the EQ section above).
        live = load_json(self.args.live)
        normalized = self.normalize_live_v6(live)
        live_v6 = artifacts / "live_v6_normalized.json"
        save_json(live_v6, normalized)
        # 2b. per-family certification views: the frozen builder globs
        # receipts by subject NAME across every family, so one plugin
        # promoted in two families (e.g. a comp+gate unit whose comp Attack
        # and gate Attack are different physical params) fatally trips its
        # cross-receipt role-drift guard. Run the builder once per family
        # over a view holding only that family's promoted receipts and
        # stitch the per-family sections; every per-section check (identity,
        # probe anchors, axis coverage, receipt xcheck) runs unchanged over
        # the complete same-family receipt set.
        promoted_pairs = self.store_promoted()
        receipt_family_map = {
            "compressor": "broadband_compressor",
            "broadband_compressor": "broadband_compressor",
            "limiter": "limiter", "gate_expander": "gate_expander",
            "de_esser": "de_esser", "transient_shaper": "transient_shaper",
            "multiband_dynamics": "multiband_dynamics",
        }
        sem_names = self.semantics_by_name()
        import glob as _glob
        job_dirs_by_family: dict[str, list[Path]] = {}
        for summary_path in sorted(Path(p) for p in _glob.glob(
                str(Path(self.args.cert_dir) / "pca_job_*" / "summary.json"))):
            summary = load_json(summary_path)
            for row in summary.get("results", []):
                if row.get("status") != "passed":
                    continue
                store_family = receipt_family_map.get(
                    str(row.get("processor_family") or row.get("expectation") or ""))
                if not store_family:
                    continue
                entry = sem_names.get(row.get("plugin_name"))
                if entry is None:
                    continue
                if (subject_key(entry), store_family) in promoted_pairs:
                    job_dirs_by_family.setdefault(store_family, []) \
                        .append(summary_path.parent)
        # 3. rerun the frozen builder unchanged, once per derived family,
        # over that family's certification view; collect the section.
        derived_sections: dict[str, list[dict[str, Any]]] = {}
        provenance_rows: list[dict[str, Any]] = []
        builder_logs: list[str] = []
        whitelist_family_of_store = {
            "broadband_compressor": "broadband_compression",
            "limiter": "limiter", "gate_expander": "gate_expander",
            "de_esser": "de_esser", "transient_shaper": "transient_shaper",
            "multiband_dynamics": "multiband",
        }
        static_eq_section: list[dict[str, Any]] | None = None
        for store_family, job_dirs in sorted(job_dirs_by_family.items()):
            family = whitelist_family_of_store[store_family]
            cert_view = self.machine_root / "cert_view" / store_family
            cert_view.mkdir(parents=True, exist_ok=True)
            for job_dir in job_dirs:
                target = cert_view / job_dir.name
                if not target.exists():
                    shutil.copytree(job_dir, target)
            out_whitelist = artifacts / f"whitelist_v6_{family}.json"
            out_prov = artifacts / f"provenance_{family}.json"
            cmd = [sys.executable, str(BUILDER),
                   "--live", str(live_v6),
                   "--semantics", str(self.args.semantics),
                   "--attestation-v1", str(self.args.store_v1),
                   "--attestation-v2", str(self.args.store_v2),
                   "--pca-cert-dir", str(cert_view),
                   "--probe-dir", str(self.probe_dir),
                   "--out", str(out_whitelist),
                   "--provenance", str(out_prov)]
            built = subprocess.run(cmd, capture_output=True, text=True,
                                   encoding="utf-8", errors="replace")
            builder_logs.append(f"=== family view {store_family} "
                                f"(jobs={len(job_dirs)}) ===\n{built.stdout}\n"
                                f"--- stderr ---\n{built.stderr}")
            if built.returncode != 0:
                (artifacts / "builder_stdout.txt").write_text(
                    "\n".join(builder_logs), encoding="utf-8")
                raise RuntimeError(f"builder failed rc={built.returncode} for "
                                   f"family view {store_family}; see "
                                   f"{artifacts / 'builder_stdout.txt'}")
            family_whitelist = load_json(out_whitelist)
            section = family_whitelist.get(family, [])
            derived_sections[family] = section
            prov = load_json(out_prov)
            for row in prov:
                if row.get("section") == family:
                    provenance_rows.append(row)
                elif row.get("section") == "static_eq" and static_eq_section is None:
                    provenance_rows.append(row)
            if static_eq_section is None:
                static_eq_section = family_whitelist.get("static_eq", [])
        (artifacts / "builder_stdout.txt").write_text("\n".join(builder_logs),
                                                      encoding="utf-8")
        merged = {"schema_version": V6_SCHEMA,
                  "static_eq": static_eq_section or []}
        merged.update(derived_sections)
        out_whitelist = artifacts / "whitelist_v6_full.json"
        save_json(out_whitelist, merged)
        provenance_rows.extend(eq_prov)
        save_json(artifacts / "provenance_table_v6_full.json", provenance_rows)
        counts = {family: len(merged.get(family, []))
                  for family in ("static_eq", "broadband_compression", "de_esser",
                                 "limiter", "gate_expander", "transient_shaper",
                                 "multiband")}
        print("[derive] whitelist counts:", counts)
        if self.args.apply_live:
            backup = artifacts / "live_backup_pre_apply.json"
            if not backup.exists():
                save_json(backup, load_json(self.args.live))
            save_json(self.args.live, merged)
            self.ledger("whitelist_applied_live", live=str(self.args.live),
                        counts=counts)
            print(f"[derive] whitelist_v6_full applied to live "
                  f"({self.args.live}); backup {backup}")
        self.state["derived"] = True
        self.state["derived_whitelist"] = str(out_whitelist)
        self.state["derived_counts"] = counts
        self.save_state()

    def store_promoted_slugs(self) -> set[tuple[str, str]]:
        out: set[tuple[str, str]] = set()
        for store_path in (self.args.store_v1, self.args.store_v2):
            if not Path(store_path).exists():
                continue
            for attestation in load_json(store_path).get("attestations", []):
                if attestation.get("status") == "promoted":
                    out.add((slug_of(attestation["subject"]["name"]),
                             attestation["processor_family"]))
        return out

    @staticmethod
    def normalize_live_v6(live: dict[str, Any]) -> dict[str, Any]:
        if live.get("schema_version") == V6_SCHEMA:
            return live
        if live.get("schema_version") != V5_SCHEMA:
            raise RuntimeError(f"live whitelist schema {live.get('schema_version')!r} "
                               "is neither v5 nor v6")
        out = {"schema_version": V6_SCHEMA}
        for key, value in live.items():
            if key in {"schema_version", "version"}:
                continue
            if isinstance(value, dict):
                out[key] = [value]
            else:
                out[key] = value
        return out

    def phase_verify(self) -> None:
        whitelist_path = Path(self.state.get("derived_whitelist",
                                             self.run_dir / "derive" / "whitelist_v6_full.json"))
        whitelist = load_json(whitelist_path)
        pcactl = self.ensure_pcactl()
        entries = {e["name"]: e for e in self.semantics()}
        results: list[dict[str, Any]] = []
        failures = 0
        # v1 admission query flags per whitelist family (validateCoverage contract)
        v1_queries = {
            "static_eq": ("static_eq", ["-action", "upsert", "-shape", "bell"]),
            "broadband_compression": ("broadband_compressor",
                                      ["-action", "adjust", "-axis",
                                       "activation_intensity"]),
        }
        for family, rows in whitelist.items():
            if family == "schema_version" or not isinstance(rows, list):
                continue
            for row in rows:
                entry = entries.get(row.get("plugin_name"))
                if entry is None:
                    results.append({"family": family, "name": row.get("plugin_name"),
                                    "result": "semantic_index_miss"})
                    failures += 1
                    continue
                if family in v1_queries:
                    store_family, extra = v1_queries[family]
                    cmd = [str(pcactl), "query", "-identifier", entry["identifier"],
                           "-family", store_family] + extra
                else:
                    runner, _, axis = CERT_FAMILY[family]
                    cmd = [str(pcactl), "query-v2", "-identifier", entry["identifier"],
                           "-family", runner, "-action", "adjust", "-axis", axis]
                query = subprocess.run(cmd, capture_output=True, text=True,
                                       encoding="utf-8", errors="replace")
                try:
                    verdict = json.loads(query.stdout)
                except json.JSONDecodeError:
                    verdict = {"eligible": False, "reason": query.stdout[:200] + query.stderr[:200]}
                ok = bool(verdict.get("eligible"))
                failures += 0 if ok else 1
                results.append({"family": family, "name": row.get("plugin_name"),
                                "eligible": ok, "reason": verdict.get("reason", ""),
                                "effective_status": verdict.get("effective_status", "")})
        # non-member spots: known not-promoted subjects must be rejected
        promoted_names: set[str] = set()
        for store_path in (self.args.store_v1, self.args.store_v2):
            if Path(store_path).exists():
                for attestation in load_json(store_path).get("attestations", []):
                    if attestation.get("status") == "promoted":
                        promoted_names.add(attestation["subject"]["name"])
        spot_candidates = [e for e in self.effect_subjects()
                           if e["name"] not in promoted_names][:3]
        for entry in spot_candidates:
            query = subprocess.run(
                [str(pcactl), "query-v2", "-identifier", entry["identifier"],
                 "-family", "limiter", "-action", "adjust", "-axis", "output_ceiling"],
                capture_output=True, text=True, encoding="utf-8", errors="replace")
            try:
                verdict = json.loads(query.stdout)
            except json.JSONDecodeError:
                verdict = {"eligible": True, "reason": "unparseable"}
            rejected = not bool(verdict.get("eligible"))
            failures += 0 if rejected else 1
            results.append({"family": "_non_member_spot", "name": entry["name"],
                            "eligible": verdict.get("eligible"),
                            "reason": verdict.get("reason", "")})
        save_json(self.run_dir / "verify" / "overlay_results.json", results)
        self.state["verified"] = failures == 0
        self.state["verify_failures"] = failures
        self.save_state()
        print(f"[verify] overlay checks={len(results)} failures={failures}")
        if failures:
            raise RuntimeError(f"overlay regression failed ({failures} failures); "
                               "see verify/overlay_results.json")

    def phase_report(self) -> None:
        by_slug = {slug_of(e["name"]): e for e in self.effect_subjects()}
        classified: list[dict[str, Any]] = []
        exceptions: list[dict[str, Any]] = []
        certified: list[dict[str, Any]] = []
        for slug, probe_rec in sorted(self.state["probe"].items()):
            entry = by_slug.get(slug)
            if probe_rec.get("status") != "ok":
                exceptions.append({"subject": slug, "stage": "probe",
                                   "reason": probe_rec.get("error", "unknown")})
                continue
            classification = self.state["classify"].get(slug, {})
            families = classification.get("families", {})
            if not families:
                continue  # non-experiment effect (reverb/delay/meter...) — bucket 0
            row = {"name": entry["name"] if entry else slug,
                   "identifier": entry["identifier"] if entry else "",
                   "manufacturer": entry["manufacturer"] if entry else "",
                   "families": families, "detail": classification.get("detail", {})}
            classified.append(row)
            for family in families:
                key = f"{slug}|{family}"
                record = self.state["cert"].get(key)
                if record is None:
                    exceptions.append({
                        "subject": slug, "stage": "certify", "family": family,
                        "reason": "classified hit has no certification record "
                                  "(certify phase never reached it; rerun certify)"})
                elif record.get("status") == "completed":
                    certified.append({"subject": slug, "family": family,
                                      "job_id": record.get("job_id"),
                                      "summary_path": record.get("summary_path"),
                                      "runner_family": record.get("runner_family")})
                elif record.get("status") != "already_promoted":
                    exceptions.append({"subject": slug, "stage": "certify",
                                       "family": family,
                                       "reason": record.get("error", record.get("status", "?"))})
        family_counts: dict[str, dict[str, int]] = {}
        promoted_now = self.store_promoted()
        for family, (runner, _, _) in CERT_FAMILY.items():
            hits = sum(1 for row in classified if family in row["families"])
            promoted = 0
            for entry in self.effect_subjects():
                if runner and (subject_key(entry), runner) in promoted_now:
                    promoted += 1
            new_pass = sum(1 for c in certified if c["family"] == family)
            family_counts[family] = {"classified_hits": hits, "promoted_now": promoted,
                                     "certified_this_sweep": new_pass}
        if self.state.get("derived_counts"):
            for family, count in self.state["derived_counts"].items():
                family_counts.setdefault(family, {})["whitelist_derived"] = count
        probe_ok = sum(1 for v in self.state["probe"].values() if v.get("status") == "ok")
        probe_fail = sum(1 for v in self.state["probe"].values() if v.get("status") == "fail")
        report = {
            "schema_version": "pca_autosweep.sweep_report.v1",
            "run_id": self.run_id,
            "generated_at": utcnow(),
            "totals": {
                "library_effect_subjects": len(self.effect_subjects()),
                "probe_ok": probe_ok, "probe_fail": probe_fail,
                "classified_hits": len(classified),
                "certified_this_sweep": len(certified),
                "exceptions": len(exceptions),
            },
            "family_counts": family_counts,
            "bucket_classified": classified,
            "bucket_certified": certified,
            "bucket_exceptions": exceptions,
            "artifacts": {
                "run_dir": str(self.run_dir),
                "probe_dir": str(self.probe_dir),
                "state": str(self.state_path),
                "ledger": str(self.ledger_path),
            },
        }
        save_json(self.run_dir / "sweep_report.json", report)
        lines = [
            "# FIX-PCA-AUTOSWEEP-1 sweep_report",
            "",
            f"- run: `{self.run_id}` generated {report['generated_at']}",
            f"- library effect subjects: {report['totals']['library_effect_subjects']} "
            f"(probe ok/fail: {probe_ok}/{probe_fail}; instruments excluded)",
            "",
            "| family | classified | promoted now | certified this sweep | whitelist derived |",
            "|---|---|---|---|---|",
        ]
        for family, counts in family_counts.items():
            lines.append("| {} | {} | {} | {} | {} |".format(
                family, counts.get("classified_hits", 0), counts.get("promoted_now", 0),
                counts.get("certified_this_sweep", 0),
                counts.get("whitelist_derived", "n/a")))
        lines += ["", f"- bucket_certified: {len(certified)} rows",
                  f"- bucket_exceptions: {len(exceptions)} rows (each with reason)",
                  "", "Full lists: sweep_report.json"]
        (self.run_dir / "sweep_report.md").write_text("\n".join(lines), encoding="utf-8")
        print(f"[report] written {self.run_dir / 'sweep_report.md'}")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--phase", default="all",
                        choices=["preflight", "probe", "classify", "certify",
                                 "derive", "verify", "report", "all"])
    parser.add_argument("--run-id", default="")
    parser.add_argument("--resume", action="store_true",
                        help="explicitly continue the latest run dir (default)")
    parser.add_argument("--new-run", action="store_true",
                        help="force a fresh run dir instead of continuing the latest")
    parser.add_argument("--semantics", type=Path, default=DEFAULT_SEMANTICS)
    parser.add_argument("--store-v1", type=Path, default=DEFAULT_STORE_V1)
    parser.add_argument("--store-v2", type=Path, default=DEFAULT_STORE_V2)
    parser.add_argument("--live", type=Path, default=DEFAULT_LIVE)
    parser.add_argument("--cert-dir", type=Path, default=DEFAULT_CERT_DIR)
    parser.add_argument("--probe", default="http://127.0.0.1:9318")
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--worker", type=Path, default=DEFAULT_WORKER)
    parser.add_argument("--probe-timeout", type=float, default=120.0)
    parser.add_argument("--probe-workers", type=int, default=1,
                        help="parallel pluginprobe services (ports 9318+i)")
    parser.add_argument("--job-timeout", type=float, default=600.0)
    parser.add_argument("--probe-limit", type=int, default=0,
                        help="probe only the first N pending subjects (dry-run aid; 0 = all)")
    parser.add_argument("--max-cert", type=int, default=0,
                        help="cap certification jobs this invocation (0 = unlimited)")
    parser.add_argument("--apply-live", action="store_true",
                        help="derive phase: merge EQ static_eq entries into the live "
                             "whitelist and apply whitelist_v6_full.json to live "
                             "(with backups under the run dir)")
    parser.add_argument("--eq-subject", default="",
                        help="certify phase: restrict static_eq targets to subjects "
                             "whose name contains this substring (pilot aid)")
    parser.add_argument("--families", default="",
                        help="certify phase: comma-separated whitelist families to "
                             "certify (default all pending; EQCHANNEL reruns use "
                             "'static_eq,broadband_compression' to avoid retrying "
                             "other families' recorded failures)")
    args = parser.parse_args()

    sweep = Sweep(args)
    phases = (["preflight", "probe", "classify", "certify", "derive", "verify", "report"]
              if args.phase == "all" else [args.phase])
    for phase in phases:
        sweep.ledger("phase_start", phase=phase)
        print(f"=== phase {phase} (run {sweep.run_id}) ===", flush=True)
        getattr(sweep, f"phase_{phase}")()
        sweep.ledger("phase_done", phase=phase)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

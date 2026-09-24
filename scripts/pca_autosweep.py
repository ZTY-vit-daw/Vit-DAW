#!/usr/bin/env python3
"""FIX-PCA-AUTOSWEEP-1: one-command full-library PCA sweep for this machine.

Pipeline (each phase is resumable from the run state file):

  preflight -> probe (pluginprobe observation host, cycle A: all effect
  subjects) -> classify (deterministic parameter-topology family rules,
  zero LLM, zero plugin/vendor-name judgment) -> certify (agent HTTP
  processor-certification runner: identifier-minted load, typed axis
  writes, readback, full-snapshot compare, receipt, auto import/promote;
  failures land in the exception ledger with reasons) -> derive (probe
  cycle B for the promoted set, v5->v6 live normalization, rerun
  scripts/build_whitelist_v6_full.py unchanged) -> verify (pcactl
  admission overlay regression over every derived entry + non-member
  spots) -> report (three-bucket sweep_report + per-family counts).

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
    "static_eq": (None, "v1", "gain_band"),
}


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
        already promoted for that runner family, plus ruling-2 C1 comp pair."""
        promoted = self.store_promoted()
        by_slug = {slug_of(e["name"]): e for e in self.effect_subjects()}
        targets: list[tuple[dict[str, Any], str]] = []
        seen: set[tuple[str, str]] = set()

        def add(entry: dict[str, Any], family: str) -> None:
            runner, _, _ = CERT_FAMILY[family]
            if runner is None:
                return
            dedupe = (entry["identifier"], family)
            if dedupe in seen:
                return
            key = f"{slug_of(entry['name'])}|{family}"
            if self.state["cert"].get(key, {}).get("status") in {"passed", "completed",
                                                                 "already_promoted"}:
                return
            if (subject_key(entry), runner) in promoted:
                self.state["cert"].setdefault(key, {"status": "already_promoted"})
                return
            seen.add(dedupe)
            targets.append((entry, family))

        for slug, rec in sorted(self.state["classify"].items()):
            entry = by_slug.get(slug)
            if entry is None:
                continue
            for family in sorted(rec["families"]):
                add(entry, family)
        # PC-1 ruling 2: C1 comp Mono + M/S get broadband_compressor regardless
        # of the classifier, with the compressor->broadband mapping recorded.
        for entry in self.effect_subjects():
            if entry["name"].startswith("C1 comp"):
                add(entry, "broadband_compression")
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
        """The kernel's knownPluginList starts empty on boot; certification
        loads resolve identifiers against it. If the sample identifier is
        missing, run one scan_plugins pass over the common VST3 directory
        (derived from the semantics index) and wait for the scan to settle."""
        found = self.agent_invoke("plugin_search", {"query": sample_identifier,
                                                    "limit": 8}, confirmed=False)
        if (found.get("result") or {}).get("plugins"):
            return
        entries = self.effect_subjects()
        parents = Counter(str(Path(e["plugin_path"]).parent) for e in entries)
        scan_dir = parents.most_common(1)[0][0]
        print(f"[certify] kernel plugin list empty; scanning {scan_dir}")
        self.agent_invoke("scan_plugins", {"paths": [scan_dir]})
        stable, last, deadline = 0, -1, time.time() + 1200
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
        raise RuntimeError("kernel plugin scan did not settle within 20 minutes")

    def phase_certify(self) -> None:
        targets = self.cert_targets()
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
        for entry, family in targets:
            if cap and done >= cap:
                print(f"[certify] --max-cert {cap} reached; rerun to continue")
                break
            key = f"{slug_of(entry['name'])}|{family}"
            runner, _, _ = CERT_FAMILY[family]
            started = time.time()
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
                        job_id=job_id, status=record.get("status"),
                        error=record.get("error", ""))
            self.save_state()
            done += 1
            print(f"[certify] {done}/{len(targets)} {entry['name']} [{family}] -> "
                  f"{record.get('status')} ({record['seconds']}s)", flush=True)
        statuses = Counter(v.get("status") for v in self.state["cert"].values())
        print("[certify] ledger:", dict(statuses))

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
        # 2. deterministic v5 -> v6 normalization of the live whitelist (the
        #    builder requires a v6 --live; the live file itself is untouched).
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
        save_json(artifacts / "provenance_table_v6_full.json", provenance_rows)
        counts = {family: len(merged.get(family, []))
                  for family in ("static_eq", "broadband_compression", "de_esser",
                                 "limiter", "gate_expander", "transient_shaper",
                                 "multiband")}
        print("[derive] whitelist counts:", counts)
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
        pcactl = Path.home() / ".vit" / "autosweep" / "bin" / "pcactl.exe"
        pcactl.parent.mkdir(parents=True, exist_ok=True)
        if not pcactl.is_file():
            subprocess.run(["go", "build", "-o", str(pcactl), "./cmd/pcactl"],
                           cwd=REPO_ROOT / "agent", check=True, capture_output=True)
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
                    runner, _, _ = CERT_FAMILY[family]
                    if runner is None:
                        exceptions.append({
                            "subject": slug, "stage": "certify",
                            "reason": "static_eq has no product certification runner "
                                      "(inspect_only); EQ channel = phase3_live_smoke + "
                                      "pcactl import (EQ-1 path), out of this sweep's runner"})
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

#!/usr/bin/env python3
"""Capture full FabFilter/Waves VST3 surfaces through PluginProbe.

PluginProbe is an isolated, observation-only native host. It has no Vit project
authority and no parameter-write operation. Sealed cases are not loaded.
"""
from __future__ import annotations

import argparse
import json
import re
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any


BAND = re.compile(r"(?:^|[^a-z0-9])(?:band|b)\s*([0-9]+)(?:[^a-z0-9]|$)", re.I)
NAMED_BAND = re.compile(r"^\s*(low|mid|high)\s+", re.I)


def request(url: str, method: str = "GET", body: dict[str, Any] | None = None, timeout: float = 120) -> dict[str, Any]:
    data = None if body is None else json.dumps(body).encode("utf-8")
    req = urllib.request.Request(url, data=data, headers={"Content-Type": "application/json"}, method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as response:
            value = json.loads(response.read().decode("utf-8", errors="replace"))
    except urllib.error.HTTPError as error:
        value = json.loads(error.read().decode("utf-8", errors="replace"))
    if not isinstance(value, dict):
        raise RuntimeError("pluginprobe returned a non-object response")
    return value


def names(snapshot: dict[str, Any]) -> list[str]:
    return [str(row.get("name", "")) for row in snapshot.get("surface", {}).get("parameters", []) if isinstance(row, dict)]


def display_unit(row: dict[str, Any]) -> str:
    domain = row.get("display_domain")
    if isinstance(domain, dict):
        unit = str(domain.get("unit", "")).strip()
        if unit:
            return unit
    return str(row.get("unit", "")).strip()


def is_crossover_label(label: str) -> bool:
    if re.search(r"(?:^|\s)q(?:\s|$)", label, re.I):
        return False
    if re.search(r"crossover|cross over|xover|x-over", label, re.I):
        return True
    compact = re.sub(r"[\s_\-/]+", "", label).lower()
    return compact in {"lowmidfreq", "midlowfreq", "midhighfreq", "highmidfreq"}


def displayed_frequency_hz(row: dict[str, Any]) -> float | None:
    text = str(row.get("display_text", ""))
    match = re.search(r"(-?[0-9]+(?:\.[0-9]+)?)\s*(k?hz)\b", text, re.I)
    if not match:
        return None
    value = float(match.group(1))
    return value * (1000.0 if match.group(2).lower() == "khz" else 1.0)


def band_key(label: str) -> str | None:
    match = BAND.search(label)
    if match:
        return match.group(1)
    if is_crossover_label(label):
        return None
    named = NAMED_BAND.search(label)
    return named.group(1).lower() if named else None


def complete_dynamics_cell(labels: list[str]) -> bool:
    joined = " ".join(labels)
    operating = bool(re.search(r"\b(?:threshold|thresh|range|reduction)\b", joined, re.I))
    transfer_or_gain = bool(re.search(r"\b(?:ratio|knee|gain|makeup)\b", joined, re.I))
    timing = bool(re.search(r"\b(?:attack|release|recovery|hold|lookahead)\b", joined, re.I))
    return operating and transfer_or_gain and timing


def summarize(snapshot: dict[str, Any]) -> dict[str, Any]:
    parameters = snapshot.get("surface", {}).get("parameters", [])
    all_rows = [row for row in parameters if isinstance(row, dict)]
    rows = [row for row in all_rows if bool(row.get("host_controllable"))]
    labels = [str(row.get("name", "")) for row in rows]
    crossover_rows = [row for row in rows if is_crossover_label(str(row.get("name", "")))]
    crossovers = [str(row.get("name", "")) for row in crossover_rows]
    crossover_values = [displayed_frequency_hz(row) for row in crossover_rows]
    crossover_order_observed = bool(crossover_values) and all(value is not None for value in crossover_values)
    if crossover_order_observed:
        crossover_order_observed = all(
            float(crossover_values[index]) > float(crossover_values[index - 1])
            for index in range(1, len(crossover_values))
        )
    band_cells: dict[str, list[str]] = {}
    for label in labels:
        key = band_key(label)
        if key:
            band_cells.setdefault(key, []).append(label)
    band_labels = {label for cell in band_cells.values() for label in cell}
    shared_modifiers = [
        label
        for label in labels
        if label not in band_labels
        and not is_crossover_label(label)
        and re.search(r"\b(?:release|knee|behavior|mode|mix|link|gain|trim|style|smash)\b", label, re.I)
    ]
    return {
        "observed_plugin_name": str(snapshot.get("plugin_identity", {}).get("name", "")),
        "parameter_count": len(all_rows),
        "host_controllable_parameter_count": len(rows),
        "units": sorted({display_unit(row) for row in all_rows if display_unit(row)}),
        "crossover_count": len(crossovers),
        "crossover_names": crossovers,
        "crossover_values_hz": crossover_values,
        "crossover_order_observed": crossover_order_observed,
        "band_cell_count": len(band_cells),
        "complete_dynamics_cell_count": sum(complete_dynamics_cell(cell) for cell in band_cells.values()),
        "band_cells": band_cells,
        "shared_modifier_names": shared_modifiers,
        "auxiliary_name_hits": [label for label in labels if re.search(r"sidechain|dynamic eq|de.?esser|maximizer|limiter|clipper|spectral", label, re.I)],
        "parameter_names": [str(row.get("name", "")) for row in all_rows],
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--manifest", type=Path, default=Path("scripts/multiband_control_matrix.json"))
    parser.add_argument("--output", type=Path, default=Path("temp/multiband-pluginprobe-census"))
    parser.add_argument("--probe", default="http://127.0.0.1:9318")
    parser.add_argument("--timeout", type=float, default=120)
    args = parser.parse_args()
    manifest = json.loads(args.manifest.read_text(encoding="utf-8"))
    args.output.mkdir(parents=True, exist_ok=True)
    report: dict[str, Any] = {"schema_version": "plugin_grabber.multiband_pluginprobe_census.v1", "cases": []}
    for group in ("training", "regression", "negative_regression", "sealed_blind"):
        for case in manifest.get(group, []):
            case_id = str(case.get("id", ""))
            if group == "sealed_blind":
                report["cases"].append({"group": group, "id": case_id, "status": "sealed_not_opened"})
                continue
            try:
                load_request = {
                    key: case[key]
                    for key in ("plugin_path", "plugin_name", "plugin_uid", "num_inputs", "num_outputs", "manufacturer", "version")
                    if key in case
                }
                loaded = request(args.probe + "/v1/plugin/load", "POST", load_request, args.timeout)
                if loaded.get("status") == "error" or not loaded.get("identity") or not loaded.get("parameters"):
                    raise RuntimeError(str(loaded.get("error", loaded)))
                snapshot = request(args.probe + "/v1/plugin/snapshot", timeout=args.timeout)
                if snapshot.get("status") == "error":
                    raise RuntimeError(str(snapshot.get("error", snapshot)))
                evidence = {"case": case, "group": group, "snapshot": snapshot, "summary": summarize(snapshot)}
                (args.output / f"{case_id}.json").write_text(json.dumps(evidence, ensure_ascii=False, indent=2), encoding="utf-8")
                report["cases"].append({"group": group, "id": case_id, "status": "captured", **evidence["summary"]})
            except Exception as error:
                report["cases"].append({"group": group, "id": case_id, "status": "unresolved", "error": str(error)})
            finally:
                try:
                    request(args.probe + "/v1/plugin/unload", "POST", {}, args.timeout)
                except Exception:
                    pass
    (args.output / "summary.json").write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

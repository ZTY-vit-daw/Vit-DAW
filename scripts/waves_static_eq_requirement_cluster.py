#!/usr/bin/env python3
"""Symbolically cluster generic static-EQ control requirements over Waves evidence.

This experiment is deliberately request- and section-oriented.  It consumes the
read-only phase-one topology signatures, never loads a plug-in, never writes a
parameter, and never calls the production recognizer.
"""
from __future__ import annotations

import argparse
import csv
import hashlib
import json
import math
import re
from collections import Counter, defaultdict
from pathlib import Path
from typing import Any, Iterable


SHAPES = ("bell", "low_shelf", "high_shelf", "low_cut", "high_cut")
ACTIONS = ("upsert", "modify", "disable", "remove", "undo")
STATUS_RANK = {"rejected": 0, "quantized": 1, "exact": 2}
NUMERIC_ROLE_FIELDS = {
    "frequency_hz": "frequency",
    "gain_db": "gain",
    "q": "q",
    "slope_db_per_oct": "slope",
}

REJECTION_CODES = {
    "no_candidate_section": "No local section is a candidate for this shape.",
    "shape_not_provably_reachable": "The requested filter shape is not proven by names, enums, or local structure.",
    "frequency_anchor_unavailable": "Neither a writable frequency binding nor a recoverable fixed anchor exists.",
    "required_gain_binding_absent": "Bell and shelf requests require a gain binding.",
    "required_q_binding_absent": "The request explicitly supplied Q but the selected section exposes no Q binding.",
    "required_slope_binding_absent": "The request explicitly supplied slope but the selected cut exposes no slope binding.",
    "gain_law_not_arbitrary_bipolar": "The gain law cannot prove arbitrary positive and negative dB targets.",
    "coupled_analog_network_excluded": "Split boost/attenuate or another coupled analog network is outside generic static EQ.",
    "dynamic_section_excluded": "Dynamic peripherals are local to this section, so static independence is not proven.",
    "stateful_surface_dependency_unresolved": "Capture, match, learn, or calibration state is present and independence is unproven.",
    "linked_stereo_contract_incomplete": "The linked-stereo role bindings are asymmetric or incomplete.",
    "repeated_bank_address_unstable": "A repeated bank has no stable address key.",
    "sidechain_or_detector_section_excluded": "A compressor side-chain or detector filter is not an audible main-EQ section.",
    "physical_domain_unavailable": "An explicitly requested physical value has no recoverable domain or enum mapping.",
    "frequency_out_of_reachable_domain": "The target frequency is outside every proven section domain.",
    "gain_out_of_reachable_domain": "The target gain is outside the proven physical domain.",
    "q_out_of_reachable_domain": "The target Q is outside the proven physical domain.",
    "slope_out_of_reachable_domain": "The target slope is outside the proven physical domain.",
    "activation_binding_unavailable": "The section has no proven disable operation.",
    "section_not_deallocatable": "Only an allocatable slot can be removed; resident and anchored sections can only be disabled or restored.",
    "forward_control_unavailable": "No supported forward edit exists from which an undo operation could have been journalled.",
    "valid_control_ref_required": "Modify, remove, and precise disable operations require a current topology-scoped control_ref.",
    "valid_operation_ref_required": "Undo requires a valid operation_ref and an unchanged topology generation.",
}


def read_json(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8-sig"))
    if not isinstance(value, dict):
        raise RuntimeError(f"{path} is not a JSON object")
    return value


def write_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2), encoding="utf-8")


def canonical_json(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"))


def hash_json(value: Any) -> str:
    return hashlib.sha256(canonical_json(value).encode("utf-8")).hexdigest()


def compact(value: str) -> str:
    return re.sub(r"[^a-z0-9]+", " ", value.casefold()).strip()


def key_direction(value: str) -> str:
    tokens = set(compact(value).split())
    if tokens & {"low", "lo", "lf", "lows", "l"}:
        return "low"
    if tokens & {"high", "hi", "hf", "highs", "h"}:
        return "high"
    return "unknown"


def outer_band_direction(value: str) -> str:
    tokens = set(compact(value).split())
    if tokens & {"mid", "middle", "lmf", "hmf", "tone"}:
        return "unknown"
    return key_direction(value)


def shape_from_text(value: str, direction: str = "unknown") -> set[str]:
    text = compact(value)
    joined = text.replace(" ", "")
    out: set[str] = set()
    if any(token in joined for token in ("highpass", "hipass", "hpf", "lowcut", "locut")):
        out.add("low_cut")
    if any(token in joined for token in ("lowpass", "lopass", "lpf", "highcut", "hicut")):
        out.add("high_cut")
    if any(token in joined for token in ("bell", "peaking", "parametric", "pqbell", "peak")):
        out.add("bell")
    if "lowshelf" in joined or "loshelf" in joined:
        out.add("low_shelf")
    if "highshelf" in joined or "hishelf" in joined:
        out.add("high_shelf")
    if text == "shelf" or ("shelf" in text and not out):
        if direction == "low":
            out.add("low_shelf")
        elif direction == "high":
            out.add("high_shelf")
    return out


def observations_for_block(signature: dict[str, Any], block: dict[str, Any]) -> list[dict[str, Any]]:
    key = str(block.get("key", ""))
    direction = outer_band_direction(key)
    rows: list[dict[str, Any]] = []
    for row in signature.get("observations", []):
        if not isinstance(row, dict):
            continue
        band_key = str(row.get("band_key", ""))
        if band_key == key:
            rows.append(row)
            continue
        # Directional Bell switches are often published as a neighbouring
        # "low bell" / "high bell" block instead of sharing the gain block key.
        if direction != "unknown" and outer_band_direction(band_key) == direction:
            named = str(row.get("named_filter_kind", "unknown"))
            if named == "bell" and row.get("role") in {"filter_kind", "toggle_unknown"}:
                rows.append(row)
    return rows


def role_rows(rows: Iterable[dict[str, Any]], role: str) -> list[dict[str, Any]]:
    return [row for row in rows if row.get("role") == role]


def numeric_values_from_labels(labels: Iterable[str], role: str) -> list[float]:
    out: list[float] = []
    for label in labels:
        lower = str(label).casefold().replace(" ", "")
        match = re.search(r"[-+]?\d+(?:\.\d+)?", lower)
        if not match:
            continue
        value = float(match.group(0))
        if role == "frequency" and "khz" in lower:
            value *= 1000.0
        out.append(value)
    return out


def role_domain(rows: list[dict[str, Any]], role: str) -> dict[str, Any]:
    candidates = role_rows(rows, role)
    modes: set[str] = set()
    minimums: list[float] = []
    maximums: list[float] = []
    reachable: list[float] = []
    unavailable = False
    for row in candidates:
        domain = row.get("domain") if isinstance(row.get("domain"), dict) else {}
        scale = str(domain.get("scale", "unknown"))
        enum_count = int(domain.get("enum_count", 0) or 0)
        if scale == "enum" or enum_count > 0:
            modes.add("discrete")
            reachable.extend(numeric_values_from_labels(domain.get("enum_labels", []), role))
            if enum_count > 0 and role in {"frequency", "gain", "q", "slope"} and not reachable:
                unavailable = True
        else:
            modes.add("continuous")
        lo, hi = domain.get("minimum"), domain.get("maximum")
        if isinstance(lo, (int, float)) and isinstance(hi, (int, float)):
            minimums.append(float(min(lo, hi)))
            maximums.append(float(max(lo, hi)))
    mode = "absent"
    if modes == {"continuous"}:
        mode = "continuous"
    elif modes:
        mode = "discrete" if "discrete" in modes else sorted(modes)[0]
    return {
        "mode": mode,
        "binding_count": len(candidates),
        "minimum": min(minimums) if minimums else None,
        "maximum": max(maximums) if maximums else None,
        "reachable_values": sorted(set(reachable)),
        "physical_mapping_unavailable": unavailable,
    }


def channel_contract(block: dict[str, Any]) -> str:
    if "channel asymmetry" in block.get("issues", []):
        return "asymmetric"
    values = [tuple(channels) for channels in (block.get("channels_by_role") or {}).values()
              if channels]
    if not values or all(channels == ("shared",) for channels in values):
        return "shared"
    if all(channels == ("left", "right") for channels in values):
        return "mirrored_lr"
    if all(channels in {("shared",), ("left", "right")} for channels in values):
        return "mixed_shared_and_mirrored"
    return "asymmetric"


def reachable_shapes(block: dict[str, Any], rows: list[dict[str, Any]]) -> tuple[list[str], dict[str, list[str]]]:
    key = str(block.get("key", ""))
    direction = key_direction(key)
    evidence: dict[str, list[str]] = defaultdict(list)
    for kind in block.get("filter_kinds", []):
        if kind in SHAPES:
            evidence[kind].append("named_filter_kind")
    for row in rows:
        named = str(row.get("named_filter_kind", "unknown"))
        if named in SHAPES:
            evidence[named].append("named_filter_kind")
        if row.get("role") in {"filter_kind", "toggle_unknown"}:
            domain = row.get("domain") if isinstance(row.get("domain"), dict) else {}
            for label in domain.get("enum_labels", []):
                for shape in shape_from_text(str(label), direction):
                    evidence[shape].append("filter_kind_enum")
            for shape in shape_from_text(str(row.get("name", "")), direction):
                evidence[shape].append("filter_kind_name")
            # A directional "LF Bell" boolean proves Bell when on and the
            # complementary outer-band shelf when off.
            if named == "bell" and direction in {"low", "high"}:
                shelf = f"{direction}_shelf"
                evidence["bell"].append("directional_bell_toggle")
                evidence[shelf].append("directional_bell_toggle_complement")
        for shape in shape_from_text(str(row.get("name", "")), direction):
            evidence[shape].append("parameter_name")
    roles = block.get("role_counts") or {}
    has_frequency = roles.get("frequency", 0) > 0 or block.get("fixed_frequency_hz") is not None
    has_gain = roles.get("gain", 0) > 0
    if block.get("frequency_mode") == "fixed_label" and has_gain:
        evidence["bell"].append("fixed_graphic_anchor")
    key_tokens = set(compact(key).split())
    generic_band = bool(re.search(r"\b(?:band|node)\s+\d+\b", compact(key)))
    middle_band = bool(key_tokens & {"mid", "middle", "lmf", "hmf"})
    if has_frequency and has_gain and (roles.get("q", 0) > 0 or generic_band or middle_band):
        evidence["bell"].append("complete_peak_section")
    return sorted(evidence), {shape: sorted(set(values)) for shape, values in sorted(evidence.items())}


def synthesized_filter_blocks(signature: dict[str, Any], existing_keys: set[str]) -> list[dict[str, Any]]:
    """Recover gainless HP/LP sections omitted by the phase-one gain-centric block builder."""
    by_key: dict[str, list[dict[str, Any]]] = defaultdict(list)
    for row in signature.get("observations", []):
        if isinstance(row, dict):
            by_key[str(row.get("band_key", ""))].append(row)
    blocks: list[dict[str, Any]] = []
    for key, rows in by_key.items():
        if not key or key in existing_keys or not role_rows(rows, "frequency"):
            continue
        kinds: set[str] = set()
        for row in rows:
            named = str(row.get("named_filter_kind", "unknown"))
            if named in {"low_cut", "high_cut"}:
                kinds.add(named)
            kinds.update(shape_from_text(str(row.get("name", "")), key_direction(key))
                         & {"low_cut", "high_cut"})
        if not kinds:
            continue
        roles = Counter(str(row.get("role")) for row in rows)
        channels_by_role: dict[str, list[str]] = {}
        asymmetry = False
        for role in ("frequency", "gain", "q", "activation", "filter_kind", "slope"):
            channels = {str(row.get("channel", "shared")) for row in rows
                        if row.get("role") == role}
            if channels == {"shared", "right"}:
                values = ["left", "right"]
            else:
                values = sorted(channels)
            channels_by_role[role] = values
            if values and values != ["shared"] and values != ["left", "right"]:
                asymmetry = True
        frequency_domain = role_domain(rows, "frequency")
        activation_rows = role_rows(rows, "activation")
        current_labels = {str(row.get("current_text", "")).casefold().strip()
                          for row in activation_rows}
        if activation_rows:
            activation = {
                "strategy": "explicit_binding", "known": True,
                "current_state": "inactive" if current_labels
                    and current_labels <= {"off", "out", "disabled", "0"} else "active_or_mixed",
                "binding_count": len(activation_rows),
            }
        else:
            activation = {
                "strategy": "always_active_or_unexposed", "known": False,
                "current_state": "unknown", "binding_count": 0,
            }
        issues = ["channel asymmetry"] if asymmetry else []
        blocks.append({
            "key": key,
            "ordinal_start": min(int(row.get("ordinal", 0)) for row in rows),
            "ordinal_end": max(int(row.get("ordinal", 0)) for row in rows),
            "role_counts": dict(roles),
            "channels_by_role": channels_by_role,
            "channel_inference": {},
            "fixed_frequency_hz": None,
            "frequency_mode": "writable_discrete" if frequency_domain["mode"] == "discrete"
                else "writable_continuous",
            "activation": activation,
            "filter_kinds": sorted(kinds),
            "dynamic_peripheral_count": roles.get("dynamic_peripheral", 0),
            "enum_roles": sorted(role for role in ("frequency", "q", "slope")
                                 if role_domain(rows, role)["mode"] == "discrete"),
            "coupled_gain": False,
            "gain_law": "absent",
            "complete": not asymmetry,
            "issues": issues,
            "synthesized_for_phase2": True,
        })
    return blocks


def section_facts(signature: dict[str, Any]) -> list[dict[str, Any]]:
    observations = [row for row in signature.get("observations", []) if isinstance(row, dict)]
    stateful_count = sum(row.get("role") == "stateful_control" for row in observations)
    bank = signature.get("bank_axis") if isinstance(signature.get("bank_axis"), dict) else {}
    source_blocks = [block for block in signature.get("local_blocks", [])
                     if isinstance(block, dict)]
    source_blocks += synthesized_filter_blocks(
        signature, {str(block.get("key", "")) for block in source_blocks})
    source_blocks.sort(key=lambda block: (int(block.get("ordinal_start", 0)),
                                          str(block.get("key", ""))))
    sections: list[dict[str, Any]] = []
    for ordinal, block in enumerate(source_blocks):
        if not isinstance(block, dict):
            continue
        rows = observations_for_block(signature, block)
        shapes, evidence = reachable_shapes(block, rows)
        frequency_mode = str(block.get("frequency_mode", "opaque"))
        activation = block.get("activation") if isinstance(block.get("activation"), dict) else {}
        activation_strategy = str(activation.get("strategy", "unknown"))
        if frequency_mode == "fixed_label":
            addressing = "anchored"
        elif frequency_mode.startswith("writable"):
            addressing = "allocatable" if activation_strategy == "slot_lifecycle" else "resident"
        else:
            addressing = "unresolved"
        domains = {role: role_domain(rows, role)
                   for role in ("frequency", "gain", "q", "slope")}
        if addressing == "anchored":
            domains["frequency"] = {
                "mode": "fixed_anchor", "binding_count": 0,
                "minimum": block.get("fixed_frequency_hz"),
                "maximum": block.get("fixed_frequency_hz"),
                "reachable_values": ([block.get("fixed_frequency_hz")]
                                     if block.get("fixed_frequency_hz") is not None else []),
                "physical_mapping_unavailable": block.get("fixed_frequency_hz") is None,
            }
        facts = {
            "ordinal": ordinal,
            "section_ref_seed": hash_json({
                "relative_order": ordinal,
                "normalized_key": compact(str(block.get("key", ""))),
                "role_counts": block.get("role_counts", {}),
                "frequency_mode": frequency_mode,
            })[:20],
            "structural_key": str(block.get("key", "")),
            "addressing": addressing,
            "frequency_mode": frequency_mode,
            "fixed_frequency_hz": block.get("fixed_frequency_hz"),
            "roles": sorted(role for role, count in (block.get("role_counts") or {}).items()
                            if count),
            "domains": domains,
            "reachable_shapes": shapes,
            "shape_evidence": evidence,
            "activation": activation,
            "channel_contract": channel_contract(block),
            "gain_law": str(block.get("gain_law", "absent")),
            "coupled_gain": bool(block.get("coupled_gain")),
            "dynamic_peripheral_count": int(block.get("dynamic_peripheral_count", 0) or 0),
            "sidechain_or_detector_section": bool(
                set(compact(str(block.get("key", ""))).split())
                & {"sc", "sidechain", "detector"}),
            "stateful_surface_parameter_count": stateful_count,
            "unstable_bank_address": bool(bank.get("present") and not bank.get("address_key_observed")),
            "source_issues": list(block.get("issues", [])),
            "evidence_level": "structurally_inferred_not_write_tested",
        }
        facts["section_fact_signature_sha256"] = hash_json({
            key: facts[key] for key in (
                "ordinal", "addressing", "frequency_mode", "roles", "domains",
                "reachable_shapes", "activation", "channel_contract", "gain_law",
                "coupled_gain", "dynamic_peripheral_count",
                "sidechain_or_detector_section",
                "stateful_surface_parameter_count", "unstable_bank_address")
        })
        sections.append(facts)
    return sections


def domain_support(domain: dict[str, Any], target: float | None, role: str) -> tuple[str, str | None]:
    if domain.get("binding_count", 0) == 0 and domain.get("mode") != "fixed_anchor":
        return "rejected", f"required_{role}_binding_absent"
    if domain.get("physical_mapping_unavailable"):
        return "rejected", "physical_domain_unavailable"
    if target is not None and domain.get("mode") != "fixed_anchor":
        reachable = [float(value) for value in domain.get("reachable_values", [])
                     if isinstance(value, (int, float))]
        lo, hi = domain.get("minimum"), domain.get("maximum")
        if reachable:
            # Discrete/fixed domains are intentionally quantizable, but a
            # target outside the advertised edge range is not silently clamped.
            if target < min(reachable) or target > max(reachable):
                return "rejected", f"{role}_out_of_reachable_domain"
        elif isinstance(lo, (int, float)) and isinstance(hi, (int, float)):
            if target < float(lo) or target > float(hi):
                return "rejected", f"{role}_out_of_reachable_domain"
    return ("quantized", None) if domain.get("mode") in {"discrete", "fixed_anchor"} else ("exact", None)


def base_rejections(section: dict[str, Any], shape: str) -> list[str]:
    reasons: list[str] = []
    if shape not in section.get("reachable_shapes", []):
        reasons.append("shape_not_provably_reachable")
    if section.get("addressing") == "unresolved":
        reasons.append("frequency_anchor_unavailable")
    if section.get("stateful_surface_parameter_count", 0):
        reasons.append("stateful_surface_dependency_unresolved")
    if section.get("dynamic_peripheral_count", 0):
        reasons.append("dynamic_section_excluded")
    if section.get("sidechain_or_detector_section"):
        reasons.append("sidechain_or_detector_section_excluded")
    if section.get("unstable_bank_address"):
        reasons.append("repeated_bank_address_unstable")
    if section.get("channel_contract") == "asymmetric":
        reasons.append("linked_stereo_contract_incomplete")
    if shape in {"bell", "low_shelf", "high_shelf"}:
        if "gain" not in section.get("roles", []):
            reasons.append("required_gain_binding_absent")
        elif section.get("coupled_gain") or section.get("gain_law") == "split_boost_attenuate":
            reasons.append("coupled_analog_network_excluded")
        elif section.get("gain_law") != "bipolar":
            reasons.append("gain_law_not_arbitrary_bipolar")
    return sorted(set(reasons))


def selection_program(addressing: str, action: str) -> str:
    if action == "modify":
        return "resolve_control_ref"
    if addressing == "anchored":
        return "select_nearest_fixed_anchor"
    if addressing == "resident":
        return "select_resident_by_shape_and_reachability"
    if addressing == "allocatable":
        return "reuse_compatible_or_reserve_inactive_slot"
    return "unresolved"


def plan_for_section(section: dict[str, Any], shape: str, action: str,
                     fields: dict[str, Any] | None = None) -> dict[str, Any]:
    fields = fields or {}
    reasons = base_rejections(section, shape)
    addressing = str(section.get("addressing", "unresolved"))
    shape_roles = ["frequency"] + ([] if shape in {"low_cut", "high_cut"} else ["gain"])
    explicit_roles = [role for field, role in NUMERIC_ROLE_FIELDS.items() if field in fields]
    requested_roles = list(dict.fromkeys(shape_roles + explicit_roles))
    preconditions: list[str] = ["linked_stereo", "fresh_topology_generation"]
    status = "exact"
    conversion_modes: dict[str, str] = {}

    if action == "undo":
        if reasons:
            return rejected_plan(shape, action, reasons + ["forward_control_unavailable"], section)
        plan = {
            "action_program": "restore_transaction_snapshot",
            "addressing": "transaction_journal",
            "shape_program": "journalled_forward_edit",
            "selection_program": "resolve_operation_ref",
            "role_obligations": [],
            "conversion_modes": {},
            "write_order": ["restore_before_values_reverse_dependency_order"],
            "activation_program": "restore_recorded_state",
            "channel_program": "restore_recorded_bindings",
            "preconditions": ["valid_operation_ref", "unchanged_topology_generation"],
            "postcondition": "physical_readback_matches_before_snapshot",
            "rollback": "undo_attempt_is_itself_journalled",
        }
        return supported_plan("exact", shape, action, section, plan,
                              ["valid_operation_ref_required"])

    if action == "remove":
        preconditions.append("valid_control_ref")
        if addressing != "allocatable":
            reasons.append("section_not_deallocatable")
        if reasons:
            return rejected_plan(shape, action, reasons, section)
        plan = {
            "action_program": "deallocate_section",
            "addressing": addressing,
            "shape_program": shape,
            "selection_program": "resolve_control_ref",
            "role_obligations": ["activation"],
            "conversion_modes": {"activation": "categorical"},
            "write_order": ["activation_off_last"],
            "activation_program": "slot_lifecycle_deactivate",
            "channel_program": section.get("channel_contract"),
            "preconditions": preconditions,
            "postcondition": "slot_inactive_and_readback_verified",
            "rollback": "restore_atomic_snapshot",
        }
        return supported_plan("exact", shape, action, section, plan,
                              ["valid_control_ref_required"])

    if action == "disable":
        strategy = str((section.get("activation") or {}).get("strategy", "unknown"))
        if strategy not in {"explicit_binding", "implicit_in_domain", "slot_lifecycle"}:
            reasons.append("activation_binding_unavailable")
        if reasons:
            return rejected_plan(shape, action, reasons, section)
        plan = {
            "action_program": "disable_existing_section",
            "addressing": addressing,
            "shape_program": shape,
            "selection_program": "resolve_control_ref_or_unambiguous_shape",
            "role_obligations": ["activation"],
            "conversion_modes": {"activation": "categorical"},
            "write_order": ["activation_off"],
            "activation_program": strategy,
            "channel_program": section.get("channel_contract"),
            "preconditions": preconditions + ["valid_control_ref_or_unambiguous_shape"],
            "postcondition": "inactive_state_readback_verified",
            "rollback": "restore_atomic_snapshot",
        }
        return supported_plan("exact", shape, action, section, plan, [])

    if action == "modify":
        preconditions.append("valid_control_ref")
    if reasons:
        return rejected_plan(shape, action, reasons, section)

    for role in requested_roles:
        target = None
        for field, mapped in NUMERIC_ROLE_FIELDS.items():
            if mapped == role and isinstance(fields.get(field), (int, float)):
                target = float(fields[field])
                break
        result, reason = domain_support(section["domains"].get(role, {}), target, role)
        if result == "rejected":
            reasons.append(reason or "physical_domain_unavailable")
        else:
            conversion_modes[role] = str(section["domains"][role].get("mode"))
            if result == "quantized":
                status = "quantized"
    if reasons:
        return rejected_plan(shape, action, reasons, section)

    write_order = ["filter_kind_if_needed"]
    if addressing != "anchored":
        write_order.append("frequency")
    if "q" in requested_roles:
        write_order.append("q")
    if "slope" in requested_roles:
        write_order.append("slope")
    if "gain" in requested_roles:
        write_order.append("gain")
    activation_strategy = str((section.get("activation") or {}).get("strategy", "unknown"))
    if action == "upsert" and addressing == "allocatable":
        write_order.append("activation_last")
    elif action == "upsert" and activation_strategy == "explicit_binding":
        write_order.append("activation_if_inactive_last")
    plan = {
        "action_program": "upsert_static_filter" if action == "upsert" else "modify_static_filter",
        "addressing": addressing,
        "shape_program": shape,
        "selection_program": selection_program(addressing, action),
        "role_obligations": requested_roles,
        "conversion_modes": conversion_modes,
        "write_order": write_order,
        "activation_program": activation_strategy,
        "channel_program": section.get("channel_contract"),
        "preconditions": preconditions,
        "postcondition": "all_explicit_physical_fields_readback_verified",
        "rollback": "restore_atomic_snapshot",
    }
    limitations = ["valid_control_ref_required"] if action == "modify" else []
    return supported_plan(status, shape, action, section, plan, limitations)


def supported_plan(status: str, shape: str, action: str, section: dict[str, Any],
                   plan: dict[str, Any], limitations: list[str]) -> dict[str, Any]:
    return {
        "status": status,
        "shape": shape,
        "action": action,
        "section_ref_seed": section.get("section_ref_seed"),
        "section_ordinal": section.get("ordinal"),
        "addressing": plan.get("addressing"),
        "reasons": [],
        "precondition_notes": limitations,
        "control_plan_signature": plan,
        "control_plan_signature_sha256": hash_json(plan),
        "evidence_level": "structurally_inferred_not_write_tested",
    }


def rejected_plan(shape: str, action: str, reasons: Iterable[str],
                  section: dict[str, Any] | None = None) -> dict[str, Any]:
    normalized = sorted(set(reason for reason in reasons if reason)) or ["no_candidate_section"]
    return {
        "status": "rejected",
        "shape": shape,
        "action": action,
        "section_ref_seed": section.get("section_ref_seed") if section else None,
        "section_ordinal": section.get("ordinal") if section else None,
        "addressing": section.get("addressing", "unresolved") if section else "unresolved",
        "reasons": normalized,
        "precondition_notes": [],
        "control_plan_signature": None,
        "control_plan_signature_sha256": None,
        "evidence_level": "structurally_inferred_not_write_tested",
    }


def choose_plan(plans: list[dict[str, Any]], shape: str, action: str) -> dict[str, Any]:
    supported = [plan for plan in plans if plan["status"] != "rejected"]
    if supported:
        return sorted(supported, key=lambda plan: (
            -STATUS_RANK[plan["status"]], int(plan.get("section_ordinal") or 0),
            str(plan.get("control_plan_signature_sha256"))))[0]
    if plans:
        matching = [plan for plan in plans
                    if "shape_not_provably_reachable" not in plan.get("reasons", [])]
        candidates = matching or plans
        return sorted(candidates, key=lambda plan: (
            len(plan.get("reasons", [])), int(plan.get("section_ordinal") or 0),
            str(plan.get("section_ref_seed"))))[0]
    return rejected_plan(shape, action, ["no_candidate_section"])


def optional_role_status(section: dict[str, Any] | None, role: str) -> str:
    if not section:
        return "rejected"
    result, _ = domain_support(section.get("domains", {}).get(role, {}), None, role)
    return result


def analyze_signature(signature: dict[str, Any], corpus: dict[str, Any]) -> dict[str, Any]:
    sections = section_facts(signature)
    section_by_seed = {section["section_ref_seed"]: section for section in sections}
    matrix: list[dict[str, Any]] = []
    for shape in SHAPES:
        for action in ACTIONS:
            plans = [plan_for_section(section, shape, action) for section in sections]
            chosen = choose_plan(plans, shape, action)
            selected = section_by_seed.get(chosen.get("section_ref_seed"))
            row = {
                **chosen,
                "candidate_section_count": sum(shape in section.get("reachable_shapes", [])
                                               for section in sections),
                "q_capability": optional_role_status(selected, "q"),
                "slope_capability": optional_role_status(selected, "slope"),
            }
            matrix.append(row)
    intents: list[dict[str, Any]] = []
    for intent in corpus.get("canonical_intents", []):
        shape, action = str(intent["shape"]), str(intent["action"])
        fields = intent.get("fields") if isinstance(intent.get("fields"), dict) else {}
        plans = [plan_for_section(section, shape, action, fields) for section in sections]
        chosen = choose_plan(plans, shape, action)
        intents.append({"intent_id": intent["intent_id"], "fields": fields, **chosen})
    return {
        "schema_version": "static_eq.model_capability.v2",
        "case_id": signature.get("case_id"),
        "plugin_name": signature.get("plugin_name"),
        "surface_kind": signature.get("surface_kind"),
        "parameter_count": signature.get("parameter_count"),
        "source_topology_signature_sha256": signature.get("topology_signature_sha256"),
        "sections": sections,
        "shape_action_matrix": matrix,
        "canonical_intents": intents,
        "inference_policy": {
            "plugin_identity_used_as_feature": False,
            "manufacturer_used_as_feature": False,
            "production_recognizer_used": False,
            "audio_behavior_used": False,
            "parameter_writes_used": False,
            "section_and_request_oriented": True,
        },
    }


def refinement_trace(plan_rows: list[dict[str, Any]]) -> dict[str, Any]:
    unique: dict[str, dict[str, Any]] = {}
    for row in plan_rows:
        digest = row.get("control_plan_signature_sha256")
        plan = row.get("control_plan_signature")
        if digest and isinstance(plan, dict):
            unique[digest] = plan
    items = sorted(unique.items())
    groups: list[list[int]] = [list(range(len(items)))] if items else []
    dimensions = [
        "action_program", "addressing", "shape_program", "selection_program",
        "role_obligations", "conversion_modes", "write_order", "activation_program",
        "channel_program", "preconditions", "postcondition", "rollback",
    ]
    trace: list[dict[str, Any]] = []
    for dimension in dimensions:
        next_groups: list[list[int]] = []
        for group in groups:
            buckets: dict[str, list[int]] = defaultdict(list)
            for index in group:
                value = items[index][1].get(dimension)
                buckets[canonical_json(value)].append(index)
            next_groups.extend(buckets[key] for key in sorted(buckets))
        groups = next_groups
        trace.append({"dimension": dimension, "partition_count": len(groups)})
    partitions = [{
        "partition_id": f"P{number:03d}",
        "member_plan_signatures": [items[index][0] for index in group],
        "behavior": items[group[0]][1],
    } for number, group in enumerate(groups, start=1)]
    return {
        "schema_version": "static_eq.exact_plan_partitions.v1",
        "method": "exact_partition_refinement",
        "identity_features_used": False,
        "unique_input_plan_count": len(items),
        "trace": trace,
        "partition_count": len(partitions),
        "partitions": partitions,
    }


def addressing_minimality(corpus: dict[str, Any], waves_allocatable_count: int) -> dict[str, Any]:
    programs: list[dict[str, Any]] = []
    for prototype in corpus.get("identity_free_archetype_prototypes", []):
        mode = prototype["frequency_mode"]
        activation = prototype["activation_strategy"]
        addressing = "anchored" if mode == "fixed_label" else (
            "allocatable" if activation == "slot_lifecycle" else "resident")
        behavior = {
            "addressing": addressing,
            "frequency_write": addressing != "anchored",
            "section_allocation": "reserve_inactive_slot" if addressing == "allocatable" else "none",
            "activation_order": "activation_last" if addressing == "allocatable" else "if_needed",
            "frequency_selection": "nearest_fixed_anchor" if addressing == "anchored"
                else "reachable_domain",
        }
        programs.append({
            "prototype_id": prototype["prototype_id"],
            "behavior": behavior,
            "addressing_signature_sha256": hash_json(behavior),
        })
    witnesses: list[dict[str, Any]] = []
    for left_index, left in enumerate(programs):
        for right in programs[left_index + 1:]:
            differing = sorted(key for key in left["behavior"]
                               if left["behavior"][key] != right["behavior"][key])
            witnesses.append({
                "left": left["prototype_id"], "right": right["prototype_id"],
                "distinguishing_observables": differing,
                "merge_forbidden": bool(differing),
            })
    return {
        "schema_version": "static_eq.addressing_minimality.v1",
        "method": "behavioral_distinguishability",
        "prototype_count": len(programs),
        "partition_count": len({row["addressing_signature_sha256"] for row in programs}),
        "programs": programs,
        "pairwise_witnesses": witnesses,
        "minimal": len(programs) == 3 and all(row["merge_forbidden"] for row in witnesses),
        "waves_allocatable_observation_count": waves_allocatable_count,
        "allocatable_evidence_source": "identity_free_control_requirement_prototype",
    }


def write_matrices(output_dir: Path, models: list[dict[str, Any]]) -> None:
    shape_fields = [
        "case_id", "plugin_name", "surface_kind", "shape", "action", "status",
        "addressing", "candidate_section_count", "q_capability", "slope_capability",
        "section_ref_seed", "control_plan_signature_sha256", "reasons",
        "precondition_notes", "evidence_level",
    ]
    with (output_dir / "shape_action_matrix.csv").open("w", encoding="utf-8-sig", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=shape_fields)
        writer.writeheader()
        for model in models:
            for row in model["shape_action_matrix"]:
                writer.writerow({
                    "case_id": model["case_id"], "plugin_name": model["plugin_name"],
                    "surface_kind": model["surface_kind"],
                    **{key: row.get(key) for key in shape_fields if key not in {
                        "case_id", "plugin_name", "surface_kind", "reasons", "precondition_notes"}},
                    "reasons": ";".join(row.get("reasons", [])),
                    "precondition_notes": ";".join(row.get("precondition_notes", [])),
                })
    intent_fields = [
        "case_id", "plugin_name", "intent_id", "shape", "action", "status",
        "addressing", "section_ref_seed", "control_plan_signature_sha256",
        "reasons", "precondition_notes", "fields",
    ]
    with (output_dir / "canonical_intent_matrix.csv").open("w", encoding="utf-8-sig", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=intent_fields)
        writer.writeheader()
        for model in models:
            for row in model["canonical_intents"]:
                writer.writerow({
                    "case_id": model["case_id"], "plugin_name": model["plugin_name"],
                    **{key: row.get(key) for key in intent_fields if key not in {
                        "case_id", "plugin_name", "reasons", "precondition_notes", "fields"}},
                    "reasons": ";".join(row.get("reasons", [])),
                    "precondition_notes": ";".join(row.get("precondition_notes", [])),
                    "fields": canonical_json(row.get("fields", {})),
                })


def report_markdown(run_dir: Path, output_dir: Path, corpus: dict[str, Any],
                    models: list[dict[str, Any]], partitions: dict[str, Any],
                    minimality: dict[str, Any]) -> str:
    matrix_rows = [row for model in models for row in model["shape_action_matrix"]]
    intent_rows = [row for model in models for row in model["canonical_intents"]]
    statuses = Counter(row["status"] for row in matrix_rows)
    intent_statuses = Counter(row["status"] for row in intent_rows)
    lines = [
        "# 通用静态 EQ 控制需求重聚类与识别器详细设计（第二阶段）", "",
        "> 本阶段只重放第一阶段的真实只读参数证据并进行符号规划；没有加载新插件、写参数、主动音频探测、已退役映射链路、B4 或 Plugin Alliance 参数读取。", "",
        "## 1. 结论", "",
        f"- Waves 型号：{len(models)}。", f"- 通用 shape：{', '.join(SHAPES)}。",
        f"- 通用 action：{', '.join(ACTIONS)}。",
        f"- 逐 shape/action 行数：{len(matrix_rows)}；exact={statuses['exact']}，quantized={statuses['quantized']}，rejected={statuses['rejected']}。",
        f"- 规范自然语言意图重放：{len(intent_rows)}；exact={intent_statuses['exact']}，quantized={intent_statuses['quantized']}，rejected={intent_statuses['rejected']}。",
        f"- 精确 plan 分区：{partitions['partition_count']}；寻址最小分区：{minimality['partition_count']}；最小性成立={str(minimality['minimal']).lower()}。", "",
        "核心结论：`anchored_band`、`resident_section`、`allocatable_section` 不是插件类型，而是逐 section、逐请求推导的三个最小寻址原型。Bell/Shelf/Cut 与寻址原型正交。", "",
        "## 2. 控制需求边界", "",
        "纳入：Bell/Peak、Low Shelf、High Shelf、Low Cut、High Cut；upsert、modify、disable、remove、undo；linked stereo；默认原子批量。", "",
        "排除：动态 EQ 外设、耦合模拟音色网络、捕获/匹配/校准状态、M/S 或非对称声道控制、Notch。动态插件中的静态 section 只有在局部独立性可证明时才允许进入通用集合。", "",
        "## 3. 最小聚类方法", "",
        "聚类对象是 `控制需求 × section 事实` 产生的符号写入程序，不是插件。先生成身份无关 `control_plan_signature`，再依次按 action program、addressing、shape obligations、selection、conversion、write order、activation、channel、precondition、postcondition 和 rollback 做精确分区细化。全程不用距离阈值。", "",
        "### 三个寻址原型不可合并", "",
        "| 原型 | Frequency 写入 | 槽位分配 | 激活顺序 |", "|---|---:|---|---|",
    ]
    for row in minimality["programs"]:
        behavior = row["behavior"]
        lines.append(f"| {behavior['addressing']} | {behavior['frequency_write']} | {behavior['section_allocation']} | {behavior['activation_order']} |")
    lines += ["", "任意两类至少在 Frequency 是否写入、是否分配槽位或 Activation 是否必须最后写之一不同；合并后会改变前置条件或写入程序，因此三类是行为最小分区。Waves 本次没有 allocatable 实例，该类来自当前 Agent ‘创建频段’需求的身份无关契约原型，不使用 Plugin Alliance 数据。", "",
              "## 4. Request-agnostic 识别器", "", "```text",
              "flat parameter observations", "  -> role candidates + physical domains + enums",
              "  -> local section graph", "  -> per-shape reachability evidence",
              "  -> addressing / channel / activation / gain-law facts",
              "  -> EQControlTopology (no execution decision)", "```", "",
              "识别器仅输出事实。具体 section 选择、量化、字段硬约束和拒绝由 `EQIntentPlanner` 根据请求计算。插件级 `set_eq_point_supported` 应被逐 shape/action capability 替代。", "",
              "### Shape 完整性矩阵", "", "| Shape | 必需角色 | 可选角色 | 额外证据 |",
              "|---|---|---|---|", "| Bell/Peak | Frequency + 双极 Gain | Q | 显式 Bell、完整 peak section 或固定 graphic anchor |",
              "| Low Shelf | Frequency + 双极 Gain | Q | 明确 Low 方向与 Shelf 证据 |",
              "| High Shelf | Frequency + 双极 Gain | Q | 明确 High 方向与 Shelf 证据 |",
              "| Low Cut | Frequency | Q、Slope | 明确 Low Cut/High Pass；不要求 Gain |",
              "| High Cut | Frequency | Q、Slope | 明确 High Cut/Low Pass；不要求 Gain |", "",
              "## 5. Action 语义", "",
              "- `upsert`：按 shape 和可达域选择 resident/anchor，或在 allocatable surface 上复用/预留槽位。",
              "- `modify`：必须解析当前 topology generation 下的 `control_ref`，不得重新猜 band。",
              "- `disable`：只允许 explicit activation、implicit sentinel 或 slot lifecycle；无可证明关闭操作时拒绝。",
              "- `remove`：只适用于 allocatable slot；resident/anchored 只能 disable 或 undo。",
              "- `undo`：使用 `operation_ref` 恢复事务前快照，不通过识别器猜逆操作。", "",
              "## 6. Agent-facing 工具协议", "", "```json",
              '{"edits":[{"action":"upsert","shape":"low_cut","frequency_hz":80,"slope_db_per_oct":24},{"action":"upsert","shape":"bell","frequency_hz":3400,"gain_db":-3,"q":0.5}],"atomic":true}',
              "```", "", "LLM 只提交声学目标，不提交插件名、band 编号、参数 ID、normalized value 或寻址原型。工具结果返回 `exact|quantized|rejected`、`control_ref`、`operation_ref`、requested/actual、量化字段、稳定拒绝码、回读和回滚状态。所有显式字段默认是硬要求。", "",
              "## 7. 原子批量事务", "",
              "1. 从同一不可变参数快照识别 topology。", "2. 为全部 edits 选择或预留 section。",
              "3. 检测 section 冲突、槽位不足和字段缺失。", "4. 在写入前完成全部物理域转换与 dependency graph。",
              "5. 以 topology generation/fingerprint 做过期检查。", "6. 按依赖顺序写入，Activation 永远最后。",
              "7. 对所有显式物理字段回读。", "8. 任一失败则逆序恢复整个批次并验证回滚。", "",
              "## 8. Waves 50 型号逐 shape 的 upsert 摘要", "",
              "| 型号 | Bell | Low Shelf | High Shelf | Low Cut | High Cut |", "|---|---|---|---|---|---|" ]
    for model in models:
        lookup = {(row["shape"], row["action"]): row for row in model["shape_action_matrix"]}
        values = [lookup[(shape, "upsert")]["status"] for shape in SHAPES]
        lines.append(f"| {model['plugin_name']} | {' | '.join(values)} |")
    lines += ["", "完整 1,250 行矩阵见 `shape_action_matrix.csv`；规范指令的目标值重放见 `canonical_intent_matrix.csv`。", "",
              "## 9. 各型号详细能力与拒绝", ""]
    for model in models:
        lines += [f"### {model['plugin_name']}", ""]
        for shape in SHAPES:
            rows = [row for row in model["shape_action_matrix"] if row["shape"] == shape]
            rendered = []
            for row in rows:
                suffix = f" ({','.join(row['reasons'])})" if row["reasons"] else ""
                rendered.append(f"{row['action']}={row['status']}{suffix}")
            lines.append(f"- `{shape}`：" + "；".join(rendered))
        lines.append("")
    lines += ["## 10. 稳定拒绝码", ""]
    for code, description in sorted(REJECTION_CODES.items()):
        lines.append(f"- `{code}`：{description}")
    lines += ["", "## 11. 分阶段实施建议", "",
              "1. 先引入只读 `EQControlTopology` 与逐 section shape capability，不改变现有执行。",
              "2. 引入 request-dependent planner，并用 shadow comparison 对照旧三分类。",
              "3. 接入单 edit 原子执行、严格字段与物理回读。",
              "4. 接入 `control_ref` / `operation_ref` 和 modify/disable/undo。",
              "5. 最后接入多 edit 资源预留与全批次回滚，再冻结 Plugin Alliance 外部测试集做验证。", "",
              "## 12. 产物与可复现命令", "",
              f"- 输入 run：`{run_dir}`", f"- 输出目录：`{output_dir}`",
              "- 分析：`python scripts/waves_static_eq_requirement_cluster.py --run-dir <run> --corpus scripts/waves_static_eq_requirement_corpus.json --report-path docs/WAVES_STATIC_EQ_RECOGNIZER_PHASE2.md`",
              "- 验收：`python scripts/waves_static_eq_requirement_acceptance.py --repo-root <repo> --run-dir <run> --corpus scripts/waves_static_eq_requirement_corpus.json --report docs/WAVES_STATIC_EQ_RECOGNIZER_PHASE2.md`", "",
              "## 13. 停止点", "",
              "第二阶段到此结束。本阶段没有修改生产识别器、执行器、工具 API 或生产测试；未经用户再次确认，不开始生产开发。", ""]
    return "\n".join(lines)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--run-dir", required=True)
    parser.add_argument("--corpus", required=True)
    parser.add_argument("--report-path", required=True)
    args = parser.parse_args()

    run_dir = Path(args.run_dir).resolve()
    corpus = read_json(Path(args.corpus).resolve())
    signatures_dir = run_dir / "analysis" / "signatures"
    output_dir = run_dir / "analysis" / "static_eq_phase2"
    models_dir = output_dir / "models"
    models_dir.mkdir(parents=True, exist_ok=True)

    models: list[dict[str, Any]] = []
    input_signatures: list[dict[str, Any]] = []
    for path in sorted(signatures_dir.glob("*.json")):
        source_signature = read_json(path)
        model = analyze_signature(source_signature, corpus)
        models.append(model)
        input_signatures.append({
            "case_id": source_signature.get("case_id"),
            "topology_signature_sha256": source_signature.get("topology_signature_sha256"),
            "source_file_sha256": hashlib.sha256(path.read_bytes()).hexdigest(),
        })
        write_json(models_dir / path.name, model)
    models.sort(key=lambda row: str(row["case_id"]))
    write_matrices(output_dir, models)

    # Cluster every candidate program, not only the model-level winning plan.
    # This preserves anchored alternatives on surfaces that also expose a more
    # capable resident Bell section.
    candidate_plan_rows: list[dict[str, Any]] = []
    for model in models:
        for section in model["sections"]:
            for shape in SHAPES:
                for action in ACTIONS:
                    candidate_plan_rows.append(plan_for_section(section, shape, action))
            for intent in corpus.get("canonical_intents", []):
                candidate_plan_rows.append(plan_for_section(
                    section, str(intent["shape"]), str(intent["action"]),
                    intent.get("fields") if isinstance(intent.get("fields"), dict) else {}))
    partitions = refinement_trace(candidate_plan_rows)
    waves_allocatable_count = sum(
        section.get("addressing") == "allocatable"
        for model in models for section in model["sections"])
    minimality = addressing_minimality(corpus, waves_allocatable_count)
    write_json(output_dir / "exact_plan_partitions.json", partitions)
    write_json(output_dir / "addressing_minimality_proof.json", minimality)
    catalog_counts = Counter(
        row["control_plan_signature_sha256"] for row in candidate_plan_rows
        if row.get("control_plan_signature_sha256"))
    catalog_plans = {}
    for row in candidate_plan_rows:
        digest = row.get("control_plan_signature_sha256")
        if digest and digest not in catalog_plans:
            catalog_plans[digest] = row["control_plan_signature"]
    write_json(output_dir / "control_plan_catalog.json", {
        "schema_version": "static_eq.control_plan_catalog.v1",
        "identity_features_used": False,
        "candidate_program_count": sum(catalog_counts.values()),
        "unique_plan_count": len(catalog_plans),
        "plans": [{
            "control_plan_signature_sha256": digest,
            "occurrence_count": catalog_counts[digest],
            "control_plan_signature": catalog_plans[digest],
        } for digest in sorted(catalog_plans)],
    })
    write_json(output_dir / "rejection_codes.json", {
        "schema_version": "static_eq.rejection_codes.v1", "codes": REJECTION_CODES})
    phase1_acceptance_path = run_dir / "acceptance.json"
    phase1_acceptance = read_json(phase1_acceptance_path)
    write_json(output_dir / "analysis_provenance.json", {
        "schema_version": "static_eq.phase2_provenance.v1",
        "method": "offline_symbolic_replay_of_phase1_signatures",
        "input_signature_count": len(input_signatures),
        "input_signature_manifest_sha256": hash_json(input_signatures),
        "input_signatures": input_signatures,
        "phase1_acceptance_path": str(phase1_acceptance_path),
        "phase1_acceptance_status": phase1_acceptance.get("status"),
        "runtime_calls": {
            "plugin_load": 0, "plugin_parameter_read": 0,
            "plugin_parameter_write": 0, "audio_probe": 0, "b4": 0,
            "plugin_alliance_parameter_read": 0,
        },
    })

    report_path = Path(args.report_path).resolve()
    report = report_markdown(run_dir, output_dir, corpus, models, partitions, minimality)
    report_path.parent.mkdir(parents=True, exist_ok=True)
    report_path.write_text(report, encoding="utf-8")
    (output_dir / "WAVES_STATIC_EQ_RECOGNIZER_PHASE2.md").write_text(report, encoding="utf-8")

    matrix_status = Counter(row["status"] for model in models
                            for row in model["shape_action_matrix"])
    intent_status = Counter(row["status"] for model in models
                            for row in model["canonical_intents"])
    summary = {
        "schema_version": "static_eq.phase2_analysis_summary.v1",
        "model_count": len(models),
        "shape_count": len(SHAPES),
        "action_count": len(ACTIONS),
        "shape_action_row_count": sum(len(model["shape_action_matrix"]) for model in models),
        "shape_action_statuses": dict(matrix_status),
        "canonical_intent_row_count": sum(len(model["canonical_intents"]) for model in models),
        "canonical_intent_statuses": dict(intent_status),
        "exact_plan_partition_count": partitions["partition_count"],
        "addressing_partition_count": minimality["partition_count"],
        "addressing_minimal": minimality["minimal"],
        "identity_features_used": False,
        "plugin_parameter_writes": 0,
        "audio_probes": 0,
        "plugin_alliance_parameter_reads": 0,
        "report_path": str(report_path),
    }
    write_json(output_dir / "analysis_summary.json", summary)
    print(f"models={len(models)} matrix_rows={summary['shape_action_row_count']} "
          f"plan_partitions={partitions['partition_count']}")
    print(f"statuses={dict(matrix_status)} intents={dict(intent_status)}")
    print(f"report={report_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

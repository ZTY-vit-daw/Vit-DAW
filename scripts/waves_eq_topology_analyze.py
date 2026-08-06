#!/usr/bin/env python3
"""Build vendor-neutral EQ topology signatures and clusters from census JSON."""
from __future__ import annotations

import argparse
import csv
import hashlib
import json
import math
import re
from collections import Counter, defaultdict
from difflib import SequenceMatcher
from pathlib import Path
from typing import Any


ROLE_HINTS = {
    "frequency": {"frequency", "freq", "frq", "cutoff", "hz"},
    "gain": {"gain", "level", "boost", "attenuate", "atten", "cut", "decibel", "decibels", "db", "peak", "dip"},
    "q": {"q", "bandwidth", "width"},
    "activation": {"on", "off", "enable", "enabled", "active", "used", "in", "out", "inout"},
    "filter_kind": {"shape", "type", "kind", "filter"},
    "slope": {"slope", "oct", "octave"},
}
DYNAMIC_HINTS = {
    "threshold", "thresh", "attack", "release", "ratio", "range",
    "dynamic", "dynamics", "compress", "compression", "expander", "gate",
}
STATEFUL_HINTS = {
    "capture", "reference", "match", "matching", "learn", "learning",
    "analyze", "analysis", "target", "transfer", "correction", "adaptive",
}
TONAL_BAND_HINTS = {
    "bass", "treble", "presence", "tone", "low", "mid", "high", "air",
    "lf", "lmf", "hmf", "hf",
}
NON_EQ_DB_HINTS = {
    "input", "output", "dry", "wet", "mix", "trim", "makeup", "volume",
    "pan", "balance", "drive", "headroom", "noise", "meter",
}
ROLE_TOKENS = set().union(*ROLE_HINTS.values()) | DYNAMIC_HINTS
CHANNEL_WORDS = {"left": "left", "right": "right", "l": "left", "r": "right"}
BAND_TOKEN_ALIASES = {"lf": "low", "hf": "high", "lmf": "low mid", "hmf": "high mid"}
INACTIVE_LABELS = {"off", "out", "disabled", "disable", "unused", "bypass", "bypassed"}
ACTIVE_LABELS = {"on", "in", "enabled", "enable", "used", "active"}


def read_json(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8-sig"))
    if not isinstance(value, dict):
        raise RuntimeError(f"{path} is not a JSON object")
    return value


def write_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2), encoding="utf-8")


def canonical_json(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, sort_keys=True,
                      separators=(",", ":"))


def signature(value: Any) -> str:
    return hashlib.sha256(canonical_json(value).encode("utf-8")).hexdigest()


def text(row: dict[str, Any], *keys: str) -> str:
    for key in keys:
        value = row.get(key)
        if value is not None and str(value).strip():
            return str(value).strip()
    return ""


def tokenize(name: str) -> list[str]:
    value = re.sub(r"(?i)k\s*hz", " khz ", name)
    value = re.sub(r"(?i)d\s*b", " db ", value)
    value = re.sub(r"(?i)(?<!k)hz", " hz ", value)
    value = re.sub(r"([a-z])([A-Z])", r"\1 \2", value)
    value = re.sub(r"(?i)(\d)(khz|hz)", r"\1 \2", value)
    tokens = re.findall(r"(?i)\d+(?:\.\d+)?|[a-z]+", value.casefold())
    return ["hz" if token == "khz" else token for token in tokens]


def parse_number(value: str) -> float | None:
    match = re.search(r"[-+]?\d+(?:\.\d+)?", value.replace(",", ""))
    return float(match.group()) if match else None


def parse_frequency(value: str) -> float | None:
    match = re.search(r"(?i)(\d+(?:\.\d+)?)\s*(k\s*hz|khz|hz)", value)
    if not match:
        return None
    result = float(match.group(1))
    if "k" in match.group(2).casefold():
        result *= 1000.0
    if 10.0 <= result <= 100000.0:
        return result
    return None


def probe_labels(row: dict[str, Any]) -> list[str]:
    probe = row.get("display_probe")
    if not isinstance(probe, dict):
        return []
    labels = probe.get("discrete_labels")
    if not isinstance(labels, list):
        return []
    return [text(item, "label") for item in labels if isinstance(item, dict)]


def probe_samples(row: dict[str, Any]) -> list[dict[str, Any]]:
    probe = row.get("display_probe")
    if not isinstance(probe, dict) or not isinstance(probe.get("samples"), list):
        return []
    return [item for item in probe["samples"] if isinstance(item, dict)]


def domain_features(row: dict[str, Any]) -> dict[str, Any]:
    domain = row.get("display_domain_candidate")
    domain = domain if isinstance(domain, dict) else {}
    labels = probe_labels(row)
    samples = probe_samples(row)
    unit = text(domain, "unit").casefold()
    scale = text(domain, "scale").casefold()
    if not unit:
        joined = " ".join(labels + [text(sample, "text") for sample in samples])
        if parse_frequency(joined) is not None:
            unit = "hz"
        elif "db" in joined.casefold():
            unit = "db"
    numeric_samples: list[float] = []
    for sample in samples:
        sample_text = text(sample, "text")
        value = parse_frequency(sample_text)
        if value is None:
            value = parse_number(sample_text)
        if value is not None and math.isfinite(value):
            numeric_samples.append(value)
    monotonic = "unknown"
    if len(numeric_samples) >= 3:
        deltas = [right - left for left, right in zip(numeric_samples, numeric_samples[1:])]
        if all(delta > 0 for delta in deltas):
            monotonic = "increasing"
        elif all(delta < 0 for delta in deltas):
            monotonic = "decreasing"
        elif all(delta == 0 for delta in deltas):
            monotonic = "constant"
        else:
            monotonic = "non_monotonic"
    step_count = row.get("num_steps", row.get(
        "numSteps", row.get("derived_step_count")))
    return {
        "unit": unit or "unknown",
        "scale": scale or ("enum" if labels else "unknown"),
        "minimum": domain.get("min"),
        "maximum": domain.get("max"),
        "enum_count": len(labels),
        "enum_labels": labels,
        "step_count": step_count,
        "step_count_source": row.get("step_count_source", "unavailable"),
        "sample_count": len(samples),
        "monotonicity": monotonic,
    }


def explicit_channel(tokens: list[str]) -> str:
    for token in tokens:
        if token in CHANNEL_WORDS:
            return CHANNEL_WORDS[token]
    return "shared"


def named_filter_kind(tokens: list[str]) -> str:
    joined = " ".join(tokens)
    if "low pass" in joined or "lowpass" in joined or "lp" in tokens or "lpf" in tokens:
        return "high_cut"
    if "high pass" in joined or "highpass" in joined or "hp" in tokens or "hpf" in tokens:
        return "low_cut"
    if "low shelf" in joined or "lowshelf" in joined:
        return "low_shelf"
    if "high shelf" in joined or "highshelf" in joined:
        return "high_shelf"
    if "notch" in tokens:
        return "notch"
    if "bell" in tokens or "peak" in tokens:
        return "bell"
    return "unknown"


def infer_role(name: str, row: dict[str, Any], domain: dict[str, Any]) -> tuple[str, list[str]]:
    tokens = tokenize(name)
    token_set = set(tokens)
    evidence: list[str] = []
    compact = re.sub(r"[^A-Za-z0-9]", "", name)
    compact_role = re.fullmatch(r"([A-Z]{2,6})([FG])", compact)
    if compact_role:
        return ("frequency" if compact_role.group(2) == "F" else "gain"), ["compact_suffix_role"]
    if token_set & DYNAMIC_HINTS:
        return "dynamic_peripheral", sorted(token_set & DYNAMIC_HINTS)
    if token_set & STATEFUL_HINTS:
        return "stateful_control", sorted(token_set & STATEFUL_HINTS)
    if "bypass" in token_set:
        return "global_bypass", ["bypass_name"]
    if domain["unit"] == "db" and parse_frequency(name) is not None:
        return "gain", ["fixed_frequency_label_plus_db_domain"]
    for role in ("activation", "slope", "q", "frequency", "filter_kind", "gain"):
        hits = token_set & ROLE_HINTS[role]
        if hits:
            if role == "gain" and token_set & NON_EQ_DB_HINTS:
                return "other", ["non_eq_level_name"]
            if role == "gain" and hits == {"cut"} and (
                    "high" in token_set or "low" in token_set):
                continue
            return role, sorted(hits)
    labels = " ".join(domain["enum_labels"])
    if any(parse_frequency(label) is not None for label in domain["enum_labels"]):
        return "frequency", ["enum_frequency_labels"]
    if domain["unit"] in {"hz", "khz"}:
        return "frequency", ["physical_frequency_domain"]
    if domain["unit"] == "db" and not token_set & NON_EQ_DB_HINTS:
        return "gain_candidate", ["physical_db_domain"]
    lo, hi = domain.get("minimum"), domain.get("maximum")
    if (token_set & TONAL_BAND_HINTS and isinstance(lo, (int, float))
            and isinstance(hi, (int, float)) and lo < 0 < hi):
        return "gain_candidate", ["tonal_name_plus_bipolar_numeric_domain"]
    label_set = {label.casefold().strip() for label in domain["enum_labels"]}
    if label_set & (INACTIVE_LABELS | ACTIVE_LABELS):
        return "toggle_unknown", ["activation_like_enum_without_role_name"]
    if any(value in labels.casefold() for value in ("bell", "shelf", "pass", "notch")):
        return "filter_kind", ["filter_kind_enum_labels"]
    return "other", evidence


def normalized_template(name: str) -> str:
    value = name.casefold()
    value = re.sub(r"(?i)\d+(?:\.\d+)?\s*(?:k\s*hz|khz|hz)", " <freq> ", value)
    value = re.sub(r"\d+(?:\.\d+)?", " <n> ", value)
    value = re.sub(r"\b(left|right|l|r)\b", " <ch> ", value)
    value = re.sub(r"[^a-z<>]+", " ", value)
    return " ".join(value.split())


def band_key(name: str, role: str) -> str:
    tokens = tokenize(name)
    compact = re.sub(r"[^A-Za-z0-9]", "", name)
    compact_role = re.fullmatch(r"([A-Z]{2,6})([FG])", compact)
    if compact_role:
        stem = compact_role.group(1).casefold()
        return BAND_TOKEN_ALIASES.get(stem, stem)
    channel = explicit_channel(tokens)
    paired_channel_mode = channel != "shared" and ("l" in tokens or "r" in tokens)
    kept: list[str] = []
    for token in tokens:
        if token in CHANNEL_WORDS or token in ROLE_TOKENS:
            continue
        if paired_channel_mode and token in {"m", "s"}:
            continue
        if role in {"gain_candidate", "activation_candidate"} and token in NON_EQ_DB_HINTS:
            continue
        kept.extend(BAND_TOKEN_ALIASES.get(token, token).split())
    return " ".join(kept) or "global"


def parameter_observation(index: int, row: dict[str, Any]) -> dict[str, Any]:
    name = text(row, "name", "raw_param_name", "alias")
    domain = domain_features(row)
    role, evidence = infer_role(name, row, domain)
    fixed_hz = parse_frequency(name) if role in {"gain", "gain_candidate"} else None
    if role == "gain_candidate" and fixed_hz is not None:
        role = "gain"
        evidence.append("frequency_label_plus_db_domain")
    return {
        "ordinal": index,
        "name": name,
        "tokens": tokenize(name),
        "template": normalized_template(name),
        "role": role,
        "role_evidence": evidence,
        "channel": explicit_channel(tokenize(name)),
        "band_key": band_key(name, role),
        "fixed_frequency_hz": fixed_hz,
        "named_filter_kind": named_filter_kind(tokenize(name)),
        "tonal_band_hint": bool(set(tokenize(name)) & TONAL_BAND_HINTS),
        "current_text": text(row, "value_text"),
        "domain": domain,
    }


def resolve_candidates(observations: list[dict[str, Any]]) -> None:
    by_band: dict[str, list[dict[str, Any]]] = defaultdict(list)
    for item in observations:
        by_band[item["band_key"]].append(item)
    for items in by_band.values():
        has_frequency = any(item["role"] == "frequency" for item in items)
        eq_roles = {item["role"] for item in items}
        for item in items:
            if item["role"] == "gain_candidate" and has_frequency:
                item["role"] = "gain"
                item["role_evidence"].append("local_frequency_pair")
            if item["role"] == "activation_candidate" and eq_roles & {
                    "frequency", "gain", "gain_candidate", "q", "filter_kind"}:
                item["role"] = "activation"
                item["role_evidence"].append("local_eq_block")
            if item["role"] == "gain_candidate" and item["tonal_band_hint"]:
                item["role"] = "gain"
                item["role_evidence"].append("tonal_name_plus_db_domain")


def activation_summary(items: list[dict[str, Any]]) -> dict[str, Any]:
    bindings = [item for item in items if item["role"] == "activation"]
    if not bindings:
        return {"strategy": "always_active_or_unexposed", "known": False,
                "current_state": "unknown", "binding_count": 0}
    labels = [item["current_text"].casefold().strip() for item in bindings]
    if any("unused" in label for label in labels):
        strategy = "slot_lifecycle"
    else:
        strategy = "explicit_binding"
    if labels and all(label in INACTIVE_LABELS for label in labels):
        state = "inactive"
    elif labels and all(label in ACTIVE_LABELS for label in labels):
        state = "active"
    else:
        state = "mixed_or_unknown"
    return {"strategy": strategy, "known": True, "current_state": state,
            "binding_count": len(bindings)}


def build_blocks(observations: list[dict[str, Any]]) -> list[dict[str, Any]]:
    by_band: dict[str, list[dict[str, Any]]] = defaultdict(list)
    for item in observations:
        if item["role"] in {
                "frequency", "gain", "q", "activation", "filter_kind", "slope",
                "dynamic_peripheral"}:
            by_band[item["band_key"]].append(item)
    blocks: list[dict[str, Any]] = []
    for key, items in by_band.items():
        roles = Counter(item["role"] for item in items)
        fixed_values = sorted({item["fixed_frequency_hz"] for item in items
                               if item["fixed_frequency_hz"] is not None})
        tonal_gain = any(item["role"] == "gain" and item["tonal_band_hint"] for item in items)
        has_anchor = roles["frequency"] > 0 or bool(fixed_values)
        has_gain = roles["gain"] > 0
        is_filter = roles["frequency"] > 0 and (
            roles["slope"] > 0 or any(item["named_filter_kind"] != "unknown" for item in items))
        if not ((has_anchor and has_gain) or is_filter or (tonal_gain and has_gain)):
            continue
        channels_by_role: dict[str, list[str]] = {}
        channel_inference: dict[str, str] = {}
        for role in ("frequency", "gain", "q", "activation", "filter_kind", "slope"):
            values = {item["channel"] for item in items if item["role"] == role}
            if values == {"shared", "right"}:
                channels_by_role[role] = ["left", "right"]
                channel_inference[role] = "unsuffixed_plus_right_mirror"
            else:
                channels_by_role[role] = sorted(values)
        explicit_sets = [set(values) for values in channels_by_role.values()
                         if values and values != ["shared"]]
        channel_asymmetry = any(values != {"left", "right"} for values in explicit_sets)
        gain_names = [item["tokens"] for item in items if item["role"] == "gain"]
        coupled = (
            any("boost" in tokens for tokens in gain_names)
            and any("attenuate" in tokens or "atten" in tokens for tokens in gain_names)
        )
        enum_roles = sorted({item["role"] for item in items
                             if item["domain"]["enum_count"] > 0})
        unresolved_enum_roles = sorted({item["role"] for item in items
            if item["role"] in {"frequency", "gain", "q"}
            and item["domain"]["scale"] == "enum"
            and item["domain"]["enum_count"] == 0})
        activation = activation_summary(items)
        gain_items = [item for item in items if item["role"] == "gain"]
        gain_polarities: set[str] = set()
        for item in gain_items:
            token_set = set(item["tokens"])
            labels = item["domain"]["enum_labels"]
            values = [parse_number(label) for label in labels]
            values = [value for value in values if value is not None]
            lo = item["domain"]["minimum"]
            hi = item["domain"]["maximum"]
            if "boost" in token_set or "peak" in token_set:
                gain_polarities.add("boost_only")
            elif "attenuate" in token_set or "atten" in token_set or "dip" in token_set:
                gain_polarities.add("attenuate_only")
            elif values and min(values) < 0 < max(values):
                gain_polarities.add("bipolar")
            elif isinstance(lo, (int, float)) and isinstance(hi, (int, float)) and lo < 0 < hi:
                gain_polarities.add("bipolar")
            else:
                gain_polarities.add("unknown_or_one_sided")
        if {"boost_only", "attenuate_only"}.issubset(gain_polarities):
            gain_law = "split_boost_attenuate"
            coupled = True
        elif gain_polarities == {"bipolar"}:
            gain_law = "bipolar"
        elif gain_polarities:
            gain_law = sorted(gain_polarities)[0]
        else:
            gain_law = "absent"
        complete = has_gain and has_anchor and not channel_asymmetry and not unresolved_enum_roles
        issues: list[str] = []
        if not has_gain:
            issues.append("gain role absent")
        if not has_anchor:
            issues.append("frequency anchor absent")
        if channel_asymmetry:
            issues.append("channel asymmetry")
        if unresolved_enum_roles:
            issues.append("enum physical values unavailable")
        if coupled:
            issues.append("coupled boost/attenuate topology")
        blocks.append({
            "key": key,
            "ordinal_start": min(item["ordinal"] for item in items),
            "ordinal_end": max(item["ordinal"] for item in items),
            "role_counts": dict(sorted(roles.items())),
            "channels_by_role": channels_by_role,
            "channel_inference": channel_inference,
            "fixed_frequency_hz": fixed_values[0] if len(fixed_values) == 1 else None,
            "frequency_mode": "writable_discrete" if roles["frequency"] and "frequency" in enum_roles
                else "writable_continuous" if roles["frequency"]
                else "fixed_label" if fixed_values else "opaque",
            "activation": activation,
            "filter_kinds": sorted({item["named_filter_kind"] for item in items
                                    if item["named_filter_kind"] != "unknown"}),
            "dynamic_peripheral_count": roles["dynamic_peripheral"],
            "enum_roles": enum_roles,
            "coupled_gain": coupled,
            "gain_law": gain_law,
            "complete": complete and not coupled,
            "issues": issues,
        })
    return sorted(blocks, key=lambda item: (item["ordinal_start"], item["key"]))


def global_controls(observations: list[dict[str, Any]]) -> dict[str, Any]:
    activation = [item for item in observations
                  if item["role"] == "activation" and item["band_key"] == "global"]
    bypass = [item for item in observations if item["role"] == "global_bypass"]
    unknown_toggles = [item for item in observations if item["role"] == "toggle_unknown"]
    return {
        "activation": [{"name": item["name"], "current_text": item["current_text"],
                        "labels": item["domain"]["enum_labels"]} for item in activation],
        "bypass": [{"name": item["name"], "current_text": item["current_text"],
                    "labels": item["domain"]["enum_labels"]} for item in bypass],
        "unknown_toggles": [{"name": item["name"], "current_text": item["current_text"],
                             "labels": item["domain"]["enum_labels"]}
                            for item in unknown_toggles],
    }


def repeated_templates(observations: list[dict[str, Any]]) -> list[dict[str, Any]]:
    by_template: dict[str, list[dict[str, Any]]] = defaultdict(list)
    for item in observations:
        by_template[item["template"]].append(item)
    rows = []
    for template, items in by_template.items():
        if len(items) < 2:
            continue
        rows.append({
            "template": template,
            "count": len(items),
            "roles": sorted({item["role"] for item in items}),
            "channels": sorted({item["channel"] for item in items}),
            "ordinals": [item["ordinal"] for item in items],
        })
    return sorted(rows, key=lambda item: (-item["count"], item["template"]))


def repeated_bank_axis(observations: list[dict[str, Any]]) -> dict[str, Any]:
    sequence = [item["template"] for item in observations]
    positions: dict[str, list[int]] = defaultdict(list)
    for index, template in enumerate(sequence):
        positions[template].append(index)
    candidates: list[tuple[int, int, str, list[int]]] = []
    for template, indexes in positions.items():
        if len(indexes) < 3:
            continue
        differences = [right - left for left, right in zip(indexes, indexes[1:])]
        period, count = Counter(differences).most_common(1)[0]
        if period >= 8 and count >= 2:
            candidates.append((count, period, template, indexes))
    if not candidates:
        return {"present": False, "repetition_count": 0}
    _, period, anchor, indexes = max(candidates)
    chains: list[list[int]] = []
    current = [indexes[0]]
    for index in indexes[1:]:
        if index - current[-1] == period:
            current.append(index)
        else:
            if len(current) >= 3:
                chains.append(current)
            current = [index]
    if len(current) >= 3:
        chains.append(current)
    if not chains:
        return {"present": False, "repetition_count": 0}
    chain = max(chains, key=len)
    start = chain[0]
    similarities = []
    for left in chain[:-1]:
        right = left + period
        similarities.append(SequenceMatcher(
            None, sequence[left:left + period], sequence[right:right + period]).ratio())
    confidence = sum(similarities) / len(similarities) if similarities else 0.0
    if confidence < 0.80:
        return {"present": False, "repetition_count": 0}
    address_observed = "<n>" in anchor or "<freq>" in anchor
    return {
        "present": True,
        "period": period,
        "repetition_count": len(chain),
        "ordinal_start": start,
        "ordinal_end": chain[-1] + period - 1,
        "anchor_template": anchor,
        "mean_sequence_similarity": round(confidence, 6),
        "address_key_observed": address_observed,
        "interpretation": "addressed repeated band/section axis" if address_observed
            else "repeated local control banks with identical public parameter names",
    }


def execution_assessment(blocks: list[dict[str, Any]], observations: list[dict[str, Any]],
                         surface_kind: str, bank_axis: dict[str, Any]) -> dict[str, Any]:
    stateful = [item for item in observations if item["role"] == "stateful_control"]
    complete = [block for block in blocks if block["complete"]]
    coupled = [block for block in blocks if block["coupled_gain"]]
    asymmetrical = [block for block in blocks if "channel asymmetry" in block["issues"]]
    reasons: list[str] = []
    status = "full"
    if stateful:
        status = "unsupported"
        reasons.append("adaptive/stateful/capture topology")
    if coupled:
        status = "unsupported"
        reasons.append("coupled boost/attenuate topology")
    if asymmetrical:
        status = "unsupported"
        reasons.append("channel asymmetry")
    if bank_axis.get("present") and not bank_axis.get("address_key_observed"):
        status = "unsupported"
        reasons.append("repeated bank axis has no stable address key")
    if not complete:
        status = "unsupported"
        reasons.append("no complete local EQ set-point block")
    if blocks and not complete and any("frequency anchor absent" in block["issues"] for block in blocks):
        reasons.append("fixed frequency unknown or frequency role incomplete")
    if status == "full" and any(
            set(block["enum_roles"]) & {"frequency", "gain", "q", "filter_kind", "slope"}
            for block in complete):
        status = "partial_quantized"
        reasons.append("one or more requested roles are quantized")
    restricted_gain = [block for block in complete
                       if block.get("gain_law") not in {"bipolar", "absent"}]
    if restricted_gain:
        status = "unsupported"
        reasons.append("one-sided or unknown gain law cannot satisfy arbitrary set-point gain")
    incomplete_gain_blocks = [block for block in blocks
                              if not block["complete"] and block["role_counts"].get("gain", 0) > 0]
    if incomplete_gain_blocks:
        status = "unsupported"
        reasons.append("incomplete gain-bearing EQ blocks remain")
    activation_unknown = [block for block in complete
                          if block["activation"]["current_state"] == "inactive"
                          and not block["activation"]["known"]]
    if activation_unknown:
        status = "unsupported"
        reasons.append("inactive block without activation binding")
    capabilities = {
        role: any(block["role_counts"].get(role, 0) > 0 for block in blocks)
        for role in ("frequency", "gain", "q", "filter_kind", "slope", "activation")
    }
    return {
        "status": status,
        "set_eq_point_supported": status in {"full", "partial_quantized"},
        "evidence_level": "structurally_inferred_not_write_tested",
        "reasons": sorted(set(reasons)),
        "capabilities": capabilities,
        "complete_block_count": len(complete),
        "local_eq_isolation_required": surface_kind == "channel_strip",
        "forbidden_role": "dynamic_peripheral",
    }


def project_public_classification(blocks: list[dict[str, Any]]) -> str | None:
    if not blocks:
        return None
    if any(block["activation"]["strategy"] == "slot_lifecycle" for block in blocks):
        return "free_floating"
    writable = sum(block["frequency_mode"].startswith("writable") for block in blocks)
    fixed = sum(block["frequency_mode"] in {"fixed_label", "opaque"}
                and block["role_counts"].get("gain", 0) > 0 for block in blocks)
    if writable >= fixed and writable > 0:
        return "fixed_slot_adjustable"
    if fixed > 0:
        return "fixed_freq"
    return None


def analyze_capture(capture: dict[str, Any]) -> dict[str, Any]:
    case = capture.get("case") if isinstance(capture.get("case"), dict) else {}
    parameters = [row for row in capture.get("parameters", []) if isinstance(row, dict)]
    observations = [parameter_observation(index, row)
                    for index, row in enumerate(parameters)]
    resolve_candidates(observations)
    blocks = build_blocks(observations)
    repeats = repeated_templates(observations)
    bank_axis = repeated_bank_axis(observations)
    globals_summary = global_controls(observations)
    role_counts = Counter(item["role"] for item in observations)
    channel_counts = Counter(item["channel"] for item in observations)
    domain_counts = Counter(
        f"{item['domain']['scale']}:{item['domain']['unit']}" for item in observations)
    public_classification = project_public_classification(blocks)
    execution = execution_assessment(
        blocks, observations, text(case, "surface_kind"), bank_axis)
    canonical_topology = {
        "parameter_count": len(parameters),
        "role_sequence": [item["role"] for item in observations],
        "channel_sequence": [item["channel"] for item in observations],
        "domain_sequence": [{
            "scale": item["domain"]["scale"],
            "unit": item["domain"]["unit"],
            "enum_count": item["domain"]["enum_count"],
            "monotonicity": item["domain"]["monotonicity"],
        } for item in observations],
        "normalized_name_templates": [item["template"] for item in observations],
        "blocks": [{
            "relative_order": index,
            "role_counts": block["role_counts"],
            "channels_by_role": block["channels_by_role"],
            "frequency_mode": block["frequency_mode"],
            "activation": block["activation"],
            "filter_kinds": block["filter_kinds"],
            "dynamic_peripheral_count": block["dynamic_peripheral_count"],
            "enum_roles": block["enum_roles"],
            "coupled_gain": block["coupled_gain"],
            "gain_law": block["gain_law"],
            "complete": block["complete"],
        } for index, block in enumerate(blocks)],
        "repeated_templates": [{
            "template": item["template"], "count": item["count"],
            "roles": item["roles"], "channels": item["channels"],
        } for item in repeats],
        "global_controls": globals_summary,
        "bank_axis": bank_axis,
        "public_projection": public_classification,
    }
    cluster_features = {
        "public_classification": public_classification or "unresolved",
        "surface_kind": text(case, "surface_kind"),
        "parameter_count": len(parameters),
        "block_count": len(blocks),
        "complete_block_count": execution["complete_block_count"],
        "frequency_modes": dict(Counter(block["frequency_mode"] for block in blocks)),
        "activation_strategies": dict(Counter(
            block["activation"]["strategy"] for block in blocks)),
        "role_counts": dict(role_counts),
        "channel_counts": dict(channel_counts),
        "domain_counts": dict(domain_counts),
        "coupled_block_count": sum(block["coupled_gain"] for block in blocks),
        "dynamic_parameter_count": role_counts["dynamic_peripheral"],
        "stateful_parameter_count": role_counts["stateful_control"],
        "bank_repetition_count": bank_axis.get("repetition_count", 0),
        "repeated_template_shapes": sorted(
            f"{item['template']}:{item['count']}" for item in repeats),
        "role_sequence": canonical_topology["role_sequence"],
    }
    return {
        "schema_version": "eq_control_topology.experimental.v1",
        "case_id": text(case, "id"),
        "plugin_name": text(case, "plugin_name"),
        "surface_kind": text(case, "surface_kind"),
        "plugin_identifier": capture.get("plugin_identifier"),
        "parameter_count": len(parameters),
        "pagination": capture.get("pagination"),
        "surface_signature_sha256": capture.get("surface_signature_sha256"),
        "topology_signature_sha256": signature(canonical_topology),
        "public_classification": public_classification,
        "observations": observations,
        "local_blocks": blocks,
        "global_controls": globals_summary,
        "bank_axis": bank_axis,
        "repeated_structures": repeats,
        "execution": execution,
        "cluster_features": cluster_features,
        "canonical_topology": canonical_topology,
        "inference_policy": {
            "plugin_identity_used_as_feature": False,
            "manufacturer_used_as_feature": False,
            "parameter_names_used": True,
            "parameter_groups_used": True,
            "parameter_order_used": True,
            "domains_and_enums_used": True,
            "audio_behavior_used": False,
            "production_classifier_used": False,
        },
    }


def counter_distance(left: dict[str, int], right: dict[str, int]) -> float:
    keys = set(left) | set(right)
    if not keys:
        return 0.0
    numerator = sum(abs(left.get(key, 0) - right.get(key, 0)) for key in keys)
    denominator = sum(max(left.get(key, 0), right.get(key, 0)) for key in keys)
    return numerator / denominator if denominator else 0.0


def set_distance(left: list[str], right: list[str]) -> float:
    a, b = set(left), set(right)
    if not a and not b:
        return 0.0
    return 1.0 - len(a & b) / len(a | b)


def topology_distance(left: dict[str, Any], right: dict[str, Any]) -> float:
    a, b = left["cluster_features"], right["cluster_features"]
    categorical = 0.0 if a["public_classification"] == b["public_classification"] else 1.0
    block_scale = max(1, a["block_count"], b["block_count"])
    block_distance = abs(a["block_count"] - b["block_count"]) / block_scale
    parameter_scale = max(1, a["parameter_count"], b["parameter_count"])
    parameter_distance = abs(a["parameter_count"] - b["parameter_count"]) / parameter_scale
    sequence_distance = 1.0 - SequenceMatcher(
        None, a["role_sequence"], b["role_sequence"]).ratio()
    return (
        0.15 * categorical
        + 0.10 * block_distance
        + 0.05 * parameter_distance
        + 0.18 * counter_distance(a["frequency_modes"], b["frequency_modes"])
        + 0.12 * counter_distance(a["activation_strategies"], b["activation_strategies"])
        + 0.15 * counter_distance(a["role_counts"], b["role_counts"])
        + 0.08 * counter_distance(a["channel_counts"], b["channel_counts"])
        + 0.07 * counter_distance(a["domain_counts"], b["domain_counts"])
        + 0.05 * set_distance(a["repeated_template_shapes"], b["repeated_template_shapes"])
        + 0.05 * sequence_distance
    )


def hierarchical_clusters(items: list[dict[str, Any]], threshold: float) -> list[list[int]]:
    clusters = [[index] for index in range(len(items))]
    distances = {(i, j): topology_distance(items[i], items[j])
                 for i in range(len(items)) for j in range(i + 1, len(items))}

    def average_distance(left: list[int], right: list[int]) -> float:
        values = [distances[min(i, j), max(i, j)] for i in left for j in right]
        return sum(values) / len(values)

    while True:
        best: tuple[float, int, int] | None = None
        for i in range(len(clusters)):
            for j in range(i + 1, len(clusters)):
                candidate = (average_distance(clusters[i], clusters[j]), i, j)
                if best is None or candidate < best:
                    best = candidate
        if best is None or best[0] > threshold:
            break
        _, i, j = best
        merged = sorted(clusters[i] + clusters[j])
        clusters = [cluster for index, cluster in enumerate(clusters)
                    if index not in {i, j}] + [merged]
        clusters.sort(key=lambda cluster: min(cluster))
    return clusters


def cluster_explanation(members: list[dict[str, Any]]) -> dict[str, Any]:
    classifications = Counter(item["public_classification"] or "unresolved" for item in members)
    modes = Counter(mode for item in members for mode in
                    item["cluster_features"]["frequency_modes"])
    activations = Counter(strategy for item in members for strategy in
                          item["cluster_features"]["activation_strategies"])
    return {
        "member_count": len(members),
        "dominant_public_classification": classifications.most_common(1)[0][0],
        "frequency_modes": sorted(modes),
        "activation_strategies": sorted(activations),
        "parameter_count_range": [min(item["parameter_count"] for item in members),
                                  max(item["parameter_count"] for item in members)],
        "block_count_range": [min(len(item["local_blocks"]) for item in members),
                              max(len(item["local_blocks"]) for item in members)],
        "explanation": "Members share the listed role/domain/channel topology features; plugin identity was not part of the distance function.",
    }


def report_markdown(run_dir: Path, analyzed: list[dict[str, Any]],
                    failed: list[dict[str, Any]], clusters: list[dict[str, Any]],
                    audit: dict[str, Any]) -> str:
    classification_counts = Counter(
        item["public_classification"] or "unresolved" for item in analyzed)
    execution_counts = Counter(item["execution"]["status"] for item in analyzed)
    lines = [
        "# Waves EQ 控制拓扑普查与分类重构设计（第一阶段）",
        "",
        "> 本报告来自真实 Godot → Kernel → Agent 只读参数链。没有插件参数写入、音频主动探测、已退役映射链路、B4 或 Plugin Alliance 参数读取。",
        "",
        "## 1. 普查结论摘要",
        "",
        f"- 目标型号：{len(analyzed) + len(failed)}",
        f"- 完整采集：{len(analyzed)}",
        f"- 明确失败：{len(failed)}",
        f"- 数据驱动簇：{len(clusters)}",
        f"- 公开投影：fixed_slot_adjustable={classification_counts['fixed_slot_adjustable']}，"
        f"fixed_freq={classification_counts['fixed_freq']}，free_floating={classification_counts['free_floating']}，"
        f"unresolved={classification_counts['unresolved']}",
        f"- 结构执行结论：full={execution_counts['full']}，partial_quantized={execution_counts['partial_quantized']}，"
        f"unsupported={execution_counts['unsupported']}",
        f"- 原始产物：`{run_dir}`",
        "- 所有执行能力结论均为参数结构推断，未经写入烟测或音频行为验证。",
        "",
        "## 2. 方法与边界",
        "",
        "每个 Stereo 型号按 128 条一页循环读取 `plugin.get_parameters`，校验 total 稳定、offset 连续、参数 ID 唯一以及实际条数等于 total。`surface_signature` 保留精确参数表；`topology_signature` 删除插件身份后使用名称 token、分组、顺序、物理域、枚举、激活、通道和局部重复块。聚类使用平均链接层次聚类，不预设 Waves 产品族。",
        "",
        "五点域采样只调用宿主 value-to-string 映射，不改变参数值，也不输入音频。Channel Strip 只从外围控制中提取局部 EQ 块。",
        "",
        "## 3. 型号归属、能力与拒绝矩阵",
        "",
        "| 型号 | 参数/页 | 公开分类 | 簇 | 完整块 | 能力 | 拒绝或限制 |",
        "|---|---:|---|---|---:|---|---|",
    ]
    cluster_by_case = {case_id: cluster["cluster_id"] for cluster in clusters
                       for case_id in cluster["member_case_ids"]}
    for item in sorted(analyzed, key=lambda value: value["plugin_name"].casefold()):
        pagination = item.get("pagination") or {}
        execution = item["execution"]
        reason = "; ".join(execution["reasons"]) or "无结构性拒绝；仍需后续写入验证"
        lines.append(
            f"| {item['plugin_name']} | {item['parameter_count']}/{pagination.get('page_count', '?')} | "
            f"{item['public_classification'] or 'unresolved'} | {cluster_by_case.get(item['case_id'], '?')} | "
            f"{execution['complete_block_count']} | {execution['status']} | {reason} |")
    for item in sorted(failed, key=lambda value: value.get("plugin_name", "").casefold()):
        lines.append(
            f"| {item.get('plugin_name', item.get('id', '?'))} | — | unresolved | — | 0 | unsupported | "
            f"采集失败：{item.get('error', 'unknown')} |")

    lines += ["", "## 4. 数据驱动聚类", ""]
    for cluster in clusters:
        explanation = cluster["explanation"]
        lines += [
            f"### {cluster['cluster_id']}", "",
            f"- 成员：{', '.join(cluster['member_plugin_names'])}",
            f"- 主投影：{explanation['dominant_public_classification']}",
            f"- 参数数范围：{explanation['parameter_count_range']}",
            f"- EQ 块数范围：{explanation['block_count_range']}",
            f"- 频率模式：{', '.join(explanation['frequency_modes']) or 'none'}",
            f"- 激活模式：{', '.join(explanation['activation_strategies']) or 'none'}",
            "- 解释：成员共享上述 role/domain/channel 结构；距离函数未使用插件名、manufacturer 或 identifier。",
            "",
        ]

    lines += [
        "## 5. 各型号拓扑证据详表",
        "",
    ]
    for item in sorted(analyzed, key=lambda value: value["plugin_name"].casefold()):
        execution = item["execution"]
        pagination = item.get("pagination") or {}
        cluster_id = cluster_by_case.get(item["case_id"], "?")
        lines += [
            f"### {item['plugin_name']}",
            "",
            f"- identifier：`{item.get('plugin_identifier', '')}`",
            f"- 参数/分页：{item['parameter_count']} 条，{pagination.get('page_count', '?')} 页，complete={pagination.get('complete')}",
            f"- surface signature：`{item.get('surface_signature_sha256', '')}`",
            f"- topology signature：`{item.get('topology_signature_sha256', '')}`",
            f"- 公开投影/簇：`{item.get('public_classification') or 'unresolved'}` / `{cluster_id}`",
            f"- 执行能力：`{execution['status']}`；证据级别：`{execution['evidence_level']}`",
            f"- capabilities：`{json.dumps(execution['capabilities'], ensure_ascii=False, sort_keys=True)}`",
            f"- 拒绝或限制：{'; '.join(execution['reasons']) or '无结构性拒绝；仍未进行写入验证'}",
        ]
        bank_axis = item.get("bank_axis") or {}
        if bank_axis.get("present"):
            lines.append(
                f"- 重复轴：period={bank_axis.get('period')}，repetitions={bank_axis.get('repetition_count')}，"
                f"addressed={bank_axis.get('address_key_observed')}，{bank_axis.get('interpretation')}")
        globals_summary = item.get("global_controls") or {}
        lines.append(
            f"- 全局控制：activation={len(globals_summary.get('activation', []))}，"
            f"bypass={len(globals_summary.get('bypass', []))}，"
            f"unknown_toggles={len(globals_summary.get('unknown_toggles', []))}")
        if item["local_blocks"]:
            lines.append("- 局部 section/band：")
            for block in item["local_blocks"]:
                role_text = ", ".join(
                    f"{role}×{count}" for role, count in block["role_counts"].items())
                channel_text = ", ".join(
                    f"{role}={'+'.join(channels)}" for role, channels in
                    block["channels_by_role"].items() if channels)
                issue_text = "; ".join(block["issues"]) or "none"
                lines.append(
                    f"  - `{block['key']}`：placement={block['frequency_mode']}；"
                    f"roles=[{role_text}]；channels=[{channel_text or 'shared/unexposed'}]；"
                    f"activation={block['activation']['strategy']}/{block['activation']['current_state']}；"
                    f"gain_law={block.get('gain_law', 'unknown')}；complete={block['complete']}；issues={issue_text}")
        else:
            lines.append("- 局部 section/band：未从静态参数面恢复；归入待解决特殊结构并拒绝执行。")
        lines.append("")

    lines += [
        "## 6. 内部 EQControlTopology 方案",
        "",
        "三个公开分类名只保留为兼容投影。建议内部模型为：",
        "",
        "```text",
        "EQControlTopology",
        "  evidence: ordered parameter observations + confidence",
        "  sections: local repeated EQ blocks",
        "  axes:",
        "    placement: writable_continuous | writable_discrete | fixed_label | opaque",
        "    role: frequency | gain | q | filter_kind | slope | activation",
        "    channel: shared | left/right mirrored | linked | asymmetric",
        "    activation: always_active | explicit_binding | slot_lifecycle | unknown",
        "    gain_law: bipolar | quantized | split_boost_attenuate | one_sided",
        "    interaction: independent | coupled | dynamic_peripheral | stateful",
        "  execution_policy:",
        "    supported_roles + quantization + ordering + fail_closed_reasons",
        "```",
        "",
        "投影规则：有 slot lifecycle 的可创建槽投影为 `free_floating`；有可写频率的固定 section 投影为 `fixed_slot_adjustable`；只有可恢复中心频率和 Gain 的图形/固定频段投影为 `fixed_freq`。无法恢复结构时不强行投影，返回 unresolved 并拒绝执行。",
        "",
        "## 7. 安全执行原则（后续阶段设计，不在本阶段实现）",
        "",
        "- 只允许完整局部块进入执行。",
        "- 动态 EQ 的 Range/Threshold/Attack/Release 永远不属于静态 set-point 写集。",
        "- inactive slot 必须先写 Shape/Frequency/Q/Gain，最后写 Activation。",
        "- L/R 镜像必须拥有完整、对称的角色绑定。",
        "- 离散频率、增益和 Q 返回 requested/actual/quantized。",
        "- split boost/attenuate、状态型捕获、自适应匹配、未知固定频率一律 fail-closed。",
        "- topology signature 和聚类只能提供结构证据，不能替代未来的写入回读验证或音频行为证据。",
        "",
        "## 8. 待解决结构",
        "",
        "- 参数名无法表达的共享模式、内部联动和滤波器行为仍需后续受控实验。",
        "- `always_active_or_unexposed` 只能说明没有观察到 activation 参数，不能证明 DSP 永远启用。",
        "- 枚举标签若不能恢复物理值，应继续拒绝，而不是按 normalized 值猜测。",
        "- 状态型、自适应和校准型 EQ 不能仅凭静态参数表获得通用 set-point 能力。",
        "- 当前生产 `buildMarvelGEQSummary` 仍按 `Marvel GEQ` 名称进入专用频率表，违反新的无插件名称生产分支方向；本阶段仅登记，不修改。",
        "",
        "## 9. 禁用路径与现场保护证明",
        "",
        f"- Agent 工具调用：`{json.dumps(audit.get('agent_tools', {}), ensure_ascii=False, sort_keys=True)}`",
        f"- Kernel 命令：`{json.dumps(audit.get('kernel_commands', {}), ensure_ascii=False, sort_keys=True)}`",
        f"- 插件参数写入：{audit.get('plugin_parameter_write_count', 'unknown')}",
        f"- 音频主动探测：{audit.get('audio_probe_count', 'unknown')}",
        f"- B4：{audit.get('b4_call_count', 'unknown')}；已退役映射链路不在运行时能力面中。",
        f"- Plugin Alliance 参数读取：{audit.get('plugin_alliance_parameter_read_count', 'unknown')}",
        "",
        "生产文件哈希、受保护运行文件哈希和 Git 状态前后对比见同一产物目录下的 `run_manifest.json` 与最终 `acceptance.json`。",
        "",
        "## 10. 失败清单",
        "",
    ]
    if failed:
        for item in failed:
            lines.append(f"- {item.get('plugin_name', item.get('id'))}: {item.get('error')}")
    else:
        lines.append("- 无。50 个目标均完成参数分页采集。")
    lines += [
        "",
        "## 11. 停止点",
        "",
        "第一阶段到此结束。本报告没有修改生产识别器、执行器或公开分类 API；下一阶段必须经用户确认后才能开始。",
        "",
    ]
    return "\n".join(lines)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--run-dir", required=True)
    parser.add_argument("--report-path", required=True)
    parser.add_argument("--cluster-threshold", type=float, default=0.27)
    args = parser.parse_args()

    run_dir = Path(args.run_dir).resolve()
    raw_dir = run_dir / "raw"
    summary = read_json(raw_dir / "census_summary.json")
    captures_dir = raw_dir / "captures"
    output_dir = run_dir / "analysis"
    signatures_dir = output_dir / "signatures"
    signatures_dir.mkdir(parents=True, exist_ok=True)

    analyzed: list[dict[str, Any]] = []
    failed = [row for row in summary.get("results", [])
              if isinstance(row, dict) and row.get("status") != "captured"]
    for row in summary.get("results", []):
        if not isinstance(row, dict) or row.get("status") != "captured":
            continue
        case_id = text(row, "id")
        capture = read_json(captures_dir / f"{case_id}.json")
        topology = analyze_capture(capture)
        analyzed.append(topology)
        write_json(signatures_dir / f"{case_id}.json", topology)

    analyzed.sort(key=lambda item: item["case_id"])
    index_clusters = hierarchical_clusters(analyzed, args.cluster_threshold)
    clusters: list[dict[str, Any]] = []
    for number, indices in enumerate(index_clusters, start=1):
        members = [analyzed[index] for index in indices]
        clusters.append({
            "cluster_id": f"C{number:02d}",
            "member_case_ids": [item["case_id"] for item in members],
            "member_plugin_names": [item["plugin_name"] for item in members],
            "exact_topology_signature_groups": dict(Counter(
                item["topology_signature_sha256"] for item in members)),
            "explanation": cluster_explanation(members),
        })
    cluster_payload = {
        "schema_version": "eq_topology_clusters.experimental.v1",
        "method": "average_link_agglomerative",
        "threshold": args.cluster_threshold,
        "identity_features_used": False,
        "clusters": clusters,
    }
    write_json(output_dir / "clusters.json", cluster_payload)

    cluster_by_case = {case_id: cluster["cluster_id"] for cluster in clusters
                       for case_id in cluster["member_case_ids"]}
    with (output_dir / "capability_matrix.csv").open(
            "w", encoding="utf-8-sig", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=[
            "case_id", "plugin_name", "surface_kind", "parameter_count",
            "pagination_complete", "surface_signature_sha256",
            "topology_signature_sha256", "cluster_id", "public_classification",
            "complete_block_count", "execution_status", "set_eq_point_supported",
            "frequency", "gain", "q", "filter_kind", "slope", "activation",
            "reasons", "evidence_level",
        ])
        writer.writeheader()
        for item in analyzed:
            execution = item["execution"]
            capabilities = execution["capabilities"]
            writer.writerow({
                "case_id": item["case_id"],
                "plugin_name": item["plugin_name"],
                "surface_kind": item["surface_kind"],
                "parameter_count": item["parameter_count"],
                "pagination_complete": (item.get("pagination") or {}).get("complete"),
                "surface_signature_sha256": item["surface_signature_sha256"],
                "topology_signature_sha256": item["topology_signature_sha256"],
                "cluster_id": cluster_by_case[item["case_id"]],
                "public_classification": item["public_classification"] or "unresolved",
                "complete_block_count": execution["complete_block_count"],
                "execution_status": execution["status"],
                "set_eq_point_supported": execution["set_eq_point_supported"],
                **capabilities,
                "reasons": "; ".join(execution["reasons"]),
                "evidence_level": execution["evidence_level"],
            })

    report = report_markdown(
        run_dir, analyzed, failed, clusters, summary.get("audit", {}))
    report_path = Path(args.report_path)
    report_path.parent.mkdir(parents=True, exist_ok=True)
    report_path.write_text(report, encoding="utf-8")
    (output_dir / "WAVES_EQ_CONTROL_TOPOLOGY_CENSUS_PHASE1.md").write_text(
        report, encoding="utf-8")
    write_json(output_dir / "analysis_summary.json", {
        "schema_version": "waves.eq_topology_analysis_summary.v1",
        "analyzed_case_count": len(analyzed),
        "failed_case_count": len(failed),
        "cluster_count": len(clusters),
        "public_classifications": dict(Counter(
            item["public_classification"] or "unresolved" for item in analyzed)),
        "execution_statuses": dict(Counter(
            item["execution"]["status"] for item in analyzed)),
        "report_path": str(report_path.resolve()),
    })
    print(f"analyzed={len(analyzed)} failed={len(failed)} clusters={len(clusters)}")
    print(f"report={report_path.resolve()}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""build_whitelist_v6_full.py — 实验插件白名单 v6 全候选派生构建器（入仓工具）。

来源与先例：PORT-WL-EQ-1 build_whitelist_eq1.py、PORT-PLUGIN-SELECT-MAC-1
build_whitelist_v6.py 系列整理泛化（PORT-PCA-FULL-CANDIDATES-1 目标 2）。
机器本地运行产物（白名单/溯源表）不入仓，仅本脚本入仓。

派生规则（PORT-PCA-FULL-CANDIDATES-1 卡面冻结，含 PC-1 步1 四裁定校准）：
  候选 = attestation store 内 status=promoted 的主体
       ∧ 认证 coverage 含该族实验轴（轴名映射见 FAMILY 表；transient 轴按
         PC 裁定③不预放宽：仅 envelope_timing 计入，envelope_emphasis 不算）
       ∧ 参数锚点可由探测快照诚实派生（零手编红线）；
  不可派生者如实不入列 + 溯源表记因（EMO-F2 先例）。
  Mono/Stereo 变体全入列（PC 裁定 B：通道形态是结构性事实，去重=策展）。

五源溯源（provenance 输出的 source 编号）：
  S0 活白名单（--live；--live-sha 可选哈希钉住）——static_eq 条目原样承接
     （自有溯源链在前卡 provenance_table 中），schema 必须已是 v6。
  S1 语义索引（--semantics）——条目身份五字段来源；与 S2 主体逐字节交叉核对。
  S2 attestation store（--attestation-v1 / --attestation-v2）——晋升与轴覆盖
     判定真源；v1 承载 broadband_compressor（broadband_compression 族），
     v2 承载 de_esser/transient_shaper/limiter/gate_expander/multiband_dynamics。
  S3 探测快照目录（--probe-dir）——本卡双周期 pluginprobe 快照（
     <slug>.a/.b.snapshot.json）；锚点参数必须双周期零漂移 + stable_id。
  S4 PCA 认证收据目录（--pca-cert-dir）——pca_job summary/cases evidence；
     apply_response.result.controls[].actual_readback 的 role→param_id 写锚，
     与 S3 探测锚点交叉核对（multiband 收据写的是 band gain/attack，族级
     证据核对用，threshold 参数锚仍以 S3 为准——PC-1 步1 §4 multiband 注）。

用法（mac 本机实测形态）：
  python3 scripts/build_whitelist_v6_full.py \
    --live ~/.vit/free_state_experiment_plugins.json \
    --semantics ~/.vit/plugin_semantics.json \
    --attestation-v1 ~/.vit/processor_control_attestations.v1.json \
    --attestation-v2 ~/.vit/processor_control_attestations.v2.json \
    --pca-cert-dir ~/.vit/pca_certifications \
    --probe-dir ~/Documents/vit-pca-full-candidates-artifacts/probe \
    --out whitelist_v6_full.json \
    --provenance provenance_table_v6_full.json

输出：
  --out         vit.free_state_experiment_plugins.v6 白名单（七族全数组形态；
                broadband 条目按共享单参形态写 threshold_param_id——
                FIX-BROADBAND-SHARED-1 决策点 A；若探测面呈 ch1/ch2 对则
                fail-closed 报错而非猜形态）。
  --provenance  溯源表 JSON 行数组：section/field/value/source/note，
                含排除主体记因行（_excluded）。

任何不一致（身份字段 S1≠S2、探测锚≠收据锚、漂移、非 stable_id、
族内重复 identifier、multiband 带号不连续、歧义参数名多命中）→ 非零退出。
"""
import argparse
import glob
import hashlib
import json
import re
import sys
from pathlib import Path

V6 = "vit.free_state_experiment_plugins.v6"

# 每族派生配置：白名单 key、attestation 主体族名、所需轴覆盖（S2 coverage axes
# 必须全含）、收据角色（S4 写锚 role）、探测面锚点参数名匹配、条目锚字段名。
# transient 轴严格化（PC 裁定③）：axis_requirements 仅 envelope_timing，
# envelope_emphasis 近轴不计入。
FAMILY_DEFS = [
    {
        "key": "de_esser",
        "att_family": "de_esser",
        "store": "v2",
        "axis_requirements": ["threshold_sensitivity"],
        "axis_field": "threshold_param_id",
        "probe_patterns": [r"^Threshold$"],
        "receipt_roles": ["threshold"],
    },
    {
        "key": "transient_shaper",
        "att_family": "transient_shaper",
        "store": "v2",
        "axis_requirements": ["envelope_timing"],
        "axis_field": "attack_param_id",
        "probe_patterns": [r"^AttackDuration$"],
        "receipt_roles": ["attack", "attack_duration"],
    },
    {
        "key": "limiter",
        "att_family": "limiter",
        "store": "v2",
        "axis_requirements": ["output_ceiling"],
        "axis_field": "ceiling_param_id",
        # 实测名：L1 "Ceiling"、L2 "Ceiling Slider"
        "probe_patterns": [r"^Ceiling( Slider)?$"],
        "receipt_roles": ["ceiling"],
    },
    {
        "key": "gate_expander",
        "att_family": "gate_expander",
        "store": "v2",
        "axis_requirements": ["attenuation_floor"],
        "axis_field": "range_param_id",
        "probe_patterns": [r"^Range$"],
        "receipt_roles": ["range"],
    },
    {
        "key": "multiband",
        "att_family": "multiband_dynamics",
        "store": "v2",
        "axis_requirements": ["band_dynamics", "band_timing"],
        "axis_field": "band_threshold_param_ids",
        # 实测名：C4/LinMB "Band N Thresh"、C6 "Band N Threshold"
        "probe_patterns": [r"^Band (\d+) Thresh(old)?$"],
        "receipt_roles": [],  # 收据写 band gain/attack，作族级核对，见下
    },
    {
        "key": "broadband_compression",
        "att_family": "broadband_compressor",
        "store": "v1",
        "axis_requirements": ["activation_intensity"],
        "axis_field": "threshold_param_id",
        "probe_patterns": [r"^Threshold$"],
        "receipt_roles": ["threshold"],
    },
]

IDENTITY_FIELDS = ("plugin_name", "manufacturer", "format",
                   "plugin_identifier", "plugin_path")


def fatal(msg):
    sys.exit(f"FATAL: {msg}")


def load(path):
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def sha256(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def slug_of(name):
    return name.replace(" ", "_").replace("/", "_")


class Sources:
    def __init__(self, args):
        self.args = args
        live_sha = sha256(args.live)
        if args.live_sha and live_sha != args.live_sha:
            fatal(f"live whitelist sha256 {live_sha} != pinned {args.live_sha}")
        self.live = load(args.live)
        if self.live.get("schema_version") != V6:
            fatal(f"live whitelist is {self.live.get('schema_version')}, expected {V6}")
        self.live_sha = live_sha
        self.sem = load(args.semantics)
        self.sem_by_name = {e["name"]: e for e in self.sem["entries"]}
        self.att_v1 = load(args.attestation_v1)
        self.att_v2 = load(args.attestation_v2)

    def promoted(self, store, att_family):
        att = self.att_v1 if store == "v1" else self.att_v2
        out = []
        for a in att["attestations"]:
            if a["status"] == "promoted" and a["processor_family"] == att_family:
                out.append(a)
        return out

    def snapshots(self, name):
        slug = slug_of(name)
        base = Path(self.args.probe_dir)
        pa, pb = base / f"{slug}.a.snapshot.json", base / f"{slug}.b.snapshot.json"
        if not pa.exists() or not pb.exists():
            return None, None, None
        sa, sb = load(pa), load(pb)
        ident_a = {p["name"]: str(p["id"]) for p in sa["surface"]["parameters"]}
        ident_b = {p["name"]: str(p["id"]) for p in sb["surface"]["parameters"]}
        return sa, ident_a, ident_b

    def receipt_anchors(self, name):
        """(role → param_id) 写锚 + 收据文件清单，来自本机 pca_certifications。
        锚点直读 evidence.apply_response.result.controls[].actual_readback
        （role/param_id/channel 字段原生在场，无需解码 control_ref）。"""
        anchors, receipts = {}, []
        for summary in sorted(glob.glob(str(Path(self.args.pca_cert_dir)
                                           / "pca_job_*" / "summary.json"))):
            j = load(summary)
            for r in j.get("results", []):
                if r.get("plugin_name") != name or r.get("status") != "passed":
                    continue
                receipts.append(str(summary))
                ev_files = glob.glob(str(Path(summary).parent / "cases" / "*"
                                         / "evidence.json"))
                for ev_file in ev_files:
                    ev = load(ev_file)
                    apply = ev.get("apply_response") or {}
                    for control in (apply.get("result") or {}).get("controls", []):
                        for w in control.get("actual_readback", []):
                            role, pid = w.get("role"), str(w.get("param_id"))
                            if not role or pid == "None":
                                continue
                            prev = anchors.get(role)
                            if prev is not None and prev != pid:
                                fatal(f"{name}: receipt role {role} param drift "
                                      f"{prev} != {pid}")
                            anchors[role] = pid
        return anchors, receipts


def identity_from_semantics(sources, subject_name, att_subject, prov, section):
    entry = sources.sem_by_name.get(subject_name)
    if entry is None:
        fatal(f"{subject_name} not in semantic index")
    for k_sem, k_att in (("name", "name"), ("manufacturer", "manufacturer"),
                         ("format", "format"), ("identifier", "identifier"),
                         ("plugin_path", "installed_path")):
        if entry[k_sem] != att_subject[k_att]:
            fatal(f"{subject_name} identity mismatch on {k_sem}: "
                  f"S1 {entry[k_sem]!r} != S2 {att_subject[k_att]!r}")
    prov.append({"section": section, "field": "plugin_name", "value": entry["name"],
                 "source": "S1 semantic index; byte-equal to S2 attestation subject",
                 "note": ""})
    for k, k_sem in (("manufacturer", "manufacturer"), ("format", "format"),
                     ("plugin_identifier", "identifier"),
                     ("plugin_path", "plugin_path")):
        prov.append({"section": section, "field": k, "value": entry[k_sem],
                     "source": "S1 = S2 cross-checked", "note": ""})
    return entry


def match_axis_params(name, ident_a, patterns, multi=False):
    """返回各 pattern 的命中列表 [(param_name, param_id)]（multi=True 时每
    pattern 可多命中，供 multiband 带面；否则零/多命中歧义 fail-closed）。"""
    hits = []
    for pattern in patterns:
        matched = [(n, i) for n, i in ident_a.items() if re.match(pattern, n)]
        if not multi and len(matched) > 1:
            fatal(f"{name}: axis pattern {pattern!r} matched multiple params "
                  f"{matched} — ambiguous, refusing to guess")
        hits.append(matched)
    return hits


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--live", required=True)
    ap.add_argument("--live-sha", default="",
                    help="expected sha256 of --live (fail-closed pin)")
    ap.add_argument("--semantics", required=True)
    ap.add_argument("--attestation-v1", required=True)
    ap.add_argument("--attestation-v2", required=True)
    ap.add_argument("--pca-cert-dir", required=True)
    ap.add_argument("--probe-dir", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--provenance", required=True)
    args = ap.parse_args()

    sources = Sources(args)
    prov = []
    whitelist = {"schema_version": V6}

    # ---- static_eq：S0 承接（前卡自有溯源链）-------------------------------
    if not isinstance(sources.live.get("static_eq"), list):
        fatal("live static_eq is not the v6 array form")
    whitelist["static_eq"] = sources.live["static_eq"]
    ids = [e["plugin_identifier"] for e in whitelist["static_eq"]]
    if len(ids) != len(set(ids)):
        fatal("duplicate plugin_identifier across live static_eq entries")
    prov.append({"section": "static_eq", "field": "_carryover",
                 "value": f"{len(ids)} entries verbatim",
                 "source": f"S0 live v6 (sha256 {sources.live_sha[:16]}…); "
                           "provenance chain of prior cards applies unchanged",
                 "note": "候选面全量派生卡不动 EQ 已锚定条目"})

    # ---- 六族全候选派生 ------------------------------------------------------
    candidate_counts = {}
    for fam in FAMILY_DEFS:
        key = fam["key"]
        entries, seen_ids = [], set()
        promoted = sources.promoted(fam["store"], fam["att_family"])
        att_by_name = {}
        for a in promoted:
            att_by_name.setdefault(a["subject"]["name"], []).append(a)
        excluded = []
        for subject_name in sorted(att_by_name):
            attestations = att_by_name[subject_name]
            a = attestations[0]
            cov = [c.get("axis") for c in a.get("coverage", [])]
            cov_str = ",".join(cov)
            missing_axes = [ax for ax in fam["axis_requirements"] if ax not in cov]
            snap, ident_a, ident_b = sources.snapshots(subject_name)
            if snap is None:
                excluded.append((subject_name,
                                 f"no dual-cycle probe snapshots for {subject_name}"))
                continue
            if ident_a != ident_b:
                fatal(f"{subject_name}: probe identity drift across cycles")
            entry_sem_key = None
            receipt_anchors, receipts = sources.receipt_anchors(subject_name)
            hit_lists = match_axis_params(subject_name, ident_a,
                                           fam["probe_patterns"],
                                           multi=(key == "multiband"))
            if missing_axes:
                # 如实不入列 + 记因（冻结规则；探测/收据侧事实一并记录以备后查）
                axis_param_note = ""
                for pattern, matched_list in zip(fam["probe_patterns"], hit_lists):
                    axis_param_note += (f"; probe {pattern!r} → "
                                        + ("no surface match" if not matched_list
                                           else (f"{matched_list[0][0]!r} "
                                                 f"id={matched_list[0][1]}"
                                                 if len(matched_list) == 1
                                                 else f"{len(matched_list)} matches "
                                                      f"{matched_list}")))
                excluded.append((
                    subject_name,
                    f"S2 coverage [{cov_str}] lacks axis "
                    f"{fam['axis_requirements']}{axis_param_note}"
                    + (f"; S4 receipts {len(receipts)} passed" if receipts else
                       "; no local S4 receipt")))
                continue
            if key == "multiband":
                matched = hit_lists[0]
                if not matched:
                    excluded.append((subject_name,
                                     "S2 axis coverage pass but probe surface "
                                     f"exposes no band threshold params "
                                     f"({fam['probe_patterns'][0]!r})"))
                    continue
            else:
                if not hit_lists[0]:
                    excluded.append((subject_name,
                                     "S2 axis coverage pass but probe surface "
                                     f"exposes no {fam['axis_field']} param "
                                     f"({fam['probe_patterns'][0]!r})"))
                    continue
            entry_sem = identity_from_semantics(sources, subject_name,
                                                a["subject"], prov,
                                                f"{key}[{len(entries)}]")
            section = f"{key}[{len(entries)}]"
            entry = {"plugin_name": entry_sem["name"],
                     "manufacturer": entry_sem["manufacturer"],
                     "format": entry_sem["format"],
                     "plugin_identifier": entry_sem["identifier"],
                     "plugin_path": entry_sem["plugin_path"]}
            if key == "multiband":
                matched = hit_lists[0]
                bands = {}
                for n, i in matched:
                    num = int(re.match(fam["probe_patterns"][0], n).group(1))
                    if num in bands:
                        fatal(f"{subject_name}: duplicate band number {num}")
                    bands[num] = (n, i)
                if not bands:
                    excluded.append((subject_name, "no band threshold params"))
                    continue
                ordered = [bands[n] for n in sorted(bands)]
                if sorted(bands) != list(range(1, len(bands) + 1)):
                    fatal(f"{subject_name}: band numbers not contiguous 1..N: "
                          f"{sorted(bands)}")
                ids_list = [i for _, i in ordered]
                if len(ids_list) != len(set(ids_list)):
                    fatal(f"{subject_name}: duplicate band threshold ids")
                entry["band_threshold_param_ids"] = ids_list
                prov.append({"section": section, "field": "band_threshold_param_ids",
                             "value": ids_list,
                             "source": f"S3 dual-cycle probe (zero drift): "
                                       f"{[n for n, _ in ordered]}",
                             "note": ("band threshold ids probe-derived; S4 "
                                      "receipts certify the family (band "
                                      f"gain/attack writes x{len(receipts)}), "
                                      "threshold level is probe-only "
                                      "(PC-1 step1 §4 note)"
                                      if receipts else
                                      "band threshold ids probe-derived")})
            elif key == "broadband_compression":
                (t_name, t_id), = hit_lists[0]
                # 共享单参 vs ch1/ch2 对：本工具按共享单参写（决策点 A）；
                # 探测面若呈 *_ch1/_ch2 对形态则 fail-closed 上交人工形态裁定。
                ch_style = [n for n in ident_a
                            if re.match(r"^Threshold(_ch)?[12]$", n)]
                if ch_style:
                    fatal(f"{subject_name}: probe surface exposes channel-pair "
                          f"threshold params {ch_style} — dual form needs manual "
                          "shape decision, refusing to guess")
                entry["threshold_param_id"] = t_id
                prov.append({"section": section, "field": "threshold_param_id",
                             "value": t_id,
                             "source": f"S3 dual-cycle probe (zero drift): "
                                       f"param {t_name!r} id={t_id} single shared",
                             "note": "FIX-BROADBAND-SHARED-1 shared single-param "
                                     "form (decision A)"})
            else:
                (a_name, a_id), = hit_lists[0]
                entry[fam["axis_field"]] = a_id
                prov.append({"section": section, "field": fam["axis_field"],
                             "value": a_id,
                             "source": f"S3 dual-cycle probe (zero drift): "
                                       f"param {a_name!r} id={a_id}",
                             "note": ""})
            # S4 收据写锚交叉核对（收据存在该 role 时）
            for role in fam["receipt_roles"]:
                if role in receipt_anchors:
                    probe_id = (entry.get(fam["axis_field"])
                                if key != "multiband" else None)
                    if probe_id is not None and probe_id != receipt_anchors[role]:
                        fatal(f"{subject_name}: probe anchor {probe_id} != "
                              f"receipt role {role} anchor "
                              f"{receipt_anchors[role]}")
                    prov.append({"section": section,
                                 "field": fam["axis_field"] + "_xcheck",
                                 "value": receipt_anchors[role],
                                 "source": f"S4 receipt write role={role}",
                                 "note": "MATCH probe anchor"
                                 if probe_id == receipt_anchors[role]
                                 else "family-level receipt (no direct id compare)"})
            # stable_id 校验（锚点参数必须稳定）
            anchor_ids = ({entry[fam["axis_field"]]}
                          if key != "multiband"
                          else set(entry["band_threshold_param_ids"]))
            for p in snap["surface"]["parameters"]:
                if str(p["id"]) in anchor_ids:
                    if not p.get("stable_id"):
                        fatal(f"{subject_name}: anchor param id={p['id']} "
                              f"({p['name']}) not stable_id")
            if entry_sem["identifier"] in seen_ids:
                fatal(f"duplicate identifier {entry_sem['identifier']} in {key}")
            seen_ids.add(entry_sem["identifier"])
            entries.append(entry)
            prov.append({"section": section, "field": "_attestation",
                         "value": a["attestation_id"],
                         "source": f"S2 {fam['store']} store promotion; "
                                   f"coverage=[{cov_str}]",
                         "note": f"status=promoted family={fam['att_family']}"
                                 + (f"; receipts={len(receipts)}" if receipts else "")})
        whitelist[key] = entries
        candidate_counts[key] = len(entries)
        for name, reason in excluded:
            prov.append({"section": key, "field": "_excluded",
                         "value": f"{name} not listed",
                         "source": reason, "note": "frozen derivation rule: "
                         "promoted ∧ axis coverage ∧ derivable anchor"})

    with open(args.out, "w", encoding="utf-8") as f:
        json.dump(whitelist, f, indent=2, ensure_ascii=False)
        f.write("\n")
    with open(args.provenance, "w", encoding="utf-8") as f:
        json.dump(prov, f, indent=2, ensure_ascii=False)
        f.write("\n")

    total = sum(candidate_counts.values()) + len(whitelist["static_eq"])
    print(f"OK wrote {args.out}")
    print(f"  static_eq={len(whitelist['static_eq'])} (S0 carryover)")
    for k, v in candidate_counts.items():
        print(f"  {k}={v}")
    print(f"  total entries={total}")
    print(f"provenance rows: {len(prov)} -> {args.provenance}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

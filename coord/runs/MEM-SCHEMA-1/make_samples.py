#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""MEM-SCHEMA-1 设计期样例生成与自校验（非生产代码）。

作用：
  1) 从仓库内既有证据工件重建样例条目所需的 surface digest；
  2) 生成符合 docs/PLUGIN_MEMORY_SCHEMA_V1.md §3 字段规范的 12 条样例条目；
  3) 按同一份 schema 自校验，输出 SAMPLE_ENTRIES.jsonl / SAMPLES_FOR_DOC.md / VALIDATION.txt。

边界：
  - 本脚本不是 D14 memory store，不实现持久化、查询、consolidation、画像；
  - 不写任何生产代码路径，不进入 agent/、VitApp/、scripts/；
  - sample_mode=True 时允许 plugin_identifier / plugin_version / captured_at 为 null，
    但必须在 limitations 中显式出现 "[待用户确认]"；生产捕获不允许 null。
"""

import argparse
import hashlib
import json
import re
import sys
from datetime import datetime, timezone, timedelta
from pathlib import Path

SCHEMA_VERSION = "plugin-memory.entry/v1"
TZ = timezone(timedelta(hours=8))

CONTENT_KEYS = {
    "semantic_hint": {"subject", "candidates", "caveats"},
    "gotcha": {"symptom", "trigger_conditions", "observed_signature", "impact", "safe_policy", "verification"},
    "write_order_precondition": {"ordered_roles", "condition", "observed_signature", "authority_note"},
    "user_approved_setting": {"setting", "approval"},
    "ambiguity_resolution": {"question", "resolution", "rejected_alternatives", "basis"},
    "unsupported_boundary": {
        "attempted_tool",
        "rejection_code",
        "observed_scope",
        "family_label_vs_topology",
        "retry_policy",
    },
}
CONTENT_REQUIRED = {
    "semantic_hint": {"subject", "candidates", "caveats"},
    "gotcha": {"symptom", "trigger_conditions", "observed_signature", "impact", "safe_policy"},
    "write_order_precondition": {"ordered_roles", "condition", "observed_signature", "authority_note"},
    "user_approved_setting": {"setting", "approval"},
    "ambiguity_resolution": {"question", "resolution", "basis"},
    "unsupported_boundary": {"attempted_tool", "rejection_code", "observed_scope", "retry_policy"},
}
TRUST = {"observed", "user-confirmed", "agent-inferred", "conformed", "unsupported"}
TRUST_ALLOWED = {
    "semantic_hint": {"observed", "user-confirmed", "agent-inferred"},
    "gotcha": {"observed", "user-confirmed", "conformed", "agent-inferred"},
    "write_order_precondition": {"observed", "user-confirmed", "conformed", "agent-inferred"},
    "user_approved_setting": {"user-confirmed"},
    "ambiguity_resolution": {"observed", "user-confirmed", "agent-inferred", "conformed"},
    "unsupported_boundary": {"unsupported"},
}
# §3.4：content 中出现这些键即视为「可执行载荷」，schema 校验拒绝。
FORBIDDEN_KEY_RE = re.compile(
    r"(^|_)(mapping|alias|credential|secret|token|password|command|tool_call|tool_route|"
    r"control_ref|param_map|apply_plan|schedule|normalized_curve|instructions)($|_)",
    re.IGNORECASE,
)
TAG_RE = re.compile(r"^[a-z0-9][a-z0-9._-]*$")
ENV_KEY_RE = re.compile(r"^env1_[0-9a-f]{16}$")
DIGEST_RE = re.compile(r"^sha256:([0-9a-f]{16}|-)$")
EVIDENCE_RE = [
    re.compile(r"^(repo|run):[A-Za-z0-9._\-/]+#[A-Za-z0-9._\-\[\]/+~]+$"),
    re.compile(r"^workspace:[A-Za-z0-9._\-/]+#[A-Za-z0-9._\-\[\]/+~]+#sha256=[0-9A-F]{64}$"),
    re.compile(r"^vit://[a-z0-9._]+/[^\s]+/t=(all|\d+\.\.\d+)@[^#\s]+#(sha256:[0-9a-f]{16}|-)$"),
]
ENTRY_REF_RE = re.compile(
    r"^vit://memory\.entry/memory\.env:env1_[0-9a-f]{16}/t=all@[A-Za-z0-9._\-]+#(sha256:[0-9a-f]{16}|-)$"
)


def canonical_json(obj) -> str:
    return json.dumps(obj, ensure_ascii=False, sort_keys=True, separators=(",", ":"))


def h16(text: str) -> str:
    return hashlib.sha256(text.encode("utf-8")).hexdigest()[:16]


def parse_iso(text: str) -> datetime:
    dt = datetime.fromisoformat(text)
    return dt if dt.tzinfo else dt.replace(tzinfo=TZ)


def surface_digest_pro_q_3(capture_path: Path) -> str:
    """Pro-Q 3：对记录的完整参数面（param_id|name|display domain|discrete|enum labels）做 canonical 摘要。

    摘要算法是设计期约定（见文档 §3.6）：按 param_id 排序、逐行 UTF-8、sha256 截 16 hex。
    D14 落地时可替换为 pluginprobe/物化层的正式 canonical form；此时旧摘要按版本升级处理。
    """
    data = json.loads(capture_path.read_text(encoding="utf-8"))
    rows = []
    for x in data["parameters"]["parameters"]:
        domain = x.get("display_domain_candidate") or {}
        labels = ""
        probe = x.get("display_probe") or {}
        if probe.get("discrete_labels"):
            labels = ";".join(f"{l.get('value')}={l.get('label')}" for l in probe["discrete_labels"])
        rows.append(
            "|".join(
                [
                    str(x.get("param_id")),
                    str(x.get("name")),
                    str(domain.get("text")),
                    str(x.get("is_discrete")),
                    labels,
                ]
            )
        )
    rows.sort()
    return "sha256:" + h16("\n".join(rows))


def surface_digest_valhalla(surface_path: Path) -> str:
    data = json.loads(surface_path.read_text(encoding="utf-8"))["valhallasupermassive"]
    rows = []
    for x in data:
        pid = str(x["id"])
        key = int(pid) if pid.isdigit() else 10**9
        rows.append((key, "|".join([pid, str(x["name"]), str(x["value"]), str(x["discrete"]), str(x["steps"])])))
    rows.sort(key=lambda r: (r[0], r[1]))
    return "sha256:" + h16("\n".join(r[1] for r in rows))


def env_key(machine_key: str, plugin_identity: dict, surface_fingerprint: dict) -> str:
    canon = canonical_json(
        {
            "machine_key": machine_key,
            "plugin_identity": plugin_identity,
            "surface_fingerprint": surface_fingerprint,
        }
    )
    return "env1_" + h16(canon)


def repo(path: str, anchor: str) -> str:
    return f"repo:{path}#{anchor}"


def run_ref(path: str, anchor: str) -> str:
    return f"run:{path}#{anchor}"


def ws(path: str, anchor: str, sha256: str) -> str:
    return f"workspace:{path}#{anchor}#sha256={sha256.upper()}"


class SampleBuilder:
    def __init__(self, main_repo: Path, worktree: Path):
        self.main_repo = main_repo
        self.worktree = worktree
        self.entries = []

    def add(self, *, seq, entry_type, trust, confidence, evidence_count, evidence_refs, context_tags,
            plugin_identity, surface_fingerprint, content, limitations, created_at, expires_at=None,
            captured_at=None):
        machine_key = "m_local"  # 样例占位：真实条目的 machine_key 是机器本地不透明指纹（见 §3.6）
        sfp = dict(surface_fingerprint)
        sfp["captured_at"] = captured_at
        env = env_key(machine_key, plugin_identity, sfp)
        payload = {
            "schema_version": SCHEMA_VERSION,
            "entry_type": entry_type,
            "environment": {"machine_key": machine_key, "env_key": env},
            "trust": trust,
            "confidence": confidence,
            "evidence_count": evidence_count,
            "evidence_refs": evidence_refs,
            "context_tags": context_tags,
            "created_at": created_at,
            "updated_at": created_at,
            "expires_at": expires_at,
            "plugin_identity": plugin_identity,
            "surface_fingerprint": sfp,
            "content": content,
            "limitations": limitations,
        }
        digest = "sha256:" + h16(canonical_json(payload))
        entry_ref = f"vit://memory.entry/memory.env:{env}/t=all@memschema-sample-{seq}#{digest}"
        entry = {
            "schema_version": SCHEMA_VERSION,
            "entry_type": entry_type,
            "entry_ref": entry_ref,
            "environment": payload["environment"],
            "trust": trust,
            "confidence": confidence,
            "evidence_count": evidence_count,
            "evidence_refs": evidence_refs,
            "context_tags": context_tags,
            "created_at": created_at,
            "updated_at": created_at,
            "expires_at": expires_at,
            "plugin_identity": plugin_identity,
            "surface_fingerprint": sfp,
            "content": content,
            "limitations": limitations,
        }
        self.entries.append(entry)
        return entry


def build_entries(main_repo: Path, worktree: Path) -> list:
    b = SampleBuilder(main_repo, worktree)
    pq_capture = main_repo / "artifacts/eq_compat_matrix/20260728_011728/capture/pro_q_3.json"
    pq_summary = main_repo / "artifacts/eq_compat_matrix/20260728_011728/capture/summary.json"
    if not pq_capture.exists():
        sys.exit(f"missing evidence artifact: {pq_capture}")
    pq_sha = hashlib.sha256(pq_capture.read_bytes()).hexdigest().upper()
    pq_sum_sha = hashlib.sha256(pq_summary.read_bytes()).hexdigest().upper()
    pq_digest = surface_digest_pro_q_3(pq_capture)

    val_run3 = "coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/pgtdc_20261016_205639"
    val_run2 = "coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/pgtdc_20261016_204214"
    val_surface = worktree / val_run3 / "surfaces_compact.json"
    v_digest = surface_digest_valhalla(val_surface)
    report = main_repo / "coord/reports/2026-10-18-reaper-mcp-plugin-param-research.md"
    rep_sha = hashlib.sha256(report.read_bytes()).hexdigest().upper() if report.exists() else None

    ts = "2026-10-18T14:30:00+08:00"

    pq_identity = {
        "vendor": "FabFilter",
        "product": "Pro-Q 3",
        "plugin_identifier": "VST3-Pro-Q 3-a1aba170-ba5f7ab2",
        "plugin_format": "VST3",
        "plugin_version": "3.2.3.0",
    }
    pq_fp = {
        "tool": "plugin_grabber.inspect_eq",
        "generation": "eqt1_1c011654e3fffcaf6d968457",
        "param_surface_digest": pq_digest,
    }
    c2_identity = {
        "vendor": "FabFilter",
        "product": "Pro-C 2",
        "plugin_identifier": None,
        "plugin_format": "VST3",
        "plugin_version": None,
    }
    c2_fp = {
        "tool": "plugin_grabber.inspect_compressor",
        "generation": "c2t1_6b96b6b6969df9a2ddbf22af",
        "param_surface_digest": "sha256:-",
    }
    v_identity = {
        "vendor": "valhalla",
        "product": "ValhallaSupermassive",
        "plugin_identifier": "VST3-ValhallaSupermassive-5cffe1ee-7e8fe5d5",
        "plugin_format": "VST3",
        "plugin_version": None,
    }
    v_fp = {
        "tool": "plugin_grabber.inspect_delay",
        "generation": "dl1t1_9fc850d507fac64a9601736e",
        "param_surface_digest": v_digest,
    }

    # ---------- Pro-Q 3 ----------
    b.add(
        seq="q1",
        entry_type="write_order_precondition",
        trust="observed",
        confidence=0.85,
        evidence_count=3,
        evidence_refs=[
            ws("artifacts/eq_compat_matrix/20260728_011728/capture/pro_q_3.json", "smoke.result.writes", pq_sha),
            repo("agent/internal/workflows/plugingrabber/eq_band_summary.go", "L110-L111"),
            repo("docs/EQ_CONTROL_TOPOLOGY_MONOTONIC_REDESIGN_RESEARCH.md", "section-8-monotonic-extension-contract"),
        ],
        context_tags=[
            "domain.eq",
            "product.fabfilter.pro-q-3",
            "tool.plugin_grabber.apply_eq_edits",
            "context.static-eq",
            "risk.write-order",
        ],
        plugin_identity=pq_identity,
        surface_fingerprint=pq_fp,
        created_at=ts,
        captured_at="2026-07-28T01:17:28+08:00",
        content={
            "ordered_roles": ["shape", "frequency", "q", "gain", "activation"],
            "condition": "free_floating（allocatable slot）EQ 的 upsert，经 plugin_grabber.apply_eq_edits 原子事务执行",
            "observed_signature": "真实 apply 的 writes/actual_readback 序列为 shape→frequency→q→gain→used(activation)，activation 位于末尾且整体 status=exact；参数面含 24 组 Used/Enabled allocatable slot",
            "authority_note": "advisory only：写入顺序的执行权威是 typed executor 契约（activation-last）与 live inspect 绑定；memory 不得覆盖事务计划，也不得据此改成其它写序",
        },
        limitations=[
            "产品版本 3.2.3.0；非通用规则（其它主机/工具/版本可能不同）",
            "第三方调研报告存在相反写序主张（见 C4 冲突条目），本条与 C4 并存保留、不合并",
            "不得作为执行权威（见文档 §5）",
        ],
    )
    b.add(
        seq="q2",
        entry_type="gotcha",
        trust="observed",
        confidence=0.8,
        evidence_count=2,
        evidence_refs=[
            ws("artifacts/eq_compat_matrix/20260728_011728/capture/pro_q_3.json",
               "parameters.parameters", pq_sha),
            repo("docs/COMPRESSOR_CONTROL_V1.md", "apply-contract"),
        ],
        context_tags=[
            "domain.eq",
            "product.fabfilter.pro-q-3",
            "risk.non-finite-endpoint",
            "tool.plugin_grabber.apply_eq_edits",
        ],
        plugin_identity=pq_identity,
        surface_fingerprint=pq_fp,
        created_at=ts,
        captured_at="2026-07-28T01:17:28+08:00",
        content={
            "symptom": "Dry Level / Wet Level 在 normalized=0 的显示文本为 `-INF dB`（非有限端点）；Band N Threshold 在 normalized=1 的显示文本为命名端点 `Auto`",
            "trigger_conditions": [
                "按显示最小/最大值反推 normalized↔physical 线性曲线",
                "把 -INF 或 Auto 当作连续数值端点参与插值",
            ],
            "observed_signature": "display_probe：Dry/Wet Level samples [0→-INF dB, .25→-12.0, .5→-6.0, .75→-2.5, 1→+0.0]；Band 1 Threshold samples [0→-90.0 dB, .25→-45.0, .5→-30.0, .75→-15.0, 1→Auto]",
            "impact": "有限目标值可能被错误解析到非有限/命名端点，或把端点状态当连续域，导致 typed 事务拒绝或语义误写",
            "safe_policy": [
                "一律以 live inspect 的 measured domain 与 display probe 为准",
                "有限写入目标必须落在有限数值区间；-INF/Auto 等只作为特殊状态处理",
                "写入后使用 typed readback（exact/quantized）校验，禁止自行反推曲线",
            ],
            "verification": "apply 前后 full preimage readback；遇到目标落在非有限端点时由 typed 层拒绝",
        },
        limitations=[
            "端点语义是 Pro-Q 3 3.2.3.0 记录的值；其它版本可能不同，使用前重新 live inspect",
            "只描述显示端点现象，不构成任何 normalized↔physical 映射",
        ],
    )
    b.add(
        seq="q3",
        entry_type="semantic_hint",
        trust="observed",
        confidence=0.75,
        evidence_count=3,
        evidence_refs=[
            ws("artifacts/eq_compat_matrix/20260728_011728/capture/pro_q_3.json", "parameters.parameters", pq_sha),
            repo("docs/EQ_CONTROL_TOPOLOGY_MONOTONIC_REDESIGN_RESEARCH.md", "section-4-1-surfacescope"),
            repo("docs/EQ_AUXILIARY_EXCLUSION_FIX_VALIDATION.md", "root-cause-and-fix"),
        ],
        context_tags=[
            "domain.eq",
            "product.fabfilter.pro-q-3",
            "context.static-eq",
            "context.ambiguity",
        ],
        plugin_identity=pq_identity,
        surface_fingerprint=pq_fp,
        created_at=ts,
        captured_at="2026-07-28T01:17:28+08:00",
        content={
            "subject": {
                "control_name": "Band N Dynamic Range / Dynamics Enabled / Threshold / External Side Chain / Speakers / Ex Band N Speakers",
                "display_group": "Dynamics / Mix 等",
            },
            "candidates": [
                {
                    "meaning": "静态 EQ core（Frequency/Gain/Q/Shape/Activation）的同频段辅助字段",
                    "confidence": 0.8,
                    "basis": "参数面为 24 组同构 Band N 字段；EQ 拓扑研究把 Dynamic/Threshold/External Side Chain 归入 auxiliary bindings，不参与静态 core 写入",
                },
                {
                    "meaning": "Ex Band N Speakers 属 external side chain 扬声器配置，不是独立 band",
                    "confidence": 0.85,
                    "basis": "同 section 辅助字段不改变 main band 归属（EQ 拓扑研究 §4.1）",
                },
            ],
            "caveats": [
                "参数名含 Band/Threshold 不等于主链路控制；任何 role 绑定以 live inspect 结果为准",
                "不得把 Dynamic Range 当作静态 Gain 使用",
            ],
        },
        limitations=[
            "解释性提示（weak hint 族），不是已验证 DSP 结论",
            "只提示命名歧义，不提供参数绑定或映射",
        ],
    )
    b.add(
        seq="q4",
        entry_type="gotcha",
        trust="observed",
        confidence=0.75,
        evidence_count=3,
        evidence_refs=[
            ws("artifacts/eq_compat_matrix/20260728_011728/capture/pro_q_3.json", "parameters.parameters", pq_sha),
            repo("docs/EQ_CONTROL_TOPOLOGY_MONOTONIC_REDESIGN_RESEARCH.md", "section-4-2-eqsectionfacts"),
            repo("docs/COM_V1_TEST_CONTRACT.md", "L132"),
        ],
        context_tags=[
            "domain.eq",
            "product.fabfilter.pro-q-3",
            "risk.channel-linkage",
            "context.static-eq",
        ],
        plugin_identity=pq_identity,
        surface_fingerprint=pq_fp,
        created_at=ts,
        captured_at="2026-07-28T01:17:28+08:00",
        content={
            "symptom": "每个 band 带 Stereo Placement（enum Left/Right/Stereo/Mid/Side）、Speakers 与 Ex Band N Speakers 等通道相关字段；全局另有 Output Pan / Output Pan Mode",
            "trigger_conditions": [
                "把全局请求静默落成单通道写入",
                "未证明 channel_contract 就写通道相关或 stereo placement 字段",
            ],
            "observed_signature": "参数面 24×Stereo Placement/Speakers/Ex Band N Speakers；EQ 拓扑研究要求先证明 channel_contract ∈ {shared, linked mirror, unsafe asymmetric} 再决定通道写入方式",
            "impact": "通道语义可能被误改（只改单通道、立体声联动失配），或把全局请求错误扁平化",
            "safe_policy": [
                "全局请求优先 shared binding；通道特定写入仅在 live inspect 证明 linked/shared 后执行",
                "不得把多路径/通道联动当作单一标量控制",
                "写后校验所有被绑定通道的读回",
            ],
            "verification": "typed readback + full preimage restore；channel_link 类事实不得由音频推断（COM 测试契约既有规则）",
        },
        limitations=[
            "通道契约按实例/版本由 live inspect 给出；本条只是风险提示，不代替 inspect",
            "不同插件族的 channel_contract 不同，禁止跨插件套用",
        ],
    )
    b.add(
        seq="q5",
        entry_type="unsupported_boundary",
        trust="unsupported",
        confidence=0.8,
        evidence_count=3,
        evidence_refs=[
            repo("docs/EQ_CONTROL_TOPOLOGY_MONOTONIC_REDESIGN_RESEARCH.md", "section-6-dynamic-eq-boundary"),
            repo("docs/MULTIBAND_DYNAMICS_CONTROL_V1.md", "L64-L74"),
            repo("docs/EQ_AUXILIARY_EXCLUSION_FIX_VALIDATION.md", "section-conclusion"),
        ],
        context_tags=[
            "domain.eq",
            "product.fabfilter.pro-q-3",
            "risk.capability-boundary",
            "tool.plugin_grabber.apply_eq_edits",
        ],
        plugin_identity=pq_identity,
        surface_fingerprint=pq_fp,
        created_at=ts,
        captured_at="2026-07-28T01:17:28+08:00",
        content={
            "attempted_tool": "plugin_grabber.apply_eq_edits（typed 静态 EQ core）",
            "rejection_code": "dynamic_section_excluded / unsupported_dynamic_eq",
            "observed_scope": "Dynamic Range / Threshold / Attack / Release / Ratio / External Side Chain 等动态/侧链行为不进入 typed 静态 EQ 控制范围；辅助字段本身仍可读",
            "family_label_vs_topology": "Pro-Q 3 产品上可做动态段，但 typed 静态 core 只控制静态 Frequency/Gain/Q/Shape/Activation；产品名/家族名不是控制证据",
            "retry_policy": "动态行为请求按 capability boundary 返回或换用支持该行为的处理器；不得用静态 Gain 冒充动态行为，不得让模型自行发明 fallback",
        },
        limitations=[
            "边界范围随 typed 工具能力演进；本条不是永久结论",
            "不得据此放宽 PCA 或 typed 准入门槛",
        ],
    )

    # ---------- Pro-C 2 ----------
    b.add(
        seq="c1",
        entry_type="gotcha",
        trust="observed",
        confidence=0.8,
        evidence_count=1,
        evidence_refs=[repo("docs/COMPRESSOR_REFERENCE_PRO_C_2.md", "measured-reference-surface")],
        context_tags=[
            "domain.compressor",
            "product.fabfilter.pro-c-2",
            "risk.parameter-domain",
            "context.parallel",
        ],
        plugin_identity=c2_identity,
        surface_fingerprint=c2_fp,
        created_at=ts,
        captured_at=None,
        expires_at="2027-01-16T14:30:00+08:00",
        content={
            "symptom": "Pro-C 2 的 Mix 实测物理域为 0–200%，不是通用的 0–100%",
            "trigger_conditions": [
                "按通用假设把 Mix 百分比钳制到 100%",
                "把 Mix 当作压缩强度控制",
            ],
            "observed_signature": "COMPRESSOR_REFERENCE_PRO_C_2 §Measured Reference Surface：`mix` 0 to 200% on this product; never assume 0 to 100%",
            "impact": "parallel balance 设置被错误截断或误读为压缩量",
            "safe_policy": [
                "百分比目标必须落在 live inspect 测得的物理域内",
                "禁止假设 0–100%；域检查以 inspect/apply 的 typed 校验为准",
            ],
            "verification": "typed readback（exact/quantized）+ measured domain 校验",
        },
        limitations=[
            "[待用户确认] 该测量来自 tracked 参考文档；原始测量工件路径（VitApp/Workspace/Artifacts/smoke/product_path_20260804_094717/…/pro_c_2.json）在 2026-10-18 核查时不在盘，使用前应重新 live inspect",
            "TTL 为 90 天建议值（expires_at 示例），实际过期执行归 D14",
        ],
    )
    b.add(
        seq="c2",
        entry_type="gotcha",
        trust="observed",
        confidence=0.8,
        evidence_count=3,
        evidence_refs=[
            repo("docs/COMPRESSOR_REFERENCE_PRO_C_2.md", "measured-reference-surface"),
            repo("docs/COMPRESSOR_CONTROL_V1.md", "apply-contract"),
            repo("docs/PLUGIN_ALLIANCE_COMPRESSOR_BLIND_V1_ROUND1.md", "failure-taxonomy"),
        ],
        context_tags=[
            "domain.compressor",
            "product.fabfilter.pro-c-2",
            "risk.non-finite-endpoint",
        ],
        plugin_identity=c2_identity,
        surface_fingerprint=c2_fp,
        created_at=ts,
        captured_at=None,
        content={
            "symptom": "Wet Gain / Dry Gain 的实测域带非有限端点；显示文本可出现 `-INF dB`",
            "trigger_conditions": [
                "按显示 min/max 线性反推 normalized 曲线",
                "把 -INF 当作连续数值域端点",
            ],
            "observed_signature": "COMPRESSOR_REFERENCE_PRO_C_2：wet_gain/dry_gain `-18 to +36 dB, with a non-finite endpoint`；同类 -INF 命中已在 blind round 记录为拒绝（Zip threshold 目标 -60 dB → 显示 -inf dB，事务被拒并原子回滚）",
            "impact": "有限目标解析到非有限端点 → typed 层拒绝，或读到与预期不一致的端点状态",
            "safe_policy": [
                "有限目标不得映射到非有限端点（typed 契约既有规则）",
                "端点只按命名状态处理；曲线反推以 measured anchors 为准",
            ],
            "verification": "apply rejected 时确认无残留状态；成功时 exact/quantized readback",
        },
        limitations=[
            "[待用户确认] wet/dry 域名来自 tracked 参考文档，原始测量工件不在盘；使用前重新 live inspect",
            "不要据此条为其它插件推导非有限端点行为",
        ],
    )
    b.add(
        seq="c3",
        entry_type="ambiguity_resolution",
        trust="observed",
        confidence=0.85,
        evidence_count=2,
        evidence_refs=[
            repo("docs/COMPRESSOR_REFERENCE_PRO_C_2.md", "canonical-semantic-axes"),
            repo("docs/COMPRESSOR_REFERENCE_PRO_C_2.md", "directional-semantics"),
        ],
        context_tags=[
            "domain.compressor",
            "product.fabfilter.pro-c-2",
            "context.parallel",
            "context.ambiguity",
        ],
        plugin_identity=c2_identity,
        surface_fingerprint=c2_fp,
        created_at=ts,
        captured_at=None,
        content={
            "question": "`Mix` / `Wet Gain` / `Dry Gain` 是否表示压缩强度？",
            "resolution": "否。它们属 output/parallel balance 轴：降低 mix 削弱可听湿声贡献，但不削弱压缩器内部 gain reduction；output gain 不是 compression intensity",
            "rejected_alternatives": [
                "把 mix 当作 compression intensity",
                "用 output gain 代替压缩量（output gain 只是电平补偿）",
            ],
            "basis": "COMPRESSOR_REFERENCE §Canonical Semantic Axes 与 §Directional Semantics：parallel balance / output normalization 是独立轴",
        },
        limitations=[
            "语义澄清，不是参数映射；实际角色绑定以 live inspect 为准",
            "不同压缩器家族的 output 段结构不同，本条不跨插件套用",
            "[待用户确认] Pro-C 2 的 identity/version/identifier 未在证据工件中记录（原始测量工件不在盘）",
        ],
    )
    if rep_sha:
        b.add(
            seq="c4",
            entry_type="write_order_precondition",
            trust="agent-inferred",
            confidence=0.35,
            evidence_count=2,
            evidence_refs=[
                ws("coord/reports/2026-10-18-reaper-mcp-plugin-param-research.md", "section-5-3", rep_sha),
                repo("docs/EQ_CONTROL_TOPOLOGY_MONOTONIC_REDESIGN_RESEARCH.md", "section-8-monotonic-extension-contract"),
            ],
            context_tags=[
                "domain.eq",
                "domain.compressor",
                "product.fabfilter.pro-q-3",
                "product.fabfilter.pro-c-2",
                "risk.write-order",
                "source.external-report",
            ],
            plugin_identity=c2_identity,
            surface_fingerprint=c2_fp,
            created_at=ts,
            captured_at=None,
            content={
                "ordered_roles": ["used/enable activation", "frequency/gain/q"],
                "condition": "第三方 REAPER MCP 报告声称：Pro-Q 3 / Pro-C 2 需先写 Band 的 Used/Enabled 再写 Frequency/Gain，否则参数写了没声音，更严重会触发崩溃",
                "observed_signature": "外部报告（REAPER/host 工具链，未在 Vit-DAW 栈复现）；Vit 侧真实 apply 记录相反（activation-last，见 Q1）",
                "authority_note": "冲突保留、不合并：本条 trust=agent-inferred、仅作规划期风险提示；写入顺序权威=typed executor 契约 + live inspect；任何执行不得据此改变写序，不得据此绕过事务校验",
            },
            limitations=[
                "[待用户确认] 外部报告未经 Vit 栈复现；主机/工具/版本差异可能解释冲突",
                "不得把本条升级为执行权威或通用写序规则",
            ],
        )
    b.add(
        seq="c5",
        entry_type="user_approved_setting",
        trust="user-confirmed",
        confidence=0.9,
        evidence_count=2,
        evidence_refs=[
            repo("docs/COMPRESSOR_CONTROL_V1.md", "vit-natural-language-validation"),
            repo("docs/COMPRESSOR_REFERENCE_PRO_C_2.md", "worked-projection-examples"),
        ],
        context_tags=[
            "domain.compressor",
            "product.fabfilter.pro-c-2",
            "context.user-approved",
            "tool.plugin_grabber.apply_compressor_controls",
        ],
        plugin_identity=c2_identity,
        surface_fingerprint=c2_fp,
        created_at=ts,
        captured_at=None,
        content={
            "setting": {
                "roles": [
                    {"role": "threshold", "target_kind": "physical_absolute", "value": -12.0, "unit": "dB"},
                    {"role": "ratio", "target_kind": "physical_absolute", "value": 4.0, "unit": ":1"},
                ],
                "context": "Vit natural-language compressor validation：用户明确要求 Threshold=-12 dB、Ratio=4:1；disposable Pro-C 2 实例",
            },
            "approval": {
                "event_ref": "conversation:compressor_nl_final_20260803_2350",
                "scope": "单次会话/单实例的显式设置请求，不跨工程、不成为默认",
                "approved_at": "2026-08-03T23:50:00+08:00",
            },
        },
        limitations=[
            "只记录客观可复核的设置事实与用户显式请求，不承载偏好、不做默认值、不自动复用",
            "用户偏好类信息必须走 D13 质询确权 + D14 用户偏好类，不在本 schema",
            "[待用户确认] Pro-C 2 的 identity/version/identifier 未在证据工件中记录（原始测量工件不在盘）",
        ],
    )

    # ---------- Valhalla（同族 reverb） ----------
    b.add(
        seq="v1",
        entry_type="unsupported_boundary",
        trust="unsupported",
        confidence=0.9,
        evidence_count=3,
        evidence_refs=[
            run_ref(f"{val_run3}/valhallasupermassive.inspects.json", "inspects.reverb"),
            run_ref(f"{val_run2}/valhallasupermassive.inspects.json", "inspects.reverb"),
            run_ref("coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/README.md", "L58"),
        ],
        context_tags=[
            "domain.reverb",
            "domain.delay",
            "product.valhalla.supermassive",
            "risk.capability-boundary",
        ],
        plugin_identity=v_identity,
        surface_fingerprint=v_fp,
        created_at=ts,
        captured_at="2026-10-16T20:56:39+08:00",
        content={
            "attempted_tool": "plugin_grabber.inspect_reverb",
            "rejection_code": "unsupported_delay",
            "observed_scope": "reverb 家族插件无 provable reverb stage/decay 控制（无衰减词证据），inspect_reverb 拒绝；同一实例 inspect_delay 可解析为 feedback_delay",
            "family_label_vs_topology": "family=reverb 来自清单元数据；家族/产品名不是拓扑证据，reverb 词表拒绝不等于产品不可用",
            "retry_policy": "不得据家族名强行按 reverb 控制；按 capability boundary 返回或走手动指引/替换处理器；不得发明 fallback、不得默认近似写",
        },
        limitations=[
            "两条独立 run 均得到同一拒绝码（run2/run3）；插件版本未在工件中记录",
            "边界随词表/识别器演进；本条不是永久结论",
            "[待用户确认] ValhallaSupermassive 的 version 未在工件中记录",
        ],
    )
    b.add(
        seq="v2",
        entry_type="ambiguity_resolution",
        trust="observed",
        confidence=0.85,
        evidence_count=3,
        evidence_refs=[
            run_ref(f"{val_run3}/valhallasupermassive.inspects.json", "inspects.delay"),
            run_ref(f"{val_run2}/valhallasupermassive.inspects.json", "inspects.delay"),
            run_ref(f"{val_run3}/matrix_summary.json", "matrix[valhallasupermassive]"),
        ],
        context_tags=[
            "domain.reverb",
            "domain.delay",
            "product.valhalla.supermassive",
            "context.ambiguity",
        ],
        plugin_identity=v_identity,
        surface_fingerprint=v_fp,
        created_at=ts,
        captured_at="2026-10-16T20:56:39+08:00",
        content={
            "question": "ValhallaSupermassive 应按 reverb 还是 delay 控制？",
            "resolution": "以 live inspect 的分类为准：inspect_reverb 拒绝 unsupported_delay；inspect_delay 返回 feedback_delay（timing/output/feedback 段）。家族标签 reverb 不是控制证据",
            "rejected_alternatives": [
                "按 vendor family=reverb 猜测 reverb 参数",
                "按产品名套用 reverb 词表/写序",
            ],
            "basis": "两次独立 run 的 inspects 工件：reverb 面拒绝、delay 面 resolved=true、classification=feedback_delay、confidence 0.96",
        },
        limitations=[
            "两条 run 的 generation 不同（dl1t1_f8c727… / dl1t1_9fc850…），说明 generation 是实例级、不可跨会话缓存复用",
            "分类结果随识别器版本变化，使用前重新 inspect",
            "[待用户确认] ValhallaSupermassive 的 version 未在工件中记录",
        ],
    )
    b.add(
        seq="v3",
        entry_type="semantic_hint",
        trust="agent-inferred",
        confidence=0.45,
        evidence_count=2,
        evidence_refs=[
            run_ref(f"{val_run3}/surfaces_compact.json", "valhallasupermassive"),
            run_ref(f"{val_run3}/valhallasupermassive.inspects.json", "inspects.delay"),
        ],
        context_tags=[
            "domain.reverb",
            "domain.delay",
            "product.valhalla.supermassive",
            "context.ambiguity",
        ],
        plugin_identity=v_identity,
        surface_fingerprint=v_fp,
        created_at=ts,
        captured_at="2026-10-16T20:56:39+08:00",
        content={
            "subject": {
                "control_name": "Mix（其余面：DelaySync / DelayNote / Delay_Ms / DelayWarp / Feedback / Density / Width / LowCut / HighCut / ModRate / ModDepth / Mode / Reserved1-4）",
                "display_group": "output",
            },
            "candidates": [
                {
                    "meaning": "inspect 的 output 段声明 dry/mix/wet 三个角色，但实测参数面只录到 1 个 Mix（display 50.0 %）",
                    "confidence": 0.5,
                    "basis": "surfaces_compact 仅 1 个 mix 参数；inspects 工件声明 output=[dry,mix,wet]，两者绑定关系未在工件中给出",
                },
                {
                    "meaning": "不得假定 dry/wet 有独立可写 control_ref；角色绑定以 live inspect 结果为准",
                    "confidence": 0.8,
                    "basis": "同上；dry/wet 可能是 inspect 派生角色而非独立参数",
                },
            ],
            "caveats": [
                "[待用户确认] 需要一次 live inspect 验证 dry/wet 绑定",
                "本条仅为 agent 推断（trust=agent-inferred），不得作为参数映射或写计划来源",
            ],
        },
        limitations=[
            "[待用户确认] 工件未包含角色→参数绑定明细，推断待 live inspect 复验",
            "不得据此构造 normalized↔physical 映射",
        ],
    )
    return b.entries


def validate(entries, sample_mode=True):
    errors, warnings = [], []
    for e in entries:
        tag = e.get("entry_ref", "<no-ref>")[-40:]
        def err(msg):
            errors.append(f"[{tag}] {msg}")

        for key in ("schema_version", "entry_type", "entry_ref", "environment", "trust", "confidence",
                    "evidence_count", "evidence_refs", "context_tags", "created_at", "updated_at",
                    "expires_at", "plugin_identity", "surface_fingerprint", "content", "limitations"):
            if key not in e:
                err(f"missing field {key}")
        if errors and errors[-1].startswith(f"[{tag}] missing"):
            continue
        if e["schema_version"] != SCHEMA_VERSION:
            err("schema_version mismatch")
        et = e["entry_type"]
        if et not in CONTENT_KEYS:
            err(f"unknown entry_type {et}")
            continue
        if e["trust"] not in TRUST:
            err(f"unknown trust {e['trust']}")
        elif e["trust"] not in TRUST_ALLOWED[et]:
            err(f"trust {e['trust']} not allowed for {et}")
        conf = e["confidence"]
        if not isinstance(conf, (int, float)) or not (0.0 <= float(conf) <= 1.0):
            err("confidence out of [0,1]")
        ec = e["evidence_count"]
        if not isinstance(ec, int) or ec < 1:
            err("evidence_count must be int >= 1")
        refs = e["evidence_refs"]
        if not isinstance(refs, list) or not refs:
            err("evidence_refs must be non-empty list")
        else:
            for r in refs:
                if not isinstance(r, str):
                    err("evidence_ref must be string")
                elif not any(rx.match(r) for rx in EVIDENCE_RE):
                    err(f"evidence_ref grammar violation: {r[:60]}")
        tags = e["context_tags"]
        if not isinstance(tags, list) or not tags:
            err("context_tags must be non-empty list")
        else:
            for t in tags:
                if not isinstance(t, str) or not TAG_RE.match(t):
                    err(f"bad context_tag: {t!r}")
        try:
            c_at = parse_iso(e["created_at"])
            u_at = parse_iso(e["updated_at"])
            if u_at < c_at:
                err("updated_at < created_at")
            if e["expires_at"] is not None:
                if parse_iso(e["expires_at"]) <= c_at:
                    err("expires_at <= created_at")
        except (TypeError, ValueError):
            err("bad timestamp")
        env = e["environment"]
        if not isinstance(env, dict) or not ENV_KEY_RE.match(str(env.get("env_key", ""))):
            err("bad environment.env_key")
        elif env_key(env.get("machine_key"), e["plugin_identity"], e["surface_fingerprint"]) != env["env_key"]:
            err("environment.env_key not derivable from fields")
        if not ENTRY_REF_RE.match(e["entry_ref"]):
            err("entry_ref template mismatch")
        else:
            payload = {k: v for k, v in e.items() if k != "entry_ref"}
            digest = "sha256:" + h16(canonical_json(payload))
            if not e["entry_ref"].endswith("#" + digest):
                err("entry_ref content digest mismatch")
            if f"memory.env:{env['env_key']}" not in e["entry_ref"]:
                err("entry_ref env key mismatch")
        ident = e["plugin_identity"]
        for k in ("vendor", "product", "plugin_identifier", "plugin_format", "plugin_version"):
            if k not in ident:
                err(f"plugin_identity missing {k}")
            elif ident[k] is None:
                if not sample_mode or "[待用户确认]" not in " ".join(e["limitations"]):
                    err(f"plugin_identity.{k} null without [待用户确认] limitation")
                else:
                    warnings.append(f"[{tag}] plugin_identity.{k}=null（sample_mode，已标 [待用户确认]）")
            elif not isinstance(ident[k], str) or not ident[k]:
                err(f"plugin_identity.{k} bad type")
        sfp = e["surface_fingerprint"]
        if not isinstance(sfp.get("tool"), str) or not sfp.get("tool"):
            err("surface_fingerprint.tool missing")
        if not isinstance(sfp.get("generation"), str) or not sfp.get("generation"):
            err("surface_fingerprint.generation missing")
        if not DIGEST_RE.match(str(sfp.get("param_surface_digest", ""))):
            err("surface_fingerprint.param_surface_digest bad")
        if sfp.get("captured_at") is None:
            if not sample_mode or "[待用户确认]" not in " ".join(e["limitations"]):
                err("surface_fingerprint.captured_at null without [待用户确认] limitation")
        else:
            try:
                parse_iso(sfp["captured_at"])
            except (TypeError, ValueError):
                err("bad captured_at")
        content = e["content"]
        if not isinstance(content, dict):
            err("content must be object")
            continue
        allowed = CONTENT_KEYS[et]
        extra = set(content) - allowed
        if extra:
            err(f"content keys not allowed for {et}: {sorted(extra)}")
        missing = CONTENT_REQUIRED[et] - set(content)
        if missing:
            err(f"content required keys missing for {et}: {sorted(missing)}")
        stack = [content]
        while stack:
            node = stack.pop()
            if isinstance(node, dict):
                for k, v in node.items():
                    if FORBIDDEN_KEY_RE.search(str(k)):
                        err(f"forbidden executable payload key: {k}")
                    stack.append(v)
            elif isinstance(node, list):
                stack.extend(node)
        if et == "user_approved_setting":
            approval = content.get("approval") or {}
            setting = content.get("setting") or {}
            if not approval.get("event_ref"):
                err("user_approved_setting.approval.event_ref missing")
            if not setting.get("roles"):
                err("user_approved_setting.setting.roles missing")
            for role in setting.get("roles", []):
                if not {"role", "value"} <= set(role):
                    err("setting role missing role/value")
        if not isinstance(e["limitations"], list) or not e["limitations"]:
            err("limitations must be non-empty list")
        if any(not isinstance(x, str) for x in e["limitations"]):
            err("limitations entries must be strings")
    return errors, warnings


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--main-repo", default=r"D:\Vit_DAW", help="共享主检出（存放 gitignored artifacts/）")
    ap.add_argument("--worktree", default=str(Path(__file__).resolve().parents[3]), help="本卡 worktree 根")
    ap.add_argument("--out-dir", default=str(Path(__file__).resolve().parent))
    args = ap.parse_args()

    main_repo = Path(args.main_repo)
    worktree = Path(args.worktree)
    out_dir = Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)

    entries = build_entries(main_repo, worktree)
    errors, warnings = validate(entries, sample_mode=True)

    jsonl = out_dir / "SAMPLE_ENTRIES.jsonl"
    with jsonl.open("w", encoding="utf-8", newline="\n") as f:
        for e in entries:
            f.write(json.dumps(e, ensure_ascii=False, sort_keys=False) + "\n")

    # 供文档内联的 Markdown 片段
    by_plugin = [
        ("Pro-Q 3（FabFilter Pro-Q 3，VST3 3.2.3.0）", "q"),
        ("Pro-C 2（FabFilter Pro-C 2，VST3；identity 子字段未记录 → [待用户确认]）", "c"),
        ("Valhalla 同族 reverb（ValhallaSupermassive，VST3；version 未记录）", "v"),
    ]
    md = []
    for title, prefix in by_plugin:
        group = [e for e in entries if e["entry_ref"].split("@memschema-sample-")[1].startswith(prefix)]
        md.append(f"### {title}\n")
        md.append("| seq | entry_type | trust | confidence | evidence_count | 一句话 |")
        md.append("|---|---|---|---|---|---|")
        for e in group:
            seq = e["entry_ref"].split("@memschema-sample-")[1].split("#")[0]
            c = e["content"]
            if e["entry_type"] == "gotcha":
                short = c.get("symptom", "")[:60]
            elif e["entry_type"] == "semantic_hint":
                short = c.get("subject", {}).get("control_name", "")[:60]
            elif e["entry_type"] == "write_order_precondition":
                short = c.get("condition", "")[:60]
            elif e["entry_type"] == "ambiguity_resolution":
                short = c.get("question", "")[:60]
            elif e["entry_type"] == "unsupported_boundary":
                short = c.get("rejection_code", "")[:60]
            else:
                short = "user-approved setting（threshold=-12 dB / ratio=4:1）"
            short = short.replace("|", "/").replace("\n", " ")
            md.append(f"| {seq} | `{e['entry_type']}` | `{e['trust']}` | {e['confidence']} | {e['evidence_count']} | {short} |")
        md.append("")
        for e in group:
            seq = e["entry_ref"].split("@memschema-sample-")[1].split("#")[0]
            md.append(f"#### 样例 {seq.upper()}")
            md.append("")
            md.append("```json")
            md.append(json.dumps(e, ensure_ascii=False, indent=2))
            md.append("```")
            md.append("")
    (out_dir / "SAMPLES_FOR_DOC.md").write_text("\n".join(md), encoding="utf-8", newline="\n")

    lines = []
    lines.append("# MEM-SCHEMA-1 样例自校验报告")
    lines.append("")
    lines.append(f"- 生成时间：2026-10-18（设计期）")
    lines.append(f"- schema_version：{SCHEMA_VERSION}")
    lines.append(f"- 样例条目数：{len(entries)}（Pro-Q 3 / Pro-C 2 / ValhallaSupermassive）")
    lines.append(f"- 校验结果：{'PASS' if not errors else 'FAIL'}")
    lines.append("")
    lines.append("## 校验覆盖")
    lines.append("- 必填字段全集、schema_version、entry_type/trust 枚举与每类型 trust 允许集；")
    lines.append("- confidence ∈ [0,1]、evidence_count ≥ 1、evidence_refs 文法、context_tags 词法；")
    lines.append("- created_at/updated_at/expires_at 时序；environment.env_key 可由字段重算；")
    lines.append("- entry_ref 模板（vit://memory.entry/memory.env:…）、env key 与内容摘要自洽；")
    lines.append("- plugin_identity/surface_fingerprint 结构；sample_mode 下 null 必须带 [待用户确认]；")
    lines.append("- content 允许键闭合（additionalProperties=false 语义）+ 禁止可执行载荷键递归扫描；")
    lines.append("- user_approved_setting 的 approval/setting 结构；limitations 非空字符串列表。")
    lines.append("")
    lines.append("## 结果")
    if errors:
        for x in errors:
            lines.append(f"- ERROR {x}")
    else:
        lines.append(f"- {len(entries)}/{len(entries)} entries PASS")
    if warnings:
        lines.append("")
        lines.append("## 警告（设计期可接受，生产捕获不允许）")
        for x in warnings:
            lines.append(f"- WARN {x}")
    (out_dir / "VALIDATION.txt").write_text("\n".join(lines) + "\n", encoding="utf-8", newline="\n")

    print(f"entries={len(entries)} errors={len(errors)} warnings={len(warnings)}")
    for x in errors:
        print("ERROR", x)
    print(f"wrote {jsonl}")
    print(f"wrote {out_dir / 'SAMPLES_FOR_DOC.md'}")
    print(f"wrote {out_dir / 'VALIDATION.txt'}")
    return 1 if errors else 0


if __name__ == "__main__":
    sys.exit(main())

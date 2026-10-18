### Pro-Q 3（FabFilter Pro-Q 3，VST3 3.2.3.0）

| seq | entry_type | trust | confidence | evidence_count | 一句话 |
|---|---|---|---|---|---|
| q1 | `write_order_precondition` | `observed` | 0.85 | 3 | free_floating（allocatable slot）EQ 的 upsert，经 plugin_grabber. |
| q2 | `gotcha` | `observed` | 0.8 | 2 | Dry Level / Wet Level 在 normalized=0 的显示文本为 `-INF dB`（非有限端点） |
| q3 | `semantic_hint` | `observed` | 0.75 | 3 | Band N Dynamic Range / Dynamics Enabled / Threshold / Extern |
| q4 | `gotcha` | `observed` | 0.75 | 3 | 每个 band 带 Stereo Placement（enum Left/Right/Stereo/Mid/Side）、 |
| q5 | `unsupported_boundary` | `unsupported` | 0.8 | 3 | dynamic_section_excluded / unsupported_dynamic_eq |

#### 样例 Q1

```json
{
  "schema_version": "plugin-memory.entry/v1",
  "entry_type": "write_order_precondition",
  "entry_ref": "vit://memory.entry/memory.env:env1_2c7d9afdbd260293/t=all@memschema-sample-q1#sha256:1b2b8b781b58bbd5",
  "environment": {
    "machine_key": "m_local",
    "env_key": "env1_2c7d9afdbd260293"
  },
  "trust": "observed",
  "confidence": 0.85,
  "evidence_count": 3,
  "evidence_refs": [
    "workspace:artifacts/eq_compat_matrix/20260728_011728/capture/pro_q_3.json#smoke.result.writes#sha256=95E9CC67765EF2A2ABE4FDA591AB08780947B95978F9D1EBD50D4B3406FCC7C4",
    "repo:agent/internal/workflows/plugingrabber/eq_band_summary.go#L110-L111",
    "repo:docs/EQ_CONTROL_TOPOLOGY_MONOTONIC_REDESIGN_RESEARCH.md#section-8-monotonic-extension-contract"
  ],
  "context_tags": [
    "domain.eq",
    "product.fabfilter.pro-q-3",
    "tool.plugin_grabber.apply_eq_edits",
    "context.static-eq",
    "risk.write-order"
  ],
  "created_at": "2026-10-18T14:30:00+08:00",
  "updated_at": "2026-10-18T14:30:00+08:00",
  "expires_at": null,
  "plugin_identity": {
    "vendor": "FabFilter",
    "product": "Pro-Q 3",
    "plugin_identifier": "VST3-Pro-Q 3-a1aba170-ba5f7ab2",
    "plugin_format": "VST3",
    "plugin_version": "3.2.3.0"
  },
  "surface_fingerprint": {
    "tool": "plugin_grabber.inspect_eq",
    "generation": "eqt1_1c011654e3fffcaf6d968457",
    "param_surface_digest": "sha256:24db54ccf8dc4064",
    "captured_at": "2026-07-28T01:17:28+08:00"
  },
  "content": {
    "ordered_roles": [
      "shape",
      "frequency",
      "q",
      "gain",
      "activation"
    ],
    "condition": "free_floating（allocatable slot）EQ 的 upsert，经 plugin_grabber.apply_eq_edits 原子事务执行",
    "observed_signature": "真实 apply 的 writes/actual_readback 序列为 shape→frequency→q→gain→used(activation)，activation 位于末尾且整体 status=exact；参数面含 24 组 Used/Enabled allocatable slot",
    "authority_note": "advisory only：写入顺序的执行权威是 typed executor 契约（activation-last）与 live inspect 绑定；memory 不得覆盖事务计划，也不得据此改成其它写序"
  },
  "limitations": [
    "产品版本 3.2.3.0；非通用规则（其它主机/工具/版本可能不同）",
    "第三方调研报告存在相反写序主张（见 C4 冲突条目），本条与 C4 并存保留、不合并",
    "不得作为执行权威（见文档 §5）"
  ]
}
```

#### 样例 Q2

```json
{
  "schema_version": "plugin-memory.entry/v1",
  "entry_type": "gotcha",
  "entry_ref": "vit://memory.entry/memory.env:env1_2c7d9afdbd260293/t=all@memschema-sample-q2#sha256:70543fdf6490b68b",
  "environment": {
    "machine_key": "m_local",
    "env_key": "env1_2c7d9afdbd260293"
  },
  "trust": "observed",
  "confidence": 0.8,
  "evidence_count": 2,
  "evidence_refs": [
    "workspace:artifacts/eq_compat_matrix/20260728_011728/capture/pro_q_3.json#parameters.parameters#sha256=95E9CC67765EF2A2ABE4FDA591AB08780947B95978F9D1EBD50D4B3406FCC7C4",
    "repo:docs/COMPRESSOR_CONTROL_V1.md#apply-contract"
  ],
  "context_tags": [
    "domain.eq",
    "product.fabfilter.pro-q-3",
    "risk.non-finite-endpoint",
    "tool.plugin_grabber.apply_eq_edits"
  ],
  "created_at": "2026-10-18T14:30:00+08:00",
  "updated_at": "2026-10-18T14:30:00+08:00",
  "expires_at": null,
  "plugin_identity": {
    "vendor": "FabFilter",
    "product": "Pro-Q 3",
    "plugin_identifier": "VST3-Pro-Q 3-a1aba170-ba5f7ab2",
    "plugin_format": "VST3",
    "plugin_version": "3.2.3.0"
  },
  "surface_fingerprint": {
    "tool": "plugin_grabber.inspect_eq",
    "generation": "eqt1_1c011654e3fffcaf6d968457",
    "param_surface_digest": "sha256:24db54ccf8dc4064",
    "captured_at": "2026-07-28T01:17:28+08:00"
  },
  "content": {
    "symptom": "Dry Level / Wet Level 在 normalized=0 的显示文本为 `-INF dB`（非有限端点）；Band N Threshold 在 normalized=1 的显示文本为命名端点 `Auto`",
    "trigger_conditions": [
      "按显示最小/最大值反推 normalized↔physical 线性曲线",
      "把 -INF 或 Auto 当作连续数值端点参与插值"
    ],
    "observed_signature": "display_probe：Dry/Wet Level samples [0→-INF dB, .25→-12.0, .5→-6.0, .75→-2.5, 1→+0.0]；Band 1 Threshold samples [0→-90.0 dB, .25→-45.0, .5→-30.0, .75→-15.0, 1→Auto]",
    "impact": "有限目标值可能被错误解析到非有限/命名端点，或把端点状态当连续域，导致 typed 事务拒绝或语义误写",
    "safe_policy": [
      "一律以 live inspect 的 measured domain 与 display probe 为准",
      "有限写入目标必须落在有限数值区间；-INF/Auto 等只作为特殊状态处理",
      "写入后使用 typed readback（exact/quantized）校验，禁止自行反推曲线"
    ],
    "verification": "apply 前后 full preimage readback；遇到目标落在非有限端点时由 typed 层拒绝"
  },
  "limitations": [
    "端点语义是 Pro-Q 3 3.2.3.0 记录的值；其它版本可能不同，使用前重新 live inspect",
    "只描述显示端点现象，不构成任何 normalized↔physical 映射"
  ]
}
```

#### 样例 Q3

```json
{
  "schema_version": "plugin-memory.entry/v1",
  "entry_type": "semantic_hint",
  "entry_ref": "vit://memory.entry/memory.env:env1_2c7d9afdbd260293/t=all@memschema-sample-q3#sha256:62237ad81acb8f66",
  "environment": {
    "machine_key": "m_local",
    "env_key": "env1_2c7d9afdbd260293"
  },
  "trust": "observed",
  "confidence": 0.75,
  "evidence_count": 3,
  "evidence_refs": [
    "workspace:artifacts/eq_compat_matrix/20260728_011728/capture/pro_q_3.json#parameters.parameters#sha256=95E9CC67765EF2A2ABE4FDA591AB08780947B95978F9D1EBD50D4B3406FCC7C4",
    "repo:docs/EQ_CONTROL_TOPOLOGY_MONOTONIC_REDESIGN_RESEARCH.md#section-4-1-surfacescope",
    "repo:docs/EQ_AUXILIARY_EXCLUSION_FIX_VALIDATION.md#root-cause-and-fix"
  ],
  "context_tags": [
    "domain.eq",
    "product.fabfilter.pro-q-3",
    "context.static-eq",
    "context.ambiguity"
  ],
  "created_at": "2026-10-18T14:30:00+08:00",
  "updated_at": "2026-10-18T14:30:00+08:00",
  "expires_at": null,
  "plugin_identity": {
    "vendor": "FabFilter",
    "product": "Pro-Q 3",
    "plugin_identifier": "VST3-Pro-Q 3-a1aba170-ba5f7ab2",
    "plugin_format": "VST3",
    "plugin_version": "3.2.3.0"
  },
  "surface_fingerprint": {
    "tool": "plugin_grabber.inspect_eq",
    "generation": "eqt1_1c011654e3fffcaf6d968457",
    "param_surface_digest": "sha256:24db54ccf8dc4064",
    "captured_at": "2026-07-28T01:17:28+08:00"
  },
  "content": {
    "subject": {
      "control_name": "Band N Dynamic Range / Dynamics Enabled / Threshold / External Side Chain / Speakers / Ex Band N Speakers",
      "display_group": "Dynamics / Mix 等"
    },
    "candidates": [
      {
        "meaning": "静态 EQ core（Frequency/Gain/Q/Shape/Activation）的同频段辅助字段",
        "confidence": 0.8,
        "basis": "参数面为 24 组同构 Band N 字段；EQ 拓扑研究把 Dynamic/Threshold/External Side Chain 归入 auxiliary bindings，不参与静态 core 写入"
      },
      {
        "meaning": "Ex Band N Speakers 属 external side chain 扬声器配置，不是独立 band",
        "confidence": 0.85,
        "basis": "同 section 辅助字段不改变 main band 归属（EQ 拓扑研究 §4.1）"
      }
    ],
    "caveats": [
      "参数名含 Band/Threshold 不等于主链路控制；任何 role 绑定以 live inspect 结果为准",
      "不得把 Dynamic Range 当作静态 Gain 使用"
    ]
  },
  "limitations": [
    "解释性提示（weak hint 族），不是已验证 DSP 结论",
    "只提示命名歧义，不提供参数绑定或映射"
  ]
}
```

#### 样例 Q4

```json
{
  "schema_version": "plugin-memory.entry/v1",
  "entry_type": "gotcha",
  "entry_ref": "vit://memory.entry/memory.env:env1_2c7d9afdbd260293/t=all@memschema-sample-q4#sha256:fa1b84905d4db84c",
  "environment": {
    "machine_key": "m_local",
    "env_key": "env1_2c7d9afdbd260293"
  },
  "trust": "observed",
  "confidence": 0.75,
  "evidence_count": 3,
  "evidence_refs": [
    "workspace:artifacts/eq_compat_matrix/20260728_011728/capture/pro_q_3.json#parameters.parameters#sha256=95E9CC67765EF2A2ABE4FDA591AB08780947B95978F9D1EBD50D4B3406FCC7C4",
    "repo:docs/EQ_CONTROL_TOPOLOGY_MONOTONIC_REDESIGN_RESEARCH.md#section-4-2-eqsectionfacts",
    "repo:docs/COM_V1_TEST_CONTRACT.md#L132"
  ],
  "context_tags": [
    "domain.eq",
    "product.fabfilter.pro-q-3",
    "risk.channel-linkage",
    "context.static-eq"
  ],
  "created_at": "2026-10-18T14:30:00+08:00",
  "updated_at": "2026-10-18T14:30:00+08:00",
  "expires_at": null,
  "plugin_identity": {
    "vendor": "FabFilter",
    "product": "Pro-Q 3",
    "plugin_identifier": "VST3-Pro-Q 3-a1aba170-ba5f7ab2",
    "plugin_format": "VST3",
    "plugin_version": "3.2.3.0"
  },
  "surface_fingerprint": {
    "tool": "plugin_grabber.inspect_eq",
    "generation": "eqt1_1c011654e3fffcaf6d968457",
    "param_surface_digest": "sha256:24db54ccf8dc4064",
    "captured_at": "2026-07-28T01:17:28+08:00"
  },
  "content": {
    "symptom": "每个 band 带 Stereo Placement（enum Left/Right/Stereo/Mid/Side）、Speakers 与 Ex Band N Speakers 等通道相关字段；全局另有 Output Pan / Output Pan Mode",
    "trigger_conditions": [
      "把全局请求静默落成单通道写入",
      "未证明 channel_contract 就写通道相关或 stereo placement 字段"
    ],
    "observed_signature": "参数面 24×Stereo Placement/Speakers/Ex Band N Speakers；EQ 拓扑研究要求先证明 channel_contract ∈ {shared, linked mirror, unsafe asymmetric} 再决定通道写入方式",
    "impact": "通道语义可能被误改（只改单通道、立体声联动失配），或把全局请求错误扁平化",
    "safe_policy": [
      "全局请求优先 shared binding；通道特定写入仅在 live inspect 证明 linked/shared 后执行",
      "不得把多路径/通道联动当作单一标量控制",
      "写后校验所有被绑定通道的读回"
    ],
    "verification": "typed readback + full preimage restore；channel_link 类事实不得由音频推断（COM 测试契约既有规则）"
  },
  "limitations": [
    "通道契约按实例/版本由 live inspect 给出；本条只是风险提示，不代替 inspect",
    "不同插件族的 channel_contract 不同，禁止跨插件套用"
  ]
}
```

#### 样例 Q5

```json
{
  "schema_version": "plugin-memory.entry/v1",
  "entry_type": "unsupported_boundary",
  "entry_ref": "vit://memory.entry/memory.env:env1_2c7d9afdbd260293/t=all@memschema-sample-q5#sha256:4c8ac8e4605973cb",
  "environment": {
    "machine_key": "m_local",
    "env_key": "env1_2c7d9afdbd260293"
  },
  "trust": "unsupported",
  "confidence": 0.8,
  "evidence_count": 3,
  "evidence_refs": [
    "repo:docs/EQ_CONTROL_TOPOLOGY_MONOTONIC_REDESIGN_RESEARCH.md#section-6-dynamic-eq-boundary",
    "repo:docs/MULTIBAND_DYNAMICS_CONTROL_V1.md#L64-L74",
    "repo:docs/EQ_AUXILIARY_EXCLUSION_FIX_VALIDATION.md#section-conclusion"
  ],
  "context_tags": [
    "domain.eq",
    "product.fabfilter.pro-q-3",
    "risk.capability-boundary",
    "tool.plugin_grabber.apply_eq_edits"
  ],
  "created_at": "2026-10-18T14:30:00+08:00",
  "updated_at": "2026-10-18T14:30:00+08:00",
  "expires_at": null,
  "plugin_identity": {
    "vendor": "FabFilter",
    "product": "Pro-Q 3",
    "plugin_identifier": "VST3-Pro-Q 3-a1aba170-ba5f7ab2",
    "plugin_format": "VST3",
    "plugin_version": "3.2.3.0"
  },
  "surface_fingerprint": {
    "tool": "plugin_grabber.inspect_eq",
    "generation": "eqt1_1c011654e3fffcaf6d968457",
    "param_surface_digest": "sha256:24db54ccf8dc4064",
    "captured_at": "2026-07-28T01:17:28+08:00"
  },
  "content": {
    "attempted_tool": "plugin_grabber.apply_eq_edits（typed 静态 EQ core）",
    "rejection_code": "dynamic_section_excluded / unsupported_dynamic_eq",
    "observed_scope": "Dynamic Range / Threshold / Attack / Release / Ratio / External Side Chain 等动态/侧链行为不进入 typed 静态 EQ 控制范围；辅助字段本身仍可读",
    "family_label_vs_topology": "Pro-Q 3 产品上可做动态段，但 typed 静态 core 只控制静态 Frequency/Gain/Q/Shape/Activation；产品名/家族名不是控制证据",
    "retry_policy": "动态行为请求按 capability boundary 返回或换用支持该行为的处理器；不得用静态 Gain 冒充动态行为，不得让模型自行发明 fallback"
  },
  "limitations": [
    "边界范围随 typed 工具能力演进；本条不是永久结论",
    "不得据此放宽 PCA 或 typed 准入门槛"
  ]
}
```

### Pro-C 2（FabFilter Pro-C 2，VST3；identity 子字段未记录 → [待用户确认]）

| seq | entry_type | trust | confidence | evidence_count | 一句话 |
|---|---|---|---|---|---|
| c1 | `gotcha` | `observed` | 0.8 | 1 | Pro-C 2 的 Mix 实测物理域为 0–200%，不是通用的 0–100% |
| c2 | `gotcha` | `observed` | 0.8 | 3 | Wet Gain / Dry Gain 的实测域带非有限端点；显示文本可出现 `-INF dB` |
| c3 | `ambiguity_resolution` | `observed` | 0.85 | 2 | `Mix` / `Wet Gain` / `Dry Gain` 是否表示压缩强度？ |
| c4 | `write_order_precondition` | `agent-inferred` | 0.35 | 2 | 第三方 REAPER MCP 报告声称：Pro-Q 3 / Pro-C 2 需先写 Band 的 Used/Enable |
| c5 | `user_approved_setting` | `user-confirmed` | 0.9 | 2 | user-approved setting（threshold=-12 dB / ratio=4:1） |

#### 样例 C1

```json
{
  "schema_version": "plugin-memory.entry/v1",
  "entry_type": "gotcha",
  "entry_ref": "vit://memory.entry/memory.env:env1_e2daa9acfddc933d/t=all@memschema-sample-c1#sha256:c8614d12fb89425d",
  "environment": {
    "machine_key": "m_local",
    "env_key": "env1_e2daa9acfddc933d"
  },
  "trust": "observed",
  "confidence": 0.8,
  "evidence_count": 1,
  "evidence_refs": [
    "repo:docs/COMPRESSOR_REFERENCE_PRO_C_2.md#measured-reference-surface"
  ],
  "context_tags": [
    "domain.compressor",
    "product.fabfilter.pro-c-2",
    "risk.parameter-domain",
    "context.parallel"
  ],
  "created_at": "2026-10-18T14:30:00+08:00",
  "updated_at": "2026-10-18T14:30:00+08:00",
  "expires_at": "2027-01-16T14:30:00+08:00",
  "plugin_identity": {
    "vendor": "FabFilter",
    "product": "Pro-C 2",
    "plugin_identifier": null,
    "plugin_format": "VST3",
    "plugin_version": null
  },
  "surface_fingerprint": {
    "tool": "plugin_grabber.inspect_compressor",
    "generation": "c2t1_6b96b6b6969df9a2ddbf22af",
    "param_surface_digest": "sha256:-",
    "captured_at": null
  },
  "content": {
    "symptom": "Pro-C 2 的 Mix 实测物理域为 0–200%，不是通用的 0–100%",
    "trigger_conditions": [
      "按通用假设把 Mix 百分比钳制到 100%",
      "把 Mix 当作压缩强度控制"
    ],
    "observed_signature": "COMPRESSOR_REFERENCE_PRO_C_2 §Measured Reference Surface：`mix` 0 to 200% on this product; never assume 0 to 100%",
    "impact": "parallel balance 设置被错误截断或误读为压缩量",
    "safe_policy": [
      "百分比目标必须落在 live inspect 测得的物理域内",
      "禁止假设 0–100%；域检查以 inspect/apply 的 typed 校验为准"
    ],
    "verification": "typed readback（exact/quantized）+ measured domain 校验"
  },
  "limitations": [
    "[待用户确认] 该测量来自 tracked 参考文档；原始测量工件路径（VitApp/Workspace/Artifacts/smoke/product_path_20260804_094717/…/pro_c_2.json）在 2026-10-18 核查时不在盘，使用前应重新 live inspect",
    "TTL 为 90 天建议值（expires_at 示例），实际过期执行归 D14"
  ]
}
```

#### 样例 C2

```json
{
  "schema_version": "plugin-memory.entry/v1",
  "entry_type": "gotcha",
  "entry_ref": "vit://memory.entry/memory.env:env1_e2daa9acfddc933d/t=all@memschema-sample-c2#sha256:30b3b107caf76c9b",
  "environment": {
    "machine_key": "m_local",
    "env_key": "env1_e2daa9acfddc933d"
  },
  "trust": "observed",
  "confidence": 0.8,
  "evidence_count": 3,
  "evidence_refs": [
    "repo:docs/COMPRESSOR_REFERENCE_PRO_C_2.md#measured-reference-surface",
    "repo:docs/COMPRESSOR_CONTROL_V1.md#apply-contract",
    "repo:docs/PLUGIN_ALLIANCE_COMPRESSOR_BLIND_V1_ROUND1.md#failure-taxonomy"
  ],
  "context_tags": [
    "domain.compressor",
    "product.fabfilter.pro-c-2",
    "risk.non-finite-endpoint"
  ],
  "created_at": "2026-10-18T14:30:00+08:00",
  "updated_at": "2026-10-18T14:30:00+08:00",
  "expires_at": null,
  "plugin_identity": {
    "vendor": "FabFilter",
    "product": "Pro-C 2",
    "plugin_identifier": null,
    "plugin_format": "VST3",
    "plugin_version": null
  },
  "surface_fingerprint": {
    "tool": "plugin_grabber.inspect_compressor",
    "generation": "c2t1_6b96b6b6969df9a2ddbf22af",
    "param_surface_digest": "sha256:-",
    "captured_at": null
  },
  "content": {
    "symptom": "Wet Gain / Dry Gain 的实测域带非有限端点；显示文本可出现 `-INF dB`",
    "trigger_conditions": [
      "按显示 min/max 线性反推 normalized 曲线",
      "把 -INF 当作连续数值域端点"
    ],
    "observed_signature": "COMPRESSOR_REFERENCE_PRO_C_2：wet_gain/dry_gain `-18 to +36 dB, with a non-finite endpoint`；同类 -INF 命中已在 blind round 记录为拒绝（Zip threshold 目标 -60 dB → 显示 -inf dB，事务被拒并原子回滚）",
    "impact": "有限目标解析到非有限端点 → typed 层拒绝，或读到与预期不一致的端点状态",
    "safe_policy": [
      "有限目标不得映射到非有限端点（typed 契约既有规则）",
      "端点只按命名状态处理；曲线反推以 measured anchors 为准"
    ],
    "verification": "apply rejected 时确认无残留状态；成功时 exact/quantized readback"
  },
  "limitations": [
    "[待用户确认] wet/dry 域名来自 tracked 参考文档，原始测量工件不在盘；使用前重新 live inspect",
    "不要据此条为其它插件推导非有限端点行为"
  ]
}
```

#### 样例 C3

```json
{
  "schema_version": "plugin-memory.entry/v1",
  "entry_type": "ambiguity_resolution",
  "entry_ref": "vit://memory.entry/memory.env:env1_e2daa9acfddc933d/t=all@memschema-sample-c3#sha256:355ee13d8821d27e",
  "environment": {
    "machine_key": "m_local",
    "env_key": "env1_e2daa9acfddc933d"
  },
  "trust": "observed",
  "confidence": 0.85,
  "evidence_count": 2,
  "evidence_refs": [
    "repo:docs/COMPRESSOR_REFERENCE_PRO_C_2.md#canonical-semantic-axes",
    "repo:docs/COMPRESSOR_REFERENCE_PRO_C_2.md#directional-semantics"
  ],
  "context_tags": [
    "domain.compressor",
    "product.fabfilter.pro-c-2",
    "context.parallel",
    "context.ambiguity"
  ],
  "created_at": "2026-10-18T14:30:00+08:00",
  "updated_at": "2026-10-18T14:30:00+08:00",
  "expires_at": null,
  "plugin_identity": {
    "vendor": "FabFilter",
    "product": "Pro-C 2",
    "plugin_identifier": null,
    "plugin_format": "VST3",
    "plugin_version": null
  },
  "surface_fingerprint": {
    "tool": "plugin_grabber.inspect_compressor",
    "generation": "c2t1_6b96b6b6969df9a2ddbf22af",
    "param_surface_digest": "sha256:-",
    "captured_at": null
  },
  "content": {
    "question": "`Mix` / `Wet Gain` / `Dry Gain` 是否表示压缩强度？",
    "resolution": "否。它们属 output/parallel balance 轴：降低 mix 削弱可听湿声贡献，但不削弱压缩器内部 gain reduction；output gain 不是 compression intensity",
    "rejected_alternatives": [
      "把 mix 当作 compression intensity",
      "用 output gain 代替压缩量（output gain 只是电平补偿）"
    ],
    "basis": "COMPRESSOR_REFERENCE §Canonical Semantic Axes 与 §Directional Semantics：parallel balance / output normalization 是独立轴"
  },
  "limitations": [
    "语义澄清，不是参数映射；实际角色绑定以 live inspect 为准",
    "不同压缩器家族的 output 段结构不同，本条不跨插件套用",
    "[待用户确认] Pro-C 2 的 identity/version/identifier 未在证据工件中记录（原始测量工件不在盘）"
  ]
}
```

#### 样例 C4

```json
{
  "schema_version": "plugin-memory.entry/v1",
  "entry_type": "write_order_precondition",
  "entry_ref": "vit://memory.entry/memory.env:env1_e2daa9acfddc933d/t=all@memschema-sample-c4#sha256:6dd6390b1304e9da",
  "environment": {
    "machine_key": "m_local",
    "env_key": "env1_e2daa9acfddc933d"
  },
  "trust": "agent-inferred",
  "confidence": 0.35,
  "evidence_count": 2,
  "evidence_refs": [
    "workspace:coord/reports/2026-10-18-reaper-mcp-plugin-param-research.md#section-5-3#sha256=3717DF1738D19516E36C5A277A0079A65A9462034C9BE103E4D2003ADDDD4494",
    "repo:docs/EQ_CONTROL_TOPOLOGY_MONOTONIC_REDESIGN_RESEARCH.md#section-8-monotonic-extension-contract"
  ],
  "context_tags": [
    "domain.eq",
    "domain.compressor",
    "product.fabfilter.pro-q-3",
    "product.fabfilter.pro-c-2",
    "risk.write-order",
    "source.external-report"
  ],
  "created_at": "2026-10-18T14:30:00+08:00",
  "updated_at": "2026-10-18T14:30:00+08:00",
  "expires_at": null,
  "plugin_identity": {
    "vendor": "FabFilter",
    "product": "Pro-C 2",
    "plugin_identifier": null,
    "plugin_format": "VST3",
    "plugin_version": null
  },
  "surface_fingerprint": {
    "tool": "plugin_grabber.inspect_compressor",
    "generation": "c2t1_6b96b6b6969df9a2ddbf22af",
    "param_surface_digest": "sha256:-",
    "captured_at": null
  },
  "content": {
    "ordered_roles": [
      "used/enable activation",
      "frequency/gain/q"
    ],
    "condition": "第三方 REAPER MCP 报告声称：Pro-Q 3 / Pro-C 2 需先写 Band 的 Used/Enabled 再写 Frequency/Gain，否则参数写了没声音，更严重会触发崩溃",
    "observed_signature": "外部报告（REAPER/host 工具链，未在 Vit-DAW 栈复现）；Vit 侧真实 apply 记录相反（activation-last，见 Q1）",
    "authority_note": "冲突保留、不合并：本条 trust=agent-inferred、仅作规划期风险提示；写入顺序权威=typed executor 契约 + live inspect；任何执行不得据此改变写序，不得据此绕过事务校验"
  },
  "limitations": [
    "[待用户确认] 外部报告未经 Vit 栈复现；主机/工具/版本差异可能解释冲突",
    "不得把本条升级为执行权威或通用写序规则"
  ]
}
```

#### 样例 C5

```json
{
  "schema_version": "plugin-memory.entry/v1",
  "entry_type": "user_approved_setting",
  "entry_ref": "vit://memory.entry/memory.env:env1_e2daa9acfddc933d/t=all@memschema-sample-c5#sha256:8c636a7b056df5f2",
  "environment": {
    "machine_key": "m_local",
    "env_key": "env1_e2daa9acfddc933d"
  },
  "trust": "user-confirmed",
  "confidence": 0.9,
  "evidence_count": 2,
  "evidence_refs": [
    "repo:docs/COMPRESSOR_CONTROL_V1.md#vit-natural-language-validation",
    "repo:docs/COMPRESSOR_REFERENCE_PRO_C_2.md#worked-projection-examples"
  ],
  "context_tags": [
    "domain.compressor",
    "product.fabfilter.pro-c-2",
    "context.user-approved",
    "tool.plugin_grabber.apply_compressor_controls"
  ],
  "created_at": "2026-10-18T14:30:00+08:00",
  "updated_at": "2026-10-18T14:30:00+08:00",
  "expires_at": null,
  "plugin_identity": {
    "vendor": "FabFilter",
    "product": "Pro-C 2",
    "plugin_identifier": null,
    "plugin_format": "VST3",
    "plugin_version": null
  },
  "surface_fingerprint": {
    "tool": "plugin_grabber.inspect_compressor",
    "generation": "c2t1_6b96b6b6969df9a2ddbf22af",
    "param_surface_digest": "sha256:-",
    "captured_at": null
  },
  "content": {
    "setting": {
      "roles": [
        {
          "role": "threshold",
          "target_kind": "physical_absolute",
          "value": -12.0,
          "unit": "dB"
        },
        {
          "role": "ratio",
          "target_kind": "physical_absolute",
          "value": 4.0,
          "unit": ":1"
        }
      ],
      "context": "Vit natural-language compressor validation：用户明确要求 Threshold=-12 dB、Ratio=4:1；disposable Pro-C 2 实例"
    },
    "approval": {
      "event_ref": "conversation:compressor_nl_final_20260803_2350",
      "scope": "单次会话/单实例的显式设置请求，不跨工程、不成为默认",
      "approved_at": "2026-08-03T23:50:00+08:00"
    }
  },
  "limitations": [
    "只记录客观可复核的设置事实与用户显式请求，不承载偏好、不做默认值、不自动复用",
    "用户偏好类信息必须走 D13 质询确权 + D14 用户偏好类，不在本 schema",
    "[待用户确认] Pro-C 2 的 identity/version/identifier 未在证据工件中记录（原始测量工件不在盘）"
  ]
}
```

### Valhalla 同族 reverb（ValhallaSupermassive，VST3；version 未记录）

| seq | entry_type | trust | confidence | evidence_count | 一句话 |
|---|---|---|---|---|---|
| v1 | `unsupported_boundary` | `unsupported` | 0.9 | 3 | unsupported_delay |
| v2 | `ambiguity_resolution` | `observed` | 0.85 | 3 | ValhallaSupermassive 应按 reverb 还是 delay 控制？ |
| v3 | `semantic_hint` | `agent-inferred` | 0.45 | 2 | Mix（其余面：DelaySync / DelayNote / Delay_Ms / DelayWarp / Feedb |

#### 样例 V1

```json
{
  "schema_version": "plugin-memory.entry/v1",
  "entry_type": "unsupported_boundary",
  "entry_ref": "vit://memory.entry/memory.env:env1_218efe9c1a66754a/t=all@memschema-sample-v1#sha256:ac65edc7e2208c92",
  "environment": {
    "machine_key": "m_local",
    "env_key": "env1_218efe9c1a66754a"
  },
  "trust": "unsupported",
  "confidence": 0.9,
  "evidence_count": 3,
  "evidence_refs": [
    "run:coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/pgtdc_20261016_205639/valhallasupermassive.inspects.json#inspects.reverb",
    "run:coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/pgtdc_20261016_204214/valhallasupermassive.inspects.json#inspects.reverb",
    "run:coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/README.md#L58"
  ],
  "context_tags": [
    "domain.reverb",
    "domain.delay",
    "product.valhalla.supermassive",
    "risk.capability-boundary"
  ],
  "created_at": "2026-10-18T14:30:00+08:00",
  "updated_at": "2026-10-18T14:30:00+08:00",
  "expires_at": null,
  "plugin_identity": {
    "vendor": "valhalla",
    "product": "ValhallaSupermassive",
    "plugin_identifier": "VST3-ValhallaSupermassive-5cffe1ee-7e8fe5d5",
    "plugin_format": "VST3",
    "plugin_version": null
  },
  "surface_fingerprint": {
    "tool": "plugin_grabber.inspect_delay",
    "generation": "dl1t1_9fc850d507fac64a9601736e",
    "param_surface_digest": "sha256:86160d7b9e3855f8",
    "captured_at": "2026-10-16T20:56:39+08:00"
  },
  "content": {
    "attempted_tool": "plugin_grabber.inspect_reverb",
    "rejection_code": "unsupported_delay",
    "observed_scope": "reverb 家族插件无 provable reverb stage/decay 控制（无衰减词证据），inspect_reverb 拒绝；同一实例 inspect_delay 可解析为 feedback_delay",
    "family_label_vs_topology": "family=reverb 来自清单元数据；家族/产品名不是拓扑证据，reverb 词表拒绝不等于产品不可用",
    "retry_policy": "不得据家族名强行按 reverb 控制；按 capability boundary 返回或走手动指引/替换处理器；不得发明 fallback、不得默认近似写"
  },
  "limitations": [
    "两条独立 run 均得到同一拒绝码（run2/run3）；插件版本未在工件中记录",
    "边界随词表/识别器演进；本条不是永久结论",
    "[待用户确认] ValhallaSupermassive 的 version 未在工件中记录"
  ]
}
```

#### 样例 V2

```json
{
  "schema_version": "plugin-memory.entry/v1",
  "entry_type": "ambiguity_resolution",
  "entry_ref": "vit://memory.entry/memory.env:env1_218efe9c1a66754a/t=all@memschema-sample-v2#sha256:77ebee516db5f793",
  "environment": {
    "machine_key": "m_local",
    "env_key": "env1_218efe9c1a66754a"
  },
  "trust": "observed",
  "confidence": 0.85,
  "evidence_count": 3,
  "evidence_refs": [
    "run:coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/pgtdc_20261016_205639/valhallasupermassive.inspects.json#inspects.delay",
    "run:coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/pgtdc_20261016_204214/valhallasupermassive.inspects.json#inspects.delay",
    "run:coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/pgtdc_20261016_205639/matrix_summary.json#matrix[valhallasupermassive]"
  ],
  "context_tags": [
    "domain.reverb",
    "domain.delay",
    "product.valhalla.supermassive",
    "context.ambiguity"
  ],
  "created_at": "2026-10-18T14:30:00+08:00",
  "updated_at": "2026-10-18T14:30:00+08:00",
  "expires_at": null,
  "plugin_identity": {
    "vendor": "valhalla",
    "product": "ValhallaSupermassive",
    "plugin_identifier": "VST3-ValhallaSupermassive-5cffe1ee-7e8fe5d5",
    "plugin_format": "VST3",
    "plugin_version": null
  },
  "surface_fingerprint": {
    "tool": "plugin_grabber.inspect_delay",
    "generation": "dl1t1_9fc850d507fac64a9601736e",
    "param_surface_digest": "sha256:86160d7b9e3855f8",
    "captured_at": "2026-10-16T20:56:39+08:00"
  },
  "content": {
    "question": "ValhallaSupermassive 应按 reverb 还是 delay 控制？",
    "resolution": "以 live inspect 的分类为准：inspect_reverb 拒绝 unsupported_delay；inspect_delay 返回 feedback_delay（timing/output/feedback 段）。家族标签 reverb 不是控制证据",
    "rejected_alternatives": [
      "按 vendor family=reverb 猜测 reverb 参数",
      "按产品名套用 reverb 词表/写序"
    ],
    "basis": "两次独立 run 的 inspects 工件：reverb 面拒绝、delay 面 resolved=true、classification=feedback_delay、confidence 0.96"
  },
  "limitations": [
    "两条 run 的 generation 不同（dl1t1_f8c727… / dl1t1_9fc850…），说明 generation 是实例级、不可跨会话缓存复用",
    "分类结果随识别器版本变化，使用前重新 inspect",
    "[待用户确认] ValhallaSupermassive 的 version 未在工件中记录"
  ]
}
```

#### 样例 V3

```json
{
  "schema_version": "plugin-memory.entry/v1",
  "entry_type": "semantic_hint",
  "entry_ref": "vit://memory.entry/memory.env:env1_218efe9c1a66754a/t=all@memschema-sample-v3#sha256:33d82309f88e7640",
  "environment": {
    "machine_key": "m_local",
    "env_key": "env1_218efe9c1a66754a"
  },
  "trust": "agent-inferred",
  "confidence": 0.45,
  "evidence_count": 2,
  "evidence_refs": [
    "run:coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/pgtdc_20261016_205639/surfaces_compact.json#valhallasupermassive",
    "run:coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/pgtdc_20261016_205639/valhallasupermassive.inspects.json#inspects.delay"
  ],
  "context_tags": [
    "domain.reverb",
    "domain.delay",
    "product.valhalla.supermassive",
    "context.ambiguity"
  ],
  "created_at": "2026-10-18T14:30:00+08:00",
  "updated_at": "2026-10-18T14:30:00+08:00",
  "expires_at": null,
  "plugin_identity": {
    "vendor": "valhalla",
    "product": "ValhallaSupermassive",
    "plugin_identifier": "VST3-ValhallaSupermassive-5cffe1ee-7e8fe5d5",
    "plugin_format": "VST3",
    "plugin_version": null
  },
  "surface_fingerprint": {
    "tool": "plugin_grabber.inspect_delay",
    "generation": "dl1t1_9fc850d507fac64a9601736e",
    "param_surface_digest": "sha256:86160d7b9e3855f8",
    "captured_at": "2026-10-16T20:56:39+08:00"
  },
  "content": {
    "subject": {
      "control_name": "Mix（其余面：DelaySync / DelayNote / Delay_Ms / DelayWarp / Feedback / Density / Width / LowCut / HighCut / ModRate / ModDepth / Mode / Reserved1-4）",
      "display_group": "output"
    },
    "candidates": [
      {
        "meaning": "inspect 的 output 段声明 dry/mix/wet 三个角色，但实测参数面只录到 1 个 Mix（display 50.0 %）",
        "confidence": 0.5,
        "basis": "surfaces_compact 仅 1 个 mix 参数；inspects 工件声明 output=[dry,mix,wet]，两者绑定关系未在工件中给出"
      },
      {
        "meaning": "不得假定 dry/wet 有独立可写 control_ref；角色绑定以 live inspect 结果为准",
        "confidence": 0.8,
        "basis": "同上；dry/wet 可能是 inspect 派生角色而非独立参数"
      }
    ],
    "caveats": [
      "[待用户确认] 需要一次 live inspect 验证 dry/wet 绑定",
      "本条仅为 agent 推断（trust=agent-inferred），不得作为参数映射或写计划来源"
    ]
  },
  "limitations": [
    "[待用户确认] 工件未包含角色→参数绑定明细，推断待 live inspect 复验",
    "不得据此构造 normalized↔physical 映射"
  ]
}
```

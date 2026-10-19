# Capability Boundary Remediation v1（capability_boundary.v1）

状态：现行（CAP-BOUNDARY-1 产物，2026-10-19）。
实现：`agent/internal/capabilityboundary/`（schema + 确定性 resolver）；
接线：`agent/internal/chat/goalrunner_chat.go`（free-state `capability_blocked` 路径）、
`agent/internal/chat/c2_dynamic_control_runtime.go`（C2 受治理边界响应）；
prompt 契约声明：`agent/internal/agentloop/ccb_model_prompt.go`（`capabilityBoundaryRemediationPromptRule`）。

## 1. 目的与定位

能力边界拒绝（自由态准入门拒绝 / 模型终局能力边界申报 / C2 受治理边界）此前只有 L0 呈现：
`StopReason`、收据、`mutation_performed=false` 与一句话话术——没有下一步选项，而 prompt 又
禁止模型自行发明 fallback。本 schema 让**运行时**在拒绝响应上结构化披露一组**受治理的**
后续选项；判定与门法本身零改动。

三条不变量：

1. **零 LLM**：resolver 是纯函数（`capabilityboundary.Resolve`），输入为接线层用既有只读查询
   装配的事实快照；不调用任何模型，不做任何 I/O 写。
2. **零新增写路径**：v1 的五种 action kind 都不写参数、不做近似写（L5 明确不做）。加载类
   选项只是"指向既有确认流程的入口"，不绕过、不替代任何门。
3. **不伪造**：某类选项没有事实时不编造，`next_actions` 缺该类并 在
   `unavailable_reasons` 披露原因；零事实时 `next_actions=[]` 且原因齐全。

## 2. Schema

载荷嵌入 `ChatResponse.WorkflowData["capability_boundary"]`，旧客户端忽略该键即可（向后兼容，
不改变既有字段与停止语义）。

```json
{
  "schema_version": "capability_boundary.v1",
  "code": "admission_gate_failed | no_pca_admitted_dynamic_plugin_candidate | ...",
  "class": "free_state | c2_dynamic_control",
  "family": "broadband_compressor | ...（可空）",
  "track_id": "可空",
  "reason": "确定性一句话原因",
  "evidence_refs": ["..."],
  "blocked_by": ["G6_target_evidence", "..."],
  "mutation_performed": false,
  "next_actions": [ ... ],
  "unavailable_reasons": ["no_pca_admitted_same_family_candidate", "..."]
}
```

每个 `next_actions[]` 项：

| 字段 | 语义 |
|---|---|
| `kind` | 五选一闭集：`alternate_processor` \| `native_processor` \| `manual_guidance` \| `open_plugin_ui` \| `certification_entry` |
| `key` / `rank` | 稳定键与 1..N 排序位 |
| `requires_confirmation` | 仅 `alternate_processor`（走既有加载确认）与 `certification_entry`（consent token）为 true |
| `risk_level` | low / medium |
| `expected_effect` / `tradeoff` | 确定性模板文案 |
| `evidence` | 能力/覆盖证据：attestation_id、subject_key、binary_fingerprint、family、coverage_axes、coverage_level 等 |
| `target` | 可执行所需的精确标识：已准入候选 identifier+plugin_path / native 族 / 插件实例 track_id+plugin_id / 认证 endpoint+consent_token |
| `manual_params` | 仅 `manual_guidance`：live context pack 参数行（role/param_id/range/current_value/expected） |

## 3. 确定性排序（固定）

```
同族已准入 > 相邻族已准入 > native/stock > 手动指引 > 认证入口
```

- 同族/相邻族候选在排序内按（族秩, identifier 小写, plugin_path 小写）稳定排序，去重后
  截断至最多 5 项（`MaxAlternateActions`）。
- 相邻族表是包内静态声明的声学域邻接（如 broadband_compressor ↔ limiter/multiband/gate/
  transient；reverb ↔ delay），见 `capabilityboundary.AdjacentFamilies`。
- 只读族（`spectral_dynamics`、`clipper`）没有控制器词表：不提供替代与 native 选项，
  仅保留手动指引/打开 UI（及存在 runner 时的认证入口——这两个族没有 runner，故实际不提供）。

## 4. 事实来源（只读复用，不改门）

| 事实 | 既有查询 |
|---|---|
| 同族已准入候选 | `localPluginRecommendationCandidates`（catalog 扫描）→ 有 intent 覆盖时 `attestedPluginRecommendationCandidates`（PCA+coverage），否则 `admittedPluginRecommendationCandidates`（族级 PCA） |
| 相邻族已准入候选 | 同上，族级 PCA 准入 |
| 手动指引用已加载实例 | `semanticTreatmentInstances`（合格面 OwnedParameterIDs）+ 首实例一次 `get_plugin_parameters` 只读补当前值/范围 |
| native/stock | 产品事实：DAW 手动/原生 rack 路径不经 agent PCA 装载门（`docs/PCA_PLUGIN_LOAD_GATE_V1.md`） |
| 认证入口 | `/agent/processor-certification/candidates?family=X` + consent token（L4 只给入口） |

查询失败按"该类事实缺失"降级（resolver 披露），不在边界面上报错。

## 5. 接线

- **free-state**：`capabilityBlockedBoundaryResponse`（经 `settledTurnFaceResponse` 统一出口，
  plain 与闭包激活两分支同面）在既有 `capability_blocked`/收据/`mutation_performed=false`
  之上追加 `capability_boundary`；族取自 decision 的 `semantic_processor_intent.family` 或
  `processor_type`（经既有 `processorAttestationFamily` 映射）。
- **C2**：`c2BoundaryResponse`（c2_dynamic_control_runtime.go）所有出参都携带该字段；无族
  上下文的基础边界（stale_project_cut 等基础设施类）= 诚实空载荷（`unavailable_reasons`
  齐全）；带族/覆盖/轨上下文的调用点（c2DynamicPluginLoadSelection 的
  unsupported_dynamic_family / no_pca_admitted_dynamic_plugin_candidate 等）传
  `s.c2BoundaryRemediation(...)` 富化载荷。
- **prompt**：`capabilityBoundaryRemediationPromptRule` 声明模型只获准解释/让用户选择已给
  选项，不得发明 fallback、新 kind、近似写或自排序——no-fallback 法不变。

## 6. 非目标（v1 边界）

- 不放宽/绕过 PCA 或 typed 准入；不改装载/写入判定；不改 PCA 判定、typed executor、内核。
- 不做 L4 认证流程内部改造（只给入口/链接）。
- **不做 L5 近似写**（preimage/readback/restore 的实验性近似写另卡另裁定，默认关闭）；任何
  分支都不出现近似写动作。
- 不接 hook memory（MEM-SCHEMA-1 / D14）；不让模型生成或发明 next_action 类型。
- 不新增 catalog 工具；手动指引只描述与打开 UI（`open_plugin_ui` 既有工具），不写参数。

## 7. 测试面

- resolver table-driven 单测：`agent/internal/capabilityboundary/resolver_test.go`
  （无 PCA→同族替代+认证入口；无替代→native/手动；只读族→无写动作；零候选→空+reason；
  族未解析→不伪造；截断确定性；纯函数确定性）。
- 响应面测试：`agent/internal/chat/capability_boundary_response_test.go`
  （free-state 两形态 + C2 基础/富化两形态；`mutation_performed=false`；既有收据/StopReason
  面零改动）。
- prompt 契约测试：`agent/internal/agentloop/ccb_model_prompt_capability_boundary_test.go`。

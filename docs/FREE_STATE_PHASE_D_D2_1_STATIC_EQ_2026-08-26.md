# D2-1：bounded static_eq 第二动作维度 — 晚窗执行记录（S1 + S2a）

Date: 2026-08-26 晚窗。Status: 进行中（S1、S2a 已提交；S2b/S3 待续）。

## 提交

- `5cb3588` D2-1-S1：experiment 准入门按动作域参数化。
- `83d68a1` D2-1-S2a：StaticEQ VSP 执行端口（叶子模块，未接线）。

## S1 设计要点

- `experiment/d1s1_domains.go`：`d1s1Domains` 域表。track_gain 行为逐字节不变；static_eq 新条目强度同级收紧：单 band、gain ±2 dB、frequency 20–20000 Hz、q 0.1–18、max_action_attempts=1、experiment_budget=1、observation-bound track target。
- `ValidateD1S1` 共享不变量（budget/track target/attempts）与每域参数校验分离；未知域报错并列出准入域清单。
- **chat 生产入口暂仍只放行 track_gain**（`free_state_experiment_runtime.go`）：在 static_eq 执行链落地前放行 admission 会产生"能观察不能执行/结算"的死态；错误信息显式标注 "pending D2-1-S2"。这是临时更严，不是判据放宽。
- 遗留影响：audition legacy 测试原来用 `static_eq/bounded_eq_adjust` 充当"非 D1 域"标记，现改为显式 `legacy_mix/legacy_adjust`。

## S2a 设计要点（关键决策）

- **不变量保持**：一个受治理 Action 可包含两个内核命令（instantiate_plugin 按需 + set_plugin_param）+ readback（get_plugin_parameters），共享单幂等键、产出单 receipt——对 experiment 层仍是恰好一次前向变更。
- CAS：set_plugin_param 带 `base_revision`；epoch 漂移 / revision 未前进 / readback 不匹配 → `applied_unreconciled`。
- preflight 对已存在 plugin 读取 before 值（指纹/回滚记录）；reconcile 仅凭持久化 plugin_id 可验证。
- 防御细节：`fmt.Sprint(nil)=="<nil>"` 会让"删除的 key"伪装成身份串，`actionArgText` 统一按缺失处理。

## 内核命令面勘察（S2b 直接可用，无需改 C++）

- `instantiate_plugin`：track_id + plugin_path|plugin_identifier → plugin_id（PluginRackControlService.cpp:2741 起）。
- `set_plugin_param`：plugin_id + param_id + value|normalized_value|value_text（:1816 起）。
- `get_plugin_parameters`：track_id + plugin_id（:2874 起）。
- 插件链完整性：scan_plugins / plugin_list_available / plugin_search / delete_plugin 均在。
- 注意：p01/p02 封存 fixture 无预置插件（盲测契约禁止预选）；static_eq 烟测需走 instantiate 路径或非封存测试工程。

## S2b 待办（明早窗口）

1. chat `free_state_d1_runtime.go` 对照接线：static_eq 计划构建（fingerprint 形如 `track:X:eq:<plugin>:<param>:<before>`）、`staticBalanceCapabilityID` 旁路或新 capability、verifier 复用 HarnessAcoustic 模式。
2. 放行 chat 入口域检查（删除临时 track_gain-only 限制），D1S1AdmittedDomains() 驱动。
3. G 门 frequency 维度视图映射复核（frontier 候选已支持 mix.frequency_relationship，预期改动小）。
4. S3：prompt 域说明按准入域表表述；p01/p02 烟测 + 三 disposition 探针复用。

## 验收状态

- 全量 `go test ./...` 通过（两次；期间触发两个已知 Windows 时序型 flaky——`TestProductionFreeStateRunnerObservationsSurviveDurableSlices` 与 `TestProcessorCertificationStartAcceptsBroadbandCompressorCapability` TempDir 清理竞态，单跑均稳定通过，见 docs/TEMPDIR_FLAKY_EVIDENCE_2026-08-26.md，与本轮改动无关联面）。
- S1/S2a 均为无行为变化或未接线改动，按仓库验收纪律**不要求**真实栈烟测；S2b 接线后必须过烟测才可继续放行。

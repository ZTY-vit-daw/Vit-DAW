# D2-1：bounded static_eq 第二动作维度 — 晚窗执行记录（S1 + S2a）

Date: 2026-08-26 晚窗。Status: S2b 已提交并首审通过；S3 部分收口（回归烟测绿，static_eq 实栈触发未达成，D2-1 保持开放）。

## 提交

- `5cb3588` D2-1-S1：experiment 准入门按动作域参数化。
- `83d68a1` D2-1-S2a：StaticEQ VSP 执行端口（叶子模块，未接线）。
- `7d2e2ff` D2-1-S2b：static_eq 接入 chat D1-S1 执行链（放行域检查、计划/执行/期刊变体、verifier 复用、候选分发）。
- `0b9334f` D2-1-S3：prompt 域表述表驱动 + journal 修复 + 烟测域参数化；p01/p02 回归 PASS，static_eq 实栈触发未达成（开放项见 S3 状态第 5 条）。

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

## S2b 状态（2026-08-26 提交 7d2e2ff）

1. ✅ chat 对照接线：`d1StaticEQPlan` / `executeD1StaticEQ` 镜像 track_gain；BeforeFingerprint `track:<id>:eq:<param_id>:pending`（端口 Preflight 读 before 值）；capability 采用设计记录的"旁路"分支——D1 执行路径按显式端口路由，ActionSet/Proposal 用 `static_mix.static_eq.v0`（审计元数据，不注册 registry）；verifier 复用 HarnessAcoustic fresh 观察模式（StaticEQ + VerifyStaticEQ，D2-1 措辞），readback 断言收据驱动。
2. ✅ 放行 chat 入口域检查：`D1S1DomainSpecFor` 域表驱动（domain+kind 同时匹配），track_gain 形状逐字节不变。
3. ⏸ G 门 frequency 维度视图映射复核：现状 `mix.frequency_relationship` 已产生 frontier 候选，未发现缺口；本轮未改 audio_closure_controller.go（留给 S3/GLM 复核）。
4. ⏳ S3：prompt 域说明按准入域表表述（ccb_model_prompt.go）；p01/p02 烟测 + 三 disposition 探针复用。**未烟测不可关账。**

## S3 状态（2026-08-26 晚窗）

1. ✅ prompt 域表述表驱动：`experiment/d1s1_domains.go` 增加 `PromptParameterHint`（每域 parameter_bounds 形状与绝对界）与 `D1S1DomainSpecs()` 只读导出；`ccb_model_prompt.go` 的 needs_experiment 域规则由域表生成（含"Every listed domain is admitted and executable end-to-end by the experiment runtime in this phase"显式声明），needs_experiment 示例 JSON 改为双域 union 语法（沿用本 prompt 既有 union 惯例），pattern guidance 的频域条目直指 static_eq。
2. ✅ S2b 首审遗留修复：static_eq journal Command 的 plugin_id 取不到时补记 `plugin_identifier`（实例 ID 继续由回执 result payload 承载，测试断言补齐）。
3. ✅ 烟测 runner 域参数化：`ADMITTED_DOMAIN_KINDS` 镜像域表（准入判定始终在 agent 侧）；`find_d1_loop` 接受全部准入域并按 `updated_at` 取最新快照（修复一个潜在 harness bug：同一响应内嵌多份同 ID loop 投影时"last wins"会选中陈旧副本，曾致一次中性 p01 误报失败，报告 20260826_204256）；`--prompt-flavor`（neutral/frequency，case 无关，不破坏盲测契约）与 `--expect-domain`（any/track_gain/static_eq，回归钉子）两个旋钮，报告记录 `selected_domains`。
4. ✅ 真实栈回归（新 agent 构建，改动全量生效）：p01 中性 PASS（20260826_211629，track_gain）+ p02 PASS（20260826_211929，track_gain，readback -1.0dB）——S3 改动未破坏既有链路。
5. ❌ **static_eq 实栈触发未达成（D2-1 开放项）**：frequency flavor + `-ExpectDomain static_eq` 共 3 次（新 agent：20260826_210107/210831/211231），模型均完成正确观察序列（project.structure → mix.frequency_relationship → track.timbre_frequency）后返回 `capability_blocked`，自拟 rejection code `unsupported_action_domain`（代码库中不存在该串），声称"运行时不支持 static_eq"——与 prompt 规则/示例/引导直接矛盾。另有 2 次旧 agent 对照（20260826_205240/205733）同样 blocked，说明该拒答跨 prompt 版本不变。runner 侧 NOT_EXERCISED 机制（exit 3 + selected_domains 证据）按设计工作。
   - 已排查无果：代码内无该 rejection code 来源；ledger/frontier/snapshot 无域能力披露；部署二进制确认含新规则文本；无 prompt 缓存路径。
   - 后续选项（待 GLM 裁定，D2-1 关账前置）：a) 用 `VIT_AGENT_LLM_TELEMETRY_PATH` 抓实弹 system prompt 逐字核对；b) `-AdmissionOnly` frequency 探针隔离提案/执行两段；c) few-shot 增补一条 static_eq 决策示例；d) 若确认是模型合规性问题，重新审视 D2-1 烟测口径（自主选域 vs 运营者提示域）。

## 验收状态

- 全量 `go test ./...` 通过（两次；期间触发两个已知 Windows 时序型 flaky——`TestProductionFreeStateRunnerObservationsSurviveDurableSlices` 与 `TestProcessorCertificationStartAcceptsBroadbandCompressorCapability` TempDir 清理竞态，单跑均稳定通过，见 docs/TEMPDIR_FLAKY_EVIDENCE_2026-08-26.md，与本轮改动无关联面）。
- S1/S2a 均为无行为变化或未接线改动，按仓库验收纪律**不要求**真实栈烟测；S2b 接线后必须过烟测才可继续放行。

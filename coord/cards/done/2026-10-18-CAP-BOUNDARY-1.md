# CAP-BOUNDARY-1：能力边界结构化 remediation（L1–L3，确定性生成、无 LLM 判定）

- 发卡：PC 调研/决策会话（DSH lead）/ 2026-10-18（依据=用户 2026-10-18「可以出卡，放入池中」+ 调研结论报告 §5.4）
- 派发确认：**已确认（用户 2026-10-18 明确授权出卡）**；授权记录见 [decisions/2026-10-18-plugin-memory-and-boundary-cards.md](../../decisions/2026-10-18-plugin-memory-and-boundary-cards.md)
- 验收负责人：主管决策流（PC）
- 池序：**136（2026-10-18 晚窗 GLM 主管裁定：与 54 FS-BLOCKED-SURFACE-CLOSURE-1 同触 goalrunner_chat.go，两卡串行=54 先、136 后——54 为 10-10 老卡先行解阻，136 2-3 天大卡随其后）**；P2 / 2–3 天 / **无硬依赖**；**与池序 54 `FS-BLOCKED-SURFACE-CLOSURE-1` 同触 `agent/internal/chat/goalrunner_chat.go`，领取前须由主管排序，两卡串行**；与 MEM-SCHEMA-1 零文件域交集；**晚窗参考序①（2026-10-19 /morning 标注：54 验收合入后晚窗 /night 按池序自取本卡，今日早窗未排）**；同日池深补充（2026-10-19 /morning）：53 HARNESS-STRUCT-NORMALIZE-1 + 51 FS-RECEIPT-REVISION-1 激活为 PC 可执行余卡（两卡均已确认派发；51 依赖 FS-LEDGER-PERSIST-1 已 done 满足，与 54 同包不同文件可并行）
- 目标仓库=D:\Vit_DAW；模型分级：L2（agent 多包面）
- 归属：PC 决策侧；执行方须为 PC 执行流

## 现状与依据（勿重复取证）

- 当前拒绝只有 L0：`freeStateCapabilityBlockedReply`（"未通过准入门 + 没有修改/不会自动重试"）与 `c2BoundaryResponse`（一句话边界）；有 `StopReason`/`mutation_performed=false`，但没有下一步选项。锚点：`agent/internal/chat/goalrunner_chat.go:2104-2133`、`agent/internal/chat/c2_dynamic_control_runtime.go:931-934`。
- 现行 prompt **禁止模型自行发明 fallback**（unsupported → `capability_blocked`）：`agent/internal/agentloop/ccb_model_prompt.go`（"instead of inventing a fallback"）。本卡方向=给受治理的选项，不是放宽判定。
- 可复用零件（只读引用，不扩权威）：
  - `agent/internal/capabilityruntime/readiness.go:17-70`：`BlockedBy` + `Remediation`，确定性、绝不调用 LLM；
  - `agent/internal/chat/plugin_recommendation.go:297-302`：recommended/alternative + tradeoff/confidence；
  - `agent/internal/chat/semantic_treatment_strategy.go:695-711`：`choice_required`、`next_planner=capability_boundary|native`；
  - `agent/internal/chat/processor_certification_entry.go:20-34`：产品内 PCA 认证流程（临时轨/apply/readback/restore/consent token）；
  - `docs/PCA_PLUGIN_LOAD_GATE_V1.md:21-34`：只拦 agent 主动 load、手动加载路径不变、拒绝有稳定 `pca_load_gate:` reason。
- 背景调研：`coord/reports/2026-10-18-reaper-mcp-plugin-param-research.md` §5.4（PC 工作区报告，未入库；本卡面已含所需结论）。

## 目标（v1 = L1–L3；L4 只给入口，L5 不做）

1. **响应 schema**：新增 `capability_boundary.v1`（嵌入 `ChatResponse.WorkflowData`，不新增可写工具）：`boundary{code,class,family,reason,evidence_refs,blocked_by,mutation_performed=false}` + `next_actions[]`，每项含 `kind`（`alternate_processor | native_processor | manual_guidance | open_plugin_ui | certification_entry`）、`requires_confirmation`、`risk_level`、能力/覆盖证据、`expected_effect`/`tradeoff`、以及可执行所需的精确标识（已准入候选 key / native 族 / 插件实例与参数面引用）。v1 **不得包含近似写动作**。
2. **确定性 resolver**：从 readiness/PCA/topology 边界 + 已安装清单 + 已准入候选，确定性生成 `next_actions`；**零 LLM 调用**。排序固定：同族已准入 > 相邻族已准入 > native/stock > 手动指引 > 认证入口；无候选时返回空列表 + `unavailable_reasons`，不得伪造。替代候选必须先过 PCA/coverage 校验（只复用现有查询，不改门）。
3. **接线**：free-state `capability_blocked` 路径与 C2 边界响应都携带该字段；保留原 `StopReason`、收据、`mutation_performed=false` 和现有话术语义；模型只获准"解释/选择已给选项"，不得新增 fallback 类型。
4. **手动指引**：由 live context pack 生成（插件/轨道/参数 role 或 param_id/范围/当前值/预期），通过现有 `open_plugin_ui` 类能力打开 UI；**不写参数**。
5. **向后兼容**：旧客户端忽略新字段；不改变既有字段/停止语义；不新增 catalog 工具（如确需只读工具，先上交主管）。

## 文件域

- 新增：`agent/internal/capabilityboundary/`（schema + resolver + 测试）；
- 修改接线与测试：`agent/internal/chat/goalrunner_chat.go`、`agent/internal/chat/c2_dynamic_control_runtime.go`、对应 `_test.go`；如 prompt 契约需要声明，改 `agent/internal/agentloop/` 中对应 prompt 文件及其测试；
- 文档：新增 `docs/CAPABILITY_BOUNDARY_REMEDIATION_V1.md`（schema + 语义 + 非目标/边界）；
- 只读复用：`agent/internal/capabilityruntime/`、`processorattestation/`、`processorcertification*`；**不改 PCA 判定、typed executor、kernel**。越域即停。

## 非目标

- 不放宽/绕过 PCA 或 typed 准入；不改装载/写入判定；
- 不做 L4 认证流程内部改造（只给入口/链接）；
- **不做 L5 近似写**（preimage/readback/restore 的实验性近似写另卡另裁定，默认关闭）；
- 不接 hook memory（MEM-SCHEMA-1 / D14）；
- 不让模型生成或发明 next_action 类型。

## 并行与资源

与 doing 卡零文件域交集；与 todo 54 同触 `goalrunner_chat.go`，须主管排序后串行领取。真栈按 AGENTS §5：领取/启栈前核对 `coord/resources/PC-RUNTIME-STACK.md`，用独立 worktree。

## 验收标准

- 单测（resolver table-driven，至少覆盖）：无 PCA → 同族替代 + 认证入口；无替代 → native/手动；只读族 → 无写动作；零候选 → 空列表 + reason；**任何分支都不出现近似写**；resolver 无 LLM 依赖。
- 响应测试：free-state 与 C2 两路径均含 `capability_boundary.v1`，`mutation_performed=false`，原字段与既有测试零改动全绿；旧客户端兼容。
- `go build ./...` + `go test ./... -count=1` 0 FAIL + gofmt 净。
- **真栈（AGENTS §5）**：`scripts/dev_agent_smoke.ps1 -StartKernel -RestartAgent ...` 既有场景 exit 0；并至少一条 PCA 拒绝/能力边界场景断言 `next_actions` 由确定性 resolver 生成且未发生 mutation。若现有 ps1 无对应场景，按同一模式扩展现有脚本（不另起机制）；回执记录 run ID、工件、HEAD/工作树状态与开关。
- 文档存在且与实现一致；回执显式声明端测覆盖边界。

## 停止条件

- 需要修改 PCA 判定、typed executor 或内核才能生成替代 → 停止并提交缺口锚点（不自行扩域）；
- 替代候选无法从现有 API 确定性获得（如需新内核命令）→ 停止上交；
- 与池序 54 的文件域冲突未经主管排序 → 不领取；
- 要求把 L5 近似写并入本卡 → 停止（另卡另裁定）；
- 真栈环境中断 → 按 AGENTS §8 记录为环境中断，不把下游断言失败当功能结论。

- 领取：（2026-10-19 18:03 +0800 / origin/main=f32fae4d / GLM-5.3 L2 PC ZCode 执行会话（用户口令「开工 136」指定领取）/ 分支=port/cap-boundary-1 / worktree=D:/Vit_DAW_wt_capbnd1 / 领取提交=本提交；领取时核对：卡在 todo 派发已确认（用户 2026-10-18 授权）、依赖无硬依赖（串行前置 54 已验收合入 32810859）、doing 仅 GEN-SUNO-WEB-DESIGN-1（设计件零生产码）文件域零冲突、行号锚已按 54 回执重锚（settledTurnFaceResponse :2105-2132、capabilityBlockedBoundaryResponse 现 :2134 起、C2 锚点 c2_dynamic_control_runtime.go:931-934 待现场核对）；真栈腿用前登记 PC-RUNTIME-STACK.md（当前空闲，今晚用户 K3.1 手测优先，遇占用让行））
- 回执：（**done 2026-10-19 18:5x +0800，执行侧自验完成待主管验收**）实现 commit=**d42deed1** @ port/cap-boundary-1（已推 origin；工件追加 commit=2d209ab9；领取=5176272f）。**实现面**：①新增 `agent/internal/capabilityboundary/`（capability_boundary.v1 schema：boundary{code,class,family,reason,evidence_refs,blocked_by,mutation_performed=false}+next_actions[] 五 kind 闭集 {alternate_processor|native_processor|manual_guidance|open_plugin_ui|certification_entry} 各含 requires_confirmation/risk_level/evidence/target 精确标识/manual_params+unavailable_reasons；resolver=纯函数零 LLM 零 I/O，固定排序 同族已准入>相邻族已准入>native>手动>认证，稳定排序+去重+截 5，只读族（spectral_dynamics/clipper）无替代与 native，空候选诚实 unavailable_reasons 不伪造，任何分支无近似写）；②接线：`capabilityBlockedBoundaryResponse`（经 settledTurnFaceResponse 统一出口，plain+闭包两分支同面，签名未动既有测试零改）追加 WorkflowData["capability_boundary"]，族取自 decision semantic_processor_intent.family 或 processor_type（经既有 processorAttestationFamily）；`c2BoundaryResponse`（c2_dynamic_control_runtime.go:931 区）variadic 扩展——所有 C2 边界出参都携带（无族上下文=诚实基础载荷），c2DynamicPluginLoadSelection 三处带族调用点传 `s.c2BoundaryRemediation` 富化；事实装配只读复用既有查询（localPluginRecommendationCandidates→admitted/attestedPluginRecommendationCandidates、semanticTreatmentInstances+一次 get_plugin_parameters 只读补当前值、认证入口端点+consent token、native=PCA 文档产品事实）不改 PCA/typed executor/kernel 门；③prompt 契约：ccb_model_prompt.go 新增 capabilityBoundaryRemediationPromptRule（turn directives 动态区，模型只获准解释/选择已给选项，不得发明 fallback/新 kind/近似写，no-fallback 法不变）。**测试**：resolver table-driven 7 测（TestResolveNoPCABlockAdmitsSameFamilyAlternateAndCertificationEntry / NoAlternatesFallsBackToNativeAndManualGuidance / ReadOnlyFamilyEmitsNoWriteActions / ZeroCandidatesReturnsEmptyListWithReasons / UnresolvedFamilyDisclosesInsteadOfFabricating / CapsAlternatesDeterministically / IsDeterministic）+响应面 4 测（capability_boundary_response_test.go：free-state 无族诚实披露/带族 native+认证入口序、C2 基础/富化面）+prompt 1 测（ccb_model_prompt_capability_boundary_test.go）。**既有测试零改动**：free_state_capability_blocked_surface_test.go+closure_surface_test.go 等对 origin/main 零 diff 亲核（diff --stat 仅 10 文件 1504+/7- 全在卡域）。**验收命令**：cd agent 后 `go build ./...` OK；`go test ./... -count=1` 退出码 0 零 FAIL（96 包 ok 含新包）；gofmt——本卡 delta 净（LF 归一口径复核，本机 gofmt go1.26 对 CRLF 检出仓会全量误报，以归一化内容核）：goalrunner_chat.go 全净含**顺手修复 :181 基线偏差**（`}	// AGENT-SENDROUTE…`→`} //`，130 遗留 54 回执登记项，域内裁量回执注明）。**真栈（AGENTS §5）**：占用登记 f4067e4b（预检六端口/栈进程=0，用户 Godot 编辑器 7712 非栈面未动）→ run=**dev_agent_smoke_20261019_184026 exit 0**（`-StartKernel -RestartAgent -NoChatSmoke -NoStripSilenceSmoke -CapabilityBoundarySmoke -KernelExe 主检出 staging`；本卡按同一模式扩展 dev_agent_smoke.ps1 新增 -CapabilityBoundarySmoke 腿不另起机制）——三断言全绿：①C2 invalid_referral 边界面携 capability_boundary.v1（class/mutation_performed=false 双位/空 next_actions+unavailable_reasons 披露）；②双独立会话驱动载荷逐字节一致=resolver 确定性；③未授权 plugin.load_to_rack PCA 装载门稳定 pca_load_gate: 拒绝+工程修订 1→1 零 mutation（拒绝先于内核派发）。被测 agent=worktree 构建 SHA256=8CC481F8…（HEAD d42deed1）+内核只读 stock B6565DCF…；§8：LLM=0 全确定性腿。偏差如实记录：首轮命令行 bash 吞反斜杠 -RepoRoot 无效未起栈零副作用（K3.1 同款），正斜杠重跑一次过。18:5x 拆栈复核六端口=0/栈进程=0，释放登记随本提交。工件 coord/runs/CAP-BOUNDARY-1/capbnd_smoke_20261019_184026/console_stdout.txt（随 2d209ab9 入库）。**端测覆盖边界声明**：本卡零渲染面/零用户旅程改动（无 webui/Godot 文件）；agent HTTP 响应面真栈断言如上；**富化 next_actions 非空集合**（同族/相邻族替代、手动指引用 live 参数行）在真栈上未驱动到——其可达路径需 LLM 自由态轮或 C2 发现链（概率面，AGENTS §8），确定性覆盖由 resolver table-driven 单测承担（含 PCA 查询装配的只读复用面）；端侧已证=两路径载荷在场+确定性+PCA 门拒绝零 mutation。停止条件未触发（未改任何门；L5 未并入；替代候选全部来自现有查询）。git status=clean（实现/工件均已提交）。
- 验收：（裁定文件 / 验收 commit）

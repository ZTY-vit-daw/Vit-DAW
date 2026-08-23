# Project-aware Capability Runtime v1：实施状态

更新时间：2026-07-13

## 当前阶段

当前处于 `P5：B2/B3 v1 final cutover + one-way legacy migration`。

- 新 B2/B3 Session 只能由 `capability_runtime_v1` 拥有；历史 rollout=`off` 也不能复活 legacy owner。
- Chat 的 legacy pending maps、文本确认入口和 interaction 执行入口已删除。
- AgentLoop 的 B2/B3 preflight、decision schema、candidate builder 与对应旧测试已删除。
- 旧工程 JSON 仍可读取，但只能单向迁移：仍活跃的旧 pending 被标成 `rejected`，已完成的 `verified/committed/failed/...` 历史终态原样保留；确认 interaction 被移除，旧 authority 字段不再写回。
- Session owner 在 v1 生命周期内仍保持 immutable；执行只来自 FrozenPlan + Authorization。

## 已落地的控制平面

- `PlanningSession`：聚合目标、能力调用、固定 `engine_owner`、Proposal、FrozenPlan、Authorization、Execution 与状态迁移。
- `ProjectCut`：持久化工程 UUID、boot-scoped epoch、base revision、依赖/目标指纹、artifact lineage 和契约版本。
- `ContextRequest / ContextBundle / ContextEnvelope`：由 CCB 负责按 Manifest 获取权威观察，再按固定预算准入模型上下文；全量派生模型默认只按引用提供。
- `CapabilityDefinition / Registry`：能力 ID、版本、所需 Context Manifest 与协议显式注册。
- `FrozenPlan`：精确保存 Proposal、ActionSet、ProjectCut、ContextBundle ID 和前序 observation ID；hash-only Proposal 不可执行。
- `Authorization`：绑定 Proposal ID/revision、ActionSet hash、scope、授权 turn 与 sequence；授权后 FrozenPlan 不可替换。
- `ExecutionRecord`：逐动作持久化稳定 request ID、Receipt、Project History 引用和完整 VerificationResult。
- `VerificationResult`：分离 structural、fresh acoustic/MOM、user acceptance 与 evidence refs；用户审美默认 `unknown`。

## B2/B3 产品路径

两条能力都使用同一生命周期：

```text
Chat
→ owner resolution / PlanningSession
→ CCB-owned project snapshot + mix.observe
→ full model/readiness/solver
→ bounded ContextBundle / ContextEnvelope
→ immutable FrozenPlan
→ version-bound Authorization
→ Project History baseline
→ VSP CAS/idempotent Execution Coordinator
→ structural state readback
→ fresh full-project mix.observe/MOM verification
```

B2 仅编译 `track_gain_adjust`；B3 仅编译 `track_pan_set`。前端传入的 `project_state`、`mix_observation`、MOM 或 audio-analysis status 不会成为执行权威数据。

## Proposal 对话授权更新

B2/B3 已接入 `vit.proposal_presentation.v1` 与 `vit.approval_decision.v1`：

- Chat 回复先展示分析结论、推荐理由、Readiness/证据覆盖、分组修改和逐轨 before/target/delta；
- `proposal_approval` 使用紧凑 inline card，不再占据 Composer 的大块 modal 区域；细节默认折叠，按钮只发送协议层的 approve/cancel；
- 用户可以直接提问、排除目标、缩小范围、调整数值或选择候选方案；提问和歧义不会授权，候选切换会要求完整 re-plan；
- 每次 revision 生成新的 Proposal ID/revision/ActionSet hash，旧 authorization 与旧按钮绑定自动失效；
- 强类型 `ProposalPresentation`、`ApprovalDecision`、Presentation 和 Decision 均随 Planning Session 持久化并可在重启后恢复；
- 前端显示是投影层，Execution Coordinator 仍只接受 FrozenPlan + exact Authorization，不接受 UI payload 作为执行真相。

2026-07-13 产品路径验收证据：

- B3 对话 Proposal：`artifacts/proposal_interaction_live_acceptance/b3_conversation_prepared_v1/summary.json`；r1→r2→r3、提问只读、旧按钮拒绝、歧义只读、自然语言精确授权均通过，2 个 Receipt，structural/acoustic=`pass`；
- B2 对话 Proposal：`artifacts/proposal_interaction_live_acceptance/b2_conversation_prepared_v1/summary.json`；同一协议检查全部通过，2 个 Receipt，structural/acoustic=`pass`；
- 自动化入口：`scripts/capability_proposal_interaction_live_smoke.py`；只使用公开 Chat、interaction 和 invoke API；
- UI 视觉证据：`agent/output/playwright/capability-proposal-card-collapsed.png`、`capability-proposal-card-expanded.png` 和 `capability-proposal-inline-page.png`；DOM 中 Proposal 位于消息 article，Composer 仍是独立输入区；
- 默认 FileStore 重启后成功从 waiting revision 1 继续生成 revision 2；前端按钮携带精确 Proposal binding，Agent 重启后仅在持久化 Session 的 ID/revision/ActionSet hash/ProjectCut 全部一致时恢复 interaction，实测可安全取消且工程未变化；终态 B2/B3 Session 均持久化 Presentation、ApprovalDecision、Authorization、Receipt、Verification 与 Project History 引用。

## Kernel 与恢复

- VSP handshake 协商 `command.base_revision_cas=true` 与 `command.idempotency=true` 后，Cut 才能升级为 `strong`。
- mutation 在 apply point 校验 `base_revision`，stale Cut 返回 `stale_project_cut`。
- Kernel 以 `session_id + request_id` 缓存 Receipt；相同请求重放不会二次调用 legacy mutation handler。
- Coordinator 在 mutation 前持久化 prepared execution，并在每个 Action 后持久化 Receipt。
- partial failure/cancel 不会继续 dispatch；in-flight Receipt 先持久化。
- 重启恢复按真实工程状态 reconcile，未应用动作变成 `reconcile_incomplete`，不会盲目 retry。
- 真实子进程退出测试覆盖 prepared-before-apply、apply-before-receipt、partial batch、receipts-before-verify 和 verify-before-finalize。

## 持久化

- 默认 Store：`%AppData%\Vit\Agent\orchestration_v1.json`。
- `VIT_ORCHESTRATION_STORE_PATH=memory` 仅用于非执行开发场景，没有 mutation authority。
- FileStore 在整个 read/CAS/write 操作期间持有 OS 文件锁，并使用同目录临时文件替换，支持多个 VitAgent 进程共享用户级 Store。
- 跨进程测试由两个真实子进程并发写入 80 个 Session，最终无丢失。

## 已验证

定向包与 Chat/恢复路径通过：

```text
go test ./internal/orchestration
go test ./internal/orchestrationruntime
go test ./internal/capabilityadapters ./internal/capabilitycontext
go test ./internal/executionruntime ./internal/executionports ./internal/executionverifiers
go test ./internal/projectcut ./internal/protocolvalue ./internal/kernel
go test ./internal/chat ./internal/harness ./internal/staticbalance ./internal/panlayout
```

真实 Release VitApp 现场证据：

- VSP CAS/idempotency/stale-cut：`artifacts/orchestration_v1_live_smoke/20260713_132453`。
- 默认 policy route 下连续完成 B3 与 B2：`artifacts/orchestration_v1_live_smoke/v1_policy_both_20260713_140002_summary.json`。
- 物理退役后以 rollout=`off`、legacy creation=false 再次连续完成 B3 与 B2：`artifacts/orchestration_v1_live_smoke/v1_post_retirement_20260713_143333/evidence/summary.json`，SHA-256 `9693300BB88467293EBED7F55EFD99E4772BB6595858F57A4A22B2F2BC34ACC1`。
- 两条能力均持久化 Receipt 与 History baseline，structural/acoustic 为 `pass`，fresh MOM 为 `ready`，user acceptance 为 `unknown`。
- 最终 authority report：2 个 terminal v1 Session、`safe_to_disable_legacy_code=true`；进程内已不存在 legacy B2/B3 authority inventory。
- 所有 smoke 均使用隔离工程；dirty `VitApp/Workspace/default_project.xml` 前后 SHA-256 未变化。

## 尚未宣称的内容

- `go test ./...` 仍只在迁移前的 `internal/agentloop` 测试域失败，因此不能宣称整个仓库全绿。
- legacy B2/B3 可执行 authority 已物理删除；保留的仅是不会产生执行入口的 JSON migration reader。
- v1 当前完成的是 B2/B3 第一批 strangler seams，不代表所有未来编曲、编辑和混音能力都已迁移；后续能力应复用同一协议，而不是复制新的 pending authority。

# Free-State Phase D D1-S1 Closeout

Status: historical closeout record, accepted for the D1-S1 execution-chain scope. Date: 2026-08-25.

D1-S1 的范围：公共用例 `spv1_p02`（注入低频失衡的 stems 工程）上，从开放意图 `检查一下当前工程有什么问题？` 出发的完整执行链——自主诊断 → 有界改进提案（track_gain）→ 用户确认 → 受控 Apply → 精确 readback → after-revision 新鲜 post-action 观察 → 声学 materiality 与目标响应评估 → 人工 A/B 试听边界建立并保持。本收尾记录该链路已达成的验收状态、交付的修复层与明确的未竟事项。

## Acceptance evidence

两次连续真实栈 PASS（AGENTS.md 验收门槛：真实三件套 + 烟测脚本退出码 0）：

- `artifacts/free_state_d1_s1/20260825_214525/d1_smoke_report.json`（exit 0）
- `artifacts/free_state_d1_s1/20260825_215219/d1_smoke_report.json`（exit 0）

终轮（215219）validation 摘要：

- `status=pass`，`public_case_id=spv1_p02`；
- revision `50 → 51`，`forward_mutation_count=1`（唯一前向变更）；
- receipt 带 `transaction_id` / `idempotency_key` / `readback_verified=true`，`actual_readback_db=-0.80`；
- round 内恰好 1 条 `post_action=true` 观察（after revision）；
- `materiality=material`、`target_response=directional / human_audition_ready`、round decision `user_judgment_pending` 入账；
- `audition_session_id` 就绪、before/after render 就绪；
- `human_confirmed=false`——运行时建立并保持人工判断边界，未伪造人工确认。

loop 终态 `terminal_causes` 为 5× `experiment round is waiting for the human judgment boundary`：即收尾守卫的预期停点，不再复活、不再重复观察。

回归基线：`go test ./... -count=1` 83 包 ok；AGENTS.md 五条健康检查全过；webui 与 HEAD 内容一致未动。

## What D1 closes

- 自主执行链全程生产可达：维度闭合驱动的 FS4→FS5→FS6→FS7 推进、G1–G7 准入、提案确认门、受控 Apply、readback、post-action 观察与评估报告（materiality / target response / round decision）。
- 人工判断边界（ambiguous human A/B）的建立、持久化与保持：应答前 loop 不复活、round 内 post-action 观察幂等。
- 每一环的确定性守卫都有回归测试锁定（见下节提交）。

## Delivered fix layers（提交与根因链）

| Commit | 内容 |
|---|---|
| `e9f7ddb` | CCB 维度映射 + 建设性 limitation + 模式识别指引 |
| `54e9436` | 决策门：improvement contract 拒 diagnostic_complete；blocked 终局前必须有本轮新鲜观察；FS8 准入评估报告（跳过 G1–G7 重审）；评估形状免复述 proposal；judgment 边界 final gate |
| `3fd69fb` | chat 侧：调度器 reload/persist 竞态守卫（Fix A）；round 记录持久化；交互边界 park + interaction_id 生命周期；experiment 报告按任意决策形状入账；judgment 边界防复活 |
| `56f9f59` | experiment：D1-S1 round 内第二条 post-action 观察拒绝 |
| `8bda5a8` | 交接与测试文档 |
| `f43fb58` | `.gitattributes` 换行规范（防幻影改动） |

根因链与逐层证据：`docs/FREE_STATE_PHASE_D_D1_FIX_A_ROUNDBOUNDARY_HANDOFF_2026-08-25.md`、`docs/FREE_STATE_PHASE_D_D1_POSTACTION_CHAIN_FIXES_2026-08-25.md`、`docs/FREE_STATE_PHASE_D_D1_DIMENSION_MAPPING_FIX_2026-08-25.md`。

## Explicit non-goals / known remaining gaps

1. **真实人工听感裁决未自动化**（设计如此）：`human_confirmed=false`。运行时停在试听边界；真实 retain/rollback 由人在产品内完成，属 D1 settlement 的产品侧体验，不由烟测伪造。
2. **taskstate 语义迁移缺口**：PASS run 日志含 `[audition] canonical human judgment transition rejected: semantic transition human_judgment_requested is not allowed from improvement_proposal`（Warn）。audition session/render 就绪、验收判据不受影响；taskstate 迁移表与实验流的语义对齐留待后续任务。
3. **烟测展示字段**：`validate_d1` 报告的 `post_action_observation_id=null`（脚本读 `row["id"]` 而观察行为 `observation_id`），纯展示性，不影响 pass 判定。
4. **NOT_EXERCISED 率**：模型轨迹随机性下 4 轮中 2 轮未自主选 track_gain（exit 3，运行时按非失败处理）。维度映射提升了假设形成率但无保证；后续若需提高命中率应从模型行为侧（prompt/路由）继续，不得从运行时侧放宽判据。
5. **环境噪音**：`TestProcessorCertificationStartAcceptsBroadbandCompressorCapability` 在全量套件偶发 Windows TempDir 清理竞争（单跑稳定通过）；`.git/index.lock` 陈旧锁偶现（确认无 git 进程后可删）。

D2/D3 未开始（沿用 D1-S1 交接文档的边界：settlement 体验闭环前不开下一维度）。

## Reproduction

```powershell
cd D:\Vit_DAW\agent
go test ./... -count=1        # 83 包全绿
cd D:\Vit_DAW
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_free_state_d1_smoke.ps1 `
  -RepoRoot D:\Vit_DAW -PublicCaseId spv1_p02 -SkipBuild
```

退出码 0 = PASS（目标态）；3 = NOT_EXERCISED（模型轨迹随机性，重跑即可）；1 = FAIL（读最新 `artifacts/free_state_d1_s1/<stamp>/d1_smoke_report.json` 与落盘 state 定位）。烟测只使用临时工程副本，不改用户原始工程。

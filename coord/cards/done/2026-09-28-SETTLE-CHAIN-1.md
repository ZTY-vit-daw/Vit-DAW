# SETTLE-CHAIN-1：free_state_d1 真栈 settle 链断裂取证+修复（AGENTS §5 烟测门恢复）

- 优先级 / 预估 / 依赖：P1 / 取证 0.3 天+修复视根因 / DOSE-AUDIBLE-1 验收上交（rulings/2026-09-28-DOSE-AUDIBLE-1-pass.md §阻断二）；**该卡 exit 0 同时回补 DOSE-AUDIBLE-1 的端测门**
- 模型分级：L2 / GLM 亲自（settle 链牵动自由态核心语义与 orchestration，两形态根因未定）
- 背景（已实证，勿重跑既有取证）：
  - 形态①（主树，含 DOSE 改动）：apply→readback 全绿（四旗标 true）后 **acoustic materiality record 不产生**→experiment 停 running→continuation 32-36 耗尽；同断点两次=§8 止损已触发。run=artifacts/free_state_d1_s1/20260928_120230、_120725。
  - 形态②（基线，旧代码）：`D1 receipt requires distinct before/after revisions`，applied=false。run 取证=coord/runs/DOSE-AUDIBLE-1/baseline-20260928_121206/。
  - 09-12 后 main 无该烟测 pass 记录；disclosure×断言冲突已由 DOSE-AUDIBLE-1 脚本域修复（values_for_key_outside_disclosure），本卡只余 settle 链。
- 目标：
  1. **取证先行**：两形态是否同源（materiality 产生条件链 vs receipt revision 区分链）；最小复现（可用 -SkipBuild 复用 11:48 二进制锚定行为面）；代码锚点（settle/materiality/target_response 记录的产生方与消费方——experiment settle 状态机、orchestration settlement、chat 轮边界推进）。
  2. 根因明确则修复：根因链+diff+红绿（单测构造 settle 完整轮断言 materiality/target_response 产生+receipt revision 区分）。
  3. **门**：`scripts/run_free_state_d1_smoke.ps1` 真栈 **exit 0**（AGENTS §5，恢复主烟测通道）+agent 全量 0 FAIL；run 工件按 §9 契约。
  4. 回执：根因链+锚点+修复 diff+红绿+exit 0 run ID。
- 文件域：agent Go settle 机制相关（experiment/orchestration/chat）+scripts/free_state_d1_smoke.py；越域即停上交。
- 停止条件：取证发现断裂源于更深层设计态差异（State A/B 类结构性两态）→ 证据上交决策侧裁定，不自行改设计。
- 领取：2026-09-28 下午 / origin/main=1f4bd906（同 fetch 后确认同步）/ 分支 port/settle-chain-1 / worktree D:/Vit_DAW_worktrees/settle-chain-1 / 领取时 main 工作树仅 VitApp/Workspace/default_project.xml（运行时工程状态，会话前既有）
- 回执（2026-09-28 晚，执行侧自验完成待决策验收）：
  - **实现 commit**：`a3f20ec0`（分支 `port/settle-chain-1`，已推）。文件五件：`agent/internal/chat/audition_events.go`（+15）、`agent/internal/chat/free_state_reasoning_loop.go`（+31）、`agent/internal/chat/free_state_d1_runtime.go`（+47）、`agent/internal/chat/settle_chain_test.go`（新，301）、`scripts/free_state_d1_smoke.py`（+33）。
  - **取证结论（卡面核心问题）**：形态①②**不同源**——是同一 apply 后链上三个相继弱点（全 main 既有，DOSE 卡定性正确）：
    1. 形态①（120230/120725，materiality 缺失）：`requestAuditionJudgment`（audition_events.go）**先**做 `requireTaskHumanJudgment`（task→human_judgment_required）**后**调 `RequestUserJudgmentForSession`（因 round 无 target_response 被拒，真栈 WARN 在案）→ 悬空边界使 settle 决策在 `applyFreeStateDecisionSemantic` 被迁移表拒绝（`improvement_proposed is not allowed from human_judgment_required`，taskstate/state.go:280）→ recordFreeStateDecision 早退 → settle 报告永不摄取；
    2. 形态②（194926=主树复现 + 基线 121206，receipt applied_unreconciled）：确定性 post-action 观察 booking 单发碰 CCB 旧 revision 竞态（+6s 得 rev2/fresh=ready vs applied 4）→ 欠账搁浅，后续全押模型自发观察（executed=0）→ closure 无进展窗耗尽；
    3. 次序降级（200332 新取证）：booking 清欠账后观察摄取失去资格守卫，前动作包回放被追加到 booked 观察之后 → `RecordTargetResponse` 的"最后一条必须 post-action"不变量被破；
    4. 烟测时序（201712 新取证）：break 条件按 pre-GAP-1 时序写就（human_audition_ready=settle 完），GAP-1 提前挂卡使旗标在 settle 尾段前为真 → pre-settle 投影上提前校验。
  - **修复**：①前置 round 资格检查 + settle 落地后于 recordFreeStateDecision 尾部重驱动判定边界（store 后调 requestAuditionJudgment，防外层 clobber）；②respond 链内有界重试 6×4s（真栈 +13s 可达，194926/202511 实证）；③round 已持 post-action 观察后拒收非 post-action 信封观察；④烟测退出前轮询 round 的 materiality/target_response（断言不弱化，settle 不落地仍诚实失败）。
  - **红绿**：settle_chain_test.go 四钉全红→绿：premature task transition（红：task 变 human_judgment_required）、settle 落地+重驱动（红：UserJudgmentRequested=false）、booking 重试契约（红证据=真栈 194926）、回放降级（红：last obs 变 pre@7）。
  - **门**：`scripts/run_free_state_d1_smoke.ps1` 真栈 **exit 0 PASS**（run=`artifacts/free_state_d1_s1/20260928_202511`，validation：static_eq rev3→4 readback −1.5 post-obs obs_20260928T122819、round materiality=subthreshold target=ambiguous decision=user_judgment_pending judgment_requested=true 四旗标真 settled=false）+ `cd agent && go test ./... -count=1` 全量 EXIT=0；触碰文件 blob 级 gofmt 干净（autocrlf 教训应用）。
  - **§9 记录**：命令=`run_free_state_d1_smoke.ps1 -RepoRoot <worktree> -PublicManifest <主树 manifest> -KernelExe D:\Vit_DAW\...\VitApp.exe -SkipBuild -TimeoutSeconds 900`；被测 agent 二进制=worktree 构建 2026-09-28T20:17:07 SHA256 前 16 位 `9E43F171D4C1E8A4`（含全部 Go 修复）；内核=主树 `A29751807DC425A6`（2026-09-28T11:51:39，C++ 零 diff 本卡只改 Go+脚本）；HEAD=port/settle-chain-1@1f4bd906+本卡 diff（现 a3f20ec0）；失败史（§8：每版本每断点 ≤1 次即取证修复，无原样重跑）：194926（断裂②）→200332（断裂③）→201712（断裂④）→202511 PASS；工件不覆盖，四 run 目录俱在。
  - **端测边界声明**：本卡端测=free_state_d1 烟测真栈（脚本栈），渲染面/用户旅程未测（与 E2E-WEBUI-1/JOURNEY-1 建设前边界一致）；webui 无改动。
  - 遗留观察（不阻塞本卡）：settle 片曾见 latest_decision 短暂呈现 pre-settle 形态后终态正确（202511 终态亲核无误）；D2 multi-round 路径未触（单轮门内）。
- 验收：**pass（2026-09-28 决策侧，rulings/2026-09-28-SETTLE-CHAIN-1-pass.md）**——exit 0 直读亲核（status=pass/四旗标全真/decision=user_judgment_pending）；四轮 §8 取证史工件归档 coord/runs/SETTLE-CHAIN-1/；烟测脚本改动审定=带 deadline 轮询非断言变更（"不弱化"声明成立）；§9 二进制口径无瑕疵；合并态 main 全量门+blob gofmt 决策侧复跑。四断裂根因链各有独立取证与锚点，红绿四钉采信。**DOSE-AUDIBLE-1 端测门回补条款随本卡闭合**（run 202511 栈含 DOSE 全部改动+内核同一）。实现 cherry-pick a3f20ec0→11ff8984。

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
- 回执：（待回填）

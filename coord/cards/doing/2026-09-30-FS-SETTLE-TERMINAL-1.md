# FS-SETTLE-TERMINAL-1：d1 欠账结算尾段非法相位迁移 fs7→fs9（M1 复验实测失败根因，取证+修复）

- 池序 11（P1 用户面失败）；来源=M1 复验活栈取证（[decisions/2026-09-30-vitnote-p1-ruling.md](../../decisions/2026-09-30-vitnote-p1-ruling.md)；证据=coord/runs/M1-RETEST-20260930/evidence/）
- 优先级 / 预估 / 依赖：P1 / 取证 0.5 天+修复 0.5–1 天 / 无
- 模型分级：取证段 L2/GLM 亲自或强引导（状态机并发语义）；修复段视取证结论定
- 已核实事实（决策侧活栈只读探针钉死，勿重勘）：
  - 现象链：观察问答轮（goal_4e4c14a93473a1e2，草稿工程）→ 内核已渲染 audition A/B 预览（kernel-audition-sessions.txt：candidate-a/b@2ch44100）→ 20:09:40 free_state final gate 拒绝二次 needs_experiment 准入（"applied experiment round pending settlement...emit the settle report"，final-gate-error.jsonl）→ durable continuation（d1 owed-settlement tail）→ **minimal audio closure controller: illegal phase transition fs7_improvement_proposal -> fs9_terminal** → 轮 failed（ui-state-failed.json failure_reason 原文）。
  - 代码锚：internal/audioclosure/phase.go:93 fs7 合法后继={fs8,fs4}；:148 校验器拒绝——**校验器正确，缺陷在请求方**（d1 欠账结算续跑路径构造了 fs7 直达 fs9 的迁移）。
- 目标：
  1. **取证**：锚定 d1 owed-settlement durable continuation 的迁移构造点（agentloop/audioclosure 续跑路径）——为什么结算尾段把 next phase 定为 fs9 而 当前在 fs7；是欠账结算跳过 fs8 验证相位的路径缺陷，还是 settle 失败后的终止语义错用；给出迁移序列时序（事件/日志回放）。
  2. **修复**：按取证定方案——欠账结算合法收尾路径（fs7→fs8→fs9 走全或显式定义 fs7 的 terminal 出口语义并修迁移表——**不得直接改表放行 fs7→fs9**，除非取证证明语义上合理并申报）；gate 拒绝（要求 settle report）与续跑终止之间的交互语义一并理清。
  3. **回归**：复刻本次场景用例（已应用轮待结算+LLM 二次 needs_experiment 被拒+续跑收尾）——修复前红（illegal transition）/修复后绿（合法收尾或明确 settle 边界呈现给用户）；含 audition 预览已渲染的会话形态。
- 文件域：agent/internal/audioclosure/ + agent/internal/agentloop/（续跑/结算路径，实锚后申报）+ 测试。
- 验收标准：取证报告（迁移序列+根因定性）+修复 diff+新增回归用例红绿+全量 0 FAIL+gofmt；**真实栈复验**：同场景（观察问答→实验轮→待结算时打断/二次准入拒绝）exit 0 或失败面以用户可读的 settle 边界呈现——烟测脚本扩展按 AGENTS §5。
- 停止条件：取证发现 fs7→fs9 请求源于更上层的 d1 欠账记账缺陷（结算账本本身错账）→ 上交定 scope。
- 领取：2026-09-30 20:17 +0800 / 8e9b7b10cbc674d137dd063481e4a6a07b9e0a74 / port/fs-settle-terminal-1（worktree D:/Vit_DAW_worktrees/fs-settle-terminal-1）
- 回执：（commit hash / 取证报告路径 / 修复方案一句话 / 真栈 run ID）
- 验收：（裁定文件 / 验收 commit）

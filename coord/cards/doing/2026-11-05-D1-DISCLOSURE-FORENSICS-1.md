# D1-DISCLOSURE-FORENSICS-1：D1 场景披露面取证与修复（轨道标识披露×证据新鲜度准入）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=[256 pass 裁定](../../rulings/2026-11-05-D1-RUNNER-FORENSICS-1-pass.md) §二——两 run 两形态共同上游=披露面）
- 派发确认：已确认（链内承接卡，主管发卡）
- 验收负责人：GLM 主管决策流
- 池序 258；目标仓库=主仓 agent（chat/披露与准入面）；P1 / 0.5-1 天 / 依赖=254/256 已在 main；L1-L2 / PC 执行会话；验收腿需真栈

## 背景（两形态，工件在 coord/runs/D1-RUNNER-FORENSICS-1/verifyrun1/）

1. verifyrun1 exit 3 NOT_EXERCISED：模型 fs4 观察后 terminal_causes=「当前上下文未披露任何轨道标识，无法填写提案目标」——提案面未达；
2. verifyrun2 exit 1「D1 loop projection is missing」：首响应即 capability_blocked=证据新鲜度准入未过（零实验零修改零 loop 投影）；
- 家族背景：256 已证 M3 措辞链 system +23% 行为面漂移；本两形态=家族第三/四型（257 G6=第五型，另卡）。

## 目标

1. **取证**：钉死两形态机理——①轨道标识披露窗口：模型在 D1 场景上下文里拿不到什么（观察回执/pack 投影缺轨 ID？措辞指引要求填写但上下文无源？）②证据新鲜度准入：freshness 判据为何 fail（观察时序？pack 缓存？M3 后行为导致观察路径变化？）；产出根因+锚点；
2. **修复**：按根因最小修（预期在披露投影/准入判据/指引三面之一；agent 域内）；256 断言面**不得回退**（分支容忍已合入）；
3. **验收腿**：修复后单 run 重跑 **254/256 门组**（LW 四门+D1 settle 四门+queue view D1≠not_started）——过则 **D1 真栈点亮收口**，254 运行面挂账清。

## 约束

- 文件域=agent 披露/准入相关最小面+测试；脚本面如需只动 254 场景脚本的透传缝；禁触 automation_intent 词表/semantic_entry；
- §8 跑前口径成文：取证腿读档零 LLM；验收腿单 run ≈¥1-3；
- 治理红线不变：确认链/零裸写；修复不得以放宽治理为代价。

## 停止条件

- 取证发现根因在 M3 措辞链本体（需措辞修订）→只取证上交转卡；验收腿断言失败→取证不重跑。

## 领取/回执/验收

- 领取：2026-11-06 10:23 +0800 / origin/main=92022b7c1587ab56bb50b419174d5d669451beb2 / owner=ZCode GLM-5.3 L1 PC 执行会话（258 卡会话，GLM-5.3 / PC / ZCode 会话 sess_b01bd67c）/ 分支 port/d1-disclosure-forensics-1 / worktree D:/Vit_DAW_wt_258impl / 领取基线：`git rev-parse HEAD`=92022b7c1587ab56bb50b419174d5d669451beb2（=origin/main），`git status --short`=空（干净树），`git diff --stat`=空（无领取前已有 diff）/ 领取提交=（推送后回填）
- 回执：（执行侧回填）
- 验收：（主管回填）

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

- 领取：2026-11-06 10:23 +0800 / origin/main=92022b7c1587ab56bb50b419174d5d669451beb2 / owner=ZCode GLM-5.3 L1 PC 执行会话（258 卡会话，GLM-5.3 / PC / ZCode 会话 sess_b01bd67c）/ 分支 port/d1-disclosure-forensics-1 / worktree D:/Vit_DAW_wt_258impl / 领取基线：`git rev-parse HEAD`=92022b7c1587ab56bb50b419174d5d669451beb2（=origin/main），`git status --short`=空（干净树），`git diff --stat`=空（无领取前已有 diff）/ 领取提交=05012e5bfe35c627022e627568d77b435b40fc80
- 回执（2026-11-06 11:4x +0800，执行侧）：
  - **取证**（coord/runs/D1-DISCLOSURE-FORENSICS-1/forensics.md）：两形态根因均钉死且**均不在 M3 措辞链本体**（停止条件不触发）——②verifyrun2=pcactl 认证腿（外部写者）mid-request 推 revision 2→3→4×closure 冻结 rev2（仅请求入口 reconcile）×G7 以冻结值为准绳致准入不可满足×落账撞跨修订守卫 settle 死（closure 持久化 project_revision=2+settlement=project_revision_stale 亲证）；①verifyrun1=ledger（唯一跨轮轨 ID 绑定记忆）为最大 warm 段首降级 2176B→139B 桩（telemetry 逐轮亲证）×终局锁禁重取。触发归因：外部经 agent HTTP 认证面，脚本面零调用（最可能 Godot 面板手动，未证实）——修复不依赖归因。
  - **修复**（impl 1b9321e9，agent 三文件+三回归测试，96 包全量绿）：①gateG7 准绳漂移治愈（kernel 铸造回执严格更新 revision+无 active_capability+非 superseded→以其最新值为准绳；gap 文本同步）；②recordAudioClosureRound 录制前一次性 Revalidate 治愈（fold 后 round 重开+pre-drift 行跳过；parked/post-action/superseded 守卫不动，FIX-SETTLE-PARK-REPLAY-RACE-1 钉死语义兼容）；③observation_ledger warm 降级序最后（预算/marker/fail-closed 零改动）。256 断言面零触碰；automation_intent 词表/semantic_entry 零触碰；治理红线不变。
  - **验收腿**（run 20261106_113023，§8 单 run 无重跑 ≈¥0.1-2 内）：被测 agent=现场构建 B0122940（binary face 亲证含修复）；kernel F9E3DA0A 同款；**runner exit 0=D1-S1 PASS（branch=zero_mutation_boundary=256 合法化分支）——admission.boundary=admitted G1-G8 全 pass+target_ref=track 1032（对照 verifyrun1「无轨 ID」）+selected_domains=[reverb]（对照 verifyrun2「提案未达」）=两形态上游修复真栈亲证**；resp1=semantic_treatment_capability_boundary（armrun2 同型诚实边界零 mutation）；**PS1_EXIT=1=LW 相位 throw**（watch hook 懒注册无触发器：0-mutation 分支无 project_change/settle——非 LW 面回退，本卡零改 LW）+D1-SETTLE 四门/queue view D 门未执行到（ps1 相位序被拦断）——254/256 字面门组全过仅 1-mutation 分支可达（133 run2 参照）。TEARDOWN_ALL_ZERO 11:39:23。**处置三选项交主管**：①LW/settle 相位门分支化（脚本面卡）；②再派单 run 赌 1-mutation 分支（133 序列 8 run 仅 1 次）；③本卡收口「披露/准入修复面 pass+D1 runner 面点亮」，LW/settle 全门组另卡。
  - **事故披露（交主管裁定）**：释放登记时 checkout 协调分支被未提交改动拒绝→pull 在 port 分支合并（bc191eee）→`HEAD:main` 误将 port 分支整推上 main（efd367af，含实现 1b9321e9+取证 2ae0f39a）——违反 §3「实现进 main 须经验收 cherry-pick」。未 force-push 未改写历史；主管可选 bless（验收通过即无实质损害）/revert 提交（1b9321e9+2ae0f39a）。port 分支最新=1fc0d74f（+run 工件九件）。
  - 栈登记：占用 46b7b200（净空亲核 11:26:48+≤60s 复核 11:28:04）/释放 efd367af（TEARDOWN_ALL_ZERO 亲核 11:39:23）。
- 验收（2026-11-06 早窗收尾，主管）：**pass**——取证/修复面全过（两形态根因钉死非 M3 本体；三面最小修+红线核对成文）；运行面=部分达成按此收口：runner exit 0 D1-S1 PASS（admitted G1-G8 全过+target 1032）=两形态上游修复真栈亲证，D1 场景全链（观察→轨 ID→提案→admitted→确认→诚实边界）真栈点亮成立；LW settle/queue view 门=分支结构性未达（0-mutation 分支不可达），不宣称全门组收口。三选项裁定③+①：本卡收口+LW/settle 相位门分支化并入 260 第四项；不赌 1-mutation 重跑。254/256 运行面挂账核心清偿（残余归 260+后续 1-mutation 机会 run 顺带）。efd367af 误推事故裁定 **bless**（验收通过追认；教训入律：推 main 前显式核对 HEAD 与目标 ref 一致+登记文件单独成提交禁 `HEAD:main` 快捷式）。合入 9714865b（工件；263 先行 403d54fa）零冲突，交叉复跑 build+chat/agentloop/contextruntime 全绿。详见 [rulings/2026-11-06-D1-DISCLOSURE-FORENSICS-1-pass.md](../../rulings/2026-11-06-D1-DISCLOSURE-FORENSICS-1-pass.md)。

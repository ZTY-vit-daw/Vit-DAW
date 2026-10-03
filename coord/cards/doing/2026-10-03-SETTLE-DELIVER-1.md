# SETTLE-DELIVER-1：判定结算后续链三症状——结算确认不进对话流/二轮 goal 撞 revision stale 停/二轮消息插队（P1）

- 池序 5（P1——阻塞 JUDGMENT-SETTLE-STALL-1 等四张 conditional 卡转正）；目标仓库=D:\Vit_DAW；来源=[decisions/2026-10-03-user-manual-test-feedback.md](../../decisions/2026-10-03-user-manual-test-feedback.md) 问题 7；**取证证据已固化**：[runs/MANUAL-TEST-20261003/](../../runs/MANUAL-TEST-20261003/)（事件流 43 条全量+状态面两份+结算 checkpoint 文案）
- 优先级 / 预估 / 依赖：P1 / 取证 0.25 天+修复 0.5 天 / 无（取证已由决策侧完成大半，执行侧勿重复）
- 模型分级：L1 / GLM 首选（settle→消息发布链+修订守卫语义）
- 已核实事实（决策侧活栈取证，勿重跑）：
  1. **判定链本体全通**（webui_murptx58 事件 seq 28/34/36/37：user_judgment.requested→recorded→trajectory.settled→judgment.settled；结算确认文案已落 manual checkpoint 01:32:39Z"Track 1017音量 +3 dB 已应用"）——SETTLE-STALL-1 修复生效，本卡不碰判定通道。
  2. **症状 A（结算确认不进对话流）**：seq 37 judgment.settled 之后**零消息投递事件**，确认文案只在 project_history checkpoint，webui 对话流无新消息。缺口在 settle→conversation message 发布/渲染面。
  3. **症状 B（二轮 goal 撞 stale 停）**：第二轮 goal_0b27…（"能帮我看看bass轨道的低频怎么样吗"，contract kind=diagnostic）seq 38-43：一轮 mix_observe 后 turn.completed **stop_reason=project_revision_stale**，语义态 closed"chat turn closed without governed experiment"，25 秒即终、completed_steps=0。时间线：判定结算应用变更（revision bump）→ 新 goal 创建（09:33:04）→ 撞陈旧修订守卫。
  4. **症状 C（二轮消息插队显示在首轮输出上方）**：用户目视报告，渲染序形态，待 RED 编码。
- 目标：
  1. **症状 A 修复**：judgment.settled 触发结算确认作为**正式助手消息进入当前 conversation**（持久化+webui 可见；SETTLE-STALL-1 的"可见新回复"判据由此真正达成）。实锚 settle 收口处（334d8f48 引入的闭环收口一带）到 chat message 发布的断链点，修复并补进程内测试（RED：settle 后 conversation 含确认消息）。
  2. **症状 B 取证+修复**：判定结算应用变更后，新 goal 为何读到陈旧 project_revision（守卫语义 vs 修订同步时序）；修后同一场景（判定→结算→新输入）新 goal 正常进治理链执行。不得以放宽守卫收口——须找到修订同步的正确路径。
  3. **症状 C**：与 A/B 同场景复验消息序（链终局钉尾机制 WEBUI-MSG-ORDER 家族）；若为独立渲染缺陷，最小修复+webui 测试。
  4. RED 编码三症状（judgment_settle_stall_test.go 同文件族或新测试文件）；回归：chat 全包+全仓 -count=1 零 FAIL+webui npm test。
  5. 真栈烟测：A/B 判定→可见新回复→新输入正常执行 全链 exit 0（AGENTS §5；场景并入 dev_agent_smoke 或按 run_ab_result_smoke 模式），并留 user 手测复验点。
- 文件域：agent/internal/chat/（settle 收口+消息发布，实锚后申报）+revision/守卫相关（症状 B 实锚）+webui 渲染段（仅症状 C 需要时）+测试文件。
- 约束：不动判定通道（已验证工作）；worktree 纪律；泊位错峰（用户手测栈 10-03 上午已关，起栈前确认）；探针/烟测工件落 runs/。
- 验收标准：三症状 RED→GREEN+全量 0 FAIL+真栈烟测 exit 0+用户手测复验（判定→新回复→新输入全链）。
- 停止条件：症状 B 根因在内核修订发布面（agent 侧无法闭环）→ 实证上交转内核卡；消息发布链涉及 history/worktree 深层语义 → 上交。
- 领取：2026-10-03 10:31 +0800 / 572f2ebde0546699393ac37d7536d580c84ae936 / port/settle-deliver-1（PC 执行侧；领取时工作树仅 VitApp/Workspace/default_project.xml 运行时态改动 + 未跟踪 .zcodeignore，与本卡文件域无交集，保留不动）
- 回执：（commit hash / 实锚断链点 / 三症状红绿 / 烟测 run ID / 泊位声明）
- 验收：（裁定文件 / 验收 commit）

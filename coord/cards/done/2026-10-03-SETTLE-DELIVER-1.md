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
- 回执：实现 commit **3035c8c5**（分支 port/settle-deliver-1，已推 origin）；实锚断链点：**症状 A**=finishJudgmentSettlement（audition_events.go）只结算 audio closure+发 MessageKind=activity 瞬态事件、从不写会话图消息（正常回合走 RecordConversationNodeForProjectWithData("vit")）——修复=recordJudgmentSettlementReply 直写 history（纯文件 checkpoint+vit 节点，零内核导出零 journal 污染）+webui settlementMessagesFromEvents 事件路由；**症状 B**=recordAudioClosureRound（audio_closure_controller.go）按会话键重水合 free-state 账本，round-1 已结算 loop（retain 路径不清账本、无所有权检查）的 revision=2 观察行回放进 round-2 新闭包（revision=3）撞 RecordObservation 跨修订守卫（取证反推：round-2 合同 rev=3 而内核停在 3，陈旧方向=回放面）——修复=audioClosureLoopOwnsRound 所有权门（本闭包 goal+实验未终态），守卫零放宽（真跨修订观察仍 stale，反钉在案）；**症状 C**=取证 43 事件流原样回放（trace/renderPlan）实锤：无锚 loose 组（一轮链终局与二轮乐观输入同组）按 user/rest 切分逆 createdAt 时序——修复=WEBUI-MSG-ORDER-3 无锚组不切分整组保序。三症状红绿：Go 4 钉+webui 7 钉（症状 A/C 逐钉摘修复复验真 RED；B 钉精确复现实栈 stop_reason 与文案）；回归：chat 全包+全仓 go test -count=1 零 FAIL+webui vitest 420/420+tsc/vite build 全过。烟测：**scripts/settle_deliver_smoke.ps1**（本卡新增，d1_stall 模式）9 轮台账与证据回指见 [coord/runs/SETTLE-DELIVER-SMOKE-SUMMARY.md](../../runs/SETTLE-DELIVER-SMOKE-SUMMARY.md)——症状 A 真栈双面已证（run 114223/121934：judgment.settled 事件带结算报告+draft 工程图落 vit 节点 kind=assistant/logical=judgment_settle:&lt;evidence&gt;）；**单场 exit 0 未达成**（判定上游链——本卡明示不动的已验证工作——PC 上模型随机分支仅 2/9 轮到判定席：LLM 供给中断 1、post-action 续跑片停滞 2、无 A/B 分支 1；其余为烟测脚本自身成熟度，逐轮定性在台账）；症状 B 真栈复验与全链终验留**用户手测复验点**（判定→新回复→新输入，含刷新后结算确认仍在对话流）。泊位声明：起栈均核三端口空闲+无 VitApp/VitAgent/Godot 残留（用户手测栈已关），首栈 11:07 于 9–12 错峰窗内起，末栈 12:29 出窗后泊位实测仍无主（端口空、无用户活动）续跑收口；工程 default_project.xml 前后哈希一致（零污染）。运行时态改动（default_project.xml 领取前已存在）与 .zcodeignore 未纳入任何提交。
- 验收：**conditional pass（2026-10-03 决策会话）**——[rulings/2026-10-03-SETTLE-DELIVER-1-conditional.md](../../rulings/2026-10-03-SETTLE-DELIVER-1-conditional.md)；实现面四验全过（diff 亲核/我方复跑：chat 全包+全仓 -count=1+vitest 420/420+tsc+vite build/**RED 抽验 A/B 双钉亲手复现**/工件亲读：121934 事件+conversation_graph 节点双面证实）；转正条件=用户手测复验（判定→可见新回复→新输入→刷新仍在；同场完成 SETTLE-STALL-1 等三卡终验）；cherry-pick 3035c8c5=d8b7e2d3 合 main
- **转正 pass（2026-10-03 晚，用户栈实证）**：三症状全数达成——A=结算确认作为 assistant 消息落对话流（会话图 n_20261003T113721 logical=judgment_settle:judgment-9e0daba…）；B=二轮输入正常进治理链工具面跑完（无 stale/ownership）；C=消息序正确。证据 [runs/MANUAL-TEST2-20261003/FORENSIC.md](../../runs/MANUAL-TEST2-20261003/FORENSIC.md)；收口段新缺陷（空名工具报错）另立 REPLY-GEN-TOOLGATE-1，非本卡回归。

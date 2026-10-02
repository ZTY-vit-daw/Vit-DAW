# JUDGMENT-SETTLE-STALL-1：A/B 判定后结算停滞——判定未落 round 状态+会话所有权楔死+新输入被拒（P1）

- 池序 33（Mac 侧已将 D1-SETTLE-TAIL-MAC-1 归一至 32，本卡顺延；P1——A/B 判定是人耳判断边界唯一出口，判定后链路死=自由态实验闭环断裂）；来源=用户 2026-10-02 晚真机手测（[decisions/2026-10-02-user-manual-test-feedback.md](../../decisions/2026-10-02-user-manual-test-feedback.md) §4）；目标仓库=D:\Vit_DAW（PC 执行侧）
- 优先级 / 预估 / 依赖：P1 / 取证 0.5 天+修复 0.5 天 / **建议 AUTH-RESTORE-LOGSPAM-1 先行**（判定 POST 服务端痕迹已被刷屏摧毁一次，取证必须先有干净日志面）
- 模型分级：L2 / GLM 首选（chat/settle 机械域+时序敏感取证；D1-SETTLE-TAIL-MAC-1 同族嫌疑，两卡根因可能同源）
- 已核实事实（活栈只读取证在案，勿重跑）：
  1. goal_9c7018（webui_muqwy5sv）20:02-20:04：观察→应用 Track 1022 EQ+3dB→20:04:19 park 等判定（手工检查点+候选 B 处理检查点×2 在 /agent/state project_history）——park 前全链正常。
  2. 用户点 A/B 后：webui 卡标记"已完成/用户已选择"；但服务端 round r_922b53ffbfed 仍 `stop_reason="experiment round is waiting for the human judgment boundary"`、task_state=needs_experiment、revision=4（/agent/runtime/status continuations 字段，取证时点 20:1x）——**判定未落 round 状态**。
  3. 新输入 goal_a330（"drums 轨低频怎么样"）失败：`"conversation is already owned by minimal_audio_closure controller audio_closure_7484e3e83a33a4ea"`（/agent/state active_goal.failure_reason）——**parked round 控制器持有会话所有权未释放**。
  4. 消息循环内另有 20:04:03 gate 纠偏错误（"pending settlement 非法二次入场"）——属 park 前轮内自纠（20:04:19 重试出 park 决策），非用户报错本体，注意区分。
  5. agent_last.log 55KB 全为 authority restore 刷屏（~3 分钟轮转）——判定 POST 的服务端 Warn/接收痕迹不可恢复。中间事件流快照（/agent/events）无 judgment 类事件。
- 目标：
  1. **取证**：判定 POST 服务端真实结局——三嫌疑分诊：①409 拒绝（`recordFreeStateAuditionJudgment` 身份/revision 守卫：audition_events.go:818 起——freeStateLoop 内存态、round/AuditionSessionID 匹配、project revision 匹配三关）而 webui 仍乐观标记完成（409 显形是否覆盖此卡形态）；②落错分支（`recordMixTickAuditionJudgment` B12-2 判定席分流——mix-tick 卡语义不推进实验状态机，用户点的是否为此形态）；③判定已录但 settle 触发器缺失/停滞（judgment→settle 回合的调度路径；对照 Mac D1-SETTLE-TAIL-MAC-1 的尾段停滞形态）。复现路径=修复后须可稳定复现原缺陷（构造同形态任务或 E2E）。
  2. **修复**：按定性修——判定必须落 round 状态+驱动 settle 回合（LLM 收判定+证据→settle report→执行保留/回滚→**向用户出可见新回复**——用户裁定 2026-10-02：判定后反馈面=通用确认卡语义）+settle 完成/失败都不得楔死会话所有权（失败路径显形+可恢复）。
  3. **回归**：chat 域测试全绿+全量 0 FAIL（-count=1）+E2E 判定→settle→新回复→新输入全链新组+真栈手测复验（与用户约定）。
- 文件域：agent/internal/chat/（audition_events.go 一带+所有权释放点；实锚后申报）——若根因在 webui 乐观标记面则另申报 webui 域。
- 验收标准：三嫌疑定性结论+修复 diff+上述回归全绿+用户手测复验（判定→有新回复→新输入正常进入处理）。
- 停止条件：取证发现根因与 D1-SETTLE-TAIL-MAC-1 同源（settle 尾段调度）→ 两卡并轨上交决策侧统筹；webui 域缺陷超出本卡域→带锚点上交。
- 领取：2026-10-02 晚 / origin/main=200eddf6 / port/judgment-settle-stall-1（PC 执行侧 GLM-5.3，独立 worktree）
- 回执：（commit hash / 定性结论 / E2E 新组 / 复现路径）
- 验收：（裁定文件 / 验收 commit）

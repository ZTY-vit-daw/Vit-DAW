# FS-PARK-TURNFAIL-1：judgment park 存活下新用户消息的三拒终局失败 + turn id 复用语义（M8 手测实测，P1）

- 池序 14（P1 用户面失败，FS-SETTLE-TERMINAL-1 的同族下一案）；来源=M8 手测活栈取证（证据=coord/runs/M8-FORENSIC-20260930/evidence/：事件流/ui-state/message-loop 三件）
- 优先级 / 预估 / 依赖：P1 / 取证 0.5 天+修复 0.5–1 天 / FS-SETTLE 已合入（本卡是其姊妹案：闭包边界已修，轮终局门仍败）
- 模型分级：L2 / GLM 亲自（自由态门语义+LLM 引导设计）
- 已核实事实（2026-09-30 22:04–22:08 活栈，webui 会话 webui_muo6fygb，goal_6da74bd8/run_e5796736，草稿工程 draft_20260930T140416）：
  1. **第一轮**（22:04:59 输入"检查一下当前工程有什么问题"）：turn.started→22:05:04 turn.completed 输出正常；后续 d1 结算切片继续跑 items（22:05:11-22:05:22）→22:06:37 再次 turn.completed（**同一 run_id/turn_id 生命周期翻转：started→completed→items→completed**）；audition A/B 已渲染、trajectory.user_judgment.requested×2、loop 进 judgment park。
  2. **第二轮输入（22:07:20）**：无新 run/turn id——**被续进同一未终局 goal**（复用 run_e5796736）→ 22:07:38 准入门拒绝（needs_experiment 全准入门缺口）→ 22:08:07 加强重试仍不可解析 → `free_state_terminal_fallback: free_state_terminal_turn_unparseable: no admissible final decision after one strengthened retry` → **22:08:07 turn.failed（用户面"输出失败"）**。
  3. **前置同类门拒绝**：22:06:23 gate 拒二次 needs_experiment 准入（"applied experiment round pending settlement...emit the settle report"）——LLM 在 park 场景反复给不出 settle report/合规终局决策（含 22:06:01 一次 parse_failed）。
  4. **50s 回执耗时初判**：22:05:22（末 item 完成）→22:06:37（turn.completed）的 75s 尾段内含 ≥2 次 LLM 往返+解析修复+gate 循环——**耗时主体是 LLM 重试与门循环，非 CCB 数据组装**（用户感知的"CCB 回执清单 50+s"是尾段表象）；精确 per-call 归因（llm telemetry，UTC 时间戳）在本卡内完成。
- 目标：
  1. **取证**：a) park 存活+新用户消息的路由语义——goalrunner 同会话续用未终局 goal 时，新消息应允许"回答用户+保持 park"而非强制 settle/终局决策（锚定 goalrunner_chat 续用条件与 final gate 的交互）；b) 三拒失败链的逐次 prompt/response 回放（telemetry），LLM 实际产出了什么、gate 错误引导为何不收敛；c) turn id 复用对下游（webui 消息序/轨迹动效，关联 WEBUI-MSG-ORDER-1）的语义面。
  2. **修复**（**用户裁定已定方向，2026-09-30，decisions/2026-09-30-park-adoption-and-judgment-card-ruling.md**）：a) park 期间**新用户输入 = 对待裁决段默认采纳**——保留已应用状态、关闭该轮（**新一轮新 turn id**，解决生命周期翻转）、进入新一轮思考；结算记录如实标注 adopted_by_continuation 语义（**不得记为 human_confirmed**——证据链诚实边界）；b) park 不强制终局：门拒绝/超时不得 turn.failed，改为用户可读边界呈现（人耳判断 POST 仍是显式 settle 通道）；c) A/B 卡在新输入后被 settle 为"默认采纳"终态（与 AB-JUDGMENT-CARD-1 联动）。
  3. **回归**：复刻本场景（观察问答→实验→park→新用户消息）——修复前红（turn.failed）/修复后绿（回答落盘+park 保持或合法 settle）。
- 文件域：agent/internal/chat/（goalrunner_chat/free_state gate/continuation 面）+ agent/internal/agentloop/（final gate）——实锚后申报；测试。
- 验收标准：取证报告+修复 diff+回归红绿+全量 0 FAIL+真栈复验（park 中新消息场景 exit 0 或用户可读边界）——烟测扩展按 AGENTS §5。
- 停止条件：取证发现 park 路由在架构层无定义（需要产品设计裁定）→ 上交决策侧。
- 领取：2026-10-01 上午 / origin/main=ec93d570 / 分支 port/fs-park-turnfail-1（worktree D:/Vit_DAW_worktrees/fs-park-turnfail-1）
- 回执：
  - **实现 commit**：port/fs-park-turnfail-1 @ 2b4953e7（Part A 默认采纳+Part B 边界呈现+新枚举）+ 6b3624d1（run1 归因修复：trajectory 校验门+receipt 诚实投影+烟测探针）——等决策侧验收 cherry-pick
  - **取证**：coord/runs/FS-PARK-TURNFAIL-1/forensic-report.md——三拒链事件级回放（拒①settled 轮门→拒②G 门 gap（stale 调度器副本致 judgment boundary 失效+计数锁终局轮）→拒③终局锁拒绝 LLM 按引导给出的 needs_observation→fallback→turn.failed）；根因 RC1 路由语义缺失（主）/RC2 陈旧副本/RC3 终局强制；50s 尾段归因=LLM 重试+门循环（与初判一致）；停止条件未触发（park 路由架构有定义，缺采纳结算，用户裁定已定案）
  - **修复**：①handleChat 在 beginChatGoal 前拦截——park+新输入→DecideRound/Settle(adopted_by_continuation)+task EventTaskSettled（清 pending interaction）+loop completed+CompleteGoal→新消息全新 goal/run/turn（生命周期翻转根除）；诚实边界：不写 UserJudgmentEvidence、TargetResponse 保持 human_audition_ready、receipt human_ab=skipped_by_continuation/disposition=adopted_by_continuation/禁 claim human_confirmed 与 net_outcome=improved；已记录人耳判断的轮不触发（显式 POST 通道优先）②terminal fallback 族失败在 park 存活时不投影 turn.failed——waiting_continue+冻结边界话术+judgment_park_preserved 标记；真执行失败仍诚实失败
  - **测试**：judgment_park_continuation_test.go 六钉（采纳语义+goal 收口+receipt 诚实+裸继续/命令不触发+已记录判断不覆盖+Part B 边界保留与失败不吸收）；全量 go test ./... exit 0（0 FAIL，87 包）×2 轮（trajectory 修复前后各一）
  - **真栈烟测**：run_free_state_d1_smoke.ps1 -ContinuationAdoptionProbe（本卡新增参数，扩展现有 ps1 体系）——红轮 20261001_112221（归因：探针断言面+trajectory 校验门，补锚修复，非原样重跑）/ **绿轮 20261001_113015 exit 0 PASS**：park 同形态（fs7/waiting_continue/judgment_park=true）下新用户消息——无 turn.failed、全新 goal_55b99e11/run_91081cc5（vs parked goal_8f6855a0/run_dee81121）、回复落盘、采纳结算落 task 语义（settled/terminal/task_settled 带采纳 summary）+事件流（round.decision+settled 带 adopted_by_continuation，零伪造人耳判断）、revision 不动；工件 coord/runs/FS-PARK-TURNFAIL-1/（smoke-record+红/绿摘要 JSON；全量在复验 worktree artifacts/free_state_d1_s1/20261001_113015/）；修复前红=M8 野外取证（同场景 turn.failed，evidence/ 事件流 seq47）
  - **泊位声明：运行栈已拆除**（7878/5555 无监听、Godot 已停，2026-10-01 11:4x）——未移交，会话②可上栈
  - **端测覆盖边界**（AGENTS §5）：本烟测覆盖 agent HTTP/事件面+内核栈；webui 渲染面（A/B 卡终态样式、消息序动效）未覆盖——A/B 卡联动归 AB-JUDGMENT-CARD-1（其卡面已收裁定 2），消息序归 WEBUI-MSG-ORDER-1
- 验收：**pass（2026-10-01 决策会话）**——rulings/2026-10-01-FS-PARK-TURNFAIL-1-pass.md；合并 main=63ea54c8/f2b51443（决策侧 cherry-pick+全量烟测工件补入库）；取证报告直读+diff 直读（裁定忠实+守卫完备）+我方复跑 87 包 0 FAIL+烟测红绿链亲读（Run2 exit 0 五族断言）+泊位合规；复现触发 FIX-MIXBOARD-FLAKE-2（同族 flake 二现，与本卡无关联）。M1 复验第三轮前置就此齐（与 M8 手测同场）。

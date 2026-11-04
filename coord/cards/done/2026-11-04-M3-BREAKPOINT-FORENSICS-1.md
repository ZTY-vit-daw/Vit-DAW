# M3-BREAKPOINT-FORENSICS-1：组 1 首绿三层断点勘察（终局驱逐/misroute/continuation 前台）

- 发卡：GLM 主管决策流 / 2026-11-04（依据=243 验收裁定 [2026-11-04-M3-WRITE-GATE-TRIGGER-CLARIFY-1-pass.md](../../rulings/2026-11-04-M3-WRITE-GATE-TRIGGER-CLARIFY-1-pass.md) 归因修正后的三层断点；分叉条款「仍停+新指引在场被读→接线问题勘察」触发）
- 派发确认：已确认（链内续卡，主管发卡；用户口头开工即有效）
- 验收负责人：GLM 主管决策流
- 池序 244；目标仓库=主仓 agent；P1 / 0.5 天 / 依赖=243 已在 main（e0aa12a1）✓；L1 / PC 执行会话
- **卡型：勘察卡（零实现面）**——产出=取证报告+代码锚点+修复方向分级；改代码即越域

## 勘察输入（全部已在库，不需要新真栈 run）

- 工件：`coord/runs/M3-WRITE-GATE-TRIGGER-CLARIFY-1/writegate1/`（payload_capture 168 行含三轮终局轮请求；rounds/telemetry/summary）
- 关键载荷坐标（主管亲证）：r1[42]/r3[33] 终局轮请求中 1011/0.740818 披露消失（前 5 轮连续在场）；r2[83] 披露在场=True、全要素持有、needs_experiment+软预问（「请在确认卡里核对起止时间，确认后才会真正写入」）；三轮 needs_observation 40/80/33 次；前台 waiting_continue/already_scheduled。

## 三面目标

1. **终局轮上下文驱逐机制**（基础设施面，主攻）：定位 compaction/有界上下文在长循环后终局轮驱逐工具结果披露的代码路径——触发条件（轮数/字节阈值）、保留策略（为何 assistant 文本留、tool 结果掉/或反之，以实据为准）、与 241「有界上下文顶掉正面发现」是否同一机制。产出：代码锚点（文件:行）+writegate1 载荷对照表+驱逐发生的直接证据（日志/指纹/请求裁剪痕迹）。
2. **misroute+软预问残留**（模型面）：r2[83] 违反骨架明文（「时变目标静态提案是错误器械」+v2「不预问」）——勘察 needs_experiment 判定链上还有哪些可见面引导与之竞争（是否存在诱导 needs_experiment 的其他段落）；软预问的条件结构（「如果片段不从 0 秒开始」=时间不确定性）是否属于「真缺要素该走 needs_observation 而非文本问」的判定缺口。产出：竞争引导清单+判定缺口描述，不写新措辞。
3. **durable continuation 前台吞**（基础设施面）：goal 挂起→already_scheduled 吞「继续」的生命周期；smoke 逐轮新对话（新 conversation_id）与 continuation goal 存活窗口的交互；80 次观察回执为何持续不满足（回执跨轮投影可见性——回执内容是否进下一轮请求上下文）。产出：生命周期锚点+回执可见性结论。

## 预期与诚实边界

- 纯勘察+载荷判读，预算=0 LLM 调用（用既有工件）；如确需复现某一腿，须先上交理由，单组 ≤¥1.5 且不重跑三轮。
- 三面各自给出：代码锚点（file:line）+载荷/日志证据+修复方向分级（T1 立即接线 / T2 挂证据触发器 / 模型面措辞迭代），供主管裁决拆卡。
- 证据不足的面如实标「未定位」，不得以推测充锚点。

## 停止条件

- 勘察发现与本卡前提冲突（如驱逐实为模型行为）→停止上交证据；
- 需要改代码才能继续取证的→停止上交，转修复卡由主管拆；
- 越域（任何非报告文件改动）即停。

## 文件域

只读勘察全仓；唯一写入=coord/reports/2026-11-04-m3-breakpoint-forensics-1.md（+必要时的卡面回填）。

## 验收标准

- 报告含三面各：锚点+证据+分级；终局驱逐面必须给出可直接拆修复卡的因果链（触发→裁剪→终局轮缺披露）；
- 零代码改动（git diff --stat 仅报告文件）；
- 回执按模板回填；卡 mv done 待验收。

## 领取

- 领取时间：2026-11-04（PC 日窗）
- 领取时 origin/main：052b211e44ee96fa87729f15406bb68a787e84dd（=本地 HEAD，fetch 后核对卡在 todo 无 owner、依赖 243 已在 main e0aa12a1 ✓）
- 执行 owner：GLM-5.3 Flash（ZCode PC 执行会话，2026-11-04）
- 协调 worktree：D:\Vit_DAW_wt_244coord（分支 coord/244-claim，仅卡状态提交）
- 勘察 worktree：D:\Vit_DAW_wt_244work（分支 port/m3-breakpoint-forensics-1，只读勘察+报告写入）
- 领取时主工作树状态：`git status --short` 含既存改动（VitApp/Workspace/*.xml ×2、coord/cards/done/M3-CHAT-AUTH-STAMP-1 相关 ×2、coord/runs/UX-AB-AUDITION-IMPL-1 ×4、若干未跟踪 design/reports/runs 目录）——均为他卡/历史遗留，与本卡文件域（coord/cards/todo/2026-11-04-M3-BREAKPOINT-FORENSICS-1.md + coord/reports/2026-11-04-m3-breakpoint-forensics-1.md）不相交；卡文件本身无未提交改动。领取基线 diff --stat：无本卡相关 diff。

## 回执

- **完成（执行侧自验通过，2026-11-04）**。报告：`coord/reports/2026-11-04-m3-breakpoint-forensics-1.md`，分支 `port/m3-breakpoint-forensics-1` @ **25c71355**（已推远端，待主管验收 cherry-pick；`git diff --stat`=仅报告文件 145 行，零代码改动 ✓）。
- **零 LLM 调用** ✓：全部结论来自 writegate1 既有工件（payload_capture 168 条记录逐条解析+needle 亲扫）+源码静读；无新 run、无新真栈占用。
- **面一（终局驱逐，主攻）**：机制定案=**chat 宿主 run 边界 typed round-trip 丢弃**——`freeStateLoopFromAny`（chat/free_state_reasoning_loop.go:3400-3413）json.Unmarshal 进 `freeStateReasoningLoop` struct（:69-215 **无 AutomationSurfaces 字段**）静默丢弃顶层 `automation_surfaces`，`mergeFreeStateLoops`（:449+）无处安放，续跑上下文 `freeStateLoopMap`（:442）重建即丢；agentloop 导出快照本身保留键（helpers.go:32-54）。触发=相位（limit_reached→checkpoint→终局轮续跑），**非字节阈值**（hot 10K/warm 5K/full 24K 预算机器独立存在且未触发，model_projection.go:174-230）。载荷对照：r1 ledger 披露在场记录 7–43（36 条连续）→终局 44 消失；r3 135–161→162 消失；r2 全轮 continuation 模式无 ledger 披露、幸存走**单槽时序运气**（终局前最后工具回执恰为 get_plugin_parameters，阻塞节区永不降级）。241 同族成立但机制修正：241 钉住 run 内有效（载荷亲证），跨宿主边界不存活。T1=struct 字段+merge 分支（确定性修复，验尸 needle=终局轮 ledger 含 1011/0.740818）。
- **面二（misroute+软预问）**：竞争引导七项全清单（payload @offset 亲证+代码锚点），最重两项=①终局指令句二选无「直接调用」形态（free_state_reasoning.go:792）②**格式示例邻接缺陷**——v2 Action Path 段后唯一 final:true 示例是静态域 needs_experiment 提案（无任何 set_points tool_call 示例）+③目标级规则句「target-level evidence in hand→return needs_experiment」与 v2 正面对撞。软预问三判定缺口：time range「持有」无判据（证据背书 vs 计划假设）/不确定无出口（needs_resolution 槽只在提案路线合法）/双确认语义邻接无桥（提案世界确认门 vs 直调世界不预问）。admitted 域表 9 静态域无 automation 域，r2 自创 section_automation=词表外 improvisation。分级：措辞迭代三项+设计裁决一项（时变是否入提案词表）+T2 一项，未写新措辞 ✓。
- **面三（continuation 前台吞）**：三轮 exchanges 完全同构亲证（首句→limit_reached/waiting_continue+checkpoint persisted→「继续」→`automatic_continuation_already_scheduled` 吞，goalrunner_chat.go:194-204）；终局结果 chat 面零渲染（三轮均无第三次 exchange，能力面零暴露载荷成立）；跨轮新 conversation_id 零 loop 继承（r2 continuation_used 自 0→4）。回执可见性结论=**单槽 latest-only+ledger 预算降级 cold ref，历史回执内容不可见不累积**（runner.go:589/610/638 单槽覆写+model_projection.go:174-230 降级；observation_ledger 降级标记载荷亲证）；r2 80 次观察（78×track.time_dynamics）持续不满足=追时间证据但只见 latest 一份 digest。附发现：预算句「每 needs_observation 轮耗一 checkpoint」与机械记账（resume 边界，goalrunner_chat.go:3361）错位——可见面夸大观察代价。T1=吞回执升级携带 pending 状态+终局结果补投递 chat 面。
- **未定位四处（如实标注，未以推测充锚点）**：①observation_ledger 镜像丢失点（载荷证明 checkpoint 持久化已无镜像，宿主 merge 均 clone 式保留未知键，候选=边界导入携带钉住前旧面）；②锁定时 vs 渲染时计数差（r1 渲染 4/6+5/6 但 reason=budget_critical）；③终局片预算常量 80 来源；④r2 记录 77 整请求异形分支。另两小项见报告 §3.4（time_dynamics 模型面 digest 是否含 clip 起止未逐轮核验；终局结果其他投递面未查）。
- 与卡面前提关系：无推翻性冲突；一处精确化——「241 同族」成立但机制非单槽逐出而是宿主 typed round-trip 键丢失，与裁定修正②兼容并落地代码层（报告 §1.3）。
- 纪律自查：越域零（port 分支 diff 仅报告；本提交仅卡 mv+回执）；git mv 最后执行且 mv 后立即 add ✓；验收门槛=勘察卡无烟测腿（卡面验收标准：报告三面锚点+证据+分级+因果链 ✓，零代码改动 ✓）。

## 验收

- **pass（报告合入 319e3444）**。主管亲核五承重锚点全属实：struct 无 AutomationSurfaces（全文件 grep 零命中）/fromAny:3401 Marshal→typed Unmarshal 标准丢弃/recordWrite loop["automation_surfaces"]/goalrunner:194-204 吞回执文本与载荷逐字/runner:589 单槽覆写；TERMINAL TURN 渲染格式与载荷亲合。
- 假说修正入册：主管裁定修正②「上下文驱逐」机制层定案为 **chat 宿主 run 边界 typed round-trip 键丢失**（相位触发），compaction 字节机器独立未触发——报告以载荷+代码双证修正主管假说细节、证实方向，勘察卡价值兑现。
- 零实现面/零 LLM 调用/未定位如实标注 ✓；载荷坐标口径（168 条分组+r2 通道=单槽非 ledger）与主管独立判读吻合。
- 产出转卡：245=面一 T1+面三 T1（chat 侧确定性双修复）；246=面二措辞四项（含设计裁决：主管裁定时变维持直调正道不入提案词表，依据 2026-10-31 vocabulary-full-retirement 裁定+automation 工具链设立初衷，用户可推翻）。

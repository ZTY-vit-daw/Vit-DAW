# WEBUI-MSG-ORDER-2：活态路径钉尾残留——A/B 判定卡/park 消息族钉流底，新输入与报错插其上（M1 复验实测，P1）

- 池序 23（P1 关键呈现缺陷）；目标仓库=D:\Vit_DAW（PC 执行侧，webui 域）；来源=M1 复验第三轮（[runs/M1-RETEST-20261001/FORENSIC-NOTE.md](../../runs/M1-RETEST-20261001/FORENSIC-NOTE.md) 症状 4+evidence/）
- 优先级 / 预估 / 依赖：P1 / 取证 0.5 天+修复 0.5 天 / WEBUI-MSG-ORDER-1 已合入（0e7ba86d/8e21258e——本卡是其声明的活态覆盖缺口的补面，非回退）
- 模型分级：L1 / GLM 首选（渲染序机制取证+活态断言面设计）；flash 可接若取证路径清晰
- 已核实事实：
  1. 用户目视形态（2026-10-01 18:3x，修复后二进制）：**第二句用户输入与 turn.failed 报错都显示在第一轮输出的上面**（第一轮链内容——轨迹块/A-B 判定卡/park 系列消息——钉在流底）。
  2. **服务端序正确**：会话图节点线性（ask→回执→ask→错误，evidence/conversation-graph.json 逐节点时间戳核对）——错位纯在 webui 活态渲染层。
  3. WEBUI-MSG-ORDER-1 修的两个机制（renderPlan 链终局钉尾=isChainResultChatMessage 族+turnGroups 同 run 用户行回吸）与本症状**不同族**：本轮第一轮内容里钉底的是 A/B 判定卡/park 交互消息（audition/pending-interaction 投递通道），不在 isChainResultChatMessage 谓词覆盖内（该卡回执已声明「活态 composer 驱动时序未真栈断言」——本卡即被打中的缺口）。
- 目标：
  1. **取证**：钉出 A/B 判定卡/park 消息族在活态路径的渲染与排序通道（哪个组件/哪条 pinning 逻辑把它们钉在流底；与 renderPlan/turnGroups 的关系——是独立 pinned lane 还是消息族谓词漏网）；领取时实锚（预期 App.tsx 消息流组装+audition/interaction 卡渲染段）。
  2. **修复**：该消息族按 createdAt 归位时序槽位（与既有两机制同语义：新消息在其后即插其上/旧内容回流入序）；不破坏 A/B 卡的交互可用性（点击/试听/judgment POST 不受排序影响）与 FIX-CONFIRM 的 settleTerminatedTurnInteractions。
  3. **活态断言补面**：E2E-WEBUI-1 加活态形态组（模拟 composer 驱动的新输入接在 park/判定卡之后——非纯事件重放；参照既有 driven-turn 组模式）——这是 WEBUI-MSG-ORDER-1 遗留的「活态无真栈断言」缺口的正式补面。
  4. **回归**：水合/重放组零回退（上卡 23 组含 msg-order-M1 必须仍绿）+新活态组红绿。
- 文件域：agent/webui/src/（消息流组装/park 判定卡渲染段+测试）+ scripts/webui_rendered_dom_smoke.mjs（加组）；如取证指向服务端事件缺回合域（audition 族无 turn 归属），按停止条件上交不越域。
- 约束：E2E 走隔离泊位先例（与并行真栈卡不冲突）；npm run test 全绿+build+E2E exit 0。
- 验收标准：新活态组红绿+既有 23 组零回退+用户手测复验（顺序正常）。
- 停止条件：取证发现钉底源于服务端事件缺 turn_id/回合域（webui 无法独活排序）→ 实证锚点上交，转 agent 侧卡。
- 领取：2026-10-01 19:1x / origin/main=245b795a（并行卡①领取提交后） / 分支 port/webui-msg-order-2（独立 worktree D:\Vit_DAW_wt_webui_msg_order_2，PC 会话②）
- 回执：实现 commit=d313eb72（port/webui-msg-order-2；领取提交 f0f857b7）
  - **机制锚点（取证结论，目标 1）**：A/B 判定卡钉流底=**独立 pinned lane**，非 renderPlan 谓词漏网——App.tsx MessageStream `unanchoredSessions`（旧判据=visibleMessages 的 turn_id 集合），卡片 turnID 取自 audition.ts:93（source_turn_id→session.turn_id→payload.turn_id），而内核 audition::Session 无 turn 字段（M1-RETEST evidence 八条 audition.* 事件 turn 域全空实证）→ turnID 恒空 → 永久流尾。第二通道=renderPlan `orphanTurnIds` 尾部追加（槽位被占 B9 / 无归属证据的实验块）。自由态实验块在 B9 统一面下折叠进 run 域回合（trajectory.ts trajectoryRoundKeyOfEvent 以 source_turn_id 为先），「park 消息族」的流内可见物=该折叠块内容+A/B 卡。**停止条件不触发**：free_state 轨迹事件带 payload.turn_id=turn:free_state_* 域且 audition session_id 内嵌原生域（audition:turn:<native>:round-*），webui 可独活排序。
  - **修复（目标 2，与既有机制同语义）**：① renderPlan 孤儿块时序插列——有 turnEventMeta.startedAt 证据的孤儿块按时刻插入组边界（组内最大消息时刻≤回合起始+TURN_SLOT_ANCHOR_TOLERANCE_MS 的最后一个组之后），无证据不猜保持流尾（旧调用点逐字不变）；② 判定卡挂靠三级化——组尾（既有）→轨迹块跟随（新增，卡与块同沉浮）→流尾兜底（不猜），turnID 空的会话按 session_id 原生域经 trajectory.nativeTurnIds 新账映射回 B9 轮次（messageLifecycle.auditionSessionNativeTurnID 解析）；③ sessionSuperseded 同步用解析后回合域。增量扩展 audition.session.startedAt / trajectory.turn.nativeTurnIds（纯归约、transient 态、无持久化兼容面）。行为语义变化（设计内）：用户越过待裁卡继续对话后，卡从「未沉淀」（原钉尾逃逸了 supersedes 判据）变为按既有 supersedes 语义沉淀「卡面选项未采用」——E2E 已断言该沉淀；试听/判定控件可用性由会话状态与既有守卫驱动，排序不触及 POST 通路。
  - **红绿 + E2E（目标 3/4）**：E2E-WEBUI-1 新组 **msg-order-M2**（活态形态，composer 驱动+事件缓冲中段追加，非纯事件重放；ui/state 网络层投影补丁清空挂载 transcript=PLANBAR-1 同款先例；round-2 行全部由真实 composer 产出）。RED（修复前 bundle）：verdict=fail、唯一败组=msg-order-M2——卡 flowIndex 7 钉在 u2(4)/报错(6) 之下=复验目视形态复现，其余 23 组全绿；GREEN：verdict=pass、24 组零失败——卡 flowIndex 2 回归其回合槽位（紧随 B9 折叠块、supersedes 沉淀条在场），u2/失败块/报错全部在其下。run roots=artifacts/e2e_webui1/webui-msg-order-2-red 与 …-green（互不覆盖，prereq.txt 含 head/二进制哈希/dist 哈希/agent_pid 全量记录；两轮 head=f0f857b7+工作树改动，受测源码与 d313eb72 提交 blob 逐一比对一致）。单测 +10（renderPlan 孤儿插列 4——M1 形态用例对修复前代码红后绿、trajectory nativeTurnIds 2、audition startedAt 2、session_id 解析 2），npm run test 393/393 全绿，npm run build exit 0。
  - **泊位声明**：两次 E2E 各用独立 run root（red/green 不覆盖）；HTTP 7899（每轮先验端口空闲）；桥端口经 VIT_AGENT_* 覆盖为 5601/5602/4500/4501（prereq.txt bridge_ports 记录），draft root=各 run 目录内 agent_drafts，agent 进程运行后即停（stopped_agent_pid 在案）；与并行卡①真栈（默认端口面）零共享、全程未触碰主工作树 D:\Vit_DAW。
  - **端测覆盖边界声明**：本卡改动过渲染面端侧烟测（E2E-WEBUI-1 全量 24 组 exit 0，含渲染面 DOM 断言与新活态组）；真栈三件套（VitApp 内核+Godot 前端）未在本卡拉起——按 AGENTS §5「真实运行栈」口径以隔离 VitAgent+真实浏览器+真实 bundle 执行，agent Go 代码零改动（E2E 所用 VitAgent.exe 由仓库工作树构建）；**用户手测复验（顺序正常）留待决策侧安排**，验收标准第三项未在本卡闭合。
  - **已知边界（不在本卡域）**：① 轮内时序细粒度——卡与其回合块按 UI-FOLLOW-1 槽位语义挂在开启用户消息之后，可能位于同轮更早的 receipt 行上方（轮间边界=本卡修复的承重面，轮内相对次序为既有槽位锚定语义）；② TRAJ-IMPL-3 水合收据行（orphanReceiptTurnIds）仍流尾兜底，若未来出现「水合收据被新消息超越」的同类形态，可按本卡同款时序插列扩展（候选后续卡）。
- 验收：**pass（2026-10-01 决策会话）**——rulings/2026-10-01-WEBUI-MSG-ORDER-2-pass.md；合并 main=ef8de3d4（决策侧 cherry-pick）；机制取证采信（独立 pinned lane+内核无 turn 域实证）+diff 直读（无证据不猜纪律全过）+我方复跑 393/393+build+E2E 红绿工件亲读（活态组 msg-order-M2 上线=上卡缺口正式补面）+泊位合规；手测复验与 M1 第四轮同场销项。

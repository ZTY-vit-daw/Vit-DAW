# Vit-DAW Agent GUI 实施计划（设计基线 → webui 实改）

> 依据：design/gui/direction-approved.md §5–§9 冻结的设计基线（bc 普通档 + bc-full 完全档）。
> 本文是对 `agent/webui` 现状的只读盘点 + 差距分析 + 分卡实施计划。写于 2026-09-03。

## 一、现状盘点（对话相关，只读核实）

### 数据层（比预期齐全，大部分语义已有后端支撑）

| 能力 | 现状 | 位置 |
|---|---|---|
| 权限两档 | `AuthorityMode = manual_confirmation \| full_project_access`，state/API/恢复全链路已有（`setAuthorityMode`，UI state + runtime status 双来源恢复） | `types.ts:2`、`App.tsx:228,662-668` |
| 轨迹事件模型 | `vit.observable_trajectory.v1`：turn/round/node 三层；node_kind 含 observation/hypothesis/action/rollback/user_judgment/settlement；status 含 running/waiting_for_user；materiality 含 human_audition_ready/human_confirmed/rolled_back/insufficient_dose | `trajectory.ts` |
| 任务运行轨迹 | `vit.task_runtime_trajectory.v1` 快照：semantic 状态机（needs_experiment/human_judgment_required/settled…）、invocation 切片、状态变更史 | `taskTrajectory.ts` |
| A/B 试听 | audition session/candidates（A/B 卡、采用/查看/停止）、judgment 协议 `heard_difference/preference/reason_tags/free_text` → settlement improved/rolled_back/needs_user_judgment | `audition.ts`、`TrajectoryAuditionPanel.tsx` |
| 免选路径（同步①） | proposal 卡已有 hint「可直接回复"执行这个方案"，也可以继续提问…」；`resolveSupersededMessages`/`resolveCompletedTurnProposals` 已把被取代 proposal 标记 completed（「卡面选项未采用」的数据语义已存在） | `App.tsx:4317`、`messageLifecycle.ts:200-271` |
| 消息种类 | message_kind: activity/user/assistant/proposal/execution_receipt/verification/warning/error/system + supersedes/logical_message_id 协议 | `types.ts:8`、`messageLifecycle.ts` |
| 即时活动 | AgentEvent 流 → transient activity 消息（approval.requested/item.completed/turn.completed 驱动增删） | `messageLifecycle.ts:298-332` |

### 呈现层（差距所在）

| 设计面 | 现状 | 差距 |
|---|---|---|
| ④ 轨迹（主角） | `TrajectoryAuditionPanel`＝**深蓝圆角浮层**（rgba(12,17,27,.94)，max-height 48vh）挂在消息流**上方**（流外）；内部 `TrajectoryView` 是审计文档式大视图（900px、大标题、卡片式节点）；即时思考是 `activity-lane` 蓝底左条 | 全部重做：去深色浮层、轨迹块内嵌对话流回合内、无框行式+节点状态、收起回执+完成自动收起、排队/接力态 |
| ② 判定卡 | audition 表单＝「能/不能/不确定」+「偏好 A/B/equal/neither/unsure」两问 radio + 原因 checkboxes + 自由文本 + 「记录判断(不会自动采用)」+ 独立「采用」按钮 | 重做为四裁决按钮+磁带行+round N/M 徽章+黄底回滚信任锚+settled 灰条沉淀；表单语义需映射（见讨论②） |
| ① 确认卡 | `CapabilityProposalCard`：黄底左边条卡，信息全（改动/风险/可回滚/分组/逐轨明细/证据覆盖），childActions 按钮，免选 hint 已有 | Mondrian 化重皮（白底+页签+硬投影+2px 墨边）+ 卡面补充输入行（.c-ask） |
| ③ 权限开关 | composer 里的英文 `<select>`（Manual Confirmation / Full Project Access），turn 运行时禁用 | 移到面板头部双档分段开关+中文化（普通/完全）+黄底副注 |
| 输入/输出分色 | Mondrian 覆盖后：user 气泡=**白底 1px 墨边**；输出卡=黄底左边条（proposal）；深蓝浮层另成一色 | 落实颜色契约：输入=蓝底框(#dbe6fc)+2px 墨边、输出=白底框+页签+硬投影、轨迹=无框 |
| 容器 | conversation-panel 是 workbench 弹性中列（消息 max-width 980px），非窄栏；CEF 内嵌时靠窗口宽度被动变窄 | 消息列 max-width 收敛到 ~720（BRIEF 无重排上限）；窄栏形态靠响应式自然达成（见讨论①） |

## 二、分卡实施计划（由易到难，每卡独立可验收）

**卡 T1 · 颜色契约与 Mondrian v2 皮层**（纯 CSS，styles.css 新增覆盖段，不动组件）
- user 气泡：白底1px → 蓝底 #dbe6fc + 2px 墨边；proposal/attention 卡去黄底左边条 → 白底+2px墨边+硬投影；
- 深色浮层去深色化（trajectory-live-panel 改纸面）；composer 纸面框化；activity-lane 改无框灰字行（临时，T2 并入轨迹）。
- 验收：webui 现有全测试绿 + 端侧烟测 + 截图对照设计 tokens。

**卡 T2 · 轨迹块内嵌对话流**（核心新组件，最大工程）
- 新组件 `TraceBlock`（收起回执+展开无框行式+节点四态：完成/直接执行/spinner/排队；步骤可展开细节；完成自动收起）；
- MessageStream 按回合分组：user 消息 → 该回合 TraceBlock（消费 trajectoryState turn/round/node + activity 事件并思考行）→ 卡片；
- `TrajectoryAuditionPanel` 拆解：轨迹部分由 TraceBlock 取代，audition 部分留待 T3 改造为判定卡入流。
- 验收：真实栈跑一轮 free_state 改善（scripts 烟测模式），轨迹实时性/收起/回看可用。

**卡 T3 · 判定卡重做**（依赖 T2 回合分组）
- audition session → 白底页签判定卡：round N/M、A/B 磁带行（复用 auditionDawTarget 播放）、黄底回滚徽章、四裁决按钮、settled 灰条；
- `.c-ask` 卡面补充输入 + 免选让位呈现（复用 supersedes 语义，UI 补「卡面选项未采用」灰条）。
- 验收：A/B 试听→四裁决→沉淀全链在真实栈走通；映射讨论②定案后实施。

**卡 T4 · 确认卡重皮 + 权限开关头部化**
- CapabilityProposalCard Mondrian 化（信息结构不动，只重皮+ .c-ask + 免选 hint 保留）；
- authority select → 头部分段开关（普通/完全）+ 黄底副注（完全档常显）；完全档下确认卡让位逻辑与 bc-full 语义对齐（proposals 不再弹出——后端 needs_confirmation 行为需核）。
- 验收：两档切换在真实栈生效（API 已有），完全档统一链呈现。

**卡 T5 · 打磨与全状态回归**
- 两条微观察（对比度半档/间距节奏）、reduced-motion、键盘可达、历史会话回放轨迹折叠态、720 无重排检查。

## 三、讨论定案（2026-09-03 用户裁定）

1. **容器形态 → 弹性宽列**：按 webui 现有弹性宽列设计（主流形态），不做显式窄栏模式；消息列宽度策略（是否收敛 max-width ~720）在 T1 实施时定。
2. **判定卡裁决 → A/B 两选项 + 自定义补充**（关键定案）：不做四裁决按钮。两个大按钮「A 更好 · 回滚」「B 更好 · 保留」+ 卡面补充输入行。后端映射直达成现有协议：A→preference=a（settlement rolled_back）、B→preference=b（improved）、补充→free_text；直接选 A/B 隐含 heard_difference=yes。「听不出」等细粒度经补充输入表达。设计稿已同步改版（direction-bc*.html，preview-bc-ab-judge.png）。
3. **表单取舍 → 原因标签+自由文本折叠进补充输入**：TrajectoryAuditionPanel 的两问 radio 表单废弃，主路径=A/B 按钮，补充输入承载细粒度。
4. **activity-lane**：「细线」指 MessageStream 底部的即时活动条（`App.tsx:4036` `.activity-lane`，样式 `styles.css:1583`——蓝左边条+spinner+跳动圆点的细横条），显示上传/调用中等瞬时进度。回合内思考并入轨迹块后，非回合类活动（上传进度、无轨迹事件的调用）仍需显示位——**建议保留该条并改造成无框安静样式（仅限非回合活动）**，待用户确认。

## 四、原待讨论存档（已被 §三 定案取代）

1. **容器形态**：CEF 内嵌侧栏宽度由 Godot 决定，webui 无独立「窄栏模式」。按响应式处理（消息列收敛 max-width 720）即可，还是需要显式 embedded 模式（隐藏 SideRail/焦点面板等）？需核实 Godot 前端实际嵌入方式与宽度。
2. **四裁决 → 后端映射**：现有协议是 heard/preference 两问。初步映射：听得出·保留=(yes,b→improved)；听得出·更糟·回滚=(yes,a→rolled_back)；听不出·下一轮加码=(no→insufficient_dose→increase_dose)；**太过了·回退一半**=现有协议无对应（需 agent 侧新能力或降级为「回滚后人工调」）。是否先行三裁决、half 待后端？
3. **判定表单信息取舍**：现有原因 checkboxes + 自由文本比设计的四按钮信息丰富；建议收进「补充说明」折叠（与 .c-ask 合并），四按钮为主路径——确认取舍。
4. **activity-lane 去留**：其职责（transient 活动）并入轨迹思考行后，非回合类活动（上传/调用进度）是否保留一条细线？

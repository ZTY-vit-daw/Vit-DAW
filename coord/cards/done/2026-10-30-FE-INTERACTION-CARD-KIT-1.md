# FE-INTERACTION-CARD-KIT-1：通用交互卡控件集（声明式 fields+actions）——编舞确认卡迁移，终结特制分支链

- 发卡：GLM 主管决策流 / 2026-10-30 晚（依据=用户裁定②确认按钮走通用控件+用户架构质疑「交互卡片应该是一套控件现填的」；主管查证：webui App.tsx:6284+ 为启发式确认判定+场景特制分支链，automation_choreo_confirmation 零渲染分支=退化纯文本无按钮，确认靠话术回「确认写入」）
- 派发确认：已确认（用户 2026-10-30「②确认按钮走 224 通用控件——肯定的」+「可以开卡了」）
- 验收负责人：GLM 主管决策流
- 池序 224；目标仓库=**主仓（agent 侧卡构造+webui 渲染）**；P1 / 1 天 / 依赖=无（与 223 并行，文件域不相交）；L2（渲染架构+DOM 断言建设）

## 标的（主管已查证）

- webui 现状：`isConfirmationAction` 七八条启发式（kind/type/status 子串/plan_id 在否）+`isMixTreatmentPendingAction`/`isComposerInteraction`/`mix_session_entry` 等场景分支——每卡手搓，新增卡=新增分支，漏写=静默退化（编舞卡现状）。
- agent 侧现状：编舞确认卡 payload 已半声明式（218 六可编辑字段）但无 actions 声明，webui 无渲染器。

## 目标

1. **声明式卡契约**：payload schema 化 `fields: [{type: text|number|range|select, label, value, min/max, unit}]` + `actions: [{label, action_id, style: primary|secondary|danger}]`——编舞确认卡先行迁移（六字段+确认/取消两动作；确认动作回传 action_id+字段编辑值，与既有话术确认链等价并存）。
2. **webui 通用渲染器**：一个组件按声明画卡（字段控件+动作按钮组）；点击回传 action_id/字段值；**编舞卡经通用渲染器出确认按钮**（用户手测断言点）。特制分支链不强制本期清空（渐进退役路径写进回执），但新卡一律走通用契约。
3. **渲染面端侧烟测**（§5 门槛）：浏览器级 DOM 断言——卡渲染（字段数/类型/按钮在场）+点击链路（按钮→请求载荷含 action_id+编辑值→确认卡收敛/写入生效）。E2E-WEBUI-1 建设卡若未就绪，按 §5 显式声明端测覆盖边界交主管裁定。
4. 既有卡零回退：mix/plugin/session 等特制分支行为不变；话术确认链（「确认写入」）保持可用（通用按钮为增不是替换）。

## 探针义务

webui vitest（组件级）+浏览器级 DOM 断言（Playwright 体系，B2/RACK-VIZ 先例）双面；真栈腿：真实栈 webui 编舞话术→确认卡**按钮点击**→写入生效（与话术确认对照轮）。

## 文件域

`agent/internal/chat/automation_choreo.go`（**仅 automationChoreoConfirmationResponse 函数族**——卡 payload 构造；clarify 函数族属 225 域，函数级分界 2026-10-30 裁定）+`agent/webui/src/**`+测试+探针。领取顺序约束：先于 225 领取/提交（同文件函数域分界的前置）。越域即停。

## 停止条件

- 声明式契约需动 webui 之外的渲染基建（如 App.tsx 12k 行拆分预工程）→锚点上交另议；
- 按钮回传与既有 RiskConfirm 确认链冲突（action_id 路由与话术路由打架）→锚点+两案上交。

## 领取

- 时间：2026-10-30 晚
- owner：ZCode GLM-5.3 PC 执行会话（224 卡会话）
- 分支 / worktree：`port/fe-interaction-card-kit-1` @ `D:/Vit_DAW_wt_ick1`
- 领取基线：worktree HEAD=`bc7ee6a2`（=发卡提交；本地 main 领先 origin/main `52148fe5` 6 个主管提交，随本领取提交一并 fast-forward 推送）；worktree status 干净、diff --stat 空
- 主检出既存改动核对：`VitApp/Workspace/*`（运行时状态）+`coord/runs/UX-AB-AUDITION-IMPL-1/*`+卡移动三项——均在本卡文件域（automation_choreo.go 函数族+webui/src+测试探针）之外，不触碰
- 文件域核对：doing 池仅 GEN-SUNO-WEB-DESIGN-1（设计卡，无冲突）；223/225 均在 todo 未领取——本卡先行领取符合「先于 225」约束
- 真栈错峰：与 223（M3-CHAT-AUTH-STAMP-1）错峰，届时按 §2.2 登记 PC-RUNTIME-STACK.md（当前记录最新=221 卡 2026-10-30 20:0x 释放）
- 领取提交：`3036783d`（已推 main 远端亲读）

## 回执（2026-10-30 晚，执行侧自验完成待验收）

**分支**：`port/fe-interaction-card-kit-1` @ `a0dc4d70`（已推远端；claim `3036783d` → impl `45693031` → probe `e5ae6cee` → stack 工件 `a0dc4d70`）

**实现三件与 diff 锚点**：
- ①声明式契约（agent 侧，函数域内）：`agent/internal/chat/automation_choreo.go` 新增 `automationChoreoDeclarativeCard`（紧随 `automationChoreoPointsSummary`，卡 payload 构造族）+ `automationChoreoConfirmationResponse` 的 Payload 增 `card` 键——`interaction_card.v1`：六字段（shape:select 带 options / start·end·region×4:number / curve:range，min/max/step/unit 齐备）+approve/cancel 两动作（action_id/label/style）。min/max 只在 server 编辑校验真实执行的边界声明（curve[-1,1]、region≥0+跨度上限、level raw[0,1]=201/217 内核已证），space/tone 插件类参数域插件专属→留 description 不虚报边界。既有 Fields/Actions 双数组保留=话术确认链与旧渲染面零破坏。
- ②webui 通用渲染器：新文件 `agent/webui/src/interactionCardKit.tsx`（+364）——`declarativeCardFromAction`（契约提取+校验：缺 card/无 action 判非声明卡回落原分支；未知 type→text、未知 style→secondary 降级不失败）+`DeclarativeInteractionCard`（四型字段控件 select 选项钮/number 带 min/max/step+unit/range 滑条+读数/text；settle 只读态已确认/已取消/已处理；按钮组带 `data-action-id`/字段带 `data-field-id/type`/卡带 `data-card-schema` DOM 锚点；点击回传 action_id+`payload.fields` 全字段编辑值——与 server `automationChoreoApplyFieldEdits` 六键咬合）；接入点=`App.tsx` ActionCard 分发（特制五分支后、StandardActionCard 前，仅 payload.card 在场者走此路=既有特制卡零回退）+`export ActionCard`（测试面，CapabilityProposalCard 先例）；`styles.css` 补 danger 钮+range/input 行内布局。
- ③探针（生产零触碰）：`scripts/automation_card_button_smoke.py`（+505）+`dev_agent_smoke.ps1` 三处（`-AutomationCardButtonSmoke` param+共享工程隔离段条件+腿块）。

**测试矩阵（全绿）**：
- vitest `npm run test`：39 文件 **471/471**（新增 `interactionCardKit.test.tsx` 12 项：契约提取 4+回传载荷 2+渲染 3+分发接入 3 含 mix/synthetic 特制卡零回退双向断言）
- `go build ./...` exit 0；`go test ./internal/chat/` 全量 exit 0（106s）；`tsc --noEmit`+`vite build` exit 0
- **浏览器级 DOM 断言 8 条全 PASS**（真实浏览器 Edge+真实 fetch→vite 代理→mock agent；`coord/runs/FE-INTERACTION-CARD-KIT-1/dom_assert_20261030_1/RESULTS.md`）：D1 卡渲染（schema+标题+待确认）D2 六字段类型 D3 select 选中态 D4 approve/cancel 按钮 D5 字段编辑 0.55 D6 点击→POST /agent/interaction/respond 载荷=interaction_id+action_id=approve+payload.fields 六键含编辑值（requests_log.jsonl 原文）D7 卡收敛（dismiss+无残留按钮+回执入流）D8 cancel 腿 action_id=cancel
- **真栈腿（§2.2 登记 ab200578/释放 6497547c，与 223 错峰兑现）正式轮×2 连续全绿**：`stack_button_20261030_3`+`_4`（后者 ps1 真实退出码 **TRUE_EXIT=0**）。B1 真实浏览器主断言=真实 webui 页话术→卡（六字段+按钮）→编辑 0.20/0.80/c-0.25→点确认写入→已写入 settle→**内核 get_curve 逐点 (0,0.20,c=-0.25)/(24,0.80)=编辑值写入生效**；R2 话术「确认写入」对照轮并存（written=2+(45,0.30)/(60,0.60) 缺省渐入）；R3 responder cancel 零写入。迭代轮 _1/_2 均探针面缺陷（python 类型注解/vite ANSI regex+进程树拆栈）如实留证，被测链零失败。隔离亲证+kernel C1E32DFE 零漂移+拆栈全零，详见 `coord/runs/FE-INTERACTION-CARD-KIT-1/RESULTS.md`。

**端测覆盖边界声明（§5，交主管裁定）**：
1. 浏览器面已覆盖=真实浏览器 DOM 渲染+点击链路+真实 agent（治理写入+内核逐点核对）全链；**未覆盖**=CEF 宿主管线（agent 不托管 webui dist，真栈腿 webui 由 vite dev 托管——导出 dist 的 release 装配属 Godot 前端域）与历史刷新重放 settle 卡的浏览器面（该面由 vitest renderToStaticMarkup 断言覆盖）。
2. E2E-WEBUI-1 建设卡未就绪，本卡按卡面指示以 playwright 先例形态执行并留工件；mock-agent 腿的 server 侧回执为 canned（写入生效性由真栈腿覆盖）。
3. 渐进退役路径（卡面目标 2）：特制分支链未清空——`isConfirmationAction` 等启发式保持原样；后续卡迁移=卡构造侧加 `payload.card` 即走通用渲染器，无需 webui 改动（本卡已验证零回退路径）；建议后续卡：mix_treatment_confirmation→通用契约迁移、confirmationFallback 合成卡改声明式。

**停止条件核对**：①未触发（渲染基建未越 webui 之外）②未触发（action_id 路由与话术路由经 R2 对照轮实证并存不打架）。

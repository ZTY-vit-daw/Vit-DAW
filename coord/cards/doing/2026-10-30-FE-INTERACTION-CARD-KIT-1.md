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
- 领取提交：（推送后回填）

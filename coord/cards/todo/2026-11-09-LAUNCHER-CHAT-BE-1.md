# LAUNCHER-CHAT-BE-1：launcher 流意图路由+directive 回执+开场白（332 拆卡 C4，agent 域）

- 发卡：GLM 主管决策流 / 2026-11-09；派发确认：已确认；验收负责人：GLM 主管决策流
- 池序 337；P1 / 1.5 天 / 依赖=零（与 335 并行，契约 JSON 冻结）；L2 / PC；零栈单测+真栈腿并入 339

## 目标

1. chat 请求 `context.launcher=true` 分支（`agent/internal/chat/launcher_flow.go` 新 ~200 行）：工程意图识别→`project.recent` 匹配→唯一命中出 `launcher_directive` 回执/多命中出结果卡（ProjectResultCards 先例）/无命中说明+追问——三分支；
2. 门厅简版开场白（在场问候+最近工程一条+「想从哪开始？」——确定性模板优先，LLM 渲染可选）；
3. `/agent/state` 增 `current_project{path,uuid}` 两字段（窗徽章对账+降级自愈，~10 行）；
4. 无工程态 chat 冒测加固（F11 语义回归）+`launcher:true` 不改 authority_mode 断言（零 authority 代码改动红线）。

## 文件域

`agent/internal/chat/launcher_flow.go`（新）+`server.go`（回执字段+state 披露 ~30 行插点）+测试；**不触** authority/pendingmanager 域。

## 验收标准

三分支表驱动单测+无工程态回归+authority 不变断言全绿；§8 口径成文（真栈腿含 LLM 轮，确定性退路不依赖 LLM）。

## 领取/回执/验收

- （执行侧/主管回填）

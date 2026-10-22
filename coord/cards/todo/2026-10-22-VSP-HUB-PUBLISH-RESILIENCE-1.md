# VSP-HUB-PUBLISH-RESILIENCE-1：VSP hub 实时发布停摆韧性与 GUI 写通知断供兜底（171 上交锚点卡）

- 发卡：GLM 主管决策流 / 2026-10-22 深夜（依据=171 断点定位上交：用户手测悬空窗根因=VSP hub 停摆 2m16s 致写通知断供——vsp_hub_last.log 19:43:12→19:45:56 零事件+agent_last.log 同刻 `VSP realtime publish pending failures Post http://127.0.0.1:8787/vsp: context deadline exceeded`+agent 双发布者 19:46:52 重注册（旧连接断）；工件=coord/runs/FE-SEND-UI-LIVE-CHAIN-1/RESULTS.md §五+主检出 Logs）
- 派发确认：已确认（171 停止条件命中上交，主管裁定拆本卡）
- 验收负责人：GLM 主管决策流
- 池序 173；目标仓库=D:\Vit_DAW（agent/vsphub 面）；**P1（主旅程可感面：偶发故障直接吞掉 AI 操作的 GUI 可视化——M2 演示风险项）** / 0.5-1 天 / 依赖=无；模型分级：L2 / GLM 执行会话

## 范围（第一步=停摆根因取证，第二步=韧性修复）

①取证：hub 停摆 2m16s 的根因（vsphub 进程阻塞/重启/端口竞争/内存？——19:43-19:46 窗口的 vsphub 日志+Windows 事件+agent 发布者重连证据链）；②恢复慢机制（为何 2m16s 后才重注册——重连退避策略？）；③韧性修复双面：hub/agent 侧（发布失败重试+健康监测+快速重连）+GUI 断供兜底（写通知断供窗口内前端可感知降级或主动拉取兜底——与 161/171 拉取链协调，注意防轮询化）；④防回归腿（停摆注入复现：模拟 hub 不可达→写通知断供窗口量化→恢复后零丢失）。

## 文件域

以取证结论固定（预期 `agent/internal/vsphub`/`agent/internal/vspclient`/发布重试面；前端兜底若涉前端仓另注）。开工时写入卡面。越域即停。

## 验收标准

单测+真栈注入腿（hub 不可达注入→agent 行为+GUI 断供窗口→恢复后事件零丢失）；既有 vsphub/agent 测试零回退；工件入主仓 coord/runs/VSP-HUB-PUBLISH-RESILIENCE-1/。端测边界声明。

## 停止条件

停摆根因在内核或系统层（非 agent/hub 可控面）→ 锚点上交再裁定兜底边界。

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（根因结论 / commit / 真栈 run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）

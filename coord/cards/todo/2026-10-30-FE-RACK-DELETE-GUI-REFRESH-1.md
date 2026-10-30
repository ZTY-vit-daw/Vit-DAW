# FE-RACK-DELETE-GUI-REFRESH-1：删除回拉第三跳断裂——legacy get_project_state 未发出（UDP 串行队列竞争候选）+已删插件轮询不停

- 发卡：GLM 主管决策流 / 2026-10-30 晚（依据=212 用户手测重验失败现场取证，[repro_20261030_user_gui/REPORT.md](../../runs/FE-RACK-DELETE-REDRAW-1/repro_20261030_user_gui/REPORT.md)——主管亲读三方日志+前端代码定链）
- 派发确认：已确认（用户 2026-10-30 晚报障「删除后卡片保留在 GUI 画布节点」即诉求；现场取证闭环）
- 验收负责人：GLM 主管决策流
- 池序 221；目标仓库=**前端仓 D:/Godot/project/vit-daw-frontend**；P1 / 0.5-1 天 / 依赖=无（212 收口挂起待本卡）；L1-L2（插桩定案面 L2 判读，修复面 L1 量产可做）

## 断点定案（取证已闭环，执行侧不重考）

删除链：内核 ok+落盘 ✓ → dock 回执回拉 ✓ → VSP snapshot 207 守卫回落 ✓ → store delta 208 守卫回落 ✓ → **legacy get_project_state 未到达内核（两次删除间 ZmqGateway 全量核对零次）** → store error → dock 静默 return → repo/project_loaded/画布全链停摆（Vit_Graph_Rack.gd:7632 注释明示画布无本地回退）。

## root cause 候选（插桩定案，按优先级）

1. **UDP 串行队列竞争（主候选）**：vit_ipc_client.gd:126 `_process_queue` 在 `_active_request` 非空时零调度（严格单在飞）；envelope lane 周期轮询 `env_lane_params_*`（track_envelope_lane_2d.gd，param 族 30s 超时）与回拉 legacy（8s 软超时）同队列——排队未发出即被软超时丢弃。
2. await 轮询挂死（send_command_async_await:375 while pending 轮询，request 字典引用断裂形态——低概率）。
3. `_udp` null 态（udp_not_ready 即时 error——低概率，同 socket delete_plugin 成功）。

## 目标（三面）

1. **插桩定案**：真栈窗口形态复现（拖插件→参数面板/车道激活→删除→观察画布），插桩面=UDP 队列状态（active/depth/软超时丢弃计数）、await 返回值、store legacy 是否入队。推翻主候选则锚点上交两案，不得盲修。
2. **修复**（按定案收敛，三方向权衡执行侧带证据选）：
   - a. 队列面：未发出请求不受软超时丢弃（发出后才起表）或刷新类命令提权/防饿死；
   - b. 回拉面：dock 回拉链失败时重试/兜底（guard error + legacy error 双败时不得静默——至少 request_failed 可观察）；
   - c. 堵塞源：envelope lane 对已删插件停止轮询（track_envelope_lane_2d.gd 消费删除事件）。
3. **回归**：212 双向量删除链（KEY_DELETE+agent）回归零回退；208/207 守卫测试族零回退；envelope lane 轮询既有测试零回退。

## 探针义务

真栈窗口形态（非 headless——竞争窗口依赖 envelope lane 轮询与参数面板在飞，取证已证 headless 探针覆盖不到此面）：复现「拖插件→删除→画布节点消失」用户旅程断言；插桩读回面（队列丢弃计数/await 结果）入 RESULTS。既有 stack_probe_rack_delete_redraw 扩展或同模式新增。

## 文件域

前端仓 `app/kernel/autoloads/vit_ipc_client.gd` + `app/state/stores/project_store.gd` + `vit_dock/scenes/vit_dock_root.gd` + `app/tracks/track_scene/track_envelope_lane_2d.gd` + 测试/探针。修复方向未选前不越域；若定案落在域外（如 rack 面）→锚点上交。

## 停止条件

- 插桩推翻主候选且三个方向均不适用→锚点+证据上交，不盲修；
- 修复需动内核或 agent→上交。

## 领取

（待执行侧领取：记录 HEAD / status --short / diff --stat 基线；前端仓独立分支+worktree，按 PROTOCOL §2.2 登记真栈占用）

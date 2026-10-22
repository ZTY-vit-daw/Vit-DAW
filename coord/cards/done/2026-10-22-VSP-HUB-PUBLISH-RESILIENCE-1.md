# VSP-HUB-PUBLISH-RESILIENCE-1：VSP hub 实时发布停摆韧性与 GUI 写通知断供兜底（171 上交锚点卡）

- 发卡：GLM 主管决策流 / 2026-10-22 深夜（依据=171 断点定位上交：用户手测悬空窗根因=VSP hub 停摆 2m16s 致写通知断供——vsp_hub_last.log 19:43:12→19:45:56 零事件+agent_last.log 同刻 `VSP realtime publish pending failures Post http://127.0.0.1:8787/vsp: context deadline exceeded`+agent 双发布者 19:46:52 重注册（旧连接断）；工件=coord/runs/FE-SEND-UI-LIVE-CHAIN-1/RESULTS.md §五+主检出 Logs）
- 派发确认：已确认（171 停止条件命中上交，主管裁定拆本卡）
- 验收负责人：GLM 主管决策流
- 池序 173；目标仓库=D:\Vit_DAW（agent/vsphub 面）；**P1（主旅程可感面：偶发故障直接吞掉 AI 操作的 GUI 可视化——M2 演示风险项）** / 0.5-1 天 / 依赖=无；模型分级：L2 / GLM 执行会话

## 范围（第一步=停摆根因取证，第二步=韧性修复）

①取证：hub 停摆 2m16s 的根因（vsphub 进程阻塞/重启/端口竞争/内存？——19:43-19:46 窗口的 vsphub 日志+Windows 事件+agent 发布者重连证据链）；②恢复慢机制（为何 2m16s 后才重注册——重连退避策略？）；③韧性修复双面：hub/agent 侧（发布失败重试+健康监测+快速重连）+GUI 断供兜底（写通知断供窗口内前端可感知降级或主动拉取兜底——与 161/171 拉取链协调，注意防轮询化）；④防回归腿（停摆注入复现：模拟 hub 不可达→写通知断供窗口量化→恢复后零丢失）。

## 文件域

已按取证结论固定（2026-10-22 22:1x 机器钟开工写入）：`agent/internal/vsphub/`（websocket/telemetry/realtime/hub/diagnostics/config+hub_test）、`agent/internal/bridge/vsp_realtime.go`+新增测试文件；前端兜底涉前端仓另注：`app/kernel/autoloads/vsp_realtime_adapter.gd`、`app/kernel/autoloads/telemetry_manager.gd`（D:\Godot\projectit-daw-frontend worktree）；真栈注入腿新增 `scripts/vsp_hub_resilience_smoke.ps1`（按现有 ps1 体系模式）；工件 `coord/runs/VSP-HUB-PUBLISH-RESILIENCE-1/`。越域即停。

## 验收标准

单测+真栈注入腿（hub 不可达注入→agent 行为+GUI 断供窗口→恢复后事件零丢失）；既有 vsphub/agent 测试零回退；工件入主仓 coord/runs/VSP-HUB-PUBLISH-RESILIENCE-1/。端测边界声明。

## 停止条件

停摆根因在内核或系统层（非 agent/hub 可控面）→ 锚点上交再裁定兜底边界。

- 领取：2026-10-22 22:1x 本会话机器钟 / origin/main=9b79ceb3a7ccc8af85624eae02704f0732df988d（领取前 fetch 亲核=当前 main HEAD） / owner=GLM-5.3 L2 PC ZCode 执行会话（173 卡会话，机器=PC） / 分支=port/vsp-hub-publish-resilience-1 / worktree=D:/Vit_DAW_wt_vspr1（协调分支 coord/vspr1-claim@D:/Vit_DAW_wt_vspr1_coord） / 领取提交=本 commit
- 回执：**2026-10-22 23:0x 机器钟执行侧自验完成，done 待主管验收**
  - **根因结论**（全链证据在卷=coord/runs/VSP-HUB-PUBLISH-RESILIENCE-1/FORENSICS.md）：19:43 停摆≠hub 进程崩溃/重启/端口竞争（hub 全窗单次 start、kernel 命令面窗内健康，均亲核排除）——真实机制=**R1** `StreamClient.Write` 无写超时（websocket.go:136，对端停读即无限期阻塞）×**R2** telemetry relay 单 goroutine 逐流同步写（telemetry.go:73，一僵流断供全部 GUI 写通知；delta_update 自 hub 独占交付，bridge 已停 legacy UDP）×**R3** realtime.publish HTTP 处理器内同步写流（GUI 僵流反堵 agent POST→2s deadline→SessionID 清空→19:46:52 双发布者重注册=171 所记现象的机制）；触发面=**R4** Godot 每帧 ≤64 条 ws 消息×导入 delta 风暴（19:43:12）TCP 背压；恢复慢=**R5** GUI 无断供感知（seq gap 只在"有 delta"时可见，静默断供与空闲不可辨）+agent 发布者失败即丢帧。2m16s=Godot 消化积压时长。
  - **修复**：hub=每流有界出站队列(4096)+独立 writer+写超时 3s+满/失败驱逐 fail-fast+10s 应用层心跳+Publish/replay 全非阻塞+/health 增 relay 存活与逐流投递诊断；agent=发布者失败保留 payload 退避重试（250ms→2s、预算 110s、latest-only 替换、重注册保留）；GUI（前端仓）=ws watchdog（15s 零包含心跳=stale 沿一次信号→telemetry_manager 复用 161/171 防抖拉取链收敛一次，30s 硬阈值强制断开走既有重连+重订阅；非轮询）。
  - **commit**：agent 仓=port/vsp-hub-publish-resilience-1@**e06d54a7**（含 6+2 单测+注入脚本+工件）；前端仓=port/vsp-hub-publish-resilience-1@**ae36c55**。栈占用登记 2928a92a/释放 78622cca。
  - **测试与真栈**：单测 8 项新增全绿+`go test ./...` 全量零 FAIL；真栈注入腿 **inject_run1 两连净 exit 0**（8 检查：kill vsphub 12s 注入→探针断供连接即断→hub 同构建重启→重连 ~1.1s→恢复点后 78 条 delta seq 连续零缺口→get_project_state 收敛→relay/streams 诊断在场→agent 全程存活）；隔离腿 suspend 注入（真 adapter headless+真 hub）：stale 信号精确 15s、硬重连 30s、恢复后重连成功+hub write_failed 驱逐亲核；adapter 直测探针（真 kernel+hub）3/3 PASS。
  - **端测边界声明**：①注入腿 GUI 消费面=PS ClientWebSocket 探针（同 relay→ws 路径同协议，非 Godot 进程）；Godot adapter 行为由隔离腿+直测探针分面覆盖。②agent 发布者零丢失断言在单测面（fake hub 断供→恢复同帧送达）——headless 无播放故真栈无 transport/levels 帧流，发布者腿真栈断言留待播放旅程（归主管裁定）。③K3 289/popup 111 未跑（改动非渲染面非 sendroute 域；headless 启动零脚本错误+adapter 探针过；补跑归主管裁定）。④官方会话 vit.agent.official hub 重启后不重注册（cmd/vitagent 在卡面文件域外未动；发布者/GUI 链已自愈）——遗留边界。⑤渲染面：无布局/视觉改动（stale 仅信号+状态字段+warning 日志），渲染面烟测不适用。
- 验收：**整卡通过**（GLM 主管决策流 / 2026-10-22 深夜终验）。核对：FORENSICS 根因链六条逐条对源码锚点亲核（R1 websocket.go:136-143/R2 telemetry.go:62-79/R3 realtime.go:439-505/R4 GUI 风暴触发/R5 无感知+恢复慢/R6 进程未重启排除——时间线与 171 工件互证）；diff 亲读（写超时+驱逐+有界队列独立 writer+心跳+退避重试+watchdog 挂既有防抖拉取非轮询）；主管复跑 vsphub/bridge 单测 ok+集成态全量 go test exit 0；真栈注入腿两连 exit 0+隔离 suspend 腿+adapter 探针 3/3 亲核。端测边界三条裁定：②发布者零丢失断言留播放旅程=接受；③K3/popup=主管已在合并态补跑 289 PASS；④official 会话不重注册=遗留边界注记 gate（非本卡阻塞）。验收 commit=主仓 **9522c1d3**+前端 **eee2fcd**。

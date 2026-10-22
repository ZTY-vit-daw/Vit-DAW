# VSP-HUB-PUBLISH-RESILIENCE-1 (173) 第一步停摆根因取证

- 取证人：GLM-5.3 L2 PC ZCode 执行会话（173 卡） / 2026-10-22 22:0x–22:4x 机器钟
- 基线：origin/main 9b79ceb3（领取提交 1c76ee4c）
- 证据源：主检出 `VitApp/Workspace/Logs/`（vsp_hub_last.log mtime 19:46:52 / agent_last.log mtime 19:47:32 / MaterializeMetrics.log / VitHeadlessServer2026-10-22_19-42-42.log）+ 171 工件（coord/runs/FE-SEND-UI-LIVE-CHAIN-1/RESULTS.md）+ 两侧源码（agent/internal/vsphub、agent/internal/bridge、agent/internal/vspclient、agent/internal/kernel、Godot vsp_realtime_adapter.gd / telemetry_manager.gd）

## 一、时间线重建（全部亲核，+0800 本地=内核 UTC+8）

| 时刻 | 事件 | 证据 |
|---|---|---|
| 19:42:42 | 栈起：VspHub 单次 start（监听 8787），GUI realtime_adapter ws 连入（stream_.._1，**本会话唯一一次 ws 连接**） | hub log 行 1-4 |
| 19:42:43 | agent 三会话注册（official+transport_publisher+realtime_publisher） | hub log 行 6-8；agent log 行 9 |
| 19:43:12 | **stems 导入 delta 风暴**（6 stems × 每轨 ~50 条 node/property delta，agent shadow 同刻成批消费）——GUI 收到此批后 hub 事件静默开始 | agent log 行 45-104；hub log 行 10 后进入零事件窗 |
| 19:43:28/53 | MaterializeMetrics finalize 1-4（agent→kernel 正常，证明 kernel 命令面在窗内健康） | MaterializeMetrics.log |
| 19:43:12→19:45:56 | **hub 日志零事件窗（2m44s）**：无注册、无 stream 事件 | hub log 行 10→11 |
| 19:44:32-37 | 用户话术写 track.send.connect(1032→Reverb Bus)：kernel 收令（82120 行）、delta 流出（agent 自有 SUB 5556 正常消费至 19:44:37）——**GUI 经 hub relay 一条未收** | kernel log 82115-82125；agent log 400-408 |
| 19:44:36 | agent chat 完成 + checkpoint 完成（kernel 往返毫秒级） | agent log 405-408 |
| 19:45:55 | 用户手动 solo 1032（GUI 主线程恢复交互的最早证据） | agent log 19:45:55 delta |
| 19:45:56 | gui transport adapter 注册成功（HTTP 面+hkernel 转发恢复） | hub log 行 11 |
| 19:46:50-52 | **两个发布者首个 POST 失败**：`Post http://127.0.0.1:8787/vsp: context deadline exceeded`（2s 客户端超时）→ SessionID 清空 → 250ms 后 Hello 成功 → **19:46:52 双发布者携新 session_id 重注册**（171 所记"重注册"=此机制，非进程重启） | agent log 435-436；hub log 12-13；vsp_realtime.go:70,117,121 |
| 19:46:52 | gui godot.gui 主适配器注册 + get_project_state 拉取（GUI 拉取腿接管收敛） | hub log 14；agent log 438 |
| 此后 | **本会话 ws 再未重连**（hub log 无第二条 stream connected，文件止于 19:46:52.825） | hub log EOF |

用户可感断供窗（写 19:44:32→GUI 收敛 19:46:48±）≈2m16s，与 171 记录一致。

## 二、根因链（代码锚点逐条）

**R1（hub 缺陷·写无期限）**：`StreamClient.Write` 直接 `conn.WriteMessage`，无 `SetWriteDeadline`——对端停读/TCP 背压时**无限期阻塞**。
agent/internal/vsphub/websocket.go:136-143

**R2（hub 缺陷·head-of-line）**：telemetry relay 是**单 goroutine** 逐流同步写——任一流 Write 阻塞即卡死整条 relay，内核 delta（写通知载体，`delta_update` 自本链上线后由 hub 独占交付，bridge 已停 legacy UDP 转发）全部断流，ZMQ SUB 前向无界丢弃。
agent/internal/vsphub/telemetry.go:62-79（broadcastTelemetry 循环）；agent/internal/bridge/bridge.go:469-481（vspHubOwnsLegacyTelemetryPacket）

**R3（hub 缺陷·publish 路径与 GUI 流耦合）**：`realtime.publish` HTTP 处理器内同步 `broadcastRealtimeFrame→stream.Write`——GUI 流僵死反堵 agent 发布 POST（19:46:50 失败的直接成因），触发 SessionID 清空与重注册抖动。
agent/internal/vsphub/realtime.go:439-451,485-505；agent/internal/bridge/vsp_realtime.go:117-123

**R4（触发面·GUI 消费节流×delta 风暴）**：Godot 端每帧最多消费 64 条 ws 消息且每条结构 delta 触发防抖工程刷新+UI 重建；导入风暴下主线程饱和→停止读 ws→TCP 背压→R1/R2 生效。恢复=Godot 消化积压（~19:45:55 恢复交互），ws 写通道至 19:46:52 才完全解开。
Godot app/kernel/autoloads/vsp_realtime_adapter.gd:20（MAX_WS_MESSAGES_PER_FRAME=64）、637；telemetry_manager.gd:694-727

**R5（恢复慢机制）**：GUI 无断供感知——`_last_delta_seq_id` 只在**有 delta 到达时**检测 gap，静默断供（零消息）与正常空闲不可区分；无 staleness watchdog、无心跳；恢复完全被动依赖 Godot 消化积压。agent 发布者失败即**丢帧**（payload 不重试，channel latest-only），仅 250ms 盲间隔后等下一帧。
telemetry_manager.gd:658-668；vsp_realtime.go:84-96

**R6（hub 进程未重启、无端口竞争）**：hub 全窗单次 start、监听未变；kernel 命令面窗内多轮健康往返（§一时间线 19:43:28/53、19:44:32-36）——**排除 vsphub 进程崩溃/重启/端口竞争/内核停摆假设**；停摆本质=R1+R2 下的投递面僵死。

## 三、停止条件裁定

根因全部落在 agent/hub 可控面（R1/R2/R3/R5）+GUI 消费面（R4 触发、R5 感知缺口）——**不在内核/系统层**（kernel 命令面与 SUB 发布窗内均健康，agent 自有 SUB 同窗正常消费为证）。本卡继续执行第二步韧性修复，无需上交。

## 四、修复设计对应（第二步实施蓝图）

| 根因 | 修复 | 面 |
|---|---|---|
| R1 | StreamClient 写超时（3s）+失败即驱逐（close conn→正常清理路径） | vsphub/websocket.go |
| R2 | 每流出站有界队列+独立 writer goroutine；relay/publish 全部非阻塞入队；慢流队列满→驱逐（fail-fast，GUI 走重连+replay/pull 补齐） | vsphub/websocket.go/telemetry.go |
| R3 | publish HTTP 路径经队列入队，永不阻塞（随 R2 自动解除） | vsphub/realtime.go |
| R5-hub | hub 心跳（10s 应用层 hub.heartbeat，走 writer 直写）+Health() 增加 relay 存活/驱逐计数（注入腿量化断言面） | vsphub/websocket.go/diagnostics.go |
| R5-agent | 发布失败保留 payload 退避重试（250ms→2s 封顶，60 次预算≈110s），预算尽丢帧告警；重注册机制保留 | bridge/vsp_realtime.go |
| R4/R5-GUI | ws 活动 watchdog（15s 无任何消息→stale 信号→复用既有防抖 pull 链收敛一次；30s 仍僵→强制断开走既有重连+重订阅）；hub.heartbeat 静默跳过 | Godot vsp_realtime_adapter.gd / telemetry_manager.gd |

零丢失口径（验收脚注）：hub 存活期 ws 断连重连由 event replay 缓存补齐；hub 进程死亡（注入腿）缓存清零，恢复后以「重连点起 delta seq 连续 + GUI 拉取态与内核权威一致」为收敛判据——结构性 delta 本身为可重放快照语义（shadow 每轮 vsp.state.snapshot 全量重建），事件级零丢失断言限定恢复点之后。

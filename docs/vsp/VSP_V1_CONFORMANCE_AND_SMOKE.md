# VSP v1 Conformance and Smoke

本文定义 VSP v1 Foundation 的一致性测试和烟测验收。目标是避免协议只停留在文档层，或者 GUI 看似能跑但状态、实时数据和 Agent 控制仍旧混流。

## 1. 协议一致性测试

### Session

- client hello 必须声明 `client_id`、`role`、`vsp_version`、`capabilities`。
- Kernel 必须返回 accepted capabilities、feature flags、session_id。
- 未授权能力调用必须返回 `permission_denied` 或明确的 capability error。

### Command

- 每个 request 必须有 request_id。
- 写命令必须返回 transaction_id 或明确说明无事务。
- 成功回执必须包含状态、变更对象或 revision。
- 失败回执必须包含错误分类和可读 message。
- 重复 request_id 的行为必须可预测，至少不能重复执行危险写操作。

### State

- 客户端先取得 snapshot，再订阅 delta。
- Delta 必须包含 base_revision 和 revision。
- Revision gap 必须被检测。
- Gap 后客户端必须能请求 resync。
- 客户端本地 shadow state 经过 delta 应与新 snapshot 对齐。

### Realtime

- realtime packet 必须有 stream id、timestamp 或 frame revision。
- latest-only stream 允许丢旧包，但不能倒序应用旧包。
- GUI 订阅可见轨时，Kernel 不应推送全工程同等级高频数据。
- 关闭订阅后数据停止或显著降频。

### Asset

- 大资产只通过 asset reference 传输。
- asset reference 必须包含 asset_id、kind、revision、uri/hash 或可验证定位信息。
- asset revision 变化必须通知订阅方。
- 过期 asset 请求必须返回 stale 或 not_found，而不是静默失败。

### Plugin

- plugin describe/readback/apply batch 必须有稳定 plugin instance id。
- batch apply 必须支持多个参数。
- apply 后返回 changed params、actual normalized value、display text 或无法取得的明确说明。
- profile stale 必须能被表达。

## 2. 性能烟测

### 多轨导入

目标：

- 导入 60 条音频轨。
- GUI 不冻结。
- 导入进度可见。
- 导入期间播放控制不永久失效。

观察：

- Command latency。
- Event progress rate。
- State delta 数量和大小。
- GUI 主线程 frame time。
- Godot 控制台 warning/error。

### 时间线滚动

目标：

- 60 轨工程从顶部滚到底部。
- 滚动不会随着访问范围扩大而永久变慢。
- 轨道控件 virtualization 生效。

观察：

- Godot node count。
- 可见轨数量。
- 每帧更新对象数量。
- 不可见轨 realtime subscription 是否关闭。

### 播放指针

目标：

- 空格和 GUI 播放按钮都能稳定控制走带。
- 播放指针平滑推进。
- GUI 主线程负载升高时不导致按钮永久不可点。

观察：

- transport realtime stream。
- GUI input 响应。
- command ack。

### Meters / Spectrum

目标：

- 单轨下 meter 和 spectrum 正常。
- 多轨下可见约 17 条轨 meter 和实时 spectrum 正常。
- 不可见轨不做同等级实时渲染。

观察：

- visible subscription。
- realtime packet rate。
- Godot draw/update cost。
- spectrum 空白、延迟或错绑轨道问题。

### Waveform / Spectral Peak Asset

目标：

- 导入后显示离线波形/peak。
- 资产生成不阻塞 GUI。
- 资产按 viewport 加载。

观察：

- asset reference。
- tile cache hit/miss。
- bake progress。
- 文件 I/O warning。

### Phase 4 Reference Smoke

当前 Kernel reference 仍复用 legacy ZMQ REQ/REP 端点，因此实时和事件通道用拉取式 smoke 验证协议形状：

- `realtime.subscribe` 返回 `realtime.stream_status`，确认 `latest_only`、`drop_old`、`max_hz` 和 visible track budget。
- `realtime.frame_request` 返回一帧 `realtime.frame`，用于 REQ/REP 环境验证 `stream_id` 与单调 `frame_index`；正式 streaming transport 可直接推送 `realtime.frame`。
- `asset.request` 返回 `asset.reference`，必须使用 `vit-cache://` 等平台中立 URI，并且不得包含 waveform/spectrum 大数组。
- `event.subscribe` 返回订阅 notification，`event.poll` 在 REQ/REP 环境采样 import/bake/render 进度；无活跃 job 时返回明确 no-op notification。
- 运行脚本：`scripts/vsp_phase4_realtime_asset_smoke.py`。

## 3. Agent 烟测

### Observe / Execute / Reobserve

流程：

1. Agent 获取 project snapshot。
2. Agent 执行建轨或导入命令。
3. Kernel 返回 command ack。
4. Agent 通过 delta 或 resync 确认对象出现。

通过标准：

- Agent 不依赖 GUI 状态。
- Agent 不靠全量 project state 猜测每步结果。
- request_id、transaction_id、revision 可追踪。

### 插件 batch control

流程：

1. 加载插件。
2. describe parameters。
3. batch apply 多个参数。
4. readback。
5. Agent 记录结果。

通过标准：

- batch 一次提交。
- 返回 changed params。
- 失败可定位到具体参数或插件实例。
- 插件抓手智能仍留在 Agent 侧。

### 宏控

流程：

1. 创建宏控。
2. 绑定多个参数。
3. 设置 macro values。
4. readback 绑定传播结果。

通过标准：

- 宏控 ID 稳定。
- 绑定目标可验证。
- Agent 可把高层意图落到宏控而不是每次裸写参数。

### Phase 5 Reference Smoke

- `scripts/vsp_phase5_agent_smoke.py` 覆盖 Agent observe/execute/reobserve、import command、progress event 和 legacy compatibility。
- `scripts/vsp_phase5_plugin_macro_smoke.py` 覆盖 `plugin.set_params_batch`、`command.batch`、`macro.create`、`macro.bind`、`macro.set_values`，并验证 changed params、稳定 plugin/macro ID、子命令顺序结果和 macro binding propagation。

## 4. GUI 手测路径

真实 GUI 测试优先使用 Godot F5 路径：

1. 启动 start page。
2. 进入工程。
3. 导入多轨。
4. 滚动时间线。
5. 播放/停止。
6. 切换 waveform/spectrum。
7. 打开 mixer。
8. 拖动 clip。
9. 观察控制台 warning/error。

F6 当前场景测试可以作为局部调试，但不能替代 F5 完整路径验收。

## 5. 回归门槛

以下任一情况不能视为完成：

- GUI 只在 F6 局部场景流畅，F5 完整路径仍卡。
- Agent 可以发命令，但 GUI 仍靠旧 UDP 全量状态。
- Spectrum 单轨可显示，多轨可见轨显示失败。
- Meters 单轨可显示，多轨失效。
- 60 轨滚动后不可见轨仍在高频更新。
- 命令成功但没有可靠 ack/revision。
- 大资产仍通过 JSON/UDP payload 传输。
- 旧 IPC 被破坏且没有兼容层。

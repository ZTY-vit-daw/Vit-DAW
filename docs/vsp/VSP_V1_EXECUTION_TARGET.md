# VSP v1 Execution Target

本文是给新对话流/目标模式使用的执行目标。进入开发前先阅读本文件、[VSP_V1_FOUNDATION_SPEC.md](VSP_V1_FOUNDATION_SPEC.md) 和 [VSP_V1_CONFORMANCE_AND_SMOKE.md](VSP_V1_CONFORMANCE_AND_SMOKE.md)。

## 目标

重铸 Vit DAW 的底层会话协议 VSP v1 Foundation，使 Kernel、GUI、Agent 的通信从旧 UDP/JSON 全量状态广播和临时桥接，升级为分通道、可版本化、可测试、可迁移到未来自研内核的正式协议/SDK。

VSP v1 必须解决当前已经暴露的问题：

- 多轨导入后 Godot GUI 卡顿。
- 轨道滚动、播放指针、电平、实时 spectrum 被状态/资产/实时数据混流拖慢。
- GUI 和 Agent 都依赖全量 project state 或隐式 JSON 字段确认动作。
- 旧 UDP/JSON payload 和桥接脚本无法作为长期主干。
- Agent 自动混音、插件抓手、宏控和工程操作缺少低延迟、可批量、可验证的执行通道。

## 非目标

本阶段不做以下事情：

- 不重写完整 DAW 内核。
- 不替换 Tracktion Engine。
- 不把插件抓手智能搬进 Kernel。
- 不实现所有未来第三方生态功能。
- 不继续在旧 UDP/JSON 全量广播上做无边界补丁。
- 不以 GUI 临时不卡作为唯一完成标准。

## 开发顺序

### Phase 0: 现状审计

产物：

- 列出现有 Godot UDP、Go bridge、Kernel ZMQ、Agent harness 的实际通道。
- 标记每类数据当前走的路径：Command、State、Realtime、Asset、Event、Session。
- 找出当前最重 payload、最高频事件、GUI 卡顿相关订阅、全量快照调用点。
- 明确旧 IPC 兼容边界。

完成标准：

- 有一份现状矩阵。
- 能解释为什么旧链路导致 GUI 卡顿和状态洪水。
- 没有开始大规模改代码前就先改文档。

### Phase 1: VSP schema 与通道骨架

产物：

- VSP message envelope。
- request_id、client_id、session_id、role、capability、revision、transaction_id 基础字段。
- Command、State、Realtime、Asset、Event、Session 通道定义。
- 错误码、回执、feature flags。
- 旧 IPC command adapter 设计。

完成标准：

- schema 能表达现有 `play`、`stop`、`get_project_state`、`import_audio`、`add_track`、`move_clip`、`plugin_grabber.apply_control` 等关键动作。
- schema 不暴露 TE 私有结构。
- 有最小 SDK 类型定义或代码生成计划。

### Phase 2: Kernel VSP Server reference implementation

产物：

- Kernel 侧 VSP Server skeleton。
- Session/hello/capability。
- Command request/response。
- Legacy IPC adapter。
- Snapshot/delta revision 基础。
- 日志和 trace。

完成标准：

- 旧命令仍可用。
- 新 VSP command 可调用至少一组核心命令。
- 每个写命令都有 request_id、transaction_id、revision 或明确无状态回执。

### Phase 3: State 通道迁移

产物：

- Project snapshot。
- Track/clip/plugin/macro delta。
- Revision gap 检测。
- Client resync 机制。
- GUI 和 Agent shadow state 的新同步入口。

完成标准：

- GUI 不需要在每个动作后强拉全量 `get_project_state`。
- Agent observe/execute/reobserve 能使用 snapshot + delta。
- Delta 丢失后可恢复。

### Phase 4: Realtime 与 Asset 通道迁移

产物：

- 播放指针、电平、可见轨 spectrum 的 realtime 订阅。
- latest-only、drop-old、rate limit、visible-range subscription。
- 波形 peak/spectrum tile 的 Asset reference。
- 导入和 bake 进度 Event。

完成标准：

- 60 轨工程 GUI 不因为不可见轨实时数据而卡死。
- 17 条可见轨电平/频谱有明确预算。
- 波形和频谱资产通过引用/缓存访问，不通过大 JSON 推送。

### Phase 5: Agent 通道迁移

产物：

- Agent VSP client。
- Project observe compact package。
- Command execution/reobserve。
- Plugin batch apply/readback。
- Macro control command。
- Import command 和 progress observe。

完成标准：

- Agent 不再依赖混杂 bridge payload 判断核心动作结果。
- 自动混音所需轨道、clip、插件、宏控、渲染、分析动作都有稳定接口。
- 插件抓手仍在 Agent 侧，但执行通道走 VSP。

### Phase 6: GUI 通道迁移

产物：

- Godot VSP client 或中间 adapter。
- Timeline/track list 使用 state delta。
- Meters/spectrum/playhead 使用 realtime channel。
- Waveform/spectral peak 使用 asset channel。
- 旧 UDP telemetry 降级为兼容入口。

完成标准：

- GUI F5 真实启动路径通过手测。
- Track virtualization 仍生效。
- 滚动不会随着访问轨道范围扩大而永久变慢。
- 播放按钮和空格语义一致，按钮不因 UI 主线程负载失效。

## 禁止事项

- 禁止把实时电平、频谱、波形 tile 重新塞进 project state。
- 禁止用 UI 行号、屏幕坐标、数组 index 作为长期对象 ID。
- 禁止让 GUI 订阅不可见轨的高频实时数据。
- 禁止每个命令后默认刷新全量工程状态。
- 禁止让音频实时线程等待网络、JSON、文件或 GUI。
- 禁止为了短期跑通而把 TE 内部类型写成 VSP 外部契约。
- 禁止把 Agent 插件学习/OCR/web/用户修正逻辑搬进 Kernel。

## 完成定义

VSP v1 Foundation 完成时必须同时满足：

1. 文档完整：角色、通道、传输、状态、实时、资产、命令、权限、插件、迁移、测试都有正式说明。
2. Kernel reference implementation 可运行。
3. GUI 和 Agent 至少完成核心通道迁移，而不是只保留旧桥接。
4. 旧 IPC 兼容层存在，且有迁移说明。
5. Conformance tests 通过。
6. Smoke tests 通过。
7. 手测能确认多轨导入、滚动、播放、meters、spectrum、Agent 命令和插件 batch control 的主要链路。

## 新对话流建议开场目标

可以直接使用以下目标：

> 请在 `D:\Vit_DAW` 和 `D:\Godot\project\vit-daw-frontend` 中执行 VSP v1 Foundation 开发。先阅读 `D:\Vit_DAW\docs\vsp\README.md`、`VSP_V1_FOUNDATION_SPEC.md`、`VSP_V1_EXECUTION_TARGET.md`、`VSP_V1_CONFORMANCE_AND_SMOKE.md`，再审计当前 IPC/bridge/Godot/Agent 链路，制定分阶段实现计划。目标是把 Vit Kernel、GUI、Agent 的通信重铸为 VSP v1：分离 Command、State、Realtime、Asset、Event、Session 通道，保留旧 IPC 兼容层，完成核心 reference implementation、GUI/Agent 迁移和烟测验收。全程中文，不能回滚无关改动，真实 GUI 测试优先使用 Godot F5 路径。


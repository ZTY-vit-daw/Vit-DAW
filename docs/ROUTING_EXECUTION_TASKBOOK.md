# Vit-DAW V0.5 路由引擎执行任务书

**来源**: `docs/ROUTING_IMPLEMENTATION_PLAN_FINAL.md`  
**目标**: 将终极架构方案拆解为可实施、可验收、可并行推进的工程任务。  
**范围**: 仅覆盖 V0.5 路由引擎重构主线，不含后续商业化/云服务扩展。

---

## 0. 执行原则

1. **先拆身份，再拆路由，再拆运行时**。  
2. **每个阶段必须可编译、可加载工程、可回归验证**。  
3. **先保留兼容路径，再删除旧线性链逻辑**。  
4. **任何会影响音频线程的改动，必须配套最小可观测性输出**。  
5. **AIGC / Bridge / 媒体池属于第二波，不阻塞第一波 Rack 核心落地**。

---

## 1. 里程碑总览

### Milestone A: 线性链去索引化
- 目标：彻底移除 `pluginStableID` 的 index fallback，统一 `EditItemID`

### Milestone B: 单轨单 Rack 成立
- 目标：每轨唯一 `RackInstance`，旧链可迁移/包裹

### Milestone C: 2D IPC 通路打通
- 目标：`rack_add_node` / `rack_connect_pins` / `rack_remove_connection`

### Milestone D: 防御层与热切换
- 目标：DAG、跨域适配、播放图双缓冲

### Milestone E: Clip 副链与可视化状态
- 目标：`clip_routes[]`、scope 过滤、共享节点显示

### Milestone F: AIGC Bridge / Media / Warp
- 目标：异步结果回灌、媒体池、节拍对齐

### Milestone G: 插件抓手与控制图
- 目标：`Get Param`、模板壳、统一参数面板、参数映射

---

## 2. Phase 1 — 身份统一与旧逻辑隔离

### 2.1 目标
- 插件身份完全基于 `te::EditItemID`
- 停止新增任何基于线性 index 的协议语义

### 2.2 核心文件
- `VitApp/Source/Service/CommandDispatcher.cpp`
- `VitApp/Source/Service/CommandDispatcher.h`

### 2.3 任务
1. 删除 `pluginStableID(...fallbackIndex...)` 的旧回退逻辑
2. 重写：
   - `findPluginByID()`
   - `findPluginInEdit()`
3. 统一响应字段：
   - `plugin_item_id`
   - `node_item_id`
4. `get_project_state` / 插件相关命令统一输出 `EditItemID`
5. 保留一段短期兼容日志路径，用于识别旧前端是否仍依赖旧串

### 2.4 验收
- 删除中间插件后，其余节点 ID 不变化
- Undo / Redo 后 ID 不漂移
- `get_project_state` 中无 `type:name:index` 风格字符串

### 2.5 风险
- 旧前端仍缓存旧 ID
- 某些插件实例在未完全挂入 Edit 前 `itemID` 无效

---

## 3. Phase 2 — 单轨单 Rack 骨架

### 3.1 目标
- 所有目标轨都有且仅有一个 `RackInstance`
- 旧工程加载时不破坏播放

### 3.2 核心文件
- `VitApp/Source/Service/VitHeadlessService.cpp`
- `VitApp/Source/Service/VitHeadlessService.h`
- 新增 `VitTrackRackManager.*`

### 3.3 任务
1. 新建 `VitTrackRackManager`
2. 在 `applyLoadedEdit()` 中调用：
   - `ensureTrackRackGraphForEdit()`
3. 实现：
   - `ensureSingleRackForTrack()`
   - `findRackForTrack()`
4. 规范监控插件顺序：
   - Rack
   - Volume
   - Meter
5. 记录：
   - `vit_graph_schema_version`
   - `vit_migrated_from_linear`

### 3.4 验收
- 新工程每条音轨仅一个 Rack
- 加载旧工程不会丢失声音输出
- 多 Rack 非法状态能被识别并日志提示

### 3.5 风险
- 旧工程自动迁移时次序与原插件链不一致
- `ensureMonitoringPluginsForEdit()` 与 Rack 顺序冲突

---

## 4. Phase 3 — 2D 路由 IPC 最小闭环

### 4.1 目标
- 从前端可以新增节点、连边、删边
- `Top` 继续保持不可加载

### 4.2 核心文件
- `VitApp/Source/Service/CommandDispatcher.cpp`
- `docs/VIT_IPC_CONTRACT.md`

### 4.3 任务
1. 注册 handlers：
   - `handleRackAddNode`
   - `handleRackConnectPins`
   - `handleRackRemoveConnection`
   - `handleRackSetNodePosition`
2. 实现 `rack_add_node`
3. 实现 `rack_connect_pins`
4. 实现 `rack_remove_connection`
5. 对 `zone_id == Top` 直接拒绝
6. 扩展 `get_project_state` 输出：
   - `rack.nodes[]`
   - `rack.edges[]`
   - `clip_proxy_nodes[]`

### 4.4 验收
- 前端可新增 Z1/Z2/Z3 节点
- 边能建立并保存
- 节点坐标能在工程重开后恢复

### 4.5 风险
- `RackType` 的位置和连接状态没有及时 `flushStateToValueTree`
- 节点增删后前端状态未更新

---

## 5. Phase 4 — 路由验证与安全层

### 5.1 目标
- 非法连接被提前阻断
- 错区连接不崩溃

### 5.2 核心文件
- 新增 `VitGraphValidator.*`
- 新增 `VitDagChecker.*`
- 新增 `VitZoneBufferAdapter.*`
- 新增 `VitParallelMergePlanner.*`

### 5.3 任务
1. `VitGraphValidator::isStructurallyAllowed()`
2. `VitDagChecker::wouldCreateCycle()`
3. `VitZoneBufferAdapter` 支持：
   - Dummy audio padding
   - Wrap & Passthrough
4. `VitParallelMergePlanner` 检测并联是否缺 Merge/Sum

### 5.4 验收
- `Z3 -> Z2` / `Z3 -> Z1` / `Z2 -> Z1` 被拒绝
- MIDI 接 Audio FX 后链路不断
- 并联支路合流前若缺 Merge，能给出明确错误/建议

### 5.5 风险
- 过严校验阻止合法实验性图
- Passthrough 实现不完整导致信号断崖

---

## 6. Phase 5 — 双缓冲播放图与 revision 机制

### 6.1 目标
- 图修改不以 `suspendProcessing` 为主路径
- 有 graph revision 与 diff 摘要

### 6.2 核心文件
- 新增 `VitGraphSwapCoordinator.*`
- 新增 `VitGraphRevisionLedger.*`

### 6.3 任务
1. 定义 compiled playback graph 快照
2. 在 buffer 边界原子发布新 graph handle
3. 延迟回收旧图
4. 记录最小 diff：
   - node add/remove
   - edge add/remove
   - take switch
   - ghost state change
   - warp state change

### 6.4 验收
- 播放中改边无明显爆音
- revision 单调递增
- 可追踪最后一次图变化内容

### 6.5 风险
- 播放图快照与编辑态 `ValueTree` 不一致
- 旧图生命周期处理不当

---

## 7. Phase 6 — Clip 动态路由与 Scope Viewport

### 7.1 目标
- 同一 Track Rack 支持多 Clip 动态接入
- 前端能按 Track / Clip / Debug 三种 scope 看图

### 7.2 核心文件
- 新增 `VitClipRouteRegistry.*`
- `CommandDispatcher.cpp`

### 7.3 任务
1. `clip_id -> node_id[]` 映射
2. `get_project_state` 增加：
   - `clip_routes[]`
   - `shared_by_clip_ids[]`
   - `source_clip_id`
   - `line_color_hint`
3. 支持 scope 参数：
   - `track`
   - `clip:<id>`
   - `debug_global`

### 7.4 验收
- 多个 Clip 共用 Serum 时图仍可读
- 选中特定 Clip 时只显示相关连线/高亮共享链

### 7.5 风险
- 全量图与 clip-scope 图状态不一致
- 共享节点高亮逻辑复杂

---

## 8. Phase 7 — AIGC Bridge / 媒体池 / 恢复 / Warp

### 8.1 目标
- 异步结果能安全回灌进工程
- 工程不会因大量 AIGC 生成而无限膨胀

### 8.2 核心文件
- 新增 `VitBridgeNode.*`
- 新增 `VitAudioInjectorNode.*`
- 新增 `VitWarpBridgeNode.*`
- 新增 `VitAIGCJobRuntime.*`
- 新增 `VitMediaPoolManager.*`

### 8.3 任务
1. 结果回灌：
   - 导入资产
   - 生成 Clip / buffer source
   - 注入 Z3
2. Warp：
   - BPM / Grid 对齐
   - `vit_warp_state`
3. 媒体池：
   - `AIGC_Cache`
   - `Generated`
   - hash/path persistence
4. Asset Restoration Policy：
   - present
   - missing
   - regenerating
   - unrecoverable
5. 工程打包：
   - `Collect Project`
   - `Archive Project`
6. GC 保护：
   - active take
   - pinned
   - in use
   - archived

### 8.4 验收
- AIGC 结果能自动进入工程主链/副链
- 缺失资产时进入恢复/ghost，不崩
- 工程打包后可在另一台机器恢复 active 资产

### 8.5 风险
- 静默重生成与用户当前工程状态脱钩
- Warp 结果与原素材对齐不稳定

---

## 9. Phase 8 — Take 历史与 Ghost State

### 9.1 目标
- 每个资产型节点有多版本栈
- 异步 pending 不打断播放

### 9.2 核心文件
- 新增 `VitTakeHistoryStack.*`
- 新增 `VitAsyncGhostPolicy.*`

### 9.3 任务
1. 节点级 take 栈：
   - active take
   - switch take
2. Ghost 策略：
   - bypass
   - play_cache
   - mute
3. `get_project_state` 输出：
   - `take_histories[]`
   - `jobs[]`

### 9.4 验收
- A/B 切换不改图结构
- pending 期间选择 `play_cache` 时不会断音

### 9.5 风险
- take 切换与当前播放图不同步
- ghost policy 与主链语义冲突

---

## 10. Phase 9 — 插件抓手与统一参数面板

### 10.1 目标
- 节点保留 `Open UI` / `Get Param`
- 参数自动抓取、模板化、轻量化交互

### 10.2 核心文件
- 新增 `VitPluginGrabber.*`
- 新增 `VitPluginTemplateRegistry.*`
- 新增 `VitControlShell.*`

### 10.3 任务
1. 自动遍历插件参数
2. 标准化角色：
   - `comp_threshold`
   - `eq_freq_low`
   - 等
3. 模板壳：
   - eq
   - comp
   - bus
   - instrument
   - other
4. Param Grabber Panel：
   - 默认层：参数、搜索、推荐组
   - 高级层：Template / Link / History / Macro / Freeze / Bypass
5. 参数别名支持

### 10.4 验收
- 不同插件可被稳定套壳
- 用户不打开原生 UI 也能完成高频控制

### 10.5 风险
- 模板匹配错误导致 AI/用户误解参数
- 参数标准化表维护成本高

---

## 11. Phase 10 — 控制图与 Max 风格控件

### 11.1 目标
- 不只做 DSP 图，还做 Control Graph

### 11.2 核心文件
- 新增 `VitParamLinkGraph.*`
- 新增 `VitParamBinding.*`
- 新增 `VitMacroNode.*`
- 新增 `VitParamSurface.*`

### 11.3 任务
1. 控件节点：
   - Slider
   - Knob
   - XYPad
   - Toggle
   - Button
   - Envelope
   - LFO
2. 参数映射：
   - 参数到参数
   - 用户宏到多插件
   - AI 输出到参数

### 11.4 验收
- 跨插件统一调控生效
- 控制图不破坏音频图稳定性

### 11.5 风险
- 控制域与音频域节拍不同步
- 参数映射图可视化复杂度迅速上升

---

## 12. Phase 11 — 连接器开放与能力清单

### 12.1 目标
- 为用户自定义 Web/API/AIGC 平台接入预留统一规范

### 12.2 核心文件
- 新增 `VitNodeRegistry.*`
- 新增 `VitNodeCapabilityManifest.*`
- 新增 `VitConnectorPluginSpec.*`

### 12.3 任务
1. 节点 capability schema
2. connector manifest
3. user-facing connector profile
4. 浏览器自登录 + 下载回灌统一模型

### 12.4 验收
- 新接入节点不需要修改核心路由语义
- connector profile 与工程状态解耦

### 12.5 风险
- 自定义连接器行为不稳定
- Web 自动化受第三方页面变化影响

---

## 13. Phase 12 — 可观测性、健康检查与导出策略

### 13.1 目标
- 系统必须可诊断、可检查、可导出

### 13.2 核心文件
- 新增 `VitGraphTrace.*`
- 新增 `VitProbeNode.*`
- 新增 `VitNodeProfiler.*`
- 新增 `VitProjectHealthCheck.*`

### 13.3 任务
1. 节点调试：
   - job state
   - ghost state
   - graph revision
   - active take
   - cache hit
2. 工程健康检查：
   - 缺失资产
   - 失效连接器
   - 非法边
   - 未对齐素材
3. 导出策略：
   - wait
   - use_cache
   - bypass
   - fail_export

### 13.4 验收
- 复杂工程可快速定位问题
- 导出时 pending 策略明确且稳定

### 13.5 风险
- 观测信息过多影响 UI 简洁性
- 导出策略与用户预期不一致

---

## 14. 建议实施顺序

### 第一波（必须先做）
1. Phase 1 身份统一
2. Phase 2 单轨单 Rack
3. Phase 3 2D IPC
4. Phase 4 防御层

### 第二波（核心可用）
5. Phase 5 双缓冲播放图
6. Phase 6 Clip 动态路由
7. Phase 9 插件抓手

### 第三波（现代工作流）
8. Phase 7 AIGC Bridge / 媒体池 / Warp
9. Phase 8 Take / Ghost

### 第四波（终极扩展）
10. Phase 10 控制图
11. Phase 11 连接器开放
12. Phase 12 观测/健康/导出

---

## 15. 出口条件

可进入正式实现阶段的前提：

1. `ROUTING_IMPLEMENTATION_PLAN_FINAL.md` 作为架构基线被冻结  
2. 本任务书被接受为执行拆分基线  
3. 决定第一波只实现：
   - Identity
   - Track Rack
   - IPC
   - Safety

这样可以先建立最小可运行的 V0.5 路由核心，再逐步叠加 AIGC、Bridge、抓手和控制图。

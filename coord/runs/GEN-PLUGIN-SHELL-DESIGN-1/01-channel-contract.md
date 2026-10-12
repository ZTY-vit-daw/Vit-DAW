# 设计件一：VST3 通道契约（上下文下行 / 状态上行 / 生成参数面 / PromptFace 关系）

- 归属：GEN-PLUGIN-SHELL-DESIGN-1 / 2026-10-12
- 卡面对应：设计五件之 1；设计前提 3（通道技术结论，入卡钉死）——本件在其内做选型细化，不推翻前提。
- 交叉引用：`../../runs/GEN-UNIFY-CTX-DESIGN-1/03-injection-contract.md`（件三=注入契约单一化）；`../../runs/GEN-UNIFY-CTX-DESIGN-1/01-context-inventory.md` §4（快照模型）。

## 0. 总体形态（三段式）

```
┌─ 壳（VST3 插件，内核进程内，轨道 rack 上）─┐      ┌─ 内核宿主侧 ──────────┐      ┌─ agent 进程（Go）──────────┐
│ VSTGUI 编辑器：模板/结构化输入/预览/状态/试听 │ T2   │ GenerationShellService │ ZMQ  │ genshell runner（新包）     │
│ instrument：图内缓冲播放                     │←───→│ 队列+通道+ghost 集成   │←───→│ genctx 渲染 + genapi 生成   │
│ 状态参数（T1）/chunk（T3）/IProcessContext(T0)│ in-  │ VitAIGCJobRuntime 复用 │ REQ/ │ + ingest 落轨 + 终态回写    │
└──────────────────────────────────────────────┘ proc │ （既有，零结构改动）    │ PUB  └─────────────────────────────┘
                                                     └────────────────────────┘
```

- 壳与宿主同进程（tracktion/JUCE 宿主 VST3 为进程内装载——PluginRackControlService 经 te::ExternalPlugin 实例化，PluginRackControlService.cpp:3059-3198 rack_add_node 链亲勘）→ T2 通道零 IPC 成本。
- 壳**不做任何 provider 协议、不持任何密钥**：生成执行全在 agent 进程（genapi 方言面，件四 §3 定位）。
- 三段之间只有两条既有传输面：壳↔宿主=VST3 原生机制（本件 §2-§5）；宿主↔agent=既有 ZMQ 命令总线 5555 REQ + 5556 PUB（ZmqGateway.h:19-21 端点亲勘）——**零新传输层**。

## 1. 通道分层（T0-T3，各司其职）

前提 3 给了"tempo/拍号走 IProcessContext+富上下文走自定义通道（IConnectionPoint/参数面/chunk 任选）"。本设计裁定：**三层不是任选其一，而是分层各司其职**——三者都是 VST3 原生机制，各自承载一类语义，避免单一通道承载异质负载：

| 层 | 机制 | 方向 | 承载 | 消费方范围 |
|---|---|---|---|---|
| **T0** | `IProcessContext`（VST3 原生，零自研） | 宿主→壳（音频线程） | tempo/拍号/走带位置——**仅展示与区域换算**（拍长→秒、进度显示） | 任意宿主 |
| **T1** | 参数面（VST3 原生 automatable 参数，离散+归一化） | 双向（宿主可读写参数值） | 作业状态投影：JobState/JobProgress/JobErrorClass/SceneMode/Bypass（§3） | 任意宿主——**未来厂商壳/第三方宿主的可达下限** |
| **T2** | `IConnectionPoint`+`IMessage`（VST3 原生消息通道） | 双向推 | 富上下文快照投影、provider 目录（secret-free）、作业事件、提交/取消请求（§4 信封） | 实现本契约的宿主（v1=VitApp） |
| **T3** | chunk（`getState/setState`，VST3 原生） | 宿主↔壳（持久化时机） | 壳实例状态：模板选择/表单草稿/pending 请求队列/作业历史引用/试听缓冲引用（§5） | 任意宿主（随 edit 持久化） |

**为什么不新增第四种自定义 COM queryInterface 通道**：IConnectionPoint 已提供同等的进程内直达投递，且是 VST3 规范形态（厂商插件与宿主互通的既有惯例）——"未来厂商自家插件形态兼容（标准通道天然可达）"（前提 4）在 IConnectionPoint 上直接成立；自定义 COM 接口反而要求厂商拿我们的头文件。选型收敛于前提给的三项之内。

## 2. T2 的取得路径（实现锚点+降级序）

宿主侧要与壳的 controller 建立 IConnectionPoint 连接，需要拿到壳实例的裸 VST3 接口。**现状事实**：

- 仓库内零 IConnectionPoint 使用先例（全库 grep 仅本卡面文字命中）；
- 宿主装载链=tracktion `te::ExternalPlugin` → JUCE `VST3PluginInstance` 包装 → 裸 `IEditController`。从 te::ExternalPlugin 取裸接口需要进 JUCE 包装层（tracktion_engine 子模块内源码；本 worktree 子模块为空目录，完整源码在 PC 检出——**实现卡第一步核实点**）。

**降级序**（实现卡按序核实，任一成立即收）：

1. **主选**：经 JUCE `VST3PluginInstance` 取 `IEditController*` → `queryInterface(kVstIConnectionPointIID)` → `connect(host端IConnectionPoint)`。壳=VST3 SDK 直实现（§6），自身 IConnectionPoint 完全可控。
2. **降级 A**：`IConnectionPoint` 取得路径不通 → T2 改用**参数面+chunk 组合编码**（提交请求=壳写入自有 state 后置一个"request_pending"离散参数，宿主轮询参数变化后经 `getState` 拉取请求 chunk；下行=宿主 `setState` 推送+参数通知）。语义不变、轮询成本+百毫秒级延迟，功能无损。
3. **降级 B**：连参数通知路径也不稳 → 壳内开 loopback socket 与宿主服务直连（两端同机；最后手段，仅 VitApp 宿主可用，厂商兼容性受损，风险表在案）。

T1/T0/T3 与 T2 取得路径无关（标准宿主 API 面），降级只影响 T2 一层。

## 3. T1 状态参数面（枚举定稿）

固定小参数集（`GenShell.vst3` 的 automatable 离散参数，宿主任意面可读）：

| 参数 | 形态 | 值域 |
|---|---|---|
| `JobState` | 离散 | 0=idle, 1=queued, 2=running, 3=downloading, 4=landing, 5=done, 6=failed, 7=cancelled（与件二 §1 生命周期一致） |
| `JobProgress` | 归一化 | 0.0-1.0（provider 轮询无真进度时=阶段跳变值，不造假中间值） |
| `JobErrorClass` | 离散 | 0=none, 1=config_error, 2=auth_error, 3=quota_error, 4=rate_limit, 5=not_found, 6=provider_error, 7=moderation_error, 8=timeout, 9=network_error, 10=response_parse_error, 11=unsupported_format, 12=landing_error, 13=context_error, 14=interrupted（对齐 genapi ErrorClass 11 枚举——errors.go:12-23 亲验——+件二 §3 新增 3 类） |
| `SceneMode` | 离散 | 0=in_project, 1=free_creation, 2=experiment_control（件四 scene 枚举镜像） |
| `Bypass` | 离散 | VST3 标准绕过（图内播放静音） |

- 参数面**只承载状态投影**，不是控制面：生成动作（提交/取消）走 T2 消息（参数写无法承载结构化请求且会进 automation 轨——语义污染）。
- `SceneMode` 参数为宿主侧可写的便捷镜像（写它=改壳 GUI 的 scene 选择），v1 可只读。

## 4. T2 消息信封（vit.genshell.msg.v1）

`IMessage` 携带单个 JSON 字符串（`data` 流写入；两端都是我们的，不折损 VST3 二进制流能力）：

```json
{ "schema_version": "vit.genshell.msg.v1", "type": "<枚举>", "job_id": "<可选>", "ts": "<RFC3339>", "payload": { ... } }
```

| type | 方向 | payload 要点 |
|---|---|---|
| `context_snapshot` | 宿主→壳 | music.context.v1 快照的**只读展示副本**（genctx 组装真值，§6）+ `injection_ready:false` 标记（genctx 未合入/快照失败时如实投影） |
| `provider_catalog` | 宿主→壳 | genapi provider 列表 secret-free 投影（genapi.Describe() 字段集——config.go:275-296 亲验；含 has_api_key 布尔，永不含 key 材料） |
| `job_progress` | 宿主→壳 | `{state, progress, error_class, status_message}`（GUI 刷新） |
| `job_terminal` | 宿主→壳 | 终态+落轨结果 `{clip_id, take_stack_id, takes[], artifact_paths}`（free_creation 时=工件路径，无落轨） |
| `submit_request` | 壳→宿主 | 完整生成请求（件二 §2 请求结构） |
| `cancel_request` | 壳→宿主 | `{job_id}` |
| `audition_ready` | 宿主→壳 | `{job_id, artifact_path, take_id}`（壳加载缓冲，件二 §5） |

- `schema_version` 未知=接收端**丢弃+计数披露**（fail-closed 不崩：壳/宿主各自在 GUI/日志明示"收到未知版本消息 N 条"，对齐"不静默"纪律）。
- 消息是**通知语义**（at-most-once）：提交请求的可靠性不由 T2 承担，由宿主侧队列+agent 轮询兜底（件二 §4）——T2 丢一条 `job_progress` 只影响一次 GUI 刷新。

## 5. T3 chunk（壳实例状态，§11 语义）

```json
{ "schema_version": "vit.genshell.state.v1",
  "selected_template": "<template_id>",
  "form_draft": { "<field_id>": "<value>" },
  "scene_mode": "in_project",
  "pending_requests": [ <submit_request 载荷，按 job_id> ],
  "job_history": [ { "job_id", "terminal_state", "clip_id", "take_stack_id", "artifact_paths" } ],
  "audition_buffer_ref": { "job_id": null, "take_id": null } }
```

- **持久化通道**=tracktion 插件状态随 edit 保存（ExternalPlugin state 序列化既有面）；工程重开=壳恢复表单草稿+pending 请求（agent 轮询经宿主服务从壳拉——作业队列跨重启自愈）。
- §11 兼容：旧工程无壳节点=零影响（无 chunk 可载）；`schema_version` 未知=壳丢弃内容仅保 `job_history` 的 job_id 引用（工程内已落轨 clip 不受影响）+GUI 提示"壳状态版本不兼容已重置表单"；无任何旧记录改写。
- pending 请求载荷存壳侧 chunk 而非 job record：`VitAIGCJobRecord` 是既有内核结构（VitAIGCJobRuntime.h:8-19 亲验：jobId/nodeId/jobState/ghostState/statusMessage/requestHash/source/assetRef/trackId/clipId），requestHash 已够审计锚定，大载荷不塞内核结构（零内核结构改动的设计约束，件二 §1）。

## 6. 上下文真值单一性（与统一化件三的对齐核对）

**裁定：注入真值只有一个源——agent 侧 genctx 快照**（get_project_state 的 `music_context`+`markers` 键组装，GEN-UNIFY 件二 §2.5）。壳内三处上下文数据的角色：

| 数据 | 源 | 角色 |
|---|---|---|
| T0 IProcessContext tempo/拍号 | 音频线程 | **仅展示/换算**，永不进提示词 |
| T2 `context_snapshot` | 宿主转发 genctx 快照副本 | **仅 GUI 展示**（用户看到"将注入什么"），永不用于拼装提示词 |
| 发送的提示词 | agent 侧 `RenderGenerationPrompt(snapshot, userFreeText, profile=gen_plugin)` | **唯一注入面**（genctx 渲染器独家，件三 §1.1 单一渲染函数） |

- 壳不实现任何自动块渲染逻辑（MUSIC CONTEXT/CONSTRAINTS/GUIDANCE 四块结构只出现在 agent 渲染终文里）；壳的"模板引擎"只编排 USER 块（件三 §2 模板面裁定）。
- 预览=实发：GUI 的提示词预览经 agent dry-run 渲染回显（件三 §4），壳本地不拼预览。
- 对齐核对逐条（卡面验收项"通道契约与统一化件三对齐无冲突"）：①消费方只传 profile——壳传 `ConsumerID="gen_plugin"`、默认全字段全块，裁剪仅按模板声明（不降级——件一 §1 属性级常量纪律）；②机检矩阵——壳 v1 产物=音频（C 形态）无机检面、MUST 声明照发（件三 §3）；③scene 调用级参数——壳每次提交显式携带（件四 scene 消费映射）；④用户块原文保真——结构化字段编排进 USER 块属用户块内部组成（模板=用户的稳定表达格式），自动块仍不可触碰。

## 7. 实现基裁定：壳体=VST3 SDK 直实现+VSTGUI（不走 JUCE plugin 包装）

- **理由**：①T2 两向消息需要壳侧完全控制 IConnectionPoint——JUCE 包装层（juce_VST3Wrapper）拥有 IConnectionPoint 且不向 `AudioProcessor` 子类暴露宿主消息，走 JUCE=要么 patch 子模块（fork 维护负担，违背仓库卫生）要么受限；②壳无 JUCE 依赖则内核-壳无符号耦合；③VSTGUI（Steinberg 官方 GUI 库）+VST3 SDK 是厂商标准栈，结构化表单控件（组合框/文本编辑/按钮/容器）齐备；④仓库已因 PluginProbe native-host 接触 VST3 SDK Hosting 层（PluginProbe/native-host/src/main.cpp:1-5,619-642 亲勘），SDK 获取路径已有先例。
- **fallback**（GUI 成本失控时）：壳体改 JUCE 插件（GUI 用 AudioProcessorEditor），T2 降级走 §2 降级 A（参数面+chunk 编码）。风险表 06 §5 在案。
- **身份**：VST3 类标识固定（厂商 `Vit Audio` / 名 `GenShell`，CID 用固定 GUID 生成后钉死入契约头文件）；部署 macOS=`~/Library/Audio/Plug-Ins/VST3/GenShell.vst3`，Windows=Common Files\VST3\；发现=既有 `PluginScanCoordinator` 扫描（PluginRackControlService.cpp:1690-1770 亲勘）→ `plugin_list_available`/semantics 索引自然可达（PCA 准入的前置，件四 §2）。
- 装载后编辑器经既有命令打开：`show_plugin_editor`/`open_plugin_ui`（CommandDispatcher.cpp:3072/3078 注册亲勘）——**手测入口=用户从 Godot 前端拉起栈后打开插件编辑器**（AGENTS §5 手测入口裁定）。

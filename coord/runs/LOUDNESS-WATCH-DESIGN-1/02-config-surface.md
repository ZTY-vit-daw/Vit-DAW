# 设计②：配置面（工程级开关 + 标准预设 + 用户阈值）

LOUDNESS-WATCH-DESIGN-1 设计件 2/6。本件回答"检查什么标准、谁开谁关、存哪里"。
持久化模式完全复用 RLM-PROFILE-2 已收的 render.profile.bind 工程级运行态先例。

## 1. 存储位置与持久化模式（复用已收先例）

- **归属**：agent 侧工程级运行态 `projectAgentRuntimeState`（`agent/internal/chat/server.go:164`），与 `RenderProfileBindings` 同层（`:194-198` 注释、`:198` 字段）。
- **落盘**：快照 `server.go:7245`（`state.RenderProfileBindings = s.harness.RenderProfileBindingsSnapshot()` 同处新增 loudness watch 配置快照）；恢复 `server.go:7250-7258`（`restoreProjectAgentRuntimeStateLocked`，`:7256` 起 bindings 恢复同款）。
- **理由**：该模式已过 RLM-PROFILE-2 验收——工程级、随工程持久、重启恢复、agent 侧零内核改动；卡面"工程级用户配置"语义与其完全同构。
- **不落内核 Settings.xml**：`VitApp/Workspace/Settings.xml` 是工作区级全局面，不是工程级；混入会越域。

## 2. 配置 schema：`loudness_watch_config.v0`

```jsonc
{
  "schema_version": "loudness_watch_config.v0",
  "enabled": false,                          // 工程级开关，默认关（旧工程缺省语义见 §4）
  "trigger_mode": "after_processing",        // after_processing（默认）| periodic | realtime
  "periodic_interval_seconds": 300,          // 仅 periodic 用；默认 300，校验下限 60
  "profile_source": "builtin:ebu_r128",      // "builtin:<id>" | "custom"
  "custom": {                                // 仅 profile_source=custom 时读
    "integrated_lufs": { "min_lufs": -23.5, "max_lufs": -22.5 },   // 复用 rlm.LoudnessBand 形态
    "true_peak_max_dbtp": -1.0,
    "short_term_max_lufs": null,             // 形备位（口径缺口见设计④），null=不检查该窗
    "momentary_max_lufs": null               // 同上
  }
}
```

### 2.1 标准预设（builtin）

- 预设全集 = `agent/internal/rlm/profile.go:67-118` `builtinDeliveryProfiles` 五项：`builtin:ebu_r128`（-23.5~-22.5 LUFS / -1 dBTP，`:68-77`）、`builtin:aes_td1004`（-20~-16 / -1，`:78-87`）、`builtin:apple_music`（≤-16 / -1，`:88-97`）、`builtin:spotify`（≤-14 / -1，`:98-107`）、`builtin:gy_282_2014`（-26~-22 / -2，`:108-117`）。
- 解析经 `rlm.LookupDeliveryProfile`（`profile.go:129-137`），未知 id fail-closed（与 `NewRenderProfileBinding` 的 `:239-241` 同语义）。
- 预设自带阈值形态 = `LoudnessBand{Min *float64, Max float64}`（`profile.go:45-48`）——"带"不是单边阈值，Min 为 nil 表示标准无下限（Apple/Spotify 形态），比对函数必须尊重 nil 语义。
- 卡面点名的 R128/AES/自定义三态由五预设全集覆盖（多出的 Apple/Spotify/GY/T 是卡面验收后新增预设，见 anchor-verification DRIFT-1，为超集非冲突）。

### 2.2 用户自定阈值（custom）

- **复用而非平行**：custom 阈值形态直接借用 `rlm.DeliveryProfile` 的字段集（`profile.go:54-65`）构造 `ProfileID="custom:project_watch"` 的临时 profile 走 `rlm.ValidateDeliveryProfile`（`profile.go:141-195`）校验——band 序、跨窗序（ST≤M，`:171-181`）、TP≤0dBTP（`:191-193`）、有限性全数继承，不另写一套校验。
- **与 render.profile.bind 的边界**：binding 面 v1 只解析 builtin id（`profile.go:232-248` fail-closed），**本设计不改这一契约**——watch 配置是独立 schema，不走 `RenderProfileBinding`；custom 阈值只在 watch 配置内部存活。两面对接点在呈现层（绑定披露与 watch 状态同屏展示预设向量，设计③ §2）。
- 校验失败语义：configure 调用拒绝并回显具体违规字段（fail-closed，错误消息含字段名与值，与 `profile.go:143-193` 错误风格一致）。

### 2.3 trigger_mode 校验

- `after_processing`：合法，默认。
- `periodic`：合法；`periodic_interval_seconds` 必填位缺省取 300，<60 拒绝。
- `realtime`：**接受存储**（尊重用户意图，往返不丢），激活面如实报 `status=unavailable reason=no_realtime_producer`（依据设计① §3：内核无实时表头生产者，是能力缺口非性能取舍）。不产 verdict。

## 3. 用户操作面（工具）

按 `tools/catalog.go:1067` `render_profile_bind` 的 spec 形态新增两个工具：

| 工具 | 命令名 | 语义 | 风险级 |
|---|---|---|---|
| 配置 | `loudness.watch.configure` | 设/改/关 watch 配置；全参可选（不传 enabled 则不翻转开关）；返回生效后完整配置+校验结果 | RiskDirect（写运行态，不触内核） |
| 状态 | `loudness.watch.status` | 只读披露：当前配置+最近 N 条 verdict（含 unmeasurable）+实时态 unavailable 原因（如适用） | 只读，index 级零成本候选（进 probe 分级表争议时上交，见 06 性能件 §3） |

- `loudness.watch.configure` 的入参即 §2 schema 字段（camel→snake 已在示例）；`profile_source=custom` 时 custom 对象必填。
- 参数拼写/缺参的防呆要求与 catalog 既有 spec 字段（usage 提示）一致；`clip.warm_waveform_bake` 的缺参防错文案先例见 `agentloop/message_loop.go:5422`。

## 4. 持久化兼容与旧工程缺省语义（AGENTS §11 全项）

1. **旧工程缺省**：持久化记录无 `loudness_watch_config` 字段 = enabled=false + after_processing——**零行为变化**（与 `profile.go:7-9` 兼容契约同句式：absent field = current behavior, unchanged）。
2. **schema 版本 fail-closed**：`schema_version` 非 `loudness_watch_config.v0` 的记录解析拒绝（拒绝后按缺省关闭处理并记 dropped 计数，模式同 `render_profile.go:73-89` RestoreRenderProfileBindings 的 dropped 语义），不猜测升级。
3. **未知枚举 fail-closed**：trigger_mode/profile_source 的未知值拒绝激活并按缺省关闭，状态面显示原始值供用户修正（不静默改写用户数据）。
4. **往返测试**：配置写入→快照→恢复→读回全等（含 custom 阈值指针语义）；旧记录（无字段）反序列化=缺省关；坏枚举记录=关+dropped 计数。
5. 不用新增字段悄悄改旧记录含义：watch 配置是**新增独立字段**，不触碰 `RenderProfileBindings` 既有字段语义。

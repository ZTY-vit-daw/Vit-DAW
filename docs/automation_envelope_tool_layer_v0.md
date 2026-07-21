# Vit Automation / Envelope Tool Layer v0

日期：2026-07-07

状态：架构设计稿，用于 Phase 2 的 A4 Clip 裁剪/Fade、后续 C4 段落自动化、以及 Agent typed executor 工具层扩展。

## 1. 结论

Vit 需要的不是一个“音量包络线功能”，而是一套“参数随时间变化的控制系统”。自然语言不应该直接绑定固定话术，而应该被解析成稳定的工具层调用。

v0 采用“对象 -> 目标 -> 控制方式 -> 工具”的组合链路，而不是并列三轴列表。Agent 或 GUI 在执行自动化前，必须先回答这四个问题：

```text
1. Object：这个变化作用于什么对象？
   clip / track / bus / folder bus / master / plugin / send / project map

2. Target：这个对象上的哪个参数被控制？
   gain / pan / mute / fade / crossfade / plugin parameter / send level / speed / pitch / warp marker / tempo

3. Method：使用哪种控制方式？
   static value / clip-bound envelope / track automation lane / project map / modifier

4. Tool：最终调用哪个 typed tool？
   clip.fade.set / track.automation.points.write / plugin.automation.points.write / project.tempo_map.write ...
```

这条链路的意义是：工具层按对象归属设计，统一归入 Automation / Envelope 工具族，但不把所有能力压成一个巨大 `automation.write`。例如 clip fade、track volume automation、plugin parameter automation、project tempo map 都是“参数随时间变化”的一部分，但它们的权限边界、风险等级、时间基、回滚方式和 GUI 呈现不同。

一期实现不应直接覆盖所有高级能力。建议先补齐可验证、可回滚、最常用于混音准备的基础能力：

- `clip.fade.set/read`
- `clip.gain.set/read`
- project snapshot 暴露 clip fade/gain
- GUI 显示并可拖拽 clip fade handle
- Agent/EPM 只通过 typed tool 生成待确认动作

第二期再接：

- `automation.targets.list`
- `automation.lanes.list/read/create/delete`
- `automation.points.write/replace/remove/simplify`
- track volume/pan automation
- plugin parameter automation
- clip gain envelope
- crossfade explicit/auto route

速度、pitch、warp、tempo map 必须进入架构，但建议作为三期，因为它们会影响播放、渲染、波形显示、对齐保护与时间基转换。

## 2. 外部 DAW 依据

### Logic Pro

Logic 明确区分 track automation 与 region automation：track automation 作用于整条轨道并固定在工程时间线上；region automation 绑定到 region，region 移动或复制时自动化跟随。Logic 也支持二者互转。

设计启发：Vit 必须区分“轨道时间线自动化”和“clip/region 绑定自动化”。同一个轨道上可以同时存在 track lane 和 clip-bound envelope。

参考：https://support.apple.com/en-asia/guide/logicpro/lgcp967649d0/mac

### Cubase

Cubase 把 fades、crossfades、event volume offset、event volume curve 分开。Event volume curve 属于 audio event，移动或复制 event 时 envelope 跟随。Cubase 还支持 event-based fade、clip-based fade、auto fade/crossfade。

设计启发：Vit 需要把 clip edge fade、crossfade、clip static gain、clip gain envelope 拆成不同工具，而不是混在一个 automation 命令里。

参考：

- https://www.steinberg.help/r/cubase-pro/15.0/en/cubase_nuendo/topics/fades_crossfades_and_envelopes/fades_crossfades_and_envelopes_c.html
- https://www.steinberg.help/r/cubase-pro/15.0/en/cubase_nuendo/topics/fades_crossfades_and_envelopes/fades_event_envelopes_c.html
- https://www.steinberg.help/r/cubase-pro/15.0/en/cubase_nuendo/topics/fades_crossfades_and_envelopes/fades_auto_fades_and_crossfades_c.html

### Ableton Live

Ableton 区分 automation 与 modulation。Automation 定义参数在某个时间点的值；modulation 只影响已有值，可以与 automation 同时作用。Ableton 的 clip envelopes 可控制 mixer/device，也有 audio clip 专属 envelope，例如 pitch、volume 等，用于改变录音音高、节奏和动态。

设计启发：Vit v0 先实现 absolute automation/envelope；modulation 要在数据模型中预留，但不进入一期实现。

参考：

- https://help.ableton.com/hc/en-us/articles/209070629-Working-with-Automation-and-Modulation
- https://www.ableton.com/en/manual/live-concepts/

### Pro Tools

Pro Tools 的 clip gain 可做静态或动态调整，官方用户指南把它作为独立于轨道音量自动化的 clip 级控制。工程经验上，clip gain 常用于进入插件链前的增益整理，track volume automation 更常用于混音电平 ride。

设计启发：Vit 必须同时保留 clip gain 与 track volume automation。二者不是重复功能：clip gain 是片段/素材进入处理链前的电平整理，track automation 是轨道混音关系随时间变化。

参考：

- https://www.avid.com/pro-tools/user-guide/clip-gain
- https://resources.avid.com/SupportFiles/PT/Pro_Tools_Reference_Guide_2024.6.pdf

### REAPER

REAPER 强调 envelope/automation、modulation、VCA、routing、scripting 的开放组合；它的设计说明了 automation 不应仅限于音量、声像，而应是“可被目标参数注册系统扩展”的通用机制。

参考：https://www.reaper.fm/

## 3. Vit 当前状态审计

### 已有基础

- 内核 clip 编辑已存在：`move_clip`、`resize_clip`、`split_clip`、`remove_clips`。
- `ClipService` 在 split 或 overlap trim 后会给相关 clip 调 `applyMicroFadeIfNeeded()`，底层使用 Tracktion Engine 的 `AudioClipBase::setFadeIn/setFadeOut`。
- Tracktion Engine 本身已支持：
  - `AudioClipBase::setGainDB/getGainDB`
  - `AudioClipBase::setFadeIn/getFadeIn`
  - `AudioClipBase::setFadeOut/getFadeOut`
  - `AudioClipBase::setFadeInType/setFadeOutType`
  - `AudioClipBase::setAutoCrossfade`
  - `AudioClipBase::copyFadeToAutomation`
  - `AudioClipBase::FadeBehaviour`，包含 `gainFade` 与 `speedRamp`
  - `AudioClipBase::setTimeStretchMode`
  - `AudioClipBase::setAutoTempo`
  - `AudioClipBase::setAutoPitch`
  - `AudioClipBase::setWarpTime`
  - `AudioClipBase::setPitchChange`
  - `AutomationCurve::addPoint/movePoint/removePoint/simplify`
  - `AutomatableParameter::getCurve`
  - automation mode：read、touch、latch、write
- GUI clip 组件已有 fade hit zone 概念：`TOP_LEFT_FADE`、`TOP_RIGHT_FADE`，但尚未接成正式命令。
- Agent 工具目录已有 clip move/resize/split/remove，但没有正式的 clip fade/gain/envelope/automation 工具。
- EPM 已能输出裁剪/Fade 建议，但目前还不能把建议转成 typed executor 操作。

### 明确缺口

- 没有 `clip.set_fade`/`clip.fade.set` 内核命令。
- 没有 `clip.gain.set` 内核命令。
- project snapshot 没有稳定暴露 clip fade/gain/pan/mute/speed/pitch/warp 状态。
- GUI 没有正式显示当前 fade/gain 状态，也没有将 fade handles 写回内核。
- 没有统一的 automation target registry。
- 没有 automation lane read/write 工具。
- 没有 track/plugin automation point 写入工具。
- 没有 clip-bound gain envelope 工具。
- 没有 crossfade create/clear/read 的明确 contract。
- 没有 tempo map、speed/warp/pitch 的 Agent-safe 工具层。

## 4. 信号链顺序

Vit 对外应采用稳定的逻辑顺序：

1. source media / take selection
2. clip trim / split / offset / loop
3. clip speed / warp / pitch / reverse
4. clip gain / clip pan / clip mute
5. clip gain envelope
6. clip edge fade / crossfade
7. track inserts
8. sends
9. track fader / pan / mute automation
10. folder bus / submix bus
11. master bus
12. render/export

实现时可以映射到 Tracktion Engine 的实际处理顺序，但工具层必须让用户和 Agent 理解“clip gain 是进轨道处理前的片段整理，track automation 是轨道混音关系变化”。

## 5. 核心数据结构

### 5.1 AutomationTarget

```json
{
  "target_id": "target_track_123_volume",
  "owner_kind": "track",
  "owner_id": "track_123",
  "parameter_kind": "volume",
  "parameter_id": "master volume",
  "plugin_id": "",
  "display_name": "Track Volume",
  "value_domain": {
    "unit": "db",
    "min": -144.0,
    "max": 12.0,
    "default": 0.0
  },
  "time_base": ["seconds", "beats"],
  "automation_support": ["static", "lane"],
  "write_support": true,
  "read_support": true,
  "risk": "undoable"
}
```

### 5.2 AutomationLane

```json
{
  "lane_id": "lane_track_123_volume",
  "target_id": "target_track_123_volume",
  "owner_kind": "track",
  "owner_id": "track_123",
  "time_base": "seconds",
  "enabled": true,
  "bypass": false,
  "points": [
    {"time": 12.0, "value": -3.0, "curve": 0.0},
    {"time": 20.0, "value": -1.5, "curve": 0.25}
  ]
}
```

### 5.3 ClipFade

```json
{
  "clip_id": "clip_abc",
  "fade_in_seconds": 0.02,
  "fade_out_seconds": 0.05,
  "fade_in_curve": "equal_power",
  "fade_out_curve": "linear",
  "fade_in_behaviour": "gain",
  "fade_out_behaviour": "gain",
  "auto_crossfade": false
}
```

`fade_behaviour=speed` 预留给 tape start/stop 类速度 ramp，不进入一期默认 GUI。

### 5.4 ClipGain

```json
{
  "clip_id": "clip_abc",
  "gain_db": -2.5,
  "pan": 0.0,
  "mute": false
}
```

### 5.5 ClipEnvelope

```json
{
  "envelope_id": "clip_abc_gain_env",
  "clip_id": "clip_abc",
  "parameter_kind": "clip_gain",
  "time_mode": "clip_relative_seconds",
  "points": [
    {"time": 0.0, "value_db": -1.0, "curve": 0.0},
    {"time": 1.2, "value_db": -4.0, "curve": 0.0}
  ],
  "moves_with_clip": true
}
```

### 5.6 ProjectMap

用于 tempo、meter、key 等工程级时间地图。它不应混入普通 track automation lane。

```json
{
  "map_kind": "tempo",
  "time_base": "beats",
  "events": [
    {"beat": 0.0, "bpm": 120.0},
    {"beat": 64.0, "bpm": 128.0}
  ]
}
```

## 6. 工具层 v0

工具层先于自然语言路由。自然语言只负责把用户意图解析到这些工具，不应该成为功能定义本身。

工具族按 Object 归属组织：

```text
clip.*
  clip.fade.set/read/clear
  clip.gain.set/read
  clip.envelope.write/read
  clip.speed.set
  clip.pitch.set
  clip.warp_markers.patch

track.*
  track.volume.set
  track.pan.set
  track.automation.lanes.*
  track.automation.points.*

plugin.*
  plugin.param.set
  plugin.automation.lanes.*
  plugin.automation.points.*

send.*
  send.level.set
  send.pan.set
  send.automation.points.*

project.*
  project.tempo_map.*
  project.marker.*
```

`automation.*` 保留为跨对象的发现、校验、曲线生成和通用 point 操作层；真正写工程时应尽量落到 object-scoped tool，便于权限、确认、回滚和 GUI 刷新。

### 6.1 Target Registry

- `automation.targets.list`
  - 读取某个 scope 下可自动化目标。
  - 输入：`scope_kind`, `scope_id`, `include_plugins`, `include_sends`, `include_clip_targets`
  - 输出：`AutomationTarget[]`

- `automation.target.describe`
  - 读取单个 target 的值域、单位、是否支持 lane、是否支持 clip envelope。

### 6.2 Lane CRUD

- `automation.lanes.list`
- `automation.lane.read`
- `automation.lane.create`
- `automation.lane.delete`
- `automation.lane.set_bypass`

### 6.3 Points

- `automation.points.write`
  - append/merge 点，不清空既有 lane。
- `automation.points.replace_range`
  - 替换一个时间范围内的点。
- `automation.points.remove_range`
- `automation.points.simplify`
- `automation.points.read_range`

所有写入都必须支持：

- `time_unit`: seconds/beats
- `curve`
- `mode`: append/replace_range
- `requires_confirmation`: true by default
- undo transaction name

### 6.4 Static Value

- `automation.value.set`
  - 用于设置某 target 当前静态值。
  - 可映射到现有 `set_volume`、`set_pan`、`set_plugin_param`，但对 Agent 暴露统一 target contract。

### 6.5 Clip Tools

- `clip.fade.read`
- `clip.fade.set`
- `clip.fade.clear`
- `clip.gain.read`
- `clip.gain.set`
- `clip.envelope.read`
- `clip.envelope.write`
- `clip.crossfade.create`
- `clip.crossfade.clear`
- `clip.speed.set`
- `clip.pitch.set`
- `clip.warp.enable`
- `clip.warp_markers.patch`

其中一期只实现：

- `clip.fade.read`
- `clip.fade.set`
- `clip.gain.read`
- `clip.gain.set`

`clip.crossfade.*` 可以在 fade 可视化和 snapshot 稳定后接入。

### 6.6 Automation Mode

- `automation.mode.read`
- `automation.mode.set`

支持值：

- read
- touch
- latch
- write

v0 不建议 Agent 主动进入实时写 automation 模式。Agent 应主要写离线 points，实时 write/touch/latch 先留给 GUI/控制器。

### 6.7 Agent Curve Generator

Agent 不需要人类 GUI 那种手绘工具，但需要一套受控的“参数化曲线生成器”。它不能自由写任意函数直接改工程，而应该把曲线意图提交给工具层，由工具层编译成安全、可读、可回滚的 automation points。

- `automation.curve.generate`
  - 输入曲线意图，不直接修改工程。
  - 输出 bounded points、预览摘要、风险、需要确认的写入计划。

- `automation.curve.apply`
  - 只接受 `automation.curve.generate` 或同等 schema 产生的 points plan。
  - 写入前再次校验 target value domain、时间范围、点数上限和覆盖策略。

示例：

```json
{
  "shape": "s_curve_ramp",
  "start_time": 32.0,
  "end_time": 40.0,
  "start_value": -6.0,
  "end_value": -1.5,
  "density": "medium",
  "curve": 0.35
}
```

```json
{
  "shape": "ducking",
  "range": [64.0, 72.0],
  "floor_db": -4.0,
  "attack_ms": 80,
  "release_ms": 250
}
```

曲线生成器必须执行：

- target registry 值域校验
- 时间范围限制
- 点数上限
- 曲线平滑与简化
- 与既有 automation 的覆盖/合并策略检查
- confirmation 与 undo/rollback 信息生成

这让 Agent 可以比传统 GUI 更强：它不需要手绘，但可以通过参数化曲线、函数族和安全编译器生成复杂 envelope。

### 6.8 Project Maps

- `project.tempo_map.read`
- `project.tempo_map.write`
- `project.tempo_map.replace_range`

`set_tempo` 是单点静态设置；tempo map 是工程级时间地图，不属于普通 automation lane。

## 7. GUI 层设计原则

### 一期 GUI

- clip 上显示 fade-in/fade-out 小三角或曲线提示。
- 拖动左上/右上 fade handle 调用 `clip.fade.set`。
- clip inspector/detail panel 显示：
  - gain dB
  - fade in/out 秒数
  - fade curve
  - auto crossfade 状态
- 波形视觉随 clip gain 调整可后置，先保证数值和命令闭环。

### 二期 GUI

- automation lane header：选择 target、显示/隐藏 lane。
- point editor：添加、拖拽、删除、框选、简化。
- track lane 与 clip envelope 分层显示。
- clip gain envelope 显示在 clip 内部。

### 三期 GUI

- tempo map / speed / warp / pitch 专用编辑面。
- modulation lane 与 absolute automation lane 并存。

## 8. Agent/EPM 接入原则

这里不定义固定自然语言句式，只定义 Agent 可执行边界：

- Agent 的执行流程必须遵守 Object -> Target -> Method -> Tool，不允许直接从自然语言跳到 raw command。
- EPM 可以提出 clip trim/fade/gain/envelope 建议。
- 建议必须转成 typed pending，不直接写工程。
- pending 中必须包含：
  - tool name
  - target ids
  - before value
  - after value
  - time range
  - evidence refs
  - risk
  - rollback hint
- Agent 不直接拼 raw kernel command。
- Agent 可以设计曲线，但只能通过 `automation.curve.generate` 生成受控 points plan，再经 typed tool 写入。
- Agent 不默认添加 fade；只有用户确认、或明确 clip 编辑操作产生切口风险时才写。
- 对 full-length stem 工程，默认保持对齐，不自动裁剪、不批量 fade。
- 速度、warp、pitch、tempo map 属于高风险编辑，必须独立确认。

## 9. 实现切分建议

### Phase A：Clip Fade/Gain 闭环

后端：

- `ClipService::handleSetClipFade`
- `ClipService::handleReadClipFade`
- `ClipService::handleSetClipGain`
- `ClipService::handleReadClipGain`
- `CommandDispatcher` 注册命令
- VSP compact snapshot 暴露 clip fields：
  - `clip_gain_db`
  - `clip_pan`
  - `clip_mute`
  - `fade_in_seconds`
  - `fade_out_seconds`
  - `fade_in_curve`
  - `fade_out_curve`
  - `fade_in_behaviour`
  - `fade_out_behaviour`
  - `auto_crossfade`

GUI：

- `clip_client.gd` 增加 fade/gain 方法。
- `clip_3d_container.gd` fade hit zone 写入命令。
- timeline refresh 后保留选择与滚动位置。

Agent：

- tools catalog 增加 `clip.fade.set/read`、`clip.gain.set/read`。
- harness 转译到 kernel command。
- EPM pending 可引用这些工具。

验收：

- GUI 手动拖 fade 后，重刷工程仍显示 fade。
- Agent 生成 fade pending，用户确认后 GUI 可见。
- clip gain 设置后 snapshot 可读，撤销/回滚可恢复。

### Phase B：Automation Target Registry

- 建立统一 target id。
- track volume/pan、plugin params 先进入 registry。
- read-only list 先行，避免 LLM 猜参数 ID。

### Phase C：Track/Plugin Automation Points

- 接 `AutomationCurve`。
- 支持 read/write/replace_range/simplify。
- 先做 offline point writing，不做实时 automation recording。

### Phase D：Clip Envelope / Crossfade

- clip gain envelope moves with clip。
- crossfade explicit create/clear。
- 可选 auto crossfade 设置。

### Phase E：Speed / Pitch / Warp / Tempo Map

- speed ratio、pitch semitone、auto tempo、warp markers、tempo map。
- 需要重新验证 transport、waveform、render、EPM alignment protection。

### Phase F：Modulation

- 预留 `automation.modulation.*`。
- 对标 Ableton：modulation 是 relative influence，不覆盖 absolute automation。
- 可映射到 Tracktion Engine modifier source。

## 10. 风险

- automation point 的值域必须来自 target registry，不能让 LLM 猜单位。
- clip gain 与 track volume automation 必须在 UI 和 Agent 回复里明确区分。
- fade/crossfade 不应默认批量作用于整首 stem。
- speed/warp 会破坏多轨相位与对齐，必须纳入 EPM 对齐保护。
- automation lane 可能导致高频 project refresh，需要 GUI 虚拟化/脏区刷新。
- plugin parameter automation 必须处理插件参数 ID 稳定性问题。

## 11. 当前目标验收结论

本设计完成后，可以进入下一个实现目标：

`实现 Clip Fade/Gain 工具层与 GUI 闭环 v0`

不建议下一步直接做完整 automation lane。先把 clip fade/gain 的后端、snapshot、GUI、Agent typed tool 闭环跑通，再扩展到 automation target registry 和 track/plugin automation。

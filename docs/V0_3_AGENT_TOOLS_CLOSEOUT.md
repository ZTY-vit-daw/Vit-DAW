# VitDAW Agent v0.3 工具收口清单

日期：2026-05-20

这份文档用于收口当前 v0.3 的“基础 DAW 工具”开发：记录已经注册的工具、已经手动验证过的闭环、需要固定回归的自然语言句子，以及进入 v0.4 插件抓手前还需要记住的缺口。

v0.3 的目标不是完整自然语言 planner，而是先跑通这条基础链路：

`Ask Vit 聊天 -> 本地意图 / 结构化命令 -> 引用解析 -> policy / 确认 -> 内核命令 -> shadow refresh / UI 更新`

内核仍然是工程真源。Agent shadow 状态和自然语言 hint 只用于 grounding、查询和推演，不作为直接改写工程的执行路径。

## 当前工具面

核对来源：

- `GET /agent/tools`
- `agent/internal/tools/catalog.go`

### Project

已注册工具：

- `project.ping` -> `ping`
- `project.state` -> `get_project_state`
- `project.health` -> `project_health_check`
- `project.recent` -> `get_recent_projects`
- `project.undo` -> `undo`
- `project.redo` -> `redo`
- `project.save` -> `save_project`
- `project.save_as` -> `save_as_project`
- `project.open` -> `open_project`
- `project.load` -> `load_project`
- `project.reload` -> `reload_project`
- `project.new` -> `new_project`
- `project.clear` -> `clear_project`

v0.3 状态：

- `project.state` 是 shadow 刷新的核心路径，并且会在用户可见摘要里隐藏内核默认轨道。
- `project.undo` / `project.redo` 已作为 undoable 工具注册。
- 会修改文件或整体工程状态的 project 操作需要确认。
- open/save/new/clear 已进入工具面，但不是当前 v0.3 手动编辑闭环的重点；后续测试时应使用 disposable project。

### Transport

已注册工具：

- `transport.play`
- `transport.stop`
- `transport.return_to_zero`
- `transport.seek`
- `transport.toggle_click`
- `transport.set_click`
- `transport.set_tempo`
- `transport.record.start`
- `transport.record.stop`
- `transport.option_stop_return_to_start`

v0.3 状态：

- 低风险 transport 命令直接执行。
- tempo 修改是 undoable。
- `recording_stopped` / `transport.record.stop` 后需要刷新 shadow，避免录音生成的工程结构不同步。

### Track

已注册工具：

- `track.list`
- `track.add`
- `track.add_audio`
- `track.append_ghost`
- `track.rename`
- `track.delete`
- `track.mute`
- `track.solo`
- `track.arm`
- `track.volume`
- `track.freeze`
- `track.unfreeze`
- `audio.route_wave_input_to_track`

v0.3 状态：

- 用户可见轨道序号和内核原始 track ID 已分离；内核默认轨道不会暴露给普通 agent 回复。
- `track.rename`、`track.mute`、`track.solo` 已通过 agent 手动验证。
- Godot 轨道头的 mute / solo 按钮已经接入内核路径。
- 新建轨道与用户可见轨道索引已经手动验证。
- delete / freeze 这类高风险操作仍然需要确认。

### Clip

已注册工具：

- `clip.select`
- `clip.import_audio`
- `clip.import_media_to_track`
- `clip.add_audio`
- `clip.move`
- `clip.resize`
- `clip.split`
- `clip.clone`
- `clip.remove`
- `clip.warm_waveform_bake`

v0.3 状态：

- 已通过 Ask Vit / agent 手动验证：`clip.select`、`clip.import_media_to_track`、`clip.move`、`clip.resize`、`clip.split`、`clip.clone`、`clip.remove`。
- Godot 剪刀 / cut 工具已接到内核 `split_clip`。
- clip 引用解析支持 selected clip、当前/选中轨道、用户可见轨道序号、显式 clip ID / name、`clip_index`。
- “当前轨道的一个 clip”“track 1 轨道的一个 clip”这类泛指不会再被当成假的 clip 名称，而是通过 shadow 状态解析。
- 如果某条轨道里有多个候选 clip，agent 不会乱猜，应要求用户进一步指定。
- 绝对路径导入和资料库已选中素材导入已经验证；按不完整素材名进行 fuzzy search 仍是后续任务。

## 安全策略核对

直接执行：

- project 读取类操作
- transport 播放、停止、定位、click
- clip 选择
- plugin 参数读取

无需确认但应可 undo：

- 小型 track 编辑，例如 rename / mute / solo / arm / volume
- tempo 修改
- undo / redo

必须确认：

- 删除 track 或 clip
- 移动、裁切、切分、复制、导入 clip
- 打开、清空、重载工程
- 文件写入、导入、渲染、资产回灌
- rack、plugin、MIDI、批量图编辑

每个修改型动作都应该带：

- `agent_action_id`
- journal 记录
- risk metadata
- 支持时带 `undo_label`

## 手动回归清单

前置条件：

- 用正常 release 环境启动 `VitApp.exe` 内核。
- 启动 `D:\Vit_DAW\agent\bin\VitAgent.exe`。
- 打开 Godot 编辑器或打包 UI，并确保 UDP `4444/4445` 路径连通。
- 至少创建一条用户可见轨道，并在 Track 1 上放一个音频 clip，用于泛指 clip 选择测试。

冒烟检查：

- `GET http://127.0.0.1:7878/health` 返回 `{"status":"ok","service":"VitAgent"}`。
- `GET http://127.0.0.1:7878/agent/tools` 能列出 project / transport / track / clip 工具。
- `GET http://127.0.0.1:7878/agent/state` 只显示用户可见轨道，内核默认轨道不应出现。

Track 句子：

- `新建一条轨道`
- `把 track 1 改名为 Drums`
- `静音 track 1`
- `取消静音 track 1`
- `solo track 1`
- `取消 solo track 1`
- `列出当前轨道`

Clip 句子：

- `选中当前轨道的一个clip`
- `选中track 1轨道的一个clip`
- `选中这个 clip`
- `把选中的clip移动到20秒`
- `把这个音频移动到播放头`
- `把这个音频裁到3秒`
- `把选中的clip在1秒处切开`
- `在这里切开这个音频`
- `复制选中的clip到后面`
- `把这个音频复制到播放头`
- `删除选中的clip`
- `把资料库选中的音频导入当前轨道`
- `import C:\Samples\Kick Loop.wav to current track`

UI 检查：

- 轨道头 mute 按钮能改变内核状态，并同步 UI 状态。
- 轨道头 solo 按钮能改变内核状态，并同步 UI 状态。
- 连续新建轨道后，轨道头电平区域显示正常，不出现裸 `id?`。
- 剪刀 / cut UI 工具通过 `split_clip` 正常切开 clip。
- agent 执行 clip 修改后，工程视图会刷新。

安全检查：

- `clip.select` 直接执行。
- clip import / move / resize / split / clone / remove 会先出现确认预览。
- 用户确认后，journal 能看到对应 action。
- 内核支持 undo 时，可以撤回最近一次确认执行的编辑。

## v0.4 前已知缺口

- Ask Vit 的确认预览仍然过于技术化，会暴露 JSON / ID。未来应改成类似“将 test_target_3s 导入到 Track 1”的人类文本，技术细节折叠隐藏。
- 当前自然语言层仍是 v0.3 本地 intent bridge，不是最终 LLM planner。最终目标应是 `NL -> planner -> tool plan -> reference resolver -> policy -> execute`。
- 资料库导入的 fuzzy search 还没有完整实现。绝对路径和资料库点选导入可用，不完整素材名需要 library index / glob / grep 流程。
- `clip.add_audio` 和 `clip.warm_waveform_bake` 已注册，但如果要作为 Ask Vit 用户操作，需要单独回归。
- project save / open / new / clear 已注册并有 policy gate，但只能用 disposable project 谨慎测试。
- rack、MIDI、plugin grabber、render、assets 工作流不属于 v0.3 收口，应放到 v0.4+。

## v0.3 退出标准

满足以下条件时，可以认为 v0.3 基础工具阶段收口：

- 上面的手动回归清单在当前 release agent binary 上通过。
- `/agent/tools` 继续暴露 project / transport / track / clip 工具，并带正确 risk metadata。
- `project.state` 继续隐藏内核默认轨道。
- 所有确认执行的修改型操作都会写 journal。
- 普通聊天回复不要求用户理解或使用内核 track ID。

之后主线进入 v0.4 插件抓手：

- plugin parameter snapshot
- cutoff / resonance / drive / mix / attack / release / threshold / ratio 等语义别名
- grabber profile
- macro mapping
- control graph node creation
- plugin explanation
- common role learning

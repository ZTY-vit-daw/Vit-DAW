# Vit-DAW 项目白皮书

## 一句话定位

Vit-DAW 是一个面向 AI 时代的原生数字音频工作站：它以 Tracktion/JUCE 音频内核为底座，以 Godot 构建可进化的 3D/节点式界面，并计划通过独立 Agent 进程把自然语言、快捷键、工程状态、AIGC 工具、本地模型和外部插件统一编排进音乐生产流程。

它不是一个单纯的“AI 生成音乐按钮”，而是一个尝试把 DAW 变成音乐 IDE 的开源工程。

## 项目愿景

传统 DAW 的核心工作流仍然高度线性：作曲、编曲、录音、混音、母带、反馈、返工往往依赖人工传文件、导 stem、发压缩包和等待上游交付。Vit-DAW 的目标是把这些流程重新设计为可分支、可回滚、可对比、可由 Agent 协助执行的非线性音乐工程。

长期目标包括：

- 用 Godot UI 提供可持续重塑、可自定义、可交互的现代 DAW 界面。
- 用独立 Go Agent 进程统一连接 UI、音频内核、本地模型、云端模型和外部工具。
- 用 Git for Music 思路让 AI 操作进入沙箱、分支、Take 和 Merge，而不是直接破坏主工程。
- 用 MIDI ghost notes、AI Instrument Builder、Stem Separation、Hum-to-MIDI 等功能把 AI 变成创作过程中的低延迟助手。
- 用开放的工具封装协议，让用户把自己的 AIGC 平台、网页工具、本地脚本、模型和插件接入机架。

## 当前架构概览

当前仓库已经形成了可运行的三层雏形：

- 音频内核：`VitApp/`
  基于 Tracktion Engine + JUCE 的 headless 服务，负责工程、走带、音频导入、轨道状态、插件/机架命令、遥测和谱图烘焙。

- 前端界面：Godot 工程
  负责多轨时间线、走带控制、3D 波形/频谱视图、资料库拖拽、机架 UI、IPC 客户端和可视化状态分发。

- 桥接层：`scripts/bridge_core.py`
  当前负责 Godot UDP 与内核 ZMQ 的桥接、遥测转发、控制命令转发、超时保护、影子工程状态和诊断日志。未来计划被 Go Agent 逐步替代。

未来目标运行形态：

```text
ui.exe      Godot UI / 交互 / 可视化
agent.exe   Go Agent / 命令验证 / 模型路由 / 任务编排 / 工具生态
kernel.exe  Tracktion-JUCE 内核 / 实时音频 / 工程真源 / DSP
```

## 当前已实现能力

### 1. Tracktion/JUCE Headless 音频内核

状态：已实现。

当前 `VitApp` 已经可以作为无头音频服务运行，Release 构建可产出 `VitApp.exe` / `VitHeadlessServer`。内核维护 Tracktion Edit 工程，并通过 ZMQ 暴露 JSON 命令接口。

已实现能力包括：

- 工程加载、重载、清空、状态查询。
- 走带控制：播放、停止、回零、定位。
- 节拍与节拍器相关命令。
- 音频轨添加、删除、静音、音量控制。
- 音频导入与 clip 添加。
- 工程全量快照 `get_project_state`。
- 最近工程、保存、另存、加密工程相关契约。
- Undo/Redo 命令。

### 2. ZMQ / UDP IPC 控制链路

状态：已实现，仍需后续收敛为 Agent 统一协议。

当前内核通过：

- ZMQ REP `tcp://127.0.0.1:5555` 接收 JSON 命令。
- ZMQ PUB `tcp://127.0.0.1:5556` 发布遥测、日志和扩展事件。
- Python Bridge 将 Godot UDP 命令转发到内核 ZMQ，并将内核遥测转发回 Godot。

这使 Vit-DAW 具备“外部系统可控”的基础：UI、脚本、测试工具和未来 Agent 都可以通过结构化 JSON 操作工程。

### 3. Godot 多轨 UI 与 3D 可视化

状态：已实现雏形，视觉系统仍需长期重构。

当前 Godot 前端已经具备：

- 主界面布局。
- 多轨列表。
- 轨道头基础控制。
- 新建/删除轨。
- 走带栏。
- 空格播放控制。
- 本地资料库浏览。
- 音频拖拽导入。
- 3D 波形/频谱瓦片视图。
- 时间标尺、HUD、相机缩放与滚动。
- 基础 LLM 聊天窗口雏形。

现阶段 UI 已经能支撑演示和功能联调，但还不是最终产品视觉。后续重点是设计系统、鼠标胶囊提示、快捷键栏、Agent Shell、主题化和用户自定义 UI。

### 4. 谱图烘焙与共享内存传输

状态：已实现。

内核端已经实现 `TiledSpectrogramBaker`：

- 后台读取音频文件。
- 执行 STFT 与对数频轴映射。
- 将结果写入 Windows 命名共享内存。
- 通过 `tile_ready` / `track_duration_ready` 通知前端。

Godot 侧通过 GDExtension `VitWaveformReader` 读取共享内存并构建纹理，用于 3D 波形和频谱显示。

### 5. Python Bridge 与影子工程

状态：已实现，未来计划迁移到 Go Agent。

`scripts/bridge_core.py` 当前承担：

- 遥测 SUB 到 Godot UDP 的转发。
- Godot UDP 到 ZMQ REQ 的控制转发。
- 请求超时与 ZMQ socket 重建。
- `get_project_state` 后的 Python 侧影子工程初始化。
- delta 更新合并。
- delta 序号间隙日志。
- tile_ready 诊断日志。
- recording stopped 后强制刷新工程快照。

这些能力未来会成为 Go Agent 状态同步和命令验证层的重要参考。

### 6. MIDI 与 2D Rack / AIGC / Take 命令基础

状态：底层命令能力已落地，完整产品体验仍在规划中。

当前 IPC 契约中已经记录并实现了多类高级命令，包括：

- MIDI clip 插入。
- MIDI notes 添加、批量添加、修改、删除、读取。
- Rack 节点添加。
- Rack pin 连接与断开。
- Clip scope 路由。
- AIGC job 注册。
- 生成资产回灌。
- Take 切换。
- 异步 ghost 状态设置。

这说明 Vit-DAW 已经具备“AI 生成结果进入工程”的底层抓手。后续需要把这些命令整合进 Go Agent、UI 操作、任务队列和 Git for Music 工作流。

### 7. 路由宪法与机架语义

状态：核心语义已设计并部分落地。

Vit-DAW 已经建立了明确的 Rack 路由原则：

- 横线表达 Tracktion `RackType` 执行 DAG。
- 竖线表达 clip 到插件节点的事件来源。
- `RACK_INPUT` / `RACK_OUTPUT` 作为母线锚点。
- MIDI pin 与 audio pin 有明确映射。
- `rack_set_node_clip_scope` 用于 clip 级作用域绑定。

这一设计为未来的插件节点、AIGC 节点、Bridge Injector、AI Instrument、Stem 和自封装工具提供了统一的机架语义。

## 当前尚未实现或仍在规划中的能力

以下内容属于路线图目标，不应被理解为当前已完成能力。

### 1. Go Agent 三进程架构

状态：规划中。

目标是用 `agent.exe` 替代当前 Python Bridge，并承担：

- UI 与内核之间的统一通信层。
- JSON schema 验证。
- 命令权限与危险操作拦截。
- 工程影子状态。
- 本地/云端模型路由。
- AIGC、Stem、Hum-to-MIDI、Automation 等任务编排。
- Agent 对话框和鼠标胶囊提示的上下文来源。

### 2. UI 设计系统与智能交互

状态：规划中。

规划内容包括：

- 高保真产品化视觉系统。
- 鼠标胶囊提示 UI。
- `C` 键呼出 Agent 对话框。
- `/` 键呼出全局快捷键设置栏。
- Command Registry。
- Shortcut Registry。
- 用户自定义主题、布局和快捷键。

### 3. 本地模型矩阵与算力校准

状态：规划中。

目标是围绕 `0.7B` 到 `8B` 的本地微调模型建立可选部署体系：

- 本地硬件探测。
- 短 benchmark。
- 模型兼容矩阵。
- 模型下载、校验、删除、回滚。
- 用户手动覆盖推荐。
- 本地模型与云端 API 的自动路由。

### 4. MIDI Ghost Notes 与多轨上下文 MIDI Model

状态：规划中。

这是 Vit-DAW 的核心护城河之一。目标不是简单续写单轨 MIDI，而是发展为 arrangement-aware completion：

- 理解鼓、bass、chord、melody、人声之间的关系。
- 在钢琴卷帘窗中生成半透明 ghost notes。
- 用户按 Tab 接受。
- 可撤销、可拒绝、可记录用户偏好。
- 可在本地小模型与云端大模型之间切换。

### 5. Vit AI Instrument Builder

状态：规划中。

这是 AI 乐器方向的长期功能，拆成两部分：

- `Neural Timbre Engine`
  负责用 DDSP、RAVE、MusicGen 派生模型或未来更好的开源模型模拟音色。目标是允许用户用自录采样复刻自己的乐器或音色。

- `Performance Agent`
  负责推理真实演奏法，把普通 MIDI 转成带弓法、气口、连奏、断奏、滑音、颤音、力度曲线和人性化 timing 的 expressive performance。

这一方向的市场逻辑是：商业云端 AI 乐器可以很贵，但 Vit-DAW 可以用免费、开源、本地可控和用户自封装的方式提供替代路径。

### 6. Vit Tool Plugin Protocol

状态：规划中。

目标是提供一个低门槛工具封装协议，让用户把以下内容接入 Vit 机架：

- 网页端 AIGC 平台。
- 本地脚本。
- 命令行工具。
- REST API。
- 本地模型。
- 音频分析器。
- MIDI 生成器。
- 自定义 Agent provider。

它的定位不是取代 VST3/CLAP，而是让更多非传统插件形态的工具能进入音乐工程。

### 7. Git for Music 与 AI 原生工程元数据

状态：规划中。

目标是让 AI 操作不直接覆盖主工程，而是进入可比较、可回滚、可合并的音乐分支：

- AI 沙箱 bus。
- A/B/C 候选分支。
- Take history。
- Merge 回主线。
- Agent 操作日志。
- Prompt、model、seed、provider、asset metadata。
- 可重生成 / 可替换资产。
- 缺失依赖 ghost placeholder。

### 8. Stem Separation / Hum-to-MIDI / AIGC 外部吸盘

状态：规划中，底层回灌命令已有基础。

规划目标包括：

- 右键 clip 一键拆分人声、鼓、bass、伴奏。
- 哼唱转 MIDI。
- 外部 AIGC 音频一键回灌工程。
- 使用 take history 管理多个生成结果。
- 通过 Agent task/provider 统一展示任务状态。

### 9. 高级音频监督与自动化实验室

状态：远期规划。

规划内容包括：

- Haas Effect Monitor。
- 3D 相位/声像/频段冲突预警。
- 乐谱双向翻译。
- Automation Skill Scripts。
- 多普勒、磁带停机、泵感侧链等自动化脚本生成。

### 10. 异地协同与数字唱片式分享

状态：远期规划，需要网络中继、P2P 或云端服务，不属于纯本地基础能力。

规划方向包括：

- Agent 侧远程协同会话。
- 发起者权限管理。
- 非线性音乐协作。
- `.vitbranch`、`.vitpatch`、`.vitlink` 或 manifest。
- 工程分享链接。
- PNG/SVG 数字唱片式封面分享。
- 二维码、短码、哈希、签名和压缩损坏回退。

## 产品差异化

Vit-DAW 的核心差异不是“把 AI 放进 DAW”，而是把 DAW 改造成一个可由 Agent 理解和操作的音乐工程系统。

### AI-Native DAW Interface

UI 不只是按钮和轨道，而是未来 Agent 获取上下文、提示用户、执行任务和显示结果的交互层。

### Agent-Orchestrated Music Workflow

Agent 不是聊天窗口附属品，而是未来连接 UI、内核、模型、插件、AIGC、自动化和工程状态的中枢。

### Regenerable / Branchable Music Project

工程不是静态文件，而是可以记录来源、分支、Take、生成参数、Agent 操作和可重生成资产的音乐项目。

### Open Neural Instrument Ecosystem

Vit-DAW 计划支持用户训练、替换、封装和加载自己的神经音源，而不是被昂贵云端 AI 乐器服务绑定。

## 面向用户的价值

对独立音乐人：

- 用免费、开源、本地可控的方式获得 AI 协助。
- 不必依赖庞大采样库或昂贵云端服务。
- 可以把自己的工具、模型、音色接入 DAW。

对制作人与编曲师：

- 通过 Agent 加速工程操作。
- 通过 MIDI ghost notes、Stem、Hum-to-MIDI 降低创作阻塞。
- 通过 Git for Music 对比多个编曲/混音分支。

对开发者与研究者：

- 可以基于 JSON IPC、Rack 节点、Agent provider 和工具封装协议扩展系统。
- 可以把新的 AI 音频模型、MIDI 模型、神经音色模型接入真实 DAW 工作流。

## 当前项目状态总结

当前 Vit-DAW 已经完成了最关键的底层雏形：

- 可运行的 Tracktion/JUCE headless 音频内核。
- 可用的 ZMQ/UDP IPC 控制链路。
- Godot 多轨 UI 与 3D 波形/频谱显示。
- Windows 共享内存谱图瓦片管线。
- Python Bridge 与影子工程。
- 基础工程、轨道、音频、MIDI、Rack、AIGC 回灌、Take/Ghost 相关命令。
- 路由语义和 IPC 契约文档。

但它还不是完整商业 DAW，也还没有完成最终 AI Agent 架构。本项目下一阶段的核心不是堆更多 AI 功能，而是先完成 UI 设计系统、Go Agent 三进程架构、命令注册表、快捷键体系、状态可靠性和可验证 IPC。

## 路线图摘要

当前版本规划保持在 `V0.x`，`V1.0` 以后视为正式发布后版本。

- `V0.81`：UI/交互设计系统。
- `V0.82`：Go Agent 三进程架构。
- `V0.83`：快捷键、命令注册表、鼠标胶囊提示。
- `V0.84`：高级编辑基础与 IPC 可靠性。
- `V0.85`：本地模型矩阵、算力校准、混合路由。
- `V0.86`：MIDI ghost notes、AI Instrument Builder、Stem、Hum-to-MIDI、AIGC 回灌。
- `V0.87`：Hermes Memory、Git for Music、AI 原生工程元数据。
- `V0.88-V0.89`：高级音频监督、乐谱翻译、自动化实验室。
- `V0.891+`：异地协同、工程分享格式、数字唱片式分享。

## 结语

Vit-DAW 的长期目标，是成为一个由开源社区共同塑造的 AI 原生音乐工作站。它尊重传统 DAW 的工程精度，也试图把 Agent、神经音源、可重生成资产、分支化工程和工具封装协议带入真实音乐生产。

当前版本已经证明了核心链路可行：Godot UI 可以控制 Tracktion/JUCE 内核，内核可以输出工程状态和谱图数据，外部脚本可以通过 JSON IPC 操作工程。下一步，是把这些能力收束成稳定、可扩展、可被用户理解的产品形态。

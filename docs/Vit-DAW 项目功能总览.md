# Vit-DAW 项目功能总览

本文档概括当前仓库内**已实现**的主要能力，便于新人、演示与发布对齐。细节以代码与 [`docs/VIT_IPC_CONTRACT.md`](VIT_IPC_CONTRACT.md) 为准。

---

## 1. 仓库组成

| 区域 | 路径 | 作用 |
|------|------|------|
| 内核（无头服务） | `VitApp/` | Tracktion Engine + JUCE 控制台应用 `VitHeadlessServer`，ZMQ 网关、工程与走带、谱图烘焙、遥测发布 |
| 前端（Godot） | `D:\Godot\project\vit-daw-frontend\`（或工作区映射） | 多轨时间线、3D 波形/频谱视图、走带 UI、资料库拖拽、IPC 客户端 |
| GDExtension | `extension/` | 注册 `VitWaveformReader`，从 Windows 共享内存读取烘焙瓦片数据供 Godot 使用 |
| 桥接 | `scripts/bridge_core.py`、`bridge_prod.py`、`godot_bridge.py` | ZMQ ↔ UDP，连接内核与 Godot |
| 工程与脚本 | `scripts/`、`calibration/`、`docs/` | 校准、诊断、发布清单、IPC 契约与复盘文档 |
| 第三方引擎 | `tracktion_engine/` | Tracktion / JUCE 依赖树 |

---

## 2. 内核端（VitApp）已实现功能

### 2.1 构建与运行形态

- CMake 工程，Release 可产出 `VitApp.exe`（产品名 VitHeadlessServer）。
- 默认工程 XML：`Workspace/default_project.xml`；可通过环境变量覆盖（见 `VitPaths.h`：`VIT_PROJECT_XML` 等）。

### 2.2 网络与 IPC

- **ZMQ REP**：`tcp://127.0.0.1:5555`，接收 JSON 命令并返回 JSON 应答。
- **ZMQ PUB**：`tcp://127.0.0.1:5556`，推送遥测与扩展事件（如 `tile_ready`、自定义 `command` 字段）。
- 命令在消息线程解析，**Edit/走带相关变更在 JUCE 消息线程执行**；网关侧有等待超时保护。

### 2.3 已实现 JSON 命令（`CommandDispatcher`）

包括但不限于：

- **走带**：`play`、`stop`、`return_to_zero`、`seek`、`transport_option_stop_return_to_start`
- **工程/查询**：`ping`、`reload_project`、`list_tracks`、`get_project_state`、`set_tempo`、`clear_project`
- **轨道**：`add_audio_track`、`delete_track`（**仅**按 `track_id`）、`append_ghost_track`
- **音频**：`add_audio_clip`、`import_audio`（导入后触发谱图烘焙）
- **轨参数**：`set_volume`、`set_mute`
- **节拍器**：`toggle_click`、`set_click`

完整字段与语义见 `docs/VIT_IPC_CONTRACT.md`。

### 2.4 遥测

- **transport**：`is_playing`、`position_seconds` 等。
- **levels**：各音频轨 `level_db`（依赖 LevelMeter 等插件注入）。
- **烘焙通知**（经 PUB 转发到前端 UDP）：`track_duration_ready`、`tile_ready`（含共享内存名、tile 索引、时长等）。

### 2.5 谱图烘焙（TiledSpectrogramBaker）

- 后台线程读取音频，STFT + 对数频轴映射到固定纹理布局（与 Godot `telemetry_manager` 常量一致）。
- 输出写入 **Windows 命名共享内存**；Godot 侧经 `VitWaveformReader` 读入并贴图。
- 支持环境变量诊断模式（如 `VIT_BAKER_DEBUG`、`VIT_BAKER_MODE` 等，见 `TiledSpectrogramBaker.cpp`）。
- 诊断追加日志默认写入 `VitApp/Workspace/Logs/baker_diag.log`（启用诊断时）。

### 2.6 日志与诊断

- 日期戳会话日志：`Workspace/Logs/VitHeadlessServer*.log`。
- ZMQ 侧日志队列与发布（另有 `5557` 等端点定义，按 `ZmqGateway` 实现为准）。
- 可选：`VIT_ENABLE_SHARED_MEMORY_TEST=1` 时创建测试共享内存映射（默认关闭）。

---

## 3. 桥接层（Python）

- **核心**：`scripts/bridge_core.py` — 遥测 SUB → UDP 4444；控制 UDP 4445 → ZMQ REQ，带超时与 REQ 套接字重建。
- **发布入口**：`scripts/bridge_prod.py` — 默认静默，滚动写入 `bridge_last.log`；支持 `bridge_prod.config.json` / 环境变量覆盖端口与超时（见 `bridge_prod.config.example.json` 与发布清单文档）。
- **开发入口**：`scripts/godot_bridge.py` — 同核心，verbose 与独立 dev 日志文件。

---

## 4. Godot 前端已实现功能

### 4.1 全局与 IPC

- **VitIpcClient**：UDP 与桥通信，异步命令、超时、与主控握手（如启动 `ping`、`clear_project`、`list_tracks`）。
- **VitTrackIdRegistry**：场景轨 `Track_Root_*` 与内核 `track_id` / `list_tracks` 对齐。
- **VitTelemetryManager**：UDP 4444 收包，分发 `levels` / `transport` / `tile_ready` / `track_duration_ready`。

### 4.2 主界面与多轨

- **vit_control_v_1.0**：主布局、浏览器/机架切换、多轨列表、新建/删除轨、全局时间轴控制器、空格走带转发、LLM 聊天窗口（系统窗口）等。
- **Track_Row_Controller**：单轨 UI（MSR、音量声像、高度拖拽）、拖入音频 → `import_audio`、接收瓦片纹理与时长、与机架联动。

### 4.3 3D 波形与频谱

- **tile_manager**：瓦片网格、shader 材质、`view_mode`（时域/频域/总览）、与 transport 注入对齐；频域下切片与 transport 解耦（见复盘文档）。
- **Turntable_Camera / right_3d_wrapper**：相机、缩放、滚动、HUD 与标尺、输入转发与走带同步。
- **time_ruler / HUD**：时间轴与频域刻度、振幅标尺等。

### 4.4 走带栏

- **top_transport_bar**：播放/停止/回零、Stop 回起点选项、LCD 时间显示、与内核遥测同步播放按钮外观等。

### 4.5 资料库与机架

- **left_library_dock**：本地 Places 目录树、音频文件拖拽负载。
- **Vit_Graph_Rack**：GraphEdit 机架、节点拖放/连线/删除、右键菜单；当前打印的 ZMQ 载荷多为**演示/占位**，是否与后端真实服务对齐需单独对接。

### 4.6 调试与静默

- 各模块 `debug_ui_log`（导出）+ **全局** `VitDebugFlags`（autoload）：`VIT_UI_LOG` 环境变量与 `Workspace/Commands/debug_flags.json` 热切（见 `docs/Vit-DAW 调试开关热切指令.md`）。

### 4.7 说明：前端已发、内核未注册的命令

顶栏等位置可能发送如 `transport_record_arm`、`transport_loop` 等 **action**；当前 **VitApp `CommandDispatcher` 中无同名 handler**，内核会返回未知命令类错误。若需要真实录音/循环走带，需在 C++ 侧补命令实现。

---

## 5. GDExtension

- `VitWaveformReader`：按共享内存名读取 float 缓冲，供 `telemetry_manager.gd` 构建 `ImageTexture`。

---

## 6. 脚本与校准资产

- `scripts/`：桥接、监控、ZMQ 测试、立体声频校准（`stereo_freq_calibrator.py` 等）、多步 ground-truth / EXR 管线脚本等。
- `calibration/`、`calibration_tmp/`：运行报告与观测数据（CSV/JSON/Markdown）。

---

## 7. 文档索引（`docs/`）

| 文档 | 内容 |
|------|------|
| Vit-DAW 项目功能总览.md | 本文：能力总览 |
| Vit-DAW 发布模式一键启动配置清单.md | 启动顺序、Bridge/UI/内核发布配置 |
| Vit-DAW 调试开关热切指令.md | 无 UI 后台切换 UI 日志 |
| Vit-DAW 展示版发布前检查清单.md | 演示前检查项 |
| Vit-DAW 频谱漂移问题复盘.md | 频域漂移根因与修复要点 |
| VIT_IPC_CONTRACT.md | IPC 契约正文（Godot ↔ Bridge ↔ ZMQ REQ） |
| Vit-DAW_V1.1_Roadmap.md | V1.1 核心攻坚阶段开发蓝图 |
| TRAE_MIGRATION_PROMPT.md / TRAE_MIGRATION_SKILL.md | 迁移相关说明 |

仓库根目录的 **`VIT_IPC_CONTRACT.md`** 仅保留跳转说明，指向上表中的契约正文。

---

## 8. 未在本文展开但存在的目录

- `baked_tiles*`、`spectrum_data.exr` 等：数据与烘焙产物。
- `docs/TRAE_MIGRATION_*.md`：迁移相关说明。

---

*文档版本：与当前仓库实现同步整理；若新增命令或 UI，请同步更新本节与 `docs/VIT_IPC_CONTRACT.md`。*

# Vit-DAW 发布模式一键启动配置清单

## 1) 目标

- 发布模式默认安静（UI/Bridge/Kernel 控制台无调试刷屏）。
- 出问题时保留最后日志（可快速定位）。
- 一键启动顺序固定，避免桥接时序抖动。

## 2) 启动顺序（发布模式）

1. 启动内核 `VitApp.exe`。
2. 启动桥接 `python scripts/bridge_prod.py`。
3. 启动 Godot UI。

## 3) 内核发布配置

- 不设置 `VIT_ENABLE_SHARED_MEMORY_TEST`（默认关闭测试共享内存注入）。
- 不设置 `VIT_BAKER_DEBUG`（默认关闭 baker 诊断日志）。
- 如需诊断：
  - 临时设置 `VIT_BAKER_DEBUG=1`。
  - 诊断日志写入 `VitApp/Workspace/Logs/baker_diag.log`。

## 4) Bridge 发布配置

- 使用 `scripts/bridge_prod.py`，不要使用开发入口 `scripts/godot_bridge.py`。
- 配置优先级：**环境变量 > `scripts/bridge_prod.config.json` > 代码默认值**。
- 可参考模板：`scripts/bridge_prod.config.example.json`（复制为 `bridge_prod.config.json` 后修改）。
- 默认参数：
  - REQ 超时：2000ms
  - 重试次数：1
  - 最后日志文件：`VitApp/Workspace/Logs/bridge_last.log`
- 日志策略：
  - 默认不输出详细日志到控制台。
  - 仅错误和关键告警可见。

### 可用 Bridge 环境变量

- `VIT_BRIDGE_CONFIG`
- `VIT_BRIDGE_ZMQ_SUB_URL`
- `VIT_BRIDGE_ZMQ_REQ_URL`
- `VIT_BRIDGE_GODOT_IP`
- `VIT_BRIDGE_UDP_TO_GODOT`
- `VIT_BRIDGE_UDP_FROM_GODOT`
- `VIT_BRIDGE_REQ_TIMEOUT_MS`
- `VIT_BRIDGE_REQ_MAX_RETRIES`
- `VIT_BRIDGE_KEEP_LAST_LOG_LINES`
- `VIT_BRIDGE_LAST_LOG_PATH`
- `VIT_BRIDGE_VERBOSE`

## 5) UI 发布配置

- `track_row.tscn` 中以下开关应保持 `false`：
  - `Right_3D_Wrapper.debug_print_freq_runtime`
  - `Right_3D_Wrapper.debug_frontend_diag`
  - `Turntable_Pivot.debug_print_freq_runtime`
  - `Turntable_Pivot.debug_frontend_diag`
  - `Tile_Manager.debug_print_slice_params`
  - `Tile_Manager.debug_frontend_diag`
  - `HUD_Y_Left.debug_frontend_diag`
  - `HUD_2D_Overlay.debug_frontend_diag`
- 新增 UI 日志开关默认关闭：
  - `right_3d_wrapper.gd` -> `debug_ui_log = false`
  - `Turntable_Camera.gd` -> `debug_ui_log = false`
  - `tile_manager.gd` -> `debug_ui_log = false`

### UI 全局日志后台开关（无需 UI 页面）

- Autoload 单例：`res://VitDebugFlags.gd`
- 环境变量：
  - `VIT_UI_LOG=1` 开启全 UI 日志
  - `VIT_UI_LOG=0` 关闭全 UI 日志
- 命令文件热切（Godot 运行中可改）：
  - 默认读取：`D:/Vit_DAW/VitApp/Workspace/Commands/debug_flags.json`
  - 内容示例：`{"ui_log_enabled": true}`
  - 可用 `VIT_DEBUG_FLAGS_FILE` 指定自定义路径

## 6) 快速验收（1分钟）

- 内核进程启动后不崩溃。
- Bridge 启动后不报编码错误。
- UI 打开后无调试刷屏，播放/停止/seek 正常。
- 查看 `VitApp/Workspace/Logs/bridge_last.log` 文件存在。

## 7) 排障临时开关（仅开发时）

- UI 层：按需打开单个脚本 `debug_*` 开关，不要全量开启。
- Bridge 层：使用 `scripts/godot_bridge.py`（开发入口，verbose）。
- Kernel 层：仅在需要频谱诊断时设置 `VIT_BAKER_DEBUG=1`。

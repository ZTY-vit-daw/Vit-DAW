# Vit-DAW V1.1 核心攻坚阶段开发蓝图

## 一、 核心架构决策
1. **契约驱动与 UI 先行**: 先在 Godot 中用假数据搭出视觉与交互骨架，以此倒逼 C++ 内核提供必需的 ZMQ JSON 数据。
2. **多维生命体征仪表盘**: 在顶栏首创融合 `底层物理 (DSP/Buffer) + 视觉引擎 (FPS) + 神经中枢 (AI 状态)` 的三位一体动态仪表盘。
3. **按需扫描策略**: 抛弃启动全盘扫描，采用用户将文件夹拖入资料库才触发的 C++ 后台异步扫描。
4. **“三大一主”极简拓扑**: 确立 `Audio (音频)`、`MIDI (乐器)`、`Bus (总线)`、`Master (主输出)` 的轨道分类。Bus 必须在主时间轴占位以承载自动化曲线与冻结波形。

## 二、 极细化执行步骤 (Cursor 投喂指南)

### 第一阶段：纯前端 UI 视觉与交互冲刺 (Mock Data)
*不启动 C++ 内核，只在 Godot 里画图和写 GDScript*

* **Step 1：顶栏动态仪表盘 (Top Bar Dashboard)**
  * `top_transport_bar` 右侧实现音频硬件、性能监视、AI 状态灯。
  * 接入本地 FPS API，其余填假数据。
* **Step 2：轨道头拓扑升级 (Track Headers & Master)**
  * 重构 `Btn_AddTrack` 为下拉菜单 (Audio, MIDI, Bus)。
  * 重构单轨 UI (`Track_Row_Controller`)：根据类型切换颜色，添加 `Output` 路由下拉菜单。
  * 主视图底部硬编码不可删除的宽体 Master 轨道。
* **Step 3：全局设置模态窗口 (Global Settings Modal)**
  * 制作独占式配置面板 `GlobalSettingsModal.tscn`。
  * 纯 UI 绘制 Audio Tab 和 Plugins Tab。

### 第二阶段：底层 C++ 核心补全与 ZMQ 联调 (Real Data)
*UI 搭建完毕，依据契约深入 Tracktion C++*

* **Step 4：物理设备接管 (I/O Engine)**
  * C++ 端实现 `get_audio_devices`, `set_audio_device`。
  * 前端仪表盘和设置面板接入 C++ 真实数据。
* **Step 5：底层轨道拓扑与 VST 按需扫描**
  * C++ 修改 `add_track` 支持 `track_type`。
  * 实现 ZMQ 异步 VST 扫描队列。
* **Step 6：工程状态网 (Undo & Shadow XML)**
  * C++ 引入 `UndoManager`。
  * Python 桥接层开始接收增量推送，搭建 LLM 影子 XML。

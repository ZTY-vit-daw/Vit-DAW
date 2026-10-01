# VITNOTE-DOCK-MOUNT-1：便签层挂载补到 Vit-Dock 工作区（产品默认主场景）——M8 手测不可见的根因修复（P1）

- 池序 24（P1，M8 终裁前置）；**目标仓库=D:\Godot\project\vit-daw-frontend（仓库外 Godot 4 工程）——本卡登记在主仓 coord，实现工作全部在 Godot 仓**；来源=M8 手测受阻取证（决策侧 2026-10-01，锚点已核）
- 优先级 / 预估 / 依赖：P1 / 0.5 天 / **基于 port/vitnote-impl-1 分支（098cd25）续做**——便签三脚本与 legacy 挂载都在该分支，不在 main
- 模型分级：L1 / GLM 或 flash 可接（挂载模式已定+取证完备）
- 已核实事实（决策侧逐一锚点核过，勿重勘察）：
  1. 启动页进工程默认加载 **Vit-Dock 工作区**：`app/startup/start_page.gd:5` `MAIN_WORKSPACE_SCENE := "res://vit_dock/scenes/VitDockRoot.tscn"`；仅 `VIT_USE_LEGACY_MAIN=1` 时走 legacy（:997-999）。
  2. **便签管理层只挂在 legacy 主控台**：`app/legacy/vit_control_v_1.0.gd:155-158`（VIT_NOTE_MANAGER_SCRIPT.new()+add_child）；dock 场景没有 vit_control（`vit_dock/scenes/vit_dock_root.gd:1105` 注释原话「Dock 作主场景时没有 vit_control」）。
  3. dock 时间轴**复用同一套 track lane**（`vit_dock/views/timeline_dock_adapter.gd:5` 实例化 track_row.tscn）→ 用户拖选时 `_notify_vit_note_hint()`（vit_track_lane_2d.gd:2296-2298）确实触发，但组 "vit_note_manager" 查找为空 → 按设计「manager 缺位零影响」**静默无操作**；N 键动作的消费方同样在未挂载的管理层里。用户实测：拖选无胶囊、按 N 无反应——与机制逐点吻合。
  4. 文件无缓存陈旧问题（vitnote 文件 09-30 20:34 落盘，编辑器 10-01 18:29 才启动）。
- 目标：
  1. **dock 挂载**：vit_dock_root.gd `_ready` 按 legacy 同款模式挂 VitNoteLayer（preload vit_note_manager.gd+new+add_child，全幅 Control）；legacy 挂载保留不动。
  2. **context 供给适配**：vit_note_manager 的 context 靠祖先链找 `LLM_Chat_Window` 调 `_build_agent_context`——核验 dock 工作区内是否存在等价节点/入口（vit_dock_root.gd:2325 注释「供 VitAgentShell：与主控 vit_control 同入口名」是线索）；若 dock 无等价物，给 manager 补一条 dock 侧查找路径或降级为最小 context（如实申报降级面，不阻塞 M8 判据=框选→胶囊→开便签→中文输入→观察问答有回复）。
  3. **快捷键核验**：VitShortcutManager 是 autoload（dock 内亦在）——确认 vit_note_open 消费链在 dock 挂载后可用。
  4. **自验**：headless `--import` 零 SCRIPT ERROR+主场景 headless 运行确认 VitNoteLayer 入树（沿 IMPL-1 自验先例；跑完删临时脚本）；Godot 仓既有运行时目录勿动勿提交。
  5. 用户复验：修后在**默认入口**（无需环境变量）F5→进工程→拖框→胶囊→9 步清单（M8 判据）。
- 文件域：Godot 仓 `vit_dock/scenes/vit_dock_root.gd`（挂载点）+ `app/tracks/vitnote/vit_note_manager.gd`（context 供给适配，如需）；**主仓零改动**（除本卡状态流转）。越域=停下上交。
- 约束：新分支 `port/vitnote-dock-mount-1` 自 `port/vitnote-impl-1`（098cd25）切出，不动 Godot 仓 main/既有分支；零自动化测试基建如实申报，手测清单代端测（AGENTS §5 渲染面边界）。
- 验收标准：dock 默认入口下 M8 最小闭环手测走通（用户复验）+headless 自验记录+改动文件清单。
- 停止条件：dock 工作区结构与假设不符（无稳定 _ready 挂载点/context 完全无等价且降级不可行）→ 实证上交。
- 领取：2026-10-01 19:02（PC 会话③）/ Godot 仓基线分支=port/vitnote-impl-1 @ 098cd25（领取时工作树 HEAD 即该 commit，status 干净仅未跟踪运行时目录）/ 新分支名=port/vitnote-dock-mount-1 / 主仓 origin/main=245b795a
- 回执：（commit hash / 挂载+context 锚点 / headless 自验 / 手测清单）
- 验收：（裁定文件 / 验收 commit）

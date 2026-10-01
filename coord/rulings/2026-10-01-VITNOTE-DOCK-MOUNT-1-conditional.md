# Ruling：VITNOTE-DOCK-MOUNT-1 — 代码面 pass，终裁待用户默认入口 M8 手测（2026-10-01 决策会话）

- 实现：Godot 仓 port/vitnote-dock-mount-1 @ **66e59f0**（自 098cd25 单提交，2 文件 +8/-1）；主仓零改动核实（卡片流转除外）。
- 亲核项：
  1. **diff 直读**（全部 8 行）：vit_dock_root.gd——preload 常量（:26）+ `_ready` 尾部五行挂载（:178-182，`_bind_window_file_drop()` 后/`call_deferred` 前，与 legacy 先例 vit_control_v_1.0.gd:155-158 同款 new+命名+add_child）；vit_note_manager.gd——仅头注释更新（双挂载点+降级声明）。legacy 挂载零触碰。lane 组广播（vit_track_lane_2d.gd:2311 全树组查找）与快捷键（autoload InputMap）确认零改动即通——与领取前决策侧取证机制逐点吻合。
  2. **context 降级核验采信**：dock 无 LLM_Chat_Window 等价物（回执三锚点：vit_dock_root.gd:76 空值守卫/open_llm_chat_from_shell=开窗非供给/唯一实现 LLM_Chat_Controller.gd:4529 不在 dock 实例化）——走卡面授权的 manager 内建空 context 降级，`{}`→panel 省略 payload.context，7878 主管道不受阻。**降级面如实入账：便签问答暂不带工程上下文（选区/插件/history 缺位）**，IMPL-2 的 vit_note 上下文块落地时一并补。
  3. **headless 自验采信**：--import EXIT=0 零 SCRIPT ERROR+VitDockRoot 30 帧挂载检查 PASS（入树/组命中/空 context 降级三断言，临时脚本已删）——我方不独立复跑（用户编辑器占用同仓 .godot 缓存，争用风险>8 行 diff 的复验收益）；证据重量由 diff 极简+真机手测终裁承担。
- **终裁条件**：用户从默认入口（无需环境变量）Godot 拉起→启动页进含音频轨+clip 工程→IMPL-1 卡 9 步清单走通（框选→胶囊→三出口→便签结构→中文 IME→观察问答有回复→收回重开→拖动→焦点守卫；加分=双 note）。走通后本卡与 VITNOTE-IMPL-1 一并终裁（后者条件=legacy 面 9 步，本卡=dock 面 9 步——**同一份 9 步清单在 dock 默认入口走通即可同时销两张卡的条件**）。
- 用户操作注记：Godot 工作树已被执行侧切至 port/vitnote-dock-mount-1；**测试前关掉再重开 Godot 编辑器**（分支在工作树下切换过，编辑器缓存重载最稳妥），F5 走默认入口即可。

# VITNOTE-INPUT-FIX-1：便签面板鼠标链灭修复——router 顺序修正+旁路根扩展+仲裁器短路（V2 设计 §7.3 三锚点，P1）

- 池序 27（明日池头候选）；**目标仓库=D:\Godot\project\vit-daw-frontend（Godot 仓，自 port/vitnote-dock-mount-1@66e59f0 切出）**；来源=[VITNOTE_V2_INTERACTION_DESIGN.md](../../../docs/VITNOTE_V2_INTERACTION_DESIGN.md) §7（设计已验收 pass，rulings/2026-10-01-VITNOTE-INTERACT-V2-DESIGN-1-pass.md）——本卡=V2 IMPL 五卡排程的首张（输入修复独立于 D1/D2 裁定，可先行）
- 优先级 / 预估 / 依赖：P1 / 0.5 天 / V2 设计 §7 已定锚点；与 D1/D2 用户裁定无依赖
- 模型分级：L1 / GLM 或 flash 可接（锚点已函数级；Godot 输入面经验优先）
- 已核实事实（设计 §7.1-7.3，静态链在设计验收时亲核）：
  1. 症状：dock 面 N 键开窗成功但面板**含收起按钮**全部鼠标交互无效（键盘链通鼠标链灭）；用户两轮实测在案。
  2. 主嫌疑（高先验）：`timeline_track_list_drop_router.gd:_input` 左键分支**先转发 lane（:1730-1733）后查 interactive 旁路**，且 `_candidate_interactive_hit_roots`（:1990）根枚举**不含 VitNoteLayer 子树**——面板/胶囊上的 press 被 lane marquee 起手消费+`set_input_as_handled`，GUI 命中测试从未发生；同函数右键/双击分支已有 check-before-consume 先例（:1725-1729）。
  3. legacy 同根推论：router 脚本两面同挂，legacy 面板悬浮于画布上方时同链成立。
- 目标（V2 设计 §7.3 三锚点+探针，逐字执行）：
  1. **取证先行**（§7.2 探针 A/B/C+legacy 同根验证）：hover 归属探针、GUI 到达性探针、`DEBUG_TRACK_UI_INPUT_DIAG` 消费链探针——确认主嫌疑/修正方向后再动手（设计允许静态链被探针证据修正，如实申报）。
  2. **锚点 A**：router `_input` 左键分支——interactive 旁路检查移到 lane 转发**之前**（对齐右键/双击分支先例）。
  3. **锚点 B**：`_candidate_interactive_hit_roots` 增加枚举组 `vit_floating_interactive` 的根（VitNoteLayer 及未来悬浮层 `_ready` 入组）；`_control_tree_has_interactive_hit` 随之命中面板内 BaseButton/LineEdit。
  4. **锚点 C**：TimelineInputArbitrator `_should_accept_mouse_at_global`（:211）加同组短路（点落 `vit_floating_interactive` 控件矩形内即 IGNORE——补非按钮控件体盲区）。
  5. **回归**：V2 设计 §10.2 第 8 步三态全操作+时间线画布正常手势零改变（修复不得改变画布内正常 marquee/选中/拖拽）；headless `--import` 零 SCRIPT ERROR。
  6. 用户复验：dock 默认入口面板可交互（收起/重开/拖动/输入/发送）。
- 文件域：Godot 仓 `app/timeline/timeline_track_list_drop_router.gd`+`app/tracks/vitnote/vit_note_manager.gd`（入组）+仲裁器文件（实锚后申报）——预计 ≤3 文件。
- 约束：新分支 `port/vitnote-input-fix-1` 自 dock-mount 分支切出；不动 Godot 仓 main；运行时目录勿动；无自动化测试基建如实申报（探针+手测代端测，AGENTS §5 渲染面边界声明）。
- 验收标准：三锚点落地+探针前后对比留档+画布手势零回退（手测目检）+用户面板复验通过。
- 停止条件：探针证伪主嫌疑（如 hover 归属报其他 STOP 控件=候选 A 成立）→ 按设计 §7.2 分支处理并如实申报，不硬套锚点。
- 领取：（时间 / Godot 仓基线 hash / 分支名）
- 回执：（commit hash / 探针对比 / 三锚点 diff / 手测清单）
- 验收：（裁定文件 / 验收 commit）

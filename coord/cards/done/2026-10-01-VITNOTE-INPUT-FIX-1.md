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
- 领取：2026-10-02 10:08 / Godot 仓基线 66e59f0cb102e6ddd9cc75b6faf965799d12c551（=port/vitnote-dock-mount-1 HEAD，仓原停此） / 分支 port/vitnote-input-fix-1（自基线切出）；Vit_DAW 仓 HEAD=ccdc0a7a，领取时工作树仅 VitApp/Workspace/default_project.xml（运行时状态，不属本卡不混入）
- 回执：**commit=d97293c**（Godot 仓 port/vitnote-input-fix-1，已推 origin；3 产品文件 +89/-1 + 探针 tools/probe_vitnote_input.gd +360 新建）
  - **探针前后对比**（tools/probe_vitnote_input.gd，渲染窗口模式 legacy 面完整判据）：
    - 基线（候选 B 实锤，五条证据）：①面板按钮处 `gui_get_hovered_control()=<null>`（motion 被 lane 转发消费，GUI hover 被吞）②`collapse_pressed=false`（press 未达 GUI）③press 后 `lane_marquee_press=true`（**press 穿透面板直接起 lane marquee**——§7.2 行为签名命中）④`bypass_interactive_hit=true` 而 press 仍被消费（旁路判定为真但位于 lane 转发之后，顺序 bug 直接实证）⑤`candidate_roots` 缺 VitNoteLayer（根枚举缺口实证）；对照组（同点面板隐藏）lane marquee 正常起手=时间线链路本身正常。对照实验（对齐 empty zone 无 lane 处）hover=面板按钮路径=**候选 A 出局**。
    - 修复后：`collapse_pressed=true`（收起按钮触发、面板收起）+ hover=面板按钮路径（恢复）+ 面板在场 `lane_marquee_press=false`（不再穿透）+ 隐藏对照 `=true`（**画布手势零回退**）+ `candidate_roots=[Track_Master, VitNoteLayer]`（根枚举补全）。
    - dock 面 headless 探针：EXIT=0 零 SCRIPT ERROR，证组注册生效（group=[VitDockRoot/VitNoteLayer]）+ manager 全幅修复；链路/GUI 判据在 headless 不可判（Godot 4.6 固定 64x64 窗口致 canvas 出窗 + 无内核时 dock 时间线视图周期重建——环境限制如实申报），dock 终验归用户手测。
    - 附加发现并修复：manager `set_anchors_preset` 不重设 offsets 致 rect 残留 (0,0,0,0)（面板靠绝对定位显示故从未暴露；锚点 B 根枚举按根 rect 过滤会漏层）——已改 `set_anchors_and_offsets_preset`。
  - **三锚点 diff**（d97293c）：A=router `_input` 左键分支新增 `_global_point_targets_floating_interactive_control` 组旁路前置（:1734 转发前）+motion 分支同款；**设计偏离申报**：未整段前移原 interactive 旁路——取证证实 smart 模式下 lane 自身判交互（`bypass=true` 实测），整段前移会灭画布 marquee，故只前置组旁路（新函数只扫组根子树）、原旁路保持后置语义不变（满足"画布零回退"回归约束，属设计 §7.3 锚点 A 的保守落地）。B=`_candidate_interactive_hit_roots` 组根枚举（兜底前）+manager `_ready` 入组+anchors 修复。C=仲裁器 `_should_accept_mouse_at_global` 组短路（组根子树可见非 IGNORE 控件 rect 含点即 IGNORE，补非按钮体盲区）。
  - **验收证据**：headless `--import` 修复前后均 EXIT=0 零 SCRIPT ERROR；探针 legacy 修复后全绿（上表）；无自动化测试基建如实申报（探针+手测代端测，AGENTS §5 渲染面边界声明：本卡探针覆盖输入链逻辑层，渲染/IME/真实窗口几何归手测）。
  - **手测清单（交用户，AGENTS §5 途径：用户从 Godot 拉起前端→dock 工作区进工程）**：①N 键/胶囊开便签→面板含收起按钮全部鼠标交互有效（收起/锚点重开/拖把手移动/输入框聚焦/发送）②面板悬浮于时间线轨道区上方时重复①（bug 原场景）③画布手势零回退：lane 上左拖 marquee 框选/clip 点选/clip 拖动/边缘裁剪/空区左拖与修复前一致④legacy 面板拖到画布上方重复①（同根验证）⑤面板底板空白处点击不应触发时间线操作（锚点 C 判据）。
  - **运行栈声明：本卡全程未启动真实三件套（VitApp 内核/Godot 前端正式栈/Go agent）；探针自起自退的 Godot 进程已全部退出（tasklist 核对零残留），无栈移交事项。**
- 验收：**pass（2026-10-02 决策会话；面板终验挂用户手测）**——[rulings/2026-10-02-VITNOTE-INPUT-FIX-1-pass.md](../../rulings/2026-10-02-VITNOTE-INPUT-FIX-1-pass.md)；三锚点 diff 亲核+偏离申报采信（组旁路前置保守落地，画布零回退优先）+我方复跑（headless import EXIT=0+窗口模式 legacy 探针全绿复证：collapse 触发/hover 恢复/面板在场零穿透/隐藏对照 marquee 正常/根枚举补全）+探针原始输出未落盘注记（我方重跑补正）；Godot d97293c 为分支链 HEAD，V2-IMPL-A 依赖解锁

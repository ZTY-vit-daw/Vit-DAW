# VITNOTE-CONTAINER-2：note 容器升级 A+B——全标题栏拖动+锚点移动/删除+缩放把手+自适应布局（用户裁定 2026-10-02，P2）

- 池序 34；**目标仓库=D:\Godot\project\vit-daw-frontend（Godot 仓，自 port/vitnote-v2-impl-d@fd7e6af 切出——已验收 HEAD）**；来源=[decisions/2026-10-02-user-manual-test-feedback.md](../../decisions/2026-10-02-user-manual-test-feedback.md) §1+追记裁定 1（用户：A+B 全要）
- 优先级 / 预估 / 依赖：P2 / 0.5-1 天 / 无（V2 五卡基线在位；与 NOTESTREAM-2 面板历史 UI 有衔接——本卡先做容器骨架，NOTESTREAM 后接历史区）
- 模型分级：L1 / GLM 或 flash 可接（纯 Godot 面板域，先例充分）
- 已核实事实（本轮决策侧亲核）：
  1. 拖动现仅 ⠿ Button（vit_note_panel.gd:73-79 button_down/up+`_process` 轮询实现健壮）——**面板体/标题栏非按钮区域按下仍被 router→lane 消费**（INPUT-FIX 锚点 B 清单只认 BaseButton/LineEdit 等交互类型），体感=不可移动。
  2. 缩放从未实现（V1 三态=收起/重开/拖动）；锚点=Button 仅点击重开（:163-170），无移动/删除语义。
  3. 圈选层/胶囊/双出口/多面摘要均在本分支链可用（用户实测好评）。
- 目标（A+B 全量）：
  1. **整标题栏拖动区**：标题栏（note_id+摘要行区域）整体可拖（复用 _drag_active 轮询机制；标题栏内按钮保持各自交互）；⠿ 保留作视觉提示或移除（实锚定）。
  2. **非按钮区域防抢**：面板体/标题栏空白处按下不得穿透到时间线——方案=面板根 PanelContainer 与标题栏加入 `vit_floating_interactive` 组命中判定（或扩展 router 旁路清单认 PanelContainer 体；**实锚后选最小侵入方案并申报**——注意锚点 C 仲裁器侧已按 mouse_filter!=IGNORE 短路，主要补 router 侧清单）。
  3. **锚点升级**：拖动=移动锚点位置；右键=删除（收起态便签从锚点永久移除——note 数据随之丢弃，与 IMPL-6 便签池解耦，v2 即删无回收站，卡面确认此语义）；保留点击=重开。
  4. **缩放**：右下角缩放把手（最小/最大尺寸钳制，min≈现尺寸、max=viewport 内自由）；输出区改 ScrollContainer。
  5. **布局重排**：输入框常驻底部；输出区自适应高度（内容少收缩/多滚动）；发送按钮与收起按钮位置按现布局习惯微调（回执附前后截图对比）。
  6. **回归**：三态（收起/重开/拖动）零回退+IME 输入零回退+多 note 并存零回退+圈选/胶囊链零回退（INPUT-FIX/V2 判据复用）；headless `--import` 零 SCRIPT ERROR+既有三探针回归零破坏；探针可断言面（拖动闩/缩放钳制数值）新增 headless 断言，**原始输出落 coord/runs/VITNOTE-CONTAINER-2/ 留档**。
- 文件域：`app/tracks/vitnote/vit_note_panel.gd`（主）+`vit_note_manager.gd`（锚点挂载协作如需）+router/仲裁器**仅在方案 2 需要时**最小侵入（实锚申报）——预计 ≤3 文件。
- 约束：新分支 `port/vitnote-container-2` 自 port/vitnote-v2-impl-d@fd7e6af 切出；不动 main；运行时目录勿动；渲染面归用户手测复验（回执附自验清单）。
- 验收标准：目标 1-5 落地+探针/回归全绿+用户手测复验（拖动/缩放/锚点移动删除/布局/三态零回退）。
- 停止条件：router 清单扩展影响画布正常手势（回归发现）→ 退回仅组内方案并如实申报取舍；锚点删除语义与持久化耦合超预期 → 上交裁定。
- 领取：2026-10-02 PC 执行侧（ZCode GLM-5.3）/ fd7e6af0b7cff88afea29a4a6882fa81d7f5244b / port/vitnote-container-2
- 回执（2026-10-02 PC 执行侧）：
  - **commit**：Godot 仓 `port/vitnote-container-2` @ `42b1856`（基线 fd7e6af+1；5 文件：3 产品 + 2 工具）
  - **方案 2 实锚申报（最小侵入）**：router 仅改 `_global_point_targets_floating_interactive_control` 一处——组成员自身 `mouse_filter!=IGNORE`+矩形命中即旁路（体部判定，语义对齐仲裁器锚点 C）；`_is_interactive_timeline_control` 清单与其余两调用点（画布侧 hover 链/非 lane 判定）不动 → 画布手势零影响；面板根 `_ready` 入组 `vit_floating_interactive`（先例=manager 锚点 B 入组）。
  - **目标落地**：①整标题栏拖动=Header STOP+gui_input 起闩（⠿ 保留视觉提示，按钮子先于父命中不受扰）②体部防抢如上 ③锚点三分=原位 down→up 重开（位移<4px 判定替代 pressed 直连）/拖动移动（_process 轮询+钳制）/右键即删（`manager.remove_note`，note 数据丢弃，无回收站）；collapse 清闩防悬挂 ④缩放=右下 16px 把手（FDIAG 光标+三斜线），min=PANEL_MIN_SIZE/max=viewport 内自由；基类 PanelContainer→Panel（把手驻角前提，vbox FULL_RECT 自管布局）⑤布局重排=标题栏/输出滚动区（中）/输入行常驻底部（LineEdit+发送同行）；钳制抽静态纯函数（探针可断言面）。
  - **探针输出留档**：`coord/runs/VITNOTE-CONTAINER-2/` 10 份——新探针 probe_container_run1（43/43 EXIT=0：组注册/结构不变量/拖动闩/缩放闩/两钳制数值表/锚点三语义/删除链）；回归 face_resolve 127/127+circle_state 52/52+panel_summary 16/16+input dock EXIT=0；`--import` 零 SCRIPT ERROR；input legacy EXIT=2 经基线 fd7e6af 临时 worktree 对照判定=既有 headless 64x64 几何限制（两版输出逐字相同，probe_input_legacy_baseline_check.txt 归因留档；IMPL-D 当时留档亦仅 dock 面）。
  - **布局前后截图**：layout_before/after_{open,collapsed}.png 四张（窗口模式渲染截图，工具 tools/capture_vitnote_panel_layout.gd 两版通用）。
  - **自验清单（用户手测复验项）**：1) 打开态：标题栏空白处长按拖动整面板（含摘要文本区），⠿/收起/输入/发送各自交互不受扰；2) 右下角把手缩放（光标变斜箭头；不小于 ~340×300、不出屏）；3) 输入框常驻底部、输出区内容多时滚动；4) 收起→锚点：单击重开、按住拖动只挪位置、右键即删（无确认无回收站——删除不可恢复请用测试 note）；5) 多 note 并存互不干扰；6) Q 圈选→胶囊→开便签链路照常；7) 面板压在时间线上方时画布 marquee/右键菜单照常（防抢不扩大到面板外）；8) IME 中文输入照常。
  - **运行栈状态声明**：本卡全程未启动真实运行栈（VitApp 内核/Godot 完整前端应用/Go agent 三件套均未运行）；仅 Godot `--headless -s` 探针与窗口模式截图脚本（Script 模式加载 dock 场景取证，先例=probe_vitnote_input）；无残留进程、无端口占用；两仓工作树除本卡 diff 外无其他改动（Godot 仓未跟踪运行时目录未动）。
  - **端测覆盖边界（AGENTS.md §5）**：渲染面/交互手感/IME 实输在 headless 不可判（探针 NOTE 自声明），按卡面约定归用户手测复验；闩位/钳制数值/结构不变量已由 43 项探针锁定。
- 验收：（裁定文件 / 验收 commit）

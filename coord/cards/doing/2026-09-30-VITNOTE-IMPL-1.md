# VITNOTE-IMPL-1：vit note v1 编排面最小闭环——框选形变提示+便签容器三态+会话接线（M8 前置）

- 池序 7（vit note v1 IMPL 第二张，IMPL-4 已闭合）；来源=[docs/VITNOTE_V1_DESIGN.md](../../../docs/VITNOTE_V1_DESIGN.md) §4/§10。**目标仓库=D:\Godot\project\vit-daw-frontend（仓库外 Godot 4 工程）——本卡登记在主仓 coord，实现工作全部在 Godot 仓**
- 优先级 / 预估 / 依赖：P1 / 1–1.5 天 / 设计文档 §4 交互规格（P2 快捷键默认 N、P3 提示超时 8s 为已定默认值可调）；**无 P1 便签池拍板依赖**（便签池属 IMPL-6）；vit_note 上下文块属 IMPL-2（本卡复用既有选区组包）
- 模型分级：L1 / GLM 首选（新交互形态首例、Godot 侧环境坑未知）；flash 可接（规格已定，坑内求助）
- 已核实事实（VITNOTE-RECON-1 §1/§2/§3 采信，基线 Godot HEAD=5280822，行号可能漂移以符号为准）：
  - marquee 手势状态机+全局 rect 广播+跨轨命中字典在位（vit_track_lane_2d.gd：_begin_marquee_press/_finish_marquee_gesture/set_global_marquee_preview/_clip_ids_intersecting_global_marquee）
  - 选区状态=TimelineEditStore（set_selected_clip_ids/set_time_selection）；_build_agent_context 组包链现行存活（轨/clip/时间窗三要素已入 request_context）
  - LineEdit 中文 IME 生产实证；_has_text_input_focus 焦点守卫与 _set_send_enabled 单会话串行先例在
  - chat 入口=http://127.0.0.1:7878/agent/chat；payload={conversation_id, message, context}；空 conversation_id→服务端新建
  - Control 级浮层先例：vit_tooltip_trigger.gd（可挂任意 Control）/PianoRollClipOverviewHUD（Control 内绘）
- 目标（M8 最小可演示闭环）：
  1. **形变提示**：框选完成后框右侧弹出可点击胶囊（spring 缩放+透明度渐入，简单 tween）；三出口=点击胶囊/快捷键 N（经 VitShortcutManager 注册）/8s 无交互淡出（选区保留）；文本输入焦点态不触发快捷键（复用 _has_text_input_focus 模式）；空区域框选同样出提示（选中集为空是合法态）。
  2. **便签容器**：Control 级浮层，出现于框选 rect 右上角外侧偏移（不遮辖区）；结构=头部（辖区摘要一行=轨数/clip 数+收回按钮+拖拽把手）+输入区（LineEdit）+输出区（滚动消息流）；三态：打开/收回（v1 最简锚点视觉=rect 左上角小点位标记，正式锚点小标签属 IMPL-6）/自由移动（拖把手重定位，松手即停无吸附）。
  3. **会话接线**：每 note 独立 conversation_id（初始空，首答服务端赋值后持有）；输入走 7878 chat 主管道（**禁止旁路触发**，概念裁定 4）；单 note 一轮未完禁发（复用 _set_send_enabled 串行先例）；context 复用 _build_agent_context 既有组包（vit_note 块与 rect 派生时间窗属 IMPL-2）。
  4. **多 note 并存**：≥2 note 各持独立会话互不干扰（M8 加分项基础；v1 不设数量硬限）。
- 文件域：Godot 仓编排面交互（vit_track_lane_2d.gd 及交互节点组挂点+新增便签容器/形变提示脚本+VitShortcutManager 注册项）；主仓零改动。**越域=停下上交**（如需 agent 侧新端点/字段配合）。
- 验收标准：Godot 仓既有测试/冒测路径零回归（仓内无自动化测试基建时如实申报，以手测清单代）；**手测路径=M8 最小闭环**（框选→形变提示→开便签→中文输入→观察问答→收回→重开；加分=双 note 并存）——按 AGENTS §5 手测入口=用户从 Godot 拉起前端进入工程；决策侧冒测脚本如覆盖编排面则扩展场景（实锚后申报）。回执附：领取时 Godot 仓 git status（RECON 记录该仓有未跟踪运行时目录，领取态如实记录）、改动文件清单、手测步骤清单（供用户复验）。
- 停止条件：①编排面 marquee/选区链路与 RECON §1 事实不符（前端已漂移）→ 停下实证上交；②需要 agent 侧配合 → 上交不越域；③IME/焦点路由在多 note 场景发现 RECON §2 未预见的问题 → 记录证据上交。
- 风险点：Godot 仓与主仓分支纪律不同——在 Godot 仓开 port/vitnote-impl-1 分支，**不直接动该仓 main/既有 port 分支**；仓内未跟踪运行时目录勿动勿提交。
- 领取：2026-09-30 20:18 / Godot 仓 HEAD=5280822（=RECON 基线，当时 checkout 在 codex/auto-mix-session-entry，新分支从 5280822 切出） / port/vitnote-impl-1（Godot 仓）。领取时 Godot 仓 status：仅未跟踪运行时目录（.vit_agent/ .vit_derived/ 7.31A5.vit_project/ 7.31test.vit_project/ 8.1test/ 8.1test1/ 及若干中文命名工程目录），无已跟踪文件改动；主仓另有 M VitApp/Workspace/default_project.xml（运行时状态，非本卡，不动不提交）
- 回执：（Godot 仓 commit hash / 改动清单 / 手测步骤清单 / 端测边界声明）
- 验收：（裁定文件 / 验收记录）

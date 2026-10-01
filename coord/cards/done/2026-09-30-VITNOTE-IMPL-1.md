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
- 回执：**执行完成 2026-09-30 21:00，[等待真栈手测验收]**
  - **Godot 仓 commit**：`port/vitnote-impl-1` @ `098cd25`（9 文件，+633/−5；从 5280822 切出，未动 main/既有 port 分支，未跟踪运行时目录未动未提交）
  - **改动清单**：
    - 新增 `app/tracks/vitnote/vit_note_manager.gd`（+uid）：全幅 Control 管理层（组 `vit_note_manager`）；hint 生命周期（一次一个，重复框选重建）；`open_note` 多 note 并存（不设数量硬限）；`vit_note_open` 快捷键消费（`_has_text_input_focus` 焦点守卫复用先例：LineEdit/TextEdit/RichTextLabel 聚焦不触发）；context 供给=祖先链找 `LLM_Chat_Window` 后 call `_build_agent_context`（复用既有组包，含 project history/选区/插件上下文）
    - 新增 `app/tracks/vitnote/vit_note_hint.gd`（+uid）：形变提示胶囊；spring 生长（TRANS_BACK 过冲）+渐入；三出口=点击/快捷键/8s Timer 淡出（选区保留）；文案经 `VitShortcutManager.get_shortcut_text` 动态获取（未手写 [N]）；`present_at` 在 scale=1 语义下定位+position/scale 同步补间（修复缩放污染位置的动画 bug）
    - 新增 `app/tracks/vitnote/vit_note_panel.gd`（+uid）：便签容器三态——打开（头部辖区摘要"note_N · X 轨 · Y clip"/空辖区 + ⠿ 拖把手 + 收起 + LineEdit + 发送 + 滚动消息流）/收回（折叠为辖区 rect 左上角外侧锚点小蓝点，点击重开，会话与消息流保留）/自由移动（拖把手跟手、松手即停无吸附）；会话=每 note 独立 conversation_id 初始空、首答从 `reply.conversation_id` 赋值持有；发送走 7878 `/agent/chat` 主管道（payload={conversation_id,message,context}，`_post_json` 模式复用先例）；一轮未完禁发（发送中 editable+disabled，复用 `_set_send_enabled` 先例）；错误 fail 显示在消息流（连接失败带 error 详情）
    - 改 `app/tracks/track_scene/vit_track_lane_2d.gd`：`_finish_marquee_gesture` 真实拖框分支（`_marquee_drag_active`）调用 `_notify_vit_note_hint`（经组反向查找 manager，has_method 防御，manager 缺位零影响；单击清选区不出提示；空区域框选同样通知）；提取 `_collect_marquee_hits_global`（跨轨命中字典含 track_id/clip_id/track_y/start_seconds）供选区写入与辖区快照共用——行为等价重构，选区写入与 marquee 广播零改变
    - 改 `app/kernel/autoloads/vit_shortcut_manager.gd`：`ACTION_VITNOTE_OPEN="vit_note_open"` 默认 KEY_N（可配置）+ LIST 前缀分支
    - 改 `app/legacy/vit_control_v_1.0.gd`：`_ready` 动态挂载 `VitNoteLayer` 到主控台根（先例=GlobalTimeAxisController 同款 add_child）
  - **主仓零改动**（除本卡状态流转）
  - **自验记录**（命令+退出码/结果）：
    1. `godot --headless --import`（4.6.1）：零 SCRIPT ERROR（过程中抓到并修复 2 处编译错：BoxContainer 对齐枚举实名 `AlignmentMode.ALIGNMENT_END`）
    2. 主控台场景 headless 运行 300 帧：VitNoteLayer 入树；Node not found 错误 4 个与基线 worktree（5280822）完全一致——预存 Browser 节点错误，非本卡引入（对比后基线 worktree 已清理）
    3. vitnote 逻辑冒烟（临时 SceneTree 脚本，跑完已删）：**23/23 PASS**——manager 挂载/action 注册+绑 N/焦点守卫/hint 创建与辖区快照（clip=3、track 去重=2）/空区域合法态/便签创建 note_1+位置/note_id/会话初始空/收回-锚点-重开/多 note 并存独立编号独立会话
  - **端测边界声明（AGENTS §5）**：本卡全部改动在 Godot 交互/渲染面；Godot 仓无自动化测试基建，E2E 浏览器级断言不适用（属 webui 面）。未自动化覆盖：7878 实发请求、IME 真实组字、tween 视觉效果、快捷键真实按键路径、headless 64×64 视口下精确位置（断言退化为 clamp 语义）。以上交下方手测清单覆盖。
  - **已知边界（如实申报，非阻塞）**：①工程切换时已开 note 不自动回收（v1 简化，IMPL-6 便签池线再议）；②8s 淡出后"hover 选区重唤出"未做（卡面三出口未含，再次框选即可重唤）；③收回锚点为一次性定位（轨道滚动/缩放后不跟随辖区——正式锚点属 IMPL-6）；④vit_note 上下文块与 rect 派生时间窗未随消息发送（属 IMPL-2，本卡 context=既有组包）
  - **手测步骤清单（M8 判据，按 AGENTS §5 手测入口：用户从 Godot 拉起 DAW 前端→启动页进入含音频轨+clip 的工程）**：
    1. 编排面空白处左键拖框松开：框内 clip 照旧选中（行为不变），框右侧出现"[N] 打开便签"胶囊（生长+渐入动画）
    2. 三出口：a) 点胶囊开便签；b) 再框选后直接按 N 开便签；c) 再框选后静置 8s 胶囊淡出且选区保留
    3. 便签结构：头部"note_N · X 轨 · Y clip"（空区域框选="空辖区"）+⠿ 把手+"收起"；下方输入框+发送+消息流
    4. 中文 IME：点击便签输入框，中文组字→回车/发送（组字期回车不上屏不误发）
    5. 观察问答：发"这一段听起来怎么样？"→思考中→回复显示；同 note 再追问验证会话延续；发送期输入禁用
    6. 收回→点辖区左上角小蓝点→重开（消息流保留）
    7. 拖 ⠿ 移动便签：跟手、松手即停
    8. 焦点守卫：便签输入框聚焦时按 N 不触发（N 进输入框）；无文本焦点时 N 触发
    9. 加分（M8 判据）：两块辖区各开一 note 各自问答，会话互不干扰
- 验收：**代码面 pass、终裁待 M8 手测（2026-09-30 决策会话）**——裁定=[rulings/2026-09-30-VITNOTE-IMPL-1-conditional.md](../../rulings/2026-09-30-VITNOTE-IMPL-1-conditional.md)；Godot@098cd25 diff/结构亲核（挂点唯一出口+选区零改变+全要件在位）+自验声明采信；终裁条件=用户按卡面 9 步手测走通 M8 最小闭环（Godot 仓 checkout port/vitnote-impl-1；agent 二进制已重建含 FS-SETTLE 修复）。
- 终裁改挂（2026-10-01 用户裁定）：本卡条件与 V2 验收同场销项（decisions/2026-10-01-vitnote-v2-interaction-ruling.md 第 8 条）——V2 手测一场销三卡；现行形态不再单独要求走 9 步。

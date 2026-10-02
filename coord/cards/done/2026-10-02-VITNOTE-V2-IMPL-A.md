# VITNOTE-V2-IMPL-A：全局圈选层落地——Q+左拖手势状态机+截图式视觉+挂载契约+lane 通知退役（V2 五卡第二张，P2）

- 池序 28；**目标仓库=D:\Godot\project\vit-daw-frontend（Godot 仓，自 port/vitnote-input-fix-1 切出——该分支含锚点 B 的 manager 入组基线）**；来源=[VITNOTE_V2_INTERACTION_DESIGN.md](../../../docs/VITNOTE_V2_INTERACTION_DESIGN.md) §4/§8/§10.1/§10.2（**已含 2026-10-02 D1/D2 用户裁定修订**：圈选=Q+左拖、plain marquee 不出胶囊，[decisions/2026-10-02-vitnote-v2-d1d2-ruling.md](../../decisions/2026-10-02-vitnote-v2-d1d2-ruling.md)）
- 优先级 / 预估 / 依赖：P2 / 1 天 / **VITNOTE-INPUT-FIX-1 完成后串行**（设计 §11 原标"可并行"不成立：两卡文件域在 vit_note_manager.gd 重叠——本卡将其升格圈选协调者，INPUT-FIX-1 在其上入组；串行避免同文件冲突）
- 模型分级：L1 / GLM 首选（Godot 输入传播序+状态机；与 INPUT-FIX-1 同会话续卡最佳——上下文连续）
- 已核实事实（设计 §2 事实基线 F5/F7/F8/F16 + §4 规格，设计验收时亲核，勿重勘）：
  1. 触发键=**Q 持续按住**（`Input.is_key_pressed(KEY_Q)`，F1 克隆同款 API 换键；Q 非修饰键不走 event.alt_pressed）；Q 前端仓零占用（F16）。
  2. manager 挂载两面主场景根末位（F8），`_input` 逆树序先收——press 先于 router/lane 到达，圈选消费后 lane marquee 收不到（§4.3 无双重框）。
  3. 手势状态机规格=§4.1 状态/转移表（IDLE/PRESS_PENDING/DRAGGING/RESOLVING+取消路径），阈值 4.0px 复用 lane 先例（F5）；焦点文本区让位=§4.1 条件②；**无 clip 体让位分支**（D1 迁键后全域可发起）。
  4. Q 为字符键边界（§4.1 声明）：焦点文本控件外的 Q+拖期间 Q 字符透传进焦点控件——不吞字符，如实申报不改语义。
- 目标：
  1. 新建 `app/tracks/vitnote/vit_circle_select_layer.gd`（Control）：§4.1 状态机**实现为无场景依赖纯类**（供 headless 探针）+§4.3 视觉（暗幕镂空+亮框+0.12s 淡出）+§4.4 捕获机制（`_input`+活跃期 set_input_as_handled+`_process` 兜底）。
  2. `vit_note_manager.gd` 升格圈选协调者：持有圈选层、手势→胶囊入口（§4.5 双出口沿 V1 hint）。
  3. **lane 通知退役**（§8，D2 已裁定）：`_notify_vit_note_hint`（vit_track_lane_2d.gd:2311）+manager `present_marquee_hint` 同卡原子删除（调用点与入口一起）；采集函数保留不动（IMPL-B 转供）。
  4. 挂载契约固化：两面挂载点（vit_control_v_1.0.gd:156 / vit_dock_root.gd:182）加注释——VitNoteLayer 须保持主场景根末位（§4.4 顺序不变量）。
  5. headless 探针（§10.1）：tools/ 下状态机探针脚本（逐条注入 InputEvent 断言转移/消费），`godot --headless -s` 跑通；手测 1/2/6/7 步由用户执行（本卡回执附自验清单与结果）。
- 文件域：`app/tracks/vitnote/vit_circle_select_layer.gd`（新建）+`vit_note_manager.gd`+`vit_track_lane_2d.gd`（仅退役位）+两面挂载点注释+tools/ 探针——预计 ≤5 文件。
- 约束：新分支 `port/vitnote-v2-impl-a` 自 port/vitnote-input-fix-1 切出；不动 Godot 仓 main；运行时目录勿动；无自动化测试基建如实申报（headless 探针+手测代端测，AGENTS §5 渲染面边界声明）。
- 验收标准：状态机探针全绿+四类取消路径零残留+lane 通知退役后普通框选仍正常（纯选区、零胶囊）+Alt 克隆零改变+Q 圈选全域可发起+用户手测 §10.2 第 1/2/6/7 步复验通过；**探针原始输出落 coord/runs/VITNOTE-V2-IMPL-A/ 留档**（INPUT-FIX-1 ruling 注记要求：结论须可回指原始工件）。
- 停止条件：状态机探针发现设计 §4.1 转移表自相矛盾 → 停止上交（设计缺陷转决策侧）；挂载顺序不变量在真实两面验证失败（press 先被 router 收走）→ 保留探针证据上交。
- 领取：2026-10-02 10:54 / Godot 仓基线 d97293c（port/vitnote-input-fix-1，已验收 HEAD）/ 分支 port/vitnote-v2-impl-a
- 回执（done，2026-10-02 12:10）：
  - **commit**：Godot 仓 `port/vitnote-v2-impl-a@091cf80`（已推 origin；自 d97293c 切出，main 未动，运行时目录未动）。7 文件 +730/-45：vit_circle_select_layer.gd 新建（含纯类 GestureMachine）+.uid、vit_note_manager.gd（升格协调者）、vit_track_lane_2d.gd（仅退役位）、两面挂载点注释、tools/probe_vitnote_circle_state.gd 新建。
  - **状态机探针**：`"D:\Godot\Godot_v4.6.1-stable_win64_console.exe" --headless --path . -s tools/probe_vitnote_circle_state.gd` 两轮均 **EXIT=0，52/52 绿**（套件 41：S1 IDLE 透传/S2 发起闸门含焦点让位/S4-S5 阈值守候与 Q+单击空操作/S6-S8 拖拽完成出口+RESOLVING 瞬态+矩形规范化与 viewport 钳制/S9-S12 四类取消路径零残留（ESC/松 Q/RMB/pending 态三式+Q+单击）/S13-S15 滚轮与 Q 字符透传+重复 press 吞/S16-S17 宿主 cancel 与 observe_position 兜底/S18 可重入；lane 静态退役 1；两面挂载烟测各 5：manager 在位+圈选层子节点+present_marquee_hint 退役+_on_circle_select_finished 在位+脚本绑定）。`--import` EXIT=0 零 SCRIPT ERROR。**原始输出落 [coord/runs/VITNOTE-V2-IMPL-A/](../../runs/VITNOTE-V2-IMPL-A/README.md)**（两轮探针+import 零错误+首轮含修复前 parse error 的过程证据）。
  - **退役 diff**（§8 原子）：lane `- _notify_vit_note_hint()` 调用点+函数体（vit_track_lane_2d.gd，-16 行）；manager `present_marquee_hint`+`_snapshot_from_hits`（-33 行）。采集函数 `_collect_marquee_hits_global`/`get_clip_marquee_hits_global`/`_marquee_rect_global`/跨 lane 广播**保留零改动**（IMPL-B 转供）。全仓 grep 证退役目标引用封闭。
  - **自验清单（headless 已证）**：状态机转移/消费全表、四类取消零残留、透传边界（滚轮/Q 字符/其余按钮）、lane 静态退役、两面挂载接线、挂载顺序不变量现状合规（legacy 树转储：VitNoteLayer 后仅频轴等无输入回调节点）。
  - **手测待用户执行（§10.2 第 1/2/6/7 步，渲染面归手测）**：①时间线空区 Q+左拖→拖拽中暗幕镂空+亮框+四角标记，松开 0.12s 淡出+胶囊在框右侧（文案「[N] 打开便签 · 圈选」，**无面数——faces 解析归 IMPL-B，本阶段 note 摘要显示「空辖区」为预期非缺陷**）；②[N] 与点胶囊双出口均开 note，文本焦点态 [N] 不触发；⑥拖拽中 ESC/拖拽中松 Q/Q+单击不过阈值→皆无胶囊无 note 无残留遮罩；⑦普通左拖空 lane=纯选区零胶囊（退役后行为）、Alt+左拖 clip 体仍克隆、Q+左拖 clip 体发起圈选、时间线编辑零回归。
  - **边界如实申报**：a) 渲染视觉/淡出动画/真实输入链交互 headless 不可断言，归手测；b) Q 字符透传边界（§4.1 声明）：焦点在文本控件且 press 点在其矩形外圈选时，按住的 Q 会作为重复字符透传进该控件（不吞，主防线=焦点文本区让位）；c) 分离窗口圈选不达（§4.4 声明边界）；d) 探针 headless 环境 ObjectDB leaks 警告为 teardown 产物，退出码 0 为门槛；e) 设计 §8 表第 4 行「V1 文档加修订标注」不在本卡目标/文件域，未动——留决策侧裁处。
  - **运行栈声明：本卡全程零运行栈**（仅 headless import 与 `-s` 探针，进程均已退出；tasklist 核查无 godot/VitAgent/VitApp 残留）——无栈可拆/移交。
- 验收：**pass（2026-10-02 决策会话；渲染面手测挂 V2 手测场）**——[rulings/2026-10-02-VITNOTE-V2-IMPL-A-pass.md](../../rulings/2026-10-02-VITNOTE-V2-IMPL-A-pass.md)；状态机纯类与 §4.1 转移表逐条亲核+manager 升格/退役/挂载注释 diff 亲核（hint 文案预置假设核实成立）+我方复跑（headless 探针 52/52 EXIT=0+import 零错误自跑）+工件四件可回指（过程诚实申报优质）；V1 文档标注随裁定更新（Alt→Q 追记）；V2-IMPL-B 依赖解锁

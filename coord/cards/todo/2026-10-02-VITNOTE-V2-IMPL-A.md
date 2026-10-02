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
- 领取：（时间 / Godot 仓基线 hash / 分支名）
- 回执：（commit hash / 状态机探针输出 / 退役 diff / 自验清单）
- 验收：（裁定文件 / 验收 commit）

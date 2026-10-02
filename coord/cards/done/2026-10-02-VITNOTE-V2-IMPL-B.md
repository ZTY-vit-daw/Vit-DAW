# VITNOTE-V2-IMPL-B：注册处+解析管线+timeline/control 供给器——faces[] 命中解析落地（V2 五卡第三张，P2）

- 池序 29；**目标仓库=D:\Godot\project\vit-daw-frontend（Godot 仓，自 port/vitnote-v2-impl-a@091cf80 切出——已验收 HEAD）**；来源=[VITNOTE_V2_INTERACTION_DESIGN.md](../../../docs/VITNOTE_V2_INTERACTION_DESIGN.md) §5（解析管线）/§6.0（供给器契约）/§6.1（timeline）/§6.4（control 快照）/§9（payload v2）
- 优先级 / 预估 / 依赖：P2 / 1 天 / V2-IMPL-A 已验收（圈选出口在位）；与 IMPL-C（机架/资料库供给器）文件域不同、依赖本卡注册处——串行
- 模型分级：L1 / GLM 首选（契约设计+Godot 结构遍历；与 IMPL-A 同会话续卡上下文连续）
- 已核实事实（设计 §2 F6/F10 + IMPL-A 落地基线，勿重勘）：
  1. IMPL-A 已交付：圈选完成信号 `circle_select_finished(rect_global)`→manager `_on_circle_select_finished`（现 note 载荷 faces=[] 空辖区占位）；`_hint_text` 预置机制在位。
  2. 采集函数保留在 lane：`_collect_marquee_hits_global`/`get_clip_marquee_hits_global`/`_marquee_rect_global`（F6，IMPL-A 退役时零改动保留转供本卡）。
  3. 控制面状态源（F10）：BPM=`main_ui_controller.get_project_bpm`；播放位置=Top_Transport_Bar `_transport_position_now`；循环区=lane `_time_selection_state`；轨道头=Track_Row_Controller 名称/音量读数函数。
- 目标：
  1. **供给器契约+注册处**（§6.0）：duck-typing 四函数（identity/affordance/rects/resolve）+组 `vit_face_supplier` 注册；解析编排宿主=manager（`resolve_circle(rect)`：§5.1 管线①–⑤——面枚举/占比双口径（D3：主=selection_share 辅=face_coverage）/MIN_FACE_INTER_AREA_PX=4.0 噪声门/供给器解析/降序合并）。
  2. **timeline 供给器**（§6.1）：face_rects=各行 Right_3D_Wrapper 全局矩形；resolve 复用 `_collect_marquee_hits_global`（跨 lane 组聚合）→entries=clip 清单+track_ids+time_window（命中包络，空命中=rect×时间轴映射派生 V1 §5.1 规则）。
  3. **control 快照供给器**（§6.4）：face_rects=走带条+各轨 Header_2D 两块；resolve=snapshot 形态（BPM/播放位置/循环区/相交轨参数当前值）；observe only。
  4. **payload v2 接线**（§9）：`_on_circle_select_finished` 从 faces 占位升级为真解析结果；胶囊文案按命中面数更新（「圈选 N 面」）；note 载荷=faces[] 降序+空辖区合法态。
  5. **headless 探针**：§10.1 解析管线探针（伪造供给器 rect/entries 断言占比/排序/合并/空辖区/噪声门过滤+契约四函数签名）落 tools/，`--headless -s` 跑通；**原始输出落 coord/runs/VITNOTE-V2-IMPL-B/ 留档**。
  6. 手测覆盖边界如实申报：真实跨面矩形/占比目检归手测（§10.2 第 3[单面 timeline]/4[控制面]步——本卡两面的手测判据）。
- 文件域：`app/tracks/vitnote/vit_note_manager.gd`（解析编排+payload 接线）+新建 `vit_face_supplier_timeline.gd`+`vit_face_supplier_control.gd`（vitnote 目录）+tools/ 探针——预计 ≤4 文件。
- 约束：新分支 `port/vitnote-v2-impl-b` 自 port/vitnote-v2-impl-a@091cf80 切出；不动 main；运行时目录勿动；lane 采集函数**只调用不修改**（越界即停）；无自动化测试基建如实申报。
- 验收标准：headless 解析/契约探针全绿 EXIT=0+`--import` 零 SCRIPT ERROR+探针输出落 runs/+timeline/control 两供给器注册与 resolve 形态与设计 §5.2/§6.4 逐字段对上+用户手测 §10.2 第 3（单面）/4 步复验通过（挂 V2 手测场）。
- 停止条件：解析编排发现设计 §5.1 管线与 IMPL-A 出口不兼容（如 rect 坐标域不一致）→ 停止上交带锚点；采集函数复用发现形态缺口（缺字段）→ 如实申报最小补齐方案转决策侧裁（不擅自改 lane）。
- 领取：2026-10-02 PC 执行侧（GLM）/ Godot 仓基线 091cf80a00c2e565e670859cf8f3dd8719300932（port/vitnote-v2-impl-a 已验收 HEAD，领取时该分支工作树干净，仅运行时未跟踪目录）/ 分支 port/vitnote-v2-impl-b
- 回执：2026-10-02 PC 执行侧（GLM）。
  - **commit**：Godot 仓 port/vitnote-v2-impl-b@3ea8a0a（自 091cf80 切出，已推 origin）；diff=manager 150 行改+两供给器新建（timeline 170 行/control 221 行）+探针 510 行，共 6 文件 +1046/-7。
  - **探针输出留档**：coord/runs/VITNOTE-V2-IMPL-B/（README + 4 轮探针原始输出 + import 零错误日志）。终版 74/74 EXIT=0（静态契约 10+沙盒管线 52+两面烟测 12）；`--import` EXIT=0 零 SCRIPT/PARSE ERROR。
  - **供给器 diff**：vit_face_supplier_timeline.gd（§6.1：Right_3D_Wrapper 行矩形+`_collect_marquee_hits_global` 只调不改+time_window 包络/映射两分支+face_id scope=场景根名两面可区分）；vit_face_supplier_control.gd（§6.4：走带+轨头两块+snapshot 形态 F10 锚点只读+observe only）；vit_note_manager.gd（§6.0 注册处+resolve_circle §5.1 管线①–⑤+payload v2 faces 真接线+胶囊「圈选 N 面」/0=空辖区）。
  - **自验清单**：①headless 探针四轮（run1 红=实现真实缺陷：§6.0 字面「_ready 入组」不对称，重挂不重注册——改 _enter_tree/_exit_tree 对称后全绿，首红日志留档）；②lane 文件域零改动核实（git diff 091cf80..3ea8a0a 对 track_scene/timeline/shell/legacy/vit_dock 全空）；③停止条件未触发——IMPL-A 出口兼容（rect_global 全局坐标域直连）、F6 采集形态无缺口（命中字典含所需字段、秒映射公开 API 在位）。
  - **申报**：payload 键名沿 IMPL-A 仓内约定 rect_global（panel 消费方兼容）；v2 下面板 _rebuild_summary 读 v1 顶层字段显「空辖区」——多面版摘要归 IMPL-D（panel 零改动，文件域外）；真实跨面矩形/占比目检归手测 §10.2 第 3[单面]/4 步挂 V2 手测场；**运行栈未起**（本卡验收面=headless 探针+import，无真实栈烟测需求）。
- 验收：（裁定文件 / 验收 commit）

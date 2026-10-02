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
- 领取：（时间 / Godot 仓基线 hash / 分支名）
- 回执：（commit hash / 探针输出留档 / 供给器 diff / 自验清单）
- 验收：（裁定文件 / 验收 commit）

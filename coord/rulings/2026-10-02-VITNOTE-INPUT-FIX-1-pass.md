# Ruling：VITNOTE-INPUT-FIX-1 — pass（2026-10-02 决策会话；面板终验挂用户手测）

## 验收四层

1. **diff 直读**（Godot 仓 port/vitnote-input-fix-1@d97293c，3 产品文件 +89/-1+探针 360 行）：
   - 锚点 A ✓：router 左键分支 `_global_point_targets_floating_interactive_control` 组旁路前置于 lane 转发（:1734 前）+`_left_ui_interaction_in_progress` 压放全序闩锁（面板起手拖拽穿出不误起 marquee）+motion 分支同款（GUI hover 更新不被 lane 消费阻断）。
   - 锚点 B ✓：`_candidate_interactive_hit_roots` 组根入枚举（置于 roots.is_empty() 兜底之前——组根在时间线宿主外，兜底扫自身子级永远覆盖不到，注释有据）+manager `_ready` 入组+`set_anchors_and_offsets_preset` 修正（仅 set_anchors_preset 不重设 offsets 致 rect(0,0,0,0) 残留——附加发现合理且必要：根枚举按根 rect 过滤会漏层）。
   - 锚点 C ✓：仲裁器 `_should_accept_mouse_at_global` 组短路，`mouse_filter != IGNORE` 判据正合设计 F4 盲区（PanelContainer 底板非 BaseButton 不被旧清单认）。
   - **设计偏离申报采信**：未整段前移原 interactive 旁路，只前置组旁路——取证证实 smart 模式下 lane 自判交互，整段前移会灭画布 marquee；"画布零回退"回归约束优先于卡面字面，属 §7.3 锚点 A 的保守落地，偏离理由充分且如实申报（§11 纪律正确执行）。
2. **我方独立复跑**：①headless `--import` 我方自跑 EXIT=0 零 SCRIPT ERROR；②**窗口模式 legacy 探针我方自跑全绿 EXIT=0**：`collapse_pressed=true panel_visible_after=false`（收起触发）+hover=面板按钮路径（恢复）+`lane_marquee_press=false`（面板在场零穿透）+隐藏对照 `=true`（画布手势零回退）+`candidate_roots=[Track_Master, VitNoteLayer]`（根枚举补全）+组注册 `[VitNoteLayer]`——回执声称的修复后状态逐项独立复现。
3. **工件亲读**：探针为真仪器（Input.parse_input_event 注入+A/B/C 三判据+VERDICT 解读行，产品代码零改动）。**注记**：探针原始输出未落盘（前后对比仅卡内结构化记录）——本验收以我方重跑复证补正；后续 Godot 卡探针输出建议落 coord/runs/ 留档（§9 回指要求）。
4. **边界声明核对**：dock 面 GUI 链 headless 不可判（64x64 固定几何+无内核周期重建）如实申报；渲染/IME/真实窗口几何归用户手测；运行栈声明在案（未启三件套，探针进程零残留）。

## 裁定

- **pass**。用户面板复验（卡面手测清单①-⑤）为终验项——可并入今日合并手测场（A/B 卡点击+采纳+新输入）顺带做，或 V2 手测场；不另设卡。
- Godot 分支链：port/vitnote-input-fix-1@d97293c 为现 HEAD；**V2-IMPL-A 依赖解锁**（卡已入池，自本分支切出）。

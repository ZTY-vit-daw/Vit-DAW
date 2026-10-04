# Ruling：VITNOTE-REGION-TIME-1 —— conditional pass（2026-10-04，PC 决策侧）

## 裁定

**conditional pass**。机制面三件套（diff 直读+我方独立复跑+工件核对）全过；转正挂**用户真栈手测三复验点**（卡面回执"手测复验点"①②③，四号场正靶：问「这个范围是什么内容」回答含时间段）。

## 验收依据

1. **diff 直读**：
   - Godot `ecc722d`（4 文件 +258/-1）：`vit_face_supplier_timeline.gd` resolve 增 `range_time_span`（框选左右缘×`timeline_seconds_at_global_point`，与 time_window 空命中同源映射）+每条目 `range_clip_start/end`（静态纯函数 `clip_range_time_intersection`，end<=start=零交缺省）+`vit_note_panel.gd` v3.1 顶层提升（异形/缺省键不加）；`vit_note_manager.gd` 零改动与申报一致。
   - agent `1770f3c9`+`0364bcc5`（2 文件 +324/-5）：`NoteChatPayload.RangeTimeSpan` 顶层（防 JSON 静默丢弃补丁）+`noteJurisdictionDigest` 摘要函数组（m:ss.d 格式化/「时间线 0:03.2–0:08.5：命中 N 个 clip：」头行/相交段合法性 rs>=span[0] 核对）+系统段 prompt 指令（带时间界、字段缺席不伪造）。旧载荷快照形态与 v3 完全一致（fail-open 兼容面）。
   - 停止条件解除核实：像素↔时间映射在 resolve 层可得（同源映射调用），无需 lane 内部新状态——成立。
2. **我方独立复跑**（[verify_pc](../runs/VITNOTE-REGION-TIME-1/verify_pc/)）：`--import` exit 0；探针族 notestream **26/0**、container **141/0**、face_resolve **143/0**（含新时间维度套件：静态五例+假 lane 真链三态+降级）、circle_state PASS、panel_summary **16/0**、input exit 0（取证形态为其设计形态）；Go `go build ./...`+chat 全包 `ok 87.301s`（含 note 新钉）。
3. **口径小差留档**：回执 +326/-7 vs diff +324/-5——两 commit 分别统计相加口径，无实质差异。

## 归档与配套

- agent 腿合 main=**4c98c643**（--no-ff）；Godot `ecc722d` 已推 `port/vitnote-region-time-1`（前端工作树即此分支，用户手测直接可用）。
- 主树收口重建：`agent/bin/VitAgent.exe` 11:39:24 + webui dist 11:39:53（含本卡与 SESSION-SEMANTICS-1 双腿）。

## 转正条件

用户真栈手测（Godot 拉起途径）三复验点全过 → 转正 pass；任一不过 → 取证卡。

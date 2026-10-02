# Ruling：VITNOTE-V2-IMPL-B — pass（2026-10-02 决策会话；跨面目检挂 V2 手测场）

## 验收四层

1. **diff 直读**（Godot 仓 port/vitnote-v2-impl-b@3ea8a0a，6 文件 +1046/-7）：
   - `resolve_circle` 与设计 §5.1 管线①–⑤**逐条对齐**：组枚举+多块面逐块求交（块内重叠退化配置钳制 1.0 防占比虚高——设计未明说的边界，实现补齐且注释有据）；占比双口径（D3 selection_share 主/face_coverage 辅）；4.0px² 噪声门；供给器 resolve 失败=空 entries 不阻塞他面；降序合并（平局按 priority 再 face_id 字典序=确定性增益，priority 排序后抹除不进载荷——§6.0 口径）。零面积矩形守卫。
   - 供给器契约：timeline（Right_3D_Wrapper 行矩形+`_collect_marquee_hits_global` **只调不改**+time_window 包络/映射两分支+face_id 场景根 scope 两面可区分）与 control（走带+轨头两块+snapshot F10 锚点只读+**缺源=-null 如实降级不造假值**）均合规；注册生命周期 `_enter_tree/_exit_tree` 对称（探针 run1 红暴露的字面「_ready 入组」不对称真实缺陷已修——红先行证据在档）。
   - **lane 文件域零改动**：diff 091cf80..3ea8a0a 对 track_scene/timeline/legacy/vit_dock 全空（卡面约束"只调不改"兑现）。
   - payload v2 真接线：faces[] 降序入载荷+胶囊「圈选 N 面/空辖区」两分支文案；rect_global 键名沿 IMPL-A 仓内约定（panel 消费方兼容申报合理）。
2. **我方独立复跑**：headless 探针我方自跑 **checks=74 failed=0，EXIT=0**（静态契约 10+沙盒管线 52+两面烟测 12）；`--import` 零 SCRIPT/PARSE ERROR。与回执终版一致。
3. **工件亲读**：coord/runs/VITNOTE-V2-IMPL-B/（README+四轮探针+import）——四轮含 run1 红（真缺陷）与 run2 探针侧 scope 修正的过程证据，红绿链完整可回指。
4. **边界核对**：跨面真实矩形/占比目检归手测（§10.2 第 3[单面]/4 步挂 V2 手测场）；v2 下面板 `_rebuild_summary` 读 v1 顶层字段显「空辖区」=已知边界（多面版摘要归 IMPL-D，panel 在其文件域——申报合理）；运行栈未起（验收面=headless，无真实栈需求）声明在案。

## 裁定

- **pass**。停止条件正确未触发（IMPL-A 出口兼容+F6 采集形态无缺口——均按卡核验）。
- 分支链：port/vitnote-v2-impl-b@3ea8a0a 为现 HEAD；**V2-IMPL-C 依赖解锁**（机架+资料库供给器，设计 §6.2/§6.3+诚实降级注释）。

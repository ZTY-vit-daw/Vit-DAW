# Ruling：VITNOTE-V2-IMPL-D — conditional pass（实现面 pass；九步手测+三卡销项待用户执行，2026-10-02 决策会话）

## 实现面验收（本裁定覆盖部分）

1. **diff 直读**（Godot 仓 port/vitnote-v2-impl-d@fd7e6af，2 文件 +176/-4）：
   - `_rebuild_summary` 升级读 faces[]：label+整数百分比（roundi）按载荷原序拼接（resolve_circle 契约已降序，呈现层不重排——§5.3 无主导面语义）；**空辖区/非数组/非字典元素/label 空三级兜底（label→face_kind→「面」）全在位**；tooltip 同步；v1 顶层 track_ids/clip_ids 读取移除（IMPL-B 起上移入 timeline 面 domain，无活生产者——删除正当）。
   - 探针 16 项红绿链：run1 一红=face_kind 空串边界真缺陷（get 默认值不生效）→修复后两轮 16/16——红先行纪律兑现，日志在档。
2. **我方独立复跑（全套五件）**：panel 探针 16/16 EXIT=0+face_resolve 回归 127/127+circle_state 回归 52/52+input 探针 EXIT=0+`--import` 零 SCRIPT/PARSE ERROR——**四卡分支链累计回归零破坏**。
3. **工件亲读**：runs/ 八件含 MANUAL_TEST_9STEPS.md——**编排质量优**（九步操作+判据逐条+AGENTS §5 唯一入口+§9 排他声明+止损条款+记录方式；第 7 步兼作 D1 迁键新行为回归、第 8 步兼作 INPUT-FIX 主判据——一场多卡复验设计合理）。
4. 运行栈：执行侧未起真实栈（泊位声明）——手测栈由用户拉起独占，合规。

## 待用户执行（终裁条件）

- **九步手测**按 MANUAL_TEST_9STEPS.md 执行（前端分支已就位 port/vitnote-v2-impl-d@fd7e6af）；结果落 MANUAL_TEST_9STEPS_RESULT.md。
- 第 9 步三卡销项：VITNOTE-IMPL-1（步骤 1/2/8 条件项）+VITNOTE-DOCK-MOUNT-1（全程 dock 面条件项）+V2 五卡收官。
- **九步全过 → 本卡转 pass 终裁+三卡销项落簿；任何一步不过 → 卡移 blocked 附现象+锚点**（停止条件照卡）。

## 关联提醒

- 今日合并手测（AB-JUDGMENT+FS-ADOPT 终验：park→点 A/B 卡→settle→新输入）与九步手测同需三件套，**建议同场顺做**；agent 二进制（10-01 22:29）与 dist 仍现势（其后 main 仅文档/coord/测试/VitApp-mac-面变更，PC 栈零影响）。

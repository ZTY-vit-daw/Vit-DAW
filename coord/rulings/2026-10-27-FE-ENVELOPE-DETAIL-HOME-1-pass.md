# Ruling：FE-ENVELOPE-DETAIL-HOME-1（200）——pass（含一处工件纪律偏差记档）

- 验收人：GLM 主管决策流 / 2026-10-27 午后
- 合入：前端 a0bc0c4（实现 61f2135+探针修复 0cc771a）+主仓 5604c13e（真栈工件，主管代归档）。

## 亲核面

1. **diff 亲读**（十文件关键面）：①vit_dock_root.gd——vit_open_envelope_detail（复用键 track:<id> 与方块按钮同窗 ✓ F1=A 裁定①）+_open_or_focus_detail_window view_type 参数化+已开窗同视图 set_view_context 刷新；②viewport_registry.gd——envelope spawnable_in_viewport=false（media_pool 同款语义，台前启动器撤列 ✓ 裁定③）；③timeline_dock_adapter.gd——⤢ 转发重定向 vit_open_envelope_detail（✓ 裁定②）；④旧口 vit_open_envelope_in_stage_slot 降级为警告空实现零 spawn；⑤viewport_frame.gd+ViewportFrame.tscn——set_window_view_options 显式清单（detail/envelope，不共 spawnable——**未触发概念级 registry 停止条件**，两全方案成立）。
2. **判定面**：新探针 52/52+**194 探针 R/O 段迁移 79→83 不弱化**（撤列翻转+清单完整性并行+新入口/零 stage spawn/降级存根三面新断言）+真栈 R2/R3 连续 30/30（W0 repo 同步/W1 方块按钮链/W2 同窗复用+窗内切换面两项/W3 ctx 预选+呈现读回内核对照逐点一致/W4 ⤢ 带参直达/W5 启动器撤列真树断言/W6 旧口零 spawn）；R1=探针悬空捕获缺陷轮留证零生产改动（0cc771a 修复面亲读）。
3. **栈纪律**：登记 db0bbf8e 即时复核/释放/错峰在案（PC-RUNTIME-STACK 亲核）。

## 偏差记档

- **工件未推 -stack 分支**：run 工件仅以未跟踪副本滞留主检出（**未跟踪副本第四例**：194/195/197/200/198 五案中的第四发生序）——主管按 §9 工件契约代归档（5604c13e）并已随案入 PROTOCOL 工件纪律（2026-10-27 入律）。回执声称「工件随分支入库」与实际不符，记执行侧回执准确性偏差一次；不阻塞验收（工件内容完整、判定面独立成立）。

## 裁定

**整卡 pass**。F1=A 三裁定条款全数落地（同窗复用/⤢ 重定向/撤列），断言面无弱化；窗内切换形态=显式清单两选项，无新概念。

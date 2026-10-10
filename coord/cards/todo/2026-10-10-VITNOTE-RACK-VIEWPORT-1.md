# VITNOTE-RACK-VIEWPORT-1：多机架视口圈选归因修复（逐视口供给，v2 聚合妥协清偿）

- 发卡：GLM 主管决策侧 / 2026-10-10（用户质疑"圈选能否识别是哪个轨道的机架"——查证结论：单视口能/多视口归因错位，系 V2 已登记妥协，本卡清偿）
- 派发确认：已确认（用户 2026-10-10 指认该缺口）
- 验收负责人：GLM 主管决策流
- 池序 47；目标仓库=**D:\Godot\project\vit-daw-frontend**
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / 无（多视口常开使用时升 P1——用户使用习惯确认后调整）
- 模型分级：L1 / flash 可接（vitnote 供给器族+probe 先例密集）

## 查证结论（2026-10-10 实读，vit_face_supplier_rack.gd 全文）

1. **单机架视口（常规布局）：能识别**——`vit_face_resolve`（:70-96）返回 `track_id`（经 adapter `get_selected_kernel_track_id`）+`track_name`+全链清单（每节点带 track_id/plugin_name/type/params），摘要标签即「机架·<轨名>」（:115-120）。
2. **多机架视口同屏：归因错位**——`vit_face_rects` 枚举**全部**可见 adapter 矩形（:57-64），但 `_rack_scope`/`_selected_track_id` 一律锚定 `adapters[0]`（首个可见 adapter，:106-153）→ 在第二/三视口上圈选会被归因到**首视口的选定轨**。这是 V2 实现时明文登记的聚合妥协（:11-14 注释："逐视口拆供给器需 adapter 侧注册/通知钩，越本卡只读域"）——**非未知 bug，是已登记的 v2.x 待办**；本卡为正式清偿。
3. 附带语义注记：track 来自**选定态**而非圈选空间位置——单视口下两者恒一致（机架跟随选定轨），无额外缺陷。

## 目标

1. adapter 场景根（GraphRackDockAdapter，脚本在 vit_control_v_1.0.gd 所在树）加组注册/通知钩：每视口实例自供 face（rects+resolve 锚定**自身**选定轨）——**越 V2 卡"三只读文件"约束由本卡正式授权**。
2. 静态聚合供给器退役或降级为兜底（无注册 adapter 的异常布局仍可用）。
3. 圈选按被圈矩形归因到所属视口（vit_note_manager 归因面如需配合则最小改）。

## 文件域

`app/tracks/vitnote/vit_face_supplier_rack.gd` + adapter 侧注册小改（vit_control_v_1.0.gd/场景）+ `tools/probe_vitnote_face_resolve.gd` 扩多视口断言。越域即停。

## 验收标准

- headless probe：双 adapter 场景（两视口选定轨不同）各自矩形圈选→各自 track_id 归因正确；单视口全量回归（既有 probe 不动全绿）。
- 归档注释更新（:11-14 妥协注记标记已清偿+本卡号）。

## 停止条件

adapter 场景根结构不支持按实例注册（如场景是单例不可多实例）→ 形态上交（多视口布局本身如何产生的事实清单一并附上）。

## 并行与资源

不占真栈；与 FE-RACK-CTX-PLUGIN-LOAD-1 文件域不相交（本卡碰 vitnote+adapter，彼卡碰 Vit_Graph_Rack+新弹窗）——**如 adapter 改动面重叠则串行，领取时核对**。

- 领取：（时间 / 前端仓 HEAD / owner / 分支 / 领取提交）
- 回执：（commit / probe 断言结果 / 归因锚点）
- 验收：（裁定文件 / 验收 commit）

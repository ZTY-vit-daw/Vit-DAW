# FE-ENVELOPE-DETAIL-HOME-1：包络视窗并入轨道特写窗——主场重定向（F1=A 裁定实施）

- 发卡：GLM 主管决策流 / 2026-10-27（依据=[decisions/2026-10-27-envelope-home-f1a.md](../../decisions/2026-10-27-envelope-home-f1a.md) 用户 F1=A 裁定；取代 2026-10-26 envelope-editing-home-ruling 的 D1-C 入口条款）
- 派发确认：已确认（用户裁定链）
- 验收负责人：GLM 主管决策流
- 池序 200；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**（领取基线 main=a81e0a5）；P2 / 1 天 / 依赖=无（与 198/199 文件域不相交可并行）
- 模型分级：L2 / PC 执行会话（宿主接线首例）
- 并行与资源：**真栈腿与用户手测节错峰**（§2.2 增补③）；文件域与 198（app/tracks/track_scene）、199（envelope_closeup_canvas.gd）不相交。

## 勘察已核事实（发卡侧，执行侧领取后复核锚点）

1. 特写窗机制：`vit_open_track_detail`→`_open_or_focus_detail_window(ctx, "Track Detail/<id>", 复用键 track:<id>)`——window_manager.create_empty_window+frame.`activate_view("detail", ctx)`；**frame 支持 activate_view 按类型切视图**（vit_dock_root.gd:4754-4780）。
2. envelope_dock_adapter 场景已就位（194 交付），canvas/页签/手势逻辑零改即挂。
3. ⤢ 转发链现状：timeline_dock_adapter.gd `_on_lane_request_open_envelope_closeup`→`vit_open_envelope_in_stage_slot`（:3047-3050）——本卡重定向目标。
4. 空槽启动器选项源=viewport_registry list_spawnable_descriptors（viewport_frame.gd:45）。

## 目标

1. **特写窗包络模式**：新入口 `vit_open_envelope_detail(kernel_track_id, plugin_id="", param_id="")`——打开/聚焦该轨特写窗（与方块按钮同窗，复用键 track:<id>）→activate_view("envelope", ctx 含 plugin/param 预选）；特写窗内视图切换面（拉下菜单/现有切换机制，勘察定具体挂点）含「Envelope」项。
2. **⤢ 重定向**：timeline_dock_adapter 转发目标改 vit_open_envelope_detail；`vit_open_envelope_in_stage_slot` 移除或降级为警告空实现（不得再向 stage 空 slot spawn）；194 探针 R（注册）/O（装配+⤢ 入口）段断言面同步迁移，断言语义不弱化。
3. **全局清单撤下**：台前空槽启动器不再列出 Envelope。机制勘察定：若撤下与特写窗切换共用 spawnable 清单不可兼得，取最小拆分（如 registry 增独立标记或表面过滤）；若需新概念级 registry 结构改动→**形态两案上交**（形态类先过用户面），不擅自扩。
4. **探针**：特写窗 activate envelope 装配+ctx 预选/⤢ 重定向目标/启动器撤列/既有探针零回退（closeup 79/curve 55/lane 39/auto 41/badge 33/datachan 50——其中 closeup R/O 段为断言迁移非弱化）。
5. **真栈腿**：装载 VST→轨道头方块按钮进特写窗→切 Envelope→页签/画布呈现读回一致（内核 get_curve 对照）→车道 ⤢ 带参直达特写窗；§2.2 登记+三纪律+188 守卫+错峰手测节。

## 文件域

`vit_dock/views/envelope_dock_adapter.gd`（+.tscn 如需）/`vit_dock/scenes/vit_dock_root.gd`/`vit_dock/core/viewport_registry.gd`/`vit_dock/views/timeline_dock_adapter.gd`/特写窗 frame 切换机制邻域（viewport_frame.gd 如需）+探针。**不触** `vit_dock/views/envelope_closeup_canvas.gd`（199 域）/`app/tracks/**`（198 域）/`app/rack/**`。越域即停。

## 验收标准

独立探针全绿+既有探针零回退+真栈腿 exit 0（工件入主仓 coord/runs/FE-ENVELOPE-DETAIL-HOME-1/）+**装配面断言**（特写窗内 envelope 模式在真实装配树在场、空槽启动器选项集不含 Envelope）+端测边界声明（渲染像素/手感留手测面）。

## 停止条件

- 特写窗视图切换机制与 activate_view 勘察假设不符（结构漂移）→锚点上交；
- 撤下全局清单与特写窗内可切换不可兼得、需概念级 registry 改动→形态两案上交；
- window_manager 复用键/聚焦链与 ⤢ 直达行为冲突（如已开窗不聚焦/ctx 不刷新）→上交取证。

- 领取：2026-10-27 12:12 +0800 / origin/main=2fa99bd34908b06cc4eb53ca94f521d70ba5782d / owner=GLM-5.3 PC 执行流（ZCode 会话）@Windows PC / 实现分支 port/fe-envelope-detail-home-1（前端仓）/ 实现worktree D:/Godot/vit-daw-frontend-envdetail1（基线 a81e0a5）/ 协调worktree D:/Vit_DAW_wt_envdetail1_coord
- 回执：（2026-10-27 13:00 +0800 / GLM-5.3 PC 执行流 ZCode 会话）实现 commit=61f2135+0cc771a@port/fe-envelope-detail-home-1（61f2135=交付面；0cc771a=探针悬空捕获修复零生产改动）。探针=新独立探针 probe_envelope_detail_home **52/52**（A 注册撤列/B 源码接线/C frame 单件切换面含 Envelope 项+菜单标准切换链/D dock_root 全树：方块按钮开窗→同窗复用切 envelope→ctx 管线读回→⤢ 真树转发直达→旧口零 spawn）+194 探针 R/O 段迁移**不弱化 79→83** 全绿（撤列翻转+清单完整性并行+新入口/零 stage spawn/降级存根三面）+既有零回退（curve 55/lane 39/auto 41/badge 33/datachan 50 全绿）。真栈腿=ps1 最小腿 exit 0+stack_probe_envelope_detail_home **R2/R3 连续 30/30 exit 0**（R1 探针悬空捕获缺陷轮留证：activate_view 重建 adapter 后沿用旧捕获，修复=重取新实例断言）；W 面=方块按钮链→envelope 直达同窗复用+切换面两项→repo 生产链目录+ctx 真选中+外部写三点呈现读回+内核 get_curve 对照逐点一致→⤢ 带参直达（回 detail 后转发直达+新装配预选）→启动器撤列（stage snapshot+全树 launcher 标签）→旧口降级零 spawn。run=coord/runs/FE-ENVELOPE-DETAIL-HOME-1/envelope_detail_home_20261027_1245（RESULTS.md；工件含 ps1/probe r1-r3/隔离副本）。占用=登记 db0bbf8e（12:39 即时复核 <60s）→12:50 拆栈复核全零；错峰在案（196 已收口+198 探针进程留档未动）。撤列机制=registry `spawnable_in_viewport=false` 单标记翻转（media_pool 同款语义），特写窗切换面用 `set_window_view_options` 显式清单不共用 spawnable——**未触发概念级 registry 停止条件**。端测边界声明：装配树/状态机/数据链 headless 断言面已覆盖；窗口视觉呈现/菜单与 ⤢ 点击手感=用户手测面（渲染面 E2E-WEBUI-1/旅程 JOURNEY-1 建设卡前按 AGENTS §5 显式声明）。
- 验收：**整卡 pass**（GLM 主管决策流 2026-10-27；rulings/2026-10-27-FE-ENVELOPE-DETAIL-HOME-1-pass.md；前端合入 a0bc0c4+工件主管代归档 5604c13e。工件未推分支偏差记档一次）

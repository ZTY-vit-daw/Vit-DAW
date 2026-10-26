# M3-ENVELOPE-CLOSEUP-RECON-1：特写窗口「包络视窗」勘察+设计（C4 前置）
- 发卡：GLM 主管决策流 / 2026-10-26 晚（依据=用户 2026-10-26 会话裁定：特写窗口为多视窗宿主——View_Gizmo Top/3D/Side+轨头钢琴卷帘切换；包络编辑=新增专用视窗，2D 画布型；曲率编辑首版要做且更易用；车道（189）仅呈现+入口）
- 派发确认：已确认（用户会话裁定链；形态细节仍须预览过用户后才能开实施卡）
- 验收负责人：GLM 主管决策流
- 池序 193；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**（领取基线 main=20873f8）+主仓报告；P3 / 0.5 天 / 依赖=无（190 已合入供消费面参考）
- 模型分级：L1 / PC 执行会话；纯勘察只读，无真栈腿，不需要栈登记（只读跑 Godot 结构探针也不启栈）

## 目标（勘察七问）
1. **视图切换机制现状**：Right_3D_Wrapper 的 View_Gizmo（Btn_Top/Btn_3D/Btn_Side）与轨头 Btn_PianoRoll_Toggle（:3816 _on_piano_roll_pressed）的切换链路——新增一个「包络」视窗的挂载点与切换按钮位置择型（View_Gizmo +1 按钮 vs 轨头按钮 vs 底栏 Btn_Mode_Toggle）。
2. **视窗本体择型**：2D 画布型（纯 Control/_draw，推荐——手势=2D 拾取无深度误触）vs 相机预设型（正交相机拍 3D 平面）；给出推荐与理由。
3. **时间轴/播放头/缩放复用面**：特写区 time_zoom_factor/播放头/纸面背景/HUD 的复用边界；包络视窗时间范围=全轨（非 clip 焦点）的取数路径。
4. **参数选择与多参数**：视窗内参数页签/列表形态（Serum 式）；数据源=rack_nodes 目录+get_plugin_parameters 描述符（含 envelope_point_count 在场性，外部插件参数覆盖面如实）。
5. **编辑手势面盘点**：单击折线加点（按下即拖）/拖点（锁轴修饰键）/Delete 删点/段中点曲率句柄+右键预设（线性/快入/快出/S）——逐项列实现锚点候选（既有手势机/仲裁器可复用面；视窗内局部状态机 vs 全局仲裁器接入择型）。
6. **写回链设计**：释放恰一笔 set_points（含 c 曲率字段，内核面已就绪）/区域替换/回显门/编辑中外部变更锚定重绘——锚到 M3-KERNEL-1/187/C1 既有件。
7. **车道↔视窗关系与文件域盘点**：车道（189）=呈现+点击进视窗入口的接线点；新视窗文件清单+与 191（app/rack）/192（timeline 车道）并行关系核对。

## 产出
设计报告入主仓 coord/runs/M3-ENVELOPE-CLOSEUP-RECON-1/（含给用户的形态选项+预览——静态 HTML/PNG 或结构描述）；**形态类裁定点单列清单（视窗切换入口/参数页签/曲率句柄形态），过用户面后才开实施卡**（决策档 2026-10-25）。

## 文件域
前端仓只读勘察+主仓 run 目录报告；零生产代码改动。越域即停。

## 验收标准
报告覆盖七问（每问有锚点行号+推荐案）；形态选项≥1 组带预览；无生产 diff（git status 前后一致）。

## 停止条件
- 视图切换机制与假设不符（如视窗挂载需重构宿主容器）→锚点上交；
- 时间轴复用面缺关键件（播放头/缩放无法在 2D 画布视窗复用）→如实记录并给新建件估算。

- 领取：2026-10-26 晚 / origin=main b3b87248 / owner=GLM-5.3 PC 执行会话 / 协调分支 coord/closeup1-claim（纯勘察无实现分支，前端仓只读基线 20873f8）/ worktree D:/Vit_DAW_wt_closeup1_coord / 领取提交=本条 commit
- 回执：（报告=coord/runs/M3-ENVELOPE-CLOSEUP-RECON-1/recon-report.md（七问全锚点+推荐案）+ 预览=envelope-closeup-form-options-v1.html / 形态裁定点：D1 入口（推荐 C 启动器首版+B 车道⤢待 192 合入，A 轨头备选）、D2 参数页签（推荐 A Serum 式横向条）、D3 曲率句柄（推荐 A 段中点单柄+右键四档预设）——均待用户裁定后才开 C4 实施卡 / 锚点摘要：①视窗接入=viewport_registry.gd:5-67 注册 envelope+vit_dock_root.gd:4877 同式推槽（钢琴卷帘双路径先例 :4804/:4877；View_Gizmo=right_3d_wrapper.gd:1178-1240 轨内 3D 相机切换、非视窗面）②2D 画布定型（vit_detail_canvas.gd 同型仿写）③t→x/播放头三级回退复用 track_envelope_lane_2d.gd:584-649+total_duration 喂链 telemetry_manager.gd:1010 ④数据链 189 就绪+envelope_point_count=190 内核字段前端首消费 ⑤手势=视窗局部状态机（仲裁器领土互斥 TimelineInputArbitrator.gd:175）⑥写回=释放一笔 set_points region 模式+回显门（automation_client.gd:59-268 全就绪，c 三键在内）⑦车道入口接线点 Track_Row_Controller.gd:1070-1126；视窗核心域与 191/192 零相交，轨头/车道入口按钮与 192 域相交须错峰 / 纪律：前端仓零改动亲证（tracked diff=0、HEAD=20873f8 不变、untracked 仅既有运行时目录）；无真栈腿未启栈；主仓改动仅 coord/）
- 验收：**整卡 pass**（GLM 主管决策流 2026-10-26；rulings/2026-10-26-M3-ENVELOPE-CLOSEUP-RECON-1-pass.md；D1/D2/D3 待用户裁定后开 C4 实施卡）

# M3-ENVELOPE-CLOSEUP-VIEW-1：特写窗口「包络视窗」——画布+页签+点位编辑+写回闭环（C4 一段）
- 发卡：GLM 主管决策流 / 2026-10-26 晚（依据=193 勘察验收+用户形态裁定 D1/D2/D3+决策档 2026-10-26-envelope-editing-home-ruling）
- 派发确认：已确认（用户裁定链）
- 验收负责人：GLM 主管决策流
- 池序 194；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**（领取基线 main=53293a5）；P2 / 1 天 / 依赖=无（191/192 已合入；190 字段在内核与前端消费链均已实证）
- 模型分级：L2 / PC 执行会话（新视窗组件首例，量级同 189）
- 并行与资源：与 195（曲率卡）同域串卡（195 依赖本卡）；与其他在飞卡文件域不相交；真栈腿按 §2.2 三纪律+188 守卫，无并行真栈卡时无需错峰（领取时核对）。

## 设计裁定（用户 2026-10-26，权威约束）
- D1 入口=**C 启动器菜单+B 车道头「放大」按钮/车道双击**（A 轨头按钮备选不做）；
- D2 参数页签=**A Serum 式横向页签条**（有包络参数带标记）；
- 曲率句柄归 195（D3A 已裁）；本卡点位编辑不含曲率编辑（c 字段透传保持不丢）；
- 车道=仅呈现+入口，车道内零编辑（v2 裁定）；预览=envelope-closeup-form-options-v1.html（主仓 193 run 目录）。

## 目标（依 193 勘察七问锚点）
1. **视窗注册与打开**：viewport_registry.gd +envelope 条目（:5-67）；vit_dock_root.gd +vit_open_envelope_in_stage_slot（仿钢琴卷帘 :4804-4893/:4877 先例）；入口=启动器菜单项+车道头「放大」按钮/车道双击（track_envelope_lane_2d.gd 车道头邻域+timeline_dock_adapter.gd 信号转发）。
2. **2D 画布呈现**：envelope_closeup_canvas.gd（Control._draw，仿 vit_detail_canvas+车道绘制式样）：t→x/播放头三级回退+18ms scrub+重绘节流复用（lane :584-658 同式）；全轨时间范围=total_duration 广播链；点+折线+值域标签（描述符单位）。
3. **参数页签**（D2A）：横向页签条，数据链=rack_nodes 目录+get_plugin_parameters（189 同源）+C1 get_curve；有包络参数标记=envelope_point_count>0（190 字段，徽章/自动档后第三消费者）。
4. **点位编辑**：单击折线加点（按下即拖）+拖点（锁轴修饰键+实时数值标签）+Delete 删点；**视窗内局部手势状态机**（不接 timeline 全局仲裁器，勘察 Q5；钢琴卷帘视窗同款领土隔离）。
5. **写回闭环**：释放恰一笔 automation.set_points（points=[{t,v,c}]，region 模式，C1/automation_client.gd:59-94,122-139,195-268）+回显门（request_id 路由回执重拉）+编辑中外部变更=拖拽态忽略外部刷新、释放合并（勘察 Q6 案）；覆盖面=外部 rack VST 插件参数（轨级缺口主管已另裁，如实呈现）。

## 文件域
vit_dock/views/envelope_dock_adapter.gd+.tscn（新）/ vit_dock/views/envelope_closeup_canvas.gd（新）/ vit_dock/core/viewport_registry.gd（+条目）/ vit_dock/scenes/vit_dock_root.gd（+打开函数）/ app/tracks/track_scene/track_envelope_lane_2d.gd（车道头入口按钮，**不触三态档位逻辑邻域**）/ vit_dock/views/timeline_dock_adapter.gd（信号转发）+探针新文件。不触 app/rack。越域即停。

## 验收标准
独立探针全绿（注册/打开/页签/呈现/手势机/写回链 fake 传输）+既有探针零回退（192 合入后全家含 lane 41/auto 基线）+真栈腿 exit 0（装载插件→开视窗→页签选参→呈现读回逐点一致→加点/拖点/删点→内核 get_curve 读回逐点一致→外部写→回显门合并）；工件入主仓 coord/runs/M3-ENVELOPE-CLOSEUP-VIEW-1/；端测边界声明。

## 停止条件
- registry/dock_root 挂载点与勘察锚点不符（结构已重构）→锚点上交；
- 写回链 c 字段/region 语义与 C1 契约不符→锚点上交；
- 与钢琴卷帘/详绘画布视窗的状态互斥或资源冲突超勘察预期→上交。

- 领取：2026-10-26 晚窗 / 主仓 origin/main=5bc607a1 / 前端仓领取基线 main=53293a5（worktree envcloseup1 新建后 git status --short 干净亲核）/ owner=ZCode GLM-5.3 PC 执行会话（M3-ENVELOPE-CLOSEUP-VIEW-1 卡 194 专属）/ 前端分支 port/m3-envelope-closeup-view-1 / worktree D:/Godot/project/vit-daw-frontend-envcloseup1 / 领取提交=（hash 后补）；领取前核对：卡在 todo+派发已确认（fetch 亲读 origin/main=5bc607a1）、依赖无（191/192 已合入前端 main=53293a5）、doing 仅 GEN-SUNO-WEB-DESIGN-1（域不相交）、195 未领取（依赖本卡）、PC-RUNTIME-STACK 当前占用=空（最新释放 192@20:13）。
- 回执：（commit / 探针断言面 / 真栈 run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）

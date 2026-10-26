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

- 领取：2026-10-26 晚窗 / 主仓 origin/main=5bc607a1 / 前端仓领取基线 main=53293a5（worktree envcloseup1 新建后 git status --short 干净亲核）/ owner=ZCode GLM-5.3 PC 执行会话（M3-ENVELOPE-CLOSEUP-VIEW-1 卡 194 专属）/ 前端分支 port/m3-envelope-closeup-view-1 / worktree D:/Godot/project/vit-daw-frontend-envcloseup1 / 领取提交=29879928；领取前核对：卡在 todo+派发已确认（fetch 亲读 origin/main=5bc607a1）、依赖无（191/192 已合入前端 main=53293a5）、doing 仅 GEN-SUNO-WEB-DESIGN-1（域不相交）、195 未领取（依赖本卡）、PC-RUNTIME-STACK 当前占用=空（最新释放 192@20:13）。
- 回执：2026-10-26 晚窗自验完成，交主管验收（详 [runs/M3-ENVELOPE-CLOSEUP-VIEW-1/envelope_closeup_20261026_2131/RESULTS.md](../../runs/M3-ENVELOPE-CLOSEUP-VIEW-1/envelope_closeup_20261026_2131/RESULTS.md)）——
  - **commit**：前端仓 `port/m3-envelope-closeup-view-1@2f232a0`（8 文件 2293 行：envelope_closeup_canvas.gd 891 行/envelope_dock_adapter.gd+.tscn/registry 条目/dock_root vit_open_envelope_in_stage_slot/车道 ⤢+信号/timeline_dock_adapter 递归转发/独立+真栈探针；**vit_dock 其余四件/app/rack 零触；lane 改动仅在车头按钮区=三态档位逻辑邻域零触；vit_track_lane_2d.gd/rack_client.gd/automation_client.gd 零改**=文件域遵守）；主仓 `port/m3-envelope-closeup-view-1-stack@84662133`（真栈工件+RESULTS.md）；均已推远端。
  - **探针断言面**：独立探针 probe_envelope_closeup **79/79**（R 注册 5/O 装配+⤢入口 8/T 页签 12[envelope_point_count>0 亮标 ●N/零值与字段缺席不标+灰显/自动选首个有包络参]/P 呈现 6[逐点 t,v,c+描述符值域+全轨时间窗线性]/G 手势 24[拖点恰一笔 region 区域点集 c 透传/Shift 锁 t/Ctrl 锁 v/零位移零写回/折线加点按下即建/Delete 区域重建与唯一点 clear_region 分流]/W 写回链 11[回显门赋值前比对——探针驱动修复真缺陷：恒等比对使 mismatch 永不触发/回显不一致对齐内核+披露/写回 error 回滚+文案透传/拖拽态外部忽略+释放合并 echo wins/他方写重拉]/E 错误面 6[beats 拒呈现/文案逐字/queue_failed/空目录]）；既有探针零回退：lane_ui 39/lane_auto 41/badge 33/datachan 50 全 PASS（探针驱动中还修复两处生产缺陷：页签重建 queue_free 延迟期同名改名失锚=先摘树再释放；set_curve_state_pending 全清打断拖拽态/回显挂起=轻量置态）。
  - **真栈 run ID**：`coord/runs/M3-ENVELOPE-CLOSEUP-VIEW-1/envelope_closeup_20261026_2131`——ps1 最小腿 exit 0 + stack_probe_envelope_closeup **28/28 连续两轮 exit 0**（R2/R3；R1=探针期望缺陷轮留证非生产缺陷）；登记 d3bcd919/释放 f6092939，三纪律+VIT_PROJECT_XML 隔离三面亲证全执行；kernel=192 自建只读复用（本卡零内核改动亲核 e70bafc7..origin/main 无 VitApp/ 提交）。腿覆盖卡面验收③全链：装载→开视窗 ctx 预选→页签真描述符链→呈现读回逐点一致→加点/拖点/删点（局部手势）→内核 get_curve 逐点对照一致→外部写→回显门合并（区域内本地覆盖+region 外外部保留）→删至唯一点 clear_region 回空。
  - **端测边界声明**：①渲染面——Godot 视窗组件非 webui，浏览器 DOM 烟测门槛不适用；探针断言=画布内部状态+内核对照（headless 无像素输出），**_draw 像素级视觉（网格/播放头/拖拽标签）未自动化断言**；②GUI 交互路径——启动器菜单点击/dock_root 舞台槽推入的**装配面断言=registry 条目+adapter 场景 setup_view 生命周期级**（源码级断言 vit_open_envelope_in_stage_slot 在场），全 dock 装配与真实点击旅程未自动化（headless 无 dock_root 实例化——autoload 依赖，189 探针同款边界）；③手测途径=用户从 Godot 拉起前端→空槽启动器选 Envelope/车道 ⤢（ctx 带当前参数直达）；④独立窗路径（window_manager）未做（卡面只要求 stage_slot 口径）。
- 验收：**整卡 pass**（GLM 主管决策流 2026-10-26；rulings/2026-10-26-M3-ENVELOPE-CLOSEUP-VIEW-1-pass.md；前端合入 ef3fc29；195 解锁）

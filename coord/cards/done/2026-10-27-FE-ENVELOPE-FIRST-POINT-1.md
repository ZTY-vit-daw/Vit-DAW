# FE-ENVELOPE-FIRST-POINT-1：包络视窗空曲线首点创建（手测节第二发现）

- 发卡：GLM 主管决策流 / 2026-10-27（依据=手测节用户实测「无法进行任何手动编辑」+发卡侧代码亲核）
- 派发确认：已确认（手测节发现链）
- 验收负责人：GLM 主管决策流
- 池序 199；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**（领取基线 main=a81e0a5）；**P1**（手动创建链路死路）/ 0.25-0.5 天 / 依赖=无
- 模型分级：L1 / PC 执行会话
- 并行与资源：与 198（轨道头开关，Track_Row_Controller 域）文件域不相交可并行；**真栈腿与用户手测节错峰**（§2.2 增补③）。**注**：视窗归属（F1）形态讨论在飞——本卡只动 canvas 件交互逻辑，与归属接线方案（adapter/宿主域）不相交，不因讨论停工。

## 缺陷事实（发卡侧代码亲核）

`envelope_closeup_canvas.gd _on_left_pressed`：加点唯一路径=`_curve_state == CURVE_STATE_LOADED` 且按下命中折线（垂距≤6px，`_line_hit_insert_index_at_local`）；**CURVE_STATE_EMPTY 无折线→点击落到聚焦清理分支，零反应=空曲线无法创建第一个点**。194/195 探针全程探针侧先写点再测编辑，从零旅程未覆盖（端测边界声明内，手测节实证）。

## 目标

1. **空曲线首点创建**：CURVE_STATE_EMPTY 态单击画布曲线区=创建首点（按下即建即拖，与既有折线加点手势一致；值=点击 y 映射，clamp 描述符值域，无描述符按 assumed 标记既有形态）。
2. **写回契约**：首笔 set_points 的 region 构造与 C1 契约一致（从零到有的区域语义执行侧勘察定并在回执披露；不破坏既有 region/clear_region 幂等面）。
3. **空态呈现从简**：不新增文案面（可建点提示后补，非本卡义务）。
4. **探针**：空→单击建首点→拖动→释放恰一笔→回执读回一致；既有探针零回退（closeup 79/curve 55/lane 39/auto 41/badge 33/datachan 50）。

## 文件域

`vit_dock/views/envelope_closeup_canvas.gd`+**独立探针新文件**（194/195 既有探针文件**零改动**——回归以只读复跑验证；与 200 的 194 探针 R/O 段迁移面解除冲突，发卡侧 2026-10-27 补注）。**不触** envelope_dock_adapter/registry/dock_root/车道/app/rack。越域即停。

## 验收标准

独立探针全绿+既有探针零回退+真栈腿 exit 0（空→建点→内核 get_curve 读回一致；§2.2 登记+三纪律+错峰手测节）；工件入主仓 coord/runs/FE-ENVELOPE-FIRST-POINT-1/；端测边界声明。

## 停止条件

- 首点 region 语义与 C1 契约（automation_client.gd）/内核 set_points 区域校验冲突→锚点上交；
- 空态手势与既有 scrub 带/吸附开关命中面冲突超预期→上交。

- 领取：（2026-10-27 12:1x +0800 / origin/main=46400263c45a06c08d7a4ba66b78a566f2f79bd5 / ZCode GLM-5.3 PC 执行会话（ZCode，199 卡会话）/ 分支=port/fe-envelope-first-point-1（前端仓 worktree D:/Godot/project/vit-daw-frontend-firstpt1，领取基线 main=a81e0a5）+ 协调 worktree D:/Vit_DAW_wt_firstpt1_coord（coord/fe-envelope-first-point-1-claim）/ 领取提交=本提交。领取核对：卡在 todo+派发已确认（手测节发现链）+依赖=无；doing=GEN-SUNO-WEB-DESIGN-1（Mac 侧 coord/runs 域）+198 FE-ENVELOPE-LANE-TOGGLE-1（前端仓 track_scene 两文件域，与本卡 envelope_closeup_canvas.gd+独立探针新文件域不相交，卡面并行注记亲核）；真栈腿启栈前按 §2.2 另行登记并错峰用户手测节（资源记录最新释放 195@2026-10-26 23:10））
- 回执：（done 2026-10-27 13:0x +0800 / 实现=**前端仓 port/fe-envelope-first-point-1@3f52ad5 已推远端**（生产改动仅 envelope_closeup_canvas.gd 12+/1-：EMPTY 态单击建首点按下即建即拖=折线加点同手势机；值=点击 y 映射 clamp 描述符值域/assumed [0,1] 兜底；scrub 带/吸附开关命中面守卫不变；+新探针 3 文件，194/195 既有探针零改动 git diff 空）/ **首笔 set_points region 契约=零新语义**（`_edit_region` 既有无锚分支：start=0（无前锚）+end=max(t,total,view)+1.0 尾垫（无后锚）=「编辑首锚点」与「尾加点」两既有分支的组合；C1 校验面 start≥0/end>start/新点∈[start,end] 闭域全满足，无 malformed/越界面——RESULTS.md §一披露）/ 离线探针 probe_envelope_first_point **41/41 exit 0**（S 从零装配/G 守卫/F 主旅程恰一笔+回显读回/E 错误回滚重试/A assumed clamp）/ 既有探针只读复跑 **79/55/39/41/33/50 全 exit 0**（lane_ui 首轮败=worktree .godot 导入缓存缺 ctex 环境面，--import 后绿非代码回归）/ **真栈腿 exit 0**：run=coord/runs/FE-ENVELOPE-FIRST-POINT-1/envelope_first_point_20261027——ps1 最小腿 exit 0+新栈探针 **连续两轮 17/17 exit 0**（从零 EMPTY 前提双源（内核 get_curve 空+视窗披露）→按下即建→手势期零写（bus=0）→释放恰一笔（bus automation.set_points==1）→回显门 empty→loaded→**内核 get_curve 读回逐点一致**）；§2.2 登记 09453dfb–13:05 释放三纪律+188 守卫+错峰（200/198 释放亲核后才登记）在案；VIT_PROJECT_XML 隔离三面亲证；工件随 port/fe-envelope-first-point-1-stack 入库待验收合入 / **端测边界声明**：覆盖=真栈场景探针（生产传输链 UDP4445→agent→内核 ZMQ；_gui_input 生产手势机同构事件注入；内核读回对照）+离线假传输全链；不覆盖=真实像素级鼠标输入的 GUI 视觉面、视窗归属 F1 宿主接线形态（200 域）、用户主观体验（手测节面）；空态「可建点提示」文案未新增（卡面目标③从简））
- 验收：（待主管裁定）

# FE-AUTOLANE-EDIT-1：车道内编辑迁移——手势机/写回/回显门/曲率/undo（重设计链卡 3）

- 发卡：GLM 主管决策流 / 2026-10-27（依据=GUI 重设计裁定链「编辑回到车道，所见即所得」；**依赖=202 合入**）
- 派发确认：已确认（用户裁定链）
- 验收负责人：GLM 主管决策流
- 池序 203；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**；P1 / 1 天
- 模型分级：L2 / PC 执行会话
- 并行与资源：真栈腿错峰同 202。

## 目标（194/195 编辑逻辑**迁移**入 202 产出的车道画布；特写窗组件形态作废但本卡不拆窗——拆除归 204）

1. **车道编辑手势机**：拖点（Shift 锁 t / Ctrl 锁 v+实时数值标签）/折线加点（按下即建）/Delete 删点/删至唯一点=clear_region；车道画布内局部手势状态机（不接时间线全局仲裁器——194 领土隔离先例）。
2. **写回链迁移**：释放恰一笔 set_points region（c 透传）+回显门（回执比对重拉）+写回 error 回滚+拖拽态外部写挂起/释放合并——194/195 已证逻辑迁移，断言语义不弱化。
3. **曲率编辑**：段中点单柄（柄位=曲线采样值）+左键拖连续改 c+右键预设四档（线性/快入/快出/S 曲）+T/V 吸附开关（默认关）。
4. **撤销入口**：Ctrl+Z / Ctrl+Y 接内核 undo 事务（M3-KERNEL-1 已就绪面）；undo 后车道呈现与内核读回对齐断言。

## 文件域

202 产出的车道画布文件（app/tracks/track_scene/** 车道画布邻域）+探针新文件；不触 vit_dock/**（194/195 件本卡零改——其拆除归 204）。越域即停。

## 验收标准

独立探针全绿（编辑全链假传输：手势/写回恰一笔/回显门/回滚/曲率/预设/吸附/undo）+202 探针零回退+真栈腿两轮（车道编辑写点带 c→内核 get_curve 读回逐点一致→预设切换读回→undo 事务往返后车道与内核一致）+工件入 coord/runs/FE-AUTOLANE-EDIT-1/；端测边界声明。

## 停止条件

- 车道画布手势与时间线滚动/框选/clip 手势冲突超勘察预期→上交；
- undo 事务与前端回显门交互出现不一致（undo 后内核态与呈现漂移）→取证上交。
- 领取：2026-10-27 16:58 / origin/main=889a373c / owner=GLM-5.3 PC 执行会话（ZCode）@Windows PC / 前端分支 port/fe-autolane-edit-1 / worktree D:\Godot\project\vit-daw-frontend-edit1（基线 f5b7329）/ 协调分支 coord/edit1-claim
- 回执：（**执行侧自验完成 2026-10-27 17:50 前后，交主管验收**。实现=前端仓 `port/fe-autolane-edit-1` @ **c77d816**（已推远端；deee903 实现+94386bb 栈探针+单点延伸线命中分支+c77d816 真栈探针 R1 缺陷修复（探针侧））；工件=主仓 `port/fe-autolane-edit-1-stack`（run=**coord/runs/FE-AUTOLANE-EDIT-1/autolane_edit_20261027_1730/**，RESULTS.md 回执）。
  **离线探针全绿 11 件 536 检 exit 0**：新 probe_autolane_edit **63/63**（G 手势机 11 检：拖点 Shift 锁 t/Ctrl 锁 v（按下快照原值）/折线加点按下即建/零位移零写回/Delete 区域重建/删至唯一点 clear_region/单点延伸线加点/W 写回链：恰一笔 region 前后锚+ε/c 透传/回显门一致与不一致披露+对齐内核/error+队列失败+超时回滚 baseline/拖拽态外部回执挂起+零位移释放合并/K 曲率：柄位=采样值参考实现一致/连续 c 映射/预设四档含 S 曲符号与幂等/S 吸附：默认关+开关位翻转+时间步网格+值步 1/8/U 撤销：Ctrl+Z/Y 入队 rid=env_lane_undo/ok 重拉对齐+●N 刷新/error 逐字披露/拖拽态与收起态不劫持）+202 探针零回退 10 件 **473 检逐件同数全绿**（autolane_rebuild 73/lane_ui 36/lane_auto 25/lane_toggle 27/closeup 81/curve 55/first_point 41/detail_home 52/badge 33/datachan 50——closeup 族四件打 vit_dock 面=本卡零改文件天然不回退）。
  **真栈腿两轮全绿**：§2.2 登记 799fe53a（17:41:50 全零核对+占用空 fetch 亲核+错峰）→ps1 最小腿 exit 0（188 守卫+PREFLIGHT_CLEAN+VIT_PROJECT_XML 隔离）→stack_probe_autolane_edit **30/30×2 连续两轮 exit 0**（R1 17:45:10/R2 17:45:22）：E1 装配+默认音量车道生产链落定→E2 建首点恰一笔+回显门+**内核 get_curve 读回逐点一致**→E3 单点延伸线加点→E4 拖点→E5 **曲率柄 c=-0.5 内核读回一致**→E6 **预设三档切换读回（fast_in/smooth_s/linear）**→E7 删点→E8 **undo/redo 事务往返（Ctrl+Z/Y 生产按键链→内核 undo/redo 事务→车道重拉对齐+内核读回=编辑前/后态）**。R1 首试 6/8=探针缺陷（未注目录致轨级描述符请求不发车→绑定挂起；探针侧修复 set_plugin_catalog([]) 空目录注入=202 同姿势，零生产改动，缺陷轮留证）。隔离三面亲证（run 副本 287956e7→7ca02e9c 内核写入/stack worktree git 干净/主检出 mtime 14:01 零触碰）；拆栈 17:45:47 复核全零；释放记录随本批推送。
  **新增面披露**：①车道单点延伸线命中分支（202 渲染面画水平延伸线，命中面随绘制=所见即所得；194 特写窗无此绘制故无此分支；离线 G11+真栈 E3 双面覆盖）——车道侧补齐非弱化；②delete/预设在写回 pending 期加防呆守卫（194 无此守卫；防 echo 簿记串扰）。vit_dock/** 零改实证（diff 仅 track_envelope_lane_2d.gd+tools/ 探针 4 件）。
  **端测边界声明**：渲染面（拖拽标签/曲率柄/吸附开关观感）未过浏览器级 DOM 断言（E2E-WEBUI-1 未就位）；手感面（拖点/锁轴/删点/undo 快捷键）留用户手测（裁定链：实施后用户手测节点终验）；探针断言到内部状态+真内核读回一致，按键链经生产 _gui_input 事件注入（headless 无真实键盘焦点路径）。停止条件两支均未触发（条内局部手势机零仲裁器接触=194 领土隔离先例；undo×回显门交互 E8 实证对齐无漂移）。）
- 验收：（待主管裁定）

# FE-ENVELOPE-LANE-TOGGLE-1：包络车道轨道头三态开关——GUI 入口缺位补齐

- 发卡：GLM 主管决策流 / 2026-10-27（依据=2026-10-27 手测节用户实测发现「GUI 无任何包络内容」+用户 2025-10-25 形态裁定「手动开关在轨道头，首版三态『自动/开/关』默认自动」）
- 派发确认：已确认（用户 2026-10-25 形态裁定链+2026-10-27 手测节发现；控件微观形态主管裁=MenuButton 下拉三选，用户可改口修订卡）
- 验收负责人：GLM 主管决策流
- 池序 198；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**（领取基线 main=a81e0a5）；**P1**（手测节主线阻塞项）/ 0.5 天 / 依赖=无（191/192/194/195 已合入 ✓）
- 模型分级：L1 / PC 执行会话
- 并行与资源：**真栈腿与用户手测节错峰**（§2.2 增补③——手测占用主检出栈期间不启栈，领取/登记前 fetch 重读 PC-RUNTIME-STACK.md 互查）；与其他在飞卡文件域不相交。

## 缺陷事实（发卡侧亲核，2026-10-27 手测节首发现）

1. **轨道头三态开关从未建成**：`set_envelope_lane_open/set_envelope_lane_mode` 全前端调用方仅探针（tools/probe_envelope_*.gd，grep 亲核）；轨道头（Track_Row_Controller.gd）、轨道右键菜单（timeline_dock_adapter.gd _ensure_track_context_menu 仅 4 项：New folder/新建轨道/删除/New group）、任何 UI 面零接线。
2. **默认档=off + 无控件=车道在真实 GUI 永不可达**：189 一段验收采信「两态简化、默认关」（两段计划内偏离）；192 维持「默认档=现两态行为，自动档需显式选」。两卡验收均走探针直调 API，装配面缺口未现——194/195 回执已声明「真实点击旅程未自动化」边界，本卡收此边界第一洞。
3. 已就位面（本卡零重复实现）：三态状态机+在场性扫描+粘性（192）、车道头 ⤢/关闭/参数选择器（189/194）、空槽启动器 Envelope 按钮可达特写视窗（194，registry spawnable 亲核）。

## 目标

1. **轨道头三态开关控件**：MenuButton 下拉三选——「自动 / 开 / 关」（对应 auto/manual/off；标签中文按 GUI 现行惯例，执行侧对齐相邻控件文案）——挂载于轨道头控件邻域（Track_Row_Controller.gd；具体槽位执行侧勘察轨道头现有按钮族定，原则=不挤占 FE-GUI shrink 成果、不破坏既有布局）；选中态与 `get/set_envelope_lane_mode` 双向同步；轨道头空间不足的布局形态（如图标化/tooltip）执行侧勘察定并回执披露。
2. **默认档回归裁定=auto**：`_lane_mode` 缺省由 off 改 auto（2026-10-25 用户裁定「首版三态默认自动」；189 两段式偏离随本卡收回——手测实证默认关+无控件=功能不可达）。旧工程/旧状态零迁移（无持久化面，缺省即档）。
3. **探针**：独立探针断言——控件在场性（行装配后轨道头含三态控件）/默认档=auto/三选切换与 lane 可见性联动/双向同步（mode 外部变更后控件选中态跟随）/off 行为回退；既有探针零回退（lane_ui 39/lane_auto 41/closeup 79/curve 55/badge 33/datachan 50）。
4. **真栈腿**（§2.2 登记+三纪律+188 守卫，错峰手测节）：装载 VST→默认 auto 车道收起→set_points 写点→车道自动展开（192 腿复用形态）→手动切「关」粘性→clear_region。

## 文件域

`app/tracks/track_scene/Track_Row_Controller.gd`（轨道头控件邻域）+`app/tracks/track_scene/track_envelope_lane_2d.gd`（默认档+控件回调）+探针新文件。**不触** `app/rack/**`、`vit_dock/**`（⤢ 入口已在车道头就位）、`vit_track_lane_2d.gd`。越域即停。

## 验收标准

独立探针全绿+既有探针零回退+真栈腿 exit 0（工件入主仓 coord/runs/FE-ENVELOPE-LANE-TOGGLE-1/）+**装配面断言**（控件在真实行装配树在场，非仅 API）+端测边界声明（渲染像素/手感仍留手测面）。

## 停止条件

- 轨道头控件挂载点与 FE-GUI 系列结构漂移（锚点不符）→锚点上交；
- 默认 auto 在多轨工程引发扫描性能问题（卡顿/超帧）→取证上交，默认档可退 manual 待用户裁定；
- 三态控件与轨道头既有让路链/命中路由冲突超预期→上交。

- 领取：（2026-10-27 12:12 +0800 / origin/main=2fa99bd34908b06cc4eb53ca94f521d70ba5782d / ZCode GLM-5.3 PC 执行会话（ZCode，198 卡会话）/ 分支=port/fe-envelope-lane-toggle-1（前端仓 worktree D:/Godot/project/vit-daw-frontend-lanetoggle1，领取基线 main=a81e0a5）+ 协调 worktree D:/Vit_DAW_wt_lanetoggle1_coord（coord/lanetoggle1-claim）/ 领取提交=本提交。领取核对：卡在 todo+派发已确认+依赖 191/192/194/195 已合入；doing 仅 GEN-SUNO-WEB-DESIGN-1（域=coord/runs/GEN-SUNO-WEB-DESIGN-1/** Mac 侧零生产代码，与本卡不相交）；真栈空闲（最新释放 195@2026-10-26 23:10；本腿启栈前按 §2.2 另行登记并错峰用户手测节））
- 回执：（**执行侧自验完成 2026-10-27 13:0x**）实现=**port/fe-envelope-lane-toggle-1@09503b2**（已推远端；基线 a81e0a5）；真栈腿=**coord/runs/FE-ENVELOPE-LANE-TOGGLE-1/envelope_lane_toggle_20261027**（工件随 **port/fe-envelope-lane-toggle-1-stack** 分支入库；RESULTS.md）。交付面：①轨道头 MenuButton 三态「自动/开/关」挂 MSR 行电平 label 前（40x22，master/folder 隐藏，不挤占 shrink 成果）；②默认档 `_lane_mode` off→auto（2025-10-25 裁定；_ready 兜底首扫描+bind_virtual_track 包络车道 id 跟随——虚拟化行池重绑缺陷最小修，同文件域）；③双向同步=envelope_lane_mode_changed 信号面（实际迁移即发/两态重分类同发/粘性与同档重入不发）。验收面：新探针 probe_envelope_lane_toggle **40/40**+既有六件零回退（lane_auto 41/lane_ui 39/closeup 79/curve 55/badge 33/datachan 50 全绿）+真栈 ps1 exit 0+stack probe **连续两轮 25/25**（S1 默认 auto 零显式 set+装配面控件在场+L2 写点自动展开内核对照+L3 菜单关粘性+L4 clear 回零）。**披露四项交主管裁定（RESULTS.md §五）**：ⓐ lane_auto 探针 A1/A2 期望 off→auto 更新（与卡目标 2 结构互斥的必然更新，断言面不变 41 检——**越出卡面「探针新文件」字面域**）；ⓑ bind_virtual_track 补丁（默认 auto 在生产 GUI 扫错轨的缺陷面最小修）；ⓒ 进程误杀事故自报（12:35:30 拆探针泄漏时误杀一并行执行流 -s 进程 12836，命令行核对非本会话；-s 只读零持久损害，对方自行重跑恢复；教训=拆进程逐 pid 命令行亲核）；ⓓ 默认 auto 多轨扫描压力留手测节观察（停止条件②未触发）。**端测边界声明**：本腿覆盖装配面/档位语义/生产传输链（UDP 4445→agent→内核全链）；未覆盖=渲染像素观感、下拉真实鼠标点击旅程（headless 经 handler 面驱动）、多轨 GUI 布局真机观感——留手测面。
- 验收：（待主管裁定）

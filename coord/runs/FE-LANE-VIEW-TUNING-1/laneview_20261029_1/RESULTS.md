# FE-LANE-VIEW-TUNING-1（214）真栈 run 工件：laneview_20261029_1

- owner=ZCode GLM-5.3 PC 执行会话（214 卡会话）；栈登记 828141cb（10:00）/释放（本文件同批提交）
- kernel=主检出重建二进制只读复用（2026-10-28 21:26 主管重建 III-A 面，sha256 c1e32dfe…，209/211/212 同源）；agent=D:/Vit_DAW_wt_214stack 构建（Go 零改动 exit 0）pid 22908；vsphub=主仓 agent/bin 只读复用本腿手启 pid 37152（启前亲核 8787 空）；kernel pid 22476 ps1 -StartKernel 自启
- VIT_PROJECT_XML=run 目录隔离副本 project_iso.xml（287956e7 基线=202-212 腿同基线 blob cat-file 亲核；运行后 248fe2bf=内核写入被副本承接=隔离生效；stack worktree VitApp/+agent/ 运行后 git 干净；主检出 default_project.xml mtime 00:33=本腿前遗留零触碰）
- 前端被测 worktree=D:/Godot/project/vit-daw-frontend-lanetune1@port/fe-lane-view-tuning-1@34ca579（基线 37dc4b6+本卡实现）
- 基线 DIAG worktree=D:/Godot/project/vit-daw-frontend-laneviewdiag@37dc4b6 detached+探针拷贝（208 先例形态）

## B 垂直错位定案（窗口模式渲染测量，基线 DIAG=diag_baseline_probe.log）

1. **候选①（strip 实际渲染高度与映射不一致）排除**：P0i strip 真实 rect=(315,332,2237×96)、effective=96（baseline const）——渲染高度==映射基线，逐像素级一致（修复分支 P2d 渲染位 vs 期望位 0.1px）。
2. **候选②（ScrollContainer 裁剪）坐实微型形态**：DIAG P0z scroll viewport=118 vs stack=120——＋条按钮 themed 最小高 24（font_size 11 行高+内衬，离线 diag_bar 亲测 BAR min=(140,24)）>lane_area 公式常量 _ADD_BAR_HEIGHT=22 → stack 恒溢出 2px：**滚动条常驻+条底 2px 裁剪**（N=1 即现）。修复=常量校准 24（单行；修复后 P0k「N=1 无溢出滚动」PASS）。
3. **候选③（值域显示映射观感）坐实为主症状源**：真栈 value_scale={min:0.0,max:1.0,assumed:false}（轨级 volume 描述符=Tracktion fader 域）而 value_text="+0.00 dB"@0.7408（201 工件亲证）——0dB 位于条高 74%，可用值域带=[35,89] 仅 54px（车头 26+垫 14=42% 为非值域区）→「上提一半到顶/下拉一半消失/看不到底」观感=可用值域带窄+死区占比高+fader 线性域与 dB 听感错位。**A 的 128 重设=主修**（净高 54→86px）；dB 域显示映射（0dB 参考线/刻度）为后续观感面未在本卡实施。
4. **映射一致性断言（卡面必入）**：P2c 管线建点 v==y→value 往返 err=0.0000；P2d 新点渲染位 vs 释放位 0.1px（±5px 容差内）；P1b 外部写 0/50/100% 三值渲染像素位 vs _content_rect 映射期望位全 PASS。

## C 拖柄定案（回执披露面）

- 形态=车头区右端 `Btn_Envelope_Lane_Height`（Button "⇕"，22×22，CURSOR_VSIZE，tooltip「拖拽调整车条高度（会话内生效，不持久化）」）。**BaseButton 形态=overlay 网关 interactive 让路面天然认领**（211 勘察：网关只认 BaseButton/Range/文本类+名单特例——非 Button 需改 timeline_dock_adapter=域外，Button 免=P3a 网关让路 PASS 实证）。
- 交互=竖直拖：下拉增高/上拉减低（motion 用 global y；拖柄锚车头顶部调高过程不漂）；多条统一（容器内全部车条同步，P3d 双条同高 PASS）；会话内不持久化（202 即态：行池回收/重开容器回 env/缺省，离线 H7 新容器回缺省 PASS）；clamp[48,256]。
- budget 联动=wanted=24+N×实际条高（P3e lane_area=min(wanted,budget) PASS；P3f wanted>budget 时内部滚动 scroll<stack PASS；拖柄交互零内核写 P3g PASS）。

## 验证矩阵

| 面 | 结果 | 工件 |
|---|---|---|
| 基线 DIAG 轮（96，取证不判死） | P0 装配全绿+三候选取证在卷 | diag_baseline_probe.log |
| 探针迭代轮 r1-r4 | 全探针面非生产面（r1 采样位/空集守卫、r2 零位移释放=203 语义、r3 段中点=曲率柄位[211 P5 同款]、r4 内核读竞态轮询） | iter_r1-r4_probe.log |
| **正式轮×2（窗口模式全腿）** | **31/31 PASS×2 连续 exit 0** | formal_r1/r2_probe.log |
| env 覆盖轮（VIT_GUI_LANE_STRIP_HEIGHT=112） | 31/31 exit 0（P0i strip=112 source=env 实锚） | env112_r1_probe.log |
| 真栈族既有 3 件 | edit 30/30+rebuild 23/23+layout_grow 16/16 全绿零回退（layout_grow 含 128/24 新期望） | （stdout 随会话记录） |
| 离线全家（终态） | 26 -s 件全绿+file_reply 场景件 7/7（四件初判红=输出格式误判：lane_catalog 18/18+vsp_delta 30/30[SUMMARY_JSON 格式]+vitnote_notestream 26/26[checks 格式]+vitnote_input=取证件非 PASS/FAIL 族；survey×2=209 勘察件挂起无看门狗，先行 kill 不计） | offline_family_rerun（随会话记录） |

## 旅程面断言明细（正式轮 P0-P4）

P0 §5.1 装配（adapter/TrackList/仲裁器/mapping/行/容器/strip 实件+strip 渲染高==有效高+拖柄在场+N=1 无溢出滚动）／P1 B 渲染测量（外部写 0/50/100% 三点经生产 _request_curve 落定+逐点渲染像素位 vs 映射期望±5px+content rect 全局面实测）／P2 映射一致性（折线段上管线按压建点：往返 err=0.0000+渲染位 0.1px+内核 4 点一致[轮询兜竞态]）／P3 C 拖柄（网关让路+管线 press→motion+36→release：会话高 128→164 source=session+双条统一+budget clamp 316+内部滚动+零内核写）／P4 211 回归面（新高度下网关让路+画布建点 5 点内核一致）。

## 端测覆盖边界声明

- 探针窗口模式=真实 GUI 派发管线+真实渲染像素采样（与用户手测同构），但非用户键鼠；栈为探针最小腿（-StartKernel -RestartAgent，无 UI 全旅程）。
- **[等待用户:手测观感终值验收]**——用户从 Godot 前端拉起进入工程（同 shell 先设 VIT_GUI_LANE_STRIP_HEIGHT=112/128/144 试值），验收：车条高度观感/垂直映射观感/车头区 ⇕ 拖柄调高（多条统一+重开回默认=202 语义）。

## 上交主管知悉

1. **B 主症状=候选③（fader 线性域观感）非机械错位**：渲染/映射/交互三面像素级一致已证；A 128 为主修，dB 域显示增强（0dB 参考线/刻度网格）为独立观感小卡候选，是否开卡由主管裁定。
2. **＋条常量 22→24 校准**属 lane_area 公式面修复（候选②坐实形态，域内单行）；若未来主题/字号变更再漂移，探针 P0k 会拦。
3. 容器对他方 automation 写回执只刷 ●N 不重拉曲线=既有行为（192 触发链设计），探针显式触发 _request_curve 测量；是否补「外部写自动重拉」为独立行为面由主管裁定。
4. lane 族探针期望值更新五件（layout_grow 离线/stack、rebuild、envelope_lane_ui、本卡新探针）=授权面（198 先例）；N=3 语义随 128 变预算 clamp 形态（206「N=3 全展开」不再成立——22+3×128=408>316）。

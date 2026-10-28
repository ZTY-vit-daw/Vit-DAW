# FE-AUTOLANE-LAYOUT-GROW-1（206）真栈腿 RESULTS

- 日期：2026-10-28；owner=ZCode GLM-5.3 PC 执行会话（206 卡会话，扩域裁定后续作）
- 被测：前端 `D:/Godot/project/vit-daw-frontend-lgrow1@port/fe-autolane-layout-grow-1`：cf25ccc（实现）+a8a22a0（bind 重绑自愈+stack 探针终态）
- kernel：stack worktree `D:/Vit_DAW_wt_lgrow1_stack` 自建 VitApp.exe（源=origin/main@535c98b4，VitApp/ 零改动亲核——206 全部改动在前端仓；CMake configure+build exit 0，sha256 前缀 c9a258de；tracktion/libzmq/cppzmq 源=主检出只读复用零网络）
- agent：stack worktree 构建（Go 零改动）；kernel pid 10660 / agent pid 24036（本腿 ps1 -StartKernel/-RestartAgent 自启）

## 一、最终验收轮（连续两轮 exit 0）

| 探针 | R1（第 1 轮全绿） | R2（第 2 轮全绿） | 日志 |
|---|---|---|---|
| stack_probe_autolane_layout_grow | **16/16 exit 0**（R4） | **16/16 exit 0**（R5） | console/probe_layout_r4.log、probe_layout_r5.log |
| stack_probe_autolane_edit（编辑链新几何回归） | **30/30 exit 0** | **30/30 exit 0** | console/probe_edit_r1.log、probe_edit_r2.log |

layout_grow 断言面（V 段虚拟池=扩域裁定案一消费面；env `VIT_GUI_VIRTUAL_TRACK_LIST_THRESHOLD=2` 降档构造=产品现役机制）：V0 env 机制生效→V1 虚拟池激活（真内核轨数据 records）→V2 池行装配→**V3 行 A 轨道头开关展开：默认音量车道经生产链落定（真 repo+真 IPC 描述符）→仅行 A 高 358（240+118），B/C 基线 240（按行覆盖生效）**→L1 真栈行内布局（车道容器=行子节点+车道区在波形下方非叠加）→V4 行 B 独立覆盖（454）→V5 既有全局缩放零破坏（行 C vit_set 300=全局变，A/B 覆盖保持）→V6 收起=覆盖清除回全局 300→V7 记录移除+同 id 重放=行高与 lane 保留态一致（bind 自愈重发）→V8 默认阈值语义零破坏（env 清空 3<24 不虚拟化）。

autolane_edit=203 编辑链旅程在新几何下回归：建点/拖点/曲率柄写 c/预设/Delete/undo-redo 事务往返+内核 get_curve 读回逐点一致（探针零改动复跑=命中/手势零回退实证）。

## 二、迭代轮留证（红轮如实记录，探针侧修正）

- probe_layout_r1.log：records 全量（14 轨）≠3——隔离工程副本非空（clear_project 不清轨面，203 先例不断言轨数）；探针修=按本腿三轨过滤。
- probe_layout_r1b.log：segfault@`_ensure_timeline_edit_stack`——router store 解析链缺环境无限 deferred；探针修=预置 TimelineEditStore 子节点（生产类，205 repo 同款环境补全族）。
- probe_layout_r1c.log：V2 行不可达——行装配 deferred mutation queue；探针修=条件等待。
- probe_layout_r1d.log：V3a 默认车道 0 条——headless 无 project_repository autoload；探针修=205 同款真 repo 生产写入（apply_kernel_project_state 内核真实回执）。
- probe_layout_r1e.log：15/16（V7 红期望 300 实测 454）。
- probe_layout_r2/r3.log：V7 语义修正两轮（r2=want 基面错 300 行 C；r3=正确 454 但 lambda 值捕获行引用不回写——探针修=等待后重取）。
- **生产补丁一轮（域内）**：bind_virtual_track 尾自愈重发——行池热复用（released_rows 同 id 快速复用）lane 态保留与 router 随记清除的一致性（V7 实证：454=lane 开态一致）；真删轨路径（行 recycle 绑新轨）=D10 段收起面。补丁后离线核心六件复跑全绿（rebuild 75/edit 63/prodchain 65/layout 11/toggle 27/ui 36）。

## 三、离线全家回归（offline_regression/，24 件全绿 exit 0）

probe_autolane_layout_grow（新，D 段直接行形态 11 断言：行高增长/回落/装配位置/spacer x 对齐/N=4 超高内部滚动/U 基面派生/重绑回收起）+rebuild 75（S 段更新为 b+a 形态+4 新断言）+edit 63+prodchain 65+lane_toggle 27+lane_auto 25+lane_ui 36（A2/A10/A11 更新）+teardown 50+datachan 50+shrink a/b/c+fx_wet_chain_badge 36+rack 四件（111/35/15/313）+start_feed+vitnote 六件。`*_post.log`=自愈补丁后复跑（六件核心全绿）。

## 四、§2.2 三纪律与隔离亲证

- 登记 1f66f40a 推送 12:55:50→12:56:03 即时复核（远端亲读置顶+进程/端口全零，13s<60s）→driver PREFLIGHT_CLEAN 12:56:11 二次预检→启动；无 already listening；登记先于启动；释放 d2d8b056（13:2x 拆栈后复核全零）。
- VIT_PROJECT_XML 隔离：run 副本基线 287956e7（hash-object 亲核=202/203/205 首腿同基线）→运行后 c0b6072f（内核写入=隔离生效）；stack worktree VitApp/Workspace/default_project.xml 仍 287956e7（本体零污染）；主检出 78b828ee（本腿前既有值零触碰）。

## 五、披露（交主管验收核对）

1. **仲裁器 envelope 面几何边界**：车道区移出 wrapper 后，TimelineViewMapping 的行接受判定（wrapper rect，域外文件）不再覆盖车道区——车道体点击本就走 strip `_gui_input` 先收（203 领土隔离，编辑手势零影响）；受影响面=车道头部按钮间隙点击的分派语义（原=wrapper 内波形语义，现=行外空白语义，微小差异，mapping 域外不在扩域五面故未改）。
2. **N>3 滚动条占宽**：ScrollContainer 垂直滚条出现时占右侧 ~12px，条数≤3（无溢出）零影响；溢出滚动时 x 映射画布略窄于波形区（溢出场景优先可用性，从简裁定面内）。
3. **V7 语义细化**（裁定「随记清除」执行面）：同 kernel id 移除+重放=工程重放面（生产删轨再加回=新 id）——router 差分清除+bind 自愈重发=行高与 lane 保留态一致；真删轨（行 recycle 绑新轨）=收起清覆盖（D10）。行池热复用与随记清除的一致性由自愈补丁保证。
4. **既有探针期望更新四处**（198 先例：形态裁定授权）：autolane_rebuild S 段（overlay→行内流+行高断言）、envelope_lane_ui（A2 路径+A10/A11）、autolane_edit（独立挂载显式宽——容器不再 anchors 跟随父宽）、autolane_prodchain（容器释放路径×3 wrapper→行内）。
5. **端测边界声明**：覆盖=装配面/行高联动语义/虚拟池按行覆盖/生产链目录（真 repo+真 IPC）/编辑链全旅程；未覆盖=渲染像素观感、真实鼠标点击旅程（headless 经 handler/事件构造驱动）、多轨大工程真机滚动观感——留手测面。

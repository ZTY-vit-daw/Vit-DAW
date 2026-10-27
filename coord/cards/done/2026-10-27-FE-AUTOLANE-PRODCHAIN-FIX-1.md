# FE-AUTOLANE-PRODCHAIN-FIX-1：车道生产链修复——轨道 id 懒创建绑定+弹层屏幕坐标（手测节发现）

- 发卡：GLM 主管决策流 / 2026-10-27 晚（依据=用户手测节实证三缺陷中两项+发卡侧代码亲核根因闭环）
- 派发确认：已确认（手测节发现链）
- 验收负责人：GLM 主管决策流
- 池序 205；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**（领取基线 main=f52b78f）；**P1** / 0.5 天 / 依赖=无
- 模型分级：L1 / PC 执行会话
- 并行与资源：真栈腿与用户手测节错峰（§2.2 增补③——用户现场可能仍在跑）。

## 缺陷事实（发卡侧代码亲核，2026-10-27 手测节实证）

1. **轨道 id 懒创建断链**：`get_track_envelope_lane()` 懒创建走 `_ensure_envelope_lane_2d_layer(false)`——创建时不同步轨道 id；行装配期绑定先于容器存在、`bind_virtual_track` 跟随（:2650/:2677，198 补丁在位）仅在容器存在后的重绑生效→生产 GUI 容器恒空 id→目录查询全空→选择器只剩发送占位组（用户实证：无音量/声相/插件组）。
2. **弹层坐标错置**：`_on_selector_pressed` 以 `_selector_button.global_position`（canvas 坐标）直接赋 PopupMenu（Window）`position`（屏幕坐标）→弹层出现在窗口顶部而非点击处（用户实证）。
3. **探针注入盲区（验收判定面缺口，主管记档）**：202/204 真栈腿目录经 `set_plugin_catalog`/`set_track_volume_plugin_id` 注入（stack_probe :353-354），生产 repo 目录链零覆盖——「E2 三组收口」非生产链证据。

## 目标

1. **id 绑定修复**：懒创建即推当前轨道 id（`_ensure_envelope_lane_2d_layer(true)` 或创建路径等效推 id）——覆盖「行已绑→后创建」时序；id 到达且展开态时目录刷新（既有 set_kernel_track_id 行为保持）。
2. **弹层定位修复**：选择器/曲率预设弹层改屏幕坐标映射（canvas→screen；对齐仓库既有弹层定位先例——轨道右键菜单/timeline_dock_adapter popup 模式）；headless 守卫保持。
3. **探针生产链断言（本卡核心义务）**：离线——真行装配（track_row.tscn）+真 repo 形态（带 rack_nodes 快照的 project_repository 或等价真路径）断言：开关展开→容器 id=行当前 id→**目录三组从生产源到达**（轨道组恰 volume/pan+插件组实载）——**禁 set_plugin_catalog/set_track_volume_plugin_id 注入**；既有注入态探针标注「注入边界」不删（它们测编辑/写回面仍有价值）。真栈腿同理禁注入，目录走真实 repo 链。
4. **回归**：全家探针零回退（202/203 面）。

## 文件域

`app/tracks/track_scene/Track_Row_Controller.gd`（懒创建 id 推送）+`app/tracks/track_scene/track_envelope_lane_2d.gd`（弹层坐标）+探针（新断言+注入边界标注）。不触 vit_dock/**、app/rack/**。越域即停。

## 验收标准

独立探针全绿（含生产链目录断言）+真栈两轮（**无注入**：装载 VST→展开→选择器三组在场→默认音量车道绑定→建点→内核读回一致）+弹层定位断言（屏幕坐标映射单测或等价断言面）+工件入 coord/runs/FE-AUTOLANE-PRODCHAIN-FIX-1/；端测边界声明。

## 停止条件

- 真 repo 快照形态与探针假设不符（rack_nodes 键型/结构漂移）→锚点上交；
- 弹层坐标映射在多 viewport/缩放场景仍有偏移（Godot 嵌入 viewport 变换复杂度超预期）→取证上交两案。

## 领取

- 2026-10-27 19:22 / origin/main=8791405c / owner=GLM-5.3 PC 执行会话（ZCode，本卡会话，PC 机）/ 前端仓分支 port/fe-autolane-prodchain-fix-1@基线 main=f52b78f / worktree D:/Godot/project/vit-daw-frontend-autofix1 / 主仓协调分支 coord/fe-autolane-prodchain-fix-1-claim（worktree D:/Vit_DAW_wt_autofix1_coord）/ 领取提交=397176ea
- **续作领取**（2026-10-27 21:36 / origin/main=9b7885ff=主管裁决提交 / 同 owner 同会话同 worktree 续用，分支 port/fe-autolane-prodchain-fix-1 @ 8295ca4 之上续提交 / 范围=扩域面+重放：project_repository.gd 单点并源修复+两消费面红绿+真栈 R1b 重放全绿两轮；已绿交付面①②③不重做）

## 回执

- **状态：blocked——停止条件①命中，锚点上交主管裁决（2026-10-27 21:2x）**
- 实现：前端仓 `port/fe-autolane-prodchain-fix-1` @ **8295ca4**（已推远端；ed64f0f 实现+8295ca4 探针加固/诊断件）。①懒创建即推轨道 id（`get_track_envelope_lane()` 慢路径 false→true）+②弹层 canvas→屏幕坐标（选择器/曲率预设两处 embed 两态取实时鼠标，对齐 timeline_dock_adapter 先例；headless 守卫保持）+add_lane 轨道优先分组（生产链探针发现的注入盲区分歧）+get_kernel_track_id 读回面。
- 离线：新探针 `probe_autolane_prodchain` **47/47**（真行装配+真 ProjectRepository 生产写入路径+禁注入三组+懒创建时序+弹层定位断言）；**基线负对照红绿证据**（f52b78f 懒重建容器 id=空 vs 修复=行当前 id）；全家回归 **23 件全绿零回退**（rebuild 73/edit 63 与 203 收据同数）；注入边界标注 8 件（不删）。
- 真栈：§2.2 登记 76619335（20:49）+12s 即时复核+三纪律+VIT_PROJECT_XML 隔离亲证（287956e7→072ac158）；ps1 最小腿 exit 0；`stack_probe_autolane_prodchain` **R1b 17/20（核心面全绿）**：A5 懒创建即推 id **修复面真栈实证**（修复前恒空=用户手测根因）+A2 生产绑定 registry 落册+**S2 目录从真 repo 无注入到达**；3 红=同一根因（下述阻塞）。
- **停止条件①命中（锚点上交）**：racked 轨（rack_add_node 装载 VST 后）内核轨行 rack 字典**独占** `rack_nodes` 快照→轨级 Volume & Pan 插件行被排除→`get_volume_pan_plugin_item_id_for_track` 恒空（plugins 兜底=死路：tracks_out 无 plugins 键）；消费面①车道轨道组+默认音量车道（用户「无音量/声相组」第二层根因，与缺陷①复合）②轨道头声相推子 IPC 静默 no-op。**修复点 `vit_dock/core/project_repository.gd`（apply_kernel_project_state rack 分支+get_volume_pan_plugin_item_id_for_track 兜底）=卡面明令不触域**→按 AGENTS §11 锚点上交；最小复现件 `tools/stack_diag_205_rackshape.gd/.tscn`（真栈 30 行留证）。真栈验收「三组在场+默认音量车道→建点→读回」链在 racked 轨被阻塞，两轮全绿需主管裁决（扩域单点修复或另开卡）后重放探针（R1b 形态就绪）。
- 工件：run=`coord/runs/FE-AUTOLANE-PRODCHAIN-FIX-1/prodchain_20261027_2050/`（**分支 port/fe-autolane-prodchain-fix-1-stack @ 326dcd9e**，RESULTS.md+R1/R1b/diag 日志+离线回归 26 件+隔离副本）；栈登记 76619335–释放 d7c8d2d8（21:21 拆栈全零）。
- 端测边界：弹层真窗口落点未过自动化断言（headless 无窗口面；policy 单测+鼠标源集成断言=卡面等价断言面），留用户手测节；渲染面/手感面同留手测。

## 验收

（待主管验收：续作扩域面+重放已全绿收口，见下方续作回执）

## 续作回执（扩域面+重放，2026-10-27 22:0x 收口）

- **状态：done（执行侧自验完成）——扩域修复+两消费面红绿+真栈 R1b 重放连续两轮全绿；交主管验收**
- **扩域修复（d611b21，前端仓 port/fe-autolane-prodchain-fix-1 已推远端）**：`project_repository.gd` 并源=**解析器兜底+纯增键**（裁决两案取最小面）——`apply_kernel_project_state` 轨行 plugins 全量留存新快照键 `plugins_by_track`（既有键零破坏）+`get_volume_pan_plugin_item_id_for_track` rack 候选后追加该键候选（同一过滤；legacy 路径保持）。**既有消费方零破坏亲证**：rack 宏测试 PASS（rack 投影断言面）+离线 K1（racked 形态 rack_nodes 仍=rack.nodes 恰 VST）。
- **两消费面红绿（离线 K 段）**：修复前红=K2 并源解析空/K3 无新键/K6 车道轨级插件 id 空/K7 无默认音量车道（`prodchain_k_red.log` exit 1）；修复后绿=`probe_autolane_prodchain` **65/65**（+K14-K16 声相门红绿：空解析=零派发 no-op 形态 vs 解析达=set_plugin_param{value=0.625} 派发）。
- **全家离线回归 23 件零回退**（rebuild 73/edit 63 与 203 收据同数；verdict 形态三件与首腿逐字节一致或仅运行时实例 id 差异）。
- **真栈 R1b 重放（主管裁决验收面）**：§2.2 登记 c13a7f78（21:55）+17s 即时复核+三纪律+188 守卫+VIT_PROJECT_XML 隔离亲证（287956e7→c6105261）；kernel=**205 首腿自建 8440636F 只读复用**（扩域零内核改动亲核）；**R1 34/34 + R2 34/34 连续两轮 exit 0（零修正轮）**——racked 轨三组在场（轨道恰 volume/pan+发送占位+插件组实载 VST 实参）+默认音量车道生产链落定（真实 bus 描述符回执）+建点恰一笔+内核 get_curve 读回逐点一致+**W5-W6 声相推子 IPC 写读对照**（生产路径 `_header_send_pan_ipc`→并源解析→`set_plugin_param`→内核 pan 行 normalized_value=0.625 读回一致；修复前=静默 no-op）。
- 工件：续作 run=`coord/runs/FE-AUTOLANE-PRODCHAIN-FIX-1/prodchain_20261027_2155/`（**分支 port/fe-autolane-prodchain-fix-1-stack @ 55c07e66**，RESULTS.md+R1/R2 日志+离线回归 27 件含红绿三件+rack 宏测试+隔离副本）；首腿 run=`prodchain_20261027_2050/`（@326dcd9e 不覆盖）；栈登记 c13a7f78–释放 869ce03a（21:59 拆栈全零）。
- 端测边界（承首腿+扩域面）：弹层真窗口落点/推子拖动手感留用户手测节；声相写读=数值面断言（生产方法直调驱动，真实拖动手感留手测）。

## 主管裁决（2026-10-27 晚——扩域授权，卡回 todo 续作）

- **blocked 采信**：停止条件①正确触发（racked 轨 rack.nodes 独占 rack_nodes 快照→轨级 Volume & Pan 行被排除→解析恒空；锚点+最小复现 stack_diag_205_rackshape 在卷）；按 §11 上交不越域=纪律正确。
- **扩域授权**：`vit_dock/core/project_repository.gd` 单点入域（apply_kernel_project_state rack 分支+get_volume_pan_plugin_item_id_for_track 兜底）。修复形态=轨级 built-ins 与 rack.nodes **并源**（合并入快照或解析器兜底，按 diag 证据取最小面并回执披露）；**不得**改变 rack_nodes 既有消费方的行为语义（纯增轨级行/并源读取，既有键零破坏——AGENTS §11）。
- **两消费面回归断言义务**：①车道轨道组+默认音量车道；②**轨道头声相推子 IPC**（racked 轨声相静默 no-op=预存生产缺陷，随本修复收口，修复前后红绿留证）。
- **验收面增补**：真栈 R1b 重放=**20/20 两轮**（racked 轨三组在场+默认音量车道→建点→读回+声相推子写读对照）；其余卡面验收不变。交付面①②③（已绿部分）不重做，续作只补扩域面+重放。
- 原执行 owner 优先续作（分支 port/fe-autolane-prodchain-fix-1 续提交）；206 仍排本卡合入后。

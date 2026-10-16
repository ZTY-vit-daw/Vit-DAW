# MIX-SENDRETURN-DESIGN-1 设计件：send/return 轨间路由能力——内核暴露 + agent 语义 + 观察面 + 机架 GUI

- 执行：GLM-5.3 L2 PC 执行会话（ZCode，2026-10-16 晚窗领取）
- 基线：origin/main=54283662（领取 129eebd5）；worktree D:/Vit_DAW_wt_mixsend_design
- 硬输入：四项用户裁定（2026-10-16 晚窗）+ 混合轨道模型——见卡面，本设计不改动任何一条
- 底座实证：[experiment/RESULTS.md](experiment/RESULTS.md)（AuxMultiReturnProbe **VERDICT PASS**：
  轨到轨多目标回流 2.0001× 实证 + 静音联动 + 乱序防环有界）
- 状态：**待决策侧评审 + 用户过目 GUI 交互规格（§5）后方可拆实现卡**

---

## 0. 设计总纲（一句话）

**用 tracktion 原生 AuxSend/AuxReturn 原语搭「双 bus 三明治」拓扑**：FX 轨=普通混合轨（零新轨类型），
每条 FX 轨分配一对 bus（send_bus 汇干 / return_bus 播湿）；VitApp 在命令面把它包装成
「源轨→FX 轨→回流目标轨」的有向边模型，GUI 画线/列表只是这个模型的两张皮。

```
目标轨 T1 ──AuxSend(send_bus, amount)──┐   ┌──AuxReturn(return_bus)→ [干+湿] → vol → master
目标轨 T2 ──AuxSend(send_bus, amount)──┼─→ FX轨: AuxReturn(send_bus) → [效果器链] → AuxSend(return_bus, 1.0)
目标轨 T3 ──AuxSend(send_bus, amount)──┘   └      （FX 轨 vol=-96dB：湿声只经 return_bus 回流，不直入 master）
                                            └──AuxReturn(return_bus)→ [干+湿] → vol → master
```

四项裁定的落点：
| 裁定 | 落地机制 | 实证 |
|---|---|---|
| 1 湿回流=回目标轨、干湿轨内混合、回线不入 send | AuxReturn 注入位置=目标轨链内 send 之后、vol 之前；湿=bus1 广播注入 | L2=2.0×（湿回两条轨）+ L5（乱序被引擎剔除，封装层强制链序使不可达） |
| 2 发送量=目标轨侧为主 | AuxSend 挂在**源轨**链首位，量=send level 参数；FX 轨侧只读（读回列表） | L3 单量梯度 + 设计 §1 命令面 |
| 3 多轨共享+各轨独立量 | 同 bus 多 send 经 SummingNode 求和，量按源轨各自 GainNode 生效 | L2（440+880 全体入湿）+ L4（单源静音独立抑制） |
| 4 连线=rack 图一等公民 | VitApp 模型层「轨间边」实体 + GUI 腿 4（icon 同级+画线） | 设计 §5 |

---

## 1. 腿 1：内核暴露面（VitApp）

### 1.1 seam 裁定：tracktion 原语，不自实现 graph 层连线

**裁定：用 AuxSend/AuxReturn 原语（双 bus 三明治），不在 VitApp 自建轨间连线机制。**

依据：
1. **能力已实证**（experiment/RESULTS.md）：多目标回流、多源求和、按源独立电平、静音联动、图级防环，
   全部是引擎现成语义，渲染走同一 `EditNodeBuilder` 建图路径（`createPluginNodeForList`
   [tracktion_EditNodeBuilder.cpp:1289-1301](../../../tracktion_engine/modules/tracktion_engine/playback/graph/tracktion_EditNodeBuilder.cpp)：
   AuxSend→`SendNode(node, busID)`、AuxReturn→`ReturnNode(node, busID)`，按插件链序包裹）。
2. **VitApp 无独立音频 graph**：插件链/机架/渲染全部经 tracktion node builder（`VitGraphSwapCoordinator`
   等是机架管理面）。自实现轨间连线=绕开或 fork EditNodeBuilder，成本高且引入第二真相源。
3. **轨到轨输出路由是单目标**（`TrackOutput::setOutputToTrack`
   [tracktion_TrackOutput.cpp:242-246](../../../tracktion_engine/modules/tracktion_engine/model/tracks/tracktion_TrackOutput.cpp)，
   outputDevice 单字符串），**不能**承担多目标回流——排除「FX 轨输出直接路由到目标轨」方案，
   该方案只在单目标场景可行（引擎自测 `runTrackDestinationRendering` 只测 fan-in）。
4. 裁定 4「连线=rack 图一等公民」是**呈现层**要求：VitApp 在模型层维护边实体（source→fx+amount），
   aux 插件只是它的引擎侧投影，呈现与底层解耦，无冲突。
5. bus 编址无冲突：aux busNumber 为小整数，rack/sidechain 的 busID 是大整数 hash
   （`getRackInputBusID` 等，[tracktion_EditNodeBuilder.cpp:55-70](../../../tracktion_engine/modules/tracktion_engine/playback/graph/tracktion_EditNodeBuilder.cpp)），
   VitApp 分配器控制 aux 对号段即可。引擎层 bus 数量无硬上限（`AUXBUSNAMES` 开放 ValueTree，
   [tracktion_Edit.cpp:1755-1790](../../../tracktion_engine/modules/tracktion_engine/model/edit/tracktion_Edit.cpp)）。

### 1.2 模型与持久化

- **FX 轨=普通 AudioTrack**，零新轨类型（用户核心思想）。FX 角色以 ValueTree 属性标注：
  `track.state` 加 `vit_fx_role`（"send_return_fx"）+ `vit_send_bus` / `vit_return_bus`（int 对）。
- 旧工程兼容（§11）：无这些属性的轨读侧=普通轨（fail-open 缺省）；aux 插件本身是 tracktion 原生
  状态（PLUGIN 节点 type=auxsend/auxreturn+busNum），旧内核加载新工程会看到未知 PLUGIN type——
  按既有 unknown-plugin 缺省语义处理，**新工程不可回退到旧内核正常工作**（边界如实登记）。
- bus 对分配器：`vit_send_bus` 取「最小未占用偶数对」；轨删除/降级时回收。bus 名用 FX 轨名
  （≤20 字符，`Edit::setAuxBusName`）供 GUI/读回显示。
- 防环三道闸（由强到弱）：
  1. **命令面拓扑校验**：connect 时跑 send 拓扑 DAG 检查（复用 `VitDagChecker` 思路扩展到轨间边）；
     FX 轨不得再作为 send 源（`vit_fx_role` 轨调 connect 且 source=FX 轨 → 拒绝）；
  2. **链序强制**：VitApp 封装只生成「AuxSend 在 pluginList 首位（含 clip 干信号 tap）、
     AuxReturn 在其后」的链序，乱序在命令面不可达（L5 实证：乱序时引擎剔除行为复杂，不可依赖）；
  3. 引擎 `findSendNodes` 图级剔除兜底（上游环检测，
     [tracktion_TestNodes.h:688-726](../../../tracktion_engine/modules/tracktion_graph/tracktion_graph/tracktion_TestNodes.h)）。

### 1.3 命令 schema（CommandDispatcher 新增 5 命令，风格对齐既有 rack_* 面）

| 命令 | 参数 | 语义（幂等/CAS） |
|---|---|---|
| `track.send.connect` | `source_track_id`, `fx_track_id`, `amount_db?`（缺省 -6.0）, `auto_create_fx_track?`（name） | 建边。幂等：同 (source,fx) 已连→只调量。首连时：源轨插 AuxSend(send_bus)@首位；插 AuxReturn(return_bus)@次位；FX 轨 lazy 初始化（AuxReturn(send_bus)@首位、AuxSend(return_bus,1.0)@RackInstance 后、vol=-96dB、vit_fx_role 标注）。回执含读回快照 |
| `track.send.set_amount` | `source_track_id`, `fx_track_id`, `amount_db` | 调源轨侧发送量（裁定 2 主控面） |
| `track.send.disconnect` | `source_track_id`, `fx_track_id` | 拆边：删源轨 AuxSend；该源轨对其他 FX 无依赖→删其 AuxReturn；FX 轨零源→拆 FX 侧插件+清 role+回收 bus 对 |
| `track.send.list` | `fx_track_id?`（缺省全量） | 拓扑读回：`[{source_track_id, source_name, fx_track_id, fx_name, amount_db, send_bus, return_bus}]`——观察面/CCB 的唯一读源 |
| `track.send.set_fx_role` | `track_id`, `role`（"none"/"send_return_fx"） | 手动升降级 FX 角色（GUI「新建 FX 轨」与 auto_create 的底层原语） |

- 非目标（明确不做）：FX 轨侧调发送量（裁定 2 只读）、pre/post-fader 切换（干 tap 固定 pre-insert 链首，
  后续有需求再议）、send 自动化曲线（后续卡）。
- VSP/CAS/幂等：沿用 vsphub 既有体系（dedupe 键=source+fx+revision；读回命令天然幂等）；
  执行回执带 `track.send.list` 快照做读回核对（对齐 §7 回执口径）。
- 渲染交互：connect/set_amount 后 `restartPlayback`+`ensureContextAllocated`（对齐
  `handleSetFolderRoutingBus` 的收尾模式，[TrackService.cpp:697-710](../../../VitApp/Source/Service/TrackService.cpp)）；
  renderAllowlist 需评估加 `track.send.list`（渲染中只读拓扑允许）。
- 实现落点：新 `SendRouteService`（或并入 PluginRackControlService——建议独立文件，域清晰），
  CommandDispatcher 注册；单测按 Tests/CMakeLists.txt 既有 console-app 模式，把
  AuxMultiReturnProbe 的五腿断言收编为内核测试（VitApp/Tests/SendRouteTests.cpp）。

### 1.4 与机架（RackType）共存

FX 轨的效果器装在 FX 轨自己的机架（Z3 区，现状装载面不变）。链序约束：FX 轨 pluginList =
`[AuxReturn(send_bus), RackInstance..., AuxSend(return_bus), VolumeAndPan(-96dB), Meter]`
——send 插在 RackInstance 之后（发的是处理后湿声），封装层管索引，rack_add_node 自动链接逻辑
（`vitTryChainNewRackPluginSerial`）感知 aux 边界（在 aux send 前串接）。

---

## 2. 腿 2：agent daw 适配 + 编排器拓扑语义

### 2.1 命令封装

- `agent/internal/harness` 命令表 + `chat/free_state_invoke_guard.go:87` 写命令白名单扩 5 命令；
  `chat/server.go:2987` 命令路由、`chat/events.go:431` 事件路由同步扩。
- policy.Analyze 决策面新增 send 边的变更类别（对齐 rack 边的 mutation 分类）。

### 2.2 拓扑决策表（话术「给某轨加混响/延迟」的分叉点）

现状接缝：`runPluginGrabberLoadWorkflow`
（[plugin_grabber_workflow.go:68](../../../agent/internal/chat/plugin_grabber_workflow.go)）固定
insert 直挂（rack_add_node 到目标轨自己机架，效果器硬编码 Z3）。加拓扑决策分叉：

| 话术/上下文信号 | 拓扑决策 | 呈现 |
|---|---|---|
| 单轨 + 空间类效果器，无共享信号 | **insert（默认，现状不变）** | 「已装载到 X 轨机架」 |
| 话术含「共享/总线发送/多条轨一起/同一个空间」 | **send 结构** | pendingplan 呈现回路图（源轨们→FX 轨→回流+各自发送量），确认后 auto_create FX 轨+批量 connect |
| agent 主动建议：≥2 轨有同类空间诉求（C3 观察+MOM 关系证据） | 建议 send（用户可拒） | 建议理由=共享空间+省实例+独立发送量 |
| 用户明说「只给这条轨」 | insert 锁定 | 不再建议 send |

- 意图确认面：send 结构属多轨结构性变更——走既有 PendingPlan 预览确认（preview 含拓扑 diff：
  新建 FX 轨名、各源轨发送量缺省、回流示意），不新增确认机制。
- 话术回复模板（send 执行后）：呈现「回路=源轨→FX 轨→回流，发送量列表」+提示可在机架视窗列表
  手动接回（对齐用户核心思想「列表标识，用户可手动接回」）。
- C3 空间深度域升级：`track.send.list` 让空间观察从「插件参数」扩展到「轨间结构」
  （c3 现边界：plugin_path 词法分类，[c3_space_depth_runtime.go:22-25](../../../agent/internal/chat/c3_space_depth_runtime.go)——
  send 拓扑读回是结构事实，不依赖插件语义 inspector，恰好绕开 MIX-CAP-RECON-1 §2 缺口）。

### 2.3 诚实边界

- 话术拓扑决策是**启发式**（词法+上下文），不是语义理解保证；决策表落在 plugin_grabber_workflow
  的意图分类层，误判走用户确认面拦截（pendingplan 可拒）。
- FX 轨命名建议缺省「Reverb Bus」/「Delay Bus」类（按效果器类别），用户可改。

---

## 3. 腿 3：观察投影面

原则：投影只消费 `track.send.list` 的结构事实（evidence refs 指向 mix.read），不展开音频包。

| 投影 | 新增内容 | 形态 |
|---|---|---|
| **MOM**（轨间关系域，主承载） | 新投影器 `send_topology`：输入=mixboard 新包 `project.send_topology_inputs`（agent 侧轻量派生自 track.send.list，对齐 [project_package.go:70-76](../../../agent/internal/mixboard/project_package.go) 既有 inputs 模式）；投影行=`{source→fx, amount_db, return_edge, status}`，标注「共享空间结构事实，非效果语义」 | 与 frequency/static_level/masking 关系并列；CCB 注册 view `mix.send_topology`；LLMContext 紧凑事实（peer 投影纪律：不做自动 view 选择） |
| **TOM**（轨组织） | 轨行加 `fx_role`（该轨是否 FX 轨）+ `send_edge_count`（轻量计数，关系细节归 MOM 不重复） | tom.projection.v0_2 版本升级（v0_3），旧字段不动 |
| **FXM**（A/B 同路由对比） | 路由指纹（routing fingerprint）纳入 send 拓扑 hash：send 边变化→同路由前提破坏→FXM 缓存标记 stale | 复用现有 fingerprint 失效机制，不新增投影 |
| **TIM**（技术完整性） | 新断言行：send 边悬空检测（fx_track_id 指向已删轨/bus 对失配/AuxSend 插件丢失）→ partial 行 + 修复建议命令 | 沿用 tim 断言框架 |

依赖：MOM/TOM/FXM/TIM 四投影全部依赖内核 `track.send.list` 读回面（腿 1 先行）。

---

## 4. 腿 4：机架 GUI 交互规格（Godot 前端）

> 现状事实（取证）：`Vit_Graph_Rack.gd`（GraphEdit，单轨画布，clip/z1/z2/z3 泳道）；
> **拖拽画线链路已存在**（`connection_request`→`_on_connection_request:4894`→`rack_connect_pins`，
> 经 UDP 4445→godot_bridge→ZMQ 内核）；自绘电缆 `_draw:5224`；边只画内核确认边；
> `vit_daw_track_kind.gd:9` 已有未使用的 `EFFECT_RETURN=3` 枚举；
> 前端可发起命令（rack_client 命令面）。

### 4.1 FX 轨 icon（与插件 icon 同级）

- 机架图新增节点类型 `KIND_FX_TRACK`（常量表扩展，对齐插件 icon 三类）：
  - **渲染来源**：本工程全部 `fx_role` 轨（跨轨实体注入每轨画布；快照源 `get_rack_nodes` 附
    fx_track 实体行——缺口 a/缺口 d 的补法）；
  - **视觉**：轨道色底+效果器徽标（与插件 icon 区分：FX 轨 icon 带轨道名，无参数面板）；
  - **落位**：z3 泳道尾（效果器区），跨轨常驻（每轨画布都显示同一组 FX 轨 icon）；
  - 轨道列表侧启用 `EFFECT_RETURN=3` 角色显示（FX 轨在轨列表带角色徽标）。
- 双击 FX 轨 icon = 跳转该 FX 轨的机架画布（复用 track_focus_changed 换轨链）。

### 4.2 拖拽画线（send 连接的一等交互）

- 复用现有画线链：从 FX 轨 icon 输出锚点拖到本轨（=目标轨）的任何插件节点/轨道输出锚 →
  `connection_request` → 拓扑门 `can_connect_rack_topology` 放行 FX 轨节点类型（缺口 c）→
  payload 映射 `track.send.connect{source=本轨, fx=该 FX 轨}`（rack_client 新增 3 命令封装）→
  乐观 pending 边 → 内核确认边回读（`rack_pending_edge_rules` 回滚模式不变）。
- 反向拖（插件→FX icon）= 同一连接（拓扑门归一方向）。
- 边样式：send 边与机架内电缆**同款渲染**（一等公民），但带发送量 badge（如 `-6.0dB`）；
  badge 拖动上下=调量（`track.send.set_amount`），右键边=断开（`track.send.disconnect`）。

### 4.3 连接列表标识（用户核心思想的「列表」面）

- **目标轨侧（send 槽位，主控面——裁定 2）**：通道条/机架面板加 send 槽位列表，
  每行=「FX 轨名 + 发送量 + 通/断开关」；行内调量即 set_amount。
- **FX 轨侧（只读）**：FX 轨画布顶部来源列表=「哪些轨在发送+各自量」，不可调（视觉降权），
  行点击=跳转源轨画布。
- 手动接回三操作（断开/重连/调量）全走内核命令+确认回读，不走前端本地状态。

### 4.4 缺口清单（实现卡范围）

a) `Vit_Graph_Rack` 单轨绑定→跨轨 FX 轨实体渲染源；b) `rack_connect_pins` payload 无源轨→FX 轨
schema（新增 send 命令封装而非改 rack_connect_pins 本身）；c) 拓扑门未放行 FX 轨节点；d) 跨轨
send 边不在 `get_rack_edges_for_track` 快照（需内核快照面扩展 send 边行）；e) send 量参数面
（badge/槽位调量 UI）；f) EFFECT_RETURN 枚举启用+轨列表徽标。

---

## 5. 烟测场景（实现卡验收口径）

1. **内核腿**（真栈脚本扩展，复用 dev_agent_smoke/g 只读白名单模式）：
   connect→list 读回核对→`l2_render_probe`/start_render 断言湿分量（探针五腿的端侧版）→
   disconnect→list 归零→渲染对照；乱序/环配置命令面拒绝断言。
2. **agent 腿**：话术「给 vocals 和 guitar 共用一个板式混响」→pendingplan 呈现 send 结构→
   确认→执行回执（track.send.list 断言+MOM send_topology 投影行断言）；单轨话术仍走 insert（回归）。
3. **GUI 腿**：渲染面烟测（E2E-WEBUI-1 体系外的 Godot 面按 FE 系列脚本模式扩展：
   FX icon 渲染/画线/槽位 DOM 级断言）；用户手测清单按 §5 手测入口书写
   （Godot 拉起→进工程→机架画线→听感 dry+wet）。

## 6. 实现拆卡建议（三线排期）

| 卡 | 内容 | 文件域 | 依赖 |
|---|---|---|---|
| **K1 内核**（VitApp） | SendRouteService：5 命令+bus 分配器+FX 轨 role+拓扑 DAG 校验+链序强制+读回；SendRouteTests（探针五腿收编）；真栈烟测脚本扩展 | VitApp/Source/Service/{SendRouteService.*,CommandDispatcher.cpp}、VitApp/Tests/、scripts/ | 无（先行，命令面冻结后放行 K2/K3） |
| **K2 agent** | harness 命令表+guard 白名单+policy 变更类别；plugin_grabber 拓扑决策表+pendingplan 拓扑 diff+话术模板；C3 升级；MOM send_topology+TOM v0_3+FXM 指纹+TIM 断言 | agent/internal/{chat,harness,policy,mom,tom,fxm,tim,mixboard,capabilitycontext} | K1 |
| **K3 前端**（Godot 仓） | FX 轨 icon+跨轨渲染源+画线拓扑门+send 边 badge+send 槽位+FX 侧只读列表+快照源扩展（缺口 a-f） | D:\Godot\project\vit-daw-frontend（app/rack、vit_dock、app/tracks） | K1（可先做视觉壳） |

排期：K1 →（K2 ∥ K3）。K3 手测（画线听感）=用户在场节点，卡内标 [等待用户]。

## 7. 开放问题（交决策侧/用户）

1. **solo 语义边界**：目标轨 solo 时其他源轨被静音（引擎 mute-state 默认），湿声退化为 solo 轨
   自己的发送（裁定 1 主意图「solo 不掉自己的混响尾」满足；「共享空间在 solo 时保留他人湿声」
   需 EngineBehaviour 定制 `shouldProcessAuxSendWhenTrackIsMuted`——建议 v1 不做，如实呈现）。
2. **FX 轨主输出**：设计=vol 压 -96dB（湿只走回流）。备选=FX 轨输出可切换「回流/master 双出」
   （传统 DAW 行为）——v1 建议纯回流（贴裁定 1），双出留开关后续议。
3. **旧内核加载新工程**的 unknown-plugin 缺省行为需在 K1 落实测试（§11 持久化兼容）。
4. FX 轨命名/数量上限（bus 对号段建议 0-63，够 32 条 FX 轨）。

## 8. 取证锚点索引（本设计的证据链）

- 底座实证：[experiment/RESULTS.md](experiment/RESULTS.md)（五腿数据+可复现命令）
- AuxSend/AuxReturn 原语：tracktion_AuxSend.cpp:14-29（busNum+send level 0..1）、tracktion_AuxReturn.cpp:14-17
- 链序包裹：tracktion_EditNodeBuilder.cpp:1289-1301（createPluginNodeForList）
- 求和/广播/防环：tracktion_TestNodes.h:688-747（findSendNodes：多 send→SummingNode、多 return 各自收集、上游环剔除）
- 发送电平生效点：tracktion_AuxSendNode.cpp:16-52（gainFunction 在 return 侧 GainNode 应用，含 mute/反相）
- 轨到轨单目标：tracktion_TrackOutput.cpp:242-246/297-303（setOutputToTrack 单串+feedsInto 防环）
- bus 命名/无上限：tracktion_Edit.cpp:1755-1790
- VitApp 命令面先例：PluginRackControlService.cpp:3360-3826（rack_add_node/connect_pins/remove_connection）、
  TrackService.cpp:669-710（routing_bus 收尾模式）、CommandDispatcher.cpp:3450-3474（render 命令）
- agent 接缝：plugin_grabber_workflow.go:68-140（insert 直挂+Z3 硬编码）、
  c3_space_depth_runtime.go:22-46（空间域只读边界+词法分类）、free_state_invoke_guard.go:87（写命令白名单）、
  mixboard/project_package.go:70-76（agent 侧轻量派生 inputs 模式）
- 前端接缝：Vit_Graph_Rack.gd（画线链 4894/自绘边 5224/单轨绑定 168）、rack_client.gd:19-32（命令面）、
  vit_daw_track_kind.gd:9（EFFECT_RETURN 枚举）、rack_ipc_payload_rules.gd:62-77（x/y 消费）

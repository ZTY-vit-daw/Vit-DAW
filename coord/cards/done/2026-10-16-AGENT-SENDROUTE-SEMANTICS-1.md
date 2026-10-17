# AGENT-SENDROUTE-SEMANTICS-1（K2）：send/return agent 语义与观察面

- 发卡：GLM 主管决策侧 / 2026-10-16 晚窗（依据=设计件评审 pass+GUI 定稿 §9；授权同 K1）
- 派发确认：**已确认（同 K1 链）**；验收负责人：GLM 主管决策流
- 池序 128；目标仓库=D:\Vit_DAW；P1 / 2-3 天 / **依赖 K1（127）合入后领取**
- 模型分级：L2 / GLM 执行会话（agent 多包面）

## 规格（权威=设计件 §2+§3+§9）

1. **命令适配**：harness 命令表+`chat/free_state_invoke_guard.go:87` 写命令白名单扩 5 命令；`chat/server.go` 命令路由+`chat/events.go` 事件路由同步；policy.Analyze 新增 send 边变更类别（对齐 rack 边 mutation 分类）；
2. **编排器拓扑决策表**（接缝=plugin_grabber_workflow.go:68-140 现状 insert 直挂+Z3 硬编码）：单轨+空间类无共享信号→insert（现状不变）；话术含「共享/总线发送/多条轨一起/同一个空间」→send 结构（pendingplan 呈现回路+各源发送量，确认后 auto_create+批量 connect）；agent 主动建议（≥2 轨同类空间诉求，C3+MOM 证据）→建议可拒；用户明说「只给这条轨」→insert 锁定。决策是启发式，误判走 pendingplan 用户拦截——诚实边界声明；
3. **观察投影**：MOM 新投影器 `send_topology`（输入=mixboard `project.send_topology_inputs`，agent 侧轻量派生自 track.send.list；投影行=source→fx+amount+return_edge；CCB 注册 view `mix.send_topology`，peer 投影纪律零自动选择）；TOM v0_3（轨行加 fx_role+send_edge_count，旧字段不动）；FXM 路由指纹纳入 send 拓扑 hash（边变→缓存 stale）；TIM send 边悬空断言（fx_track 指向已删轨/bus 失配/AuxSend 丢失→partial+修复建议）；
4. **C3 空间深度域升级**：`track.send.list` 让空间观察从插件参数扩展到轨间结构（绕开 MIX-CAP-RECON-1 §2 词法缺口——send 拓扑读回是结构事实）；
5. 话术回复模板：send 执行后呈现回路+发送量列表+机架视窗手动接回提示。

## 文件域

agent/internal/{chat,harness,policy,mom,tom,fxm,tim,mixboard,capabilitycontext}；领取前与决策侧核对。

## 验收标准

- 单测：拓扑决策表四分支（insert 默认/send 话术/主动建议可拒/锁定）+投影行 schema+FXM 指纹失效+TIM 断言各至少一例；
- 真栈（依赖 K1 合入）：话术「给 vocals 和 guitar 共用一个板式混响」→pendingplan send 结构→确认→执行回执（track.send.list 断言+MOM send_topology 投影行断言）；单轨话术仍 insert（回归）；exit 0；
- `go test ./... -count=1` 0 FAIL + gofmt 净。

## 停止条件

- K1 命令面实测 schema 与设计件偏差 → 停手上交对齐，不自行改内核；
- CCB/MOM 注册遇 view 目录红线（自动选择/partial 升级）→ 上交裁定。

- 领取：2026-10-17 深夜 +0800 / origin/main=b0ab76556bce20c531a1769fbf16f00086390b5e（本地=远端一致亲核） / owner=GLM-5.3 L2 PC 执行会话（ZCode，本会话） / 分支 port/agent-sendroute-semantics-1 / worktree D:/Vit_DAW_wt_sendroute2 / 领取提交=coord/agent-sendroute-semantics-1-claim（本提交）；领取基线 HEAD=b0ab7655、status 净、diff 空（亲核）
- 领取时核对：卡在 todo、派发已确认（同 K1 链）；前置 K1 已合入 main（f4926543 在历史，SendRouteService 五命令+点式/下划线双别名亲核=CommandDispatcher.cpp:3439-3497）；schema 与设计件 §1.3 对齐亲核（connect 含 auto_create 按名幂等复用+回执内嵌 sends/dangling/fx_tracks/sidechain 拓扑快照——停止条件第一条不触发）；doing 池两卡文件域零交集（LOUDNESS-B 修订腿只动 scripts/run_free_state_d1_smoke.ps1，其 chat 域阶段一已按裁定先落 5e9d2f65；GEN-SUNO 零生产代码）；agent 侧九包接缝亲核=catalog spec 表/guard :87 白名单/server :2987 broad-mix 面/events :431 toolDisplayName/plugin_grabber_workflow :68-140 insert 直挂+Z3/c3 :22-46 三载体/mixboard project_package inputs 模式/mom-tom-fxm-tim 投影结构/CCB view 目录=capabilitycontext+contextruntime+harness 三处
- 回执（执行侧 done，2026-10-17 深夜）：**impl=a57e0621+fix=cd5d7a9d**（port/agent-sendroute-semantics-1，已推 origin）。①命令适配：catalog 5 spec（connect/disconnect/set_fx_role=RiskConfirm 对齐 rack 边；set_amount=RiskUndoable；list=RiskDirect）+下划线工具别名+free_state_invoke_guard 白名单 4 写命令+server broad-mix 面+events 显示名+harness broadMixWriteCommand 写面表；②拓扑决策表四分支=chat/send_topology_decision.go 纯函数（锁定>send 结构（共享词法+≥2 具名源轨）>主动建议（单轨+空间意图+≥2 词法同类轨）>insert 默认），单测钉四分支+边界（共享但单源不升级）；③send 工作流=send_route_workflow.go：循环入口确定性拦截（runAgentLoopChat 语义分类前，M1-2 queue 先例同层；legacy 尾部同名分叉保留）→pendingplan send_topology_connect（每源一条 track.send.connect auto_create 同名幂等，拓扑 diff 预览=回路+各源-6.0dB+回流示意）→确认执行→收尾话术模板（回路+发送量列表+机架视窗手动接回提示）；写闸豁免=harness.AuthorizeSendTopologyConnect（计划源轨集合凭据，JSON 形态可持久往返，越轨源/无凭据不豁免）；agent_suggest 分支附建议话术不改 insert 计划；④观察面：requestMixObservation 装配末尾 track_send_list 轻量读注入 state.send_topology（读失败键缺席投影如实 missing）→mixboard send_topology_inputs（agent_side_lightweight 面）→MOM send_topology 投影（mom.send_topology.v0_1；行=source→fx+amount_db+send/return_bus+return_edge+悬空计数；evidence=mix.read+kernel.track_send_list；LLMContext 紧凑事实零 raw 包）+CCB view mix.send_topology（注册=capabilitycontext 目录+contextruntime 决策投影两处+harness intent 复用 multitrack；bounded disclosure 零自动选择）+TOM v0_3（轨行 fx_role+send_edge_count 旧字段不动，无输入保 v0_2 形态）+FXM 路由指纹折叠 send 拓扑 hash（SendTopologyHash canonical 排序+RoutingHashWithSendTopology 进 ProjectionID 种子旋转，复用现有失效机制）+TIM evaluateSendEdges（悬空 fail 行带 disconnect 修复建议 Detail；读缺席 not_evaluable）；⑤C3 第四载体=track.send.list 进只读白名单+报告结构面段+渲染（白名单 3 契约同步）。**验收命令**：go test ./... -count=1=95 包 0 FAIL；触碰文件 gofmt 净（CRLF 检出伪影外）；契约同步四处如实（view 目录 18→19、维度词表+space_structure、C3 白名单 2→3+send 写命令入交叉黑名单、五处 observe 断言放行只读 track_send_list——原不变量=无特性请求/副作用，非无读）。**真栈**：dev_agent_smoke -StartKernel -RestartAgent -SendRouteAgentSmoke 四 run（run1 环境中断=worktree 无内核产物→补建含 K1 内核；run2=集成缺陷修复（循环入口）；run3=param 误删修复；均不同签名）→**run4 sendroute_agent_20261017_221712 PASS exit 0 五腿全绿**：send 话术→pendingplan（plan_6d0f51745a3c8882）→确认→track.send.list=2 边+1 FX(1013 Reverb Bus)+0 悬空→MOM send_topology=ready/2edges/1fx→FX 轨 rack 装载 ZamVerb 后边保持 2（K1 归一钩子边界腿覆盖）→单轨话术零 send 执行非 send 工作流。栈占用按协议登记/释放（22:20 释放复核=0）。
- 边界申报：①send 分叉只在【共享词法命中+话术内可按名解析≥2 轨】时抢占（词法启发式非语义理解，误判走确认面拦截——卡面诚实边界保持）；②FX 轨效果器装载不在 send 工作流内执行（设计件 §1.4「现状装载面不变」）：确认收尾提示用户走常规装载话术，真栈腿用内核 ZMQ 直驱 rack_add_node 覆盖 K1 归一钩子边界；agent 面装载仍受 PCA 准入（run2 亲证 full-access 下 Abbey Road Plates 非 promoted 被 pca_load_gate 拒——闸行为正确非缺陷）；③MOM send_topology 投影随 observe 装配在场（零自动 view 选择）；CCB mix.send_topology 仅显式请求披露；TOM v0_3 的 send 事实依赖 state.send_topology 注入（chat canary 运行时走 VSPStateSnapshot LegacyState 无该键→字段省略保 v0_2 形态，如实边界）；④TIM 悬空断言依赖内核 dangling 行（K1 读回面），bus 失配/AuxSend 丢失的内核侧检测粒度=K1 实现现状；⑤run4 的 insert 回归腿形态=no_plan（循环对单轨话术的形态），断言=非 send 工作流+零 send 执行；insert 正向装载腿未单独断言（PCA 环境依赖），以 run2 循环装载尝试+既有 insert 面测试背书。
- 验收：（待决策侧）
- 验收：（裁定文件 / 验收 commit）

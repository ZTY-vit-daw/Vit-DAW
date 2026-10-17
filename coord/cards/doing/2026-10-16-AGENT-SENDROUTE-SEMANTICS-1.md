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
- 回执：（commit / run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）

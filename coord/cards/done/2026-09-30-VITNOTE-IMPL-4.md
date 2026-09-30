# VITNOTE-IMPL-4：工程级写租约——多 agent 会话写互斥（vit note v1 IMPL 骨架第一张可执行卡）

- 池序 5（vit note v1；来源=docs/VITNOTE_V1_DESIGN.md §7.3 + §10）；**并行域：agent/internal（本卡）× agent/webui（FIX-CONFIRM-CARD-1/OPT-OBSERVE-OUTPUT-1）——不同包域可并行**
- 优先级 / 预估 / 依赖：P1 / 0.5–1 天 / 设计依赖=docs/VITNOTE_V1_DESIGN.md §7.3（已落笔 2026-09-30，规格已定）；无代码依赖，领取即开工
- 模型分级：L1 / GLM 或 flash 可接（规格已定，照设计实现）
- 背景与已核实事实（VITNOTE-RECON-1 §4/§6 采信，勿重勘）：
  - 并发模型=多会话并行：每 /agent/chat 请求在自身 HTTP goroutine 同步跑完整 agentloop（internal/chat/server.go 全文零 go func）——主代理与 vit note 会话并发写同工程会互相踩（产品语义层冲突）
  - 写串行化现状仅三粒度：内核命令（ZMQ REQ/REP 逐命令，kernel/client.go）/ CAS 授权一次性消费 / orchestration store OS 级文件锁——保数据一致性，不保任务互斥
  - RiskCeiling 现为元数据、全仓零执行消费方；builtins.go 六项能力全部声明 bounded_reversible（均写参类）
  - executionruntime.Coordinator.ExecuteWithPersistence（coordinator.go:45+）为带持久化的能力执行路径（frozen plan hash+ProjectCut hash 校验+授权消费）
- 目标：
  1. 新建工程级写租约组件（落 agent/internal/executionruntime，或其内聚子包）：`Acquire(projectID, ctx) → lease / Release` 语义；粒度=**单次能力执行段**（begin→commit/release，不跨轮持有）；竞争者阻塞排队（不拒绝、不失败）；持有超时自动释放（默认 120s，具名常量）；等待者 ctx 取消即退出并返回取消错误。
  2. 接入 Coordinator 能力执行路径前置：**v1 闸域=全部能力执行**（当前六项内置能力均为写类 bounded_reversible；观察/queryengine 查询不经能力执行路径天然并行——此边界为设计既定，卡内不扩大不缩小）。裸内核命令直写不在 v1 租约域（已知边界，仅记卡面）。
  3. 单测（executionruntime 包内）：两 goroutine 竞争同工程租约断言执行段串行不重叠；超时自动释放后后来者可得；等待者 ctx 取消返回；跨工程租约不互斥（反例）；Release 幂等。
  4. 真栈烟测：scripts/dev_agent_smoke.ps1 体系新增（或扩展参数）"并发双 chat 各触发一次能力执行"场景——断言两执行段时间区间不重叠（租约日志或工件时间戳为证），exit 0。具体入口参数领取后按脚本现状实锚，卡内不预写死。
- 文件域：agent/internal/executionruntime/（新组件+Coordinator 接入+测试）+ scripts/dev_agent_smoke.ps1（场景扩展）。接口冻结：不改变 Coordinator 既有对外签名与语义（前置钩子式接入）；不动 capabilityruntime/chat 包（source 身份标注属 VITNOTE-IMPL-3）。
- 验收标准：`go test ./internal/executionruntime/...` 全绿 + 全量 87 包 0 FAIL + gofmt 通过 + 真实栈烟测场景 exit 0（AGENTS §5）+ 回执附租约并发证据工件路径。
- 停止条件：取证发现能力执行存在多条并行入口、无法在 Coordinator 单点收口（架构前提失效）→ 停下实证上交（附入口清单锚点），由决策侧定接入面；禁止自行扩大文件域。
- 风险点：等待者长阻塞占用请求 goroutine——v1 接受（单机单 agent、能力执行段短）；若实测执行段 >30s 须在回执申报实际时长分布。
- 领取：2026-09-30 09:11 / origin/main=1d98667e（FIX-CONFIRM-CARD-1 领取提交后同步点）/ 分支 port/vitnote-impl-4（独立 worktree D:/Vit_DAW_worktrees/vitnote-impl-4）；领取时工作树预存改动=VitApp/Workspace/{Settings.xml,default_project.xml}（运行时工程态，非本卡，不动）；领取时取证：六项能力执行全部经 internal/chat → orchestrationRuntime.ExecuteActionSetWithPersistence → executionruntime.Coordinator 单点（c1/c2/b4/semantic_eq/pan_layout/canary/free_state_d1 共 10 处调用锚），无第二执行入口，架构前提成立
- 回执：commit=75e57a63（port/vitnote-impl-4，4 文件 +1172：writelease.go 305 / writelease_test.go 395 / coordinator.go +25 / dev_agent_smoke.ps1 +448）；真栈烟测 run=write_lease_20260930_101123（exit 0，工件 D:/Vit_DAW_worktrees/vitnote-impl-4/VitApp/Workspace/Artifacts/smoke/write_lease_20260930_101123/{summary.json,lease_events.json,propose_*.json,approve_*.json}；原始事件流 Logs/write_lease_events_20260930_101100.jsonl）。烟测实测：双 conversation propose B2 后并发 approve——A 段 [02:12:19.312→22.706] 3394ms executed_verified（含验证渲染），B 于 19.638 wait_started 真实排队 3.07s，22.7062324 与 A 释放同纳秒交接零重叠，B 段内 5ms 以 "execution preflight: stale_project_cut" 诚实拒绝（分层安全语义：租约串行化段 + revision CAS 段内拦截）。验收命令全过：go test ./internal/executionruntime/ 全绿（8 新测试）+ 全量 87 包 0 FAIL（分支 run4 EXIT=0；run1/2 命中 chat TestWorkspaceSwitchSettlesInFlightChainExplicitly TempDir 清理竞态、run3 命中 harness TestMixboardSnapshotConcurrentDualWrite 5s 截止超时——归因：harness 0 依赖 executionruntime、chat 失败测试零能力执行路径，隔离复跑 3/3 绿+主基线全量绿，属负载敏感环境 flake，日志留档 .fulltest*.log/.fulltest_main_baseline.log）+ gofmt 干净（本卡 4 文件；新 worktree CRLF 检出坑已用 gofmt -w 归一，提交内容为 LF）+ go vet 通过。风险点申报：实测执行段 3394ms（<30s 阈值）；等待者阻塞 3.07s（v1 接受面）；race detector 本机不可用（无 gcc，CGO 门），并发断言以 channel/mutex 时序化+真栈事件纳秒时间戳补位。端测边界声明：①烟测栈=VitApp 内核+Go agent（脚本构建重启），场景断言域=agent HTTP 面（chat/invoke/interaction/respond）+租约事件工件，零 Godot UI 接口交互——带 -StartUI 实测两轮均因 Godot start-page 自行 spawn VitAgent 与脚本 -RestartAgent 竞态双绑固定 UDP bridge 4445 双亡（脚本体系既有竞态，非本卡引入，建议决策侧另开脚本体系卡）；②渲染面（webui）零改动，无 E2E-WEBUI-1 义务；③并行会话实录：VitAgent 全实例共抢固定 UDP 4445（单机单实例约束），本卡烟测两次被并行 webui 域会话的 VitAgent.e2ewebui1（7897）阻断——§9 跨会话端口栈冲突实录，建议决策侧知悉该固定端口约束。已知边界照卡面记：裸内核命令直写不在 v1 租约域；Reconcile（崩溃恢复路径同为变异面）已纳入租约域（卡面"全部能力执行"语义内聚，回执声明）；观察/queryengine 天然并行不受影响。停止条件未触发（单点收口领取时已取证）。
- 验收：**pass（2026-09-30 决策会话）**——裁定=[rulings/2026-09-30-VITNOTE-IMPL-4-pass.md](../../rulings/2026-09-30-VITNOTE-IMPL-4-pass.md)；合并 commit 4583d31d（cherry-pick）；复跑 go test executionruntime ok+租约五要素/接口冻结/烟测 summary 亲读；flake 两处按 §11 首次记录采信；脚本体系两问题（-StartUI 竞态/并行真栈端口冲突）上交决策侧待开卡。

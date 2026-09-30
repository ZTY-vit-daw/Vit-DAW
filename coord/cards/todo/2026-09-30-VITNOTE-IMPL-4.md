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
- 领取：（时间 / origin/main hash / 分支名）
- 回执：（commit hash / 烟测 run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）

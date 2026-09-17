# PORT-C4：只读冒测 mac 等价物（g_runtime_readonly_smoke 白名单 GET 模式移植）

- 优先级 / 预估 / 依赖：P1 / 1 天（PORT_AUDIT 层 C，运行时缺口第二道护栏）/ 依赖 agent 可在 mac 本机运行（go build 已 green，满足）
- 目标：`scripts/g_runtime_readonly_smoke.ps1` 的白名单 GET 探测模式移植为 mac 可执行脚本（bash/zsh），在真实运行中的 Go agent 进程上验证 HTTP/ZMQ 探活面
- 文件域：`scripts/`（新脚本，命名对齐现有惯例如 `g_runtime_readonly_smoke_mac.sh`）；不修改 ps1 原件
- 验收标准：mac 本机跑通一次完整链路（启动 agent → 白名单 GET 全部返回期望状态码 → 脚本 exit 0）；探针清单与 ps1 版白名单一一对应并附对照表；运行时状态写独立临时目录（AGENTS §10）
- 停止条件：发现 agent 在 mac 上启动失败/端口面异常 → 转 blocked 上报（那本身就是有价值的运行时缺口证据，按失败类型记录）
- 领取：2026-09-17 14:32 CST / origin/main af5aa65723ac9a52a0d67a5cc17213b2466294e8 / 分支 port/c4-readonly-smoke-mac
- 回执：实现 commit 560ddef（分支 port/c4-readonly-smoke-mac；文件域内：scripts/g_runtime_readonly_smoke_mac.sh 新增 + SMOKE_TESTS.md 补条目，ps1 原件未动）。执行侧自验 2026-09-17：
  1. 真实 agent 全链路 PASS：`g_runtime_readonly_smoke_mac.sh --start-agent --timeout 30` → exit 0（14:39 CST）。链路=构建当前工作树 agent（sha1 db3fe3a06759e7d8aa40792d530aac778636bd4b）→ 状态隔离启动（mktemp 工作目录 /var/folders/bm/9470lvs56ss6lfw1n9g66k2c0000gn/T//g_readonly_smoke_mac.Kqcvf1KI，VIT_ORCHESTRATION_STORE_PATH/日志/回执目录全部钉入该目录）→ 4 个白名单 GET 全部 2xx → 摘要输出 → 进程干净收尾（pgrep 空、7878 无监听、日志含 shutdown complete）。
  2. fixture 等价性 PASS：对 g_runtime_readonly_fixture_server.py（:7879）exit 0，摘要 12 项字段与 fixture 已知轨迹逐一相符（task-g-readonly / slice_count=2 / event_count=1 / observation_in_progress 等）。
  3. 探针对照表（1:1，见脚本头与 SMOKE_TESTS.md）：/health、/agent/runtime/status、/agent/state、/agent/events。唯一差异：events 探针补 `conversation_id=<probe>&limit=200`——真实 agent 契约（chat/events.go）无 conversation_id 答 400，ps1 裸 `?limit=200` 只满足忽略全部查询参数的 fixture server（即 ps1 版对真实 agent 会失败，属移植中修正的契约缺口，已记入对照表）。limit=200 在真实 agent buffer 上限 500 内。
  4. 只读自守卫：自扫描非 GET 构造 token（碎片拼装防自匹配，对齐 ps1 长格式做法）；负例测试注入 `-X POST` / `--data` 各自 fail-closed exit 1。
  5. AGENTS §10 核查：全部运行时状态在独立 mktemp 目录，运行后 `git status` 仅本卡新增文件，源码树零污染。
  6. 运行时观察（非缺陷）：无内核时 /agent/state 每次调用重试内核拨号（VSP snapshot + legacy 各 ~252ms connection refused 后 200，总计 ~0.5s/次）；agent HTTP 面与内核 ZMQ 面解耦良好，无内核可完整探活。
  端测边界声明：本冒测覆盖 agent HTTP 白名单探活面（ps1 1:1 口径）；ZMQ 5555/5556 为内核向 connect-out、不在 ps1 白名单内故未纳入；未涉及 webui 渲染面与用户旅程面（本卡无相关改动）。
- 验收：**pass**（裁定 [2026-09-17-C4-pass.md](../../rulings/2026-09-17-C4-pass.md)，决策侧隔离 worktree 四项独立复验全过）；实现 `560ddef` cherry-pick 入 main `8e198a5`，随裁定批推送

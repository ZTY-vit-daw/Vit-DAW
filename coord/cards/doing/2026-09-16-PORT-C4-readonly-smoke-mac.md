# PORT-C4：只读冒测 mac 等价物（g_runtime_readonly_smoke 白名单 GET 模式移植）

- 优先级 / 预估 / 依赖：P1 / 1 天（PORT_AUDIT 层 C，运行时缺口第二道护栏）/ 依赖 agent 可在 mac 本机运行（go build 已 green，满足）
- 目标：`scripts/g_runtime_readonly_smoke.ps1` 的白名单 GET 探测模式移植为 mac 可执行脚本（bash/zsh），在真实运行中的 Go agent 进程上验证 HTTP/ZMQ 探活面
- 文件域：`scripts/`（新脚本，命名对齐现有惯例如 `g_runtime_readonly_smoke_mac.sh`）；不修改 ps1 原件
- 验收标准：mac 本机跑通一次完整链路（启动 agent → 白名单 GET 全部返回期望状态码 → 脚本 exit 0）；探针清单与 ps1 版白名单一一对应并附对照表；运行时状态写独立临时目录（AGENTS §10）
- 停止条件：发现 agent 在 mac 上启动失败/端口面异常 → 转 blocked 上报（那本身就是有价值的运行时缺口证据，按失败类型记录）
- 领取：2026-09-17 14:32 CST / origin/main af5aa65723ac9a52a0d67a5cc17213b2466294e8 / 分支 port/c4-readonly-smoke-mac
- 回执：
- 验收：

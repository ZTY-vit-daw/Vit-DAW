# PORT-VSPHUB-1：vsphub mac 运行时就位（三件套栈基座，解锁 VSP 端测五件套）

- 优先级 / 预估 / 依赖：P1 / 1 天 / 无（与 PORT-SMOKE-MAC-1 并行）
- 模型分级：L3 / GLM-5.3（CAS 执行协调链语义 + 前端通道翻回判断；关键点可找参谋讨论）
- 背景事实：vsphub 代码面 mac 就绪——决策侧 2026-09-19 实测 `GOOS=darwin go build ./cmd/vsphub` exit 0；A2 已移植其 SHM 读段 darwin 路径；B3 已给 start_page 预置 mac 候选（`agent/bin/vsphub`、`agent/vsphub`）。缺口=从未构建放置、从未起栈验证。agent 注册面：`-vsp-hub-url` / `VIT_AGENT_VSP_HUB_REQUIRED`（默认 false，required 模式带 30×2s 有界重试，commit 7793927）。
- 目标：
  1. **构建放置**：mac 构建 vsphub → `agent/bin/vsphub`（gitignore 落点，不入仓；sha256 入工件）
  2. **三件套起栈**：内核 + vsphub + agent（A5/C2 冒测模式：真实起停、端口归属断言、独立临时目录运行态）——hub 端口/配置以 vsphub 源码与 PC 侧运行口径为准（开工先读 `agent/cmd/vsphub/main.go` 与既有 VSP 冒测脚本的拉起配方）
  3. **注册链验证**：agent 以 `-vsp-hub-url` 指向运行中 hub + `VIT_AGENT_VSP_HUB_REQUIRED=true` required 模式注册成功（有界重试语义实证）；hub 侧 agent 在册证据（hub 查询端点或日志）
  4. **最小 VSP 面探活**：经 hub 的至少一条只读请求往返（对照 codex_vsp_readonly_probe 的 GET 模式）——证明 CAS 协调链通，五件套完整端测归 PORT-SMOKE-MAC-3
  5. **前端通道翻回**（验证过后）：`vsp_realtime_adapter.gd` ws 通道在 hub 在位形态下连通（`VIT_GUI_VSP_REALTIME_WS_ENABLE=1` 强开验证）；连通成立后 fix5 默认值翻回（或改为"hub 可达即启用"的条件形态，二选一说明理由），fix6 两处安静化还原为真告警（vsphub 在位后缺失才是异常）；**Windows 臂零变化**
  6. **产出**：三件套冒测入口（扩展 `scripts/dev_agent_smoke_mac.sh` 加参数或新增同模式脚本，AGENTS §5 复用体系）+ SMOKE_TESTS.md 补条目
- 文件域：`scripts/`（冒测入口）+ 前端仓（fix5/fix6 翻回，单独 commit 供审计区分）；不改 `agent/` 源码（vsphub 运行时缺口若需改代码 → 域外上报另卡）；`VitApp/` 零触碰
- 验收标准：①三件套冒测 exit 0（run ID/各段日志/端口归属/停栈记录）；②required 注册成功 + hub 在册证据；③VSP 只读往返证据；④ws 通道连通证据 + fix5/fix6 翻回 diff（Windows 臂逐字保留）；⑤两件套回归（不带 hub 的 A5 形态仍 exit 0，legacy 通道无损）；⑥工件目录齐备（含 vsphub sha256/HEAD/dirty）
- 停止条件：vsphub mac 运行时崩溃/端口绑定失败 → 按类型取证转 blocked（SHM/平台缺陷另开修复卡）；required 注册两败同断点 → §8 止损转卡；发现必须改 agent/vsphub 源码才能跑通 → 域外上报；前端 ws 翻回导致 Windows 面风险 → 不翻回、保留 env 强开并上报裁定
- 领取：2026-09-19 18:52 CST / origin/main=0f554357f054ccf4efb9dee914659cc8cebdac6e / 分支 port/vsphub1-mac-runtime（独立 worktree）
- 回执：（阶段回执 2026-09-19 19:4x CST，实施完成、两项验收段待端口释放）
  - **交付①（构建放置）**：`agent/bin/vsphub`（主仓 gitignore 落点，start_page 首选候选）就位，darwin/arm64，sha256 `77fdffe9388585b821363b4a2245639b10d25d84ec5c840a7ac30175edce975c`，构建自 HEAD=e840279（源码零改动，GOOS 原生 go build exit 0）。
  - **交付②③④（三件套/注册/只读往返）——Stage 1 冒测 exit 0**（`--external-kernel` 形态，run `vsp_hub_three_piece_mac_20260919-190510`，summary 全 gate 绿，工件 `/private/tmp/vsphub1-artifacts/stage1_workdir/`）：
    - 有界重试两向实证：负例先导段 required=true 无 hub → 29 条 pending 后 `attempts=30` 自停（60s，exit 0）；正例 agent 先起捕获 pending 重试窗 → hub 起 2s 内注册成功（`VitAgent registered ... session_id=sess_kernel_…` 内核签发，证明 hello 打穿 hub→kernel）；hub `/vsp/status` 在册 role=agent client_id=vit.agent.official。
    - 只读往返（codex probe 语义 10 检查全过）：health/能力断言/hello(extension,read_only)/register/在册/state.snapshot 经 hub 抵内核（payload ok+内轨 id 零泄漏）/event.poll/unregister/除名。
    - 环境注记：卡领取时 5555/7878 被用户 10:44 编辑器会话遗留栈（pid 89803/89804）持有且 Godot 编辑器（pid 97623，18:15 起）存活——按 §9 未动，取证 `/private/tmp/vsphub1-artifacts/leftover_stack_forensics.txt`，采 external-kernel 形态（复用内核只连不占，收尾断言 untouched）。
  - **交付⑤（ws 通道+翻回）**：ENABLE=1 强开连通成立（ws state=open、transport=vsp.hub.websocket、内核签发会话、ws 帧 527ms、零 HTTP 回退）；翻回=**fix5 默认值翻回**（移除 macOS 默认关分支，对 9987c5a^ 逐字一致；理由：根因（hub 缺位）已由①消除，条件形态会留永久双臂分叉与 Windows 面新风险；ENABLE 开关功成身退，WS_DISABLE 安静开关保留）+ fix6 两处还原无条件 push_warning（对 b55c2b7^ 逐字一致）——Windows 臂零变化由逐字一致证明。复验：两文件 check-only exit 0；**翻回后不设 ENABLE** 重跑 ws 活体探针 exit 0（默认即通，0 条 TCP no-delay 告警）。前端仓单独 commit `08ecdb3`（branch port/b7b-cef-webview-host，本地仓无远端）；编辑器内手测归用户闸门。
  - **交付⑥（冒测入口+条目）**：`scripts/vsp_hub_three_piece_smoke_mac.sh`（941 行，含 --external-kernel/--ws-probe/--skip-bounded-fail）+ SMOKE_TESTS.md 条目；实现 commit `bb26c54`（port/vsphub1-mac-runtime 已推 origin）。
  - **待补（端口释放后，两命令即成）**：验收①全栈形态（自有内核起停+端口归属+停栈记录，`vsp_hub_three_piece_smoke_mac.sh --ws-probe`）与验收⑤两件套回归（`dev_agent_smoke_mac.sh --kernel-bin <主仓内核>`，不带 hub 的 A5 形态）。**[等待用户:关闭 Godot 编辑器（pid 97623）与其 10:44 内核/agent 遗留栈（pid 89803/89804）以释放 5555——内核 ZMQ 端点为编译期常量无法并行]**
  - 端测覆盖边界声明：本卡全部证据来自真实三进程栈（外部内核形态）；未覆盖=自有内核形态的起停/端口归属段与 A5 两件套回归（上述待补项）；前端面 headless 探针已覆盖 ws 数据通道，编辑器 UI 面未覆盖（用户手测闸门）。
- 验收：

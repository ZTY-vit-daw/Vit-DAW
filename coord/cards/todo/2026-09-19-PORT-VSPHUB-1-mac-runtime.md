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
- 领取：
- 回执：
- 验收：

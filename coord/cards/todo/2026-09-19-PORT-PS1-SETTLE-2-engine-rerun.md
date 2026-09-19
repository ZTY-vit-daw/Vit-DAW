# PORT-PS1-SETTLE-2：强引擎 F⑤ 重跑收口（SETTLE-1 拆卡，用户裁定①）

- 优先级 / 预估 / 依赖：P1 / 0.25 天 / 依赖 PORT-PS1-SETTLE-1（已合入 main，settle 驱动在位）
- 模型分级：L1 / GLM-5.3 flash 可接（操作卡：引擎切换+复跑+取证，无代码改动预期）
- **背景（SETTLE-1 ruling + 用户裁定 2026-09-19）**：settle 驱动三轮实证有效，run2 证明 completed 路径断言全过；唯一阻塞是 deepseek-v4-flash 在 vocal focus 续跑轮 ~2/3 概率给 needs_experiment 终局被 admission gate 结构性拒→诚实 failed。用户裁定①：配强引擎重跑 F⑤ 至绿。
- **前置 `[等待用户: ~/.vit/config.json 配置强引擎]`**：现配置仅 rightapi.ai/deepseek-v1 单引擎（决策侧亲核形态：顶层 baseUrl/apiKey/defaultModel）。用户需提供第二个更强引擎（如 GLM 系）的 OpenAI 兼容端点与 key，或明示临时切换。**执行侧不猜引擎凭据**；配置变更前备份原 config（前后 sha256 入回执），临时切换跑完还原。
- 目标：
  1. 强引擎下 F⑤ 全脚本权威 run **exit 0**（断言零改动——PS1-SYNC-1 契约 + settle 驱动已在 main；跑法同 SETTLE-1 三轮：端口栈核对→起栈→run）
  2. 引擎事实入回执（defaultModel/endpoint 域名与 key 零入工件零入仓，同 JOURNEY 口径只记布尔与名称）
  3. 若引擎切换为临时：config 还原核验（前后哈希一致）
- 文件域：**零代码改动预期**；config 备份工件落 `coord/runs/ps1-settle-2/`；如需注记引擎前提，SMOKE_TESTS PC ⑤ 条目 Notes 一行
- 验收标准：①F⑤ 权威 run exit 0（§8 ≤3 轮、两败止损）；②引擎事实与 config 前后哈希入回执；③工件（run ID/日志/退出码）
- 停止条件：强引擎仍 failed（needs_experiment 或其他）→ 取证上交（说明存在非引擎因素，不追加轮次）；引擎连不通/限流 → 环境中断记录；发现需改 agent/内核/ps1 代码才能绿 → 立即域外上报；环境中断如实记录
- 领取：
- 回执：
- 验收：

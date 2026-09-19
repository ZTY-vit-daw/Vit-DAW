# PORT-PS1-SETTLE-1：PC ps1 chat settle 等待补齐（F⑤ 跑绿收口，PS1-SYNC-1 拆卡）

- 优先级 / 预估 / 依赖：P1 / 0.5 天 / 依赖 PORT-PS1-SYNC-1（已合入 main，断言已升级）
- 模型分级：L2 / GLM-5.3（chat 驱动语义对齐 + 真实栈跑绿）
- **背景（PS1-SYNC-1 ruling 裁定 1，两轮工件+日志实证）**：PC `run_vit_product_path_smoke.ps1` 的 `Invoke-AgentChat` 为单发 POST 无 settle 等待；vocal focus 回合在升级后终态路由（`ccb.observation_catalog`）下确定性超 max_turns 切片进 durable continuation（两轮 `stop_reason=limit_reached`/`completed_steps=1`/`continuation.arm durable=cont_*` 实证）。mac 侧同现象已以 `--chat-settle-seconds`（默认 300s）解决并文档化于 SMOKE_TESTS.md mac 条目；PC 体系内 settle 先例：`scripts/d1_audition_gap_smoke.ps1` 事件轮询+deadline 形。
- 目标：
  1. PC ps1 `Invoke-AgentChat`（及必要调用点）补 chat settle 等待：等待回合收敛到终态（非 `limit_reached` 切片态）或超时（默认 300s，可参数化）再返回，语义对齐 mac `--chat-settle-seconds`
  2. F⑤ 全脚本权威 run **exit 0**（vocal focus 回合含升级后别名组断言全绿）
  3. SMOKE_TESTS.md PC ⑤ 条目补 settle 语义描述
- 文件域：`scripts/run_vit_product_path_smoke.ps1`（函数级最小改动）；`scripts/SMOKE_TESTS.md`；**不动** `_mac.sh`（mac 已有 settle）与其他 ps1/探针
- 验收标准：①diff 函数级最小（不重构不动其他段）；②F⑤ 权威 run exit 0（§8 ≤3 轮、两败止损，AGENTS §8/§9 工件纪律）；③SMOKE_TESTS.md 同步；④工件（run ID/日志/退出码）
- 停止条件：settle 等待就位后 vocal focus 仍不收敛（`limit_reached` 持续）→ 取证上交（可能涉 agent 续跑/settle 语义，域外）；需改 agent/内核代码才能绿 → 立即域外上报；环境中断如实记录
- 附注（PS1-SYNC-1 ruling 备案）：A② legacy 模板耦合 `VitApp/Workspace/default_project.xml` 须含 VIT_AUDIO_SETTINGS，本卡不处理，跑 A 脚本无关
- 领取：
- 回执：
- 验收：

# PORT-PS1-SYNC-2：断言大修（⑤ schema 族 + needle 扩容 + settle 窗口），吸收 SETTLE-2

- 优先级 / 预估 / 依赖：P1 / 1 天 / 无（与 PORT-SMOKE-MAC-3 跨机并行）
- 模型分级：L2 / GLM-5.3（断言语义对实际 schema 的重设计 + 双端同步）
- 背景（三源汇流）：①SMOKE-MAC-2 取证——⑤ kernel-prepared/feature_snapshot 断言族**双端从未真正执行过**（PC 绿轮走 else 分支），与实际快照 schema 三层错配（rows=每 request 历史轨迹行、全称 ready vs per-track ready、DAD L3 行 provenance 族无 request_id）；②2026-09-20 决策侧 pro A/B 实验——**flash 短探针 15/15 能力充足，"换强引擎"前提被推翻**（pro 无能力溢价仅 6 倍延迟；配置已留 flash），④⑥⑦-product-path 的真实挡路者=needle 词面窄（"执行/继续"两词，PC 原版设计，ps1:571-578 与 mac 同款）+ settle 窗口；③SETTLE-2（强引擎重跑）前提失效，本卡吸收其 F⑤ 跑绿目标，SETTLE-2 作废。
- 目标（四段）：
  1. **⑤ 断言族重设计（双端）**：对 `run_live_material_observation_smoke.ps1` + `_mac.sh` 的 kernel-prepared/feature_snapshot 断言块做整块静态审读（对照实际快照工件：SMOKE-MAC-1/2 两代工件在 `~/Documents/vit-smoke-mac1-artifacts/`、`~/Documents/vit-smoke-mac2-artifacts/`，mac 侧 per-track ready 语义修复已先行验证推进方向）——按实际 schema 一次修齐（per-track ready、provenance 族字段断言、历史轨迹行语义），ps1 与 _mac.sh 同步
  2. **needle 扩容（双端）**：observe 回复确认请求 needle 从 {执行, 继续} 扩为语义组 {执行, 继续, 确认}（命中"确认"即视为确认请求措辞——覆盖"先等你确认/请确认/待确认"形态）；涉及 `run_mix_single_tick_e2e` 与 `run_vit_product_path_smoke` 的 ps1+_mac.sh 四处 needle 定义；判据保持"needle 命中或 L3-incomplete 只读分支"结构
  3. **settle 窗口升级**：PC ps1 `Invoke-AgentChat` 的 settle 默认 300s → **720s**（SETTLE-1 实现处）；mac `_mac.sh` `--chat-settle-seconds` 默认同步 720——理由入注释（推理型模型重回合偶超 300s，实测 720 稳）
  4. **PC 跑绿**：⑤⑥④ 三件 ps1 各权威 run exit 0（吸收 SETTLE-2 的 F⑤ 目标：vocal focus 轮在扩容 needle+720s 窗下到达 admissible 终态）——flash 引擎口径（不换模型）
- §8 纪律：每件 ≤3 轮、同断点两败止损取证上交
- 文件域：`scripts/`（⑤⑥④ 的 ps1 与 _mac.sh 各处 needle/断言/默认值 + SMOKE_TESTS.md 相应描述）；`agent/`、`VitApp/`、前端仓零触碰
- 验收标准：①⑤ 断言块重设计 diff（含静态审读笔记：新断言↔实际 schema 字段对照表）；②needle 扩容 diff（双端四文件）；③settle 默认值变更 diff；④PC ⑤⑥④ 三件 exit 0 工件（run ID/日志）；⑤SMOKE_TESTS.md 同步
- 停止条件：⑥ vocal focus 在新 needle+窗口下仍两败同断点 → 取证上交（届时红因排除脚本面后指向 agent 续跑语义，另开 agent 侧卡）；⑤ 新断言与 schema 仍有未预见层 → 逐层记录按轮推进；需改 agent/内核 → 域外上报
- 领取：2026-09-20 18:15 / PC 执行侧（GLM-5.3） / origin/main=4826248379367db4a9bb99b11178e2740a1fd27d / 分支 port/ps1-sync-2 / 领取时工作树：M VitApp/Workspace/default_project.xml（烟测已知保留项）、D coord/cards/doing/2026-09-19-PORT-PS1-SETTLE-1-chat-settle-driver.md（决策侧遗留，不代提交）、?? coord/runs/ + extension 构建产物 + godot-cpp/（均域外不动）
- 回执：
- 验收：

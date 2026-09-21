# PORT-WL-1：mac 实验白名单机器采纳 + ⑤ 复跑判定（放大器假说检验）

- 优先级 / 预估 / 依赖：P1 / 0.5 天 / 依据 [2026-09-21-pc-45-reply.md](../reports/2026-09-21-pc-45-reply.md) §五处置意见（白名单先行→复跑→pro 二线）；与 PC 侧 PORT-PS1-N5-1（已派）跨机并行
- 模型分级：L3 / GLM-5.3（字段实测映射 + 假说检验判定）
- 背景：PC 清点⑤ vocal focus 09-20 后 **7/7 提案面停车**（flash+白名单在位）vs mac **0/5**（flash+白名单缺失）；mac ⑤ R5 双跳前两跳全绿、死在 `~/.vit/free_state_experiment_plugins.json` 缺失（hop-2 `d1_execution_blocked`）。机制假说：实验路径死→提案无门→强化重试→无可采纳终态→诚实终止族。本卡=补白名单+复跑，一次检验假说与修复问题。
- 目标：
  1. **白名单构建**：按 PC 参照 schema（`vit.free_state_experiment_plugins.v5`，七族 `static_eq/broadband_compression/de_esser/transient_shaper/limiter/gate_expander/multiband`；字段 `plugin_name/manufacturer/format/plugin_identifier(VST3-名-hash)/plugin_path/bands[](center_hz+gain_param_id_ch1/ch2)`）构建 mac 机器态文件——**数据源=mac 自己的实测**：主体身份/path/identifier 取 C2 白名单草稿+`~/.vit` 语义索引（跨端逐字节一致已证）；bands 的 center_hz/gain_param_id 从 C3 reprobe 工件的参数面快照（`~/Documents/vit-c3-artifacts/*/probe/`）实测提取；**不得手编**——无法实测的字段如实标注留空并说明
  2. **落盘与校验**：写 `~/.vit/free_state_experiment_plugins.json`（机器本地，AGENTS §10）；schema 校验 + agent 启动读取无错；逐字段溯源表入工件（字段值↔实测来源）
  3. **⑤ 复跑判定（假说检验）**：白名单就位后 `run_vit_product_path_smoke_mac.sh` ×5（MAC-4 同口径：≥4/5 达标、失败逐轮归类）——**假说预测：停车率显著回升**；顺带跑 ④×5（同受提案路径影响的姊妹件）
  4. 结果三去向：达标 → ④⑤ 收口+端测收官声明（另行裁定）；停车率回升但 <4/5 → 残差归因上交（届时 pro 对照升一线，判别力已干净）；无变化 → 放大器另有其因，全部取证上交
- 文件域：`~/.vit/`（机器本地白名单）+ `scripts/`（如需构建/校验脚本，最小化）+ 工件目录；`agent/`、`VitApp/`、前端仓零触碰
- 验收标准：①白名单文件+逐字段溯源表（实测来源可回指）；②agent 读取校验证据；③④⑤ 各 ×5 运行账目（run ID/退出码/逐轮归类/成功率）+ 与 MAC-4 基线的对照表；④假说判定结论；⑤工件齐备（LLM key 零入工件，flash 口径）
- 停止条件：bands 参数面实测发现 C3 工件不含所需 param_id 粒度 → 改用 pluginprobe/reprobe 现跑提取（C3 驱动在 main），仍不行则上交；白名单就位后 agent 读取报错 → 取证上交；复跑暴露 mac 脚本新缺陷 → 修补列明该轮作废重跑
- 领取：
- 回执：
- 验收：

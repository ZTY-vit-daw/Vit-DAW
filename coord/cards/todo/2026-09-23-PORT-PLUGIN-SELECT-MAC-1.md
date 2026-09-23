# PORT-PLUGIN-SELECT-MAC-1：mac 白名单 v6 升级四 EQ 候选 + LLM 自选验证腿

- 优先级 / 预估 / 依赖：P1 / 0.3-0.5 天 / **FIX-PLUGIN-SELECT-1 合入**（v6 schema+membership 准入+有界披露）；操作序列与验证口径已由实现侧备好：`coord/runs/plugin_select1_artifacts/mac_leg_handoff.md`
- 模型分级：L1 / GLM-5.3 flash 可接（数据扩展+驱动+记账，断言零改动）
- **执行侧（mac 侧卡）**：白名单与工件均机器本地（`~/.vit/free_state_experiment_plugins.json`、`~/Documents/` 工件根）；回执写回 D:\Vit_DAW coord（记两端 HEAD）
- **背景**：PLUGIDENT 已通（EQ 全链 mac 首次走通）；PLUGIN-SELECT 已合入——v6 每族候选列表+候选>1 时模型有界披露自选+membership 准入。mac 已有 **4 个 PCA 认证 EQ 主体**（Q10 Stereo + API-550A + API-560 + EMO-F2，EQ-1 期间晋升）——正是多候选自选的现成试验场。
- 目标：
  1. **白名单 v6 化**：`static_eq` 升为数组并填入四个认证主体条目——条目字段与 v5 逐字段同（plugin_name/manufacturer/format/plugin_identifier/plugin_path/bands[center_hz+gain_param_id_ch1/ch2]）；**四源零手编纪律保持**：Q10 条目 EQ-1 已建可直接入列；API-550A/API-560/EMO-F2 若 bands 参数面数据不全（认证覆盖≠带结构参数面完备），先跑同款 probe 补测，实在不可及的主体**如实少列**（候选 2-3 个也足以验证自选），不得手编凑数；其余五族维持 v5 单对象形态（loader 兼容阀=行为逐字节不变）
  2. **加载与准入校验**：agent 启动读 v6 无错 + overlay go test（loader v5/v6 双形态+四条目 static_eq admission + 其余五族回归）
  3. **LLM 自选验证腿（本卡核心交付）**：按 `mac_leg_handoff.md` 操作序列跑——模型在候选披露面看到 ≥2 个 EQ 主体 → 提案携带所选 plugin_identifier → 准入 membership 放行 → D1 绑定所选条目 → 装载/写参/回读走完；**断言要点**：披露出现且仅含结构性字段（name/manufacturer/format/identifier+能力面，无 path/参数 id）、所选 identifier 进入计划 args、非成员 pin 拒绝路径 spot 一枚（如可行）
  4. 工件：RUN_LEDGER 式逐轮账目（run ID/退出码/所选主体/披露形态），LLM key 零入工件
- §8：真栈 1-2 轮（存在性+自选实证口径）；模型若两轮选同一主体=合法（自选≠必选不同），记录选择分布即可；同断点两败止损上交
- 验收：①v6 白名单+溯源（每条目↔实测/认证来源可回指）②overlay 绿 ③自选腿 exit 0 且证据链完整（披露→选择→准入→绑定→写链）④回执记两端 HEAD+白名单文件版本/哈希
- 停止条件：候选披露面不出现（>1 候选在列仍无披露）→ 取证上交（可能是 PC 实现缺陷，域外）；四主体参数面均不可及 → 如实降级为现有五族任一族的双候选试验（需补测该族第二主体）
- 领取：
- 回执：
- 验收：

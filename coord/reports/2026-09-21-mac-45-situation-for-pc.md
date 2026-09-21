# 情况说明：mac 端测 ④⑤ 不达标全貌（供 PC 侧对照其实验记录）

- 作者：mac 决策侧会话｜2026-09-21｜性质：跨端情况通报+对照请求（非卡片非裁定）
- 背景：PORT-SMOKE-MAC-4（收官统计卡，④⑤⑥⑦ 各 N=5）结果 ⑥ 5/5 ✓、⑦ 4/5 ✓、**④ 2/5 ✗、⑤ 0/5 ✗**（有效轮，≥4/5 为达标线）。④⑤ 上交处置中。ruling：`coord/rulings/2026-09-20-SMOKEMAC4-pass.md`；完整账目 mac 侧 `~/Documents/vit-smoke-mac4-artifacts/RUN_LEDGER.md`。

## 一、引擎口径（谁在跑——先钉死这个）

- MAC-4 的 20 个有效轮**全部 deepseek-v4-flash**：执行侧发现 config 曾滞留 pro，逐轮以 `VIT_AGENT_LLM_MODEL=deepseek-v4-flash` env 强制口径，各轮遥测 `model` 字段实证。**上述成功率全是 flash 的数字**。
- mac 当前 config 已恢复 flash；`config.json.bak-flash-20260920` 备份在。
- 中转站（rightapi.ai/deepseek/v1）健康：/models 200、双后端剖面可分（延迟/reasoning token 分布不同）。

## 二、失败形态（不是空输出，不是乱码——是"终态分支选错"）

8 个有效红轮全部属"LLM 诚实终止族"，内容完整且协议化：

1. **能力边界声明**（④×2，与 MAC-2 G 轮逐字同源）：「处理完成 任务已到达明确的能力边界；没有把能力不足解释为改善完成」——有内容的诚实拒绝；
2. **中间态标记**（②⑤ 各见）：「已完成 ccb_observation_request」——重回合未在 settle 窗口内收敛，抓到的是进行时状态；
3. **无可采纳终态**（⑤×2）：agent 终审 `terminal turn produced no admissible final decision after one strengthened retry`；
4. **done 不停车**（⑦ 传导 1 例）：模型输出**完整 EQ 提案文本**（「在主唱轨约 3.2 kHz 处小幅提升……」）但 `stop_reason=done` 而非停 `needs_confirmation` 提案面——内容完美，终态框架错；
5. **⑤ R5 复合**（非模型错）：双跳前两跳全绿（提案面+工具面都到了），死在 `~/.vit/free_state_experiment_plugins.json` mac 未配置 → hop-2 `d1_execution_blocked`（白名单采纳迷你卡已在计划）。

**一句话定性：模型不缺内容不缺诚实，缺"该停车时停车"的终态选择可靠性。**

## 三、pro 的履历（样本小，方向一致，未在 ⑤ 测过）

- ④×3（09-20 A/B，旧断言体系）：**3/3 模型层停车正确**（全达 waiting_confirmation，回复标准）；3 轮未全绿原因均为脚本侧（bash bug/settle 窗/needle 词面）——**三者现已全部修复**（391eb6d / settle 720 默认 / SYNC-3 needle 扩容+双跳断言）；
- 短探针电池（同渠道直连）：flash 15/15，pro 14/15——**pro 无短任务能力优势，仅 6-13 倍延迟**；
- **pro 在 ⑤ 零样本**。④ 的 pro 3/3 vs flash 2/5 是目前唯一对照信号。

## 四、请 PC 侧对照/补充的事项

1. **PC 的 `~/.vit/config.json` defaultModel 是 flash 还是 pro？**（尚未有人回答；若 PC 历史绿轮用的是 pro，则双端"引擎配置差异"直接成立）
2. **PC 历史实验记录里 ④⑤（或同型 vocal-focus/product-path 回合）的停车率**：SYNC-3 的 PC 绿轮是单样本。请翻 PC 侧实验记录统计——`waiting_confirmation` 停车 vs 上述诚实终止族的历史比例。若 PC 历史停车率高 → 提示 mac 侧存在未找到的放大器（我们将再挖）；若方差相近 → 模型共有现象坐实；
3. （可选但推荐）**PC ④⑤×5 flash 统计**（镜像 MAC-4 口径，断言已双端同步）——一次把"引擎差/双端差"两个问题测干净；
4. 白名单：PC 侧 `free_state_experiment_plugins.json` 的现行形态可作 mac 采纳迷你卡的 schema 参照（mac 侧有 C2 的 24 主体草稿，但缺 pluginprobe 实测机器态）。

## 五、决策状态（mac 侧）

④⑤ 处置三选项待用户裁定：a=容忍方差收官（链路已证）/ b=**pro 对照实验 ⑤×5（推荐先行）**，可扩为"mac pro + PC flash"双维对照 / c=agent 工作流强化（大卡）。白名单迷你卡无论选哪个都会出。

## 六、关键指针

- 统计与逐轮：`~/Documents/vit-smoke-mac4-artifacts/`（RUN_LEDGER.md / SUCCESS_SUMMARY.md / driver_logs/）
- 脚本面三修复：main `391eb6d`（bash）+ SYNC-2/3 系列（needle/settle/双跳断言）+ MAC-4 三 commit（bcfceeb/3217028/d180bb9）
- 早期 A/B 工件：`~/Documents/vit-smoke-mac1-artifacts/`（pro 三轮 run 115130/115619/120225）

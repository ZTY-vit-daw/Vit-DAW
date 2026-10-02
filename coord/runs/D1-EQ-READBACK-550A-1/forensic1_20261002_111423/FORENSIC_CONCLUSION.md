# D1-EQ-READBACK-550A-1 取证结论（forensic1_20261002_111423）

## 运行身份

- run：`~/Documents/vit-d1eq550a-artifacts/forensic1_20261002_111423/`
- 内核：本卡 worktree（port/d1-eq-readback-550a-1 @ 08fdfe9，VitApp 代码=main HEAD）cmake Debug 构建，
  sha256 `fc77dd75766b3603bfde1b0f44c733a5f776ec08b2c5b9ad130fe49a277e9b5e`
- 栈：内核独占（无 agent/Godot），ZMQ 直发 legacy 面（zmqprobe，与 agent 同 zmq4 库）；5555 端口本 run 单所有权
- 工程：仓内 `VitApp/Workspace/default_project.xml` 复制进隔离 fake root（track 1007）
- 插件：scan_plugins 全量 VST3 目录后按 identifier 装载——550A Stereo plugin_id=1040，Q3 Stereo plugin_id=1041
- 过程注记：probe 脚本 scan 终态字段笔误（state→status）致等待循环滞留，扫描完成后其余步骤由 drive_manual.sh/drive_sweep2.sh
  在同一内核上手动驱动（工件同目录同命名）；脚本已修正备复跑

## 定性：读回形态不匹配（写参已生效），非"写参未生效"

### 550A Stereo param 2（Mid Gain，白名单 400Hz 带）是 11 档步进控制

内核面元数据（`get_plugin_parameters`，02/04 号工件）：

- `is_discrete: true`，`num_steps: 11`
- `min/max`（原始域）：0.0 / 1.0（**不是 dB 域**）
- `display_domain_candidate`：缺失（无 linear dB 候选）
- `discrete_labels`（内核 `getValueForState` + 文本，**全表带可解析 dB 文本**）：

| index | normalized（内核步位，float32） | label |
|---|---|---|
| 0 | 0.0 | -12 dB |
| 1 | 0.100000001490116 | -9 dB |
| 2 | 0.200000002980232 | -6 dB |
| 3 | 0.300000011920929 | -4 dB |
| 4 | 0.400000005960464 | -2 dB |
| 5 | 0.5 | 0 dB |
| 6 | 0.600000023841858 | +2 dB |
| 7 | 0.699999988079071 | +4 dB |
| 8 | 0.800000011920929 | +6 dB |
| 9 | 0.900000035762787 | +9 dB |
| 10 | 1.0 | +12 dB |

dB 映射**非线性**（探针 5 点样本：0→-12, 0.25→-4, 0.5→0, 0.75→+6, 1→+12）——API 550A 硬件步进复刻。

### 吸附实测（drive_sweep2.sh，sweep2_results.tsv）

| 写入（normalized） | 即时回复 norm | surface 读回 norm | 读回文本 |
|---|---|---|---|
| 0.4166666666666667 | 0.400000005960464 | 0.400000005960464 | -2 dB |
| 0.45 | 0.400000005960464 | 0.400000005960464 | -2 dB |
| 0.5833333333333334 | 0.600000023841858 | 0.600000023841858 | +2 dB |
| 0.4（在格） | 0.400000005960464 | 0.400000005960464 | -2 dB |
| k/10 各档（k=0..10） | 精确落档（float32 回显） | 同左 | 对应档文本 |

**写入生效**：离格值吸附到最近可达档位，物理结果正确；即时回复与 surface 读回一致。

### 对照：Q3 Stereo param 12（Band 3 Gain）连续控制

- `is_discrete: false`；探针线性 ±18 dB（0→-18, 0.25→-9, 0.5→0, 0.75→+9, 1→+18）
- 写 0.4166666666666667 → 读回 0.416666656732559（float32 回显，差 ~1e-8 << 1e-4）→ 校验通过

### 失败机理还原（d1 delta 路径，与 8 轮归因表吻合）

1. d1 static_eq 带 `target_semantics: delta_db` → `planDeltaChannels` slope 探针：±0.25 归一化探写读到
   （0→0 dB, +0.25→+6 dB）→ slope=24 dB/norm
2. target -2 dB → RequestedNormalized = 0.5 - 2/24 = **0.4166667（离格）**
3. 内核写入 → 插件吸附到 0.4 档（**物理正确：-2 dB**）
4. agent 校验 `|0.400000005960464 - 0.4166666666666667| ≈ 0.0167 > 1e-4` → **applied_unreconciled**
5. 步进控制在连续归一化等式（1e-4）下**永远不可满足**（除非碰巧在格）——Q3/Q4 连续控制 float32 回显 ~1e-8 恒过

## 修复方向（agent 侧，executionports 域内闭环）

写参链路无缺陷，无需动内核。修 agent 的规划/校验形态：

1. **规划吸附**：参数面报 `is_discrete && num_steps>1` 且 `discrete_labels` 带可解析 dB 文本时，
   构造步进可达格（normalized=步位值, physical=label 解析）；
   - 绝对路径（eqGainToNormalized）：目标 dB 直接选最近可达档，写其步位 normalized
   - delta 路径（planDeltaChannels）：target=current+delta 选最近可达档（跳过无意义的 slope 探针），
     偏差（如 -1.5 dB → -2 dB 档，偏差 0.5 dB）在回执披露（stepped_snap 记录）
2. **校验不变式**：读回归一化等式（1e-4）原样保留——写入在格后 float32 回显 ~1e-8 恒过；
   **fail-closed 语义不变**（applied_unreconciled 仍非放行态；写参未生效/读回漂移照旧拦截）
3. delta 收敛环对步进控制跳过（最近档已是物理最优，迭代无法改进）

## 原始工件索引

- 基线参数面：`baseline_params.json`（550A/Q3 全参数行）
- 步进表+吸附：`sweep2_results.tsv`（17 例写读对比）；逐命令 JSON 在 `zmq/`
- 命令流：`zmq/NN_*_req.json` / `zmq/NN_*_rep.json` 全留存
- 过程日志：`probe_run.log`、`manual_drive.log`

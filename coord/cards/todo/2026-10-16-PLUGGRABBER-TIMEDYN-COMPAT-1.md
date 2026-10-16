# PLUGGRABBER-TIMEDYN-COMPAT-1：Waves+FabFilter 时域效果器批量词表兼容 pass（原 PLUGGRABBER-REVERB-PLATE-1 范围升级）

- 发卡：GLM 主管决策侧 / 2026-10-16 晚窗；**范围升级：用户 2026-10-16 晚窗口头指令「waves和fabfilter系列的时域效果器都得过一遍，确保他们的参数名全都兼容」**（原发卡依据=M2-EXPERIENCE-1 第四轮现场取证，[README 第四轮](../../runs/M2-EXPERIENCE-1/)）
- 派发确认：**已确认（用户 2026-10-16 晚窗范围升级指令）**；验收负责人：GLM 主管决策流
- 池序 123；目标仓库=D:\Vit_DAW；P1（用户直接需求+M2 受控写入面覆盖）/ 1-2 天
- 模型分级：L2 / GLM 执行会话（plugingrabber 语义面+pluginprobe 探测）

## 在场时域插件清单（2026-10-16 晚盘面实测，`C:/Program Files/Common Files/VST3/`）

- FabFilter（独立 .vst3）：**Pro-R（混响）、Timeless 3（延迟）** 为本卡目标件；其余（Pro-C 2/Pro-DS/Pro-G/Pro-L 2/Pro-MB/Pro-Q 3/Saturn 2/Simplon/Twin 3/Volcano 3/Micro/One）非时域不在范围；
- Waves（WaveShell1-VST3 壳 14.12/14.21/16.7/17.1 四版本在场）：壳内时域件以探针枚举为准（已知 **Abbey Road Plates**；Abbey Road Chamber/Reverb、H-Reverb、H-Delay、TrueVerb、Doubler 等以实际枚举+许可为准）；
- 顺手项（P3 性质，不阻卡）：ValhallaSupermassive（时域）。

## 首例实锤（2026-10-16 18:18 现场亲证，工件=events_round4_1817.json）

1. Abbey Road Plates Stereo（VST3/Waves 壳，id=1040）装载成功（processing=true 零 load_error），但 `plugin_grabber.inspect_reverb` 拒绝：`unresolved_reverb_surface: no provable supported reverb stage/control-path topology`（seq14，result.code=rejected）；
2. 根因锚点：decay-time 可证核心只认 decay/size/length 词族（reverb_topology.go:169-174）；`hasWet||hasCharacter` 而无 tail 核心即拒（reverb_topology.go:378-383）。Abbey Road Plates 的衰减控制是 **Damper**（板式物理阻尼）+ Plate Selector——explain_controls 实测 normalized_role=other，词表不覆盖板式族；
3. agent 侧行为正确（诚实不绕道、零原始参数写入、开编辑器+起点值建议）——本卡只补语义覆盖，**不改拒绝原则**。

## 目标

1. **兼容矩阵**：对在场目标件逐个探测真实参数面（pluginprobe 既有体系），跑 reverb/delay topology 解析，产出矩阵（插件 × 判定可证/拒绝 × 参数面 × 缺口原因），工件入 run 目录；
2. **词表补齐**：拒绝项按证据扩展词表——板式族（Damper/Plate→tail/decay）、FabFilter 命名族（Pro-R 的 Decay Time/Predelay/Wet 等、Timeless 3 的 delay 词族含 reverb_delay_topology 的延迟面）——**须带置信度/证据等级语义，保持「decay/delay-time 可证核心，character 参数不得冒充核心」红线**（reverb_topology.go:12-16 文件头注释）；
3. 可证后受控写入路径放行；不可证面照旧诚实拒绝（拒绝码语义不变），矩阵中留明确词表外原因。

## 文件域

agent/internal/workflows/plugingrabber/reverb_topology.go + 同包测试；probe 复用 pluginprobe 既有脚本体系（不另起机制）；不动 PCA 准入门、插件装载面与 agentloop。

## 验收标准

- 兼容矩阵工件（全部在场目标件覆盖，零遗漏或明确「许可未装/枚举失败」原因行）；
- 单测：板式 fixture（Damper+Wet+Predelay，无 decay/size/length 词）→可证 tail 核心（置信度语义在卷）；character-only 面照旧 `unresolved_reverb_surface`；既有 reverb_delay_topology_test.go 零回退；
- 真栈（栈空闲时可领）：Abbey Road Plates 同话术 → inspect_reverb 放行 + 受控参数写入落地（readback 在卷）；Pro-R 抽验一例放行写入；
- `go test ./... -count=1` 0 FAIL + gofmt 净（改动包）。

## 停止条件

- Waves 壳内插件枚举或参数读回不稳定（param_id string/numeric 混排、四版本壳冲突）→ 壳取证拆独立小卡，矩阵如实记录已覆盖件，不硬凑；
- 取证发现 Damper→decay 映射无法给出可辩护的置信语义 → 上交裁定，不硬入词表。

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / run ID / 兼容矩阵路径 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）

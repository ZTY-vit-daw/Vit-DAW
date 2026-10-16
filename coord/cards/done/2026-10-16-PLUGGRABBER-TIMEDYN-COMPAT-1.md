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

- 领取：（2026-10-16 18:54 +0800 / origin/main=2338dba08450bfadd2a715fd17d9fda9b3aaacc7 / GLM-5.3 L2 PC 执行会话 ZCode（本会话专场） / 实现 port/pluggrabber-timedyn-compat-1 + 协调 coord/pgtdc-compat-claim / worktree D:/Vit_DAW_wt_pgtdc_compat / 入池+领取提交见本提交及前一提交；领取时核对：卡在 todo 未入远端（发卡侧留盘，按 cd68da79 先例原样入库），doing 三卡文件域零交集（LOUDNESS-B=agent-loop 面、FE-CEF=前端仓、MIX-SENDRETURN=设计取证），PC-RUNTIME-STACK 被 LOUDNESS-B-TRIGGER 占用中——本卡真栈腿先等待释放再登记）
- 回执：（实现 port/pluggrabber-timedyn-compat-1 已推远端：c647da7c 板式 Damper 族→tail+plate_damper_family 分级(0.90)+pedal 守卫+Plate Selector=character；4076900a run2 证据驱动补齐（reverb time→direct/reverb type→character/tempo+bpm→sync/fdbk/单位限定 delay 词/裸 delay 词带守卫）；65c44a3d 工件入库。**run3=pgtdc_20261016_205639 compat_verify_pass exit 0**（run1=203552 驱动面缺陷轮：误走 agent 装载面被 PCA 门拒+家族误排 12 件；run2=204214 词表证据轮：61 件全量参数面暴露 5 族缺口——三轮失败各不同签名未触止损；三轮 LLM=0）。兼容矩阵=coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/README.md（23 族 61 件全覆盖：42 可证+19 原因行——IR 系 11=卷积衰减固化于 IR 采样/Clarity 4=非时域清理工具/MannyM Reverb 3=无衰减控制+内嵌 Comp/Supermassive=延迟混血 delay 面可证；零许可未装零枚举失败，四壳由内核去重呈报无歧义）。验收命令：go test ./... -count=1 0 FAIL+gofmt 净（改动文件，仓库 CRLF 检出伪差除外）+真栈脚本 scripts/pgtdc_compat_verify.ps1 exit 0。真栈抽验在卷：Plates inspect 放行（conf 0.9/tail_evidence=plate_damper_family）+Damper enum 步进受控写 0.2→0.0 exact+独立 readback；Pro-R（Decay Rate %域）display 写 100%→55% exact。**端测边界声明**：①未启 Godot GUI——agent HTTP+内核 ZMQ 面驱动（M2-EXP-TIMEDYN-GAP-1 先例边界），本卡零 webui/渲染面改动；②装载走内核 rack_add_node 探针面（M2-TIMEFX-2 先例），PCA 产品装载门未触碰未绕过（run1 证明其拒绝语义正常）；③FXM 差分探针挂载于 apply 但未断言（抽验轨道无音频素材，断言面=apply 回执+参数 readback，如实声明）；④61×原始内核参数面（12MB/run）留盘 worktree D:/Vit_DAW_wt_pgtdc_compat 待验收后处置，仓库入 compact 面+inspects 全量；⑤停止条件未触发（Waves 壳枚举与参数读回全程稳定）。栈占用/释放记录 17533832）
- 验收：（裁定文件 / 验收 commit）

# PLUGGRABBER-TIMEDYN-COMPAT-1：Waves+FabFilter 时域效果器批量词表兼容 pass

- 卡：`coord/cards/done/2026-10-16-PLUGGRABBER-TIMEDYN-COMPAT-1.md`（池序 123）
- 执行：GLM-5.3 L2 PC 执行会话 ZCode；worktree `D:/Vit_DAW_wt_pgtdc_compat`；实现分支 `port/pluggrabber-timedyn-compat-1`（词表落地 4076900a）
- 依据：用户 2026-10-16 晚窗口头指令「waves和fabfilter系列的时域效果器都得过一遍，确保他们的参数名全都兼容」；首例实锤=seq14（Abbey Road Plates `unresolved_reverb_surface` 拒绝，M2-EXPERIENCE-1/events_round4_1817.json）
- 终版 run：`pgtdc_20261016_205639` **compat_verify_pass exit 0**（三轮历程见 §5；run3 工件=本目录，run1/run2 为原因轮留档）

## 1. 词表 diff

### reverb_topology.go（classifyReverbParameter）

| 词表条目 | 变更前 | 变更后 | 证据（run2/run3 参数面） |
|---|---|---|---|
| `damper`（token，pedal/sustain 守卫） | 命中 `damp` 子串 → damping/character | `decay`/tail，tier=`plate_damper_family`，conf 0.90 | Abbey Road Plates ×3：衰减控制=Damper（板式物理阻尼，11 步离散）；EMT 板式构造上 Damper 即衰减时间 |
| `reverb time`（contains） | 无匹配 | `decay`/tail，tier=`direct_wording` | Abbey Road Chambers ×3：`Reverb Time X`（衰减倍率，直名衰减时间） |
| `reverb type`（contains） | 无匹配 | `reverb_mode`/character | Chambers `Reverb Type`（腔室模型选择，非衰减证据） |
| `plate selector`/`plate select` | 无匹配 | `reverb_mode`/character | Plates `Plate Selector`（离散板型选择，**不冒充核心**） |
| decay/size/length 词族（既有） | tail，无分级 | tail，tier=`direct_wording` conf 0.96（行为不变，语义标注） | 既有词表零回退 |

- 证据分级：`ReverbStage.TailEvidence` + summary 顶层 `tail_evidence`；plate 族仅 0.90（映射依据=板式物理而非参数直名），direct 0.96；同场共存取 direct（单测 TestDetectReverbModelDirectWordingOutranksPlateFamily）。
- `damper-pedal` 守卫：`Damper Pedal`/sustain 属键盘延音踏板，不入板式族（单测）。
- 拒绝原则零改动：character-only 面照旧 `unresolved_reverb_surface`（单测）。

### delay_topology.go（classifyDelayParameter）

| 词表条目 | 变更前 | 变更后 | 证据 |
|---|---|---|---|
| `tempo`（contains，入 sync case） | 无匹配 | `sync`/timing | MannyM Delay ×2：`Delay Left/Right Tempo`（1/4 步进） |
| `delay` token + `bpm` contains | 无匹配 | `sync`/timing | H-Delay ×3：`Delay BPM`（BPM 同步时间） |
| `fdbk`（contains，入 feedback case） | 无匹配 | `feedback` | MannyM Delay：`Fdbk Left/Right`（缩写） |
| `delay` token + `ms`/`sec` token | 无匹配 | `time`/timing | MannyM Delay `Delay Left/Right ms`；H-Delay `Delay Sec`/`Delay Sec R` |
| 末词=`delay`（兜底，尾部限定词守卫） | 无匹配 | `time`/timing | Doubler2/4 ×6：`Voice1/2 Delay`（裸 delay 词=延迟时间本体） |

- 兜底守卫：`Delay Mod`/`Delay Link` 等尾部限定词不入 timing（单测 TestDetectDelayModelQualifierWordingsStayOut）。

## 2. 兼容矩阵（run3 实测，61 目标件全覆盖 0 失败；42 可证 + 19 原因行）

| 插件（在场变体，×数） | 期望族 | 判定 | 段落（roles） | 备注/原因 |
|---|---|---|---|---|
| Abbey Road Chambers（Mono / Mono/Stereo / Stereo，×3） | reverb | **可证** decay_time_reverb conf=0.96 tier=direct_wording | tail=decay; character=reverb_mode; output=dry+mix+output_gain+wet | 延迟+混响双面可证（混血） |
| Abbey Road Plates（Mono / Mono/Stereo / Stereo，×3） | reverb | **可证** predelay_decay_reverb conf=0.9 tier=plate_damper_family | tail=decay; timing=pre_delay; character=reverb_mode; output=dry+output_gain+wet | 单面（seq14 首例已修复） |
| Clarity Vx - DeReverb（Mono / Stereo，×2） | reverb | 拒绝：unresolved_reverb_surface | — | AI 去混响清理工具，非时域效果器（Main Knob/神经网络面）→ 词表外原因行 |
| Clarity Vx - DeReverb Pro（Mono / Stereo，×2） | reverb | 拒绝：unsupported_limiter | — | 同上（Pro 面带 limiter 词被辅助边界分诊） |
| Doubler2（Mono / Mono/Stereo / Stereo，×3） | delay | **可证** feedback_delay conf=0.96 | timing=time; output=dry+wet; feedback=feedback | 单面 |
| Doubler4（Mono / Mono/Stereo / Stereo，×3） | delay | **可证** feedback_delay conf=0.96 | timing=time; output=dry+wet; feedback=feedback | 单面 |
| H-Delay（Mono / Mono/Stereo / Stereo，×3） | delay | **可证** feedback_delay conf=0.96 | timing=sync+time; output=dry+mix+output_gain+wet; feedback=feedback | 单面 |
| H-Reverb（5.0/5.1/Mono/Mono-5.0/Mono-5.1/Mono-Stereo/Stereo/Stereo-5.0/Stereo-5.1，×9） | reverb | **可证** predelay_decay_reverb conf=0.96 tier=direct_wording | tail=decay+size; timing=pre_delay; character=damping; output=dry+mix+output_gain+wet | 延迟+混响双面可证（混血） |
| H-Reverb long（同上九变体，×9） | reverb | **可证** predelay_decay_reverb conf=0.96 tier=direct_wording | tail=decay+size; timing=pre_delay; character=damping; output=dry+mix+output_gain+wet | 延迟+混响双面可证（混血） |
| IR-L（Mono / Mono/Stereo，×2）+ IR-L efficient/full（Stereo，×2） | reverb | 拒绝：unresolved_reverb_surface | — | 卷积衰减固化于 IR 采样选择，参数面仅 4-5 参数无衰减控制 → 词表外原因行（词表无法诚实修复） |
| IR1（Mono / Mono/Stereo，×2）+ IR1 efficient/full（Stereo，×2） | reverb | 拒绝：unresolved_reverb_surface | — | 同上 |
| IRLive（Mono / Mono/Stereo / Stereo，×3） | reverb | 拒绝：unresolved_reverb_surface | — | 同上 |
| MannyM Delay（Mono/Stereo / Stereo，×2） | delay | **可证** feedback_delay conf=0.96 | timing=sync+time; output=dry+output_gain+wet; feedback=feedback | 延迟+混响双面可证（内嵌 Verb Size→reverb 面 tail=direct） |
| MannyM Reverb（Mono / Mono/Stereo / Stereo，×3） | reverb | 拒绝：unsupported_broadband_compressor | — | 无衰减时间控制（Reverb Amount=湿量）；内嵌 Comp 节 → 诚实拒 |
| FabFilter Pro-R（×1） | reverb | **可证** predelay_decay_reverb conf=0.96 tier=direct_wording | tail=decay; timing=pre_delay; character=width; output=dry+mix+output_gain+wet | 单面；衰减参数实名 `Decay Rate`（% 域） |
| RVerb（Mono/Stereo / Stereo，×2） | reverb | **可证** predelay_decay_reverb conf=0.96 tier=direct_wording | tail=decay+size; timing=pre_delay; character=damping+reverb_mode; output=dry+mix+wet | 延迟+混响双面可证（混血） |
| FabFilter Timeless 3（×1） | delay | **可证** feedback_delay conf=0.96 | timing=sync+time; character=ping_pong; output=dry+mix+output_gain+wet; feedback=feedback | 延迟+混响双面可证（混血） |
| TrueVerb（Mono / Mono/Stereo / Stereo，×3） | reverb | **可证** predelay_decay_reverb conf=0.96 tier=direct_wording | tail=decay; timing=pre_delay; output=dry+mix+wet | 单面 |
| ValhallaSupermassive（×1，P3 顺手） | reverb | 拒绝：unsupported_delay | — | 延迟混血：delay 面可证（feedback_delay：Delay_Ms/DelaySync/Feedback），reverb 面无衰减词 → 诚实拒 |

- 逐件原始工件：`pgtdc_20261016_205639/`（61×`*.inspects.json`+`surfaces_compact.json`+`matrix_summary.json`；完整内核参数面 `*.parameters.json` 留盘 worktree，验收后按需迁移）。
- 枚举：`discovery.json`（90 唯一标识 → 61 时域目标 + 范围外清单含原因）；WaveShell 四版本壳由内核 plugin_search 去重呈报（17.1 壳路径），零多壳歧义。

## 3. 单测与验收命令

- `agent/internal/workflows/plugingrabber/reverb_topology_compat_test.go`：13 例（板式族可证/与 HF Damping 共存/pedal 守卫/direct 优先/character-only 照旧拒/置信度分级/summary tier 键/Chambers reverb time/Reverb Amount 不冒充/Voice 裸词/单位限定+缩写/Sec-BPM/尾部限定词守卫）。
- `cd agent && go test ./... -count=1` → **0 FAIL**；gofmt（改动文件，CRLF 检出伪差除外）净；既有 `reverb_delay_topology_test.go` 零回退。

## 4. 真栈抽验（run3 在卷）

- **Abbey Road Plates Stereo**：inspect_reverb 放行（`predelay_decay_reverb` / conf 0.9 / `plate_damper_family` / generation rv1t1_d3e5e2be…）；Damper 受控写入 `enum_label="0 "` → apply **exact**，readback 0.2→0.0（"2 "→"0 "）独立参数读在卷（`abbey_road_plates_spot.json`）。
- **FabFilter Pro-R**：inspect_reverb 放行（direct_wording / 0.96）；Decay Rate `display_value` % 域写入 → apply **exact**，readback 100.0%→55.00% 在卷（`fabfilter_pro_r_spot.json`）。

## 5. 三轮历程与 §8 记账

| run | 目录 | verdict | 性质 |
|---|---|---|---|
| run1 | pgtdc_20261016_203552 | compat_verify_fail | 驱动面缺陷：误走 agent plugin.load_to_rack 被 PCA 装载门全量拒（产品面授权，无人值守探针不得伪造）；家族关键词误排 Pro-R/IR 系 12 件 |
| run2 | pgtdc_20261016_204214 | compat_verify_fail | 词表证据轮：61 件全量参数面到手；暴露 Chambers `Reverb Time X`/Doubler 裸词/MannyM tempo-fdbk/H-Delay Sec-BPM 词族缺口；Plates 核心断言已过、Damper enum 域写入被正确拒（暴露驱动缺 enum 分支） |
| run3 | pgtdc_20261016_205639 | **compat_verify_pass exit 0** | 终版：61/61 覆盖+双抽验 readback 变化 |

- §8 声明（三轮同）：LLM=0 次调用（全确定性工具面）；失败分类 environment_interrupt/assertion_fail/capability_blocked；止损=同签名 assertion_fail ×2（三轮失败各不同签名，未触止损）。
- 装载面：内核 ZMQ `rack_add_node`（M2-TIMEFX-2 探针先例）；inspect/apply 走 agent HTTP 工具面；**未启 Godot GUI（agent HTTP+内核 ZMQ 面驱动，边界同 M2-EXP-TIMEDYN-GAP-1 先例）**。
- 被测：worktree 构建 agent（4076900a）+ 主检出 staging VitApp.exe（C++/Godot 零改动）。栈占用/释放记录见 `coord/resources/PC-RUNTIME-STACK.md`（17533832）。

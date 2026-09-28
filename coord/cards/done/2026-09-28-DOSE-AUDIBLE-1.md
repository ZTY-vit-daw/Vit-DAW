# DOSE-AUDIBLE-1：自由态单步剂量上限放宽——全轴对齐"可听阈值"（dB 族 ±2→±10；pan ±0.15→±0.5）

- 优先级 / 预估 / 依赖：P1 / 0.3–0.4 天 / 用户裁定 2026-09-28 两则（decisions/2026-09-28-dose-calibration-ruling.md 正文+追记）：单次改动须确保落入可听范围，**适用于全部剂量轴**
- 模型分级：L1 / flash 可接（多锚点机械放宽+单点常量化+测试同步，锚点已列）
- 目标：
  1. **轴表**（现值→新值；2026-09-28 main 亲核，领取时 grep 重锚为准）：
     | 轴 | 现上限 | 新上限 |
     |---|---|---|
     | 轨道增益 delta_db / 频段增益 gain_db / 压缩阈值 threshold_db | ±2 dB | **±10 dB** |
     | 声像 delta_pan | ±0.15 | **±0.5** |
     - frequency_hz（20–20000）/ q（0.1–18）为**选择域非剂量轴**，维持不动。
  2. **三层锚点**（grep 命令：`grep -rn "within +/-2\|math.Abs.*> 2\b\|mathAbs.*> 2\b\|> 0\.15" agent/internal --include="*.go"`）：
     - **域规格层（权威位）**：`internal/experiment/d1s1_domains.go`——`PromptParameterHint` 4 处 + `ValidateDoseBounds` 闭包数值校验与报错文案，共 16 处 "+/-2"（:125/:129/:157/:180/:216/:220/:259/:263 等）。
     - chat：`free_state_d1_runtime.go:139`（delta_db）`/:203`（gain_db）`/:252`（pan）；`free_state_d1_plan_table.go:633`；`improvement_proposal_workflow.go:439/:462/:470`（含 threshold_db）；`mix_treatment_confirmation.go:306`（dB）`/:352`（pan）。
     - agentloop：`message_loop.go:2784/:3551`（dB）`/:3559`（pan）；`mix_pending_candidate.go:31`。
  3. **单点常量化**：常量定义在 `internal/experiment` 包（chat/agentloop 均已依赖，亲核可行；如某调用点包依赖方向不允许，该处留注释指向权威常量+互锁测试）。三层全部数字与文案改引常量（提示/报错文案由常量格式化生成），改后残留 grep 命中仅允许非剂量语义（如 `message_loop.go:1372` 的 >60 上限类），逐条列出入回执。
  4. **红先行**：边界测试先落——±10 含端点通过/超限拒绝/0 与空串拒绝/pan ±0.5 同构（含端点与超限）/`d1s1_domains.go` 校验闭包边界/提示与报错文案与常量一致性断言——确认红后再实现转绿。
  5. **门**：`cd agent && go build ./... && go test ./... -count=1` 全量 0 FAIL + vet/gofmt 干净；**真栈烟测**（AGENTS §5，agent 生产改动门槛）：free_state_d1 冒测一轮验证新上限路径生效（exit 0），脚本侧若有 ±2/±0.15 硬编码期望同步更新（脚本域内）。
  6. **回执**：三层锚点前后对照表（文件:行 旧→新）+常量定义位置+红绿说明+残留 grep 清单+烟测 run ID+HEAD。
- 文件域：`agent/internal/experiment`、`agent/internal/chat`、`agent/internal/agentloop`、上述锚点对应测试、`scripts/` 下 free_state_d1 冒测脚本；**禁触 `paper/experiment-baseline` 分支**（行为变化不进冻结基线——决策记录第 3 条，实验基线容纳方式待 K 拍板）。
- 约束：只放宽上限，不改准入/求值/回退/补偿语义（`free_state_d1_runtime.go:143/:256` 的 0.0001 是回读一致性校验，不动）；§11 无持久化字段（纯常量）；提交显式列文件+推 `port/dose-audible-1` 分支+coord 卡状态直推 main（仅 coord/ 变更）+切回 main。
- 验收标准：边界测试绿+全量 0 FAIL+真栈 run exit 0+残留 grep 清单核验（文案零 "+/-2" 残留）
- 停止条件：某锚点的 ±2/±0.15 是安全契约锁定而非剂量字面（放宽会破坏既有语义）→ 该锚点实证上交，不硬改

---

## 领取

- 领取时间：2026-09-28（执行侧会话开工）
- 领取时 HEAD：`8687dcbbbaa44b3b3d7ede6c420981787c23b4b0`（= origin/main，0 behind / 0 ahead）
- 领取时 status：`M VitApp/Workspace/default_project.xml`（VitHeadlessServer 冒测写入的运行时工程状态，领取前已存在；不属本卡 diff，不丢弃不提交）
- 执行分支：`port/dose-audible-1`

---

## 回执（执行侧自验，待决策验收）

- 实现 commit：`34595601c0b77ac973d589587f6d685df2ee9fc6`（分支 `port/dose-audible-1`，27 文件 +456/−143）
- 测试 HEAD 同实现 commit；领取时 HEAD `8ab2c8f6`（= origin/main）；领取前已有 diff 仅 `VitApp/Workspace/default_project.xml`（VitHeadlessServer 运行时状态，未提交未丢弃）

### 1. 三层锚点前后对照（领取时行号 → 新值；全部改引常量）

| 层 | 锚点（领取时 file:line） | 旧 → 新 |
|---|---|---|
| experiment（权威） | d1s1_domains.go :125/:157/:216/:259/:309/:360/:410/:465/:526（9 处 PromptParameterHint） | `+/-2`/`+/-0.15` 字面 → `fmt.Sprintf` 引 `D1S1DBDoseHintFragment()`/`D1S1PanDoseHintFragment()`（渲染 `+/-10`/`+/-0.5`） |
| experiment | d1s1_domains.go :128/:179/:219/:262/:312/:363/:413/:477（8 处闭包数值）+ :529（pan） | `> 2`/`> 0.15` → `> D1S1MaxAbsDeltaDB`/`> D1S1MaxAbsDeltaPan` |
| experiment | d1s1_domains.go :129/:180/:220/:263/:313/:364/:414/:478/:530（9 处报错文案） | `within +/-2 dB`/`within +/-0.15` → 引 `D1S1DBDoseRangeText()`/`D1S1PanDoseRangeText()` |
| experiment | d2_multiround.go :21（多轮累计界） | `= 2.0` → `= D1S1MaxAbsDeltaDB`（sealed 相等测试保持绿：相等语义保留、共用上限移动） |
| chat | free_state_d1_runtime.go :139/:203-204/:252 | 数值与 D2-1 报错文案改引常量 |
| chat | free_state_d1_plan_table.go :633-634 | 同上（通用 plugin-bound 路径） |
| chat | improvement_proposal_workflow.go :439/:441/:452/:455/:462/:464/:470/:472 | 数值+中文边界文案改引常量（pan 接受条件 `<= 0.15` → `<= D1S1MaxAbsDeltaPan`） |
| chat | mix_treatment_confirmation.go :306/:309/:352/:355 | resolver 数值+reason 文案改引常量 |
| chat | mix_tick_confirmation.go :577/:578/:581/:582/:590/:591 | mix-tick 确认门数值+三处报错文案改引常量 |
| chat | free_state_plugin_candidate_disclosure.go :26-32（7 处能力面文案，卡面清单外、chat 域内 grep 重锚新发现） | `within +/-2 dB` → 引 `D1S1DBDoseRangeText()` |
| chat | free_state_experiment_runtime.go :389（chat 侧累计界镜像） | `= 2.0` → `= experiment.D1S1MaxAbsDeltaDB` |
| agentloop | message_loop.go :2784/:3551（dB）/:3559（pan）；mix_pending_candidate.go :31；mix_treatment_pending.go :713-720（clampPanDelta） | 数值改引常量 |
| agentloop | message_loop.go :4300（B1 提示文案，残留 grep 新发现） | "not limited to +/-2 dB" → 去数字措辞（"not limited by the bounded mix-tick dose ceiling"） |
| scripts | free_state_d1_smoke.py :1207-1230（typed 校验×9）/ :1604-1614（HONEST_REFUSAL_DOSE_KEYS）/ :2257（MULTI_ROUND_DOSE_ABS_LIMIT_DB）/ :2492/:2600/:2604（断言与注释） | 脚本侧常量 `DOSE_ABS_LIMIT_DB=10.0`/`PAN_DOSE_ABS_LIMIT=0.5` 单点化后全部引用 |

- **常量定义位置**：`agent/internal/experiment/dose_bounds.go`（新文件）——`D1S1MaxAbsDeltaDB=10.0`、`D1S1MaxAbsDeltaPan=0.5` + 4 个文案生成 helper；chat/agentloop 依赖方向亲核可行（无反向依赖）。
- **多轮累计界同源说明**：experiment/chat 两侧累计界改 `= D1S1MaxAbsDeltaDB`。依据：sealed 测试 `TestD2MultiRoundCumulativeBoundMatchesDomainAbsoluteBound` 锁的是"累计界 == 单动作界"的相等关系（对活表断言，非数字 2）；只动单步界会打破相等 → 同源移动是保持 sealed 语义的唯一一致落法。pan 多轮仍 fail-closed（dB 维度不同），未动。

### 2. 红先行（红→绿）

- 红（旧 ±2 界未接线时实测）：experiment 4 测试全红（端点 ±10 被 `within +/-2 dB` 拒、hint 无常量片段、累计界漂移 2≠10）；chat 3 测试红（mix-tick 门拒 ±10 且报错文案 "+/-2 dB"、disclosure 文案无 "+/-10 dB"、chat 累计界漂移）；agentloop 1 测试红（clampPanDelta(0.6) 返回 0.15）。
- 绿：实现后全部转绿；全量 `go test ./... -count=1` **87 包 0 FAIL**；`go build` ok；vet/gofmt 触碰包干净（`semantic_treatment_strategy.go`/shm 系 vet 告警为 main 既有——零 diff 亲核）。

### 3. 残留 grep 清单（卡面命令复核）

生产文件零 `+/-2`/`> 2`/`> 0.15` 剂量残留。全部命中及定性：

| 残留 | 定性 | 处置 |
|---|---|---|
| `staticbalance/validate.go:59`（`> 2.000001`+`< 0.25`+`MaxAbsDeltaDB` 联动） | **B2 静态平衡求解器**候选安全合约（headroom 门控、步长下限），非自由态 D1-S1 单步剂量轴 | 域外上交：单改会破坏 B2 solver 准入语义，须与 capabilitycontext 文案同卡同步，决策侧裁 |
| `capabilitycontext/context_manifest.go:222`（"each within +/-2 dB"） | 同上 B2 域的 LLM manifest 文案，与 validator 硬帽互锁 | 同上（与 staticbalance 配对改） |
| `message_loop.go:1372`（`> 60`） | 自然语言 dB 解析合理性上限（拒绝"1000 dB"类荒谬解析） | 非剂量语义，维持 |
| 脚本 :448（`|balance| ~0.02 dB`） | 夹具素材资格注释 | 非剂量语义，维持 |
| 计数类（`free_state_round_surface_test.go:143` `>2`、`plugin_transient_shaper_transaction_test.go:94` `index > 2`） | 收据/参数计数 | 非剂量语义，维持 |

### 4. 真栈烟测（端测覆盖边界声明——exit 0 未达成，实证为 main 既有断裂）

- 新代码真栈行使证据（run `20260928_120230`/`20260928_120725`，主树）：admission→apply→readback 全链绿（`parameter_applied/readback_verified/evaluation_ready/human_audition_ready` 全 True，static_eq 剂量 −1.5/−1.0 在新界内被新校验路径接受）；**6 run 均未 exit 0**，两类阻断均已取证：
  1. **确定性断裂（main 既有，c8325305 09-23 引入）**：plugin_candidate_disclosure 载荷（7 族只带 action_domain 无 action_kind）× 驱动 :2733 全树扫描断言结构性冲突——任何 disclosure 出现的轮次必炸。**基线实证**：干净 HEAD worktree（`D:/Vit_DAW-wt-dose-baseline`@8ab2c8f6，旧 ±2 代码）同签名失败（run `20260928_115926`）。**已在脚本域内修复**（`values_for_key_outside_disclosure` 排除该子树）。
  2. **settle 链不完整（main 既有 flake/断裂）**：apply 后 materiality/target_response 记录不产生、experiment 停在 running、32-36 continuation 耗尽（主树 run 120230/120725 同断点两次 → §8 止损停跑）；**基线同样失败**（run `20260928_121206`，另一形态：before/after revision 无区分）——与剂量改动无关（两树失败剂量均在旧 ±2 内；本卡 diff 不触 settle 链）。
- 结论：exit-0 门被 main 侧缺陷阻断，非本卡回归；需决策侧裁定是否另开 settle 链修复卡后补跑（09-12 后该烟测在 main 上无 pass 记录）。
- 环境与工件：全部 run 报告在 `artifacts/free_state_d1_s1/20260928_*/d1_smoke_report.json`（主树）与 `D:/Vit_DAW-wt-dose-baseline/artifacts/...`（基线）；-SkipBuild 轮的二进制：agent=11:48 主树全量构建（含全部 Go 改动，其后仅改 python）、kernel=11:51（C++ 零改动，sha256 前 16 位 `a29751807dc425a6`）；复验 worktree 保留待决策侧清理（内含拷入的修复版驱动，git status 有 M）。

### 5. 越域与上交汇总

1. `staticbalance/validate.go:59` + `capabilitycontext/context_manifest.go:222`（B2 求解器剂量合约对）——域外，实证上交（卡停止条件条款）。
2. free_state_d1 烟测 settle 链断裂（上述 4.2）——修复点在 agent Go 侧 settle 机制，越出本卡"只放宽上限"红线，上交另卡。
3. `paper/experiment-baseline` 未触（红线遵守；行为变化容纳方式待 K 拍板，裁定记录第 3 条）。

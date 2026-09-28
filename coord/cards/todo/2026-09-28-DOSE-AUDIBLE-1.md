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

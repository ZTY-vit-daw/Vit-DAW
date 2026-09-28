# DOSE-AUDIBLE-1：自由态单步剂量上限放宽——±2 dB→±10 dB（声像 ±0.15→±0.5）

- 优先级 / 预估 / 依赖：P1 / 0.3 天 / 用户裁定 2026-09-28（decisions/2026-09-28-dose-calibration-ruling.md）：单次改动须确保落入可听范围，幅度 ±10 dB
- 模型分级：L1 / flash 可接（多锚点机械放宽+常量化+测试同步，锚点已列）
- 目标：
  1. **锚点清单实测**（下列为 2026-09-28 main 亲核；领取时漂移以 `grep -n "math.Abs.*> 2\b\|> 0.15" agent/internal --include="*.go"` 重锚为准）：
     - chat：`free_state_d1_runtime.go:139/:203`（dB 族）、`:252`（pan）；`free_state_d1_plan_table.go:633`；`improvement_proposal_workflow.go:439/:462/:470`（含 thresholdDB）；`mix_treatment_confirmation.go:306`
     - agentloop：`message_loop.go:3075/:3559`（pan 族，报告基线锚 :2784/:3551 现势重锚）；`mix_pending_candidate.go:31`
     - 文案面：提示/能力描述中的 `+/-2 dB` 字面（ccb_model_prompt 族及其测试）
  2. **放宽**：dB 族上限 2→**10**；pan 族 0.15→**0.5**（57 §6 制度原案；用户裁定点名 dB，pan 如需另值由决策侧再调）。
  3. **单点常量化**：上限值提炼为常量并全部锚点改引常量——优先放两包共同依赖的单点（如 experiment 包）；包依赖方向不允许时允许 chat/agentloop 各自定义同名常量+**互锁测试断言相等**。禁止改后仍留数字字面。
  4. **红绿**：先落边界测试（±10 含端点通过/超限拒绝/0 与空串拒绝/pan ±0.5 同构/提示文案与常量一致性断言），确认编译红或断言红后再实现。
  5. **门**：`cd agent && go build ./... && go test ./... -count=1` 全量 0 FAIL+vet/gofmt 干净；**真栈烟测**（AGENTS §5，agent 生产改动门槛）：free_state_d1 冒测一轮验证新上限路径生效（exit 0）；烟测脚本侧若有 ±2 硬编码期望同步更新（脚本域内）。
  6. **回执**：锚点前后对照表（文件:行 旧值→新值）+常量定义位置+红绿说明+烟测 run ID+HEAD。
- 文件域：`agent/internal/chat`、`agent/internal/agentloop`、上述锚点对应测试、`scripts/` 下 free_state_d1 冒测脚本；**禁触 `paper/experiment-baseline` 分支**（行为变化不进冻结基线——2026-09-28 决策记录第 3 条，实验基线容纳方式待 K 拍板）。
- 约束：只放宽上限，不改准入/求值/回退语义；§11 无持久化字段（纯常量）；提交显式列文件+推 `port/dose-audible-1` 分支+coord 卡状态直推 main（仅 coord/ 变更）+切回 main。
- 验收标准：边界测试绿+全量 0 FAIL+真栈 run exit 0+`grep -rn "+/-2\|±2" agent/internal` 文案零残留（数值测试锚点除外，逐条列出）
- 停止条件：某锚点的 ±2 是安全契约而非剂量字面（放宽会破坏既有语义锁定）→ 该锚点实证上交，不硬改

# FIX-CARRIES-REPORT-TIGHTEN-1：carriesExperimentReport 审计跳过收紧（loop.Experiment!=nil 才跳过）

- 发卡：GLM 主管决策侧 / 2026-10-14 凌晨（依据=FIX-FS-SETTLE-DEADCHAIN-1 域外上交+验收裁定采纳；锚点=`agent/internal/chat/free_state_reasoning_loop.go:1047` carriesExperimentReport）
- 派发确认：已确认（主管依已核实上交证据发卡）
- 验收负责人：GLM 主管决策流
- 池序 110；目标仓库=D:\Vit_DAW
- 晚窗参考序①（2026-10-14 早窗校准：纯单测卡不占真栈，可与 109 腿 D 并行；晚窗实际执行按 /night 时池序自取）
- 优先级 / 预估 / 依赖：P3 / 0.5 天 / 无
- 模型分级：L1 / GLM 或 flash 执行会话

## 背景

109 执行侧域外发现：carriesExperimentReport 对「实验未存在+占位三件套」的审计跳过是潜在吞链面——deepseek 在提案轮逐字抄写骨架示例的 settle 占位字段时，该跳过使占位三件套绕过准入审计（C 腿已从提示面缓解并真栈验证生效，但谓词面仍是敞口）。

## 任务

收紧为 `loop.Experiment!=nil` 才允许审计跳过；回归测试钉住「无实验+占位三件套=不跳过（按缺实验处理非按已申报处理）」；`go test ./... -count=1` 0 FAIL+gofmt 净。纯单测卡，真栈腿可选（按 §5 显式声明端测边界即可）。

## 文件域

agent/internal/chat/free_state_reasoning_loop.go + 同包测试。

## 停止条件

收紧与既有 95 包测试语义冲突（说明跳过另有承重用途）→ 带测试名清单上交。

- 领取：2026-10-14 10:42 +0800 / origin/main=f86adccf（卡在 todo、派发已确认、依赖无；doing=109 域 goalrunner+free_state_experiment_runtime，与本卡 chat 包不相交）/ GLM-5.3 L1 执行会话（PC，ZCode）/ port/fix-carries-report-tighten-1 / D:/Vit_DAW_wt_crrt1 / 领取提交 84449633
- 回执：commit=43dc0010（port/fix-carries-report-tighten-1 已推 origin）。收紧=carriesExperimentReport 加 `loop.Experiment != nil &&` 前提（free_state_reasoning_loop.go:1047 域），无实验时占位三件套走 G1-G7 新准入审计（capability_blocked+admission_gate_failed receipt），有实验时 settle 豁免不变（D2-2 多轮豁免自带实验前提）。新增回归测试两面钉（无实验+占位三件套=审计拒收；单轮实验+同三件套=豁免保留），红面验证：还原旧谓词主测试 FAIL（escape: status="blocked" 无 gate receipt）后恢复复跑全绿。自验：`go test ./... -count=1` 95 包 0 FAIL（exit 0）+ gofmt 净（LF blob 口径；gofmt -l 在 chat 包检出的 ~200 文件为既有 CRLF 检出伪影，本卡两文件以 LF 形态直接过检，diff +12/-2 无全文件重排）。域外备注：agentloop 包存在同名 schema 形状谓词（agentloop/free_state_reasoning.go:80 carriesExperimentReport 方法，输出门形状校验、非审计跳过），不在本卡域未动——若需同名收紧由决策侧另议。端测边界声明：**纯单测收口，无真栈腿、不占 PC-RUNTIME-STACK**（与 109 腿 D 无串行关系）。
- 验收：**pass** / 2026-10-14 批量一（[rulings/2026-10-14-BATCH1-rulings.md](../../rulings/2026-10-14-BATCH1-rulings.md)①；diff 亲读+决策侧独立亲跑 95 包 exit 0+gofmt blob 级净；域外 agentloop 同名谓词登记不联动；实现 cherry-pick 入 main=d9e13187；worktree crrt1 拆除，分支 origin 留档）

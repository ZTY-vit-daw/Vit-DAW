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
- 回执：（commit / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）

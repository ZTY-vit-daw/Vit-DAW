# SMOKE-SCRIPT-HYGIENE-1：fs_largeproject_smoke.ps1 五项修补（复核报告 D/E 节+分类分歧）

- 发卡：GLM 主管决策侧 / 2026-10-10 晚窗（依据=[EVENING-BATCH rulings §5](../../rulings/2026-10-10-EVENING-BATCH-rulings.md) §9 缺口登记+[REVIEW-independent.md](../../runs/FS-LARGEPROJECT-SMOKE-1/REVIEW-independent.md) B/D 节）
- 派发确认：已确认（主管裁定）
- 验收负责人：GLM 主管决策流
- 池序 50；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P3 / 0.5 天 / 无；可与 FS-LEDGER-PERSIST-1 并行（不同文件域：本卡仅 scripts/）
- 模型分级：L1 / flash 可接（纯脚本修补）

## 五项目标（复核报告建议逐条）

1. **分类阶梯消费 fsLoopAdmissionStatus**：run_report.classification 阶梯增加已提取的准入态字段通道（修复"机器分类 other vs 人工 capability_blocked"分歧——run 185519 实证）。
2. **A4 补标记**：`c2.dynamic_plugin_load.governed`（单轨装载受控命令，现 _batch 子串覆盖不到）+telemetry source 面扫描（固定编排规划器独立 LLM source 标如 `b4_project_eq_planner`）。
3. **run_report 补字段**：command_line（$MyInvocation.Line 为空时改记录完整参数拼接或 PSCommandPath+Args）；exit code 数值化；agent_binary 死字段修复或删除。
4. **run_report.json 落盘健壮性**：Phase 6 类崩溃（$pid 只读变量）不吞 run_report——全局 try/catch 兜底落盘（184500 教训）。
5. 回归：既有断言组与清场约束零弱化。

## 文件域

`scripts/fs_largeproject_smoke.ps1` 单文件；越域即停。

## 验收标准

脚本语法过+干跑（-WhatIf 或早退模式如支持；无则静态审查）+复核报告五项逐条对照入回执；不占真栈（真栈回归归 FS-LEDGER-PERSIST-1）。

## 停止条件

无。

- 领取：2026-10-10T19:54+08:00 / origin/main=596ba4808e23d7c4156f67b2b5708e552a15051d / owner=GLM-5.3-Flash（PC，ZCode flash 执行会话） / 分支=port/smoke-script-hygiene-1（worktree=D:\Vit_DAW_wt_smokehyg1） / 领取提交=本提交（coord/smoke-script-hygiene-1 fast-forward → main，仅含本卡状态）
- 回执：实现 commit=a487251e（port/smoke-script-hygiene-1，worktree D:\Vit_DAW_wt_smokehyg1，基线 596ba480，仅 scripts/fs_largeproject_smoke.ps1 一文件 +177/-9）。五项对照如下，行号均指实现后脚本。

  **项1 分类阶梯消费 fsLoopAdmissionStatus**：:1376-1384 新增 rung（`$fsLoopAdmissionStatus -eq "capability_blocked" -or -eq "blocked"` → capability_blocked），位置=既有 capability_blocked stop-reason rung 之后、no_candidate_found 之前；needs_experiment/improvement_proposal 为继续态非失败类，不消费（注释写明）。验证=185519 真实数据回放（stop_reasons=['done']、admission=capability_blocked、last_error=free_state_admission_gate_failed）：旧阶梯判 other，新阶梯判 capability_blocked——机器/人工分歧修复。

  **项2 A4 补标记+telemetry source 面**：:1272 fixedMarkers 增 `c2.dynamic_plugin_load.governed`（Go 锚 c2_dynamic_plugin_load.go:20，dispatch c2_dynamic_control_runtime.go:520）；:1211-1229 新增 $fixedOrchestrationLlmSources 8 个固定编排 planner LLM source 标（b4_project_eq_planner/b4_project_eq_instance_selection/b4_project_treatment_planner/c1_project_eq_planner/c1_project_treatment_planner/c2_project_candidate_planner/c2_project_target_planner/c2_project_treatment_planner；Go 锚 b4_eq_planner.go:43/124/180、c1_frequency_cleanup_planner.go:41/108、c2_dynamic_control_runtime.go:214/308、c2_project_treatment.go:114）；:1253 telemetry source 面扫描、:1304-1306 命中并入 markerHits（kind=telemetry_llm_source）、:1308-1316 audit 记录增 telemetry_llm_source_markers/telemetry_llm_source_hits+scope_note 更新。假阳排查=185519 工件（nl_events/nl_chat/confirm_roundtrip）与活线 agent_last.log 对新标记零命中，该命令仅现于 C2 单轨执行面（Action/receipt）非工具广告面。**有意不加** plugin_grabber.apply_eq_edits.governed（复核 A 注①提及但卡面未点名）：它是模型驱动 semantic EQ 的合法应用命令（semantic_eq_runtime.go:604 校验模型动作恰为该命令），入零命中集会把模型驱动合法 EQ 误判为固定编排。

  **项3 run_report 补字段**：①command_line：:293-313 $MyInvocation.Line 为空（-File 调用）时回退 $PSCommandPath+显式绑定参数拼接（带空格参数加引号），:317 消费——探针实证 `-File` 下记录为完整命令 `D:\...\fs_largeproject_smoke.ps1 -TrainingFolder E:\__nonexistent_probe__`。②exit code 数值化：:285 初始化 $script:PlannedExitCode=1、:348+368 Write-Report 戳记 `$report.exit_code`，12 个站点逐一前置设置（:434/:498/:560=2，:677/:797/:805/:828/:1439/:1465=1，:751/:766=2，:1431=0）——探针实证 exit_code=1。③agent_binary 死字段修复：:322-327 改结构化 path/sha256/last_write_utc（初值 $null），:509-533 bring-up 后从运行中 VitAgent 进程 ExecutablePath 解析（回退 agent\bin\VitAgent.exe）+SHA256+LastWriteTimeUtc（AGENTS §9 SkipBuild 二进制记录规则）——探针在 bring-up 前崩溃故为 null（崩溃面不虚构，符合预期）。

  **项4 崩溃兜底落盘**：①Write-Report 硬化 :362-384：序列化 try/catch，失败落 run_report.min.json（最小报告：run_id/verdict/classification/exit_code/序列化错误文本）后 rethrow。②Phase 0-1 pre-net wrap：:392 try 开 + :566-585 pre-net catch 镜像（script_exception 分类+报告落盘+拆栈+exit 1）。③栈所有权守卫（实现中新发现隐患的修复）：pre-net throw 可发生在 Phase 0 清场**之前**（如 TrainingFolder 解析失败），原式无条件 Clear-OwnedStack 会误杀他人栈——:291 $script:StackOwnershipClaimed=$false、:439 清场通过后置 true、:579-589 catch 仅在已认领时拆栈并做 teardown 核验。探针实证：假 TrainingFolder → run_report.json + script_exception_prenet.txt 落盘、exit=1、teardown=null（所有权未认领不拆栈）——184500 形崩溃不再吞 run_report。

  **项5 回归零弱化**：A1/A2/A3 断言表达式逐字未动；A4 判定式（zero_hits）不变仅扫描面严格扩大；分类阶梯仅插入新 rung、既有 rung 谓词与顺序不变；全部 exit 站点退出码数值与语义不变（0/1/2）仅补记录；Phase 0 清场、E:\ manifest 前中后校验、teardown 校验、drafts 重定向、概率纪律块逐字未动。full diff +177/-9 已逐块复查。

  验收面：语法=Parser::ParseFile 0 errors（全部编辑后复跑）；干跑=探针 run fs_largeproject_smoke_20261010_200938（worktree 内未跟踪工件，报告字段证据已引用如上）；端测覆盖边界声明=本卡为脚本 hygiene，卡面验收口径即语法+干跑/静态审查，**真栈烟测回归未执行**（不占真栈，卡面明确真栈回归归 FS-LEDGER-PERSIST-1）。已声明边界：agent_binary 填充路径与 telemetry source 扫描的运行时行为未经真栈 run 实证，属静态审查+探针间接覆盖面。

- 验收：（裁定文件 / 验收 commit）

# TIM-KERNEL-HYGIENE-1：内核卫生小卡——DC offset 断言源+Hygiene Report 回传（GAPS Item 2+5 合卡）

- 优先级 / 预估 / 依赖：P1 / ~1 天 / TIM-KERNEL-GAPS-1 报告 §2 Item 2/Item 5（锚点与工作量依据）；CTES 断言器 v1 已合入（消费方在位）
- 模型分级：L2 / GLM 亲自或强督导（VitApp 域）
- **执行侧（mac 会话，夜池；ctest 双配置门）**
- 目标（两件合卡，均 VitApp 域）：
  1. **Item 5：PluginListHygiene Report 回传**——内核启动清理摘要（"PluginListHygiene: removed ..."行，日志面）结构化为 get_project_state 的卫生块（removed 计数+最近清理时间戳；零清理=显式 0）；agent 侧 TIM 断言器消费（新增 AS-HYGIENE 检查或并入 signal_hygiene 域——执行侧按现有断言器结构最小接入，三态语义保持）。
  2. **Item 2：DC offset（带符号均值）**——L3AcousticAnalyzer 增 DC offset 计算（帧级带符号均值，逐帧进 feature snapshot 透传键表）；agent 侧 AS-SIG 增 P3 断言（|dc_offset| 阈值 warn，阈值常量透出，缺键 NE）。
  3. **红绿**：内核 ctest（新特征字段+卫生块断言，双配置）+agent 红绿（P3 断言+卫生消费）；既有 7 项 ctest+84 包零回退。
  4. 回执：红绿退出码+diff stat+HEAD。
- 约束：零音频线程触碰（分析路径是离线 L3 池）；阈值常量透出可比性键（K5/K7 范式）；文件域 VitApp/Source+Tests+agent/internal/tim。
- 验收：双配置 ctest+agent 全量 0 FAIL+红绿在案
- 停止条件：GAPS 报告锚点与现实不符 → 取证上交
- 领取：2026-09-27 03:13 UTC / dd2193a3bac5f65e553b547ec5260ab72bf64c3c（origin/main）/ port/tim-kernel-hygiene-1
- 回执（2026-09-27 mac 夜池）：
  - 红绿在案：RED `ca5dbc8`（内核 L3EvidenceTests 新 ctest 目标+HYG 测试扩展对桩实现：L3 exit 1 四项 FAIL 全指向 dc_offset 键缺失/HYG exit 134；agent 7 测试 exit 1 全红）→ GREEN `08b8986`（L3 exit 0 全过/HYG exit 0 全过/agent 7/7 PASS+tim 包过）。分支 port/tim-kernel-hygiene-1 已推 origin。
  - 门：**ctest 双配置 Debug 8/8 + Release 8/8（各 exit 0，既有 7 项零回退+新增 VitL3EvidenceTests）**；主工程 VitApp Release 链接 exit 0（`VitApp/build/app-release`）——该门抓到 CommandDispatcher.cpp 卫生块一处裸指针误用编译错并修复，证明 ctest 不覆盖 dispatcher 编译、主工程门必要；agent `go test ./... -count=1` **84 包 ok exit 0**+`go vet ./internal/tim` exit 0+新增文件 gofmt 干净（gofmt 清单中 asserter_test.go 为存量未触碰）。
  - diff stat：内核 6 文件（L3AcousticAnalyzer.cpp/PluginListHygiene.cpp+.h/PluginRackControlService.cpp+.h/CommandDispatcher.cpp）+测试 3 文件（L3EvidenceTests.cpp 新 266 行/CMakeLists 加目标/PluginListHygieneTests.cpp 扩展）+agent 4 文件（asserter.go/projection.go/types.go/asserter_hygiene_test.go 新 202 行），全部在卡面文件域内。
  - GAPS 锚点复核：Item2（sumAbs :103/observe :127/all_zero :442）、Item5（PluginListHygiene.cpp:57-68 只写日志/CommandDispatcher :3268+）均与现实一致，停止条件未触发。
  - **域外登记（一行接线，决策侧裁定）**：`PluginListHygiene` 的 mixboard 装配缺一行——`agent/internal/mixboard/mixboard.go:4023` 的 `tim.Input{...}` 需加 `PluginListHygiene: mapValue(req.ProjectState["plugin_list_hygiene"])`；tim.Input 槽位已预留，未接线前 AS-HYGIENE 恒 NE（fail-safe）。dc_offset 主通道无需接线：acoustic 行整行透传（mixboard.go:532 acousticRowForTrack 选行不裁键），内核发布 dc_offset 后自动到达 TrackFact；fallback 通道 timAcousticEvidenceByTrack 白名单（mixboard.go:4071）可顺手加键但非必需。
  - 不稳定测试记录（§11 流程）：`internal/chat TestAutomaticProposalTextConfirmationKeepsTaskIdentityAndPendingProjection`——失败类型=t.TempDir 清理竞态（`unlinkat ... directory not empty`，.vit_history 会话 workspace 后台写者未收尾），非断言失败；两次失败均发生在 C++ 后台构建并发的高负载整包跑，隔离复跑×2 过+基线 origin/main worktree 整包×2 过+静默整包（84 包）过——与本卡 tim-only diff 排除关联，按 known flaky（负载相关）登记，建议开修复卡（测试结束前 join 后台写者），决策侧裁定。
  - 过程记录：红→绿中两处测试脚手架修正（T4 quality_reasons 按 juce 数组遍历而非 toString；防洪摘要断言对齐历史日志格式 "+N more" 无空格——生产格式保真于 FIX-KERNEL-PLUGINLIST-HYGIENE-1 原文）；首次主工程配置时 libzmq FetchContent 下载瞬态失败一次，重试即过（环境类，已归因）。
  - 端测覆盖边界声明：本卡门=组件级（ctest 双配置+主工程编译+agent 单测），真栈面（get_project_state 卫生块经 VSP 端到端+TIM 断言器在真投影流的行）未跑——夜间托管无真栈（PROTOCOL §4），留决策侧合入后安排。
  - HEAD：`08b8986`（port/tim-kernel-hygiene-1 已推 origin）。
- 验收：**pass（2026-09-28 决策会话）**——红绿双 commit（ca5dbc8 红/08b8986 绿）在案；**我方合入后复跑：ctest Debug 8/8+Release 8/8 双配置全绿**（Release 首跑 Not Run 系我本地未构建目标，补编后过——非测试失败）+agent tim 包 ok；主工程编译门抓到 dispatcher 裸指针误用并修复的价值声明采信（ctest 不覆盖 dispatcher 编译的覆盖面说明成立）；两处测试脚手架修正记录（生产格式保真）合规；端测边界（真栈卫生块端到端留决策侧）如实声明——与 DISCLOSE 腿完成后可合并开一张真栈腿。

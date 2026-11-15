# KERNEL-TIMESIG-1：内核 set_time_signature 命令面+创建时拍号应用+持久回读一致

- 发卡：GLM 主管决策流 / 2026-11-16 凌晨（依据=[审计 H4 阻塞点](../../reports/2026-11-12-demo-execution-audit/README.md)+[369 静态盘点=无此命令](../runs/G0-SCORE-SPIKE-1/REPORT.md)+[§13/§14 裁定]；373 域锚定后解锁（VitApp 零占用亲证）；进度驱动无工期）
- 派发确认：已确认（主管发卡）
- 验收负责人：主管决策流（真栈腿）+ 374 M3 消费方
- 池序 375；P0；依赖=无（真栈腿候栈位：373→本卡）；PC

## 目标

1. **内核命令面**：`set_time_signature`（numerator/denominator，RiskDirect 或对齐 set_tempo 既有面——领取时锚定同类命令先例与权限档）；agent 侧 catalog/工具面接线。
2. **创建时应用**：工程创建（新建/乐局装配）可设拍号——本课 2/4（§13：工程拍号由 YAML 统一、创建时应用；Tap 局部拍号不写工程=既有裁定不变）。
3. **持久化兼容（AGENTS §11 硬性）**：拍号入工程状态须定义旧工程缺省语义（无字段=4/4 现行为零变化）+旧工程加载/保存往返测试；未知值 fail-closed 策略。
4. **回读一致**：设 2/4→保存→重开→拍号回读一致；get 面可查（music_context 或等价）。
5. 真栈烟测腿（DG 口径）：设置→回读→重开一致断言 exit 0（复用 dev_agent_smoke 同模式加参数；S10 纪律照旧）。

## 非目标

- 多段拍号/tempo 变更链（谱面线 fail-honest 已裁）；Tap 控件局部拍号（不写工程）；UI 面（374/控件线）。

## 文件域

`VitApp/Source/`（命令面+持久化）+ `agent/internal/`（catalog/工具接线+单测）+ 烟测装置扩展 + coord 工件；领取时回填精确清单。禁入：webui、gen 域、coursetemplate（373）、scoreworkbench（374）。

## 验收标准

真栈腿 exit 0（设置→回读→保存→重开一致）+旧工程往返测试+内核构建/测试绿+三门不回退；374 M3 的 2/4 工程语义由此解锁。

## 停止条件

拍号持久化牵动工程文件格式不兼容且缺省策略不可收敛→取证上交（§11 裁定）。

## 领取/回执/验收

### 领取（2026-11-16 00:1x +0800）

- 领取时间：2026-11-16 00:1x +0800（池序 375，P0）
- origin/main 基线：`409435e2`（fetch 后核对；`git status --short` 领取时=空，`git diff --stat`=空——独立 worktree 全净领取）
- owner：GLM-5.3 L2 执行子代理（ZCode PC 会话，KERNEL-TIMESIG-1 专项）
- 分支：`port/kernel-timesig-1`；worktree：`C:/Users/timoz/wt_timesig`（D 盘净空不足 worktree 创建失败亲证——沿 374 卡先例落 C 盘；卡面如实记录）
- 协调分支：`coord/timesig-claim`（仅含本卡领取状态提交）
- 先例锚定（领取时行号，基线 409435e2）：
  - set_tempo 注册=`VitApp/Source/Service/CommandDispatcher.cpp:2688`；handler=`:3869`（markAsChanged+runAutoPersist 模式）
  - 权限档先例=set_tempo=`RiskUndoable`（`agent/internal/tools/catalog.go:1151`；args hint `:392`）
  - 创建时参数先例=new_project audio_settings 载荷（`VitApp/Source/Service/VitHeadlessService.cpp:1118` ipcNewBlankProject，`:1141` payload 应用+`:1170` 存盘触发条件）
  - get 面=music_context 已披露 time_signature/numerator/denominator（`CommandDispatcher.cpp:2245-2249`，读面零改动）
  - 持久化机制=tracktion TimeSigSetting CachedValue 缺省 4/4（`tracktion_engine/.../tracktion_TimeSigSetting.cpp:21-22`）——旧工程无字段=4/4 零变化的引擎侧根基
  - W01 timing 契约=`docs/LEGAME_DESIGN_CONTRACT_V0.md:125-134`（`workspace.timing.time_signature` 字符串 `N/D`）
- 精确文件清单（领取时核定）：
  - `VitApp/Source/Service/TimeSignatureService.h`（新：解析/校验/应用+命令面 handler）
  - `VitApp/Source/Service/TimeSignatureService.cpp`（新）
  - `VitApp/Source/Service/CommandDispatcher.cpp`（注册 set_time_signature+服务构造）
  - `VitApp/Source/Service/CommandDispatcher.h`（成员声明）
  - `VitApp/Source/Service/VitHeadlessService.cpp`（new_project 创建时拍号应用+W01 衔接注释）
  - `VitApp/CMakeLists.txt`（源清单）
  - `VitApp/Tests/TimeSignatureTests.cpp`（新：命令面+fail-closed+§11 旧工程往返）
  - `VitApp/Tests/CMakeLists.txt`（测试目标）
  - `agent/internal/tools/catalog.go`（spec+argHint）
  - `agent/internal/tools/catalog_test.go`（目录化断言）
  - `scripts/dev_agent_smoke.ps1`（真栈烟测腿参数，零栈段不触碰）
  - `coord/cards/POOL-INDEX.md`、本卡面、`coord/runs/KERNEL-TIMESIG-1/`（run 工件）
- 领取提交：`636d31b3`（coord/timesig-claim→main，推送后重读远端 owner 亲核）
- 资源计划：零栈段（内核实现+CMake 构建+内核测试+Go 单测）先行不占栈；真栈腿候 373（SRC-TEMPLATE-1 DG1-B）释放后按 PC-RUNTIME-STACK 协议登记

### 回执（2026-11-16 04:2x +0800，执行侧自验完成）

- **限额中断如实记录**：限额中断 04:0x（实现提交 318f2f17 已推后、真栈腿前），04:1x 续跑，断点零丢失（主管已核 worktree 干净；续跑起点=栈登记）。
- **实现 commit**：`port/kernel-timesig-1` @ `318f2f17`（实现，16 文件 +1288/-2）→ `fcfa4321`（真栈工件+RUNBOOK+探针装置修复），均已推。diff 文件清单：
  - 内核：`VitApp/Source/Service/TimeSignatureService.h`（新，命令面+W01 串解析+applyToEdit）、`TimeSignatureService.cpp`（新）、`CommandDispatcher.cpp`（注册 15 行）、`CommandDispatcher.h`（成员+前置声明）、`VitHeadlessService.cpp`（new_project 创建时应用+存盘条件+回执 echo）、`VitApp/CMakeLists.txt`（源清单）
  - 内核测试：`VitApp/Tests/TimeSignatureTests.cpp`（新，491 行）+`Tests/CMakeLists.txt`（VitTimeSignatureTests 目标）
  - agent：`tools/catalog.go`（spec+argHint）+`catalog_test.go`、`chat/goalrunner_chat.go`（transport 白名单行）、`harness/harness.go`（归一化：\"2/4\"串→整数对，不可解析不兜底）+`timesig_normalize_test.go`（新）、`presenter/reply.go`（回执文案）
  - 装置：`scripts/timesig_smoke.py`（新，探针 T1-T5）+`dev_agent_smoke.ps1`（-TimesigSmoke 参数+隔离列表+腿块）
  - 工件：`coord/runs/KERNEL-TIMESIG-1/`（RUNBOOK+三轮 run 目录文本工件；runtime/ 二进制不入库，哈希钉 RUNBOOK）
- **零栈验证**（命令与退出码）：
  - `go build ./...` exit 0；`go test ./... -count=1` exit 0（全绿）
  - 定向：`go test ./internal/tools -run 'TestTimeSignatureToolIsCataloged|TestMusicKeyfieldToolsAreCataloged|TestTransportLoopToolsAreCataloged|TestModelSummary'` ok；`go test ./internal/harness -run TestNormalizeCommandArgsSetTimeSignature` ok
  - 内核测试：`ctest -C Release -R VitTimeSignatureTests` **Passed**（直跑 VERDICT PASS failures=0：A 命令面+A5 fail-closed 十反例零写入+B W01 串解析 18 反例+C 2/4 saveAs→loadEditFromFile 回读+D §11 旧工程兼容[D1 显式 4/4 旧工程互操作/D2 bare TIMESIG 缺字段=4/4 缺省/D3 未知值 99 不被命令洗白]）
  - 全量 ctest 15/15 passed（Release）；VitApp 主目标 Release 构建成功（`kernel-build/VitApp_artefacts/Release/VitApp.exe`，sha256 7f20cb54abe63cae…c0a56baefca6f30）
  - 三门：webui `npm run test` 598 passed (48 files) exit 0；`npm run build` exit 1.69s 成功 exit 0；**`npm run test:render` 基线即红**（LOADING/FIDELITY/REC-TAPE PASS；SHADOWBOX+LAUNCHER 两面 FAIL——本卡 webui 零触碰，属域外在飞卡事项[LAUNCHER-ZEROFRICTION-1 doing]；跑该门曾重生成 coord/runs/UI-*/ 报告与截图=§10 污染，已定向 git restore 还原零残留）
- **真栈烟测（正式轮 C，run_20261116_0422）**：driver `dev_agent_smoke.ps1 -StartKernel -RestartAgent -TimesigSmoke -NoChatSmoke -NoStripSilenceSmoke -KernelExe <run-dir runtime>` **exit 0**，T1-T5 全绿（设置→回读→保存→重开一致：T4 .vit 存→开 music_context 3/4）。**run ID=run_20261116_0422**；工件分支=`port/kernel-timesig-1`；RUNBOOK=`coord/runs/KERNEL-TIMESIG-1/RUNBOOK.md`（§8 无效轮 A/B 分记：均 probe_defect 装置域[信封形态/断言锚定]，零产品断言失败、零环境中断、零原样重跑；§9 正式轮记录）。**LLM 计费上界=¥0**（-NoChatSmoke+全确定性 ZMQ 面）。栈登记链接：占用 04:17/释放 04:23 两行@`coord/resources/PC-RUNTIME-STACK.md`（main 853f1f23→42848912）；TEARDOWN_ALL_ZERO 亲证 04:23:20；S10 recents 还原全哈希亲证 d9b53ba7c1c9a332f8bf…54fa9f05（=基线）。
- **端测覆盖边界声明（AGENTS §5）**：真栈腿覆盖内核命令面+music_context 读面（kernel+agent 两件真实进程）；**不覆盖** agent HTTP 工具面运行时调用（catalog/归一化由 Go 单测覆盖，374 M3 消费时走到）与 webui 渲染面/用户旅程（本卡零 webui 触碰）。
- **产品语义发现（非阻塞，供主管知悉）**：VSP canonical 命令图不含 set_tempo/set_time_signature/new_project 等工程级命令——它们走 `legacy.command` 信封（`VspKernelReference.cpp:2993`）；本卡命令面与 set_tempo 同径可达，无需扩 canonical 图（若 374 需 canonical 形可另开小卡）。
- **遗留**：无阻塞性遗留；374 M3 的 2/4 工程语义已解锁（new_project time_signature "2/4" 或 set_time_signature 均可达）。

### 验收

- 待主管决策流验收（真栈腿证据链：RUNBOOK §8/§9+run_20261116_0422 工件+栈登记两行）。

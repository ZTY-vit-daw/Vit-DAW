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
- 领取提交：待推送后回填
- 资源计划：零栈段（内核实现+CMake 构建+内核测试+Go 单测）先行不占栈；真栈腿候 373（SRC-TEMPLATE-1 DG1-B）释放后按 PC-RUNTIME-STACK 协议登记

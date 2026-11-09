# LAUNCHER-SMOKE-1：启动面真栈烟测腿（332 拆卡 C6，收口卡）

- 发卡：GLM 主管决策流 / 2026-11-09；派发确认：已确认；验收负责人：GLM 主管决策流+用户 GUI 终审
- 池序 339；P1 / 1 天 / 依赖=335+336+337+338 全合入；L2 / PC；真栈（栈登记照旧）

## 目标

现有冒测脚本体系加 `-LauncherFlow` 参数（dev_agent_smoke.ps1 块或同模式新脚本段）：

1. **主链**：三件套拉起→启动台白话「打开小星星乐局」→directive→（有在开工程时）治理确认→进 dock→现有浮动窗自动带出+桥接播报首条在场→对话继续一轮（工程流标记）→exit 0；
2. **确定性退路腿**：结果卡点选开工程（零 LLM 依赖）全链 exit 0；
3. **降级腿**：不启 agent→经典磁带流 open 可用+引导卡在卷→exit 0；
4. §8 跑前口径成文（LLM 轮次数/预算/失败分类/止损线——复用 320 装置模式）。

## 文件域

`scripts/`（脚本扩展）+工件目录规范；栈协议两条登记。

## 验收标准

三腿各 exit 0+工件（report.json+日志）回指；渲染面/用户旅程边界显式声明（AGENTS §5）——GUI 最后一米=用户手测（手测 v2.5 入口）。

## 领取/回执/验收

### 领取（执行侧回填 2026-11-09 晚窗）

- 领取时间：2026-11-09（晚窗）
- 领取基线：origin/main `e91e1396ba7f7f09117e96f9a7b935dea68a7fb0`（fetch 重读亲证，卡仍在 todo、无 owner）；worktree HEAD=e91e1396、`git status --short` 空、`git diff --stat` 空（净树建枝）
- owner：ZCode GLM-5.3 L2 执行子代理（339smoke 会话）/ PC
- 分支：`port/launcher-smoke-1`；worktree：`D:/Vit_DAW_wt_339smoke`（独立 worktree，主工作树未触碰）
- 领取提交：本笔 coord/339-claim（仅 coord/ 变更，推 main 后核对远端卡面 owner）
- 前置核对：卡在 todo ✓ 派发已确认 ✓；依赖亲证=agent 仓 origin/main 含 launcher_flow.go/launcher_flow_test.go（337/338 已合入）✓、前端仓 D:/Godot/project/vit-daw-frontend HEAD=ff301b3=本地 main（335/336 启动面在场：launcher_chat_panel/launcher_nav_bridge/launcher_governance_dialog+71 检探针）✓；doing 三卡文件域核对=WAVESHELL-LOADING-FORENSICS-1 判别腿候栈候用户定向（其 scripts 域=既有 -WaveshellStacklegSmoke 参数复用，无在飞 diff；本卡 -LauncherFlow 为新增参数块，不相交）、ARRLOOP-AUTHBRIDGE-1（harness 装载确认域）、CLOSURE-ROUTE-ADAPT-1（chat 路由/closure 预算域）零冲突 ✓；真栈腿将按 §2.2 登记（当前栈记录=空闲 20:09 释放）

### 回执（执行侧回填 2026-11-09 深夜，done 候主管验收）

**总判：腿 3 降级 exit 0 PASS；腿 1 主链 exit 4 取证上交（337 launcher 门在真栈结构性不可达——RUNBOOK 预声明主风险坐实，双层证据）；腿 2 同门阻断如实记录不虚报。装置（-LauncherFlow 三腿+py 驱动+RUNBOOK）全量交付于 port/launcher-smoke-1。**

- **§8 口径**：coord/runs/LAUNCHER-SMOKE-1/RUNBOOK.md（commit 4a195967 先于一切真栈运行冻结；attempt1 后修订 68c9600a 如实入卷）。
- **栈两条登记**：占用 2026-11-09 23:19（main 509551c4，净空亲核 23:18:18 七面全零）→释放 23:51（main 065761b6，TEARDOWN_ALL_ZERO 亲核 23:50:54 全零+内核 recents 配置原样还原哈希一致亲证——种子机制净差零）。
- **三腿命令与退出码**（全部 `-RepoRoot wt_339smoke -LauncherFlow -NoChatSmoke -NoStripSilenceSmoke -KernelExe <部署内核 sha256 b6565dcf…> -RunArtifactsDir <run>`）：
  - 腿 1 Phase A（`-LauncherFlowLeg main -StartKernel -RestartAgent -StartUI`，产品真实 boot）：run=main_A_20261109_232310，**exit 4 `launcher_gate_closed_at_boot`**——CurrentProjectIdentity.uuid=vitproj_ce9e…（repo default XML 自带）→337 门（uuid==""）关闭；attempt1（main_A_231933）=脚本缺陷 env_interrupt（-StartUI 内核端口已监听时 no-op→腿块自启 UI 修复）零 launcher LLM；
  - 腿 1 Phase B（+`-LauncherStripBootUuid`，RUNBOOK 预声明假设缩小跑）：run=main_B_20261109_232613，**exit 4 同断点第二败停手**——剥 uuid 后仍 uuid="project_current"（VSP 紧凑快照无条件占位→shadow 恒非空）=**门在有任何内核挂载时结构性不可达**；**greeting 端点真栈全契约 PASS**（我在+《小星星乐局》+recent_count=16）入卷；
  - 腿 2 退路：**BLOCKED_BY_SAME_GATE**——结果卡走同一门，A/B 双层证据已证关，按 §8 不再消耗（不虚报 exit 0）；
  - 腿 3 降级（`-LauncherFlowLeg degraded -StartKernel -StartUI`，不启 agent）：run=degraded_20261109_234622，**exit 0**——真实 UI 引导卡锚在场（agent-not-ready 警告）+agent 端口全窗净空+内核面 recents（11 项完好）/open ok+headless 探针 **71/71**+telemetry 0 行；attempt1/2/3=三处脚本缺陷逐修（默认 agent-HTTP 腿跳过/greeting 锚点改单锚 record-only/recents 种子单 fixture 标量解包@() 修复+原快照重 seed 取净证）。
- **§8 对账（LLM 实耗 vs 预算）**：预算 腿1≤10/腿2=0/腿3=0；**实耗 launcher 链 0 调用**（门关双取证实测零消耗，未达 LLM 段）；唯一消耗=attempt1 默认 strip 腿 1 次 deepseek-flash（706 in/353 out ≈¥0.001，非本卡断言面，此后 -NoStripSilenceSmoke 关闭）。零超支。
- **主发现（取证上交，域外不扩修）**：agent/internal/chat/launcher_flow.go:64-73 `launcherFlowApplicable` 要求 uuid==""，但 ①内核 boot 恒载 default XML（uuid 在卷）；②即便剥除，VspKernelReference.cpp:721 无条件 project_id="project_current"→shadow.go:196 恒非空。336 UI 治理自查用 **path** 语义、337 门用 **uuid** 语义——两卡「无工程」判据不同构。launcher 确定性路由（directive/结果卡/白话直达）在产品路径不可触发；零栈单测以 fake kernel 绕过了该面。**建议**：修复卡对齐判据（path 语义或占位识别）→复用 339 装置重跑腿 1/2（装置与命令已就绪）。
- **实现 commits（port/launcher-smoke-1）**：4a195967 RUNBOOK / 14705743 装置 / 68c9600a+d3426dde+6b2f9250+33b5afc2+a65164fa 四处脚本缺陷修复（每处附 attempt 教训）/ 工件 run 目录 7 个全保真+report.json。测试命令=三腿 ps1 调用（上文）；探针零栈自检 71/71 exit 0（probe_selfcheck_prestack.txt，首跑 .godot 缓存陈旧挂起按环境中断处理→--import 刷新后正式自检）。
- **渲染边界声明（AGENTS §5）**：断言面=agent HTTP 回执字段+内核 ZMQ（UI 同链消费复刻：open_project[encryption=app_bound_aes]+version.project_opened notify——因门关未达）+真实 UI 进程/日志/内核命令锚点+headless 探针 71 检。GUI 渲染观感/真实点击/浮动窗视觉呈现**不在断言面**——GUI 最后一米=用户手测（手测 v2.5 入口）。腿 3 全降级下 UI 自身 open/recents 传输链（UDP 4445→agent bridge）无 agent 时不可用属架构事实：该腿口径=内核命令面可用性+UI 消费链 headless 覆盖，如实声明。
- **停止条件对照**：前端链路未断（335/336 消费面 headless 71/71 全绿）；断点两连败触发=§8 停手如实执行；域外缺陷（337 门）只给锚点+最小复现（RUNBOOK Phase A/B 配方即复现装置），未扩修。

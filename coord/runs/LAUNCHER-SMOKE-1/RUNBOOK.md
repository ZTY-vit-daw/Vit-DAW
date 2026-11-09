# LAUNCHER-SMOKE-1（339）§8 跑前口径（冻结于任何真栈运行之前）

- 卡：coord/cards/doing/2026-11-09-LAUNCHER-SMOKE-1.md（332 拆卡 C6 收口）
- 基线：origin/main e91e1396（337/338 已合入）；前端 D:/Godot/project/vit-daw-frontend HEAD=ff301b3（335/336 已 fast-forward 入本地 main）
- 执行：ZCode GLM-5.3 L2 执行子代理（339smoke 会话）/ PC / worktree D:/Vit_DAW_wt_339smoke / 分支 port/launcher-smoke-1
- 断言体：scripts/launcher_flow_smoke.py + dev_agent_smoke.ps1 `-LauncherFlow` 块（本卡交付物）
- 栈协议：coord/resources/PC-RUNTIME-STACK.md 两条登记（占用+释放）；内核=部署二进制 D:/Vit_DAW/Export/staging/runtime/VitApp.exe（本卡零内核改动，sha256 记入登记）；agent=worktree 现场构建（含 337/338）；前端=main 检出（ff301b3）

## 栈形（每腿一次独立 ps1 调用，独立 run 目录，互不覆盖）

| 腿 | 调用形态 | LLM | agent | UI |
|---|---|---|---|---|
| 1 主链 | `-LauncherFlow -LauncherFlowLeg main -StartKernel -RestartAgent -StartUI -NoChatSmoke -KernelExe <部署内核> -RunArtifactsDir <leg1目录>` | 有（仅续聊轮） | 起 | 起 |
| 2 退路 | `-LauncherFlow -LauncherFlowLeg fallback -StartKernel -RestartAgent -StartUI -NoChatSmoke …` | 零（telemetry 断言 0 行） | 起 | 起 |
| 3 降级 | `-LauncherFlow -LauncherFlowLeg degraded -StartKernel -StartUI -NoChatSmoke …`（**不** -RestartAgent，agent 端口必须净空） | 零 | 不起 | 起 |

三腿全部 `-NoChatSmoke -NoStripSilenceSmoke`（默认 chat 腿与默认 strip-silence 腿均消耗 LLM 轮——23:19 attempt1 实测 strip 腿 1 次 deepseek-flash 调用，不属本卡断言面，一并关闭保 telemetry 对账纯净）。腿间由 owner 亲停上一腿残留进程（agent/内核/Godot），TEARDOWN 确认后再起下一腿；同腿内单 run 无重跑。

**修订记录 2026-11-09 23:2x（attempt1 后，跑中修订如实入卷）**：①`-StartUI` 在内核端口已监听时是 no-op（D1 语义=假定 UI 拉起内核）——339 腿自持三件套，改由腿块自启 Godot UI（同参同 VIT_SKIP_DEV_AUTOSTART 守卫）；attempt1 因此以 env_interrupt 收场（零 LLM 于 launcher 链、strip 默认腿耗 1 调用），按 §8 脚本缺陷路径修复后该腿重跑 1 次。②命令行补 `-NoStripSilenceSmoke`。

## 成功条件（逐腿，全部满足=exit 0）

- **腿 1 主链**：greeting 端点 ok（launcher=true+我在）→ launcher chat「打开小星星乐局」（context.launcher=true）回执 `launcher_directive`（action=open_project、file_path=fixture、requires_governance=true、context_flow=launcher、确认句含《小星星乐局》）→ 治理自查前提（/agent/state current_project.path 空=零摩擦）→ UI 同链消费（kernel open_project[encryption=app_bound_aes]+version.project_opened lifecycle notify）→ 绑定观察（current_project.path==fixture）→ 同会话次轮回执桥接播报（stop_reason=launcher_bridge_broadcast、context_flow=project、typed_events[0].type=volume_switch、reply 含「已进入《小星星乐局》」）→ 再一轮工程流对话（context_flow=project、reply 非空）→ 真实 UI 侧锚点（Godot 进程存活+内核日志含 UI 经 bridge 拉取的 get_recent_projects+无 launcher 相关 script error）。
- **腿 2 退路**：launcher chat「打开退路」回执 `project_result_cards`≥2（kind=launcher_project、内嵌 launcher_directive）→ 点卡[0] 同链开工程（零摩擦+open+绑定+桥接播报）→ telemetry 恰 0 行（零 LLM 证成）。
- **腿 3 降级**：agent 端口全程无监听+无 VitAgent 进程 → Godot 日志含降级锚点（"VitAgent spawned but /agent/tools did not become ready" + "launcher greeting endpoint unavailable"）→ 内核面经典流数据源可用（get_recent_projects）+ open_project 可用（降级磁带 fixture）→ 前端 headless 探针 71/71 exit 0（引导卡形态在卷+磁带卡+directive 消费链）。
- **边界声明（AGENTS §5）**：GUI 渲染观感/真实点击/浮动窗视觉呈现不在断言面（headless 探针覆盖节点树+数据绑定+状态机；真实 UI 以进程/日志/内核命令锚点证在场）——GUI 最后一米=用户手测（手测 v2.5 入口）。腿 3 的 UI 侧 open/recents 传输链（UDP 4445→agent bridge）在零 agent 时不可用属架构事实：内核命令面可用性+UI 消费链 headless 覆盖即本腿断言口径，如实回执。

## 运行次数与 LLM 预算

- 每腿 1 次运行，无预演无重跑；探针（headless，零栈）允许在真栈前跑 1 次语法/装配自检。
- LLM 计费上界：**腿 1 ≤10 调用**（仅「续聊一轮」消耗；launcher 路由/桥接播报/greeting 均确定性零 LLM）。腿 2/3 = 0。三腿合计上界 10 调用 ×40k in/8k out（参照 282/327 计费口径 ≈ ¥0.05 以内）。
- 实际调用数以 VIT_AGENT_LLM_TELEMETRY_PATH 行数对账（腿 1 ≤10、腿 2 ==0、腿 3 无文件==0）；超预算即失败类 budget_exceeded。

## 失败分类（exit code 语义，分开记录不合并）

- `0` PASS（该腿成功条件全满足）。
- `2` env_interrupt：内核起不来/端口被占/构建失败/Godot exe 缺失/进程中途崩——保留原始 console+日志，不算有效轮。
- `3` intent_unresolved / mis_route（LLM 面）：腿 1 续聊轮 context_flow≠project 或空回复；或 launcher 轮意外落入 LLM 回落（回执无 launcher_directive 且 telemetry 出现行）。
- `4` 确定性面缺陷（取证上交，不重跑）：directive/结果卡/桥接播报/绑定观察/引导卡锚点/内核 open 任一断言失败；**含 `launcher_gate_closed_at_boot`**（见下）。
- `budget_exceeded`：telemetry 行数>上界（记为 4 类取证，含实际行数）。

## 已预声明的主风险：launcher 门（337 `launcherFlowApplicable` 要求 CurrentProjectIdentity.uuid==""）

静态勘察结论：内核 boot 恒加载 workspace default XML（repo 版内含 vit_project_uuid=vitproj_ce9e…，模板首启也会在 boot-save 时盖章 uuid）→ 真栈 boot 后 uuid 大概率非空 → launcher 确定性分支（directive/结果卡）可能永不触发。处置（跑前冻结，不因结果倒推）：

- **Phase A（产品真实 boot）**：腿 1/2 先以标准 GUI 腿隔离副本（git-clean default_project.xml，含 uuid）boot。驱动 Step 0 亲证 /agent/state current_project.uuid——空则全链照跑；**非空即记 `launcher_gate_closed_at_boot`，零 LLM 消耗 exit 4 取证**（附 state 回执+greeting 探针结果），随后转 Phase B。
- **Phase B（门开合成态，仅当 A 门关）**：同副本剥除 vit_project_uuid 属性（`-LauncherStripBootUuid`）重新起栈跑全链。B 过=链路本体证成+A 门关=产品面阻断点（两事实并列上报，主管裁定修复卡归属 agent 域）；B 也败=更深断链，取证上交。Phase B 是**假设缩小诊断跑**（换 boot 配置非原样重跑，AGENTS §8 允许项），在卡面如实标注合成态边界。
- 同一确定性断点两连败（同配置）→ 停手上交。

## 止损线

- 任一腿 exit 4 后：不原样重跑该腿；先读工件定位归属（agent 域=337/338 行为 / 前端域=335/336 / 内核域 / 脚本缺陷）。**前端域断链→停卡取证上交**（前端仓修改需另卡或主管授权）；脚本自身缺陷→修复后该腿重跑 1 次（记 attempt，仍败即停）。
- LLM 实耗>10 调用（腿 1）→ 停止后续 LLM 轮，已耗轮如实记录。
- 栈异常（端口互染/进程僵死）→ 按环境中断记录，TEARDOWN 净空后从头登记再续，不续算有效轮。

## 隔离与防污染（§10）

- 工程 fixture：由 %APPDATA%\Vit-DAW\projects\手测·小星星乐局.vit（311 布位件，只读源）**复制**到各腿 run 目录 fixtures/（腿 1=小星星乐局.vit+小星星乐局·副本.vit；腿 2=磁带流·退路甲/乙.vit；腿 3=降级磁带.vit）——不触源件、不写源码树。
- recent 列表（%APPDATA%\Vit-DAW\global_project_config.json，内核管理态）：每腿起栈前快照+prepend 本腿 fixture 路径；**全部腿结束+栈拆净后还原快照**（净差零，优于 309/327 遗留先例）。运行中内核自身的 prepend 属内核在管写入，不手工干预。
- boot 工程：GUI 腿共享隔离块 VIT_PROJECT_XML=git-clean run-dir 副本（SMOKE-STACK-GUARD-1 先例）；Phase B 仅在该副本上剥 uuid 属性（run 内文件，不触 repo 版）。
- agent 草稿根：脚本既有 VIT_HISTORY_DRAFT_ROOT 隔离（%TEMP% per-invocation）。
- 真实用户工程/ sealed fixture 零触碰。

## 工件

- 每腿：`coord/runs/LAUNCHER-SMOKE-1/<leg>_<yyyyMMdd_HHmmss>/`（console 全文、summary.json、telemetry 副本、godot log 副本、内核 log 副本、recents 快照前后、state/directive/桥接回执原文）。
- 收口：`coord/runs/LAUNCHER-SMOKE-1/report.json`（三腿对账：命令、exit code、telemetry 实耗 vs 预算、门状态、锚点清单）+卡面回执。全部随 port/launcher-smoke-1 推远端。

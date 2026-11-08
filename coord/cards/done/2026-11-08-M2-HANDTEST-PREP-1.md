# M2-HANDTEST-PREP-1：M-2 手测前置腿（乐局工程布位+旅程可达核验+282 轻腿搭载）

- 发卡：GLM 主管决策流 / 2026-11-08 晚窗（依据=11-07 gate 首排「手测前置三件套→明晚交用户手测」+282 验收「真栈轻腿候栈」——三合一一次栈占用）
- 派发确认：已确认（主管发卡）；验收负责人：GLM 主管决策流
- 池序 311；目标仓库=主仓 scripts（复用 ps1 体系）+工件；P1 / 0.5 天 / 依赖=310 释放栈；L2 / PC；真栈占用一次

## 目标

1. **乐局工程持久布位**：first-run-twinkle fixture 工程（theme 42 音+corrupted 平行轨+空白伴奏轨，281 smokerun3 同款内容）布到**用户可从 Godot 启动页打开的持久位置**（勘察启动页工程列表来源——agent workspace/projects 体系），工程名「手测·小星星乐局」；布位后启动页可见可开（API 级验证）；
2. **旅程可达核验**（真栈三件套，dev_agent_smoke.ps1 -StartKernel -StartUI 同款模式）：栈起→工程打开→chat「开始乐局」enter 生效（enter 回报在案形态）→一轮推进检查（「写好了」n0 过）——**Godot GUI 最后一米（点选工程）是用户手测本体，不在本腿断言面**，边界如实声明；
3. **282 真栈轻腿搭载**：chat 驱动 `transport.set_loop`（如 10-20 秒循环区）→读回确认→`transport.clear_loop`——282 卡验收标准的真栈兑现；
4. **生成路由活体核验**（轻）：chat 一句生成类话术→确认路由进生成域（303 链回报形态；不等 wav 全产出可停在路由确认，wav 全链已 278 exit 0 在案）；
5. 全腿 exit 0=手测放行；工件目录+步骤清单补记（coord/transfer/2026-11-08-M2-handtest-steps.md 前置段回填）。

## 约束

- 文件域=scripts 冒测脚本扩展（加参数优先）±工件+步骤清单补记；agent 源码零改动；乐局内容/level.yaml/persona 零触碰；
- 栈排队：**候 310 释放后领取栈**（重读登记，不抢）；296-R3 在栈时同理等待；
- 工程布位不得污染源码树/权威 fixture（写入=持久工作区目标位置，§10）；§8 跑前成文（预算：乐局面确定性零 LLM+生成路由 1-3 调用 deepseek-flash ≈¥0.1 内）。

## 停止条件

启动页工程列表机制勘察发现无用户可达路径（需前端改动）→取证上交转裁定（手测降级为 smoke 工程路径直开）；旅程断言失败取证不重跑。

## 领取

- 2026-11-08 晚窗 / origin/main `ad108490`（注：任务指令基线号 ad109490 为笔误一位，fetch 实际 HEAD=ad108490=310/栈登记同步笔，以实际为准）/ owner=ZCode GLM-5.3 L2-worker PC 执行子代理（会话 311prep）/ 分支 `port/m2-handtest-prep-1` / worktree `D:/Vit_DAW_wt_311prep`
- 领取基线：HEAD=`ad108490`；`git status --short` 空；`git diff --stat` 空（净树建枝）
- 领取时状态：**310 A-pure 占栈中（19:11 登记）——本卡候栈**（轮询重读栈记录）；282 未合入 main 亲证（origin/main `agent/internal/tools/catalog.go` 无 transport_set_loop）——按主管预裁 282 轻腿改**离线证据**（282 分支单测复跑）+步骤清单标注「set_loop 体验随 282 合入后生效」
- 在飞域核对：310（arm_apure_orchestrator.ps1+PLUGIN-CONTROL-EVAL-APURE-1 工件）/282（agent Go 域已实现候栈）/309（C++ 域）与本卡（scripts 冒测扩展+M2-HANDTEST-PREP-1 工件+transfer 步骤清单回填）不相交

## 回执

### 2026-11-08 晚窗执行完毕——**真栈单 run PS1 exit 0=PASS（手测放行信号）**，卡自验完成 mv done/ 待主管终验

- **实现 commit**：`81c39657`→`c70276a7`（零栈笔：driver+ps1+RUNBOOK+两笔 driver 判定精化）@ `port/m2-handtest-prep-1` 已推；本回执与工件随收尾笔同分支推送。**agent 源码零改动兑现**（diff=scripts 三件+工件+步骤清单）。
- **run**：`prep1_20261108_194514`（19:45:14 启动）；完整命令/退出码/二进制哈希/工件清单见 `coord/runs/M2-HANDTEST-PREP-1/prep1_20261108_194514/evidence.md`；**PS1 exit 0**（=§8 预写 PASS=手测放行）。agent=现场构建 SHA256 `7CB6E18A…272C8`（@c70276a7）；kernel=wt_228 `F9E3DA0A…D67A` 只读复用（310 同款，19:38 亲核）。
- **目标 1 布位 ✓**：`%APPDATA%\Vit-DAW\projects\手测·小星星乐局.vit`——3 用户轨（1007/1010/1013）+t1 clip 42 theme 音+t2 clip 42 corrupted 音（**beat37=pitch68 锚**+t2 mute=true 防剧透+t3 留白）；**get_recent_projects 首项=布位路径**（启动页列表同源面：勘察修正——列表来自内核 VitGlobalProjectConfig[%APPDATA% config]而非 agent workspace 体系，Godot start_page→project_client→内核同链）；重载后自 state clips[] 取 id 回读 42/42；agent 工作区（.vit_agent/.vit_history）随 .vit 同目录预热在场。布位写入=%APPDATA% 用户数据域，源码树/权威 fixture 零触碰（§10）。
- **目标 2 旅程可达 ✓**：三件套（-StartKernel -RestartAgent -StartUI；Godot 前端进程在场 GUI 面不在断言面）→ activated=true → **enter_attempts=1 一次过**（edu_jam_entered/first-run-twinkle/n0，回复含乐局标题+K.265 叙事）→ **n0「写好了」过**（node_done=n0→n1 读谱纠错）→ exit 终态（user_exit；用户手测新会话干净开局）→ **乐局面零 LLM**：J 完成时点 telemetry 文件未创建（零调用旁证）+G 段 4 行逐行归属亲证全属生成路由（乐局 3 轮零行）。
- **目标 3（282 轻腿）按主管预裁条件化兑现**：领取时亲证 282 未合入 main（origin/main catalog.go 无 transport_set_loop）→ **离线证据模式**：wt_282impl（@873c0e9c 只读）四组复跑全绿——`go build ./...`=0；`go test ./rules/... ./internal/tools/ ./internal/policy/ -count=1`=0；`go test ./internal/chat/ -count=1`=0（162.2s 全包）；`go test ./internal/agentloop/ -count=1`=0。步骤清单已标注「set_loop 体验随 282 合入后生效」。**未自行合入 282**。
- **目标 4 生成路由 ✓**：自然话术 turn1 即中——三证据齐：①编排受理（task_id/goal_id）②响应含 gen.generate 工具痕迹 ③prompt render 2 行 gen.generate 披露；**stop_reason=needs_confirmation**（治理确认门正确拦截=零裸写红线保持，比引擎失败更干净的路由完备形态）；不等 wav（278 exit 0 在案）。
- **目标 5 ✓**：步骤清单前置段已回填（coord/transfer/2026-11-08-M2-handtest-steps.md §〇：生效声明+工程名入口确认+**必做环境前置 VIT_EDU_LEVELS_DIR**[前端自启 agent 无此 env 乐局 enter fail-closed，勘察锚点 edu_jam_chat.go:1109 回退相对 cwd]+旅程 B 前置[YuE2 引擎+providers]+282 标注）——随 port 分支等主管合入后对用户生效。
- **计费（如实）**：4 行 deepseek-flash（全生成路由段，error 全空）——申报预算 1-3 **超限 1 行**，summary.budget_exceeded=true 如实记录（金额≪¥0.1 上界）；乐局面零 LLM。
- **栈纪律**：候栈（310 19:42 释放在卷亲证后）→净空两轮亲核 19:42:46+19:43:41 全零→占用登记 `1e555212`→≤60s 即时复核 19:44:59 全零（owner 311 亲证）→启动；PS1 完成态不自拆→owner 亲拆 VitAgent 9020/VitApp 12024（Godot 随 run 收尾先退）；**TEARDOWN_ALL_ZERO 亲核 19:47:10**；释放登记 `30b526d9`（main 在卷，当前空闲）。
- **端测边界声明（AGENTS §5）**：真栈面=agent HTTP/ZMQ 断言；**渲染面与用户旅程面不在断言面**——Godot 启动页点选工程、聊天面板视觉=用户手测本体（最后一米）；Godot 前端进程本 run 在场（三件套）但 GUI 未断言。
- **次级观察（如实）**：`ps1 exit 0` 前 default 面（tool_count=296/shadow/range_context 等）全绿零 warn；-NoStripSilenceSmoke 新增传参（默认腿会动隔离副本工程，关掉保最小面——281 形态差异如实申报）。

## 验收

- （主管回填）

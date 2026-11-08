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

- （执行侧回填）

## 验收

- （主管回填）

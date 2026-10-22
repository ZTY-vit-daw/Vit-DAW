# SOFTGUIDE-RESCHEDULE-1 §8 预声明（跑前锁定，跑后不得改）

- 卡：coord/cards/doing/2026-10-21-SOFTGUIDE-RESCHEDULE-1.md（池序 159）；本件落笔于任何产品轮之前（2026-10-22 10:2x +0800）
- 协议权威：[146 实验工件 §E 原协议](../../runs/146-SOFT-GUIDE-EXP/20261020_b2arm/RESULTS.md)（原文=coord/reports/2026-10-20-146-timefx-design-and-experiment.md §一/§E）+[156 设计稿 §3](../FS-FRONTIER-OBSERVE-GAP-1/DESIGN.md)
- 领取基线：origin/main=e101e19a；被测 agent=本卡 worktree 构建（生产码=main，9afbe12b..e101e19a 的 agent/+VitApp/ diff 为空）；**生产代码零改动**

## 1. 臂与次数（§E 原文沿用）

- **B2 臂（惯例 ON）**：10 变体 ×1 run（A1–A5、B1–B5）。惯例段=**生产 carriers.go:137（154 合入）**，无需补丁分支；每轮 prompt_render 快照亲核惯例段在场（146 attempt-2 补丁失活教训的机械面）。
- **B1 对照臂（现状提示词=同生产 agent，B2 判读已锁硬规则分支后两臂提示词同源）**：5 run（A1、B1、B2、B3、B4，§E 原名单）。职能收窄：仅作 F-gate 计数与到达率的对照采样，**不做惯例差分判读**（本腿不重判惯例效应，差分解读交主管）。
- 每变体=新会话（新 conversation/goal）+观察前置话术+full_project_access（驱动亲设+/smoke authority 探针）——§E 隔离边界原样。
- 驱动批次：B2 逐批 2 变体串行（批间机械计数），B1 同。fixture 每变体 project.open 复位（146 驱动原件行为）。

## 2. 判据（本腿只计数，不设通过线；判读交主管）

- **主判据（机械计数）**：①F-gate 计数（§E F-gate=未达提案：门拒或自报边界，含 target_identity 弹回类；逐轮留证）；②**提案面到达率**=到达提案（提案卡/typed controller/终答计划文本）的 run 数 ÷ 有效 run 数。
- **硬判据（两臂通用，§E 逐字）**：0 幻觉拓扑——提案引用的轨/send 目标必须存在于观察披露或本提案新建清单；运行后 track.send.list 读回亲核。任一幻觉=该 run 硬失败。
- F-inline / F-notopo：只如实分类计数。**本腿不构成"失败"**——B2 拓扑判读已预绑定锁硬规则分支（146 §0），惯例路由效应不在重判域。

## 3. 失败分类（§E ×5 逐字）

F-inline（inline 装载源轨）/ F-notopo（提案无位置主张）/ F-halluc（幻觉拓扑，硬失败）/ **F-gate**（未达提案：门拒或自报边界——含 target_identity 类）/ F-env（环境中断，须原始日志或退出证据方可出有效轮，AGENTS §8）。
驱动形态无效轮（127 卡 / 146 attempt-1 先例）单列，不占产品轮。

## 4. 止损线

1. **F-gate 任何一次出现**：捕获该轮原始工件后，主线暂停计数，另跑 1 枚新变体复核签名；**第 2 次 F-gate（或同签名重现）→ 全停**，保存原始工件上交=157 修复失效证据候选（是否失效交主管判读）。原样重跑同一变体 ≤2 轮（卡面停止条件）。
2. **F-halluc 任一出现**：该 run 硬失败并全停上交（§E 硬判据面）。
3. F-env：按 AGENTS §8 单列不计有效轮；环境修复后重跑不占产品轮。
4. 驱动自身缺陷轮：修复尝试 ≤2 次后停止，已捕获工件如实上交。

## 5. LLM 记账（§8）

- 模型=deepseek-flash 思考档（与 146 同款，栈 config 不动）；预算 **≤90 次调用**（15 run × 每轮约 3–6 调用 + authority 探针；fixture 构建 0 LLM）；0 error 期望如实记录，超预算即停。
- 无重试轮消耗原则沿用 146：驱动缺陷轮/环境轮不占产品轮。

## 6. 运行环境

- 真实栈（修订于起栈前，157/158 实证形态）：**手动起栈三进程**——内核=K3 build 722A79D7 主检出 Release **只读复用**（跑前 SHA 哈希亲核；含 K1 send 命令族，track.send.list 读回可用；VIT_PROJECT_XML 指 run 目录工程副本、内核工作目录=run 目录 kernel_workspace/，主检出 workspace 零触碰）+ vsphub=本卡 worktree 构建（8787，agent 自动注册）+ agent=本卡 worktree 构建（=main 生产码，HTTP 7878，工作目录=run 目录，日志/遥测/prompt_render 全落 run 目录 VitApp/Workspace/Logs/）。**Godot UI 不启**——chat 链判定面不涉 GUI（157/158 先例形态），端测边界见 §8。fixture 由驱动在 run 目录自建（§10 隔离）。
- 栈占用：**今日序末位**——162/164 真栈腿释出后，按 PROTOCOL §2.2 登记 PC-RUNTIME-STACK 再跑；释放后复核端口/进程归零。

## 7. 工件契约

per-case（prompt.txt / chat_*.json / hop_*.json / events.json / send_list_{before,after}.json / project_state_after.json / runtime_status_final.json / variant_summary.json / **prompt_render_snapshot.jsonl**（本腿新增采集：惯例段在场+frontier 披露亲核面））+ session_summary/session_log + 跑后 agent 日志/llm_telemetry/message_loop_debug 快照。工件根=本目录之下新建 run 目录；判读面回指原始工件。

## 8. 端测边界声明

本腿=agent HTTP/ZMQ 面 chat 链路真栈（真实内核+真实 deepseek 链+真实工程副本）；**不含** Godot GUI 渲染面与 webui 渲染面断言（146/157 先例同界）。提案面到达的判定以 chat 响应/events/内核回执工件为据，不以 GUI 呈现为据。

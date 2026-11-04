# RECEIPT — M3-CONTINUATION-PARK-STORM-1 真栈腿（parkrun1/2/3 三 run）

- 被测：worktree D:/Vit_DAW_wt_248impl @ port/m3-continuation-park-storm-1，实现 commit a44dafa0（唯一 diff=chat 四文件 +252/-6）；agent=worktree 构建（parkrun1 23:36 首建，后续 run 复建）；kernel=wt_228 VitApp.exe SHA256 前缀 f9e3da0a66263f0b 只读复用（23:34 哈希亲核）。
- 场景：dev_agent_smoke.ps1 -RepoRoot D:\Vit_DAW_wt_248impl -StartKernel -RestartAgent -KernelExe <wt_228 VitApp.exe> -AutomationLLMRouteSmoke -LlmRouteGroups 1 -LlmModelOverride deepseek-flash；三 run 全新栈（每 run 前 owner 手拆+零态复核：23:36:27 / 23:57:12 / 00:11:48，均 PROCS=0+TCP 六口=0+UDP=0，登记-启动 ≤60s）。
- 占用登记 5fff9ede（23:34，登记前全零 23:34:49）；拆栈 TEARDOWN_ALL_ZERO 亲核 00:45:xx（procs=0 tcp=0 udp=0，owner 手拆 VitAgent/VitApp×3 轮）。

## 判定：3/3 ps1 exit-0 稳定 run 达成（链收口前置件）+park 零风暴三轮全程

| run | 启动 | ps1 exit | 组 1 verdicts（5 样本） | park 行总量/速率 | 7878 拒绝 | choreo 腿 |
|---|---|---|---|---|---|---|
| parkrun1 | 23:36:30 | **0** | tool_chain_ok ×5（points=3/6/5/4+choreo 2 点写达读回） | 38 条/257s=0.15 条/s，峰值 1/s | **0** | yes |
| parkrun2 | 23:57:1x | **0** | tool_chain_ok ×5 | 37 条，峰值 1/s | **0** | yes |
| parkrun3 | 00:11:50 | **0** | tool_chain_ok ×3 + **no_route ×2（r4/r5）** | 51 条，峰值 1/s | **0** | yes（choreo_pending_g1.json points 2 写达内核读回） |

- **3/3 exit-0**：247 裁定诚实边界（「链收口前须补一次全组 3/3 exit-0 稳定 run」）形式达成——三轮 ps1 均 dev agent smoke completed+summary 在卷（ps1:8772 throw 路径未触发）。
- **park 零风暴**：三轮 park 行均为节流形态（签名恒定时 ≤1 条/5s 节流间隔+2s 退避扫描节奏，实测 ~0.15 条/s），**无连续高频 park 行**；对照 247 r3 风暴=4 条/s×181s+（729 条）。修复直接实证。
- **answerable 恢复红线**：三轮 settle choreo 腿全过（确认卡→恢复→set_points 写达内核+读回）——park 语义未变慢，claim 路径即时性保持（单测 TestSchedulerIdleBackoffDefersIdleScanAndWakeKeepsClaimImmediate+真栈 choreo 腿双证）。
- **listener 拒绝面判定（卡面目标 2）**：三轮 python 轮询腿零 10061（247 形态 fail-fast 未复现）；agent 日志内全部连接拒绝均指向 127.0.0.1:8787（vsphub 不在 dev_agent_smoke 栈内=既有 WARN 形态，264/143/442 次三轮），**7878=0 次**。结论：247 r3 的轮询 10061 与 park 风暴同期耦合成立（修复后不复现）；VSP→8787 拒绝与风暴无关（既有形态）。
- **run3 r4/r5 no_route×2 如实分列**：旧形态模型行为残差（模型未到达写路径，非风暴非链路缺陷；r1-r3 三轮 tool_chain_ok 同 run 在场+choreo 腿写达内核），与 248 修复面无因果——是否影响「组 1 绿」口径交主管裁定，不重跑不追绿（§8）。
- **245 挂账勘察真栈观察**：三轮 agent 日志零 `[continuation.clear]` 行=clearGoalContinuation 无痕取消路径三轮未触发；245 形态（recordGoalResult 缺席/durable=cancelled vs goal=completed）本轮未复现，锚点与 observability 行已入实现（下次复现可归因）。

## 计费披露（§8）

364 次计费调用（零错误，全 deepseek-flash）：parkrun1 118 调用 in 1.84M/out 139K≈¥2.95；parkrun2 68 调用 in 1.03M/out 87K≈¥1.72；parkrun3 178 调用 in 2.83M/out 219K≈¥4.58；**合计 in 5.70M/out 445K≈¥9.26 上界**（in ¥1/M+out ¥8/M 口径，flash 实际更低）。**超卡面 ≈¥1.5-2.5 预算线如实披露**：根因=组 1 场景设计 5 样本/run（247 的 3 轮=r3 中断截断非场景全形）+run3 no_route 轮长链；单轮 ≈¥0.4 的卡面估算对应 3 样本短链形态。

## 工件与入库边界

- 入库（port/m3-continuation-park-storm-1）：parkrun1/2/3 各 rounds/*.json+automation_llm_route_summary.json+automation_llm_route_console.txt+dev_agent_smoke_console_tail.txt（**ps1 console 仅尾段存档=捕获缺陷如实记录**，全程 console 未落盘）+logs/agent_last.log（整窗快照，含 park 行时间序列）+logs/agent_llm_telemetry.jsonl+default_project.xml+gui_project_isolation.json+automation_llm_route_project.vit+本 RECEIPT.md。
- 留 worktree 不入库（243-247 先例）：logs/agent_llm_payload_capture.jsonl（7.7/4.4/12M）+logs/agent_prompt_render.jsonl（4.4/2.5/6.7M）；clip.wav 已删 ×3。

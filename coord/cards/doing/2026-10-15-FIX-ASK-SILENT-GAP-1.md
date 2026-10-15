# FIX-ASK-SILENT-GAP-1：ask→slice 启动 122 秒静默空窗（LLM 前卡点定位+反馈面）

- 发卡：GLM 主管决策侧 / 2026-10-15 早窗（依据=M2-EXPERIENCE-1 首跑取证，[README](../../runs/M2-EXPERIENCE-1/README.md)）
- 派发确认：**已确认（用户 2026-10-15 晚窗「进入晚窗，可以发卡」）**；验收负责人：GLM 主管决策流
- 池序 117；目标仓库=D:\Vit_DAW；P1 / ≤1 天（勘察先行）
- 模型分级：L2 / GLM 执行会话
- **两腿结构（2026-10-15 晚窗派发裁定）**：本轮派发=**取证腿**——零生产代码合入（临时打点仅存取证 worktree，不入 main）；取证件+修复面清单交付后停手，修复腿由决策侧核对与 116 文件域后再派。真栈取证 run 与 116 烟测腿按栈占用登记串行。

## 缺陷实锤

- 话术 09:53:11 落账 → slice/turn 09:55:13.794 才启动（**122s**），期间零轨迹事件、零 UI 反馈（用户 60s 判死、随后按停止）；
- journal_records=0 / observations_count=0：**LLM 一次未被调用**，卡点在 ask 前置上下文组装段（mixboard 快照在 +63s 才落盘，其后还有 ~60s 无工件空窗——疑有等待/超时环）；
- 对照：smoke 同链路（6 轨 fixture，run12/13）无此空窗；本场次为**空工程（0 轨）**——疑放大因素，非充分排除项。

## 修复目标（取证先行）

1. 定位 122s 的构成：ask 处理到 slice 启动之间的阶段打点（mixboard 快照触发点、DAD/CCB 预热、空工程下的等待/超时环），先出取证件再改；
2. 反馈面：ask 到 slice 启动之间必须有用户可见的忙碌状态/轨迹事件（「正在观察工程…」级），不留静默空窗。

## 文件域

agent/internal/chat ask 处理链 + goalrunner slice 启动前置段（领取时以打点/trace 实际锚点为准）+ webui 反馈位；与 116 卡（turn_control.go 停止路径）域不相交可并行。

## 验收标准

- 取证件：122s 空窗的阶段分解（含空工程 vs 有轨工程对照各一轮）；
- 修复后真栈：ask 后 ≤3s 内出现首个可见反馈/轨迹事件；空工程不再出现分钟级静默；
- `go test ./... -count=1` 0 FAIL + gofmt 净。

## 停止条件

打点发现卡点在内核侧（C++）→ 锚点上交，不扩域。

## 领取

- 时间：2026-10-15 18:06 +0800
- origin/main：e115fd8a（领取前 fetch 核对；本卡与 116/118 三张 10-15 卡此前仅存主管本地未跟踪态，随本领取提交首次入库）
- owner：GLM-5.3 L2 PC 执行会话（ZCode，D:\Vit_DAW）
- 分支：port/fix-ask-silent-gap-1-recon（取证 worktree 内含临时打点，不合入 main）+ 协调分支 coord/asg1-claim（仅卡面状态）
- worktree：D:/Vit_DAW_wt_asg1
- 本轮范围：取证腿（阶段打点定位 122s 构成；空工程 vs 有轨工程真栈对照；取证件+修复面清单）；修复腿不在本轮

## 回执

### 取证腿（2026-10-15 晚窗交付，修复腿待派）

- **取证件**：[coord/runs/FIX-ASK-SILENT-GAP-1/FORENSICS.md](../../runs/FIX-ASK-SILENT-GAP-1/FORENSICS.md)（随 port/fix-ask-silent-gap-1-recon 入库，commit 见下）
- **结论**：122s 空窗 = 语义入口分类器一次 LLM 调用（ordinary_agent_semantic_entry，deepseek-flash）在服务退化下的重试环独占——60s 超时×2 + 退避 750ms + 104ms "Service is too busy" 终拒 = **120.855s，与 M2 现场遥测 total_ms=120855 逐毫秒吻合**；其后 1.4s 失败收尾+prepare，09:55:13.794 slice 开启。journal=0 只覆盖 message loop，语义入口调用不在 journal（卡面前提修正）。mixboard 快照 +63s 落盘=内核波形遥测后台摄取（harness.go:4180 ingestKernelWaveformTelemetry），与卡点无关。
- **空工程真相**：非放大因素（语义入口载荷与轨数无关）；实为叠加缺陷——语义入口失败 fail-open 后**跳过空工程容量短路**（goalrunner_chat.go:247 只在 err==nil 进 planObservationFirstCapabilityRoute），0 轨工程本应 ~2s 短路作答（A 轮真栈实证：4.0s 全轮、零 slice），却被放进 slice 管线。
- **真栈对照**：A 轮（空工程+健康 LLM）4.0s 短路作答；B 轮（6 轨 fixture+健康 LLM）ask→slice=11.6s（前导 4.4s+语义入口 6.2s+prepare <1s）。烟测无空窗=当晚服务健康（10-14 遥测 6990/8745ms 无错），非工程差异。
- **修复面清单**（5 项，锚点=main e115fd8a 行号，详见取证件 §4）：① semantic_entry.go:132-140 入口无 deadline；② llm/client.go:157-174 默认 60s×3 尝试最坏 ~121s；③ goalrunner_chat.go:233-262 fail-open 跳过容量短路；④ server.go:2509-2523+goalrunner_chat.go:234 分类期零用户反馈（反馈面）；⑤（次级新发现）B 轮前导段 4.4s 使首个可见事件推迟至 t+4.8s，已超 ≤3s 预算。与 116 文件域：未触 turn_control.go/harness 停止路径；goalrunner_chat.go 为两卡共享文件（分类段 vs 停止段不同函数），交决策侧核对后派修复腿。
- **§8 记账**：计划 2 轮实跑 2 轮全达成；LLM 实耗 5 次全成功 0 超时（预探测 0.83s/A 轮入口 2.09s/B 轮入口 6.16s+B 轮 message loop 3.78s）；退化态 live 复现未执行（需服务退化或改全局 config，历史遥测两独立样本+常量吻合已充分，如实登记边界）。
- **临时打点**：port/fix-ask-silent-gap-1-recon 提交 0eb31135（[asg1] 阶段日志：cmd/vitagent/main.go+internal/chat/server.go+goalrunner_chat.go+internal/llm/client.go 四文件；探针 scripts/asg1_ask_gap_forensics.ps1+asg1_prompt_codes.txt）——**不入 main**，修复腿可按需 cherry-pick。
- **栈占用/释放**：占用登记 9b817024；18:21–18:26 两轮取证后手动 teardown（VitAgent 15200/VitApp 11620/Godot 12276+22764），复核四端口 5555/5556/7878/8787 监听=0、四类进程=0；释放登记随本提交。
- 取证腿到此停手；修复腿待决策侧核对派发。

## 取证腿验收（决策侧）

- **采信（2026-10-15 晚窗）**：根因链逐毫秒实锚（122s=语义入口 LLM 重试环 120.855s，与 M2 遥测 total_ms=120855 吻合）+叠加缺陷发现（fail-open 跳过空工程容量短路）+A/B 真栈对照+修复面 5 项带行号锚点；零生产代码入 main 亲核（打点全留 recon 分支 0eb31135）；§8 记账干净（2/2 达成、退化态 live 复现边界如实）。journal=0 卡面前提修正采信（语义入口调用不在 journal——决策侧早窗「LLM 零调用」表述同步勘误：LLM 被调用且在重试环中烧了 120s）。
- **修复腿解锁条件**：基于 116 合入后 main（19e5592e）重领取；文件域核对=修复面①③④触 semantic_entry.go/llm/client.go/goalrunner_chat.go/server.go:2509-2523，与 116 已合面（server.go 字段区/turn_control.go/runtime.go）不同函数区，重领取时以合入后 main 锚点为准；修复面⑤（前导 4.4s>3s 预算）为次级项，含入修复腿范围与否由新领取提交声明。recon 分支打点可 cherry-pick 复用（0eb31135）。

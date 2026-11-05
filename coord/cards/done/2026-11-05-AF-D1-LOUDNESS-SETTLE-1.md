# AF-D1-LOUDNESS-SETTLE-1：D1 全局响度与峰值能力落地（A-F 主线首卡）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=[2026-11-05-af-line-survey.md](../../reports/2026-11-05-af-line-survey.md) §三）
- 派发确认：已确认（主管发卡；用户 2026-11-05「先勘察，发卡」口头授权）
- 验收负责人：GLM 主管决策流
- 池序 254；目标仓库=主仓 agent；P1 / 1 天 / 依赖=无（M3 链已收口）；L1-L2 / PC 执行会话；验证腿需真栈（一次）

## 背景（勘察锚点）

- D1「全局响度与峰值」节点现状：观察面已收口 `pre_master.observation.v0`（capability_routing.go:658，D1-D3 共用只读 pack）；**无执行/判断 runtime**；`agent/internal/loudnesswatch/` 包在库（trigger.go 590 行+compare.go+config.go 含测试）。
- 前置腿：port/loudness-watch-trigger-1-d1 分支（worktree wt_loudness_watch_b 保留中）有 **D1 settle 重锚 LW 真栈腿脚本**（scripts/run_free_state_d1_smoke.ps1 +601 行）与两 run 工件；其 gate4 被**产品缺陷阻断上交**（缺陷描述见该分支工件，worktree dirty 72MB 内含 run 记录）。

## 目标

1. **缺陷诊断**：从 wt_loudness_watch_b 工件读取 gate4 阻断形态，在当前 main 定位该产品缺陷（只读取证→根因→最小修；若已被 M3 链时代修复则证据关线）。
2. **D1 能力落地**：以 loudnesswatch 包为判断底座，点亮 D1 节点——工程全局响度/峰值观察（复用 pre_master.observation.v0 pack）+阈值/目标判断+**待确认动作面**（如全局增益调整建议/推子动作，走既有 mix.propose_tick→确认→执行治理链；禁裸写）；capability ledger 记录使 D1 在 queue view 从 not_started 点亮。
3. **真栈验证**：真栈单 run（隔离工程+双轨 clips 同 249 先例），判定面=D1 观察正确读取全局 LUFS/峰值域可用值+判断输出+动作面挂确认卡；§8 运行口径跑前成文（预算 ≈¥2-4，单 run 无重跑）。

## 约束

- 文件域=agent/internal/loudnesswatch/（+chat 侧 D1 路由/账本接线最小面）+scripts/run_free_state_d1_smoke.ps1（承接 90 腿脚本，从 worktree 取材后改造）+测试；**不触 automation_intent/semantic_entry**（M3 域）；
- loudnesswatch 既有行为不回退（包测试全绿为底线）；
- 确认/执行走既有治理链（247 以来口径），LLM 提议≠写入；
- 判断权冻结令适用：固定文案只做识别/执行/兜底，判断归 LLM（如涉）。

## 停止条件

- gate4 缺陷根因超出本卡文件域→取证上交转卡；
- 真栈断言失败→取证包上交不重跑；栈/环境中断按 §9；
- loudnesswatch 包测试回退→停。

## 领取

- 2026-11-05 20:46 +0800 / origin/main=b46a9096（领取基线，与本地一致）/ owner=GLM-5.3 L1-L2 PC 执行会话（ZCode，254 卡会话，sess 本流）/ worktree D:/Vit_DAW_wt_254impl / 分支 port/af-d1-loudness-settle-1 / 领取基线 HEAD=b46a9096，status --short 空、diff --stat 空（干净基线）/ 真栈占用：领取时最近记录=2026-11-05 20:11 M3(225) 释放 TEARDOWN_ALL_ZERO；真栈腿启动前另行登记（§2.2），与 255 错峰

## 真栈腿跑前口径（§8，2026-11-05 21:16 成文）

- 命令（在 D:\Vit_DAW_wt_254impl 下执行）：`powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts
un_free_state_d1_smoke.ps1 -RepoRoot D:\Vit_DAW_wt_254impl -PromptFlavor reverb -ExpectDomain reverb -LoudnessWatch -D1LoudnessSettle -LlmModelOverride deepseek-flash -SkipBuild -KernelExe D:\Vit_DAW_wt_228\VitAppuild_release\VitApp_artefacts\Release\VitApp.exe`
- 运行次数=1，无重跑；LLM=deepseek-flash（VIT_AGENT_LLM_MODEL 注入，D1 runner 轮次；D1 settle 四门与 LW 四门零 LLM），预算 ≈¥2-4 上界。
- 被测 agent=worktree 构建 bin/VitAgent.exe SHA256 F321AF5E268CE2C14FB83BDA15BE1C4F73083AFC3C3BC620299E486720C6E1AD（实现 c687c9a1）；kernel=wt_228 显式 -KernelExe SHA256 F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A（225/249/251/253 同款只读复用）。
- 成功条件（脚本 exit 0）＝①D1 runner exit 0（133 run2 同形态 -PromptFlavor reverb -ExpectDomain reverb）＋②LW 四门回归不回退（133 已 PASS 面）＋③D1 settle 四门：settle 轮 stop_reason=d1_loudness_settled+观察 loudness_peak_status=present+payload checked/config_basis/mutation_performed=False；verdict store 落 d1_settle trigger 记录；动作面诚实（breach→mix_tick_confirmation 卡+取消腿 mix_tick_rejected / 非 breach→无卡，verdict kind fixture-dependent 不硬断言=90 卡先例）；mix_queue_view D1 节点≠not_started。
- 失败分类预写：a) D1 runner 模型随机分支失败（materiality missing/budget exhausted，133 run1 先例）→取证上交不重跑；b) 栈/脚本环境中断→§9 记录后人工处置；c) settle/LW 门断言失败（实现缺陷面）→取证包上交不重跑。
- 止损线：单 run 后停（无论结果）；同代码版本同断点不烧第二轮。占用登记 coord/254-stackclaim 推 main 后 ≤60s 即时复核进程/端口再启动。

## 回执

- **实现完成（c687c9a1 @ port/af-d1-loudness-settle-1，已推远端；工件笔 9b6e443d）**：10 文件 +1196/−9——`loudnesswatch/action.go`（breach→有界推子建议纯函数：最越界载体|DeltaDB|最大+clip→track 映射（rows 内 track_id）+±maxAbsDeltaDB clamp+近似口径 Basis；pass/unmeasurable/无载体→nil 不虚构）+`trigger.go` `TriggerKindD1Settle`/`CheckWithConfig`（显式 config 检查与 `Check` 共核 `evaluateAndRecord` 零行为回退；realtime/invalid config fail-closed 不落盘）+chat `d1_loudness_settle_runtime.go`（settle 深面：watch config 未配置时 builtin:spotify 一次性判断**不写配置面**+verdict 落 store（dedupe 同规）+breach 建议挂既有 mix_tick 确认链（`storePendingMixTickCandidateForMode`→manual 挂卡/full access 按 improvement 先例直接执行同治理链；**确认前零 mutation**）+`WriteMixWorkflowQueueNodeState("D1", needs_confirmation/in_progress)` 点亮；completed 留账本推导=annotation 面红线）+`capability_owner.go` D 组显式检查意图词（全局响度检查/响度检查/峰值检查/母带前响度检查等，裸「响度」不入表=M1-4 纪律）+queue view D1 节点挂 `pre_master.loudness_settle.v0`+`d1d3_premaster_observation_runtime.go` settle 分支接线（5 行）+脚本 `-D1LoudnessSettle` 四门（settle 轮 stop_reason/观察 present/payload；verdict 落盘 d1_settle trigger；动作面诚实[breach→卡+取消腿/非 breach→无卡，verdict kind fixture-dependent 不硬断言=90 先例]；mix_queue_view D1≠not_started）+`-LlmModelOverride` 透传+UTF-8 BOM（PS5.1 中文字面量）。
- **单测全绿**：`go build ./...` exit 0；`go test ./internal/loudnesswatch ./internal/chat -count=1` 全 PASS（loudnesswatch 含新增 action 5+CheckWithConfig 3；chat 118.8s 全包含新增 settle 7；既有行为零回退）。**不触 automation_intent/semantic_entry**（卡面红线亲核：diff 无该二文件）。
- **gate4 缺陷处置结论＝证据关线**：wt_loudness_watch_b 的 `gate4_config_off_repro` 缺陷（D1 post-run 上下文 configure off 被持久快照重放回滚）已由 **133 FIX-LW-CONFIG-PERSIST-1 修复合入 main**（8b8bc70e latch 修复 + `loudnessWatchConfigExplicit` 在场亲证 capability_owner/server 代码；BATCH2 裁定 a1b4238c 记录 90+133 双 pass、真栈 run2 20261018_124639 gate4 FAIL→PASS）；本卡零 Go 修复，仅证据关线。
- **真栈腿＝armrun1 PS1_EXIT=1 取证上交**（run 20261105_211940，工件 [coord/runs/AF-D1-LOUDNESS-SETTLE-1/armrun1/](../../runs/AF-D1-LOUDNESS-SETTLE-1/armrun1/)，随 port 分支入库）：第三次调用起栈成功（前两次启动前调用形式失败——①Git Bash 反斜杠吞路径（AGENTS §5 陷阱）②worktree 缺 gitignored temp fixture 补 `-PublicManifest` 主检出件；均栈未起零 LLM 零断言不构成有效运行，如实记录）——被测 agent=F321AF5E（c687c9a1 构建 console 亲证）+kernel=wt_228 F9E3DA0A（同款复用亲证）；LW 预配置 ok（digest 0079aa999954）；**D1 runner（非本卡面）settle 链 600s 未排干 exit 非 0**：`D1 durable continuation did not drain within the smoke timeout`，continuation 停泊 waiting_continue/limit_reached(timeout)（agent 日志实证），栈健康推进 executed=5/receipts=18 非环境中断；**本卡 LW 门与 D1 settle 门未执行到**（脚本 LW 阶段前 throw）；§8 单 run 无重跑兑现。**归因候选披露**：本 run 注入 deepseek-flash，133 run2 同 fixture 同话术未注入（默认引擎）exit 0——settle 链轮次节奏与引擎选择的关联未排除，处置（重跑不注入/加 timeout/判模型分支）归决策侧。计费 51 调用 deepseek-flash（50 ok+1 client timeout）in 790,951/out 138,039 ≈¥1.1-1.3 上界（¥2-4 预算内，telemetry 切片亲数）。拆栈=脚本 throw 路径未自 teardown，owner 手动拆+**TEARDOWN_ALL_ZERO 亲核 21:40:37**；释放登记已推 main。
- **D1 点亮证据＝单测面**（真栈 queue view 断言因门未执行缺席）：`TestD1LoudnessSettleBreachParksPendingCandidateAndLightsNode` 断言 annotation needs_confirmation + `BuildMixWorkflowQueueView` D1≠not_started；真栈点亮证据待补证 run（上交处置后）。
- **补证腿＝armrun2 PS1_EXIT=1 取证上交（interim §三分叉执行，工件 [coord/runs/AF-D1-LOUDNESS-SETTLE-1/armrun2/](../../runs/AF-D1-LOUDNESS-SETTLE-1/armrun2/)，随 port 分支 c6fbb7d6 入库）**：同 armrun1 命令去掉 -LlmModelOverride（裁定字面），agent 同一二进制 F321AF5E 不重建+kernel wt_228 F9E3DA0A 显式；**形态≠排干超时**——settle 链 2 分钟完整收敛（goal completed/stop=done，executed=5 agent 日志实证），卡 D1 runner 断言 `exactly one forward mutation`（模型动作数量分支）；本卡 LW/D1 settle 门再次未执行到。**引擎变量排除闭合**：跑前亲读 config defaultModel=deepseek-flash（去注入与本机默认同值）+telemetry 10 调用全 flash 实证——两次 run 同引擎、两种失败形态（排干超时/mutation 计数断言）、均不在本卡断言面 → 133 时代 D1 runner 断言面×模型分支为系统性问题，**转卡建议=D1 runner settle 链取证卡**（排干窗口×轮次节奏×mutation 断言×模型分支四点复核）。计费 10 调用 0 err in 154,472/out 13,743 ≈¥0.2-0.3（两 run 合计 ≈¥1.3-1.6，预算内）。TEARDOWN_ALL_ZERO 亲核 22:01:13 当前空闲。**本腿单 run 无重跑兑现；卡留 doing 运行面挂起待主管处置（实现面已 interim pass，运行面转卡后本卡处置归终验）。**
- 端测边界声明：本卡改动=chat 确定性路由面+loudnesswatch 纯函数+脚本，不涉 webui 渲染面与 demo 旅程（§5 两层门槛不适用）；真栈腿按 §5 真实栈执行（栈起亲证+HTTP/ZMQ 断言路径），唯 runner 收敛失败致新门未达。

## 验收

- （主管回填）

## 主管中间裁定（2026-11-05 22:1x）

- 实现面 **pass**（diff 红线零触碰+单测亲复跑 7 例全 PASS+gate4 关线核可）——[2026-11-05-AF-D1-LOUDNESS-SETTLE-1-interim.md](../../rulings/2026-11-05-AF-D1-LOUDNESS-SETTLE-1-interim.md)；
- 真栈 exit 1 归因定案=引擎延迟×排干窗口（41 轮×9-20s 实测+第 41 轮 60s 超时+limit=timeout 停泊；非模型分支非实现缺陷）；
- **补证腿裁定：同 fixture 去注入（默认引擎）单 run**，成功条件=原卡面三件套；再超时→转 D1 runner 取证卡。跑前口径=裁定 §三。

## 终验（2026-11-05 22:2x）

- **实现面 pass 合入；运行面挂起转 256；卡以诚实状态收口**（GLM 主管决策流）——[2026-11-05-AF-D1-LOUDNESS-SETTLE-1-final.md](../../rulings/2026-11-05-AF-D1-LOUDNESS-SETTLE-1-final.md)：armrun2 亲核（settle 收敛+零 mutation 分支×断言错配）；归因修正第二笔（引擎隔离设计作废=默认引擎同值，教训入册）；实现随 port 分支合入 main；D1 真栈点亮证据由 256 验收腿补全。

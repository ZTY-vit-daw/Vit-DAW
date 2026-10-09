# L1-5-IMPL-C-REVIEW-1 独立复核报告

- 日期：2026-10-09（Asia/Shanghai）；执行 owner：Codex GPT-6 / PC ZTY / 会话 01a11f27-39ba-7313-9aa1-517fa3214c23。
- **建议结论：建议补证（主管确认平移形态偏差及 D 的适配边界后再裁定 C）；旧 message_loop 消费面的正常注册路径等价已证实，未发现需要在本卡修复的生产缺陷。** 本报告不构成 C 最终验收、不合入实现、不解锁 D。
- 关键结果：15/15 函数逐字等价，实际计数为 **12 平移 + 3 副本**；实际宿主注册顺序与旧链 10/10 一致；所有既有测试文件未改，但新增了 router_test.go，不能写“全仓测试零改动”。
- 真实栈边界：纯 Go 独立复核；没有启停 VitApp、Godot、agent，没有占用 PC-RUNTIME-STACK。双模式消费面、真实栈与用户旅程仍归 D/收口卡；本报告不新增烟测豁免。

## 1. 基线、领取与复验版本

| 项目 | 实际记录 |
|---|---|
| 原执行基线 | 4c2f75596e3571ab9f1ef3eccdaeb6fd25bbdbc8 |
| 被审实现 / 回执 | c488b811 / 89fee703e106e6dd2cc1da47dde64611ddababa6 |
| 初查 main | a25d3b9d71f38eb8ffbec7019e5f986ac9871b83 |
| 同步领取 | 首次 push 被 A 复核领取 2986df4f 抢先推进而拒；重读 C 仍 todo、A 报告域不相交；从新 origin/main 建协调分支，领取提交 635591c5 + 同步说明 05ed52e8，普通 fast-forward push 成功 |
| 复验 main 基线 | 05ed52e8（成功同步领取后的 main） |
| 独立 worktree | C:/Users/timoz/.codex/worktrees/l1-5-c-review/Vit_DAW |
| 分支 | port/l1-5-impl-c-review-1 |
| 应用提交 | c488b811 → fa4c2a41；89fee703 → 124a643d，无冲突 |
| 实际测试 HEAD | 124a643ddafaf3c81f31772b3c772b301ca27b23；领取前 status / diff stat 为空，应用后源码 status 为空 |
| 协调 checkout | D:/Vit_DAW_worktrees/l1-5-c-review-coord；主树的已有修改未带入 |

领取前 fetch 并查 origin/main 的 todo/doing/rulings：C 无最终 ruling 或新版本。实施卡本身在 done 仅是待审，不视为已验收。主树的 Settings.xml/default_project.xml/contextruntime/history_refs.go 与其他未跟踪工件未处理。协调提交仅含本复核卡状态，未把被审代码推 main。

从原执行基线到当前复验 main，另有 capabilitycontext/gain_staging.go 与 TIM 引用迁移及其测试变更；这是已共享 main 的集成背景，不属于 C diff。C 应用后的生产 diff 与原 C 生产 diff 完全相同，两个 `git diff ... -- agent | git hash-object --stdin` 均为 `e922d3db7b3331996db8bd6da616b4ec967a128b`。原始 diff 与集成 diff 均留档。

## 2. 实际 diff 与文件域

`git diff 4c2f7559 c488b811 --name-status`：一个旧源码修改 message_loop.go；八个新 fastpath 文件（七生产、一个测试）；原作者两个回执工件。message_loop.go numstat **+58 / -228**。89fee703 只改 receipt.md 的提交 hash。

亲读生产 diff；旧源码改动仅为 slices/fastpath import、newFastPathRouter 构造/守卫、loop 中链替换及每轮旁路刷新、12 个 helper 委托与说明注释。没有额外修改 preflight 状态执行体或其他存量函数。机械工具将该文件原有非 loop、非平移函数逐个比较，**446/446 原声明至结束字节相同**（保留函数体内空白及注释）；见 preserved-functions.json。

新增源码全部在原 C 卡允许域内。将三个域外 helper 复制到新包没有修改域外原件，但构成单源化偏差，见第 5 节。Router 本身含注册追加、复制 entries、返回名单副本、跳过 nil Handler 的新增 API；固定宿主注册项均非 nil，旧消费面不使用 Register，不因此改变实际链。

本卡自身写域仅为本报告目录及本复核卡状态；应用被审提交仅用于独立复验。报告提交不能连同其祖先的实现直接推 main。

## 3. 注册链与循环语义

对照从原提交 `loop()` 的真实调用表达式抽取，另一侧从被审提交 `newFastPathRouter()` 的真实 Handler 值抽取，**不是仅比较 DefaultEntryNames 与测试中的名单**。工件 registration-chain.json、old-loop.txt、new-loop.txt、new-router-constructor.txt 可重放。

| 顺序 | Entry 名 | 原调用 = 新 Handler |
|---|---|---|
| 1 | static_mix_capability_contract | preflightStaticMixCapabilityContract |
| 2 | project_blackboard_status | preflightProjectBlackboardStatus |
| 3 | clip_fade_gain_set | preflightClipFadeGainSet |
| 4 | clip_fade_gain_read | preflightClipFadeGainRead |
| 5 | strip_silence_suggest | preflightStripSilenceSuggest |
| 6 | clip_range_split | preflightClipRangeSplit |
| 7 | stems_folder_import | preflightStemsFolderImport |
| 8 | pending_section_markers_apply | preflightPendingSectionMarkersApply |
| 9 | natural_mix_observation | preflightNaturalMixObservation |
| 10 | static_mix_gain_staging_context_pack | preflightStaticMixGainStagingContextPack |

- 配置校验与 read-only mutation barrier 的原位置保持；新增 Router 在 barrier 后、for 前构造一次，绑定同一个 l 的方法值。
- 每轮先处理 pendingToolQueue；若 executePendingToolQueue 已 stopped，旧/新链都不调用。否则旧 gate 与新 SetDiagnosticOnly 的参数都在相同位置计算一次。
- 判定源仍是 free_state_reasoning.go:1678 `messageLoopFreeStateDiagnosticOnly`，只读 input.Context 及其 free_state_reasoning_loop；支持 diagnostic_only/free_state_diagnostic_only，两处上下文中任一为真均旁路。true 时零 handler，false 时按上述顺序每个至多一次。
- 未 stopped 的 handler 继续下一项；**false 不代表无副作用**：gain-staging context pack 可更新 conversation/trace 后返回 false；ClipFadeGainSet 的 disallowed 分支也会追加 final_gate 后 false。Router 传入同一 state 指针并保留这些副作用。
- 第一个 stopped=true 原样返回 Result，后续零调用；全 miss 忽略返回零 Result，继续原 `before_message_loop_model` checkpoint。未增加模型轮或重复 preflight。
- preflightFocusRelationship 保留在 natural mix 内部，未遗漏独立注册项。以上等价限定为固定正常注册与当前副本内容；新增漂移 panic 属另一行为边界，第 5 节单列。

## 4. 函数等价、计数与新增测试

工具 compare.go 使用 go/parser 定位声明，取原始源码的 `func` 到闭括号字节。只归一 CRLF→LF，且仅替换映射清单内的 **IDENT token**（不改字符串、注释或任意空白）；映射反向替换后连签名与 body 比较。每项保存 .old.txt/.mapped.txt、行号、字节数、双方 SHA-256 与宿主委托体到 function-equivalence.json。没有忽略逻辑节点，没有改写被审源码消除差异。

| 平移函数（旧名 → fastpath 名） | 结果 |
|---|---|
| messageLoopText → Text | IDENTICAL |
| messageLoopTextHasAny → TextHasAny | IDENTICAL |
| messageLoopClipFadeGainRequest → ClipFadeGainRequest | IDENTICAL |
| messageLoopClipFadeGainReadRequest → ClipFadeGainReadRequest | IDENTICAL |
| messageLoopExecutedClipRangeSplitCuts → ExecutedClipRangeSplitCuts | IDENTICAL |
| clipRangeSplitCutKey → ClipRangeSplitCutKey | IDENTICAL |
| messageLoopClipRangeSplitCompletionReply → ClipRangeSplitCompletionReply | IDENTICAL |
| messageLoopStripSilenceSuggestRequest → StripSilenceSuggestRequest | IDENTICAL |
| messageLoopA4ClipCleanupRequest → A4ClipCleanupRequest | IDENTICAL |
| messageLoopA4ClipCleanupWholeProjectRequest → A4ClipCleanupWholeProjectRequest | IDENTICAL |
| messageLoopNaturalMixRequest → NaturalMixRequest | IDENTICAL |
| messageLoopAudioObservationRequest → AudioObservationRequest | IDENTICAL |
| messageLoopGainStagingExplicitRequest → GainStagingExplicitRequest（副本） | IDENTICAL |
| messageLoopStaticBalanceIntentReference → StaticBalanceIntentReference（副本） | IDENTICAL |
| messageLoopGainStagingStrictReferenceIntent → GainStagingStrictReferenceIntent（副本） | IDENTICAL |

**实际是 12+3=15；“11 项 / 11 处委托”应修正。** 三副本来自 static_mix_gain_staging_context.go:1746/1770/1777。原作者提交只带 receipt.md 和 message_loop_diff.txt；申报的 /tmp/fpanalyze、原等价工具/输出以及闭包分析未入这两个被审提交，不能核实其原始运行。独立工具补足了 15/15 证据；**417 成员/8334 行及域外 83 定义未独立复现，不作为本报告已验证事实。**

gofmt：九个实际触碰的 Go 文件的 Git 提交 LF 内容全部 clean；本机 core.autocrlf=true，checkout 原字节均为 CRLF，而归一 LF 后全部 clean。gofmt.json 保留两个口径，未写回生产源码。

既有 *_test.go 无 M/D；新 fastpath/router_test.go 为 A，含六个 Test，日志六项 PASS。覆盖：名单字面完整性、泛型遍历/短路、miss 零值、diagnostic 旁路及恢复、Register 顺序、若干 helper 样例。局限：名单测试不调用宿主构造；sanity 不是穷尽，且“同一纯函数重复求值相等”断言不增强回归证明；nil Handler 旁路未测。当前注册名单由上述独立旧/新抽取核对补足，helper 当前等价由逐字工具补足。未要求为本卡增加生产测试。

## 5. 三项偏差请求的证据与建议

| 原回执请求 / 风险 | 可复查证据 | 建议（交主管裁定） |
|---|---|---|
| 平移形态偏离 / 全族逐字迁出前提 | runState 在 runner.go:350 为私有，所有十个 handler 仍接 `*Runner,*runState`；直接访问 state.input/trace/goal 及 r.complete/checkpoint/checkToolBudget/executeTool 等私有面。agentloop 已 import fastpath，反向 import 将成环；这些直接依赖足以证实逐字整族外移不成立，不需要接受未留档的闭包计数。 | 可接受 C 作为“旧宿主注册面归并+纯 helper 平移”的阶段交付；不得称独立全族库已完成。D 开卡前明确 host-owned adapter 和暂停/结果契约，或主管另派接口取证/补充卡。本复核不裁决新接口。 |
| 三副本的使用面与漂移 | fastpath/clip_fade_gain.go:35 ClipFadeGainRequest→GainStagingExplicitRequest→StaticBalanceIntentReference / GainStagingStrictReferenceIntent；宿主原 helper 在 static_mix_gain_staging_context.go:575/1727 等继续用于 gain-staging 分类。现版本双方 3/3 相等，未来只改原件会让“排除 clip 意图”和“gain-staging 意图”分歧。 | 当前无失配，可作为临时复制接受；建议后续独立卡做同源 helper 或明确双方一致性门。不能称现有测试永久覆盖漂移，不在本卡擅自修改域外原件。 |
| panic 完整性守卫 | message_loop.go:418-419 每次 loop 构造时 slices.Equal(router.Names(), DefaultEntryNames)；当前相等不触发。触发条件为增删/乱序/改名任一侧不一致，或其他包修改导出的可变 DefaultEntryNames。发生在处理 pending queue 和 diagnostic gate 之前，因此 diagnostic-only 也会受影响。 | 建议接受为程序配置不一致的显式失败，但必须把该例外从绝对“行为零变化”声明中分开。当前没有外部写 DefaultEntryNames；未发现可由普通用户输入触发的路径。后续可考虑不可变名单/宿主测试防漂移；不要求本卡把 panic 改 error，因为这会新增 loop 错误路径。 |

没有证据把该 panic 归为普通请求错误：它未经过 r.fail/Result 错误处理，若调用层未 recover，会按 Go panic 传播；不承诺进程或请求级恢复。检查相关 agentloop/chat 生产源码未见保护此调用的局部 recover，但本卡未以真实进程触发，避免把静态风险写成已发生崩溃。

## 6. D 接线前的真实依赖和限制

本节只读参照 A 被审版本 de1d876c 的 pullharness/loop.go（存 A-pull-loop-reference.txt），未将 A 应用到本卡测试树；这是接口对照，不是 A+C 集成验收。

| 接口/状态 | C 实际需要 | D 必须处理的限制 |
|---|---|---|
| Router | Router[*Runner,*runState,agentloop.Result]，TryMatch(ctx,r,state) 返回 stopped/result/name；newFastPathRouter 私有 | 不实现 A 的 `Route(ctx, GoalInput) (FastPathOutcome,bool)`。A 的结果只有 Reply/Outcome/ProbeCost。建议 adapter 由 agentloop 持有私有 state，避免把私有结构镜像到 pullharness。 |
| 状态读写 | 用户文本、AllowedTools、Context、Conversation、trace、goal、executed、pendingToolCall/Queue、contextSnapshot、executionMemory；helper 还读近期观察、history、预算等 | 不能每轮从原始 GoalInput 新建空 state；需同轮副作用回写和跨轮累积，否则 pack 重建、拆分重试与确认队列会失真。完整直接访问清单见 retained-handler-dependencies.json；这是直接依赖清单，不冒充完整传递闭包。 |
| Runner 执行接口 | complete / pause / checkpoint / checkToolBudget / executeTool / now，以及 l.logTiming、stems 确认与 gain-staging 分支 | fastpath handler 已执行工具并改 runtime；若 adapter 再交 Tools.Execute 会重复执行。预算/权限/取消/确认必须继续由同一 authority 保持。 |
| stopped 的含义 | 完成、clarification、confirmation、budget/timeout/interjection 暂停、失败或取消均可能 true | A 的 Route 命中一律 finish→T2；不能把确认/暂停简单投影为 OutcomeFastPath 并触发终局 retain。需要主管明确分类和 continuation 接线；仅 Reply 不足以承载当前 Result。 |
| miss 的含义 | 已观察或注入 capability context pack 后仍可 false | A 每次传同一 GoalInput，历史用局部 history 管理；adapter 的 conversation/trace 变更必须让下一个模型轮和下一次 router 看见，不能只丢掉 false 的结果。 |
| diagnostic-only | 每次尝试前刷新，来自 root Context 或 nested free_state_reasoning_loop 的两个字段 | A 现有接口没有显式旁路开关；建议由 adapter 根据相同判定源传入。不可只读一个根字段或只在 run 开头采一次。 |
| 退场、计量 | Runner complete/result 已含 runtime 完成、EndTurn、终局 observation retain 和 continuation 构造 | A finish 也执行 T2；需避免重复终局/漏暂停，以及工具调用与 ProbeCost/shortcuts 漏计。不能用静态名称映射代替这些生命周期。 |

类型参数 `[RUN any, STATE any, RES any]` 没有能力接口约束，仅使路由器不 import 宿主；它没有自动解决 state、Result、权限或执行面的解耦。最小适配选择为 host-owned adapter；接口化或镜像方案需主管另定域与契约。上述争议属于 D 前置裁定，禁止执行侧凭这份报告自行扩大文件域。

## 7. 复跑命令、失败分类与工件

工作目录为独立 worktree/agent；Go 版本留在 go-version.txt。测试时 HEAD=124a643d，生产无未提交 diff；新增复核工件均在 agent 之外，不影响 `go test ./...` 的包集。

| 命令 | 首次退出码 | 原始工件 |
|---|---|---|
| go build ./... | 0 | build.log / commands.json |
| go test ./internal/fastpath -count=1 -v | 0；6 Test PASS | fastpath.log / commands.json |
| go test ./internal/agentloop -count=1 | 0 | agentloop.log / commands.json |
| go test ./... -count=1 | **1**；90 包 ok、chat 包清理失败 | full.log / commands.json |
| go test ./internal/chat -run '^TestAutomaticProposalTextConfirmationKeepsTaskIdentityAndPendingProjection$' -count=1 -v | 0 | chat-isolated.log / diagnostic-commands.json |
| go test ./internal/chat -count=1 | 0 | chat-package.log / diagnostic-commands.json |
| go test ./... -count=1（诊断后确认复跑） | **0；91 包 ok** | full-confirmation.log / full-confirmation-command.json |

首次全仓失败为测试 cleanup：TempDir RemoveAll 的 `.vit_history/.sessions/project-same-task/.../workspace/state` “The directory is not empty”。没有该测试的业务断言失败，也没有 crash。测试位于 improvement_proposal_workflow_test.go:226，使用 t.TempDir；C 未改该测试、workspace/history 持久化层，其确认路径不依赖本轮新增 Router。静态 helper 等价、agentloop 全包及隔离/整包复跑均通过，支持这是非稳定 cleanup 故障；**仍未定位并发写入者，不记成已经修复或正式 known-flaky 豁免，不归因模型能力。** 保留首次失败而非覆盖为绿。

完整全仓确认复跑实际 **exit 0，91 个测试包 ok**，开始 13:42:32、结束 13:44:24 +08:00；见 full-confirmation-command.json / full-confirmation.log。该确认仅因首次 cleanup 故障及隔离/整包诊断而启动一次；没有原样无限重跑。卡面四个命令均已取得 exit 0 证据，但不抹去首次全仓 exit 1，也不据此声称稳定性。

机械工具首次依赖清单生成因最后一个 handler 定义在域外 static_mix_gain_staging_context.go 而报空指针；这是复核工具的定位遗漏，补显式文件选择后成功 exit 0，未修改被审实现。compare.log 为最终结果，原失败位置 compare.go 原第 90 行记录在此，不能当作生产测试失败。

## 8. 问题分级和交接

- **D 接线 blocker（非当前 C 缺陷）：** A/C 接口不兼容；stopped 的暂停语义、false 的状态更新、权限/工具执行与终局 retain 没有明确适配契约。需主管裁定后派相应范围；不能在本卡接线。
- **应修回执/补证：** 原 C 的 11 平移/11 委托应为 12；“全仓测试零改动”应为“既有测试不改、新增六测试”；417/8334/83 数字没有入库工具，若用于设计裁决须提交可重放证据，或降级为未验证申报。
- **应裁定：** 接受宿主执行体保留的 C 阶段边界、临时副本、运行时 panic 例外；本报告分别建议见第 5 节。不能只凭单测绿把原全族迁移目标写成完成。
- **建议：** 后续同源 helper、不可变注册契约/宿主注册测试；cleanup 故障若再出现应另开修复卡，不永久豁免。
- **验收边界：** 复核卡可以凭上述有效诊断和对照证据完成并转 done 待主管验收；本卡结束不等于 C 已最终通过或已合入。

报告与机械工件在 port/l1-5-impl-c-review-1 提交；卡状态单独从最新 origin/main 同步。主管可只查看/提取报告提交，禁止把该报告分支祖先中的应用实现未经验收推 main。

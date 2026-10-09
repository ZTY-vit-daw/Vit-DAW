# L1-5-IMPL-C 回执：FastPathRouter——preflight 族平移归并（行为零变化）

- 卡：coord/cards/doing/2026-10-09-L1-5-IMPL-C.md（池序 32）
- 执行：PC 执行侧（worktree D:/Vit_DAW_wt_l5c，分支 port/l1-5-impl-c）
- 领取基线：2026-10-09 11:12 / origin/main=4c2f75596e3571ab9f1ef3eccdaeb6fd25bbdbc8（=领取时 HEAD，worktree status 干净）
- 本卡 commit：（见下"提交"节，提交后回填）

## 1. 交付面

**新包 `agent/internal/fastpath/`（8 文件）**

| 文件 | 内容 |
|---|---|
| router.go | 泛型 `Router[RUN, STATE, RES]` 统一注册面：`NewRouter/Register/Names` + `TryMatch`（注册序短路，返回 stopped/result/entry 名）+ `SetDiagnosticOnly/DiagnosticOnly`（CCB 实验边界旁路的显式开关）+ `DefaultEntryNames`（规范注册清单=完整性契约）。泛型参数由 agentloop 以 `*Runner / *runState / Result` 实例化——fastpath 包零宿主类型引用，无循环 import |
| text.go | Text / TextHasAny（平移） |
| clip_fade_gain.go | ClipFadeGainRequest / ClipFadeGainReadRequest（平移） |
| clip_range_split.go | ExecutedClipRangeSplitCuts / ClipRangeSplitCutKey / ClipRangeSplitCompletionReply（平移） |
| strip_silence.go | StripSilenceSuggestRequest / A4ClipCleanupRequest / A4ClipCleanupWholeProjectRequest（平移） |
| natural_mix.go | NaturalMixRequest / AudioObservationRequest（平移） |
| gain_staging_ref.go | GainStagingExplicitRequest / StaticBalanceIntentReference / GainStagingStrictReferenceIntent（**逐字副本**，源=static_mix_gain_staging_context.go，原件原地保留） |
| router_test.go | 新增测试（见 §4） |

**`agent/internal/agentloop/message_loop.go`（唯一旧文件改动，+58/−228）**

1. loop() 10 项 preflight 链（原 ：415-449）→ `fastPaths.SetDiagnosticOnly(messageLoopFreeStateDiagnosticOnly(state))` + `fastPaths.TryMatch(ctx, r, state)` 单点调用；原英文注释三行逐字保留，旁路判定源不变（单一事实源），仅开关形态平移为 Router 显式旁路。
2. 新增 `newFastPathRouter()`：单一注册点，10 个 Entry 按原链调用序注册（Handler=方法值直连，零闭包包装零装箱）；注册后以 `slices.Equal(names, fastpath.DefaultEntryNames)` 做完整性守卫，漂移即 panic（fail-visible）。
3. loop() 开头构造无状态 Router（`fastPaths := l.newFastPathRouter()`），每轮刷新旁路开关——频次与原 gate 判定一致（原 if 每轮求值一次，现 SetDiagnosticOnly 参数每轮求值一次；`messageLoopFreeStateDiagnosticOnly` 为只读谓词，已亲核 free_state_reasoning.go:1678）。
4. 11 个平移函数原位替换为**同名一行委托**（`return fastpath.X(...)`），全部既有调用点（含 6+ 个测试文件的直调、域外文件的引用）零改动。

## 2. 注册清单（恰 10 项，序=原链调用序）

static_mix_capability_contract / project_blackboard_status / clip_fade_gain_set / clip_fade_gain_read / strip_silence_suggest / clip_range_split / stems_folder_import / pending_section_markers_apply / natural_mix_observation / static_mix_gain_staging_context_pack

第 11 个 preflight（`preflightFocusRelationship`）不在卡面清单，未注册——它是 preflightNaturalMixObservation 的内部依赖，随宿主原地保留，语义不变。

## 3. 平移范围与"cut-paste 级"裁定的偏离声明（诚实边界）

**卡面预期**："匹配器+确定性产出函数体语义零变化平移（cut-paste 级）"入新包，message_loop.go 仅调用点替换与 import。

**实际证据推翻了全族平移前提**（go/ast 闭包分析，脚本存 /tmp/fpanalyze）：
1. **Go 类型墙**：匹配器/产出函数普遍以 `state *runState`（runner.go:350，未导出类型）为参，且依赖 `r.complete/r.checkpoint/r.executeTool` 等未导出方法。fastpath→agentloop 方向 import 被循环禁令封死，任何"函数体逐字平移"对这些函数都不可编译；镜像 state+桥接方案需数百处 body 改写，违背"语义零变化可证明"。
2. **闭包规模**：10 项 preflight 的传递闭包=417 成员/~8334 行（含 free_state/MOM/退出保留等域外子系统），全族平移不成立。
3. **文件域墙**：纯函数中 83 个定义在域外文件（verification.go/helpers.go/static_mix_gain_staging_context.go 等，卡面禁改），只能逐字副本不能迁移。

**实际执行（最小可证明等价的平移形态）**：
- **平移 11 项**（message_loop.go 定义、纯签名、自洽闭包）：删原体，fastpath 逐字承接，agentloop 留同名一行委托。**逐字节等价验证**：fastpath 函数体经反向改名（`fastpath.X→messageLoopX`）+LF 归一后与 `git show HEAD` 原体**15/15 全等（IDENTICAL，含 3 副本）**——验证脚本输出存本卡工件。
- **副本 3 项**（域外定义、被平移函数依赖）：逐字复制进 gain_staging_ref.go，原件不动，agentloop 继续消费原件；副本仅供本包平移函数依赖，非死代码。
- **状态面耦合的匹配/产出函数**（如 messageLoopDeterministicClipFadeGainSetCall、messageLoopStaticMixCapabilityContractReply 等）**留原处**——这是类型墙的直接结论，归 IMPL-D/pull harness 接线轮再议适配面。
- message_loop.go diff 构成：import 两行（slices、fastpath）+ loop() 链替换 + newFastPathRouter 注册面 + 11 处委托替换。**全文 hunk 存 `coord/runs/L1-5-IMPL-C/message_loop_diff.txt`（384 行）**。

## 4. 新增测试（fastpath/router_test.go，6 个）

1. TestDefaultEntryNamesMatchesTranslatedPreflightChain——注册清单完整性：恰 10 项、无遗漏无多余、顺序=原链调用序
2. TestTryMatchWalksInRegistrationOrderAndShortCircuits——注册序遍历+命中短路（后续项零调用）
3. TestTryMatchMissReturnsZero——未命中返回 (false, 零值, "")
4. TestDiagnosticOnlyBypassesAllEntries——旁路开关：开启时零 Handler 调用+未命中；关闭恢复命中
5. TestRegisterAppendsInOrder——Register 保序
6. TestMovedMatcherSanity——平移面金测（Text/TextHasAny/ClipRangeSplitCutKey/ClipFadeGainReadRequest/NaturalMixRequest/StripSilenceSuggestRequest/GainStagingExplicitRequest 排除分支）

另：`newFastPathRouter` 内置注册面完整性守卫（slices.Equal 对 DefaultEntryNames，漂移 panic）——运行期 fail-visible，覆盖 fastpath 测试无法触及的宿主注册侧。

## 5. 验收门结果

| 门 | 结果 |
|---|---|
| 既有 preflight 相关测试不改一字 | ✅ `git diff --name-only HEAD -- "*_test.go"` = 空（全仓库测试文件零改动） |
| 既有测试全绿（等价性主证） | ✅ go test ./internal/agentloop/ -count=1 ok（含 message_loop_test.go 8023 行 preflight 行为面 + materialize_red_test.go 对 preflightStaticMixGainStagingContextPack 的直调面） |
| go build ./... | ✅ exit 0 |
| go test ./... -count=1 全量 | ✅ **91 包 ok / 0 FAIL**（原 90 包+新 fastpath 包） |
| gofmt | ✅ 全部 CLEAN（LF 归一口径；工作树 CRLF=autocrlf 检出伪影，沿 §6.5 先例口径） |
| 行为零变化声明 | ✅ 见 §3 等价验证；diff 构成核对=§1.4 |

## 6. 烟测豁免边界复述（决策侧预裁定的复述）

本卡为行为零变化平移，单元级验收充分（既有测试等价证明+逐字节对比双保险）；**真实运行栈验收豁免，归 IMPL-D 收口轮双模入口接线时统一验**。本卡未触碰真实栈、未启动任何内核/前端/agent 进程。

## 7. 提交

- port 分支：port/l1-5-impl-c（实现+测试+回执），commit hash=c488b811（实现主体）+本回执回填提交
- 卡片流转：本回执提交后 mv doing→done（main 侧 coord 提交）

## 8. 移交决策侧的三点裁定请求

1. 平移形态偏离（§3）是否接受为 C 卡达标形态，或要求另开补充卡（如"共享纯 helper 归 shared 内部包"消除副本漂移面）。
2. 状态面耦合的匹配/产出函数适配面（接口化 vs 镜像 vs 留宿主）归 IMPL-D 设定时裁定——本卡证据（类型墙+闭包分析）已备齐可复查。
3. `newFastPathRouter` 的 panic 守卫属运行期 fail-visible，若决策侧偏好 error 返回路径请明示改法（会引入 loop() 错误分支=行为面新增，本卡未做）。

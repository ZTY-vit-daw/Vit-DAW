# L1-4-IMPL-D-REVIEW-1 复核报告（独立复核腿，只读评审+复跑门）

- 复核人：PC 执行侧（flash 独立会话，不采信被审回执自述，全部结论独立重验）
- 日期：2026-10-08 晚窗
- 被审对象：main 四 commit `2bc8843c`（D1）/`16d3293c`（D2）/`4cd09238`（D3）/`b5fd1375`（D4）
- 复核基点：`2d471a5c`（含全部被审 commit；独立 worktree `D:/Vit_DAW_wt_impld_review`，分支 `impld-review-1`，领取时主树 HEAD=`477e04cd`）
- 规格面：`coord/runs/L1-4-IMPL-D/receipt.md` + `docs/CONTEXT_LAYERING_V1_DESIGN.md` §9 IMPL-D 行 + §6.4 收口线（line 477）
- 纪律遵守：零生产代码写入（唯一写入=本工件目录+复核 worktree 测试态）；未动主树任何未提交改动（含主树已暂存的 `exit_retain_test.go` 排序修正——见问题 R-1，未代为提交）

## 总判定：**pass-with-notes**

四 commit 与回执申报的三类生产改动（挂点消费切换/装载层序/权威翻转）逐符，红线零违反，复跑门全绿，真栈烟测在独立泊位复现 exit 0。1 条应修（gofmt 修复滞留未提交，纯测试文件格式）、4 条建议级（申报计数口径、测试缺口记录）。无 blocker。

---

## 面 1：diff 亲读（四 commit 全量，`git show` 逐 hunk）

| commit | 实触文件 | 结论 |
|---|---|---|
| D1 `2bc8843c` | 8 | 挂点签名 `RunTurnBoundaryHook` 二元组→`TurnBoundaryHookResult`（+ExtraUnits/+ProjectDir 输入，+ExitReport/+RetainsWritten 输出）；chat 侧 `HistoryLimit=12` 保持、ProjectDir=`projectPathFromChatContext`；agentloop 侧 `HistoryLimit=0` 保持、ProjectDir=`runProjectDirFromState`（键序 project_path>current_project_path）；写失败显式进 ExitViolations（不静默）；`runner.go` 终态漏斗接 `retainRunObservationConclusions`（trace 事件，失败不阻断 Result）；遥测键 `exit_retains_written` 加法式 |
| D2 `16d3293c` | 4 | 生产 chat/中性族装配切 embed 渲染（`chatSystemFromRuleset`/`neutralFamilySkeletonFromRuleset`，装载失败回落 legacy）；**两处 legacy 模板与翻转前生产常量逐字节一致已机械验证**（按行号区间提取+diff，非目测）；parity 测试翻向=canary（生产面=embed 全量渲染）+回落锁定（legacy=embed 减 PostMigration） |
| D3 `4cd09238` | 9 | 双入口切 `carriers.Assemble` 层序 Section（family=chat/neutral_family）；缺层 absent fail-open；`len(sections)==0` 回落单段（L1 corrupt，system 不缺席）；`AssemblyInput` +CarrierLayerStates/+CarrierWarnings（Build 零消费）→报告并入+遥测键 `carrier_warnings`；`DefaultCarrierWorkspaceDir` 沿 journal workspace-root 约定；chat struct 加注入位（伴随 gofmt 对齐） |
| D4 `b5fd1375` | 1 | `dev_agent_smoke.ps1` 场景注册+入 journeyBerth 泊位族（-StartKernel 强制）+遥测断言块（七键逐一非空+breaks 原始行 Contains+prefix_bytes 单值+chat 面 retains 恒 0） |

- **生产路径改动限于申报三类**：`chat/server.go` 全部 hunk=挂点调用（D1）+system 装配（D2/D3）+struct 注入位（D3）；`message_loop.go` 全部 hunk=挂点调用（D1）+中性族装配（D3）+struct 注入位（D3）。越域=0，未申报行为变更=0。
- **hook 签名变更涟漪核验**：生产调用方恰 3 处（`chat/server.go:5759`/`agentloop/message_loop.go:4084`/`agentloop/exit_retain.go:78`），全部随 D1 更新，无漏改调用点。
- **range 勘误（复核过程记录）**：D1-D4 全距基点=`2bc8843c^`=`eaa2efa9`；`abefe658`（FS-CAP 卡补记）夹在 D1 与 D2 之间，**不属被审面**，其对账范围内的唯一 diff（一张 todo 卡删除）已排除。
- ⚠ **实触 18 文件 vs 回执申报 16**：`agentloop/runner.go`（D1，+6 行）与 `chat/prefix_assembly_test.go`（D3，首层期望 chat_system→ctx.layer.rules）未列入回执"文件数申报"清单。两者工作内容分别在 D1 腿描述（"Runner 终态漏斗"）与 D3 后果中有申报、均在申报文件域内、无未申报行为变更——属清单遗漏非隐瞒（→问题 R-2）。

## 面 2：红线对账抽查（六条全查，非抽样）

| 红线 | 验法 | 结论 |
|---|---|---|
| 五处退场机制零改动 | chat 截尾 12=`server.go:5783 conversationHistory(...,12,...)` hunk 级未触碰；RecentTurns=8=`context.go:329`（文件不在被审 range）；观察账本窗口 24=`model_projection.go:482`（同）；冷引用=`model_projection.go` cold_data/cold_ref 面（同）；ExpiresAfterContextChange=`exit_executor.go:75`（同） | ✅ 全零改动 |
| queryengine+agentprotocol 零 diff | `git diff --stat eaa2efa9 b5fd1375 -- <两包>` | ✅ 空 |
| 账本只追加 | `WriteRetains`（`exit_executor.go:363`）逐决策仅调 `carriers.AppendLedgerEntry`；后者 `os.O_APPEND|O_CREATE|O_WRONLY`+PrevHash 链+Kind 七值封闭枚举 fail-closed；carriers 目录全距零 diff | ✅ 字面 append-only |
| ruleset 资源零字节改动 | `git diff --stat eaa2efa9 b5fd1375 -- agent/rules` | ✅ 空（整个目录，含七段+第八段） |
| 22 键 allow-list 不扩 | allow-list 边界在 `carriers/ledger.go:10` 注释锚定（不经 execution_memory 继承链）；ledger.go 全距零 diff | ✅ 不扩 |
| G1-G8 门零改动 | 被审 range 文件清单不含自由态 admission 面 | ✅ 零触碰 |

## 面 3：门复跑（独立 worktree @ 2d471a5c，全量）

| 门 | 结果 |
|---|---|
| `go build ./...` | **exit 0** |
| `go test ./... -count=1` | **exit 0；90 包 ok、0 FAIL、12 包无测试**（与回执"90 包 0 FAIL"一致；原始输出=`gate_full_build_test.log`） |
| parity 定向（卡面口径 `'Parity|TestChatSystem|TestNeutralFamily'`） | **exit 0**：chat 翻转 canary+回落锁定、中性族翻转 canary+回落锁定+四层 Section+P1 跨轮全 PASS |
| gofmt 触碰面 | checkout 层全红=autocrlf=true 的 CRLF 伪象（与回执预告一致）；**blob 级（提交真实态）16/17 净，`exit_retain_test.go` 红（import 排序）**（→问题 R-1） |

## 面 4：测试充分性审读（8 个触碰测试文件断言面全读；缺口只记录）

卡面验收点覆盖核对：

| 卡面验收点 | 覆盖测试 | 判定 |
|---|---|---|
| retain 落盘 | TestRunTurnBoundaryHookConsumesRetains（决策→账本条目+evidence refs+零违规）；TestRetainRunObservationConclusionsPersistsLedger（确定性键序+无结论行跳过） | ✅ |
| advisory 形态 | TestRunTurnBoundaryHookAdvisoryWithoutProjectDir（断言账本文件**不存在**+执法面不缺席）；TestRetainRunObservationConclusionsAdvisoryWithoutProjectDir | ✅ |
| 写失败可见 | TestRunTurnBoundaryHookRetainWriteFailureVisible（blocker 文件→"retain write failed"进 ExitViolations） | ✅ |
| 去重幂等 | TestRetainRunObservationConclusionsDedup（二轮 note 空+条目数不变） | ✅ |
| 缺层字节兼容 | TestChatAssemblyAbsentLayersKeepPrefixByteStable（头尾锚+无注入）；TestNeutralFamilyAssemblyAbsentLayersFallbackShape；两 parity 主证明（全链字节） | ✅ |
| 层序 | TestChatAssemblyCarriesFourLayerSections（五锚递增序+报告 rendered 行）；TestNeutralFamilyAssemblyCarriesFourLayerSections（rules 首/catalog 尾+ledger 渲染） | ✅ |

缺口（**只记录不补**）：

- G-1 `dedupeAgainstProjectLedger` 读失败支（账本 corrupt→全量保留、宁重复不丢）无测试。
- G-2 `DefaultCarrierWorkspaceDir`（含 `detectWorkspaceRoot` 探测与 getwd 失败回落空串）零测试覆盖。
- G-3 入口级 L1 corrupt 回落（`len(sections)==0`→单段）无直接测试（carriers 包级 corrupt 已有既有覆盖，入口级编译期 embed 理论不可达——低风险）。
- G-4 `runner.result` 终态漏斗接线（仅终态触发/暂停面不触发/trace 事件附加）无直接测试——现有测试只打 `retainRunObservationConclusions` 本体。

计数勘误：卡面"9 个新测试文件"与实际不符——**实际新增测试文件 5 + 修改 3（两 parity+prefix_assembly）=触碰 8**；回执"新增 7 文件"口径=新增文件总数（含 exit_retain.go/carrier_dirs.go 两个非测试文件），口径应写明（→R-3）。

## 面 5：端测边界核验（回执六条声明逐条对代码实态）

| 回执声明 | 代码实态 | 判定 |
|---|---|---|
| 1 渲染面零触碰 | `git diff eaa2efa9 b5fd1375 -- agent/webui`=空；Godot 在仓库外；D4 场景显式拒启 UI | ✅ 属实 |
| 2 retain 真栈样本待旅程轮 | 被审 run 111759 工件核验：2 条 section_stats 记录 `exit_retains_written=0`（chat 面确定性预期）；retain 落盘由单测覆盖（面 4）；声明与实态一致 | ✅ 如实 |
| 3 通用路径 L1 界外 | `messageLoopSystemPrompt`（`message_loop.go:4548`）非自由态分支=原始内联模板，零 carriers/ruleset 依赖；自由态分支走中性族（申报覆盖面内） | ✅ 属实 |
| 4 genesis 未接线 | `AppendGenesis` 调用方仅 `carriers_test.go`，生产调用方=0 | ✅ 属实 |
| 5 OQ-3 维持开放 | `casEligibleKinds` v1 仍仅 tool_result（`exit_executor.go:151`，被审 range 零触碰）；声明"真栈样本不足以复核裁定"与实态一致 | ✅ 如实 |
| 6 OQ-5 未触发 | `promptruntime.Build` 将全部 SystemSections 渲染进**单条** system 消息（`prompt.go:66-68`） | ✅ 属实 |

真栈证据链核验：被审 run `20261008_111759` 工件在案（`context_layering_telemetry_snapshot.jsonl`：2 记录、`prefix_bytes=49136`×2、retains/violations/warnings 全 0）——与回执申报逐项一致。

## 面 6：可选腿——真栈泊位复跑（已执行）

| 轮 | 命令 | 结果 | 分类 |
|---|---|---|---|
| 1 | `dev_agent_smoke.ps1 -Scenario context_layering -StartKernel`（worktree 泊位） | exit 1：`project_uuid` 永不稳定→泊位身份等待超时 | **环境中断**：worktree 无 `Export/staging/runtime/VitApp.exe`（内核产物不进 git）；与回执自身 run 111242 同型失败（脚本要求泊位族必有内核），不归因被审代码 |
| 2 | 同命令（主树已构建内核 exe 复制进 worktree staging 后） | **exit 0 PASS**（run `20261008_182612`，head=2d471a5c，泊位自起自拆） | ✅ 可复现性独立验证成立 |

复跑 run 遥测核验（工件已归档 `berth_rerun_20261008_182612/`）：2 条记录、`prefix_bytes=49136`×2——**与被审 run 111759 跨 run 逐字节相等**（L1 ruleset 渲染字节稳定性在四个独立装配轮上成立）、`exit_retains_written=0`/`exit_violations=0`/`carrier_warnings=0`。

---

## 问题清单

**blocker：无。**

**应修（1 条）**

- **R-1 `exit_retain_test.go` 提交态 gofmt 不净（import 排序：`agentruntime` 应列于 `contextruntime/carriers` 后）。** 回执门表"gofmt -l 零输出"仅在作者工作树成立；修复已存在于主树**已暂存未提交**的改动（`git diff --cached` 可见，一行），但被审四 commit 的 blob 态不过 gofmt。修复动作=决策侧授权后提交该暂存修正（或并入下一张涉 agentloop 的卡）；复核侧依纪律未代为提交。

**建议（3 条）**

- R-2 回执"文件数申报"16 vs 实触 18：`agentloop/runner.go`、`chat/prefix_assembly_test.go` 漏列（工作内容已申报、无越域、无未申报行为变更）；后续回执文件清单建议以 `git diff --name-status <base>..HEAD` 机械生成。
- R-3 测试文件计数口径混乱：卡面"9 个新测试文件"实际=新增 5+修改 3；回执"新增 7"=新增文件总数（含 2 非测试文件）。建议统一口径（新增/修改/触碰三分）。
- R-4 面 4 缺口 G-1~G-4（去重读失败支/DefaultCarrierWorkspaceDir/入口级 corrupt 回落/终态漏斗直测）——建议随下一张涉 agentloop/contextruntime 的卡顺带补齐，不单开卡。

## 工件索引（本目录）

- `gate_full_build_test.log` — 复跑门全量 build+test 原始输出（BUILD_EXIT=0/TEST_EXIT=0，90 ok）
- `d3_diff.txt` / `d4_diff.txt` — D3/D4 全量 diff 存档（D1/D2 见 git show）
- `berth_rerun_20261008_182612/` — 面 6 复跑 run 关键工件（遥测快照/两轮回复/summary/head）
- `berth_rerun_smoke_stdout.log` — 面 6 第 2 轮完整 stdout（exit 0）
- `berth_run1_envfail_stdout.log` — 面 6 第 1 轮环境中断原始输出（exit 1）

## 交接备注

- 复核 worktree `D:/Vit_DAW_wt_impld_review`（分支 `impld-review-1` @ 2d471a5c）保留待决策侧抽查后清理（内含面 6 完整 run 目录与复制的内核 exe；工作树除 run 工件与 Export 复制产物外零改动）。
- 主树未提交改动（`VitApp/Workspace/*.xml`、`contextruntime/history_refs.go`、已暂存的 `exit_retain_test.go`）复核全程未触碰。
- 端口纪律：泊位两轮均自检端口空闲（7878/5555），自起自拆，结束态端口已释放。

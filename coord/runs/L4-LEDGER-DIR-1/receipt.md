# L4-LEDGER-DIR-1 回执：账本宿主目录解析统一——.vit 文件路径裸喂修正 + REVIEW-1 R-4 余缺口（G-1/G-3/G-4）补齐

- 执行侧：PC 执行会话（GLM-5.3 flash 派发档）；执行方式：独立 worktree `D:/Vit_DAW_wt_ledger`，分支 `port/l4-ledger-dir-1`（基线 origin/main@a7b5fe74），实现不落主工作树。
- 本回执随 port 分支提交（实现+测试+烟测脚本+run 工件+本回执同一 commit；hash 由 done 卡回执行与分支头记录）。

## 1. 领取基线

- 领取时间：2026-10-09 09:50；领取时主仓 HEAD=`a7b5fe747a4c91a2a036029e3258390f5fbd0fec`（=origin/main，fetch 后无新提交）
- 领取时主仓 `git status --short`（既有权威输入，本卡零触碰）：`M VitApp/Workspace/Settings/Settings.xml`、`M VitApp/Workspace/default_project.xml`、`M agent/internal/contextruntime/history_refs.go`（LF/CRLF 工作树噪声在案）+ coord/runs 未跟踪工件若干
- 卡片领取提交：main@`935b15a4`（coord/cards todo→doing mv + 领取行回填，仅 coord/ 变更，已推 main）

## 2. 锚点实核（领取时回查）

- 卡面三处裸喂全部在案，行号微漂（server.go 卡面 5802→实为 5820 一带，函数内相对位置一致）：
  1. `agentloop/message_loop.go:4147` carriers.Assemble ProjectDir=runProjectDirFromState 裸路径 ✅
  2. `chat/server.go:5820` carriers.Assemble ProjectDir=projectPathFromChatContext 裸路径 ✅
  3. `agentloop/exit_retain.go:69` projectDir=runProjectDirFromState 裸路径（流经 dedupeAgainstProjectLedger 读 + RunTurnBoundaryHook 写两路）✅
- 先例：`harness/project_genesis.go:77` genesisLedgerDir（L4-GENESIS-1 已在 main），语义=已存在目录→原样；否则→文件父目录。
- 消费语义实证：`carriers.ledgerPath=Join(projectDir,"ledger/project_ledger.v1.jsonl")`；.vit 文件路径直喂⇒读侧 os.Open 恒 NotExist（L4 永 absent）、写侧 MkdirAll 落文件之下恒败。

## 3. 实现 diff 摘要（三处点名 + 两同族面 + 消重复）

共享 helper：`carriers.ResolveProjectDir(projectPath) string`（carriers/ledger.go 新增，紧邻 ledgerPath）——""→""（无工程面不落 cwd 相对面，guard 是新增语义、与全部调用方的空值守卫兼容）；已存在目录→原样；否则→filepath.Dir。读侧探测纯函数无副作用。归属采卡面首选 carriers（三调用方包均已 import；账本 `<projectDir>/ledger/` 约定归 carriers 所有）——**停止条件（归属争议）未触发**。

| 调用点 | 变更 |
|---|---|
| `agentloop/message_loop.go:4147`（卡面点名 ①） | `ProjectDir: carriers.ResolveProjectDir(runProjectDirFromState(state))` + 两行注记 |
| `chat/server.go:5820`（卡面点名 ②） | `ProjectDir: carriers.ResolveProjectDir(projectPath)` + 两行注记 |
| `agentloop/exit_retain.go:69`（卡面点名 ③） | `projectDir := carriers.ResolveProjectDir(runProjectDirFromState(state))`（dedupe 读与 retain 写两路同源解析） |
| `agentloop/message_loop.go:4088`（同族，非卡面点名） | RunTurnBoundaryHook ProjectDir 同族解析——retain 写入面（WriteRetains），与 ③ 同缺陷族；当前该面无结论单元供给（dormant），切换属预防性、行为零变化 |
| `chat/server.go:5778`（同族，非卡面点名） | 同上（chat 轮边界 hook 面） |
| `harness/project_genesis.go`（挂点侧消重复） | genesisLedgerDir 函数删除，调用点改 `carriers.ResolveProjectDir(projectPath)`，删除随之失用的 `os` import；语义逐字节等价（原函数体即现 helper 逻辑） |

行为零变化论证：既有测试面全部工作于目录路径（t.TempDir()）——helper 对已存在目录原样返回，全部调用点字节不变；"已是目录"语义与 GENESIS 先例逐字一致。

## 4. G-3 回落分支提炼（测试可达性最小重构）

`len(sections)==0` 回落原先内联在两入口（embed 装载失败理论不可达，无法经注入触发）。提炼为包内纯函数（行为逐字节不变，调用点一行切换）：
- `agentloop.neutralFamilySystemSections(bundle, state)`（message_loop.go）
- `chat.chatSystemSections(bundle, modeInstruction, catalog)`（server.go）

## 5. 新增测试（REVIEW-1 R-4：G-1/G-3/G-4 + helper + .vit 形态读面）

| 测试 | 覆盖 |
|---|---|
| `TestResolveProjectDirVitFileFallsToParent` / `...DirectoryPassthrough` / `...MissingPathFallsToParent` / `...EmptyStaysEmpty`（carriers/ledger_dir_test.go 新文件） | helper 四分支（.vit 文件→父目录/目录→原样/缺失→父目录/空→空） |
| `TestDedupeAgainstProjectLedgerCorruptReadKeepsAll`（exit_retain_test.go） | **G-1**：账本 corrupt（首行合法且语句可匹配+尾行 not-json）→ ReadLedger 整本 fail-closed → 去重全量保留（含可读前缀也不参与去重），序与内容保持 |
| `TestNeutralFamilySystemSectionsCorruptFallback`（carrier_neutral_test.go） | **G-3**（agentloop 入口）：空 bundle→单段骨架（ID=message_loop_neutral_family_selection、Stable、内容=骨架逐字节）；段在位→原样透传 |
| `TestChatSystemSectionsCorruptFallback`（chat/carrier_assembly_test.go） | **G-3**（chat 入口）：空 bundle→D2 单段（ID=chat_system、内容=chatSystemFromRuleset 逐字节）；透传 |
| `TestRunnerResultFunnelRetainsOnlyOnTerminalStates`（exit_retain_test.go） | **G-4**：终态（completed）经 `r.result` 漏斗→账本 2 条入账+trace 恰 1 条 exit_retain 事件（含 retain 计数）；暂停面（`r.pause`→waiting_continue）→账本零落盘、零 exit_retain 事件 |
| `TestNeutralFamilyAssemblyRendersLedgerForVitFileProjectPath`（agentloop）+ `TestChatAssemblyRendersLedgerForVitFileProjectPath`（chat） | 真栈路径形态（project_path=.vit 文件）下两装配读面 L4 rendered（单测级锁定） |

## 6. 全量门

- `cd agent && go build ./...`：**exit 0**
- `cd agent && go test ./... -count=1`：**exit 0，90 包全 ok，FAIL 零行**（复跑两轮一致）
- gofmt：本卡 9 个 .go 文件 `gofmt -l` 报警经 LF 化逐文件甄别——**8 文件纯 CRLF 工作树噪声**（autocrlf 同源，与 L4-GENESIS-1/REFSCHEMA-M2 记录同因）；**1 文件真实差异**（message_loop.go 组合字面量对齐）已手修并复验净。净。

## 7. 烟测脚本扩展（复用 dev_agent_smoke.ps1 体系）

`-Scenario l4_genesis` 由三腿扩为四腿（L4-LEDGER-DIR-1 段）：**LEG 4 装配读面 A/B**——真栈形态（project_path=.vit）+ 账本在场（LEG 2 播种）vs 离场（Move-Item 移出后恢复）各驱动一轮真 LLM chat（`disable_agent_loop=true` 路由到 chat 直连装配面=卡面点名 ② 的 carriers.Assemble 消费面），断言面=遥测 section_stats.prefix_bytes 差值 ≥64（账本段字节），回复文本零断言（§8 纪律）。另：l4_genesis 场景注册进遥测 env 自动注入名单（`VIT_AGENT_LLM_TELEMETRY_PATH` 泊位继承）。summary 增 leg4 三字段。

## 8. 真栈 run 工件（隔离泊位，自起自拆）

| run | 结果 | 说明 |
|---|---|---|
| `coord/runs/L4-LEDGER-DIR-1/20261009_102500` | **FAIL（LEG 4）** | 首轮验证腿设计缺陷：plain chat 轮路由进 goalrunner message_loop 非中性族装配（该面本就不含 carriers，prefix 恒 49136/delta 0）——属验证面取点错误非实现缺陷，LEG 1-3 全过；如实留档 |
| `coord/runs/L4-LEDGER-DIR-1/20261009_102924` | **PASS，SCRIPT-LASTEXITCODE=0** | 四腿全过：LEG 1 fail-open（WARN 可见+零落盘）/ LEG 2 genesis（2 条 topology_delta/genesis，tom_overview+delivery_targets）/ LEG 3 幂等（字节级不变）/ **LEG 4 prefix_bytes present=43031 absent=42324 delta=707（账本段真实渲染=真栈 .vit 形态下 L4 rendered/absent 双面实证，非"永 absent"）** |

- 命令：`powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt_ledger/scripts/dev_agent_smoke.ps1 -RepoRoot D:/Vit_DAW_wt_ledger -Scenario l4_genesis -StartKernel -KernelExe D:/Vit_DAW/Export/staging/runtime/VitApp.exe -RunArtifactsDir D:/Vit_DAW_wt_ledger/coord/runs/L4-LEDGER-DIR-1/<ts>` → PASS 轮退出码 0（stdout 尾行 SCRIPT_LASTEXITCODE=0）。
- 被测 agent=worktree 现构建（脚本内 go build，含本卡全部改动——LEG 4 delta 707 本身即修复生效的实证：修复前该形态恒 absent/delta 0）；内核=`D:/Vit_DAW/Export/staging/runtime/VitApp.exe`（2026-10-06 staged 导出，内核侧本卡零改动，如实记录非本卡构建）。
- 工件清单（PASS 轮）：genesis_{failopen,opened,reopened}_response.json、l4_genesis_summary.json（leg4 三字段）、leg4_{ledger_present,ledger_absent}_reply.json、leg4_telemetry_snapshot.jsonl、agent_llm_telemetry.jsonl、head.txt/git_status.txt（轮时 worktree 基线=a7b5fe74+本卡未提交 diff）、kernel_workspace/、project/、fixture_stems/、agent_drafts/。

## 9. 边界声明（如实）

1. **retain 落盘真栈样本不可得**：本轮旅程面无自由态结论轮（LEG 4 轮无观察账本），retain 写入真栈落盘样本未采集——retain 面由单测覆盖（TestRetainRunObservationConclusions* + G-4 漏斗直测 TestRunnerResultFunnelRetainsOnlyOnTerminalStates），卡面明示"不可得→边界如实声明不算失败"。
2. **真栈 L4 读面取点=chat 直连装配（点名 ②）**：中性族 message_loop 读面（点名 ①）真栈触发需自由态旅程（本卡未驱动），以单测 `TestNeutralFamilyAssemblyRendersLedgerForVitFileProjectPath` 锁定；两读面共用同一 helper 同一语义。
3. **4088/5778 两 hook 面为同族预防性切换**（非卡面点名三处之内）：dormant 面（当前无结论单元供给），行为零变化，申报决策侧知悉。
4. 单轮 exit 0 只证明存在一条成功路径（AGENTS §8）；稳定性比例未规定未声称。

## 10. 提交

- port 分支：`port/l4-ledger-dir-1`（D:/Vit_DAW_wt_ledger），本卡实现+测试+脚本+coord/runs/L4-LEDGER-DIR-1 两轮工件+本回执单 commit；hash 见 done 卡回执行。
- main 侧：卡片 doing→done mv + 回执行回填（仅 coord/ 变更，执行侧允许的直推路径）。

# REFSCHEMA-M5 执行回执：E 类回执族归一 + R5 去留落账

- 卡：`coord/cards/doing/2026-10-10-REFSCHEMA-M5.md`（池序 41；G1 终审 §4 M5 行 + M8 报告 R5 节）
- 执行 owner：GLM-5.3-Flash 执行会话 PC（ZCode flash 会话）
- 领取：2026-10-10T20:10+08:00 / 基线 origin/main `9c8cec8f`（依赖 M4B=`21b70b54` cherry-pick 已核在 main 史，refschema.go 基线 22 词条）/ 领取提交 `f763d37c`（一次推成，远端卡面 owner 已核对）
- 实现分支：`port/refschema-m5` @ **`9d80a573`**（已推远，待决策验收合入；**未直推 main**）
- 实现改动：3 文件，+167/−1 —— `agent/internal/agentprotocol/refschema.go`（+5 词条 + 注释块，22→27）、`refschema_test.go`（初值表 22→27 + Fatalf 消息）、`refschema_m5_test.go` 新增（4 个钉住测试）。**chat/capabilityadapters 生产代码零改动**（消费链核实结论=纯回执面零解析消费，M4 先例同判）。越域零。

## 1. 消费链核实清单（卡面目标 1 交付物）

**生成点全景**（领取基线 `9c8cec8f` 实测，行号回查一致）：

| 字面量族 | 生成点 | 载体 | 处置 |
|---|---|---|---|
| `b4.semantic_eq_batch:<key>` | `chat/b4_eq_runtime.go:618`（共享端口 SourcePrefix=b4） | **EvidenceRefs**（applyEQ 回执） | 注册（主词条） |
| `b4.semantic_eq_batch.reconcile:<key>` | `chat/b4_eq_runtime.go:525`（B4/C1 共享端口 Reconcile） | **EvidenceRefs**（reconcile 回执） | 注册（reconcile 词条） |
| `c1.semantic_eq_batch:<key>` | `chat/b4_eq_runtime.go:618`（c1BatchSpec SourcePrefix=c1，`c1_frequency_cleanup_runtime.go:31`） | **EvidenceRefs** | 注册（主词条） |
| `c1.semantic_eq_batch.reconcile:<key>` | `chat/b4_eq_runtime.go:525`（同上） | **EvidenceRefs** | 注册（reconcile 词条） |
| `c2.dynamic_plugin_load_batch:<key>` | `chat/c2_dynamic_batch.go:686` | **EvidenceRefs**（apply 回执） | 注册 |
| `b4/c1.plugin_load_batch{,.reconcile}:<key>` | `chat/b4_eq_runtime.go:678/:564` | EvidenceRefs（装载批回执，兄弟族） | **不注册**——不在 G1 M5 行点名两族内；opaque fail-visible 钉住（`TestRefSchemaM5LoadBatchSiblingsStayUnregistered`），随域触碰再注册 |
| `b4/c1/c2.*.governed` 命令常量 | `b4_eq_runtime.go:26-27`、`c1_frequency_cleanup_runtime.go:28`、`c2_dynamic_batch.go:26` | `Action.Command`/编排命令名，**非 ref** | 不注册；测试钉住不被回执词条劫持为 legacy |

**下游清点**（M8 附录 grep 口径复跑，cwd=agent，含 `internal/ cmd/ webui/src`）：

- 字符串解析消费方：**非测试零命中**。两族字面量除生成点外全库无按前缀解析/拼装的读取者。
- 测试面命中 1 处：`chat/b4_eq_planner_test.go:363`（`strings.HasPrefix(receipt.EvidenceRefs[0], "c1.semantic_eq_batch:")`）——前缀断言，注册零影响（测试全绿实证）。
- 通用流转面（非字面量）：`orchestration/types.go:657/:672`、`executionruntime/coordinator.go:356`、`orchestration/proposal_interaction.go:124` 均为 `EvidenceRefs` **纯拷贝透传**（append copy），无前缀语义。
- 判定：两族**进 EvidenceRefs 面**（非 R5 型纯响应面）但零解析消费 → 按 M2/M4 型边界归一=注册翻译条目即完成，生成点不迁移（M4 同判先例：capabilitycontext 批次生产代码零改动）。

## 2. 承载核实（卡面"含批序号与时间成分→slot=snapshot"预裁定）

- remainder=IdempotencyKey：`executionruntime/coordinator.go:115`（`"exec:"+session.ID+":"+actionSet.Hash`）+ `:130/:259`（Apply/Reconcile 传 `key+":"+action.ID`）。sessionID=`plan_`+随机量（`goalrunner_chat.go:2140` 等）→ **session 实例身份在前**，actionSetHash 虽内容衍生但整体不可复现 → 非内容指纹。
- 裁定：slot=snapshot、TargetKind 留空、family=evidence_scheme_uri（对齐 M4 批次/M4B 同款）。**无证据矛盾**，停止条件未触发。
- 前缀含尾冒号的设计事实：`b4.semantic_eq_batch:` 与 `b4.semantic_eq_batch.reconcile:` 因 `:`/`.` 之差**无包含关系**（注册表零新增包含对，动态扫描测试自动覆盖）；`.governed` 命令常量不匹配任何词条。

## 3. R5 去留（卡面范围裁定执行）

- M8 附录 A7 grep 口径复跑（`mix-report:|mixboard-project:` over `internal/ cmd/ webui/src`）：仍仅三写点（`harness/harness.go:4548`/`:4549`、`chat/mixboard_decision_projection.go:100`）+ 1 测试断言（`harness/mix_report_test.go:46`），**零下游读取者**，与 M8 实证一致。
- 按主管预裁定=**登记不动作**：不注册翻译条目（无解析消费方，注册徒增词条噪音）；不碰 harness.go:4548-4549 与 mixboard_decision_projection.go:100（卡面文件域禁区，零触碰）。已落 G1 终审记录 R5 注记。
- 领取时点未出现新读取者，停止条件未触发。

## 4. 目标三件对照

| # | 卡面目标 | 落地 |
|---|---|---|
| 1 | 消费链核实+清单入回执 | 本回执 §1/§2（生成点全景表+下游清点+通用流转面），M8 口径复跑一致 |
| 2 | 进面归一 / 纯响应面注册 | 两族进 EvidenceRefs 面零解析消费 → 五词条注册（`b4/c1.semantic_eq_batch{,.reconcile}:` + `c2.dynamic_plugin_load_batch:`，22→27），identity 族 slot=snapshot 承载；生成点零迁移 |
| 3 | G1 终审记录回写 | M5 行批注（批注含余量族登记）+ M5 表行触发格更新 + **R5 去留裁定注记**（登记不动作依据=零消费方） |

## 5. 验收命令与结果（cwd=worktree `D:/Vit_DAW_wt_refm5/agent`）

| 命令 | 结果 |
|---|---|
| `go build ./...` | exit 0 |
| `go test ./internal/agentprotocol ./internal/capabilityadapters ./internal/chat -count=1` | 三包 ok（chat 91.9s），exit 0 |
| `go test ./... -count=1`（最终代码态复跑） | **92 包 ok、0 FAIL、go test 原生 exit 0**（日志 `/tmp/m5_full_suite.log`：`GO_TEST_EXIT=0`、`OK_PACKAGES=92`、`FAIL_LINES=0`） |
| 显式跑：`go test ./internal/agentprotocol -run 'TestRefSchemaM5\|TestRefSchemaM4\|TestRegistryInitialValues' -v` | M5 四测试 + M4 三测试 + 初值表全 PASS |
| blob 级 gofmt（M4B 口径） | 三触碰文件 HEAD blob **CRbytes=0** 且 **gofmt diff=0 字节**（`git cat-file -p HEAD:<file> \| tr -dc '\r' \| wc -c` + `\| gofmt -d \| wc -c` 双口径；工作树 CRLF=autocrlf 检出效应） |

- 测试时 HEAD=`9d80a573`（port/refschema-m5）；工作树=本卡 3 文件改动，领取时 `9c8cec8f` 干净检出、零叠加 diff。
- **端测边界声明**：本卡为纯单测域（注册表数据+解析器钉住测试），卡面明示"不需要真栈；无资源占用"，未触碰真实运行栈；无渲染面/用户旅程改动，无 webui 改动。

## 6. 语义红线

- vit:// 文法零改动（ParseRef/FormatRef/转义/三态逻辑零触碰，diff 仅注册表数据+测试）。
- 既有 22 词条零改动（初值表测试逐条目逐字段对比在案）。
- legacy 往返回归（翻译表+往返恒等+幂等重解析测试全绿）。
- b4/c1/c2 生成点行为零变化（chat/capabilityadapters 生产代码零改动；三包既有测试全绿背书）。
- E 族由 opaque WARN-once 透传转为 legacy 可翻译（信息面变化=本卡目标），行为面（编排/执行/重拉）不变。

## 7. 移交与停止

- 执行侧自验完成 → 卡移 `done/` 待决策验收（验收负责人=GLM 主管决策流）。验收动作：审本回执 + diff（`port/refschema-m5@9d80a573`）+ 消费链清单（§1）。
- 停止条件两条均未触发：生成点跨域零（两族 EvidenceRefs 位点全在 b4_eq_runtime.go/c2_dynamic_batch.go 卡内域）；R5 零新读取者。
- 无阻塞、无越域、无真栈资源占用。G1 终审记录、回执与本卡状态变更经 coord/ 面 push main；实现 commit 仅在 port/refschema-m5 分支待验收。

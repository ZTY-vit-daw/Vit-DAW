# REFSCHEMA-M4 执行回执：B 类身份族 capabilitycontext 批次承载定裁+翻译条目注册

- 卡：`coord/cards/doing/2026-10-10-REFSCHEMA-M4.md`（池序 40，G1 终审 §4 M4 行 + M8 报告 R6）
- 执行 owner：GLM-5.3-Flash 执行会话 PC（ZCode flash 会话）
- 领取：2026-10-10 10:36 / 基线 origin/main `c0f8eb94` / 领取提交 `f744af45`（推 main 后远端卡面 owner 已核对）
- 实现分支：`port/refschema-m4` @ **`cf080a9a`**（已推远，待决策验收合入；**未直推 main**）
- 实现改动：4 文件，+230/−1 —— `agent/internal/agentprotocol/refschema.go`（+4 词条+注释块）、`refschema_test.go`（注册表初值表扩 15→19）、`refschema_m4_test.go`（新增）、`agent/internal/capabilitycontext/refschema_m4_anchor_test.go`（新增）。**capabilitycontext 生产代码零改动**（消费链核实结论=纯响应面，见下）。文件域 ≤5 ✓，越域零。

## 1. 承载预裁定核实结果（执行侧实读，2026-10-10，基线 c0f8eb94）

| 族/前缀 | 生成点（实读行号，与卡面锚点一致） | remainder 实态 | 预裁定 | 结论 |
|---|---|---|---|---|
| `ccbobs_` | free_state_observation.go:499 | `compactID(observationID, requestID)`=**首个非空输入的清洗透传**（非 [A-Za-z0-9_-]→`_`、截 80）；observationID=`obs_<UTC时间戳>_<random>`（mixboard/mixboard.go:1286） | identity 含时序 → slot=snapshot | **成立 ✓ 已注册** |
| `ccbobs_rejected_` / `ccbobs_batch_` | :420 / :1584 | 同上构造（同族变体，卡面未列名、消费链清点发现） | 随主条目 | **成立 ✓ 已注册**（独立词条保子族区分） |
| `cap_pack_` | pack.go:112 `stablePackID` | `sha256(capabilityID∥intent∥refs∥generatedAt-RFC3339)[:16hex]`——**种子含生成时间戳**→同内容异时刻异 id，实例身份非可复现内容指纹 | identity → slot=snapshot | **成立 ✓ 已注册** |
| `ccbr_` / `ccbr_rejected_` / `ccbr_batch_` | :567 / :439 / :1636 | 与 ccbobs_ 同构造=obs_ 身份透传。**实含时间戳+随机量**（webui trace fixture 实录：`ccbr_obs_20260911T115419_a2ef6022376c`）；非哈希、非内容寻址、非"观察 id+请求 id"组合（compactID 只取首个非空，requestID 在场时被忽略） | "compactID=观察 id+请求 id 哈希短串，内容寻址"→ ObservationFingerprint+slot=hash | **矛盾 ✗ 停止条件触发，该族上交缓注册** |

**停止条件处置（按卡面"该族单独上交（升 G1 终审记录缺口）"条款）**：ccbr_ 三前缀不注册，维持 opaque WARN-once 透传（现状零行为变化）；已升 G1 终审记录缺口（本回执 §4 回写）。上交点：ccbr_ 族承载语义需主管重裁——实读证据下其形态与 ccbobs_ 族**同构**（同为 obs_ 身份透传），执行侧注：若重裁，证据支持对齐 ccbobs_ 语义（identity→slot=snapshot）；该建议仅陈述证据指向，裁定权在主管。缓注册状态有 fail-visible 测试钉住（`TestRefSchemaM4CCBRFamilyStaysUnregistered`：出现 ccbr 注册词条即 FAIL，强制先重裁后注册）。

## 2. 消费链清单（目标 1；M8 附录 grep 口径复跑+全库清点）

**生成点全量（7 处/2 文件，全在 capabilitycontext）**：

| # | 前缀 | 锚点 | 承载字段 |
|---|---|---|---|
| 1 | `ccbobs_rejected_` | free_state_observation.go:420 | `FreeStateObservationBundle.BundleID` |
| 2 | `ccbr_rejected_` | :439 | `AuditReceipt.ReceiptID` |
| 3 | `ccbobs_` | :499 | `BundleID` |
| 4 | `ccbr_` | :567 | `AuditReceipt.ReceiptID` |
| 5 | `ccbobs_batch_` | :1584 | 合批 `BundleID` |
| 6 | `ccbr_batch_` | :1636 | 合批 `ReceiptID` |
| 7 | `cap_pack_` | pack.go:112 | `Pack.PackID`（gain_staging/static_balance/pan_layout/low_end_relation/frequency_cleanup 五构造点复用） |

**下游与解析面**：

- 非测试 Go 代码对四前缀字面量的引用**仅上述 7 生成点**（`git grep` internal/+cmd/ 全库清点）；`.BundleID`/`.AuditReceipt.ReceiptID` 在 capabilitycontext 之外**零消费方**（仅 harness/ccb_observation.go:156 调 `AssembleFreeStateObservation` 取整个 bundle 结构）。
- **不进 EvidenceRefs 解析面**：bundle 的 `EvidenceRefs []string`（:518）与 pack 的 `EvidenceRefs []string` 均装载上游 MOM/mixboard refs；`PackID`/`BundleID`/`ReceiptID` 是独立结构体字段，从不并入 refs 数组（有生成侧测试钉住：身份字段泄漏进 EvidenceRefs 即 FAIL）。
- **不被 ParseRef 消费**：注册前该族字面量若流经 ParseRef 走 opaque WARN-once 透传；注册后 ccbobs_ 三形态+cap_pack_ 变 legacy 可翻译（信息面），行为面（重拉/查询）不变。
- 展示面：webui `src/trace/__fixtures__/` 3 份录制 JSON 含 ccbr_/ccbobs_ 实录样本（chat 事件展示链），无解析逻辑。
- **定性（卡面目标 3 两形态分记）**：**纯响应面（无解析消费）→ 注册即完成**；无 M2 型边界归一位点。

## 3. 验收命令与结果（AGENTS §9 口径）

| 命令（cwd=worktree `D:/Vit_DAW_wt_refschema_m4/agent`） | 结果 |
|---|---|
| `go build ./...` | exit 0 |
| `go test ./internal/agentprotocol ./internal/capabilitycontext -count=1` | 两包 ok，exit 0 |
| `go test ./... -count=1` | **92 包 ok、0 FAIL、exit 0**（12 包 no-test-files 通知不计） |
| 触碰 4 文件 blob 级 gofmt（LF 归一后 `gofmt -l`） | 全净（工作树 CRLF 为 autocrlf=true 的 checkout 效应，blob 均洁） |

- 测试时 HEAD=`cf080a9a`（port/refschema-m4）；工作树改动=本卡 4 文件，领取时工作树为 origin/main c0f8eb94 干净检出（无叠加 diff）。
- 新词条解析测试：翻译五元组表+分解往返恒等（prefix+value==raw、二次解析 DeepEqual）+**最长匹配**（注册表前缀包含对动态扫描——ccbobs_ 主条目先于变体登记，命中取最长、与注册序无关）+ccbr_ 族 opaque 钉住；既有 15 词条回归=既有翻译表/初值表/三态/转义往返全绿。
- **端测边界声明**：本卡为纯单测域（注册表数据+解析器+生成侧 id 构造），卡面明示"不需要真栈；无资源占用"，未触碰真实运行栈；无渲染面/用户旅程改动。

## 4. G1 终审记录回写（目标 4）

`coord/runs/L1-1-REFSCHEMA-1/G1_FINAL_REVIEW.md` §4 M4 行已批注：capabilitycontext 批次落地（ccbobs_ 三形态+cap_pack_）+ ccbr_ 族停止条件上交待重裁 + 余量清单（obs_/rel_/c2_plan_ 维持"随域触碰"）。与实现合入（决策侧 cherry-pick port/refschema-m4）互不阻塞：批注以"待决策验收"口径登记。

## 5. 移交与停止

- 执行侧自验完成 → 卡移 `done/` 待决策验收（验收负责人=GLM 主管决策流）。验收动作：审本回执+diff（port/refschema-m4@cf080a9a）+ ccbr_ 族重裁（G1 记录缺口）。
- 无阻塞、无越域、无真栈资源占用。

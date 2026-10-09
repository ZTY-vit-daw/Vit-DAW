# L1-5-IMPL-C-REVIEW-1：FastPathRouter 等价性与三项偏差独立复核

- 发卡：Codex 辅助决策流 / 本次共享卡池与 harness 审查会话 / 2026-10-09（Asia/Shanghai）
- 派发确认：已确认发卡（用户本轮授权本会话作为决策流发卡；范围为已完成 harness 交付的独立复核，零生产实现）。执行仍由用户在独立执行流指定“执行 L1-5-IMPL-C-REVIEW-1”后领取。
- 验收负责人：现有主管决策流；Codex 发卡流可核查报告，C 的偏差裁决与实现合入仍交主管。
- 目标仓库：D:\Vit_DAW；Mac 可使用其同仓 checkout（纯 Go 复核，无真实栈）。
- 优先级 / 预估 / 依赖：P1 / 45-60 分钟 / 无；可独立于 A 复核。建议与 A-REVIEW-1 并行，不修改原池序。
- 文件域：只写 `coord/runs/L1-5-IMPL-C-REVIEW-1/REPORT.md` 与同目录日志/机械对照工件、以及本卡状态字段。源码、原 C 卡、docs、rulings 全部只读；复验用本流独立 worktree。
- 并行与资源：与 A-REVIEW-1 报告域不同，各自 worktree；不需要真实栈，不启停 VitApp/Godot/agent，不占用 PC-RUNTIME-STACK。

## 已核实依据与待证问题

- 发卡基线：main `2691f9279487e2c4f5018c8d0b143f6de0d38d8b`；主树已有改动不得搬入本卡复验。
- 被审提交：`c488b811`（实现）+ `89fee703`（回执回填）；执行基线 `4c2f75596e3571ab9f1ef3eccdaeb6fd25bbdbc8`。原卡 `coord/cards/done/2026-10-09-L1-5-IMPL-C.md`，报告位于被审分支 `coord/runs/L1-5-IMPL-C/receipt.md`。
- 设计依据：docs/HARNESS_V1_DESIGN.md §5/§9；原卡要求注册面归并、匹配语义与 diagnostic-only 旁路不变。
- 已核实回执申报：fastpath 新包 8 文件，message_loop.go 为唯一存量源码改动；10 项注册，11 项纯 helper 平移、3 项域外 helper 副本、状态耦合匹配器留宿主；新测试 6 个。
- 待证：15/15 逐字节等价申报能否复现；注册的实际顺序与旧调用链是否一致；留宿主形态对 D 的接口有什么限制；panic 守卫与副本漂移是否需要主管处置。

## 执行步骤

1. 读取共享协议并同步远端，检查 C 是否已有最终 ruling/修订；若状态改变先上交，避免复审旧版本。按协议领取后建立独立 review worktree，建议分支 `port/l1-5-impl-c-review-1`。
2. 从领取时确认的 main 基线应用 `c488b811`、`89fee703` 做集成复验。不复用作者 worktree；应用冲突只记录，不改实现。
3. 亲读 `git diff 4c2f7559 c488b811 --stat` 与生产 diff，核实允许文件域。比较 `git diff 4c2f7559 c488b811 -- agent/internal/agentloop`，列出 import、链替换、Router 构造与 helper 委托之外的任何逻辑变化。
4. 独立核对旧调用链与新 `newFastPathRouter`：10 项名册/顺序、每轮旁路开关、未命中续行、命中短路以及同轮调用次数。不能只检查 DefaultEntryNames 与测试自身定义一致。
5. 对已平移与复制的函数做原提交内容对照。允许 LF 归一和申报的标识符对应替换，禁止忽略任意空白/逻辑节点；保存映射清单、机械对照结果及无法对照项。回执“15/15”需解释实际计数，差异不得靠重写源码消除。
6. 逐条核验回执 §8 三个请求：全族平移前提是否确因未导出宿主类型/循环依赖不成立；3 副本的使用面和漂移风险；panic 守卫的触发条件及影响。417 成员/8334 行计数若工具未留档则注明未独立复现，不把它当作已验证事实。
7. 给出 D 接线前的真实依赖清单：留宿主 matcher 所需 state/runner 接口、Router 的泛型宿主约束、diagnostic-only 语义。只建议适配选择，不裁决新接口或扩大本卡域。

## 验收命令

在 review worktree 的 `agent/` 下运行：

```powershell
go build ./...
go test ./internal/fastpath -count=1 -v
go test ./internal/agentloop -count=1
go test ./... -count=1
```

- 四命令期望 exit 0，原始输出与直接退出码留档。另核对实际触碰 Go 文件的 gofmt 输出，区分提交内容偏差与本机行尾转换，不写回。
- 用 `git diff 4c2f7559 c488b811 --name-status` 核查既有 *_test.go 是否被改；新 fastpath/router_test.go 是新增文件，必须列入覆盖审查，不能用“全仓测试零改动”省略它。
- REPORT.md 必须含提交/复验 HEAD/实际 diff、注册链对照表、函数等价工件、三请求证据表、复跑退出码、问题分级、D 接线限制和建议结论。
- 结论为“建议通过 / 建议补证 / 建议返工 / 阻塞”。每条设计偏差分别给建议，主管决定是否接受；本卡完成不等于 C 最终验收。

## 停止条件与交付

- 行为不等价、卡外逻辑改动或源码缺陷导致复跑失败：保留最小反例和日志，交付建议返工报告，停止修复；有效失败证据可以完成复核卡。
- 接口争议、原卡前提被推翻或新版本替代：附锚点上交主管，不在本卡中重新设计 Router 或修改 D。
- 不稳定/环境失败按 AGENTS §11 分类留档，不把一次重跑通过作为豁免依据。
- 报告与机械工件走本卡分支，提交回执后按共享协议同步卡状态；不合入被审代码、不出最终 ruling。
- 端测边界：纯复核、不修改生产；行为零变化是否成立需本卡实证，真实栈与双模式消费面仍由收口卡验证。

- 领取：2026-10-09 13:39（Asia/Shanghai） / origin/main=a25d3b9d71f38eb8ffbec7019e5f986ac9871b83 / owner=Codex GPT-6，PC ZTY，本会话 01a11f27-39ba-7313-9aa1-517fa3214c23（用户明确“执行 L1-5-IMPL-C-REVIEW-1”） / port/l1-5-impl-c-review-1 / worktree=C:/Users/timoz/.codex/worktrees/l1-5-c-review/Vit_DAW / 领取提交=本条状态提交（提交后以远端 log 核对）；独立 worktree 领取前 status/diff stat 均为空；主树已有改动不带入复验。
- 领取同步补记：上述时间字段为预填；实际领取提交时间 2026-10-09 13:35:47 +08:00。首次 push 因 A-REVIEW-1 领取提交 2986df4f 被拒；已重读远端，C 仍 todo、A 仅写独立报告域。按协议从 origin/main=2986df4ff91037814e035f3ab72fa2a257aeccee 建新协调分支并 cherry-pick 本卡领取记录；复验从成功同步后的 main 建立。
- 回执：2026-10-09 / port/l1-5-impl-c-review-1，报告提交=576c8ca8d26cf8d8cbbc04da9cd24aa8e9c7d4a9（已推 origin） / coord/runs/L1-5-IMPL-C-REVIEW-1/REPORT.md 与同目录机械对照、原始日志 / 15/15 逐字等价（实际 12 平移+3 副本），注册链10/10，旧文件另446函数保持 / build、fastpath、agentloop 均0；首次全量1（chat临时目录cleanup），隔离0、chat整包0、全量确认0（91包）；全部首次证据保留，不作known-flaky豁免 / 建议补证：旧消费面正常注册等价成立，主管需确认宿主留置/副本/panic偏差与D适配边界；417/8334/83未独立复现 / 端测边界=纯Go复核，零真实栈，未合入C实现、未出最终ruling。本卡done仅复核完成待主管验收。
- 验收：（主管裁定 / 本复核卡验收 commit；与 C 原卡最终裁定分开）

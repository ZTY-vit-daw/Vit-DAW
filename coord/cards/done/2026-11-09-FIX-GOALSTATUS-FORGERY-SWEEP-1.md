# FIX-GOALSTATUS-FORGERY-SWEEP-1：goal_status 伪造面全树普查+同型修（327/330 缺陷族收尾）

- 发卡：GLM 主管决策流 / 2026-11-09（依据=327+330 两卡域外登记累积：interaction 面 catch-all[327 已修]/plan 面[330 已修]/chat 投影空回填[330 已修]/interaction 面三个 workflow 专属恢复兜底 server.go:4834/4885/4956 仍伪造——同族第四批）
- 派发确认：已确认（主管发卡；P3 不占展演前关键路径，随手清型）
- 验收负责人：GLM 主管决策流
- 池序 334；P3 / 0.5-1 天 / 依赖=327/330 已合入（语义先例与台账机制均在 main）；L1 / PC；零栈单测为主

## 目标

1. **全树普查**：grep 锚定全部「罐头文案+goal_status 回填 completed/failed」形态（关键词：`已处理或已过期`/`GoalStatus` 空回填/`completed` 兜底赋值），产出普查表（位置+触发路径+是否伪造）；
2. **同型修**：确认伪造的站点对齐 327/330 两态回执语义（retired/not_found，goal_status 空串，WARN 留痕）；复用既有 retirement 台账或按面建最小对应物；
3. **单测**：每修站点镜像三态形态；红转绿记录；
4. **负结论如实**：普查后判定某站点回填是「合法语义」而非伪造（如真实 completed 路径）→ 登记不修，说明理由。

## 文件域

agent/internal/chat/（以普查锚点为准）+测试；工件 coord/runs/FIX-GOALSTATUS-FORGERY-SWEEP-1/。

## 验收标准

普查表+（如修）单测红转绿+chat 全包不回退+回执附 diff。

## 停止条件

某站点语义牵涉 workflow 状态机重构 → 登记候裁定不硬改。

## 领取/回执/验收

### 领取（334）

- 时间：2026-11-20
- main 基线：`948d7b1c07734e89af93d857687bbaad1939ae37`
- owner：GLM-5.3 flash 执行子代理 forge334 / PC
- 分支：`port/goalstatus-forgery-sweep-1`；worktree：`D:/Vit_DAW_wt_pc334forge`
- 领取时工作树状态：主树存在他卡/人工既有改动（CURRENT-STATE.md、VitApp/Workspace/*.xml、tracktion_engine、YUE 卡删除等）——记录不触碰，与本卡文件域（agent/internal/chat + coord 工件）不相交
- 前置核对：卡仍在 todo ✓ 派发已确认 ✓ 依赖 327/330 已合入 main ✓；并行卡 440（genapi）/441（webui）文件域不相交 ✓
- 普查先行结论（survey.md 已落盘工件目录）：真伪造 4 处（三个 workflow 恢复兜底 4961/5012/5083 + finalize 空回填与 CompleteGoal default 组成的 interaction 面第二伪造面 F4）；合法/存疑/域外分类见 survey.md
- 零栈单测卡：不起真实栈、不占 PC-RUNTIME-STACK

### 回执（334，done=自验候主管验收）

- **实现 commit**：`c656d675`（分支 `port/goalstatus-forgery-sweep-1`）；**合入 main=merge `9c0fd8c3`**；领取提交 `eb89e55a`。
- **工件目录**：`coord/runs/FIX-GOALSTATUS-FORGERY-SWEEP-1/`（survey.md 普查表 v2 + red_run_probes.txt 红跑活体复现 + PROGRESS.md 命令台账；随分支入库）。
- **普查结论**（survey.md 全表在卷）：**真伪造 5 处**（F1 automation choreo 兜底 4961 / F2 mix tick respond 兜底 5012 / F3 mix treatment 兜底 5083〔即 330 登记三处〕/ **F2b no_pending_mix_tick_candidate**〔mix_tick_confirmation.go:91——首轮 grep 被 head 截断漏收，F2 可达性分析补全：approve+无候选的实际活体路径，同面同触发孪生〕/ **F4 finalize 空回填+CompleteGoal default arm**〔interaction 面第二伪造面，镜像 330 plan 面红跑揭示形态〕）；合法 14 簇（含 reject 路径=决定已执行）；**存疑登记 7 处候裁定**（expired_pending_mix_tick_candidate 的 Error+completed 自相矛盾、三个 ambiguous 澄清轮、宽泛混音拦截收束、staleMixActionResponse、空 map 防御分支等，survey §三）；域外登记 2（webui interactionGuard/StandardActionCard 旧口径属 441 域）。
- **diff**（4 文件，+386/-33）：`server.go`（F1/F2/F3 替换为显式回执 + finalize 回填与 CompleteGoal default 双守卫 + 谓词 receiptCarriesNoGoalStatus 并入旧 plan 谓词，3423 调用点随改）、`mix_tick_confirmation.go`（F2b goal_status 置空）、`interaction_retirement.go`（新 helper interactionContinuationMissingReceipt：WARN 留痕/GoalStatus 空串/decision_executed=false/mutation_performed=false/reason_candidates，payload 透传保留）、`goalstatus_forgery_sweep_test.go`（新，7 测三态镜像）。
- **红转绿**：红跑=detached worktree（948d7b1c 原样）+仅旧符号探针 4/4 FAIL，活体逐字复现四站点伪造（goal_status="completed"+罐头文案，red_run_probes.txt）；绿跑新增 7/7 PASS。
- **测试命令与退出码**（worktree pc334forge，agent/ 下）：`go build ./...` exit 0；新增定向 7/7 ok；`go test ./internal/chat/ -count=1` ok 129.632s exit 0；327/330 族定向（TestInteraction* 六测+TestPlanDecision 八测面）ok exit 0；`go test ./internal/pendingmanager/` ok exit 0；`go vet ./internal/chat/` 两处 unreachable=semantic_treatment_strategy.go **基线既有**（红跑 worktree 基线亲证一致，本卡未触该文件）。
- **语义红线落实**：本面回执为「卡已消费但续体丢失」单态 honest receipt（interaction_continuation_missing；两态 retired/not_found 分裂由 327 catch-all 承担，tombstone 已由 takePendingInteraction "consumed" 记录，重复应答得 already_retired）；豁免回执不触 goal runtime（镜像 plan 面不写 goal 先例）；存量测试亲证仅钉 stop_reason 路由、不钉被修站点 goal_status。
- **零栈声明**：未起真实栈、未占用 PC-RUNTIME-STACK、无 run ID。
- **端测边界声明（AGENTS §5）**：自验=Go 单测面（chat 全包绿）；真实栈面/渲染面/用户旅程未测——回执面为确定性服务端逻辑，与 327 已真栈活体证成的 interaction 面同构，候主管裁定是否需搭栈便车腿。
- **遗留**：存疑 7 处（survey §三）候主管裁定是否另开语义小卡；webui 呈现面对齐候 441；跨重启 in-process 台账语义与 327/330 一致不变。

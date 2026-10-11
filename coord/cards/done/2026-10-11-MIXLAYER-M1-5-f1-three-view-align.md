# MIXLAYER-M1-5：F1/queue/黑板三视图对齐（mix.report 纳入自由态结论）

- 发卡：GLM 主管决策侧 / 2026-10-11 早窗（同 M1-1 依据；MIXLAYER Q4 第⑤件事+M1 表）
- 派发确认：已确认（用户 2026-10-11 /morning 选定 PC 混音线首批立卡）
- 验收负责人：GLM 主管决策流
- 池序 68；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / **M1-1+M1-2 合入后领取**
- 模型分级：L1 / GLM 或 flash 执行会话（PC）

## 目标

F1 混音审查视图对齐（现状：mix_report.v1 账本汇总+Mixboard 决策账本现行，但看不见实验线与节点队列）：

1. mix.report 时间线纳入自由态 settle 结论（MIXLAYER Q4 第⑤件事——否则 F1 审查看不见实验线）；
2. mix.report 纳入 queue 节点状态视图（M1-2 产出）。

## 文件域

mixboard/mix.report 工具面（chat catalog 注册处）+ 对应单测；不越入 M1-1/M1-2 已合入文件的重构。

## 并行与资源

依赖 M1-1+M1-2；PC 真栈串行占用（PC-RUNTIME-STACK.md）。

## 验收标准

- 真栈烟测：mix.report 输出含最近 settle 结论行+queue 节点状态行，exit 0。
- `go test ./... -count=1` 0 FAIL + gofmt 净。

## 停止条件

M1-1/M1-2 合入形态与本卡接口假设不符 → 上交对齐。

- 领取：2026-10-11 20:35 PC / origin/main=4f185c27105bb51c860bb9fc6610d47344a08797（含 M1-3 领取 4f185c27，已核对其域 canary/routing 与本卡零交集）/ owner=GLM-5.3 执行流（ZCode PC 会话 sess_155c5ae2，2026-10-11 晚窗）/ 分支=port/mixlayer-m1-5 / worktree=D:/Vit_DAW_wt_mixlayer_m1_5 / 领取提交=013b6615
- 回执：实现 commit **8e9ce5f1** + 工件 commit **6cc0ec06**（均已推远端 @port/mixlayer-m1-5）；栈占用 435ebd4b→释放 8538a26d；真栈烟测 run **210405 与 210903 两轮 PASS exit 0**（coord/runs/FS-LARGEPROJECT-SMOKE-1/，被测 HEAD=8e9ce5f1）。
  - **目标①settle 结论纳入时间线（mixboard 面）**：`decision_ledger.go` BuildMixReport 只读消费 M1-1 持久层 `ReadFreeStateConclusionBoard`（同 Store 方法，零 store 文件改动）→ 新增 `free_state_conclusion_timeline`（SettledAt 升序全量）+ `latest_settle_conclusion`（最近一条）两字段（omitempty）。缺省语义：旧工程无结论=空板无段落无限定；坏盘 fail-open=报告照常+`free_state_conclusions_unreadable` 限定（不 partial 泄漏）。
  - **目标②queue 节点状态纳入（chat 面）**：新文件 `chat/mix_report_three_view.go`——只读消费 M1-2 `BuildMixWorkflowQueueView`+`mixWorkflowQueueViewMap`，向 mix_report 结果加 `mix_workflow_queue` 视图与三视图渲染 `message`（**最近 settle 结论行**（读报告体 latest 字段：kind outcome—summary（收口于 时戳），无结论显"无记录"诚实缺省）+ **queue 节点状态行**（23 节点+9 态汇总计数）+ 非缺省节点明细行 + 数据来源行（三路计数）+ 读取受限行）。**双面接线**：执行器拦截（goalrunner_chat.go `mixReportToolRequest`→base 分发后富化，模式=queue 拦截族之后第二个消费者面）+ `/agent/invoke` handleInvoke 富化分支（executor 不经过的传输面）。**零 catalog 变更**（mix_report 已注册 spec+alias，无新 args——与在飞 M1-3 零交集）、零 canary/routing 触碰、`read_only_observation_guard` 白名单语义不变（富化零变异）。传输面核verify：`compactStripSilenceInvokeResponseForTransport` 仅作用于 strip_silence 形态，新键完整过输。
  - **单测**：mixboard 4（时间线折叠升序+最新结论选择 / 旧工程空板缺省 / 坏盘 fail-open 限定不泄漏 / §11 旧 JSON 反序列化缺省+新 JSON 往返保序）+ chat 5（真实 store 结论+E1 标注富化全链断言 / 无结论诚实缺省+全 not_started 汇总 / nil 结果透传 / 工具匹配器含大小写与近义拒识 / 状态汇总固定序空态跳过）；`go test ./... -count=1` **exit 0（92 包 0 FAIL）**；gofmt 触碰文件净（检出 CRLF 误报按 M1-1/M1-2 先例 LF 归一核verify，decision_ledger.go 一处真实对齐已修）。
  - **真栈烟测（exit 0 达成）**：`fs_largeproject_smoke.ps1 -M1QueueRoundtrip -M1FiveThreeView`（新参数门控，默认关=原 M1-2 语义逐字不变；误用守卫=-M1FiveThreeView 必须伴 -M1QueueRoundtrip，E1 延期 needle 骑 M1Q q2 标注）。run **210405=首轮 PASS exit 0**：M15 八门全过（invoke ok+schema mix_report.v1+最近 settle 结论行+queue 节点状态行 23 节点+E1 母带 EQ[延期] 可见+**双折叠一致性**（报告 timeline=queue sources 读同一 M1-1 板，0=0）+settle 行真实性+A4 标记族零命中）+M1Q 四门+A1/A4 pass+A2/A3 软记录（M1-2 ruling 已接受语义）。run **210903=第二轮 PASS exit 0**（同形态复核轮）。两轮栈自 teardown 净，末轮后实测端口/进程=0。
  - **门槛语义声明（供验收裁定）**：`-M1FiveThreeView` 下 exit 门=M1Q+M15+A1+A4（A2/A3 软记录，同 M1-2 ruling 语义）；两轮 round 形态均 goal_terminal turns=1 无 settle 落盘→真栈行使的是 settle 行"无记录"诚实分支+双折叠一致门；**"有结论时行携带 conclusion_id/outcome/summary"分支由单测全链覆盖**（mixboard 折叠测+chat 富化测用真实 store 写入断言），真栈未行使（round 形态系 LLM 概率行为，第二轮后按 §8 止损不加跑）——若主管裁定需真栈行使该分支，可复用 settle_deliver_smoke 的判定腿组合另跑。
  - **端测边界声明**：①真栈覆盖=/agent/invoke HTTP 面（确定性，无 LLM 分支依赖）；执行器拦截面（LLM 工具调用路径）由代码接线+单测覆盖，真栈未驱动 LLM 发起 mix_report 调用（需 LLM 概率配合，属渲染面/旅程面范畴）②webui 渲染面未涉及（本卡无 webui 改动，message 键走 chat 事件流既有呈现路径）③两 run E:\ 素材 manifest 前中后一致（run_report 在卷）④mix.report 在 free-state 只读守卫白名单内，富化零变异，无 authority 面、无 canary/routing 依赖。
- 验收：**pass**（2026-10-11 晚窗，主管独立复核：全量 go test 亲跑 92 包 0 FAIL@worktree；run 210405 run_report 亲核——M15 八门全过（invoke_ok/report_schema/settle_line/queue_line/e1_deferred/fold_agree 0=0/settle_truth/a4_zero_markers）+M1Q 四门+A1/A4=true+teardown 净+exit 0（210903 复核轮同形态 PASS 回执在卷）；文件域亲核=decision_ledger.go 只读折叠+mix_report_three_view.go 新文件+goalrunner/server 双面接线，零 catalog/args 变更、canary/routing 零触碰，与 M1-3 零交集成立。**边界裁定两件**：①settle 行「有结论」分支真栈未行使（round 无 settle 落盘走诚实缺省分支，§8 止损不加跑）=接受——单测全链覆盖真 store 写入；**真栈正面行使挂 G3-REOPEN-COSTCMP-1 对比轮附加断言（卡面已加 rider）**。②执行器拦截面（LLM 发起 mix_report）真栈未驱动=接受，代码接线+单测覆盖，归旅程面后续。门槛语义=M1Q+M15+A1+A4 沿 M1-2 ruling。裁定文件 rulings/2026-10-11-M1-3-M1-5-pass.md）。实现+工件 cherry-pick 入 main=**905512b0+00ab1370**（port 原提交留远端；ps1 与 M1-3 合并版=主管手工并集，语法 ParseFile 校验过，组合态真栈行使挂下一轮 SMOKE 家族——见裁定§合并注记）。**M1 五序 5/5 收官**。

# D1GATE-SCRIPT-HYGIENE-1：D1 settle 门组脚本三处卫生修补（254 时代模式活栈验证欠账）

- 发卡：GLM 主管决策流 / 2026-11-06 晚窗（依据=[265 回执](../../cards/done/2026-11-06-D2-STEREO-THIN-1.md) 域外观察②+[265 裁定](../../rulings/2026-11-06-D2-STEREO-THIN-1-pass.md) §三-1——254 时代 D1 settle 门 B/D 脚本模式从未被活栈验证，265 四轮证明至少三处在 1-mutation 分支会同样失败）
- 派发确认：已确认（主管发卡，自主批次）；验收负责人：GLM 主管决策流
- 池序 268；目标=scripts/run_free_state_d1_smoke.ps1（D1 settle 门 B/D 段）+如需配套 py；P2 / 0.25 天 / 无依赖（与 265 已修的 D2 门组四处不同段）；L1 / PC；零真栈（构造验证，模式对齐 265 修复先例）

## 目标

1. 三处修补（对齐 265 在 D2 门组的同款修复模式）：①ANSI 读 UTF-8 store（verdict/needs_confirmation 文件读取编码）②`mix_queue_view` 工具名→现役名（`mix.report` 族）③needs_confirmation 属性访问路径（continuations[].pending_interaction 面 vs goal.status 面——参 262 L3 取证锚点）；
2. 构造验证：桩工程/离线 store 走 D1 settle 门 B/D 两门断言路径（零真栈零 LLM，模式参 260 三 probe）；
3. 回归：260 已合入的相位门分支化语义零回退。

## 约束

文件域=run_free_state_d1_smoke.ps1 D1 settle 门段+如需 py 配套+构造验证工件；禁动 D2 门组（265 已修）；禁动 runner 断言语义本体（只修读取面/工具名/访问路径）；§10 防污染纪律照用。

## 停止条件

构造验证暴露第四处同族缺陷→如实记录一并上交（属同段可顺手修，跨段则另开）；断言语义问题（非读取面）→取证上交转裁定。

## 领取/回执/验收

- 领取：2026-11-06T21:58+08:00 / origin/main=96aa9240（领取时最新；卡面 dispatch 基线 46497e3b 之后仅协调笔 96aa9240=267 卡面移动，agent/scripts 零 diff 亲证 git diff 46497e3b..96aa9240 --stat 仅 coord/cards/ 一个文件）/ owner=GLM 主会话 flash-worker 子代理（PC，ZCode 执行会话 268impl）/ 分支=port/d1gate-script-hygiene-1（worktree=D:\Vit_DAW_wt_268impl，实现基线=96aa9240）/ 领取时主仓工作树既有改动与 LOT 全部不属于本卡文件域（VitApp Workspace 运行时 XML×2、UX-AB-AUDITION 旧工件×4、未跟踪 design/缓存若干），不整理不覆盖 / 领取提交=aee1fa6c（已推 main）
- 回执（执行侧 2026-11-06 22:1x）：
  - **实现**：`port/d1gate-script-hygiene-1`（已推 origin）e0f477cf=scripts/run_free_state_d1_smoke.ps1 四处读取面修补+构造验证工件 87 文件。①门 B（:1123）store 读取加 `-Encoding UTF8`（265 ca721b18 门 B 同根因：Go 写的 BOM-less UTF-8 含中文摘要，ANSI 默认读多字节截断→ConvertFrom-Json throw→catch 吞为 no-record 静默失败）；②门 D（:1229-1248）`mix_queue_view`→`mix.report`+结果路径 `result.view.nodes`→守卫访问 `result.mix_workflow_queue.nodes`（265 门 D 同款：queue view 挂 M1-5 三视图，无独立 invoke 工具）；③门 C（:1143-1224）needs_confirmation 读取面=守卫访问（265 D2 门 C 模式：PSObject.Properties 守卫）+262 L3 durable walk——响应顶面不完整时读 `/agent/runtime/status` continuations[].pending_interaction（按会话过滤、kind=mix_tick_confirmation 精确匹配，形状参 run_mix_single_tick_e2e.ps1:398-430 既有重建 walk），breach 分支顶面/持久面任一可见即卡在，no-card 分支两 face 均无才放行——**断言语义本体零改动**（breach 必须挂卡/非 breach 必不挂卡/取消腿 mix_tick_rejected 逐字保留，diff 可核仅读取面行与新增读源块）；④**第四处同族顺手修**（:658-665，如实记录）：相位分支检测 `Get-Content $report -Raw` 加 `-Encoding UTF8`——py runner write_report ensure_ascii=False 落 BOM-less UTF-8（真实报告含中文），ANSI 读 throw 被吞→branch=""→合法 zero-mutation run 误入全门路径。段归属判定：该行是 D1 settle 门分支选择读取面（`if ($D1LoudnessSettle -and -not $zeroMutationBranch)` 的直接输入），与门段同一相位机；**非跨段**（跨段同族两处已如实记录不修：`:630` NOT_EXERCISED reason 读取、`:445` LW manifest 读取，均不在本卡文件域）。D2 门组零触碰（diff hunk 全集=D1 settle 段+行 658，grep 计数 0 亲证）。
  - **构造验证（零真栈零 LLM，模式参 260 三 probe）**：`probe_268.ps1` 8 腿 **ALL PASS exit 0**（attempt2；机器可读 probe_268_summary.json）——leg1 post-fix settle_pass 全绿 0 / leg2 pre-fix 同场景诚实败 1（门 B ANSI 吞 store 在盘 raw harvest 亲证+门 D PropertyNotFoundStrict 崩溃，两处缺陷一腿双证）/ leg3 post-fix breach 顶面挂卡 0（face=chat_response_interaction_requests+取消腿 mix_tick_rejected）/ leg4 post-fix breach L3 拆分面 0（face=runtime_status_pending_interaction=durable walk 找到卡）/ leg5 pre-fix 同面崩溃 1 / leg6 post-fix zero-mutation 中文报告 skip 面 0 / leg7 pre-fix 同报告全门误路由 1 / leg8 pre-fix ASCII 报告控制腿 skip 面 0（隔离编码变量）。260 相位分支化语义零回退（leg6/leg8 判定面=260 probe3 leg1 形制）。attempt1 6/8 留档（两腿败于本 probe 断言锚点与桩忠实性——StrictMode 文案本地化改用 PropertyNotFoundStrict 锚+桩补 needs_confirmation:false 恒序列化忠实性——被测脚本零改动，工件 attempt1_20261106_2207/）。
  - **端测边界声明（AGENTS §5）**：本卡=冒测脚本读取面修补+构造验证，零 Go/零 webui 渲染面/零用户旅程，E2E-WEBUI-1/JOURNEY-1 不适用；卡面口径=零真栈构造验证，本回执按此交付；构造腿证明脚本层读取面/工具名/访问路径行为，不证明真实栈缺陷触发频率（该事实由 265 四轮真栈工件钉死）。修复后门组的活栈验证将由 265 exit 0 补跑腿（主管已挂今夜非高峰自主批次）顺带覆盖——D1 settle 门与 D2 门共用同一 runner 路径。
  - **工件**：coord/runs/D1GATE-SCRIPT-HYGIENE-1/construct/（RESULTS.md 腿表+证据链+诚实边界、probe_268.ps1、stubs×3、8 腿 console+gates+chat+cancel+queue_view+verdict_store raw、probe_268_summary.json、attempt1 留档），随 e0f477cf 在 port 分支。
- 验收（2026-11-06 深夜，主管）：**pass**——四处修补（三处卡面+第四处 :658 同段追认=zero-mutation 误路由根因，段归属=相位分支检测直接输入成立）；构造验证 8/8+**主管交叉复跑 8/8 ALL PASS 亲跑**（pre-fix 自 96aa9240 字节级提取）；断言语义本体零改动+D2 门组零触碰+260 分支化零回退；attempt1 双轮留档+locale 无关断言=工件质量加分；并发 main 两度推进 autoStash 零丢失+域核验良好。跨段两处（:630/:445）挂编码读取卫生族随下张卫生卡。合入 e98caae0 零冲突——**D1 settle 门组仪器债（254 时代模式）就此清偿**，活栈验证由 265 补跑腿顺带覆盖（同 runner 路径）。详见 [rulings/2026-11-06-D1GATE-SCRIPT-HYGIENE-1-pass.md](../../rulings/2026-11-06-D1GATE-SCRIPT-HYGIENE-1-pass.md)。

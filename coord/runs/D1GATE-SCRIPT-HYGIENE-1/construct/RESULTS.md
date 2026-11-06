# D1GATE-SCRIPT-HYGIENE-1 构造验证工件（probe_268）

- 执行：GLM 主会话 flash-worker 子代理（PC，worktree D:\Vit_DAW_wt_268impl，分支 port/d1gate-script-hygiene-1）
- 日期：2026-11-06 晚窗（attempt1 22:07 / attempt2 22:10）
- 口径：零真栈零 LLM（卡面定义）；模式参 260 三 probe（TEMP 隔离桩仓 × 每腿独立 × pre/post-fix 对照）
- 驱动：`probe_268.ps1`（8 腿）——**attempt2 exit 0，8/8 ALL PASS**；attempt1 6/8（两腿败于本 probe 断言面与桩忠实性，非被测脚本，见 §三）
- pre-fix 脚本=领取基线 `git show 96aa9240:scripts/run_free_state_d1_smoke.ps1`（bash 字节级重定向，BOM 保留）；post-fix=本分支工作树
- 桩面：`stubs/stub_agent_face_268.py`（场景化 agent HTTP face+5555 裸听）、`stubs/free_state_d1_smoke_268_stub.py`（runner 报告+verdict store 桩）、`stubs/dev_agent_smoke_stub.ps1`（栈启动桩，260 框架）

## 一、腿表（attempt2，全部机器可读于 probe_268_summary.json）

| 腿 | branch | scenario | 修复点 | 期望 | exit | 断言 | 结果 |
|---|---|---|---|---|---|---|---|
| leg1_postfix_settle_pass | applied | settle_pass | ①②③ | 全绿+PASS 尾 | 0 | gates 四面 true+failures 0+no-card ok+D1 node in_progress | PASS |
| leg2_prefix_settle_pass | applied | settle_pass | ①②证据 | 诚实败 | 1 | 门 B「no d1_settle trigger record」+ store 在盘（raw harvest）+ PropertyNotFoundStrict 崩溃+无 PASS 尾 | PASS |
| leg3_postfix_breach_response | applied | breach_response | ③ | 全绿 | 0 | face=chat_response_interaction_requests+取消腿 mix_tick_rejected | PASS |
| leg4_postfix_breach_durable | applied | breach_durable | ③（L3 面） | 全绿 | 0 | face=runtime_status_pending_interaction（durable walk 找到卡） | PASS |
| leg5_prefix_breach_durable | applied | breach_durable | ③证据 | 诚实败 | 1 | PropertyNotFoundStrict（裸访问 needs_confirmation 缺席键）+无 face=+无 PASS 尾 | PASS |
| leg6_postfix_zero_mut_utf8 | zero_mutation | settle_pass | ④回归 | skip 面+exit 0 | 0 | phase_reachable=false+d1_branch=zero_mutation_boundary+四门 skipped_branch_unreachable+BRANCH-SKIP+PASS 尾 | PASS |
| leg7_prefix_zero_mut_utf8 | zero_mutation | settle_pass | ④证据 | 诚实败 | 1 | 全门路径误路由（console「gate A」实跑）+无 BRANCH-SKIP | PASS |
| leg8_prefix_zero_mut_ascii | zero_mutation | settle_pass | ④控制腿 | skip 面+exit 0 | 0 | 同 pre-fix 代码、仅报告 ASCII——skip 面完好（隔离编码变量） | PASS |

## 二、逐修复点证据链

- **修复①（门 B ANSI 读 UTF-8 store）**：leg2 pre-fix——桩 store 落盘（BOM-less UTF-8，中文摘要紧贴闭引号，265 run3 同款字节形），pre-fix `Get-Content -Raw`（ANSI）多字节截断→ConvertFrom-Json throw→catch 吞为「no d1_settle trigger record found」FAIL（console 逐字在案）；同字节 store post-fix leg1 解析过、trigger_kind=d1_settle 命中（leg1 gates verdict_on_disk=true）。store 原件两腿均 harvest：`leg1/leg2_..._loudness_watch_verdict_store_raw.json`。
- **修复②（mix_queue_view→mix.report+result.mix_workflow_queue.nodes）**：leg2 pre-fix——桩 face 对 mix_queue_view 回真实栈同款 unknown-tool 错误信封（无 result 键），门 D `$queueResp.result` 裸访问→PropertyNotFoundStrict 崩溃（console：门 D 头「mix_queue_view」后紧跟 FullyQualifiedErrorId: PropertyNotFoundStrict）；post-fix leg1/3/4 门 D 头「mix.report queue view」→D1 节点 state 读出（in_progress / needs_confirmation 各场景）。
- **修复③（needs_confirmation 读取面=响应顶面守卫访问+continuations[].pending_interaction durable walk）**：leg5 pre-fix——L3 拆分面（响应无 needs_confirmation 键、卡只在 runtime status）→裸访问 `$settleResp.needs_confirmation` PropertyNotFoundStrict 崩溃；leg4 post-fix——同一 L3 面下 walk 在 continuations[].pending_interaction 找到 mix_tick_confirmation park→「breach parked a governed mix_tick_confirmation card (face=runtime_status_pending_interaction) with zero mutation」，取消腿 mix_tick_rejected 照常；leg3 post-fix——直挂响应顶面场景 face=chat_response_interaction_requests 同绿（两读取面并存，顶面优先语义不变）。
- **修复④（相位分支检测读 UTF-8 报告——第四处同族，D1 settle 门分支选择读取面）**：leg7 pre-fix——runner 报告（ensure_ascii=False 中文注释，真实 write_report 形）ANSI 读→ConvertFrom-Json throw→吞为 branch=""→合法 zero-mutation run 误入全门路径（console 门 A/B/C/D 实跑+门 B 结构性 FAIL，随后门 D pre-fix 崩溃）；leg6 post-fix——同报告 UTF8 读→branch=zero_mutation_boundary→skip-with-reason 判定面+exit 0；leg8 控制腿——同 pre-fix 代码+ASCII 报告→skip 面 exit 0（证明 leg7 败因=编码非分支逻辑）。260 分支化语义零回退（leg6/leg8 判定面与 260 probe3 leg1 形制一致）。

## 三、attempt1→attempt2 变更（probe/桩面，非被测脚本）

1. 桩忠实性修正：settle_pass 响应补 `needs_confirmation:false`（真实 ChatResponse 无 omitempty 恒序列化；attempt1 漏键使 leg2 在门 C 提前崩、未走到门 D 的 mix_queue_view 取证点）；
2. probe 断言锚点：PS 5.1 StrictMode 错误文案为本地化中文，改用 locale 无关的 `PropertyNotFoundStrict`（FullyQualifiedErrorId）；leg2 增 store-on-disk raw harvest 断言（证明败因=读取面而非缺席）。
- attempt1 全套工件留档：`attempt1_20261106_2207/`。

## 四、诚实边界（§7 口径）

- 本验证=脚本层读取面/工具名/访问路径的构造证明；**不证明**真实栈上这些缺陷的触发频率（该事实由 265 四轮真栈工件钉死）；桩响应形状按 Go 源（ChatResponse 序列化、mix.report 三视图、runtime status continuations 面）与 265/262 真栈工件钉定；
- 门 A 断言语义、取消腿语义、门 B/D 期望值、1-mutation 分支门组本体=逐字未动（git diff 可核：仅读取面行+守卫与 walk 块）；
- 跨段同族观察（本卡不修，交决策侧）：`run_free_state_d1_smoke.ps1:630`（NOT_EXERCISED reason 读取，无 -Encoding UTF8，runner exit 3 面）与 `:445`（LW manifest 读取，同族）；均不在 D1 settle 门段文件域内。

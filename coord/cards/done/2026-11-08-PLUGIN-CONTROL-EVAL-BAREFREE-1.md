# PLUGIN-CONTROL-EVAL-BAREFREE-1：裸面+守卫许可臂（真裸写能力测量——「如果允许，模型能不能」的唯一干净测试）

- 发卡：GLM 主管决策流 / 2026-11-08 深夜（依据=[结论档 ⓪ 修订](../../rulings/2026-11-08-plugin-control-eval-line-conclusion.md)——9/9 写入失败轮 payload 解剖=治理守卫阻断非编码错误；**用户裁定 2026-11-08 深夜：需要一轮新实验，确保模型真的能够裸写参数**）
- 派发确认：已确认（用户裁定+主管发卡）；验收负责人：GLM 主管决策流
- 池序 318；目标仓库=主仓 agent（守卫许可缝）+scripts/工件；P1 / 0.5-1 天 / 依赖=317 工件在库（缝已合入 main）；L2 / PC；零栈先行+真栈

## 目标

1. **零栈解剖先行**：T1 写入闸门机理定位（B-R1 payload `agent_llm_payload_capture.jsonl` T1 三轮 tool_result 错误原文+代码锚点——「先观察、再确认」判定面在读/写路径的拦截点）+T3/T4 typed-surface 守卫定位（`typed_transient_shaper_surface_unavailable` 错误源）；
2. **守卫许可缝（test-only）**：新 env 键（如 `VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE`，312 检索缝同模式——**默认 unset 生产字节不变**）：缝开时 ①T1 型写入闸门对插件参数读写路径放行 ②typed-surface 守卫对裸写放行（受护族允许落地）；配对单测（缝开=受护参数裸写落地/缝关=现状拦截两向钉）；
3. **BAREFREE 18 轮**：A-pure 同款裸面（S1-S5+S2b）+守卫许可缝开+**无检索无提示**（单变量：唯一差异=守卫许可）；cases/判据/caps/T4 标注与 317 逐字同；
4. **装置门**：既有面全复刻+**守卫许可亲证**（probe：受护参数裸写真实落地+回读变化）；缝关态回归亲证（unset=拦截恢复）；
5. **判定核心（结论档待办①兑现）**：T1/T3/T4 在守卫许可下的 pass 率=**真裸写能力**；写入落地后 readback 校验编码正确性（value_encoding 终于可测）；consolidate **新增 guard_blocked_write 分类**（tool_result 守卫错误形态识别——终态标签≠机理的教训落地，本臂若再现守卫拦截不得误标 value_encoding_error）；
6. **回报**：barefree vs A-pure（同面守卫开关对照）为核心比较+并入全阶梯对照；「模型裸写受护族能力」正式答案。

## 约束

- 文件域=守卫所在包（chat/agentloop/policy/tools 按解剖定位）±测试+工件+编排复刻；生产默认不变红线；不触 L1 三件（314/315）域（edu_jam/server 布位段）；
- 真栈协议全链；栈排队（L1 真栈腿可能候栈——登记前重读，先到先得不抢）。

## 停止条件

解剖发现守卫与语义入口/routing 深度耦合无法缝隔离→取证上交转装置设计裁定；缝泄露任何形态停段；同型环境失败两轮止损。

## 预算

≈¥2-4 上界；§8 跑前成文。

## 领取/回执/验收

- 领取：2026-11-08 深夜 / origin/main=7ddae343（≥7ddae343 含 317 装置与 BR1 缝；worktree 基线干净：status 零行、diff --stat 空）/ owner=GLM-5.3（ZCode）L2 执行子代理·PC·本会话 / 分支=port/plugin-control-eval-barefree-1 / worktree=D:/Vit_DAW_wt_318barefree / 领取提交=coord/318-claim
- 回执：commit 8d2b2f55 @ port/plugin-control-eval-barefree-1（缝 05a5869e+装置 3456d5e0/BOM 修复 c2e44870+run 工件全量在分支；报告=coord/runs/PLUGIN-CONTROL-EVAL-BAREFREE-1/RUNBOOK.md+arm_barefree_20261108_221634/arm_barefree_summary.json）
  - **零栈解剖（RUNBOOK §一 冻结）**：T1 型闸门=agentloop `messageLoopToolGuardIssue` mix-observe-first 链（message_loop.go ~5504/5595+6374 blocked 表+`messageLoopAppendGuardGate` ~2803 呈现 `<tool_gate>`；payload 实证 T1_r1 四次 `plugin.get_parameters` 全拦零写入可达）；T3/T4 守卫=chat `guardTransientShaperOwnedGenericParameterWrite`（goalrunner_chat.go ~1341）unresolved_transient_shaper_surface fail-closed（payload 实证六轮 rejection_code=typed_transient_shaper_surface_unavailable+required_tools 指向 typed 面）；**关键缺陷位**：limiter/gate-expander/de-esser 有 genericWriteGuardFamilyExemption 豁免（VSTI-WRITEGUARD-1），transient-shaper 守卫没有——provably delay/reverb 的 T3/T4 撞 transient-shaper 候选匹配被拦；
  - **缝 diff**（05a5869e，test-only 默认 unset 字节不变）：`VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE` 双半=agentloop 闸门对插件参数读写放行（messageLoopIsPluginParameterReadWriteTool 八名变体；其余 blocked 工具保持拦截）+chat 七 typed generic-write 守卫入口短路；测试=agentloop `TestBarefree*` 5 项+chat `TestBarefree*` 3 项全绿（`go test ./internal/agentloop -run TestBarefree`=exit 0；`go test ./internal/chat -run TestBarefree`=exit 0；300-S0 forensics fixture 复刻 T3/T4 现状拦截钉+缝开 kernel 零读短路钉）；五包回归 harness+tools+contextruntime+agentloop+chat 全 exit 0；
  - **成绩（run arm_barefree_20261108_221634，18/18 评估 exit 0）**：逐案 BAREFREE=**T2 3/3、T3 3/3、T5 3/3、T6 3/3、T1 0/3、T4 0/3=4/6 案例级**；**写入三案 3/9 vs A-pure 0/9（同面守卫开关单变量对照）**；六方阶梯（案例级）：BAREFREE 4/6 > B-R1(317)=B(312)=A-pure(310) 3/6 < A(307) 5/6 ≈ P 5/6（P 单 repeat）；T3 从全裸面臂 0/3 → 3/3=**真裸写能力证实**（三轮 Damper 0.7 裸写 label 7==7 编码回读精确匹配；B-R1 解剖预测「正确编码被守卫拦」在许可后兑现）；T4 三轮全部写入落地（mutation 1-2/轮）但 0/3：r1 target 250ms 达标+mode Host 对、踩联动对缺陷（param 1 非目标变更，307 在案疑似规则缺陷）+预算停；r2/r3 写错参数面（display_ms 375=初值未动）——**value_encoding_error 终态标签首次=真实编码错误**（守卫拦截归零后）；T1 0/3 见下条；
  - **T1 机理修订（新发现上交，RUNBOOK §七.8 跑前冻结条款执行）**：三轮模型全部发出裸写尝试（write attempts 2/2/6 在账本）+读路径已通（`plugin.get_parameters` status=ok——B-R1 时被 agentloop tool_gate 全拦）=agentloop 缝半生效亲证；写路径被**第三层守卫拦**：harness `broadMixObserveFirstWriteGuard`（`agent/internal/harness/harness.go:2590`，Invoke pre-journal 段调用 ~1040；判据=自然混音请求+broadMixWriteCommand 表，**不认 mix.observe 已完成**）→ `pre_journal_validation_failed stage=broad_mix_observe_first_write_guard`（T1_r1 delta 在卷）。该层在 B-R1 不可见（agentloop 闸门先拦、请求到不了 harness）。按跑前冻结「缝外新守卫面=如实分类+锚点上交不扩修」：T1 3 轮分析分类=guard_blocked_write:harness_write_guard（评价器终态 value_encoding_error 原样保留并列呈现）；**T1 真裸写能力未测及**——候主管裁定（同键补缝 harness.go 一处 3 行+重跑新 run，或数据足够）；
  - **守卫许可亲证**：GW-OPEN 活体（零 LLM，B-R1 T3 受害者 Plates/Damper param 1 写 0.7）=write_status ok+parameters_changed+独立回读 0.2→0.7（guard_live_probe.json/gw_open.json 在卷）；GW-OFF 回归（unset+agent 重启=A-pure 态）=同写被拒（typed 文本在场）+值未变（gw_off.json）；agent 二进制 SHA256 重启前后一致断言过（6A170DF8…）；语料级=18 轮+probe 零 typed/T1 守卫拒绝形态（GW 面）+零检索执行（W0）+零 317 段（P0）；
  - **装置门**：seam_verify_barefree 九面+GW-LIVE probe 门 PASS（310 六面复刻+S2b 负锚检出力）+语料收口 PASS；selftest 17/17；consolidate 干跑 B-R1 语料=9/9 写入失败轮正确分类 guard_blocked_write（防误标验证）；
  - **预算**：108 任务+probe LLM 调用（T2_r0 probe 计入 108 总账）/1,578,211 in/58,525 out tokens——caps（216/8.64M/1.728M）内；cost=missing（价目未核）；估 ≈¥2-3 上界（312 同量级 118 调用/1.78M in 参考；卡面 ≈¥2-4 上界内）；
  - **批内环境重建 1 次（预冻结上限内）**：首跑 arm_barefree_20261108_221319 装置门 GW-LIVE 误 fail=PS5.1 `Set-Content -Encoding UTF8` BOM→python utf-8 解析 `_unparseable`（harness_invalid 装置脚本缺陷类；取证在卷：九面全绿唯 GW-LIVE 两腿、GW 实际 open/off 双成功在 gw_open/off.json）；修复=utf-8-sig 容错（c2e44870）+同语料回归 pass 后重跑；
  - **栈协议**：占用 3755f9ae（净空亲核 22:10:48 全零+≤60s 复核 22:11:32 全零+远端 owner 重读确认）→启栈→run→owner 亲拆 TEARDOWN_ALL_ZERO 22:29:53（编排 torn_down+执行侧六 TCP/两 UDP/五类进程全零亲核）→释放 1f86ac74；登记链接=coord/resources/PC-RUNTIME-STACK.md 头两条；
  - **端测边界声明**：本卡为实验腿（agent HTTP/ZMQ 真栈 18 任务轮+probe+零 LLM 活体 GW probe），不涉渲染面与用户旅程——无 E2E-WEBUI/JOURNEY 覆盖主张；agent 侧源码改动=授权守卫许可缝单点（agentloop message_loop.go+chat goalrunner_chat.go/plugin_multiband_apply.go+配对单测），默认 unset 字节不变（五包回归绿），生产启动路径永不设置；
  - **判定核心待主管**：①「模型裸写受护族能力」正式答案=T3 3/3（正确编码+落地+回读）+T4 落地但编码错（能力在场、编码不全对）+T1 未测及（第三层守卫）；②barefree vs A-pure 同面开关对照=写入三案 3/9 vs 0/9（T3 逐案转化 0/3→3/3）；③harness 第三层守卫处置（补缝重跑/维持）。
- 验收：（主管回填）

### 终验（主管，2026-11-08 深夜）

- **pass 整卡收口+缝合入 main**——per_case 亲证（T3 3/3=真裸写能力证实[0.7 落地+label 7==7 回读]；T2 3/3 维持；T4 0/3=写入已落地下的**真实**编码错误+联动对缺陷；T1 0/3=第三层 `broadMixObserveFirstWriteGuard`[harness.go:2590+Invoke~1040] 拦截——agentloop 缝半亲证[读路径已通]+RUNBOOK 预冻结分类 guard_blocked_write 合规）；守卫许可双亲证（GW-OPEN 0.2→0.7 落地+GW-OFF 拦截恢复）；consolidate B-R1 语料干跑 9/9 守卫拦截正确分类=taxonomy 修正兑现；五包回归绿；栈协议全链（3755f9ae→1f86ac74）。
- **裁定**：①**T1 真裸写能力补完腿授权（319）**——seam 第三层 harness 守卫+T1×3 定向复跑（用户裁定「确保真的能裸写」的完整兑现）；②T4 定性=写入落地下的真实编码边界（联动对缺陷在案+参数面选错）——**首次 value_encoding_error 标签与机理一致**；③T3 结论=裸写能力成立的第一个直接证据。
- **harness.go 域预声明**：319 的 harness 守卫缝与 273 未合入分支（Authorize* 家族，不同函数区）同文件不同域——合入时主管处理。

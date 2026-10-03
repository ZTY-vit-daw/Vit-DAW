# REPLY-GEN-TOOLGATE-1：二轮观察问答终局回复生成失败——「未知或不允许的工具：」空名（P1）

- 池序 4（P1——判定链收口的下一层缺陷，阻塞判定→问答连续作业体验）；目标仓库=D:\Vit_DAW；来源=手测二号反馈 1；**取证已固化**：[runs/MANUAL-TEST2-20261003/FORENSIC.md](../../runs/MANUAL-TEST2-20261003/FORENSIC.md)（会话图 8 节点全量）
- 优先级 / 预估 / 依赖：P1 / 取证 0.25+修复 0.5 天 / 无
- 模型分级：L1 / GLM 首选（回复生成×工具准入链语义）
- 已核实事实（决策侧磁盘取证，勿重跑）：
  1. 场景：同会话一轮判定结算（19:37:21Z judgment_settle 落图）后，19:37:37Z 发观察问句「再帮我看看bass轨道的低频有没有什么问题」→ 工具操作**正常执行完成**（"前面的工具操作已完成"）→ 19:37:52Z 终局回复生成失败。
  2. 报错节点 `n_20261003T113752_f1df6feb`（msg_kind=error）：「前面的工具操作已完成，但最终回复生成失败：**未知或不允许的工具：**」——**工具名字段为空**。
  3. 与晨场 revision_stale 不同层（进链即停 vs 工具跑完死收口）——SETTLE-DELIVER 三症状已实证修复（见 FORENSIC），本卡是新缺陷面非其回归。
- 目标：
  1. **取证**：实锚"最终回复生成"段的工具准入判定（chat/goalrunner 回复组装面）——空名工具调用从何而来（LLM 发出空名调用？合成标记未过准入表？结算后上下文携带的内部句柄被当工具名？）；为何冒号后为空（错误模板格式化吞名 or 名字本空）。
  2. **修复**：按根因修——合法工具调用照常、非法/空名调用的失败面收口到该轮回复（不中断链、不静默吞）；错误文案含实际工具名（可诊断）。
  3. RED：进程内复现"判定结算后同会话观察问句"场景（可借 SETTLE-DELIVER 的 jsssParkedBoundaryServer 同族 fixture），pre-fix 红=报错空名/链中断。
  4. 回归：chat 全包+全仓 -count=1 零 FAIL+webui npm test（错误消息渲染面如涉）。
  5. 真栈烟测：判定→结算→观察问句→**收到正常观察回复** exit 0（泊位错峰）+用户手测复验。
- 文件域：agent/internal/chat/（回复生成/工具准入，实锚后申报）+测试。
- 约束：不动判定/结算链（已实证工作）；worktree 纪律；泊位申报。
- 验收标准：根因取证+RED→GREEN+全量 0 FAIL+真栈烟测 exit 0+用户手测复验。
- 停止条件：空名调用源自 LLM 供应商面（模型输出本身）→ 取证上交定护栏方案；根因在观察工具注册表层之外（queryengine 域）→ 实锚上交扩域裁定。
- 领取：（2026-10-03 20:20 二次领取（首领 19:55 的 mv 暂存被并行 VITNOTE-CONTAINER-4 领取提交误携后由 coord(fix) c04b39d5 归位 todo，本轮原子重领）/ origin/main=c04b39d51b5d5b4b9d5f1b02dc106c486c507d9e / main 工作树直做，PC 执行侧会话）
- 回执：
  - **实现 commit**：`edc4a077`（本地 main；推 `port/reply-gen-toolgate-1` 分支，main 合入留决策侧验收 cherry-pick）
  - **实锚根因**（不重跑取证，证据=FORENSIC+现场日志双读）：
    1. 失败点=goal_05439ab/turn_d4f4f0 的 message loop 工具准入（last_checkpoint=before_message_loop_tool）；`agent_message_loop_debug.jsonl` 两行实锤：19:37:50 `parse_failed`（模型输出 DSML 形态非 JSON）→ repair LLM 19:37:52 `repair_succeeded` 返回 `{"tool_calls":[{"name":"mix.read","arguments":{...}},{"name":"mix.derive",...}]}`——**OpenAI 风格 name/arguments 字段名**。
    2. 解码器只认 `tool`/`args`（planner.ToolCall json tag）→ 名字丢失成**空名调用**；`messageLoopOutputHasShape` 按 len(ToolCalls)>0 放行 → `allowedTool("")=false` 连拒 2 次（默认 MaxConsecutiveErrors=2）→ `r.fail("未知或不允许的工具："+空)` → chat 包装前缀=现场报错。**空名定性=协议字段名不匹配（解码层），非 LLM 裸空名、非 queryengine 域——未触发停止条件**。
  - **修复三件**（文件域实锚申报：agent/internal/planner + agent/internal/agentloop + scripts；**chat 包零改动**，判定/结算链零触碰）：
    1. `planner.ToolCall.UnmarshalJSON`：tool/args 缺位时回退 name/arguments 别名（单点覆盖 message loop/planner/repair 三条解码路径）；
    2. `toolAdmissionError`：非空名报错保留原文案（含名）；空名改报「缺少工具名（tool 与 name 别名均为空）」——5 处准入点统一，杜绝冒号后空串；
    3. 真·空名准入耗尽收口：只读观察在场且全部执行为只读族时，`r.complete`（物化观察摘要+含因诊断附注，不静默吞、不中断链）；否则照常 fail（文案已可诊断）。
  - **红绿**：新测试 `message_loop_reply_gen_toolgate_test.go` 三钉（现场 DSML/别名载荷逐字复现）。RED=三文件全退 stash 后两钉 `status="failed" error="未知或不允许的工具："`（与现场逐字一致）；GREEN=修复全在位三钉 PASS（钉①别名解码照常执行+终局回复、钉②空名收口物化观察+诊断保留、钉③文案可诊断）。
  - **回归**：`go test ./internal/planner ./internal/chat -count=1` PASS（chat 97.8s）；**全仓 `go test ./... -count=1` 退出码 0 零 FAIL**；webui 不涉（src 无该文案硬编码，错误节点通用渲染——渲染面边界声明）。
  - **真栈烟测**（泊位申报：三轮均 2026-10-03 20:10–20:24，18–24 非高峰窗口内；§8 声明在脚本头）：`scripts/reply_gen_toolgate_smoke.ps1` 三轮，**exit 0 未达成**，止损触发：
    - run 201047：判定→结算→**落库全通**（图节点 `judgment_settle:judgment-5394f3fd` 双针齐全，events 32 含 judgment.settled 全文）——但脚本 settle 断言误用 `/agent/state?detail=full` 面（该面不载异步 settle 行，兄弟卡 113348/121934 同类脚本面缺陷），未及二轮；断言已改 draft 会话图面（更强：含零 error 节点扫描）。
    - run 201555/201944：判定席前死亡，根因逐字相同 `terminal turn produced no admissible final decision after one strengthened retry`（自由态终局裁定模型随机分支，executed=0 本卡修复面未参与；兄弟台账 judgment_not_armed 同族概率面）——**同型连败 2 次触发止损**，工件保全待裁。
    - **端测边界**：全链 exit 0 留决策侧裁定（补跑 or 采信 run1 半链实证+进程内 RED→GREEN+全仓绿，兄弟 SETTLE-DELIVER「分裂证据+手测复验」先例）；[等待用户手测复验]（Godot 入口，同会话判定结算后发观察问句）。
  - **事故记录**：首领（19:55 mv+暂存未提交）被并行 VITNOTE-CONTAINER-4 领取提交误携、经 coord(fix) c04b39d5 归位 todo；20:20 原子重领（3bc43e48）——共享工作树暂存残留教训与协议 §3 增补吻合。
- 验收：（裁定文件 / 验收 commit）

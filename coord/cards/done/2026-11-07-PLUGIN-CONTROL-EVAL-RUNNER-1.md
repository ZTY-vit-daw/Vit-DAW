# PLUGIN-CONTROL-EVAL-RUNNER-1：效果器控制评估 runner 建设（S0 验证+P 基线执行）

- 发卡：GLM 主管决策流 / 2026-11-07 晚窗（依据=[299 PREP](../done/2026-11-07-PLUGIN-CONTROL-EVAL-PREP-1.md) next_card_scope+冻结 plan.json（coord/runs/PLUGIN-CONTROL-EVAL-PREP-1/plan.json）+[评估方案](../../reports/2026-11-07-plugin-control-model-evaluation-plan.md)）
- 派发确认：已确认（主管发卡）；验收负责人：GLM 主管决策流
- 池序 300；目标仓库=主仓 scripts+coord 工件；P1 / 0.5-1 天 / 依赖=299 已收口；L2 / PC；**模型矩阵=deepseek-flash 单模型（用户裁定 2026-11-07 晚：deepseek 为测试原型底线，不加对照模型）**
- **授权范围**：本卡含 S0（零 LLM 已知操作验证）+**S1-P（产品基线 6 轮，付费，预算按 plan.json per_round_caps，段级 go/no-go 回报主管）**；A*/B*/C 纯臂**不在本卡**（缝卡另立，候 P 结果）

## 目标

1. **runner 建设**（零栈先行）：按 plan.json next_card_scope——fixture builder（per-case 独立工作副本+哈希）、S0 known-op 验证器（零 LLM，per-case exit code：脚本化正确操作验证 T1-T6 目标可达/枚举单位解释正确/回读恢复有效——正确操作失败=夹具/接口问题不进模型打分）、arm driver（per-round 会话隔离+预算计数器，telemetry 口径）、独立评价器（readback+非目标项 diff+verdict JSON——评价器正确参数/目标状态不进被测模型上下文）、工件新 run_id 目录不覆盖。先例=pgtdc_compat_verify.ps1+b3_pan_layout_agent_smoke.py。
2. **S0 执行**（真栈，候栈排队：296→298→275 后）：六案例全零 LLM 验证 exit 0（或如实 blocked 证据）；§8 跑前成文（确定性零 LLM）。
3. **S1-P 执行**（S0 过后同栈窗口或再登记）：产品基线 6 轮（真实默认模型 deepseek-flash+默认提示词+默认披露），逐轮记录：入口可达/实际披露量/首次提案/执行请求响应/独立回读/非目标变化/终态/调用数+token+费用+时长；失败按 plan.json failure_taxonomy 分类；**单轮形态=任务开始→预算内操作与修正→终态**。
4. 回执：runner diff+S0 六案例结果+S1-P 六轮成绩表（case_id/arm/first_action/final_verdict/failure_tags/readback/mutation_count/tokens/cost/stop_reason）+预算实耗 vs caps；**P 结果落地即回报主管**（A/B/C 缝卡决策输入）。

## 约束

- 文件域=scripts/ 新 py/ps1（+dev_agent_smoke.ps1 加腿如需）+coord/runs/PLUGIN-CONTROL-EVAL-*/ 工件+卡面；**agent/ internals 与 VitApp/ 只读**（缝=EVAL-SEAM-1 另卡）；fixture 走独立临时副本（§10 防污染）；
- 真栈协议（§2.2+增补）全链；与在飞栈腿错峰（登记前 fetch 重读）；
- 评价器期望值不硬编码轨名等易变标识进生产码；密钥不入工件；
- S1-P 付费轮预算超 per_round_caps 即停该轮（stop_rules 按 plan.json）；环境重建上限跑前冻结。

## 停止条件

S0 正确操作腿失败→夹具/接口取证上交（不进模型轮）；S1-P 连续两轮同型环境类失败→止损取证；栈冲突按登记协议等待。

## 领取/回执/验收

- 领取：2026-11-07 18:49 +0800 / origin/main=8568f2e4175b2b293982aae4db33a7d289e86c72 / owner=ZCode GLM-5.3 L2-worker PC 执行子代理（会话 300runner）/ 分支=port/plugin-control-eval-runner-1 / worktree=D:/Vit_DAW_wt_300runner / 领取提交=61c685f2（coord/300-claim→main 已推已 fetch 重读）。领取基线：fresh worktree 自 origin/main，`git status --short` 与 `git diff --stat` 均为空（clean）。冻结计划取自 origin/port/plugin-control-eval-prep-1:coord/runs/PLUGIN-CONTROL-EVAL-PREP-1/plan.json（287 行，已通读）。
- 回执（执行侧 2026-11-07 21:00 收口，mv done/ 待主管终验）：
  - **diff 文件清单**（全在 `port/plugin-control-eval-runner-1`，agent/ 与 VitApp/ 零改动）：新增 `scripts/plugin_control_eval_cases.py`（T1-T6 冻结定义，锚 plan.json+pgtdc 工件）、`scripts/plugin_control_eval_driver.py`（selftest 45 项/s0/dryround/round/segment-p/evaluate 六子命令+独立评价器）、`scripts/plugin_control_eval.ps1`（栈包装：§8 预写+VIT_PROJECT_XML git-clean 隔离+per-run 遥测/载荷捕获 env+六口净空+TEARDOWN）；`coord/runs/PLUGIN-CONTROL-EVAL-RUNNER-1/`（RUNBOOK §8 跑前成文+8 个 run 目录+case_freeze）。commits：5a6e8a73（runner 三件）→d0b4e00d（RUNBOOK）→be05d03a/c52a4e24/4124faae（S0/dryround 四轮取证修复）→ccf8c78c（工件入库）。
  - **离线自测**：`python scripts/plugin_control_eval_driver.py selftest` → **exit 0（45 项全绿）**；ps1 `[System.Management.Automation.Language.Parser]::ParseFile` → **零错误**。
  - **S0（零 LLM，run1-5 取证迭代、run6 终局）**：`plugin_control_eval.ps1 -Mode s0` → **PS1 exit 0**（run=s0_20261107_204138）；六案例全 verified=冻结初态核对→正确操作→独立回读→非目标 diff→恢复回读；T1 Mix=35.0% exact（set_plugin_param）/T2 Decay=50.00% exact（非线性域 norm 0.5→0.0）/T3 Damper label="7"（typed apply enum，binding domain="enum" 裸字符串形态）/T4 kernel 面 0.25→251ms±10+Sync=Host 保持+**允许集冻结 {0}**（param 17 无联动）/T5 零 mutation Feedback=60/T6 5 参数无 decay 复核；**零 LLM 亲证=per-run 遥测文件零行（文件不存在）**。
  - **dryround（零 LLM 管线干跑）**：`-Mode dryround` → **PS1 exit 0**（dryround_20261107_205015），verdict=pass（scripted known-op 过全管线→独立评价器）。
  - **S1-P 成绩表（segment-p_20261107_205406，付费，deepseek-flash 产品默认路径）**：
    | case | first_action | final_verdict | failure_tags | mutation_count | llm_calls | in_tok | out_tok | cache_read | stop_reason |
    |---|---|---|---|---|---|---|---|---|---|
    | T1 | mix.observe（harness 面） | **pass** | — | 1（目标） | 5 | 103,972 | 1,450 | 45,824 | goal_completed |
    | T2 | （typed 接管路径，首响在 exchanges[0] 逐字在卷） | **pass** | — | 1（目标） | 5 | 74,679 | 1,875 | 29,440 | goal_completed |
    | T3 | 同上 | **pass** | — | 1（目标） | 7 | 103,406 | 2,250 | 52,352 | goal_completed |
    | T4 | 同上（asked_or_searched） | **fail** | unexpected_mutation | 2（目标+param17） | 9 | 189,572 | 20,104 | 61,952 | goal_completed |
    | T5 | 同上 | **pass**（零 mutation+识别 60） | — | 0 | 5 | 50,591 | 2,350 | 19,840 | goal_completed |
    | T6 | plugin_grabber.explain_controls→plugin.get_parameters（self_probed） | **pass**（限制声明+零写） | — | 0 | 7 | 72,512 | 2,921 | 28,288 | goal_completed |
    - T4 语义：模型正确达 249ms（±10 容差内）+Sync=Host 保持，但**故意同时写非目标 param 17（Delay Sec R）**（自述「左右两个时间参数同时写入」）——按冻结非目标规则记 unexpected_mutation，非接口失败；readback 工件=rounds/T4_r1/readback_{before,after}.json。
    - **预算实耗 vs caps**：38/72 调用、594,732/2,880,000 in、30,950/576,000 out（全在 caps 内；cache_read 合计 237,696 tok 逐轮在卷）；**cost=missing**（DeepSeek 价目快照未核，plan budgets.monetary pending，不事后折算）；计费=用户 DeepSeek 账户。
    - **被测模型面亲证**：telemetry 38 行零错误、model 全 deepseek-flash；payload capture 响应 raw.model=deepseek-flash（无别名重映射）；请求未设采样参数=供应商默认（如实记，preflight §1 待验项①②闭合、③账户版本快照仍开放）。
  - **域外取证两件上交主管（agent/ 只读域，只给锚点+S0 工件，未扩修）**：① generic `set_plugin_param` 在 H-Delay/Plates 上被 gate/expander+limiter typed 守卫 fail-closed 硬拒（unresolved_*_surface 不在放行集——H-Delay 非门限类却被拒；锚点 `agent/internal/chat/goalrunner_chat.go:1302-1340/1356-1394`，工件 s0 run5/run6 T4 forensics+run2 T3）；② `apply_delay_controls` value_ms=250 实落 norm 0.35065=352ms 却报 apply_status=exact（事务探针物理反演缺陷；锚点 `agent/internal/chat/plugin_compressor_apply.go:175-262`+`plugin_delay_apply.go:263-283`，工件 s0 run4/run5 T4 write_faces）。P 轮 T4 模型实走 typed 路径落 249ms 正确——与 S0 复现的 352ms 缺陷并存在卷（请求载荷差异待主管如需深挖）。
  - **真栈登记**：占用 963f7ba0（20:12）→释放随本批 coord/300-close 推 main；TEARDOWN_ALL_ZERO 亲核 21:00:01（六口 TCP+UDP 4444/4445+五类进程全零）；kernel=wt_228 F9E3DA0A 只读复用，agent=wt_300runner 现场构建（SHA256 随各 run_report）。
  - **端测边界声明**：S0/dryround=确定性腿（零 LLM，agent invoke+内核 ZMQ 面）；S1-P=agent HTTP 真栈腿（产品 chat 链路：语义入口→typed 接管→执行→独立回读）；**渲染面（webui DOM）与用户旅程不涉及**（实验腿不适用 AGENTS §5 渲染面门槛，无 E2E-WEBUI/JOURNEY 覆盖主张）。单轮=单样本，不证稳定性（plan §6 口径）。
  - 工件分支=`port/plugin-control-eval-runner-1`（ccf8c78c 已推）；工件根=`coord/runs/PLUGIN-CONTROL-EVAL-RUNNER-1/`（RUNBOOK+case_freeze+8 run 目录，17MB，每 run 新目录不覆盖）。
- 验收（2026-11-07 晚窗，主管）：**pass**——runner+S0+S1-P 全交付合格（selftest 45 绿+S0 六案例零 LLM 遥测亲证+P 六轮全评估）；**P 基线采信=5/6**（T4 败因=故意多写非目标参数的纪律问题，目标本身达标 249ms±10——非能力失败）；预算 38/72 调用 caps 内；模型面亲证（raw.model=deepseek-flash 无别名+零采样参数如实记）。**解读边界声明：T2/T3/T4 首动作均 typed 接管=产品语义机器承重，P=产品上限非模型裸能力——A/B 纯臂问题开放，缝卡 305 即发**。域外双缺陷：①typed 守卫 gate_expander/limiter 误拒=**302 卡面扩域**（同族分类修）；②apply_delay_controls ms 反演=**转卡 304**。单轮=单样本不证稳定性（plan §6 口径维持）。L1 台账 48h 窗口。

# PLUGIN-CONTROL-EVAL-S1A-1：S1-A 臂执行（裸参数 18 轮，缝开）

- 发卡：GLM 主管决策流 / 2026-11-07 深夜（依据=[300 P 基线 5/6](../done/2026-11-07-PLUGIN-CONTROL-EVAL-RUNNER-1.md)+[305 四缝合入](../done/2026-11-07-PLUGIN-CONTROL-EVAL-SEAM-1.md)+冻结 plan.json segment_caps——P 臂 go 条件已过，下一分段=A 臂）
- 派发确认：已确认（主管发卡·分段 go/no-go 第 2 段）；验收负责人：GLM 主管决策流
- 池序 307；目标仓库=主仓 scripts（复用 300 runner）+工件；P2 / 0.5 天 / 依赖=305 已合入；L2 / PC；真栈占用（登记协议照旧）

## 目标

1. **A 臂 18 轮**：六案例 × 3 重复，deepseek-flash 单模型，`VIT_PLUGIN_CONTROL_EVAL_SEAM=1` 缝开（S1-S4 全生效）——模型只见裸参数面（原生 param_id/name/unit/min/max/value/value_text）+通用读写回读工具+原生枚举，无 typed 接管/无语义字段/无语义工具描述/无 typed 路由；
2. **A 臂口径声明项**（305 验收裁定）：quick_controls 选择面+recommended_groups 组名=最小结构残留，结果解读时声明；B 臂检索（web.search/fetch）**本段不开**（A=不联网）；
3. 执行与评价：300 driver 的 round/segment 面逐轮跑（每轮新会话+同初始态+预算计数）；独立评价器判定（readback+非目标 diff）；失败按 plan.json failure_taxonomy 分类（A 臂特有：`seam_leakage` 若观察到语义字段泄露=装置失效优先报告）；
4. 预算：plan.json A 段 caps（≤18 任务轮；per_round_caps 沿用）；§8 跑前成文；单段 go/no-go——**A 结果落地即回报主管**（B/C 段决策输入）；
5. 回执：18 轮成绩表（case/arm= A/repeat/first_action/final_verdict/failure_tags/readback 工件/mutation_count/tokens/cost/stop_reason）+预算实耗+A vs P 逐案对照表+装置验证（缝开亲证：telemetry/payload capture 无语义字段+路由无 typed 接管）。

## 约束

- 文件域=工件目录+卡面；源码零改动（305 已在 main；发现装置缺陷只取证）；agent 源码只读；
- 真栈协议全链；工件新目录；评价器期望不进模型上下文。

## 停止条件

缝泄露（语义字段入模型上下文）→装置失效立即停段取证；同型环境失败两轮→止损；断言失败取证不重跑。

## 领取/回执/验收

- 领取：2026-11-07 22:14 +0800 / origin/main=d4bd3496bd07e3e949588eb71d6de9d9a82f8817 / owner=ZCode GLM-5.3 L2-worker PC 执行子代理（会话 307 s1a runner；发卡人置 doing/ 并声明「卡面回填领取信息推协调提交即视领取」）/ 分支=port/plugin-control-eval-s1a-1 / worktree=D:/Vit_DAW_wt_307s1a / 领取提交=本协调提交（coord/307-claim→main，hash 见 git log）。领取基线：fresh worktree 自 origin/main d4bd3496，`git status --short` 与 `git diff --stat` 均空（clean）。冻结面：plan.json 取自 origin/port/plugin-control-eval-prep-1（A_star 段 caps/口径），runner 三件取自 origin/port/plugin-control-eval-runner-1@ccf8c78c（300 验收 pass 但尚未合入 main——按本卡面「复用 300 runner」快照入本卡 worktree 分支，含 P 工件引用）；缝实现=已合入 main 的 1775c10b（305 验收 pass）。执行侧声明：源码零改动（agent/ 与 VitApp/ 只读；300 runner 三件原样快照不加改）；工件根=coord/runs/PLUGIN-CONTROL-EVAL-S1A-1/。
- 回执（执行侧 2026-11-07 23:05 +0800 收口，mv done/ 待主管终验）：
  - **diff 文件清单**（全在 `port/plugin-control-eval-s1a-1`，**源码零改动**——agent/ 与 VitApp/ 与 scripts/ 零触碰，300 runner 两件原样快照）：`coord/runs/PLUGIN-CONTROL-EVAL-S1A-1/`＝RUNBOOK.md（§8 跑前冻结+附录 A 跑中取证）+runner_frozen/（冻结快照 blob 逐字：cases=d48ca7cf/driver/plan.json=3fdb595a）+p_baseline/（P 对照输入 blob 逐字一致）+arm_a_orchestrator.ps1（同模式新增编排：净空预检/env 冻结/SEAM=1 透传/六口拆除面复刻 300 ps1）+seam_verify.py（装置验证门，双向合成自测）+consolidate_a.py（合并+逐轮 write_path+corpus 旗标）+selftest_console.log（45+2 项 PASS exit 0）+orchestrator_master_console.log+4 个 run 目录（222543 run1/222801 run2/223545 run3 终局/每 run 新目录不覆盖）。commits：预冻结→bcbc5fb7（run1/2 取证修复）→a06f5af1（run3 收口）。
  - **离线自测**：`python runner_frozen/plugin_control_eval_driver.py selftest`（PYTHONPATH=scripts）＝**exit 0（47 项 PASS）**；ps1 Parser::ParseFile 零错误；seam_verify 合成洁样本=pass exit 0 / P 基线当 A 输入=seam_leakage exit 4（检出力双向证明）。
  - **S0 重冻（零 LLM）**：run3 `s0`=**exit 0 六案全 verified**（T4 允许集冻结 {0}；遥测零行亲证）。run1 T3（Plates，案例序首个 Waves shell）rack_add_node JUCE 超时=fixture_failure 签名#1，判冷载时序（T4/T5 同族后到即 verified），重跑过——非同型两连败，未触止损。
  - **dryround 缝开（零 LLM）**：run3 exit 0（管线 pass+遥测零行）——305 候队列「dryround 缝开腿」就此闭合。
  - **装置验证（门=probe T1_r0 付费 6 调用，task 轮前）**：**pass**——S2 五字段 A 请求 payload 全零 vs P 对照 70/30/30/36（方法检出力先行证明；alias 键形 P 亦 0）；S4 骨架 typed 路由首规则 A=0 vs P=24；S1 semantic_entry+message_loop 在场+无 typed apply 面；模型面 telemetry 100/100+response raw.model 100/100=deepseek-flash 零错误。
  - **S1-A 18 轮成绩表**（run3=arm_a_20261107_223545；arm=A 重标口径：冻结 driver 原始记录 arm 字段="P" 原样在卷；readback 工件=rounds/<case>_r<n>/readback_{before,after}.json；cost=missing 价目未核不折算；wp=write_path）：
    | case rep | verdict | failure_tags | mut | first_action | wp | calls | in_tok | out_tok | cache_r | stop |
    |---|---|---|---|---|---|---|---|---|---|---|
    | T1 r1/r2/r3 | **pass×3** | — | 1 | mix.observe/None/None | typed_adapter×3 | 5/5/5 | 104,506/105,523/106,742 | 1,892/1,801/1,977 | 49,664/49,792/49,536 | goal_completed×3 |
    | T2 r1/r2/r3 | **pass×3** | — | 1 | None×3 | typed_adapter/none_obs/none_obs | 5/5/5 | 86,097/80,118/81,335 | 3,839/3,094/2,146 | 30,592/38,144/38,144 | goal_completed×3 |
    | T3 r1/r2/r3 | **pass×3** | r1=model_budget_exhausted | 1 | plugin.get_parameters/None/None | typed_adapter×3 | 9/8/4 | 247,323/148,700/82,749 | 5,303/3,493/1,372 | 113,536/66,432/38,144 | r1=预算停(写入已落 pass)/r2,r3=goal_completed |
    | T4 r1 | **pass** | — | 1 | None | typed_adapter | 5 | 63,073 | 1,539 | 16,512 | goal_completed |
    | T4 r2/r3 | **fail×2** | unexpected_mutation×2 | 2 | None/None | typed_adapter/none_obs | 5/5 | 65,438/63,833 | 3,024/3,219 | 19,712/19,712 | goal_completed×2（自述「左右两路同时写入」，Delay Sec R 被写=同 P 的 T4 纪律形态） |
    | T5 r1/r2/r3 | **pass×3**（零 mutation+识别 60） | — | 0 | None×3 | none_obs | 5/5/4 | 54,274/55,305/41,421 | 3,144/1,555/1,246 | 18,048/23,040/15,104 | goal_completed×3 |
    | T6 r1/r2/r3 | **pass×3**（限制声明+零写） | — | 0 | None×3 | none_obs | 5/4/4 | 56,491/36,024/37,960 | 1,911/1,459/1,629 | 19,456/12,800/12,800 | goal_completed×3 |
    - 轮级 **16/18 pass**；案例级 **5/6**（T4 1/3）。first_action=None/wp=none_observed=agent log 轮转截断证据缺口（-keep-last-log-lines 800；verdict 由独立回读产生不受影响，逐轮声明）。
  - **A vs P 逐案**：T1 3/3 vs pass｜T2 3/3 vs pass｜T3 3/3 vs pass（r1 预算停但目标已达成）｜T4 **1/3 vs fail（同为 unexpected_mutation 故意双写形态）**｜T5 3/3 vs pass｜T6 3/3 vs pass——案例级同构 5/6；A 在 T4 首次出现 pass 重复（P 单样本 fail）。
  - **预算实耗 vs caps**：任务轮 93/216 调用、1,516,912/8,640,000 in、43,643/1,728,000 out（**全在 caps 内**；cache_read 合计 631,168 逐轮在卷）；probe（run3）6 调用/142,110 in；**全程计费合计（含 run2 中止腿 probe 5+T1_r1 5 调用）=109 调用/1,867,735 in/50,189 out，用户 DeepSeek 账户**；cost=missing。
  - **装置重大发现两件（上交主管裁量，源码零改动红线内只取证）**：
    ① **typed plugin_grabber 适配器在 A 普通目录可用**——probe/T1×3/T2r1/T3×3/T4r1-r2 实走 `plugin_grabber_inspect_reverb→apply_reverb_controls`（10/18 轮 write_path=typed_adapter）；四缝（S1-S4）不裁工具目录（305 缝范围），A 数据解读=「无骨架引导下模型自选工具面（含 typed 适配器）」而非「仅裸参数工具」——若需纯裸参数臂须另立 S5 缝卡（裁 plugin_grabber 工具目录）。
    ② **seam_leakage 语料级两轮**：T3_r1（7 个请求 payload 行）+T6_r1（1 行）——`plugin_grabber_explain_controls` 工具结果经 contextruntime 结果渲染（result_summary/result_preview，锚点 `agent/internal/contextruntime/context.go:515-533` collectImportantFields/compactResultPreview）把内核 binding_targets 行（含 alias/normalized_role/template_role，内核源 `VitApp/Source/Service/PluginRackControlService.cpp`）回灌入后续模型上下文——S2 摄取剥离不覆盖此面。其余 **16/18 轮全 100 请求 payload 三字段+alias 键形全零**（25 处 alias 裸词=无关 note 工具描述措辞，已逐条归类）。两轮 verdict 原样保留+device_flags 旗标（seam_verify_corpus.json+arm_a_summary.json），卡面停止条件「五字段入上下文」已触发但发现时 18 轮已跑完（门设计按卡面枚举面=probe 干净即放行；语料复查为收口证据面）——受染轮的 A 臂数据有效性请主管裁定。
  - **真栈登记**：占用 9180ff89（22:23，净空亲核 22:22:39→≤60s 复核 22:23:33 全零后启动）；三 run 同窗（run1/run2 中止取证+run3 终局）；TEARDOWN_ALL_ZERO 脚本亲核 22:42:23+执行侧复核 22:47:11 全零；释放登记随本批 coord/307-close 推 main。kernel=wt_228 F9E3DA0A 只读复用；agent=wt_307s1a 现场构建 CC442375（SHA256 随 run_report）。
  - **端测边界声明**：s0/dryround=确定性腿（零 LLM 遥测零行亲证）；18 任务轮=agent HTTP/ZMQ 真栈腿（缝开 env 产品 chat 普通工具循环）；渲染面（webui DOM）与用户旅程不涉及。单轮=单样本，3 repeat 非稳定性主张（plan §6）。A=「裸参数+最小结构残留（quick_controls/recommended_groups）+typed 工具目录在场」臂（三项口径声明齐）。
  - 工件分支=`port/plugin-control-eval-s1a-1`（a06f5af1 已推）；工件根=`coord/runs/PLUGIN-CONTROL-EVAL-S1A-1/`（RUNBOOK+冻结快照+P 对照+4 run 目录）。A 结果落地即回报主管（B/C 段决策输入；①②两项装置发现先行裁量）。
- 验收：（主管回填）
- 验收（2026-11-07 深夜，主管）：**pass**——18 轮全评估+A/P 对照交付合格；装置验证门亲证（S2/S4 零泄露+模型面双证）；两项装置发现采信并处置：①A 臂工具目录含类型化适配器（10/18 轮自选 typed 工具）=本段 A 实义「无引导自选工具面」——**S5 缝卡（工具目录裁剪）开卡 308 后重测 A-pure**；②contextruntime 渲染面泄露两轮（context.go:515-533）=S2 覆盖缺口——**S2b 并入 308**，受染两轮（T3_r1/T6_r1）分析时标 A* 剔除严格结论。**T4 立体声联动对疑似规则缺陷**（两臂同型「双通道写入」=人类等价行为）——S1-B/C 批次前修冻结规则（联动对预声明为允许集，新批次口径），本轮 verdict 原样保留。A=P=5/6 初步结论待 A-pure 复测后定稿。L1 台账 48h 窗口。

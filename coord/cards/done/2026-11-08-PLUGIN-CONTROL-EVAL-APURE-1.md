# PLUGIN-CONTROL-EVAL-APURE-1：A-pure 终测（真纯裸参数臂 18 轮——「越过控制脚本」正式答案）

- 发卡：GLM 主管决策流 / 2026-11-08 早窗（依据=[307 验收裁定](../done/2026-11-07-PLUGIN-CONTROL-EVAL-S1A-1.md)——A 臂实义=「自选工具面」非纯裸[typed 适配器 10/18 轮自选]+渲染面泄露两轮→S5/S2b 开卡 308；[昨晚 gate 首排](../../reports/2026-11-07-gate-3.md) §七——A-pure=用户核心问题的终测，同时决定开放录音棚适配器形态）
- 派发确认：已确认（主管发卡·分段 go/no-go，307 同模式）；验收负责人：GLM 主管决策流
- 池序 310；目标仓库=主仓 scripts（复用冻结 runner）+工件；P1 / 0.5 天 / **依赖=308 合入 main**；L2 / PC；真栈占用（登记协议照旧）

## 目标

1. **A-pure 18 轮**：六案例 × 3 重复，deepseek-flash 单模型，`VIT_PLUGIN_CONTROL_EVAL_SEAM=1`（S1-S5+S2b 全生效）——模型只见**真纯裸参数面**：裸工具（plugin.get_parameters/set_plugin_param/get_plugin_parameters/通用读写回读）在场，typed 适配器族不在工具目录（S5），渲染面无 binding_targets 语义键（S2b），无语义字段/无 typed 路由/无骨架引导；
2. **agent 二进制=含 308 的现 main 现场构建**（领取时 fetch 重读；缝实现以 main 为准，不改动源码）；
3. **判定口径**：冻结 plan.json（runner_frozen 同源快照）A_star 段 caps 原样（18 rounds/216 calls/per_round_caps 沿用）；T4 联动对形态沿用 307 标注法（verdict 原样、分析层注联动对旗标）——冻结规则修正留 B/C 批次前（307 验收在案裁定，本卡不改判据）；
4. **装置验证门（task 轮前 probe + 语料级 seam_verify 双层）**：
   - S5 亲证：模型工具目录无 plugin_grabber 类型化适配器族+裸工具四件在场；
   - S2b 亲证：渲染面（contextruntime 结果渲染）无 alias/normalized_role/template_role——负锚=307 T3_r1 受染 payload（7 请求行形态）必须检不出同型；
   - **任何缝泄露（typed 工具入目录/语义键入上下文）=装置失效立即停段取证**；write_path 出现 typed_adapter=S5 失效同罪；
5. **回报**：结果落地即回报主管=B/C 段决策输入+开放录音棚适配器形态依据（「越过控制脚本」正式答案）。

## 约束

- 文件域=工件目录+卡面+编排脚本（arm_a_orchestrator.ps1 同模式复刻，307 工件在 `D:/Vit_DAW_wt_307s1a/coord/runs/PLUGIN-CONTROL-EVAL-S1A-1/` 只读参考——runner_frozen 三件+P 对照+编排+seam_verify 均可复用快照）；源码零改动；agent 源码只读；
- 真栈协议全链（净空亲核→登记→≤60s 复核→run→TEARDOWN_ALL_ZERO→释放登记）；工件新目录不覆盖；评价器期望不进模型上下文；栈排队：**领取后候栈——若 296-R2 在栈先等其释放**（登记前重读栈记录）。

## 停止条件

缝泄露（任何形态）→装置失效停段取证；同型环境失败两轮→止损；断言失败取证不重跑。

## 预算

A 段同款：plan.json A_star segment_caps（≤18 任务轮/≤216 调用/in≤8.64M 上界）；§8 跑前成文（307 实耗参考：全程 109 调用/1.87M in，估 ≈¥3-6，用户 deepseek-flash 账户）。

## 领取/回执/验收

- 领取：2026-11-08 晚窗 +0800 / origin/main=00722e1c68190e33b0c2e298bc3981c309c91413（308 已合入亲证） / owner=ZCode GLM-5.3 L2-worker PC 执行子代理（会话 310 apure runner；发卡人已置 doing/ 并派发确认） / 分支=port/plugin-control-eval-apure-1 / worktree=D:/Vit_DAW_wt_310apure / 领取提交=coord/310-claim→main（hash 见 git log）。领取基线：fresh worktree 自 origin/main 00722e1c，`git status --short`=空、`git diff --stat`=空（全净）。在飞卡域核对：296-R3=scripts 断言+真栈腿（wt_296r2，任务指令已让栈）、309=waveshell C++ 域取证腿候栈、273/271 分支未合入不进本树——与本卡（coord/runs/PLUGIN-CONTROL-EVAL-APURE-1/ 工件目录+工件内编排脚本+本卡面）不相交。308 缝在位亲证：HEAD 上 catalog.go pluginControlEvalSeamTrimsCatalogSpec×4+context.go stripBindingTargetsSemanticOverlay×6+两 seam2 测试文件在场；agent 二进制=本 worktree 现场构建（SHA256 随 run_report）。runner 三件=307 runner_frozen 快照逐字复用（blob 核对随 RUNBOOK）。
- 回执（执行侧 2026-11-08 20:00 +0800 收口，mv done/ 候主管终验）：
  - **diff 文件清单**（全在 `port/plugin-control-eval-apure-1`，**源码零改动**——agent/ 与 VitApp/ 与 scripts/ 零触碰；runner 三件=307 runner_frozen 快照逐字复用，入库 blob 核对 plan=259d6b4d/cases=d48ca7cf/driver=57abac09）：`coord/runs/PLUGIN-CONTROL-EVAL-APURE-1/`＝RUNBOOK.md（§8 跑前冻结）+runner_frozen/（三件）+p_baseline/（P 对照 blob 6c4db7c4/1a005f14 与 300/307 逐字一致）+a307_reference/（307 A 成绩表 85c5d329+T3_r1 受染负锚 10 行/7 请求行实录）+seam_verify_apure.py（六面门+selftest 8/8 exit 0 双向检出力）+consolidate_apure.py（三方对照+typed 门化）+arm_apure_orchestrator.ps1（Parser PARSE_OK）+两个 run 目录（191709 首试取证/192029 终局，每 run 新目录不覆盖）。commits：预冻结 1ddca822→收口 a668dbd0。
  - **离线自测**：driver selftest=exit 0（47 项）；seam_verify_apure --selftest=**8/8 exit 0**（洁样本 pass+五种受染形态各自检出+**真 307 T3_r1 负锚当 A 输入=seam_leakage 检出**（S2 182x/196x+S5 目录行 160x）+control_shell 键名形态放行）；consolidate 以 307 数据离线验证（typed 门化触发+三方对照成型）。
  - **栈窗**：296-R3 在跑先候（其 19:03 释放登记在案后进入）；净空亲核 19:10:51 全零→占用登记 94666a3c→≤60s 复核 19:11:39 owner 确认+全零→首试 191709（s0 T3 Waves shell 冷载 JUCE 超时=**307 run1 签名#1 同型** fixture_failure 零 LLM，T1/T2/T4/T5/T6 verified、T4/T5 同族后到即 verified 判冷载时序成立；按 RUNBOOK §8 预冻结重跑一次）→终局 192029→编排 finally 亲拆+执行侧复核 **TEARDOWN_ALL_ZERO 19:41:47**→释放登记 60c823b8。
  - **装置验证门（task 轮前 probe T1_r0，付费 4 调用）＝PASS 六面**：S2 五字段 A-pure 请求 payload 全零（P 对照四字段检出力在）；**S2b 负锚检出力 7/7/7**（307 T3_r1 受染 7 请求行同型全检出=方法对历史真实受染形态有检出力；A-pure 侧 binding_targets 0 行）；S3 生产描述尾串 0；S4 typed 路由首规则 a=0 vs **P=24**；**S5 目录行 `tool=plugin_grabber.` 0 命中+裸串 0（307 同形 5666 命中）+裸读写对在场（probe 2/2 行，语料级 71/71 行=裸面全程存在亲证）**；CS content-form control_shell 0+键名形态 0；S1+typed 执行面零 plugin_grabber/typed 调用+semantic_entry/message_loop 在场。**语料级收口（119 行请求 payload=18 轮+probe 全量）：S2 三语义键全零+S5 目录行 0+CS 0+binding_targets 0+typed 零执行（consolidate device_gate 空单）=零泄露零 typed 全程**；模型面双证 telemetry+capture raw.model 均 **119/119 deepseek-flash 零错误**。
  - **A-pure 18 轮成绩表**（run=arm_apure_20261108_192029；arm=A-pure 重标口径：冻结 driver 原始记录 arm="P" 原样在卷；wp=write_path；first_action 为 None=log 轮转截断证据缺口同 307，verdict 由独立回读产生不受影响）：
    | case rep | verdict | failure_tags | mut | first_action | wp | calls | in_tok | out_tok | cache_r | stop |
    |---|---|---|---|---|---|---|---|---|---|---|
    | T1 r1/r2/r3 | **fail×3** | value_encoding_error×3 | 0 | mix.observe/mix.observe/None | none_obs×3 | 4/5/4 | 59,821/90,511/61,187 | 1,567/1,714/1,395 | 25,600/44,288/25,472 | goal_completed×3 |
    | T2 r1/r2/r3 | **pass×3** | r1,r3=model_budget_exhausted | 1 | plugin.set_parameter×3 | **bare×3** | 10/11/13 | 183,173/232,032/262,470 | 4,807/6,313/8,671 | 81,152/115,712/130,304 | r1=输入超限停(写入已落 pass)/r2=goal_completed/r3=调用数停(写入已落 pass) |
    | T3 r1/r2/r3 | **fail×3** | model_task_failure×3 | 0 | None×3 | none_obs×3 | 5/5/8 | 69,811/72,201/91,114 | 2,025/1,705/2,698 | 33,920/33,920/46,464 | goal_completed×3 |
    | T4 r1/r2/r3 | **fail×3** | value_encoding_error×3 | 0 | None/None/plugin.open | none_obs×3 | 5/4/6 | 53,953/34,017/68,795 | 2,474/1,731/2,229 | 14,976/9,728/23,296 | goal_completed×3（T4 联动对旗标在卷：307 验收裁定沿用，verdict 原样） |
    | T5 r1/r2/r3 | **pass×3**（零 mutation） | — | 0 | None×3 | none_obs×3 | 4/7/4 | 33,364/74,147/30,869 | 1,310/3,585/2,672 | 11,136/28,544/12,288 | goal_completed×3 |
    | T6 r1/r2/r3 | **pass×3**（限制声明+零写） | — | 0 | plugin.semantic_get/None/plugin.semantic_search | none_obs×3 | 8/4/8 | 89,364/31,838/87,526 | 2,738/1,737/5,732 | 30,848/9,728/29,824 | goal_completed×3 |
    - 轮级 **9/18 pass**；案例级 **3/6**（T2/T5/T6）。
  - **A-pure vs A(307) vs P 三方逐案对照（交付核心）**：
    | case | A-pure | A(307)（自选工具面） | P（生产全语义） | 差异解读 |
    |---|---|---|---|---|
    | T1 reverb 写 | **0/3**（wp=none_obs，value_encoding_error，目标 35% 实际 100% 零写入） | 3/3（wp=typed_adapter×3） | pass | typed 适配器是 307 T1 成功的实际载体；纯裸面下模型未完成任何成功写入 |
    | T2 reverb 写 | **3/3**（wp=bare×3，裸写成功唯一案例） | 3/3（typed×1/none×2） | pass | 裸参数路径可达且可完成的案例存在（T2） |
    | T3 reverb 写 | **0/3**（model_task_failure，零写入） | 3/3（wp=typed_adapter×3） | pass | 同 T1 形态 |
    | T4 delay 写 | **0/3**（value_encoding_error） | 1/3 | fail | 三臂最弱案：P 也 fail（unexpected_mutation 纪律形态）；A-pure 连写入都未达成 |
    | T5 零写识别 | **3/3** | 3/3 | pass | 三臂等价（无需写） |
    | T6 零写声明 | **3/3** | 3/3 | pass | 三臂等价（无需写） |
    - 案例级：**A-pure 3/6 vs A(307) 5/6 vs P 5/6**。核心事实：三个需要插件参数写入的案例（T1/T3/T4）中，A-pure 仅 T2 型成功；T1/T3 从 307 typed 3/3 → 裸面 0/3（失败形态=零成功写入+值编码失败/任务未达成），**「模型越过控制脚本直接用裸参数控制插件」在 T1/T3/T4 型任务上不成立，在 T2 型上成立**；写入类能力主要由 typed 适配器承载（307 A 臂 10/18 轮自选 typed 的延续证据）。零写案例（T5/T6）三臂无差——能力差不来自读面。
  - **预算实耗 vs caps**：任务轮 115/216 调用、1,626,193/8,640,000 in、55,103/1,728,000 out（**全在 caps 内**；cache_read 707,200 逐轮在卷）；probe 4 调用/58,165 in/2,372 out（≤12 上界内）；**全程计费合计（终局 run）=119 调用/1,684,358 in/57,475 out/cache 731,264，估 ≈¥2-4（卡面 ¥3-6 上界内），用户 DeepSeek 账户**；首试 191709 零 LLM（telemetry 缺失亲证）；cost=missing（价目未核不折算）。
  - **真栈登记**：占用 94666a3c（19:11，净空亲核 19:10:51→复核 19:11:39）；释放 60c823b8（TEARDOWN_ALL_ZERO 19:41:47 亲核，含编排 finally 拆栈记录）；kernel=wt_228 F9E3DA0A 只读复用（与 300/307/296 同一二进制）；**agent=wt_310apure 现场构建 E581108B…A5F（run_report 记录；与领取时预构建逐位一致=可复现构建亲证；tool_count=256=S5 裁剪后 live 目录面）**。
  - **端测边界声明（AGENTS §5）**：s0/dryround=确定性腿（零 LLM）；18 任务轮=agent HTTP/ZMQ 真栈腿（缝开 env 产品 chat 普通工具循环）；装置门+语料收口=payload/log 取证面。渲染面（webui DOM）与用户旅程不涉及。单轮=单样本，3 repeat 非稳定性主张（plan §6）。A-pure=「真纯裸参数面」（S1-S5+S2b 全开：typed 工具不在目录+渲染面无语义回灌，均真栈亲证）。
  - 工件分支=`port/plugin-control-eval-apure-1`（a668dbd0 已推）；工件根=`coord/runs/PLUGIN-CONTROL-EVAL-APURE-1/`（RUNBOOK+冻结快照+P/307 对照+负锚+两个 run 目录）。**A-pure 结果落地回报主管**：B/C 段决策输入+开放录音棚适配器形态依据（「越过控制脚本」正式答案=上方三方对照表——typed 适配器承载写入类任务能力，纯裸面在 T1/T3/T4 型不成立、T2 型成立）。
- 验收：（主管回填）

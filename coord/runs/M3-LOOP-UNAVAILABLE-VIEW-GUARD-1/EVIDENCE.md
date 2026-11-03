# M3-LOOP-UNAVAILABLE-VIEW-GUARD-1 终局证据包（停止条件第二条触发）

- run 家族：guardrun1（HEAD f0d092fd）/ guardrun2（206b0169）/ guardrun3（9cc871d7），同 fixture 同引擎 deepseek-flash（-LlmModelOverride env 注入，用户 config 零触碰），kernel=wt_228 VitApp.exe SHA256 前缀 F9E3DA0A（235-238 同款只读复用），栈=dev_agent_smoke.ps1 -StartKernel -RestartAgent -AutomationLLMRouteSmoke，run 目录=本目录三轮子目录。
- 证据包构成（卡面停止条件 2 三件套）：§1 载荷亲证（护栏+重定向在场）、§2 收敛面（三轮组 1 verdicts+形态分型）、§3 护栏触发计数。结论=架构面终局上呈（§4）。

## §1 载荷亲证（三件套之一：护栏+披露在场）

### 引擎三面亲证（guardrun3）

- console env 行：driver_console.txt `automation LLM route engine override: VIT_AGENT_LLM_MODEL=deepseek-flash`。
- telemetry：guardrun3 106 行逐行 model=deepseek-flash（guardrun1 175 行 / guardrun2 79 行同）。
- payload：`agent_llm_payload_capture.jsonl` 每行 model 字段=deepseek-flash。

### 护栏（目标 1）真实栈触发亲证

- guardrun1：**21 行载荷携带 `<final_gate>view_set_retry_denied: ...`**（机器可读拒绝全文：status=unavailable / retry_policy=do_not_retry / 规则原文引用 ccb_model_prompt.go:720 / observation_round_consumed=false；经 final_gate trace 面直达模型上下文）。时间线：rows 100-124 denial=6 而新 unavailable 回执=0——同集重试被机械拦截后**不再物化新观察**（拦截前每 25 行 11-19 个 unavailable 回执）。
- guardrun2：denial 5 行（被拒集为纯不可用 track.* 集）。
- guardrun3：denial 5 行**全部携带 `; redirect: processor.* CCB projections do not cover ...`**（执法+重定向同回执；在场标记按 state.executed 尾扫判定）。
- 拦截红线亲证（单测四组+真栈形态一致）：异集/异 target 天然异指纹放行；revision 严格前进放行（guardrun2 期间 project_revision=5 恒定无前进样本，单测覆盖）；拒绝不消耗观察轮（guard 前置于 executeTool，无观察物化）。

### 重定向披露（目标 2）真实载荷亲证

- guardrun1/2 note=0 的两次取证迭代（如实记录）：
  1. guardrun1：note 挂 recentObservation clone 面——模型实际读的三面（model_projection via trace / ccb_observation_summary via executed record / ledger 投影白名单）均不展开该 clone → 零到达。修复 1（206b0169）：note 下沉 raw bundle 层（recentObservationForTool clone 前就地附加，trace/executed/projection 三面同引用到达）。
  2. guardrun2：note 仍 0——取证钉死=模型改用**混合集**（usable 视图+processor 视图，bundle status=partial=usable），bundle 级纯不可用前置不成立；denial 5 行仍在（纯不可用集重试被拦=护栏正确）。修复 2（9cc871d7）：披露与执法解耦——note 触发放宽到逐视图引证（omissions[processor.*]=unavailable 族或 reason 含 source evidence is；预算裁剪不注），拦截维持纯不可用集红线不动。
- guardrun3（fix2 构建）：**45/106 行载荷携带 closed-template note**，与 unavailable 回执 40 行的分布一致（每引证 processor 不可用的回执面均携带）；note 前缀稳定（`processor.* CCB projections do not cover built-in track-level plugins; the authoritative parameter surface ... plugin.get_parameters`）。

### 参数面（对照 237）

- guardrun1 param_surface 35 行 / guardrun3 6 行载荷携带 automation_param_surface 参数行（237 的双层披露延续在场）。

## §2 收敛面（三件套之二：三轮组 1 verdicts+分型）

| run | HEAD | 组1 verdicts | 结果 | 计费（flash） |
|---|---|---|---|---|
| guardrun1 | f0d092fd | no_route, no_route, **boundary_ok**, no_route, no_route | 1/5，group_pass=false | 175 行 in 2,444,356 / out 258,850 |
| guardrun2 | 206b0169 | no_route ×3 | 三连同型止损，组2/3 未跑 | 79 行 in 1,187,180 / out 119,719 |
| guardrun3 | 9cc871d7 | no_route ×3 | 三连同型止损 | 106 行 in 1,580,911 / out 154,798 |

三轮 ps1 均 exit 0（dev agent smoke completed；场景腿 COMPLETED）。

### 形态分型（与 237 对照=质变）

- **237 基线**：对 processor.identity_and_controls 同集反复请求 ~30 次至预算耗尽，零执法，模型不申报边界。
- **guardrun1 r3（boundary_ok，新形态首次出现）**：模型申报能力边界并停止——reply 原文引用 unavailable 回执：「相关观察视图返回『来源证据缺失』……在没有这些标识的情况下无法继续，也不能靠猜测填补。任务在能力边界停止：没有执行任何实验或修改，也不会自动重试。」=骨架规则期望的诚实边界行为。
- **guardrun1 r4/r5 与 guardrun2/3 的 no_route 分型**：非 237 的无限重试——模型决策申报证据缺口（「音量参数的所属插件标识尚未在返回证据中披露」/「缺少……可写数值区间」）后 continuation settled（「任务已在自动续跑队列中」），目标 completed 无写入无能力面。护栏机械阻止同集重试（§1 时间线），模型不再兜圈，但**不走向 set_points 工具链**——持有 param_surface 载荷行仍宣称参数标识/区间缺失。
- 组 2（仅 guardrun1 跑到）：[no_route, confirm_broken×3] 止损，stop_reason=plugin_selection_required（Pro-R 推荐后停在插件选择确认面，非本卡标的）。

## §3 护栏触发计数（三件套之三）

- guardrun1 denial=21 行载荷（时间线分布：4/4/3/4/6/0/0 每 25 行块；组 2/3 块零触发=形态不同）。
- guardrun2 denial=5；guardrun3 denial=5（全带 redirect）。
- 触发即拦截：denial 后同 fingerprint 不再物化新 unavailable 回执（guardrun1 rows 100-124 实证）；观察预算不被拒绝消耗（无新 observation_id/receipt 产生）。

## §4 终局结论（卡面停止条件第二条，转用户裁定）

护栏（机械执法+预算不消耗）与重定向披露（closed-template note+denial 同回执）三轮载荷亲证在场，flash（用户裁定可用最强引擎）下组 1 仍 no_route 三连同型止损（guardrun2/3），且形态已质变于 237（无限重试→被拦+偶发诚实边界申报+settled 停止）——**确定性执法/披露层已收口，剩余残差=模型在持有参数面载荷行的情况下不走向 set_points 工具链（宣称标识/区间缺失后 settled）**。此为卡面预埋的终局分叉：接更强引擎 vs 架构重构 vs 收线接受边界申报形态，三选归用户裁定，不再折卡。

## §8 计费披露（flash ¥1/M in、¥2/M out 上界口径）

- 三轮合计：360 计费行，in 5,212,447 / out 533,367 ≈ **¥6.3 上界**（含 cache_read 部分按全价上界计；实际低于此值）。
- 超卡面预估（≈237 量级 ¥1.5 档）原因如实披露：三轮取证迭代（guardrun1 挂点错面取证+guardrun2 触发面收窄取证+guardrun3 亲证轮），单轮均 ≈237 量级，三轮合计为修复面的诚实成本。
- 单 run 存在性判别（§8）：每轮独立止损，无原样重跑（三轮 HEAD 两两不同，每轮唯一 diff=披露面修复）。

## 工件精简说明

入库（port/m3-loop-unavailable-view-guard-1）：三轮 driver_console/automation_llm_route_console/ps1_exit/telemetry/payload_capture/default_project.xml/gui_project_isolation.json/automation_llm_route_project.vit + guardrun1 summary（boundary_ok 轮判读核心）+ guardrun2/3 summary+rounds + 本 EVIDENCE.md。不入库留 worktree（验收期在卷）：三轮 agent_prompt_render.jsonl（12M 渲染面与 payload 重复度高）+ guardrun1 rounds（9.5M，verdict 数据已由 summary/EVIDENCE 承载）+ 运行态目录 .vit_agent/.vit_history（非证据）；clip.wav 由既有 *.wav 规则排除（237 先例）。

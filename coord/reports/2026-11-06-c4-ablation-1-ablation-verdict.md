# C4-ABLATION-1 判定面报告：C4 首拆 ablation——A 态判定输入不足，维持 H 态（缺口=semantic_entry 准入面）

- 卡：C4-ABLATION-1（池序 263，A-F 主线首拆）｜run：`abrun1`（automation_llm_route_20261106_104227，2026-11-06 10:42:41–10:56:29 +0800）
- 被测：`port/c4-ablation-1` @ fe368796（+RUNBOOK e9807664）｜kernel=wt_228 `F9E3DA0A…67A` 同款只读复用｜引擎 deepseek-flash（-LlmModelOverride，238-253 先例）
- 判据（RUNBOOK 跑前成文）：success={tool_chain_ok, boundary_ok}，每组 ≥4/5；lexicon_hit=钩未达=配置失败；钩生效三重验证

## 一、判定结果

| 组 | 话术 | 样本 | verdicts | successes | group_pass |
|---|---|---|---|---|---|
| 1（回归面） | 这段让它越来越突出一点 | 5 | tool_chain_ok×4 + no_route×1 | **4** | **true（≥4/5）** |
| 3（ablation 目标面） | 30 秒之后动起来 | 3（3 连同型止损，4-5 轮按止损未花） | no_route×3 | **0** | **false** |

**结论：不满足「≥4/5×两组」→ C4 不判 A，维持 H 态（A 态节点数仍=0）。** 缺口清单见 §三，如实转卡不硬修。

## 二、机制钉死（组 3 三轮同型，证据链完整）

三轮逐字同型：`stop_reason=semantic_entry_unresolved`、`workflow=semantic_entry`、reply=确定性模板「我没太明白你的意思——是想继续刚才的确认…」（`semantic_entry.go:395`）——**251 形态复现**（251：12/12 unresolved conf 0.18-0.35；本 run：3/3 unresolved）。

- r1 conf=0.22 rejection="Insufficient context to determine the requested DAW interaction or target."
- r2 conf=0.22 rejection="Too ambiguous to determine whether this is playback, automation, acoustic change…no target is identified."
- r3 conf=0.32 rejection="'30 秒之后动起来' lacks a clear DAW action, target, or intended outcome."

链路：钩拆除 253 词表 pre-model 武装 → 裸时间窗话术落到 semantic_entry 仲裁器（LLM 分类面）→ 裸「动起来」无明确声学目标词，conf 过不了 0.50 下限（`semanticEntryMinimumRouteConfidence`）→ unresolved → 确定性模板作答 → message loop / free-state reasoning **零进入** → 工具面不可达 → no_route。

## 三、缺口清单（转卡输入，交主管裁定）

1. **缺口不在 harness 层**——组 1 同栈同轮 4/5 tool_chain_ok 亲证：time-varying action path guidance（v2+write-gate+terminal 三选一）、automation 工具面（get_plugin_parameters→set_points 确认链→get_curve 读回）在词表拆除后完整工作；payload needle 三面全 available（106/106 行）。
2. **缺口在入口准入层**：semantic_entry 仲裁器对「裸时间窗+泛化动词」话术族的 conf 下限（0.50）不可达。该面本卡禁改（卡面「禁：…semantic_entry」），词表（253）此前正是以 pre-model 武装短路了这一层。
3. 处置选项（主管裁量）：
   - ① **仲裁器判据面扩粒度卡**：semantic_entry 判据/路由定义对时间窗话术族的准入（如 open_semantic 判据补充「时间性变化请求」语义）——注意 2026-10-30 判断权归属裁定与 CONFIRM-CHAT-1 措辞护栏的边界；
   - ② **词表重新归类为「入口识别薄层」**：按终局裁定 §二-2「薄固定层=识别/执行/兜底」口径，把 253 词表定位从「能力实现」降为「入口识别」，C4 维持 H 态、词表作为薄层保留（不拆但性质变了）；
   - ③ **组 3 话术族判据重审**：裸「30 秒之后动起来」是否本就属 underspecified（用户意图确需澄清）——仲裁器 unresolved→指引性模板是诚实边界而非缺陷，若此裁定成立则组 3 判据应改用带目标词变体话术重跑。
4. 残差对照：组 1 r4 no_route=limit_reached 续跑未终局形态（stop=limit_reached「我还在继续处理」），与 253 r1 同族模型行为样本，组判据内不构成红线。

## 四、钩生效验证（三重，全过）

1. ps1 console：`automation LLM route ablation hook: VIT_AUTOMATION_INTENT_ABLATION=1`；
2. py summary：`ablation_hook=true`；
3. 8 轮 verdict 全集 {tool_chain_ok:4, no_route:4}，**零 lexicon_hit**（有则 py exit 4 INVALID）——钩确实到达运行中 agent，组 3 的 LLM 化路径是真实开启的（三轮各消耗一次仲裁器 LLM 分类调用）。

## 五、零栈验收（开关默认开=生产行为不变）

- `go build ./...` exit 0；`go test ./internal/chat -count=1` 全绿（124.9s 存量零回退）；`./internal/agentloop` 全绿；automation/clarify 聚焦全绿
- 新增配对单测 `TestAutomationIntentAblationPair`：默认 unset=武装接管行为不变（workflow=section_automation_choreo+pending 态落存）/ env=1=入口直通 LLM 零残留态 / 仅精确 "1" 生效（"0"/"true"/"off" 均不生效）
- py 离线测试 11/11（252 三态面零回退）；生产面 20 行 diff（代码 4 行：entry 一处 OR+const+helper）

## 六、§8 运行记录

- 单 run 无重跑兑现：组 3 止损后 4-5 轮未花（止损线设计内）；无环境中断（env_interrupt=null）；exit 0（COMPLETED）
- 计费：106 调用全 deepseek-flash，in 1,486,923 / out 114,145 ≈ **¥1.8-2.4**（换算基准=253/225 回执口径），卡面预算 ≈¥2-4 内
- choreo-pending 腿=skipped_ablation_hook×2（词汇 arm 被钩禁用，结构 n/a 如实记录）
- 拆栈：owner 亲拆 VitAgent 46848/VitApp 42628，**TEARDOWN_ALL_ZERO 亲核 10:59:56**（五端口 7878/5555/5556/8787/18787=0+三类进程=0+Godot/python=0+UDP 4444/4445=0）
- 工件：`coord/runs/C4-ABLATION-1/abrun1/`（RUNBOOK+summary_slim+telemetry 164KB+py console+dev console+g3_r1-3 全件+g1_r4 slim）；大件留 worktree `automation_llm_route_20261106_104227/logs/`（payload 6.5MB+prompt_render 3.7MB+summary 全量 3.0MB+rounds 2.9MB）

## 七、文件域偏差声明（交主管裁量）

卡面文件域=「automation_intent 开关缝（≤20 行生产面+测试）+报告」。本卡为执行 ablation run 增加了**冒测脚本参数**（`automation_llm_route_smoke.py` `-AblationHook` + `dev_agent_smoke.ps1` `-LlmRouteAblationHook`，合计 56 行，默认关=行为字节不变）：直接原因=脚本既有 choreo-pending 腿在钩模式下 `require(looks_lexicon_hit(arm))` 必失败（词汇 arm 被钩禁用是设计本身），不改则 run 在每组末轮前 ERROR 中止且组 3 永不可达。依据=AGENTS §5「新测试场景以扩展现有冒测脚本（加参数）」+卡 243 同脚本 `-Groups` 加参先例；257 矩阵 §六-2「不改脚本」的意图（栈生命周期复用）已遵守——栈启停/项目隔离/telemetry 面全部复用既有体系零改动。此偏差是否追认，请主管裁定。

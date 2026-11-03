# M3-CURRENT-VALUE-DISCLOSE-1 真栈重跑判读（disclose1）

- run：disclose1 单 run（HEAD=13a57caf，唯一 diff=本卡目标 1-4），同 fixture 同引擎 **deepseek-flash**（-LlmModelOverride env 注入，用户 config 零触碰），kernel=wt_228 VitApp.exe SHA256 前缀 **F9E3DA0A**（235-241 同款只读复用，本次哈希重核一致），栈=dev_agent_smoke.ps1 -StartKernel -RestartAgent -AutomationLLMRouteSmoke，ps1 exit 0（dev agent smoke completed）+场景腿 status=**COMPLETED**（三组全跑满）。
- 引擎三面亲证：driver_console env 行 `VIT_AGENT_LLM_MODEL=deepseek-flash`；telemetry 278/278 行 model=deepseek-flash；payload_capture 278/278 行 model=deepseek-flash。

## 组 1 判据：门未过（0/5 成功）——但**本卡命题三重亲证成立+新真断点成立**（「判定成功=组 1 绿或新真断点」第二形态）

| 样本 | verdict | 形态 |
|---|---|---|
| g1_r1 | no_route | 宣告写入计划+「提交后会在确认卡里等你确认」 |
| g1_r2 | no_route | 宣告渐强包络计划（**用当前值规划「起点略低于当前音量」**）+「写入前会弹出确认卡」 |
| g1_r3 | capability_blocked | 诚实申报：持有范围（0-1）+**当前值（约 0.74）**+曲线无点；申报缺乐段时间范围+轨道级声学证据+「本轮是最后一个回合不能再发起新的观察」 |
| g1_r4 | no_route | goal 失败结算（「任务在形成有效结算前失败」） |
| g1_r5 | no_route | 宣告完整计划（0-6 秒上扬曲线）+「**你确认后即可试听**」 |

- 组 2：confirm_broken×3（plugin_selection_required——外部插件装载候选征询，confirm phrase 不 settle）+no_route×2，非本卡域（组 2 phrase「帮我把空间铺开」的预期确认流测试面）。
- 组 3：no_route×3 止损（reply 均为「我没太明白你的意思——是想继续刚才的确认，还是提个新要求？」——phrase「30 秒之后动起来」语义歧义形态，记录非本卡域）。

## 本卡命题三重亲证（value 在场+被消费）

1. **compact 行 value 实测在场**：payload 中 `automation_param_surface` 行携带 `value=0.740818202495575, value_text="+0.00 dB"`（volume）与 `value=0, value_text="Centre"`（pan——**数值零=真实值**透传，本卡单测边界在真栈出现）；既有键（param_id/label/min/max/normalized_role/supports_automation）全部在场=红线守住。测量面 `rows_with_param_value`=**26/26**（26 行 param_surface 全带 value）。
2. **册行 value 实测在场**：ledger JSON `automation_surfaces` params 同值携带；测量面 `rows_with_pinned_value`=**44/44**（44 行册行全带 value/value_text）。
3. **value 被模型消费逐字对照**：g1_r3 申报原文「这条轨道音量参数的取值范围（0 到 1，**当前约 0.74**，曲线上没有任何自动化点）」——0.74 ↔ 实测 0.740818 吻合；g1_r2 更进一步用当前值做包络规划（「起点略低于当前音量」）。241 的「缺当前值」申报形态**消失**。

## 新真断点定性（供主管裁定，不自修不开卡）

- **主形态（组 1 五样本统一）**：模型持有全部写前要素（范围+当前值+时间窗 r1/r2/r5 均 0-6 秒+完整写入计划）但**停在 plain-text 征询确认**——r1「提交后会在确认卡里等你确认」/r2「写入前会弹出确认卡」/r5「你确认后即可试听」，而 agent 面 `needs_confirmation=False`（无 pending candidate 形成）。断点候选=free-state 写入的确认流接线/免确认直写指引缺格：模型一致预期「确认卡」，确认卡不出现也不直接调 set_points。
- **写入链路畅通排除基础设施断点**：choreo settle 腿 set_points 真实写入（Track 1010 volume，区间 0-30s 渐入 0.30→0.60，2 点逐点读回验证通过容差 1e-06，曲线留存未回退）。
- 次形态（g1_r3）：回合预算结构（「本轮是最后一个回合，不能再发起新的观察」）+时间范围/轨道级声学证据披露缺口申报。
- 工具链走向持续：r3 调 automation.get_curve（同 241），plugin.get_parameters 在全部 13 个 LLM 样本的 tools_called 中。

## 零回退亲证

- 96 包 go build+test exit 0（实现提交前）；240/241 全部既有测试原语义绿（pin 9 测+新增 1 测+240 八测，见实现回执）。
- base note（`processor.* CCB projections do not cover` 前缀）零字节改动（diff 亲证无 freeStateCCBProcessorRedirectBaseNote 触碰）；compact 行既有键集字节不变（harness.go diff 仅键列表追加+注释）。

## §8 计费披露（flash ¥1/M in、¥2/M out 上界口径）

- 单 run **278 计费调用**（跑满三组：组 1 五样本+组 2 五样本+组 3 三样本止损+choreo 腿+chat 类腿），input 4,019,085（含 cache_read 2,606,080）/output 461,552 ≈ **¥4.94 上界**（cache 按全价上界计，实际低于此值）。
- **超卡面预算如实披露**：卡面预算 1-2 轮 ≈¥1.5-3 按组 1 止损停的先例口径；本次组 1 形态混合（capability_blocked 打断 no_route 连击）未触发组内止损，脚本既定逻辑自动跑满三组（stop-loss/overflow ends this group only——组间推进非止损即继续），计费随之升至三组量级。无重跑（单 run 收口）。

## 工件清单

- 入库（随 port/m3-current-value-disclose-1）：driver_console/automation_llm_route_console/summary（11.5M 含 exchanges 全文）/project.vit/default_project.xml/gui_project_isolation.json/ps1_exit=0/telemetry（278 行）/payload_capture（278 行 18M）/rounds（g1_r1-r5+g2_r1-r5+g3_r1-r3+choreo_pending_g1/g2+setup_status）/本 RECEIPT.md。
- 不入库留 worktree：agent_prompt_render_local_only.jsonl（9.1M，与 payload 重复度高，241 先例）；.vit_agent/.vit_derived/.vit_history 运行态目录+clip.wav 已删（240/241 教训条目）。

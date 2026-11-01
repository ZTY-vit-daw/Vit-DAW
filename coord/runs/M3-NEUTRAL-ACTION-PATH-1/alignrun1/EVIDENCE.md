# M3-NEUTRAL-ACTION-PATH-1（235）真栈重验腿证据包——alignrun1

- run：`coord/runs/M3-NEUTRAL-ACTION-PATH-1/alignrun1`（worktree `D:/Vit_DAW_wt_235impl` @ `port/m3-neutral-action-path-1`，实现 e9a51a06）
- 命令：`powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt_235impl/scripts/dev_agent_smoke.ps1 -RepoRoot D:/Vit_DAW_wt_235impl -RunArtifactsDir <本目录> -StartKernel -RestartAgent -KernelExe D:/Vit_DAW_wt_228/.../VitApp.exe -AutomationLLMRouteSmoke` → **exit 0**（COMPLETED）
- 栈：VitApp f9e3da0a（wt_228 只读复用，C++ 面零改动）+ 本 worktree 构建 VitAgent（含本卡教学段+载荷捕获）；引擎=deepseek-flash（同 234，同码同 fixture 同引擎三同成立，唯一 diff=本卡目标 1/2）
- kernel 运行前后零漂移；占用登记 c4772899（登记前+启动前双复核进程/端口全零）；拆栈 TEARDOWN_ALL_ZERO（VitAgent 28944/VitApp 39256 停，TCP 5555/5556/7878/8787/18787/5178=0，UDP 4444/4445=0）

## 三件套

### 1. 实收载荷（payload capture 亲证）

- 文件：`logs/agent_llm_payload_capture.jsonl`（108 行=108 次 LLM 调用全量 request messages+response text/raw，与 telemetry 1:1）
- **新指引在场亲证**：108 行中 **102 行**的 request messages 含教学段 needle `"Action Path Selection (time-varying goals vs static adjustments)"`（summary `payload_guidance.rows_with_guidance=102/108`；6 行不含=非中性族面的前置/setup 调用）
- 教学段物理位置=user 消息 directives 块（`message_loop_neutral_family_directives` Section，234 证据的模型实际服从显著层）

### 2. 收敛数据（234 收敛测量面延续）

| 轮 | verdict（smoke 分型） | LLM calls | turns | obs_req | proposal_reached | limit_seen | probe view |
|---|---|---|---|---|---|---|---|
| g1_r1 | capability_blocked | 8 | 7 | 5 | false | false | present |
| g1_r2 | no_route | 41 | 39 | 20 | false | true | present |
| g1_r3 | no_route | 32 | 31 | 14 | false | true | present |
| g1_r4 | no_route | 26 | 25 | 0 | false | true | present |

- 引擎计费：deepseek-flash 108 调用，1,631,513 in / 163,560 out
- 组 1：`[capability_blocked, no_route, no_route, no_route]`，successes=0，**r2-r4 三连 no_route 止损**（组 2/3 skipped: stop_loss_in_earlier_group）
- probe view present 4/4（证据面就绪，rms_dbfs=-11.889 与 231/234 一致）

### 3. 形态分型（模型行为面，payload 亲证逐轮）

**r1（8 calls 干净路径）**：模型实收教学段后**正确判别时变语义**（申报原文："属于随时间变化的自动化包络需求"）→ catalog → project.structure+track.* 定位目标轨 → 试图解析参数身份（CCB `processor.identity_and_controls`×2，返回不可用）→ 拒猜（"参数标识不允许凭猜测填写"）→ capability_blocked。

**r2-r4（41/32/26 calls）**：均**调用了 `plugin.get_parameters {"track_id":"1010"}`**（reason 原文："读取该轨内置音量插件的参数标识与取值范围，用于后续写渐强包络"——教学路径第二步被完整理解并执行，234 从未出现的新行为）→ 工具返回 error：**`"plugin.get_parameters could not resolve plugin target: multiple visible plugins match; select one plugin or specify plugin_id"`**（harness.go:11752）→ **错误不列出候选清单**，模型无从选择 → 转从 CCB 观察面找参数身份（r1 同型兜圈）→ 片预算 limit_reached → 轮内终态 capability_blocked（chat 结算面 stop_reason='' → smoke 分型 no_route）。

**fixture 事实（kernel get_project_state 只读亲证）**：每轨两个内建插件——track 1010 = `1011 Volume & Pan Plugin` + `1012 Level Meter` → `visiblePluginRefs`（harness.go:11986）返回 2 项 → track 级解析必然 multiple 歧义。py setup 的 `find_track_volume_plugin`（按 volume 参数过滤）不受影响，故 234 前置链路从未暴露该缺口。

## 分型定性（执行侧如实分列，裁定归主管）

1. **停止条件触发成立**：新指引实收亲证在场（102/108）+ 组 1 三连同型止损 → 按卡面，指引层定性推翻证据包上交，不开第三张指引卡。
2. **同时新证据改变变量分布**：失败已非 234 的"模型不知道路径"形态——模型行为面四件全对（时变判别✓/路径分步执行✓/拒猜纪律✓/诚实申报边界✓）；卡点是**确定性的工具解析缺口**：`plugin.get_parameters` track 级解析在 fixture 双内建插件下必然报 multiple 歧义，且 error 文本不带候选 plugin_id 清单——任何引擎都无法从该 error 恢复（换引擎实验会复现同一卡点；可测的只剩"模型能否从未列候选的 error 中恢复选择"，先验近零）。
3. 主管选项面（供裁定，不预设）：A=工具面修复卡（error 附候选清单或 Level Meter 不入 automation 面可见集——均在 harness.go，本卡文件域外）；B=换引擎实验照跑（判别门证据完整性优先）；C=两者并行。执行侧证据倾向 A 先行（确定性卡点先拆，换引擎的边际信息量在卡点拆除前被吞）。

## 工件清单

- `logs/agent_llm_payload_capture.jsonl`（7.4M，108 调用全载荷——实收载荷三件套之一）
- `logs/agent_llm_telemetry.jsonl`（188K，元数据）+ `logs/agent_prompt_render.jsonl`（4.2M，render/directives 取证）
- `automation_llm_route_summary.json`（2.1M 含 exchanges）+ `rounds/`（g1_r1-r4 + setup 探针）+ 双 console
- `automation_llm_route_clip.wav` 排除入库（确定性可再生，write_deterministic_stereo_wav）

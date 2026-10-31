# 234 M3-CONFIRM-DRIVER-ALIGN-1 真栈重验腿证据（alignrun1+alignrun2）

run：coord/runs/M3-CONFIRM-DRIVER-ALIGN-1/{alignrun1,alignrun2}（2026-10-31 22:55–23:55 UTC+8）
被测=wt_234impl @ port/m3-confirm-driver-align-1：alignrun1 用实现 commit **13f69c28**（单短语驱动），alignrun2 用叠卡驱动 commit **65ecba63**；kernel=wt_228 VitApp.exe sha256 f9e3da0a 只读复用（运行前后零漂移）；VitAgent.exe sha256 1167f56cfe4449ac 本会话自建（无 Go 改动，alignrun2 -SkipBuild §9 口径）。
命令（两 run 同型）：
```
powershell -NoProfile -ExecutionPolicy Bypass -File D:\Vit_DAW_wt_234impl\scripts\dev_agent_smoke.ps1 -RepoRoot D:\Vit_DAW_wt_234impl -RunArtifactsDir <…>\alignrun{1,2} -StartKernel -RestartAgent [-KernelExe D:\Vit_DAW_wt_228\...\VitApp.exe] -AutomationLLMRouteSmoke   # alignrun1
powershell … -RunArtifactsDir <…>\alignrun2 -RestartAgent -SkipBuild -AutomationLLMRouteSmoke   # alignrun2（kernel 38132 原栈复用，同 owner 延续登记 fbd8284e）
```
两 run **ps1 exit 0**（dev agent smoke completed，AutomationLLMRouteSmoke COMPLETED，prompt_guidance_rendered=true）。栈登记/释放见 PC-RUNTIME-STACK.md（占用 90ea7bb1→延续 fbd8284e→释放 63e6fb79，TEARDOWN_ALL_ZERO）。

## A. 链路进度（三里程碑，全部亲证在卷）

1. **「确认提案」在 improvement_proposal 卡路由成功——233 wire2 confirm_broken 断点修通**：
   - alignrun1 g1_r2 ex1：驱动「确认提案」→ stop_reason=`improvement_proposal_native_tool_confirmation_required`、workflow=`mix_tick`、reply「改善性提案已进入原生控制工具链。精确混音单步仍需确认」——确认文本**首次路由到 pending interaction 的 settle**（233 wire2 同位驱动「确认写入」×3 全落纯文本轮）。
   - alignrun2 g1_r4 ex1 复现同型（两 run、两个脚本版本、两次独立到达，非偶然单样本）。
2. **勘察发现：确认链叠两层卡**（卡面「分卡种各用其卡面词」条款的直接延伸）：
   - settle improvement_proposal 卡后**叠出第二层 `mix_tick_confirmation` 卡**（按钮「确认执行」，acceptText 精确表含之）；「确认提案」在其词表外→`ambiguous_mix_tick_confirmation` 持卡追问「要执行请明确说"可以执行"」（alignrun1 g1_r2 ex2/ex3 全链在卷）。
   - 修复（65ecba63）：`confirm_phrase_for` 严格面分词——mix_tick→「确认执行」、improvement_proposal→「确认提案」、其余确认面→裸「确认」（唯一过全部 actionworkflow accept 表的通用词）；判定逻辑零改动（仅驱动短语选择）。
3. **确认链端到端首次全绿（alignrun2 g1_r4 ex0→ex2）**：
   ```
   ex0 短语 → improvement_proposal 确认卡（G1-G8 过+提案接受）
   ex1 「确认提案」 → mix_tick 卡叠出（已进入原生控制工具链）
   ex2 「确认执行」 → 写入+回读验证：「已应用并回读验证：Track 1010音量 +1 dB（回读 1.5 dB → 2.5 dB）」
   ```
   = **229 五卡链以来第一次：确认卡→确认→写入→读回 全链走通**（治理面零违规：确认前零写入，写入后回读在卷）。
4. choreo-pending 腿：`「确认写入」` settle **pending_intact**（points=2，alignrun1）——choreo 卡词表与 improvement 词表分卡种驱动互不干扰。

## B. 最终验收（229 口径第六承）——未达，分项如实

| 承项 | 结果 | 证据 |
|---|---|---|
| 组 1 no_route 消失 | **未达** | alignrun1 g1=[no_route×4, confirm_broken×1]、alignrun2 g1=[capability_blocked, no_route×3]，两 run 均止损于 3 连 no_route |
| set_points 确认链端到端首绿 | **部分**：确认链（卡→确认→写入→读回）首绿 ✔；但写入形态=**静态 mix tick**（+1 dB），非 automation set_points 曲线 | alignrun2 g1_r4 ex2；两组到达提案的轮（alignrun1 g1_r2、alignrun2 g1_r4）模型均选择静态小幅调整而非时变包络 |
| 组 2/3 跑满 | **未跑** | 两 run 组 1 止损连带跳过（smoke 既有止损语义：组内 3 连同型失败→终止全 run） |

- 验收门判定：**未达**。组级 0/5、0/4（<4/5）→按卡面停止条件转 §D 模型能力面终局证据包（本卡核心交付，非失败）。
- §8 止损纪律：组 1 止损形态（观察循环不收敛→3 连 no_route）在同一确定性断点连续出现两次（两 run），**禁止原样重跑**——不再开第三 run 碰运气。

## C. 收敛测量面数据（目标 3 交付，逐轮全量）

| run/轮 | verdict | LLM 调用 | in tok | out tok | max turn | 观察请求 | 提案到达 | limit_reached | round 后探针 |
|---|---|---|---|---|---|---|---|---|---|
| a1 g1_r1 | no_route | 36 | 535,616 | 46,566 | 35 | 18 | ✗ | ✓ | view=ready |
| a1 g1_r2 | confirm_broken | 17 | 250,925 | 25,670 | 16 | 8 | ✓ | ✗ | view=ready |
| a1 g1_r3 | no_route | 21 | 314,523 | 27,827 | 20 | 9 | ✗ | ✓ | view=ready |
| a1 g1_r4 | no_route | 43 | 621,947 | 45,738 | 42 | **0** | ✗ | ✓ | view=ready |
| a1 g1_r5 | no_route | 17 | 253,207 | 32,408 | 16 | 0 | ✗ | ✓ | view=ready |
| a2 g1_r1 | capability_blocked | 31 | 470,427 | 44,088 | 30 | 27 | ✗ | ✗ | view=ready |
| a2 g1_r2 | no_route | 25 | 368,239 | 29,264 | 24 | 5 | ✗ | ✓ | view=ready |
| a2 g1_r3 | no_route | 34 | 496,156 | 46,320 | 33 | 17 | ✗ | ✓ | view=ready |
| a2 g1_r4 | no_route | 27 | 422,439 | 50,562 | 18 | 0 | ✓ | ✗ | view=ready |

**判别结论（卡面目标 3 的核心判据）**：9 个观察耗尽/不收敛轮的 round 后同面确定性探针 **9/9 view_present=true 且返回 ready 事实**（track.basic_energy：rms_dbfs=-11.889、peak_dbfs=-4.188、status=ready，与 231 identity 绑定值一致）——观察面**有 ready 事实可读**而不收敛到提案 ⇒ **模型行为面**，非证据面缺失。证据面在全部轮次均备好事实。

不收敛形态分型（终局分布：no_route×7、capability_blocked×1、confirm_broken×1，9 LLM 采样轮）：
- **观察重复型**（obs=5-27，6 轮）：模型反复 ccb.observation_request 同批视图，预算耗尽 limit_reached→continuation settle goal completed 无写入无提案；
- **中性族选择循环型**（obs=0、turns 16-42，3 轮）：零工具调用、纯 message_loop_neutral_family_selection 轮次空转至预算尽（a1 g1_r4 42 turns 为最重样本）；
- **提案到达型**（2 轮）：到达提案但选择静态 mix tick 而非 set_points 包络（见 §B）；
- **能力申报型**（1 轮）：capability_blocked 诚实申报（非门拒绝，232/233 修复后形态）。

## D. 模型能力面终局证据包（<4/5 停止条件交付，转主管决策）

1. **五卡证据链**（229→234 逐环修复后的剩余形态）：230 证据 fixture ✓→231 identity 绑定 ✓→232 预算放开 ✓→233 G6/G8 接线 ✓（admission gate 零拒绝）→234 确认短语+叠卡驱动 ✓——**生产链路五环全部修通后，剩余 0/5 与 0/4 组级由模型行为面独占**：证据在、门开、卡通、写入验证，模型不在预算内收敛到提案（或收敛到静态步而非指引的 set_points 链）。
2. **逐轮收敛数据**：§C 表（turn 消耗 16-42、观察 0-27、单轮 LLM 17-43 调用/250K-620K in）。
3. **观察返回内容判别**：9/9 探针 ready 事实 ⇒ 模型行为面（§C）。
4. **待主管裁定的方向**（发卡时预埋：决策轮换引擎 or 指引效率 nudge）：观察重复型+中性族空转型的共同特征=轮次耗在"再确认一次证据/族选择"而非推进到提案；静态选择型=指引（get_plugin_parameters→set_points）未压过改善机器的静态小幅步偏好。三型都指向模型/指引面，不再折生产修复卡。

## E. §8 计费披露（deepseek-flash，真实计费）

- alignrun1：135 调用，1,976,924 in / 179,201 out
- alignrun2：118 调用，1,757,967 in / 170,416 out
- **合计：253 调用，3,734,891 in / 349,617 out**（0 env_interrupt、0 canceled）

## F. 诚实边界与工件口径

- round 后观察探针=同面确定性状态读（`ccb.observation_request` /agent/invoke，零 LLM 成本），**非模型自身逐请求返回体的回放**（该返回体不落任何工件面）——判别依据是"观察面当时确有 ready 事实可读"，对"模型读到了什么"是必要非充分推断；
- alignrun2 g1_r4 ex3：D1 事后评估面（`d1_post_action_evaluation_required`）收到 smoke 既有 REGION_FOLLOWUP（「0 到 30 秒」）→ goal failed（minimal_audio_closure「任务在形成有效结算前失败」）→该轮 no_route——smoke 追问启发式与新阶段面的交互，**按判定逻辑零改动红线未动**，形态如实入卷；
- 环境瞬态：alignrun2 默认腿 strip.chat 一次 `durable_checkpoint_persist_failed`（kernel 复用调度器锁残留，非本腿面，LLM 路腿未受影响）；alignrun1 setup acoustic excerpt=['partial'×3] 但 track 级视图返回 ready 事实（以视图事实为准）；
- 工件入库（port 分支）：rounds/、logs/（telemetry+prompt_render）、summary、console、.vit、.vit_derived 状态件、ps1 console；**排除** .vit_agent/（49M+52M 状态树）与确定性 clip wav（可再生，§10 防污染口径）；
- 组 2/3 未采样（两 run 组 1 止损连带跳过）——其跑满需求随 §D 证据包一并归主管处置，不在本卡补跑（§8 两连止损禁原样重跑）。

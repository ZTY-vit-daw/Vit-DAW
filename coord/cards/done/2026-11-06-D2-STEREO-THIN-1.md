# D2-STEREO-THIN-1：D2 全局立体声关系薄层落地（判据面先行）

- 发卡：GLM 主管决策流 / 2026-11-06 晚窗（依据=[终局裁定](../../decisions/2026-11-05-af-scaffolding-endstate.md) §二-3 薄固定层原则+§三 D/E/F 薄层方案+[257 矩阵](../../reports/2026-11-05-af-family-health.md) §五 C5/D2-D4 not_built 行+MIXLAYER-M1-4 已建观察面（e0e2f4ac：pre_master.observation.v0 含 D2 全局 stereo_relation+mix.read 逐轨 stereo summary，mono 风险阈值 corr<0.25||neg>0.1 已入库））
- 派发确认：已确认（主管发卡；gate §五-2 预定产物，判据面先行、指引面等校准）
- 验收负责人：GLM 主管决策流
- 池序 265；目标仓库=主仓 agent；P2 / 1 天 / 依赖=M1-4 观察面已在 main；L1-L2 / PC 执行会话；验证腿需真栈一次

## 背景

D2=全局立体声关系（not_built，capability IDs=nil）：观察面已由 MIXLAYER-M1-4 建成（只读 pack 三面之一），runtime 编排未建。D1 已点亮（254 实现+258 修复+runner 面真栈亲证）——D2 runtime **复用 D1 模式**（`agent/internal/chat/d1_loudness_settle_runtime.go` 的判据→verdict 落 store→建议挂 mix_tick 确认链卡零裸写→queue view 点亮）。

## 目标

1. **判据+验证器（薄层核心，零 LLM 单测先行）**：全局 stereo_relation 观察→mono 风险轨识别（阈值复用 M1-4 corr<0.25||neg>0.1，纯函数）→verdict 三态（risk_found/no_risk/unmeasurable 同 LW verdict 模式：行缺失/无生产者=unmeasurable 一等公民）；
2. **薄 runtime**：D2 话术→判据消费→verdict 落 store→有风险时建议挂既有确认链卡（零裸写）→queue view D2 节点点亮（not_started→in_progress/needs_confirmation，模式同 D1 的 WriteMixWorkflowQueueNodeState）；**入口识别先测裸话术能否过 semantic_entry**（263 教训：能过就不建词表，需词表则按「入口识别薄层」定位最小建）；
3. **验证腿**（真栈单 run）：D2 场景冒测（观察→判据→挂卡/无卡两分支诚实断言；判据腿零 LLM 预期，话术腿 LLM ≈¥1-3）；§8 跑前口径成文；脚本按 243/263 加参先例扩展现有冒测体系；
4. **指引面不在本卡**（等 B4 ablation 校准后另定，gate §五-2 原则）。

## 约束

- 文件域=chat D2 runtime 新文件+判据纯函数+queue view 挂点+测试+冒测脚本加参；禁改 D1/LW 既有语义（复用不改写）；禁重型 stage 机（终局裁定 §二-3）；治理红线（确认链/零裸写）不变；
- 持久化新字段按 §11 定义旧态缺省（omitempty/缺省关）。

## 停止条件

- 判据面发现观察面数据不支撑（M1-4 pack 字段缺失/口径不符）→取证上交转 M1 系修补卡；验证腿断言失败取证不重跑。

## 领取/回执/验收

- 领取：2026-11-06T19:07+08:00 / origin/main=50e6f859（领取时最新；卡面 dispatch 基线 bc33eb2e 之后仅协调笔 6f4231b9+0db83f8b，agent/scripts 零 diff 亲证 git diff bc33eb2e..origin/main -- agent/ scripts/ 为空；另注：50e6f859 合并说明提及"265 领取协调提交并入"但 diff 实只含 262 卡移动，本卡此刻在远端 todo 无 owner 亲核）/ owner=GLM-5.3（PC，ZCode 执行会话 265impl） / 分支=port/d2-stereo-thin-1（worktree=D:\Vit_DAW_wt_265impl，实现基线=50e6f859） / 领取提交=1f5e33ee（已推 main）
- 回执（执行侧 2026-11-06 21:2x）：
  - **实现**：`port/d2-stereo-thin-1`（已推 origin）f354430d=chat D2 薄 runtime 四文件（新 d2_stereo_check_runtime.go+test：判据纯函数 d2AssessStereoRisk 三态 risk_found/no_risk/unmeasurable 阈值复用 M1-4 MonoRisk+同款常量；verdict store stereo_check_verdicts.v0 LW 同款纪律 fail-closed/dedupe/裁剪；d1d3 handler 显式分支；queue view D2 挂 pre_master.stereo_check.v0，stage 映射 D2 仍指观察 pack 不动）+38df2eb2=冒测加参 -D2StereoCheck+三门取证修复两笔+003c5b9c=工件。**12 零 LLM 单测+chat 全包 133s+loudnesswatch/mixboard 全绿**。
  - **置顶申报①（动作面读法，请主管裁定）**：卡面"有风险时建议挂既有确认链卡"实现为 **v1 纯披露零挂卡**——确认链五操作无一能改变声道相关性（gain 对 mono 抵消比例不变、pan 收窄不修复相位抵消、真实修复属母带 E3 未建），按「不发明动作」红线（loudnesswatch/action.go 同款）不建死代码缝；queue 节点恒 in_progress（卡面允许两态）。若裁定要卡，剂量来源需先定义（B4/E3 校准后最小加）。
  - **置顶申报②（入口基线）**：裸话术"帮我检查一下这首歌用单声道播放会不会丢内容"真栈两轮均过 semantic_entry（run1=limit_reached 入循环、run3=done 完整观察回合，均非 unresolved 模板）——按 263 教训**不建词表**：检查请求词不进 inferProjectAwareCapability（单测钉死），深面入口=显式 d 标记+检查意图词（如"d2 翻译检查"，词法命中先于 semantic_entry=判据腿零 LLM）。指引面（自然话术如何路由进 D2 深面）维持等 B4 校准，本卡不动。
  - **真栈验证腿（d2run1 四轮，§8 全程在 RUNBOOK）**：占用 19:34 登记→21:10 拆净（owner 亲拆×4 TEARDOWN_ALL_ZERO 亲核，释放 1f0b0f0c 已推 main；与 264 错峰[其 19:22-19:28 占用]无重叠）。产品面全绿：门 A（stop_reason=d2_stereo_checked/verdict=unmeasurable[fixture 如实 stereo_face=missing 无生产者]/action_face=disclosure_only_v1/零 mutation）run1+run3 脚本化绿+run4 手动绿；门 B（stereo_check_verdicts.json trigger=d2_stereo_check）在盘亲证三轮；门 C 零挂卡零 mutation run3 脚本化绿+run4 手动绿；门 D（D2 节点 not_started→in_progress）持久化 agent_runtime_state 注记亲证（活 mix.report 被排干占用至拆栈，离线注记=queue view 折叠权威输入）。**exit 0 未达成**：run2/run4 死于 runner 排干族（254 armrun1 逐字同型晚高峰瞬态，非本卡面，run1/run3 同码通过）、run1/run3 死于本卡门组脚本管线 bug 四处（均已取证修复入库：吞异常盲搜/StrictMode 短路序/ANSI 读 UTF-8/mix_queue_view→mix.report 工具名）——**两处置选项交主管：①择非高峰窗补一次修复后脚本化 exit 0 轮（约 ¥0.5-1.5）②接受现证据组合**。
  - **计费**：四轮 164 调用 deepseek-flash ~10.1M chars 估 ≈¥3-6；其中排干重跑腿（run2+run4，非本卡面）133 调用 ≈¥2.5-4.5 如实入账；本卡 D2 面归属（run1+run3：probe×2+判据腿零 LLM+runner 单腿）≈¥0.8-1.5，卡面话术腿 ≈¥1-3 口径内。telemetry 原件 VitApp/Workspace/Logs/agent_llm_telemetry.jsonl 在盘可复核。
  - **端测边界声明（AGENTS §5）**：本卡改动=agent 判据/runtime/queue 挂点+冒测脚本——未触 webui 渲染面与 demo 用户旅程，两门槛（E2E-WEBUI-1/JOURNEY-1）不适用；真栈验收=run_free_state_d1_smoke.ps1 -D2StereoCheck 门组（上表证据+exit 0 未达成申报）。
  - **工件**：coord/runs/D2-STEREO-THIN-1/d2run1/（RUNBOOK §8 全程+console×4+d2_manual_gates.json+probe/check 证据，003c5b9c 随 port 分支）；大件留盘 worktree artifacts/free_state_d1_s1/*（gitignored）。
  - **域外观察**：①50e6f859 合并说明与 diff 不符（宣告并入 264/265 领取笔实只含 262）——协调面瑕疵非本卡面，已在此留痕；②D1 settle 门 B/D 的 254 时代脚本模式（ANSI 读 UTF-8 store、mix_queue_view 工具名、needs_confirmation 属性访问）从未被活栈验证过，本卡 run1-4 证明其中至少三处在 1-mutation 分支上会同样失败——D1 面脚本修补不属本卡文件域，建议主管开卫生卡（本卡修复仅及本卡门组，D1 门组代码未动）。
- 验收（2026-11-06 晚窗，主管）：**条件 pass**——判据/验证器/产品面真栈全过（门 A-D 证据组合+交叉复跑三包全绿）；**两置顶申报均接受**（①v1 纯披露零挂卡=不发明动作红线正确，动作面待 E3 后最小加；②裸话术过 semantic_entry 不建词表=263 教训正确应用）；exit 0 未达成→**选项①补跑腿挂今夜 23:00 后非高峰自主批次**（脚本 bug 四处已修入库，260 分支化已合入；补跑过=终验闭合，再败按 §8 取证）。计费诚实分账接受。域外处置：D1 门组脚本卫生**开卡 268**；50e6f859 merge message 瑕疵认账（教训=描述须与 stat 核对）。D2 认知=not_built→判据+披露薄层点亮。合入 7d6a5abe 零冲突。详见 [rulings/2026-11-06-D2-STEREO-THIN-1-pass.md](../../rulings/2026-11-06-D2-STEREO-THIN-1-pass.md)。

# D1-RUNNER-FORENSICS-1：133 时代 D1 runner 断言面取证与修复（254 运行面承接卡）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=[2026-11-05-AF-D1-LOUDNESS-SETTLE-1-final.md](../../rulings/2026-11-05-AF-D1-LOUDNESS-SETTLE-1-final.md) §四——254 两 run 两形态均卡 runner 断言面）
- 派发确认：已确认（主管发卡；链内承接卡）
- 验收负责人：GLM 主管决策流
- 池序 256；目标仓库=主仓 scripts/run_free_state_d1_smoke.ps1（+必要时 agent 侧零/最小面）；P1 / 0.5-1 天 / 依赖=254 实现已在 main；L1 / PC 执行会话；验证腿需真栈

## 背景（两 run 两形态，均已定性非 254 实现面）

- armrun1（flash 注入）：settle 链 600s 未排干——41 轮 LLM ×9-20s（瞬态延迟峰）→limit=timeout 停泊；
- armrun2（同引擎 flash）：settle 链 2 分钟收敛，卡 `exactly one forward mutation` 断言——模型走**诚实零 mutation 观察分支**（mutation_performed=False×2+mutation_authority=False 全 bundle）；
- **决策侧假说（待证）**：runner 断言校准于 10-18（M3 措辞链 243-248 之前）；v2 触发权句/v3 终局路由/guard 准入改变所有自由态循环模型行为面——D1 场景零 mutation 分支可能是新措辞下的合法行为。

## 目标

1. **取证**：对照 133 run2（10-18，coord/runs 在册）与 armrun2（coord/runs/AF-D1-LOUDNESS-SETTLE-1/armrun2/）——①载荷 needle 面（M3 措辞句在场差异）②动作序列（133 run2 的 mutation 发生轮次/工具 vs armrun2 零 mutation 的轮次路径）③排干窗口敏感性（133 run2 轮次时长分布 vs armrun1 的 9-20s 峰）——验证/否证措辞漂移假说，钉死零 mutation 分支的合法性（合法模型分支 vs 回归）；
2. **修复（按取证结果）**：runner 断言按 90 卡先例改**分支容忍**（mutation 计数=fixture-dependent 不硬断言；合法分支集=0 次[诚实观察]/1 次[前向 mutation]，各自断言其治理面完整性：0 次→无卡无写+边界申报在场；1 次→确认链/写达/读回）；排干窗口按取证结论定（如为延迟敏感→窗口参数化或重试一次 settle poll，禁无条件放大凑过）；
3. **验收腿**：修后单 run 重跑 **254 门组**（LW 四门不回退+D1 settle 四门+queue view D1≠not_started）——过则 D1 真栈点亮补全，254 运行面挂账清。

## 约束

- 文件域=scripts/run_free_state_d1_smoke.ps1（断言/窗口段）+取证工件；**agent 生产代码原则上零改动**（若取证证明 agent 面缺陷→只取证转卡不越域）；
- §8 跑前口径成文：取证腿零 LLM（读档）；验收腿单 run（预算 ≈¥1-3）；
- 断言放宽必须以取证钉死的「合法分支集」为据，禁无证据放宽（凑过红线）。

## 停止条件

- 取证发现 agent 面回归（措辞链破坏 D1 场景）→停修 runner，取证包转卡；
- 验收腿断言失败→取证上交不重跑。

## 领取

- 2026-11-05 22:07 +0800 / origin/main=c3833c67 / owner=ZCode GLM-5.3 L1 PC 执行会话（本 256 卡会话）/ 分支 port/d1-runner-forensics-1（领取提交在协调分支 coord/256-claim 上完成后转实现分支）/ worktree D:/Vit_DAW_wt_256impl / 领取提交=见 push 后回填（本提交）

## 回执

- **取证（零 LLM 读档，[coord/runs/D1-RUNNER-FORENSICS-1/forensics.md](../../runs/D1-RUNNER-FORENSICS-1/forensics.md)）——措辞漂移假说证实**，三点闭合（对照组=133 run2 `20261018_124639` 大件（loudness worktree 在册）vs armrun2 `20261105_215624`，同 fixture spv1_p01/reverb+同引擎 deepseek-flash，唯一变量=代码版本）：①**载荷 needle 面**：system prompt 14,331→17,670 chars（+23%），增长源=M3 链四笔（227 `4a439588` 闸门开放[227 卡回执亲证 system 15,468→17,045B]+235 `e9a51a06` 时变教学+243 `b6cbc007` v2 触发权+246 `40cdd189` v3 终局路由），全部 `git merge-base --is-ancestor` 亲证在 armrun2 二进制祖先且全晚于 133 run2；②**动作序列**：133=plugin_selection→装载确认→semantic_dynamic_confirmation→typed controller 写参（tx_dyn_9a04690e，rev 3→4，readback 0.3827673793）=1 forward mutation；armrun2=improvement_proposal（总线化提案）→driver 确认→semantic_treatment_strategy→渐进披露审计 partial→semantic_treatment_capability_boundary 终态（boundary=admitted+admission 治理面完整[typed reverb/budget=1]+d1_receipt=null+mutation_performed=False×2）=0 mutation；③**轮次时长**：133 也有 19.5s 单调用峰（1.9-19.5s×8 轮 2.6 分钟）——延迟分布两时代同族，armrun1 超时=41 轮随机分支×晚高峰延迟乘积，非窗口缺陷。
- **零 mutation 分支合法性判定：合法模型分支非 agent 回归**——准入正常（admitted 亲证）/诚实零写（零 interventions+零 receipt+journal 零行，与 2026-10-13 hallucinated-settle 死链本质不同：无完成宣称无 fabricated receipt）/边界拒绝=CCB 渐进披露 fail-closed 设计行为（AGENTS §5 投影纪律）/链路健康收敛（254 终验亲证 completed/done）。
- **修复（impl `3a52f640`）**：`scripts/free_state_d1_smoke.py` validate_d1 断言分支容忍（90 先例）——0 分支紧断言集=无卡无写（d1_receipt 空[死链契约]+journal free_state_d1_s1 行==0 服务端亲证+全响应无 mutation_performed=True）+边界申报在场（latest_decision.status∈{blocked,capability_blocked}+stop_reason 非空拦静默停泊）；治理预算纪律（budget==1/max_action_attempts==1/typed domain·kind·dose）公共前缀两分支共享不放宽；**1-mutation 主链全部既有断言逐字保留**（分支分派非放宽）；**排干窗口不动**（取证=非延迟敏感）。agent 生产代码零改动。离线验证：py_compile PASS+文档测试 8 函数全绿（新 test_validate_zero_mutation_boundary_forms=armrun2 形态正例+五反例）+**双向真实回放**（armrun2 真实 loop→新分支 PASS；133 真实 loop tx_dyn_9a04690e→主链）。
- **验收腿（verifyrun 两 run+一次环境重建，工件 [verifyrun1/](../../runs/D1-RUNNER-FORENSICS-1/verifyrun1/)）——门组未执行到，运行面验收挂起上交**：verifyrun1（222048）exit 3 NOT_EXERCISED（fs4 观察→`terminal_causes=当前上下文未披露任何轨道标识，无法填写提案目标`→提案面未达）；verifyrun2 首次（222843）启动腿环境中断（残留 kernel 被 5555 禁静默复用守卫拦，零 LLM，RUNBOOK 预写③重建兑现）；verifyrun2 最终（222912）exit 1「D1 loop projection is missing」（首响应即 capability_blocked=证据新鲜度准入未过，零实验零修改）。**两 LLM 形态均不在本卡修复面**（驱动循环零触碰 git diff 亲证）；无证据不扩分支（首轮拒≠D1 合法验收形态）；最终轮承诺兑现不跑第三轮。**处置选项三案交决策侧**：①择时重派单 run 验收腿（armrun2 形态复现时应 PASS——已被本卡合法化）②先派披露面取证卡（轨道标识披露窗口×证据新鲜度准入=两形态共同上游）③接受离线验证+回放作为修复面证据，门组点亮另行安排。
- 计费：取证腿零 LLM；验收腿两 LLM run 合计 ≈¥0.3-5 实际 ≈¥0.3-0.5（verifyrun1 ≈12-20 调用+verifyrun2 5 调用，agent log 亲数），预算 ¥1-3 内。
- 栈：占用 15ef1050（22:19）→释放随卡面收口提交；TEARDOWN_ALL_ZERO 亲核 22:33:40。
- 分支 `port/d1-runner-forensics-1`（3a52f640 实现+d2ba3c61 工件，已推）；领取提交 474e2dde。

## 验收

- （主管回填）

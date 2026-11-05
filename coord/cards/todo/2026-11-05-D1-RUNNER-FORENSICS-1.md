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

- （执行侧回填）

## 回执

- （执行侧回填：假说判定/断言改动依据/验收 run 判定面/计费）

## 验收

- （主管回填）

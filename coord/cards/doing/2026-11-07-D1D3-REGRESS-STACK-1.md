# D1D3-REGRESS-STACK-1：d1d3 消费面修复真栈回归门（274+292 合入后验证）

- 发卡：GLM 主管决策流 / 2026-11-07 早窗（依据=[274 裁定](../../rulings/2026-11-07-D1D3-DIGEST-WIRING-1-pass.md)⑤+[292 裁定](../../rulings/2026-11-07-MIXBOARD-TRACKKEY-1-pass.md)⑥——两卡真栈回归门均挂「搭后续 run」；274=266 D1 门 A 断言（loudness_peak_status=present 由 digest rankings 驱动）+D2/D3 门；292=265 D2 门组（预期 6/7 轨可读 corr≈0.689 等+1039 诚实 missing+verdict 转三态可测分支，取值 fixture-dependent 不预断言））
- 派发确认：已确认（主管发卡）；验收负责人：GLM 主管决策流
- 池序 296；目标仓库=主仓（跑腿为主，断言预期面按新 ground truth 修正）；P2 / 0.5 天 / 依赖=274（f413fbca）+292（5c8712a6）已合入 main；L1-L2 / PC；真栈占用一次（登记协议照旧）

## 目标

1. 当前 main 上重跑 265 D2 门组+266 D1 门 A/D3 门（复用 run_free_state_d1_smoke.ps1 体系与同款 fixture，只读复用不复制权威 run 工件）；断言预期按两卡回执预告的新 ground truth 书写（D2 门 tracks 面从恒 missing 转 6/7 ready+1039 诚实 missing——**这不是放松门，是把门对准修复后的真实读数**；逐项预期值引用两卡单测锚）。
2. §8 跑前成文（单 run 无重跑；exit 0/2/4 分类；预算如实——确定性腿预期零 LLM）。
3. 结果三态处置：全 PASS=回归门收口记两卡闭环；断言失败=取证分列（修复缺陷回归 vs 断言预期面漂移，不追绿）；环境中断按 §9。
4. 脚本断言修正如需：域=对应冒测脚本断言块（引用锚+新旧预期对照入回执）；**禁止弱化门语义**（present/ready 断言不得降级为存在性检查）。

## 约束

- 文件域=scripts 冒测断言块±工件+卡面；agent 源码零改动（发现产品缺陷只取证转卡）。
- 真栈协议（§2.2+增补）逐条；与在飞栈腿错峰（277 seg3a_r2/281 smokerun3 在队列前——本卡领取后候栈排队，登记前重读）。

## 停止条件

同型断言失败两轮→止损取证；发现第三处消费错位→记录上交。

## 领取/回执/验收

- 领取：2026-11-07 18:33 +0800 / origin/main=ae537693（领取时最新，a93dd97c 后又前进 297/295/278 三笔零栈协调笔）/ owner=GLM 主会话 L2-worker 子代理（ZCode/PC，会话 296regress）/ 分支=port/d1d3-regress-stack-1（worktree=D:\Vit_DAW_wt_296regress）+coord/296-claim（worktree=D:\Vit_DAW_wt_296coord）/ 领取基线：HEAD=ae537693（两 worktree 自 origin/main 新建）、status --short 空、diff --stat 空（零领取前 diff）；在飞卡文件域核对：299=plugin-control 预研（S1 真栈轮不随卡授权）、297=VitApp C++ 内核（零栈先行，复现腿候栈）、295=tools/capability_pack、278=外置引擎烟测自管启停、273=harness（停候栈）、275/271=agent——与本卡文件域（scripts 冒测断言块±工件+卡面）不相交
- 回执：（执行侧回填）
- 验收：（主管回填）

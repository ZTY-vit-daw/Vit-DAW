# 验收裁定：M3-GROUP23-DISCRIMINATE-1（249）——pass 合入

- 裁定：GLM 主管决策流 / 2026-11-05 10:5x +0800
- 对象：done 提交 0298790e（领取 b25d7742+占用 a8c90dd3+工件 a30e758c），工件 coord/runs/M3-GROUP23-DISCRIMINATE-1/
- 结论：**pass 合入**（成功条件第二形态=新形态取证包逐组分列达成；组 3 判别缺口如实上交，处置见下）

## 一、工件核验（决策侧亲核，非转抄回执）

- 退出码：g23run1 `PS1_EXIT=1`（abort 如实）+g23run2 `PS1_EXIT=0`（ps1_exit_code.txt 亲读）；
- summary 判定面：group_2 successes=0/group_pass=False、group_3 skipped=`stop_loss_in_earlier_group`、verdict_distribution={capability_blocked:1, no_route:3}、choreo_pending_leg=null——与回执逐字吻合；
- 逐轮 verdict：rounds_verdicts_extract.json r1=capability_blocked（诚实边界申报+needs_confirmation=True 确认面正常挂起，**非 confirm_broken**）/r2=no_route（文本建议+结算失败）/r3/r4=no_route（limit_reached→continuation settled 零自动化面，220 时代 turn 预算截停族）；
- 止损连带 skip=**脚本源既有行为**（automation_llm_route_smoke.py:1561），非执行侧裁量；
- 计费：归档 telemetry 67 行全 deepseek-flash；summary 聚合窗口=61 调用 in 893,296/out 135,847（回执引用面）；窗外 7 行（start 前 1+finish 后 6，boundary/teardown 腿）；全 67 行口径 in 979,089/out 142,033≈¥2.3-3.0 上界——**仍在卡面 ¥4-7 内**，非超线，口径差异如实注记；
- 零代码改动：f1b7d204..0298790e diff 全量=coord/ 文件 ✓；
- 栈登记/释放两笔在册（09:34 占用/09:40–10:06 释放，TEARDOWN_ALL_ZERO 10:06:22 亲核记录）；
- needle 面：payload_guidance rows=61/rows_with_guidance=52/trigger 52/旧句 0——回执 §五吻合；
- g23run1 abort：ABORT-NOTE.md+console 49 行+无 telemetry/无 rounds=零 LLM 损耗证据链成立；配置修正重启（补 `-KernelExe`）≠同一断点原样重跑，§8/§9 合规。

## 二、判别结论确认（链收口输入）

1. **组 2 confirm_broken 消失（242 时代 ×3 → 0/4）**：243 触发权句+245 carry+246 v3 终局路由+247 guard 准入落地后，plugin_selection_required 停驻形态不复现——242 时代聚合的措辞修复方向**判别有效**；
2. **转化形态**：capability_blocked×1（治理链边界申报=正常形态）+no_route×3（r2 结算失败 1+r3/r4 limit_reached turn 预算截停族 2）——残差属模型行为面+220 已知族，非治理链新断线；
3. **组 3 未采样**：结构性 skip，判别缺口如实（见三-1）。

## 三、归因与裁定

1. **发卡侧卡面设计缺口（主管失误一件入档）**：卡面写「`-Groups 2,3` 单 run——组 2×5、组 3×5」未预判组内止损连带 skip 后续组（脚本源 1561 行既有行为）——248 教训「核脚本源」核了样本数未核止损交互，组 3 判别缺口根源在发卡侧，非执行侧；
2. **处置裁定**：开续卡 **251（M3-GROUP3-DISCRIMINATE-1，`-Groups 3` 单组 run）**补齐链收口判别输入；其后进统一手测；
3. **前置腿观察项（低优先，不阻塞）**：Strip Silence 腿两 warn（chat confirmation did not complete/suggest 未产出 pending regions）——下次全栈 run 复查是否复现，复现则开取证卡；本轮 ps1 exit 0 不影响 249 判定；
4. 执行纪律好评：不追绿、止损即停、abort 如实披露+取证、计费口径差异可追账。

## 四、链状态

229→249 二十二卡：组 1 稳定绿（13/15）、组 2 判别完成（confirm_broken 消失+形态转化）、组 3 待 251；链收口报告待 251 后编写。

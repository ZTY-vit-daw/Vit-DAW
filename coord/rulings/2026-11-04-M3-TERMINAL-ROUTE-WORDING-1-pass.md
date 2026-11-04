# Ruling 2026-11-04：M3-TERMINAL-ROUTE-WORDING-1（246）——pass（实现合入 139fa254）+ 形态偏差裁可 + 新断点转 247

## 裁定

**实现 pass 合入**（merge 139fa254，含实现 40cdd189+工件 67f923a2）。四项措辞全落地；组 1 未绿（no_route×3）按停止条件出新断点取证包=卡面第二成功形态。**模型面十五卡目标达成：模型开始直接调用 set_points。**

## 主管亲核清单

1. **措辞亲读**：终局句三选（direct set_points+治理门拦卡第三形态，与 v2 触发权句同语义）；diff 恰卡面域（agentloop 三文件+smoke needle+新测试 167 行）；v2→v3 版本纪律 ✓
2. **测试亲跑**：agentloop ok 4.557s exit 0 ✓
3. **needle 亲证**：needle_scan.json 四 needle 28/33（缺席=setup 短请求 244 同型）+旧二选句 0；terminal_three_choice=0 与 rows_with_terminal_turn_directive=0 自洽（三轮直调被拒即 failed，无终局轮到达）——终局句真栈检验挂 247 后 ✓
4. **行为转变亲证**：agent log L226/260/289/347/354/408/420 七次 `automation.set_points confirmed=false`（245 同栈基线零直调）；r1 轮工件 10 次 guard 拒绝原文「mutation tool automation.set_points is forbidden」+needs_confirmation=false=guard 先于治理门截杀 ✓
5. **三承重代码亲证**：guard:26-29 全拒 MutatesProject/goalrunner:855 每工具先过 guard（先于 typedPluginApplyConfirmationResponse）/message_loop:4860-4863 final:true 清空 tool_calls
6. **计费亲核**：33 调用 in 474,640/out 46,955=¥0.85（in ¥1/M+out ¥8/M 口径，数学吻合）；链史上最便宜 run——直调路径少轮次的直接证据
7. **agent log 入库** ✓（245 裁定要求落实）

## 形态偏差裁定（项二）

**裁可**。卡面原文「final:true+set_points tool_call 示例」在运行时不可执行（final:true 清空 tool_calls+终局 output gate 拒一切 tool_calls，代码亲证）——照字面写=教模型「自以为写了实际没写」。执行侧改按合法治理链形态（直调轮 final:false+tool_calls→拦卡→终态轮 final:true 读回）并如实上交，卡面意图（消除唯一 final 示例=静态提案的邻接缺陷）完整达成。**此为 §11「证据推翻卡面前提→上交不自修」的标准执行**，入正面案例。

## 新断点定性（转 247）

chat executor 包装层既有 guard（`freeStateInvokeMutationReason`+`free_state_invoke_guard.go:26-29`）在 active free-state 期间全拒 MutatesProject 工具——它是「free-state 只读观察」旧契约的执法者，与 M3 automation surface 准入（决策轮直调→治理门拦卡）正面冲突。235-246 零直调从未显形，v3 首次走到直调暴露。**修复=guard 对 automation surface 已准入的两个 mutation 工具放行至下游治理门**（确认卡仍是唯一授权点，G7/G8 不动）；daw.invoke 与其余 MutatesProject 维持全拒。

## 链账本

229→246 十八卡：~30 违规重试→…→要素全齐停文本征询→措辞修正停 misroute→v3 直调 7 次→guard 截杀。**模型侧链闭合**；剩余唯一已知阻塞=247（guard 准入）+终局句真栈检验搭车。距首绿=一张确定性卡+一次重跑。夜 22:15 前后，247 完成后按 22:40 口径问询是否继续。

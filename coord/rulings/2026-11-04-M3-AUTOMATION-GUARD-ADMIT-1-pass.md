# Ruling 2026-11-04：M3-AUTOMATION-GUARD-ADMIT-1（247）——pass（实现合入 7b06c019）+ 链首绿成立（口径裁定）

## 裁定

**实现 pass 合入**（merge 7b06c019，含实现 0089b558+工件 366779d0）。guard 准入=确定性修复，文件域零越域（恰两文件 +156）。

**链首绿成立，口径如下（§7 分离链路进度与最终验收）：**
- **有效轮判据达成**：r1/r2 两轮一致 tool_chain_ok「confirmed write reached kernel」——**链 229→247 二十卡首次**：模型直调→guard 放行→确认卡（needs_confirmation=true+plan_id）→确认后写达内核（r1 0→4 点 0.74→0.95/24s 渐强包络；r2 0→5 点+get_curve 读回）+pre_confirm_zero_write=true 零裸写；
- **no_route 消失**：三轮零 no_route（246 同栈同引擎 no_route×3+guard 拒 10 次对照）；
- **r3 按 §8 排除出有效轮**（原始证据：729 条 continuation.park/2min+HTTP WinError 10061 fail-fast+telemetry 模型面直调亲证 22:38 三调用）——**主管再分类：非纯环境中断，是 agent 子系统缺陷**（park 循环无退避空转+listener 拒绝），开卡 248；本卡不因此翻案（r3 失败与 guard/模型/治理链无关）；
- **诚实边界**：ps1 exit 1（无 summary 按设计 throw）≠ §5 全组 exit-0 run；**链收口前须补一次全组 3/3 exit-0 稳定 run**（248 修复后），不得把「2/2+中断」表述为 3/3 全绿。

## 主管亲核清单

1. **guard diff 亲读**：两 lookup 分支白名单+helper（admission≠execution 注释），daw.invoke 位次不变全拒 ✓；goalrunner 零改动（双入口共用单函数）✓
2. **r1 治理链载荷亲证**：needs_confirmation=true+确认 preview→kernel_points_before=0→after_confirm 包络四点（0/8/16/24s 0.740818/0.8/0.88/0.95）→pre_confirm_zero_write=true ✓；r2 verdict+读回 ✓
3. **needle 亲证**：TERMINAL TURN 4 行+三选句 4/4（246 项一搭车达成）+四 needle 65/70+旧句 0 ✓
4. **r3 风暴证据亲核**：agent_last.log 729 条 park+8787 拒绝 69 行+telemetry 45 调用末次 22:38:29 ✓；agent 存活无重启 ✓
5. **计费亲核**：70 调用 in 1,082,281/out 77,918≈¥1.71（in ¥1/M+out ¥8/M 口径数学吻合）预算内 ✓
6. chat 包独立复跑：ok 128.593s exit 0（2026-11-04 23:0x 补记，验收批后确认；另定向 guard admit 测试 ok 0.191s）。

## 附带观察归档

- r1 settle_stop_reason=free_state_terminal_turn_unparseable：终局轮模型仍带 set_points tool_call 被 final gate 拒后 fallback（246 形态偏差同型，链继续未阻塞）——T2 观察挂账，组 2/3 run 时看是否复发；
- 245 挂账（recordGoalResult 缺席/durable=cancelled）并入 248 勘察范围（同属 continuation/settle 记账面）。

## 链账本与下一步

229→247 二十卡终局：~30 违规重试零执法→被拦即转向→工具链入册→缺标识/区间/当前值依次消失→文本征询面→misroute→v3 直调→guard 截杀→**guard 放行=首绿**。成本轨迹 ¥6.3→¥1.45→¥4.94→¥3.18→¥0→¥2.53→¥0.85→¥1.71（今日四 run ≈¥8.3 上界，全 flash）。
**下一步（链收口序列）**：248 park 风暴修复（含 245 挂账勘察）→全组 3/3 exit-0 稳定 run（组 1 复验+组 2/3 判别）→统一手测转正→225+A-F 主线。顾问模型触发线**解除**（两条触发线均未命中：needle 未失败、绿已达成）。

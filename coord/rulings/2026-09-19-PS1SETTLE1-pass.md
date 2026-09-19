# Ruling：PORT-PS1-SETTLE-1 pass（2026-09-19，PC 决策侧；交付面口径——settle 驱动交付，F⑤ 跑绿留待引擎裁定）

- 卡：`coord/cards/done/2026-09-19-PORT-PS1-SETTLE-1-chat-settle-driver.md`
- 实现：`port/ps1-settle-1` @`9f4cef2c`（2 文件 +167/−4：ps1 +143/−3、SMOKE_TESTS +14）→ 本裁定随附 cherry-pick 入 main
- 裁定：**pass（交付面）**——settle 驱动完整交付并被三轮真实栈实证有效；F⑤ 全脚本 exit 0 未达成，根因在卡域外（引擎终局行为），按停止条件止损上交，收口方向见 §F⑤ 处置（用户裁定）。

## 决策侧核验（全部亲验）

1. **实现**：`Wait-ChatTurnSettled`（ps1:1137）——非切片回合原样返回零开销；切片时 deadline+5s 轮询 `GET /agent/runtime/status` 的 goal.status 至六终态集，`GET /agent/events` 合成 settled 对象（stop_reason 映射/raw_*/settled_from/delivered_event_count 保留/executed_kernel_reply 补齐/typed_events 容错），`-ChatSettleSeconds` 默认 300。与 mac agent_chat_ctx 语义对齐。SMOKE_TESTS PC ⑤ 条目同步。
2. **三轮实证（工件亲核）**：
   - run1 `20260919_233041`：vocal focus 切片→settle 正确等到终态 failed（raw limit_reached 保留）→断言红。needs_experiment×2→gate 拒（G4/G5/G6/G8）→`free_state_terminal_turn_unparseable` 诚实回退——debug jsonl 证据亲核（needs_experiment×19/G4×16/G5×12/G6×12/G8×12/unparseable×2 三轮合计）。
   - run2 `20260919_233524`：**vocal focus settle→completed，stop_reason=done 断言通过 + observe 别名组断言通过**——completed 路径与升级契约双双实证（settled 对象 stop_reason=done/raw_stop_reason=limit_reached 正是设计行为）。随后崩于 typed_events 直访（脚本缺陷），exit 1。
   - run3 `20260919_233932`（修复后版本）：与 run1 同机制失败，exit 1。
   - §8 合格：run1/run3 同断点两败止损执行；run2 红为脚本缺陷且有定性+修复+下轮验证；无原样重跑。
3. **域外定性采信**：deepseek-v4-flash（PC 唯一配置引擎，config 亲核形态）在 vocal focus 续跑轮约 2/3 概率给出结构完整的 needs_experiment 终局，被 free_state admission gate 按设计结构性拒绝（BOUNDARY-1 §1.3 诚实回退）→ failed。**settle 驱动本身三轮全部正确工作**——阻塞在引擎终局行为，非脚本、非移植问题。

## 过程注记（非阻塞）

- **run1 console 尾码 0 与工件 status=failed 矛盾**：前置代码版本自身的 throw 未传播到进程退出码（被某层吞掉后自然结束）；run3 版本同路径 exit 1 实证失败传播已正确。轮次账目以 summary.json（三轮均 failed）为准，止损判定成立。执行侧回执"run1 exit 1"系按工件状态记账，与 console 尾码不符——如实记录，不影响裁定。
- 回执"push 暂挂"注记过时：分支实际已在远端（ls-remote 亲核 9f4cef2c）。

## F⑤ 处置（三选项上交，用户裁定中）

①换强引擎重跑（需用户在 `~/.vit/config.json` 配置第二引擎——现仅 rightapi/deepseek-v4-flash 单引擎）；②agent 侧开卡审视 needs_experiment 在两轨 fixture 的 admission gate 可满足性（产品行为域）；③接受"settle 就位+engine-dependent 终态"降级口径收口（deepseek 诚实 failed 是真实产品行为，非缺陷）。裁定结果见卡片验收字段回填。

# 验收裁定：M3-TIMEWINDOW-INTENT-ARM-1（253）——pass 合入

- 裁定：GLM 主管决策流 / 2026-11-05 19:3x +0800
- 对象：done 提交 9e28b1bd；实现 0593e05a+工件 1a5270bc @ port/m3-timewindow-intent-arm-1（随本裁定合入 main）
- 结论：**pass 合入**——组 3 修复达成：真栈组 3 group_pass=true（lexicon_hit 5/5 确定性），组 1 回归红线绿，词表最小面零接线。

## 一、工件核验（决策侧亲核）

- **实现面**：diff=automation_intent.go 两处补词（qualifier 增「秒之后/秒后」窄式禁裸「秒」；动词增「动起来」禁裸「动」）+automation_intent_test.go 正 3 负 5——与卡面最小面逐条吻合，**零接线 automation_choreo.go、未触 semantic_entry.go** ✓；
- **单测亲复跑**（wt_253impl）：TestAutomationIntentTimeWindowArming+TestAutomationIntentArmingBoundary 双 PASS；负例含组 2 话术「帮我把空间铺开」不武装、静态正例「把这条轨存在感调强一点」不变、「别动」「等 30 秒再说」「30 秒」裸跨度全负 ✓；区域通道断言 BareSpanSeconds=30→clarify（下游接线单测级亲证）✓；
- **真栈判定面**（summary_slim 亲读）：group_1 group_pass=true successes=4/5（回归红线绿）+group_3 group_pass=true successes=5/5；guidance needle 82/90 在场+旧二选句 0（措辞链零回退）；prompt_guidance_rendered=True ✓；
- **计费**：归档 telemetry 96 行全 deepseek-flash（summary 窗口 90+窗外 6，与 249/251 同口径差形态），in 1.46M/out 130K≈¥2.3 上界——卡面 ¥2-3 内 ✓；
- 栈两笔在册（18:49 占用/18:49–19:09 释放，TEARDOWN_ALL_ZERO 19:09:05 亲核）✓；
- 词表评估面：「秒之内/到 N 秒」窗式不入表（无正例覆盖，最小面纪律）——评估记录在案，后续话术家族需要另立卡 ✓。

## 二、判别与修复链终态（组 3）

251 定案「断点层=语义入口仲裁器」→ 本卡从确定性词表侧修复：武装门补时间窗标记后，「30 秒之后动起来」走 pre-model 接管（clarify_category 卡→lexicon_hit 成功形态，零 LLM 调用），**不再到达仲裁器**——251 的 12/12 unresolved 形态消失。仲裁器本体未动（251 判别结论维持：极歧义话术仍由其诚实拒绝）。

## 三、链终局

**M3 链 229→253 全线收口**：组 1 稳定绿（247 首绿+248 三 run+253 回归 4/5）、组 2 判别定案（confirm_broken 消失→诚实形态）、组 3 修复转绿（5/5）。链收口报告编写解除阻塞；225（澄清 LLM 交接）文件域冲突解除，可领取。

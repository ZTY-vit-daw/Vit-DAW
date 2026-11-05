# 验收裁定：M3-CLARIFY-LLM-HANDOFF-1（225）——pass 合入

- 裁定：GLM 主管决策流 / 2026-11-05 20:4x +0800
- 对象：done 提交 df459e90；impl de7c1da2+工件 4142024a @ port/m3-clarify-llm-handoff-1（随本裁定合入 main）
- 结论：**pass 合入**——判断权归属裁定（2026-10-30）落地：澄清决策上移 LLM，真栈组 4 新判据 3/3 实证。

## 一、工件核验（决策侧亲核）

- **实现面**：automation_clarify_handoff.go（445 行，单轮=固定协议指引+移交上下文[intent 结构化+候选值+能力目录只读披露+工程状态摘要]）+choreo 12 处 clarify 调用点迁移+219 三面固定问句逐字保留为 fail-safe 兜底（WorkflowData clarify_llm=handoff*/fallback 如实披露不静默）——与「确定性层只做识别+执行+兜底，判断归 LLM」冻结令一致（本卡 diff 零新增用户面固定澄清文案亲核）✓；**LLM 提议≠写入**：commands 走既有治理链 chatResponseForCommands（确认/执行分级原级）——治理红线面未被绕过 ✓；automation_intent.go 零改动（253 词表面未触）✓；
- **测试亲复跑**（wt_225impl）：handoff+ux 测试 PASS；chat 全包 112s PASS（回执）；新增 6 例覆盖五类形态/兜底披露/pending 状态机/marker 提议→治理确认卡 ✓；
- **真栈判定面**（summary_slim 亲读）：组 1 **5/5** group_pass=true（回归红线绿）+组 3 5/5 lexicon_hit+**组 4 3/3** clarify_handoff_llm（「副歌那 30 秒」→LLM 候选区间+类别确认+建议先打标记再写包络=裁定目标行为真栈实证，0 固定问句退化）；guidance needle 81/95+旧句 0；llm_calls 95（窗口口径，回执 100 含边界行）≈¥2.2-2.4 卡面内 ✓；py 组 4 为 opt-in（默认组集 1,2,3 字节不变）✓；
- **诚实披露两面核可**：①意外真实调用（单测 bring-up 初版 env 置空法失效触达真实引擎数百 token——已改 httptest 显式注入+broken-engine 复跑全绿，如实记录）；②端测边界（marker 写入内核纵深未做端到端真栈轮——组 4 判定面=第一回复 cancel 收尾，属段落标记能力线/A5 链；webui 渲染面未过浏览器断言=既有 E2E-WEBUI-1 缺席面）——边界声明与卡面一致，列入后续项；
- 栈两笔在册（19:55 占用/20:11:43 释放 TEARDOWN_ALL_ZERO）✓；迁移清单核对完整（219 三面+region/category/param 两入口；track kind 五类外不迁移=边界申报）✓。

## 二、链意义

M3 链收口报告 §六-1 的 225 就位：澄清卡从确定性模板升级为 LLM handoff（兜底保留）——组 3 武装（253）扩大的澄清通道流量有了质量层。M3 线至此无尾巴，**A-F 主线开线解除全部前置阻塞**。

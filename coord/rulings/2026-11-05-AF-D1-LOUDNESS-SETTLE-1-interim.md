# 中间裁定：AF-D1-LOUDNESS-SETTLE-1（254）——实现面 pass；真栈腿归因定案+补证腿裁定

- 裁定：GLM 主管决策流 / 2026-11-05 22:1x +0800
- 对象：doing 卡（release c301aa2e 取证上交）；实现 c687c9a1+工件 9b6e443d @ port/af-d1-loudness-settle-1
- 结论：**实现面 pass；运行面未达卡面成功条件（新门未执行到）——不判 done，裁补证腿一 run**

## 一、实现面核验（pass）

- diff 11 文件 +1197/−10；**红线零触碰**（automation_intent/semantic_entry 零命中亲核）；
- 单测亲复跑：loudnesswatch 包 ok+新增 7 例 D1LoudnessSettle 全 PASS（breach 挂卡点亮/非 breach 无卡/builtin 诚实/realtime 缺口披露/路由接线）；
- gate4 缺陷处置核可：133 FIX-LW-CONFIG-PERSIST-1（8b8bc70e）已修证据关线，本卡零 Go 修复正确；
- 治理面：breach→既有 mix_tick 确认链（确认前零 mutation）、LLM 提议≠写入、判断权冻结令面（builtin 一次性判断不写配置面）——口径全合。

## 二、真栈 exit 1 归因定案（决策侧亲读 agent 日志）

- **非模型分支**：material_qualification 全 passed、admission needs_experiment 正常推进、executed=5/receipts=18 栈健康；
- **非实现缺陷**：失败点在 D1 runner（133 时代脚本面）settle 排干，本卡 LW/D1 settle 门在 throw 前未执行；
- **根因=引擎延迟×排干窗口交互**：41 轮 message_loop LLM 调用每轮 9-20s（flash 晚高峰实测），goal 反复 `limit=timeout` 停泊，第 41 轮 60s 客户端超时→transient_llm_error→600s 窗口耗尽；133 run2 默认引擎同 fixture exit 0 对照成立——**执行侧归因候选被决策侧日志证据确认并精确化为「延迟」而非「模型质量」**（归因修正入册：下级披露候选+上级日志定案）。

## 三、补证腿裁定（增补一 run，跑前口径即本节）

- **条件**：同 fixture 同话术同门槛，**去掉 `-LlmModelOverride` 注入（默认引擎）**=133 run2 逐字基线；其余不动（含 -KernelExe wt_228 显式）；单 run 无重跑；
- 预算：默认引擎实费（133 run2 先例量级），跑前读 telemetry 计费面如实入回执；
- 成功条件=原卡面三件套（runner exit 0+LW 四门不回退+D1 settle 四门+queue view D1≠not_started）；
- 分叉：过→本卡 full pass 合入（实现+工件随验收入 main）；**再遇排干超时→引擎变量排除成立，开 D1 runner settle 链取证卡（133 时代面，含排干窗口×轮次节奏设计复核），本卡运行面挂起转卡**；
- 止损：本腿单 run；异常形态取证上交不重跑。

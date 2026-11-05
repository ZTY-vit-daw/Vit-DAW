# 验收裁定：M3-GROUP3-DISCRIMINATE-1（251）——pass 合入

- 裁定：GLM 主管决策流 / 2026-11-05 11:1x +0800
- 对象：done 提交 ee933880（领取 30ec9900+占用 d2a9f4ad）；工件 port/m3-group3-discriminate-1 @ 31b574a7（本次验收合入 main）
- 结论：**pass 合入**（成功条件第二形态=新形态取证包达成；ps1 exit 1=验证面结构性误报，机理三层源码锚点亲证，非判别中断）

## 一、工件核验（决策侧亲核）

- 判定面：summary group_3 successes=0/group_pass=False+telemetry 13 行全 deepseek-flash 全 `ordinary_agent_semantic_entry`（**零 message_loop**，与全仲裁形态自洽）+verdicts no_route×3+`prompt_guidance_rendered=False`——回执逐字吻合；
- **断点层钉死亲证**：arbiter_decisions 13 行=组 3 交换 12/12 unresolved（conf 0.18-0.35）+1 行 strip 腿 explicit_control（0.76，诚实分列）；确定性模板=semantic_entry.go:395 亲核；拒绝理由为模型逐次真实生成（13 调用即仲裁小面本身）；
- **exit 1 根因链亲证**：message_loop.go:532 `appendMessageLoopPromptRenderDiagnostic` 唯一调用点（全仲裁→零 message-loop 轮→render 文件未创建）→automation_llm_route_smoke.py:1298-1306 文件缺失即 return False→GUIDANCE_MISSING stop condition 2→exit 2→ps1:8779 throw；装配面 ccb_model_prompt.go:61 无条件追加亲核+被测 HEAD 零非 coord 改动（30ec9900/d2a9f4ad/9adfea31 全 coord/）——**非 242 装配缺陷回归**；判定数据在 throw 前完整落盘；
- 栈两笔在册（10:31 占用/10:36–10:50 释放，TEARDOWN_ALL_ZERO 10:50:22）；启动一 abort=bash 反斜杠（§5 已知陷阱）exit 127 零进程零 LLM，取证 ABORT-NOTE 留档，配置修正重启合规；
- 计费：in 9,079/out 3,452 ≈¥0.01-0.02——远低于卡面 ¥2-3.5（止损@3+仲裁小面短调用；无窗口差，249 注记口径本 run 无偏差）；
- Strip Silence 前置腿本 run 全绿——249 两 warn 未复现，维持低优先间歇性观察项。

## 二、判别结论确认（链收口输入，组 3 定案）

1. **组 3 断点层=语义入口仲裁器**（非措辞链、非治理链）：「30 秒之后动起来」话术族在 chat/semantic_entry.go 独立分类小面被拒（12/12 unresolved），回复为确定性模板；242 时代 r1/r2 形态逐字复现（同层同模板），r3「过仲裁→空转」形态未复现（解析率 1/3→0/3，小样本如实）；
2. **措辞链（243/245/246/247）效应面在自由态装配之后，对组 3 话术族结构性不可达**——组 3 改进杠杆在仲裁器解析（模型能力/仲裁面设计），不在动作路径措辞；链 229-251 的治理面结论不受影响（其作用域=过仲裁后的执行路径）；
3. lexicon_hit（组 3 成功形态）0/3：裸话术与跟进均不在确定性词表——组 3 判据未过是**判别事实**，非执行缺陷。

## 三、处置

1. **脚本判别面盲区两项开卡 252**（SMOKE-GUIDANCE-CHECK-DISCRIM-1，P2）：check_prompt_guidance 区分「文件缺失+零 message-loop 轮（全仲裁形态）」与「文件在场缺 needle」；全仲裁形态不触发 stop condition 2——避免未来全仲裁场景结构性误报 exit 1；
2. **仲裁器解析改进候选**记入链收口报告（组 3 话术族解析=新开发面，是否开卡由用户在链收口后裁定，不在 M3 链内）；
3. 执行纪律好评：越域即停未动脚本、锚点逐行可核、249 教训（-KernelExe）执行到位、计费口径按归档全行统计（249 注记落地）。

## 四、链状态

229→251 二十三卡，三组判别全部定案：组 1 稳定绿（13/15）/组 2 confirm_broken 消失→转化形态/组 3 仲裁器层钉死（措辞链不可达）。**链收口报告+统一手测清单解除阻塞，可编写。**

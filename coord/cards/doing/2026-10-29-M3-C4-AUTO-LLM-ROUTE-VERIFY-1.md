# M3-C4-AUTO-LLM-ROUTE-VERIFY-1：LLM 轮次路由验证——词表外围变体话术×真实模型轮次（217 遗留面+用户质询）

- 发卡：GLM 主管决策流 / 2026-10-29 深夜（依据=用户质询「目前这种手测你能确保连入 LLM 后是生效的吗」+217 RESULTS 边界声明自认「LLM 话术→工具选择面未测」）
- 派发确认：已确认（用户质询即诉求；运行前口径已按 §8 成文）
- 验收负责人：GLM 主管决策流
- 池序 220；目标仓库=**主仓（烟测脚本+工件；生产改动仅当验证暴露缺陷时另折修复卡）**；P2 / 0.5 天 / 依赖=无硬依赖（可与 219 并行——chat 真栈腿错峰）；L2 / PC 执行会话（LLM 判读面）

## 问题定义（用户质询的准确拆分）

**已验证面（无需重验）**：chat 入口确定性路由**优先于 LLM**——用户 10-29 手测即证（话术秒出确认卡，无 LLM 轮次延迟）；词表命中→确认→写入→生效全链用户亲证。

**未验证面（本卡标的）**：词表**未命中**的变体话术进真实 LLM 轮次后：
1. 217 的 system prompt 路由指引行（embed 双插入）能否引导模型正确选择 automation 三工具（get_plugin_parameters 读值域→set_points 带 points）；
2. LLM 发起的 set_points（RiskConfirm）在 chat 面的确认链与 choreo 确认链是否同构可用（确认卡→用户确认→写入→get_curve 读回）；
3. LLM 轮次与 choreo pending 状态的互不干扰。

## 运行口径（AGENTS §8，执行前不得改）

- 变体话术三组（词表外围形态，如「这段让它越来越突出一点」[无区间无类别词命中]、「帮我把空间铺开」[空间类但无插件轨场景]、「30 秒之后动起来」[边界形态]），每组 5 轮真实 LLM chat 轮次，上限 15 轮。
- 成功条件（组级）：≥4/5 轮呈现正确行为=组 1/2 → 模型走 automation 工具链或如实报边界（不虚写不绕路）；组 3 → 词表命中或 LLM 合理处理任一。
- 失败分类（逐轮记录）：no_route（模型纯文本建议）/ wrong_tool（选错工具）/ confirm_broken（确认链断）/ write_fail / 环境中断（仅原始日志在场时可剔除）。
- 止损线：同组连续 3 轮同型失败 → 停止，该型取证折修复卡，剩余轮次不再消耗。
- 成本面：LLM 轮次真实计费，回执记录轮数与积分定性。

## 交付

验证报告（逐轮话术/模型行为/工具调用遥测/结果分类+组级结论）入 coord/runs/M3-C4-AUTO-LLM-ROUTE-VERIFY-1/；**零生产改动为默认形态**——验证暴露的缺陷一律折卡不在本卡修。真栈腿按 §2.2 登记。

## 文件域

`scripts/`（LLM 轮次驱动脚本，现有 ps1 体系内扩展）+工件。生产代码零触碰。越域即停。

## 停止条件

- LLM 面栈不可用（API 额度/环境）→环境中断如实记录，不折功能结论；
- 发现 chat 面根本未装配 system prompt embed（217 指引未生效形态）→锚点上交，属部署缺陷非模型行为。

## 领取

- 时间：2026-10-30 早窗
- origin/main：61a25d33（领取前 fetch 亲核，本地=远端）
- owner：ZCode GLM-5.3 PC 执行会话（220 卡会话）
- 分支 / worktree：port/m3-c4-auto-llm-route-verify-1 @ D:/Vit_DAW_wt_m3c4lr1（基线 61a25d33）
- 领取时主检出既有 diff（非本卡，零触碰）：VitApp/Workspace/{Settings.xml,default_project.xml} M + coord/runs/UX-AB-AUDITION-IMPL-1/* M + 大量 coord/runs 未跟踪工件（gate 61a25d33 态）
- 错峰核对：doing 仅 GEN-SUNO（Mac 设计卡无栈腿）；219 仍在 todo 未领取；栈记录最新=218@2026-10-29 23:5x 释放

## 回执

- 执行侧自验完成：2026-10-30 晚窗；验证执行完成形态（非 pass/fail 生产门）
- commit：port/m3-c4-auto-llm-route-verify-1 @ 61e4d072（脚本 01caf02a+迭代修复 77ecb37b+工件 61e4d072；主仓 main 侧=领取 dc64a4ab+占用登记 4dd7b6b1+释放 8f09bafb）
- run：coord/runs/M3-C4-AUTO-LLM-ROUTE-VERIFY-1/{RESULTS.md, llmroute_20261030_1（迭代轮留证）, llmroute_20261030_2（定案轮）}（随 port 分支入库；运行时工程文件剔除）
- **结论一句话：停止条件 2 命中——217 automation 路由指引在变体话术的 LLM 装配分支（message_loop_neutral_family_selection）未插入，模型从未收到指引；组 1 三连 no_route 止损，组 2/3 剩余轮次未消耗；锚点上交属部署缺陷，非模型路由能力结论。**
- 逐轮话术/模型行为/工具遥测/分类：RESULTS §一§三+rounds/*.json 完整回执原文；LLM 真实计费 25 调用/261,771 in/18,042 out tokens（deepseek-flash=用户真实引擎）
- 折卡建议（RESULTS §四）：①修复卡=neutral 装配分支补 automation 指引（render 逐行证据可回指）②同锚点族=观察面剥轨级插件披露（207 同族）③turn 预算面；修复后重验卡可直接复用本卡断言体 v2
- 端测边界声明：覆盖=chat 面真实 LLM 轮次+render 逐 turn 装配取证+agent log 工具流+内核 setup/曲线面；不覆盖=指引在场前提下的模型路由行为（部署缺陷前置未达）/set_points 确认链（未触发）/choreo pending 互不干扰（未达）/组 2/3 话术/webui 渲染面（既有边界）
- 零生产改动默认形态兑现：全部改动=scripts/ 两文件

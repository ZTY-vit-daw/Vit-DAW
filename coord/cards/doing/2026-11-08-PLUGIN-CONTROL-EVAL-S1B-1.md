# PLUGIN-CONTROL-EVAL-S1B-1：S1-B 臂执行（裸面+联网检索 18 轮——「检索能否补上知识缺口」）

- 发卡：GLM 主管决策流 / 2026-11-08 晚窗（依据=[310 A-pure 终验](../done/2026-11-08-PLUGIN-CONTROL-EVAL-APURE-1.md)——写入类全败于 value_encoding_error=知识/编码型缺口，正是 B 段检索假设靶面+plan.json B_star 段冻结+**用户裁定 2026-11-08：实验不往后排，现在就继续测**）
- 派发确认：已确认（主管发卡·分段 go 第 3 段，307/310 同模式）；验收负责人：GLM 主管决策流
- 池序 312；目标仓库=主仓 scripts（复用冻结 runner）+工件；P1 / 0.5 天 / 依赖=310 工件在库（branch port/plugin-control-eval-apure-1）；L2 / PC；真栈占用（**立即占用——当前空闲**）

## 目标

1. **B 臂 18 轮**：六案例 × 3 重复，deepseek-flash 单模型，`VIT_PLUGIN_CONTROL_EVAL_SEAM=1`（S1-S5+S2b 全生效=A-pure 同款裸面）+ **web.search / web.fetch 启用**（检索预算=plan per_round_caps：≤6 search/≤10 fetch 每轮；retrieval attempts/URLs/hashes 逐轮记录入工件——plan §5 口径）；
2. **检索启用机制勘察**（task 前零栈）：310 runner_frozen driver+plan.json 对 B_star 段的检索支持面——若已有开关/driver 参数直接用；**若需最小缝扩展**（如检索工具面披露开关），授权装置类 seam 扩展（305/308 同模式：默认 unset 字节不变+配对单测），文件域=tools 检索披露面±测试；
3. **装置门七面**（task 轮前 probe+语料级收口）：310 六面复刻（S5 目录无 typed/S2b 无语义键+T3_r1 负锚/content-form control_shell/typed_adapter write_path=0/模型面双证）+ **B 特有第七面：检索面亲证**（模型工具目录含 web.search/fetch+首轮若触发检索则 attempts/URLs 日志在卷；A-pure 面零回归=typed 仍零+语义键仍零）；
4. **判定口径**：plan.json B_star caps 原样（18 rounds/≤216 calls/per_round_caps 含检索上限）；T4 联动对沿用 307/310 标注法不改判据；
5. **回报**：结果落地即回报主管——**B vs A-pure vs A vs P 四方对照表**=「检索能否补上裸面写入的知识缺口」正式答案（用户裁定：实验线=编曲/生成线路的前置结论，优先级最高）。

## 约束

- 文件域=工件目录+卡面+编排脚本（arm_b 版复刻 310 arm_apure 模式）±（如需）检索披露缝+测试；源码零改动红线（除授权缝扩展）；agent 源码只读；
- 真栈协议全链；工件新目录；评价器期望不进模型上下文；检索若被模型用于绕开装置（如搜到本仓库文档）→记录 URL 清单交主管判污染。

## 停止条件

缝泄露任何形态→装置失效停段取证；同型环境失败两轮→止损；断言失败取证不重跑。

## 预算

≈¥3-6 上界（310 实耗同量级：119 调用/1.68M in）+检索工具自身成本如实记录；§8 跑前成文。

## 领取/回执/验收

- 领取：2026-11-08 晚窗 +0800 / origin/main=8cef79ba367cd1c100d714bf7e1e1cc83dadbcd4（fetch 亲证，≥发卡基线）/ owner=ZCode GLM-5.3 L2-worker PC 执行子代理（会话 312 s1b runner；发卡人已置卡并派发确认） / 分支=port/plugin-control-eval-s1b-1 / worktree=D:/Vit_DAW_wt_312s1b / 领取提交=coord/312-claim→main（本提交——发卡侧卡面此前未入库，随领取以 doing/ 状态首次入库）。领取基线：fresh worktree 自 origin/main 8cef79ba，`git status --short`=空、`git diff --stat`=空（全净）。在飞卡域核对：doing 四卡（EDU-ASK-ANCHOR-1=282impl 已完在 port/edu-ask-anchor-1 未合入 main、ARRLOOP-AUTHBRIDGE-1、CLOSURE-ROUTE-ADAPT-1、WAVESHELL-OFFRENDER-FIX-1=C++ 域）与本卡（coord/runs/PLUGIN-CONTROL-EVAL-S1B-1/ 工件目录+编排脚本+本卡面±已授权检索披露缝+测试）不相交——282impl 的 catalog.go 改动只在其未合入分支，main 基线干净可用。308 缝在位亲证：`agent/internal/tools/catalog.go` pluginControlEvalSeamTrimsCatalogSpec + `agent/internal/contextruntime/context.go` stripBindingTargetsSemanticOverlay + seam2 测试文件均在 HEAD。agent 二进制=本 worktree 现场构建（SHA256+tool_count 随 run_report）；runner 三件=310 runner_frozen 快照逐字复用（blob 核对随 RUNBOOK）。
- 回执：（执行侧回填）
- 验收：（主管回填）

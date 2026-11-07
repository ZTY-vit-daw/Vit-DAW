# KERNEL-OFFRENDER-REPRO-1：离线渲染静音守卫复现（297 转卡：留痕插桩+M0-M3）

- 发卡：GLM 主管决策流 / 2026-11-07 晚窗（依据=[297 取证报告](../../runs/KERNEL-OFFRENDER-REPRO-1/297report.md)（实际路径=port/kernel-offrender-forensics-1 @ ba08b03c：coord/runs/KERNEL-OFFRENDER-FORENSICS-1/report.md）+[297 裁定]——候选②=tracktion_ExternalPlugin.cpp:1338/1341 双 fail-silent 守卫（hasLoadedInstance/isInstancePrepared 无日志 return）与 seg3a_r2 全观测逐项吻合，为唯一不可排除候选）
- 派发确认：已确认（主管发卡）；验收负责人：GLM 主管决策流
- 池序 301；目标仓库=tracktion 引擎最小插桩+复现腿；P1 / 0.5 天 / 依赖=297 工件在库；L2 / PC；C++ 最小插桩零栈先行，复现腿真栈一次

## 目标

1. **守卫留痕插桩（最小修同体）**：tracktion_ExternalPlugin.cpp:1338/:1341 两道 fail-silent 守卫加一次性留痕（命中即日志，不改变行为）——§5.1 留痕口径补齐；插桩后 C++ 侧构建（wt_228 同源树 D:/Vit_DAW/tracktion_engine）。
2. **M0 复现基线**（真栈，复用 arr_srcload 驱动器同场景）：seg3a_r2 同形态（装载→clip→渲染）跑一次，观察守卫日志是否命中——命中=候选②证实（物化缺陷定性成立）；未命中=回到并陈。
3. **M2 rack 二分**（免 LLM）：instantiate_plugin 直挂轨（不经 rack）渲染对照——区分 rack 链 vs 插件实例面。
4. **M3 插件特异性**（免 LLM）：换 Surge XT 同场景——区分单插件 vs VSTi 通用。
5. （可选 M1 实时对照：实时播放 vs 离线渲染差分——如前四步已定论可不做，跑前定。）
6. 结论：守卫命中与否+最小修建议（留痕已随插桩落地；渲染路径实例状态断言是否加=交主管）。

## 约束

- 文件域=tracktion_engine 两守卫点位（一行级留痕）±构建工件+scripts 复现腿复用+工件目录；**不改守卫行为语义**（只加日志）；agent 零改动；
- 真栈协议全链，候栈排队（300 S0/P 之后或错峰）；§8 跑前成文（M0 含 chat 驱动则 flash 预算 ≈¥1-3，M2/M3 零 LLM）；
- C++ 构建用 wt_228 同源配置，新二进制哈希入卷（不覆盖 wt_228 既有内核——主检出或新目录，记录清楚被测二进制）。

## 停止条件

插桩构建失败→环境中断类如实记录；M0 守卫未命中且 M2/M3 无差异→并陈上交（297 结论③维持）；同型断言失败两轮→止损取证。

## 领取/回执/验收

- （执行侧回填）/（执行侧回填）/（主管回填）

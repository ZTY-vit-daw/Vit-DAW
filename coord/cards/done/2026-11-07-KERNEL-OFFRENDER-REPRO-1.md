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

- 领取：2026-11-07 22:42 +0800 / origin/main=9180ff89b0c97594295c26fde54306358136afb7 / owner=ZCode GLM-5.3 L2-worker PC 子代理（301repro）/ 分支 port/kernel-offrender-repro-1 / worktree D:/Vit_DAW_wt_301repro（领取基线：HEAD=9180ff89，status 全净，diff --stat 空）/ 领取提交=本笔 coord/301-claim
- 回执：2026-11-07 23:1x 自验完成，mv done/ 待主管终验。**实现/工件=port/kernel-offrender-repro-1 @ 3e2300e1（prep 48d01d48+run 收口）已推**；evidence=coord/runs/KERNEL-OFFRENDER-REPRO-1/evidence.md（四腿全证据+引擎 patch 全文）。摘要：
  1. **插桩落地**：引擎树两守卫 +25 行一次性留痕（每守卫×live/render 相位各一 shot，TRACKTION_LOG→内核日志；只加日志不改行为）；插桩内核 `CC9FDACC…`（wt_228 同源配置新目录构建 exit 0；VitApp 源 vs wt_228 零 diff；wt_228 基线不动）；引擎树 diff 保留在位，处置交主管。
  2. **M0a2=seg3a_r2 逐项复现**（277 ps1/agent 62F444B7 逐位复用，唯一实质变量=内核）：装载链全绿→渲染 8.0s **data sha256 与 seg3a_r2 逐位同款的数字静音**，py exit 4 @render。
  3. **核心判决：三腿 VIT301_GUARD 全零**——两条静音腿中守卫零命中（日志管线有效性三重亲证：同宏设备枚举行在卷+插桩串只在二进制+出声腿全链健康）→ **297 候选②（守卫 fail-silent 早退）实证排除**；卡面停止条件「M0 守卫未命中」分支兑现，但 M2/M3 **有差异**故未走并陈：
  4. **M2 直挂 GR 仍静音（非 rack 特有）+M3 直挂 Surge XT 出声（-13.63 dBFS）→ 定性=插件特异性（WaveShell1-VST3 离线渲染静音，非 VSTi 通用，非 rack/合路/MIDI 馈入）**——兼容性缺陷方向（黑名单/预热策略），供主管定修复卡；M1 实时对照未跑（卡面可选，遗留问题=GR 实时是否出声，建议归修复卡）。
  5. 跑序异常如实分账：M0a1=agent 构建面错配（main 无 277 arrangement pack 能力面）exit 3 配置中止，attempt-2 修正（非原样重跑，298 先例）；四腿 TEARDOWN_ALL_ZERO 亲核（22:58:35/23:03:44/23:06:43/23:09:47）；计费合计 ≈¥0.1-0.2 flash（M0 两腿）+M2/M3 零 LLM（¥1-3 预算内）；栈登记六笔（6baa4646/0fe253dd/152a3368/ad9cb7fd+收口笔）全链在 PC-RUNTIME-STACK.md。
  6. 端测边界：agent HTTP/ZMQ 面真栈烟测+wav 物理计量+内核日志留痕；不覆盖 webui 渲染面与 Godot 旅程（本卡无该域改动）。渲染路径实例状态断言建议：守卫面已证清白，断言价值转移为渲染前 WaveShell non-realtime 自检/黑名单面，交主管裁量。
- 验收：（主管回填）

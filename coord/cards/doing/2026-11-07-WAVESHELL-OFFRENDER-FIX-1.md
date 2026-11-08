# WAVESHELL-OFFRENDER-FIX-1：WaveShell 离线渲染静音兼容修（301 定性转卡）

- 发卡：GLM 主管决策流 / 2026-11-07 深夜（依据=[301 复现判决](../done/2026-11-07-KERNEL-OFFRENDER-REPRO-1.md)——**297 候选② fail-silent 守卫实证排除（零命中+日志管线三重亲证）**；M0a2 逐位复现 seg3a_r2 静音（wav sha 同款）+M2 直挂 Grand Rhapsody 仍静音（非 rack）+M3 同链 Surge XT 出声（-13.63 dBFS，非 VSTi 通用/非合路面）→ **定性=WaveShell1-VST3 包装器在离线渲染下的插件特异性兼容缺陷**）
- 派发确认：已确认（主管发卡）；验收负责人：GLM 主管决策流
- 池序 309；目标仓库=tracktion 引擎插件处理面+VitApp；P1 / 1 天 / 依赖=301 工件在库+插桩内核 CC9FDACC 在位；L2 / PC；C++ 取证先行零栈，验证腿真栈

## 目标

1. **M1 实时对照**（301 遗留）：Grand Rhapsody 实时播放是否出声——分野「non-realtime 特异」vs「WaveShell 更深层问题」（决定修法面）；
2. **根因定位**：WaveShell 包装器离线路径（setNonRealtime/prepareToPlay 独立实例形态——297 候选②的次要保留项现为首要嫌疑面）+插桩内核 CC9FDACC 加密观测；
3. **修法**（择一或组合，证据钉死）：预热策略（渲染前 dummy process 预热 WaveShell）/黑名单+替代路径（离线渲染对 WaveShell 类回退实时式渲染）/包装器 workaround；**最小干预原则**；
4. **渲染前自检断言**（301 处置转移）：渲染路径加 WaveShell non-realtime 自检（出声探针失败即警告/降级），交主管定形态；
5. 验证：M0a2 场景复跑=出声（RMS 非零）+M3 Surge XT 不回退+全包回归。

## 约束
- 文件域=tracktion_engine ExternalPlugin 域±VitApp 渲染服务面+scripts 复用+测试；插桩留痕保留；真栈协议照旧；
- §8 跑前成文；C++ 构建走 301 同款 cmake 配置。

## 停止条件
根因进入 WaveShell 闭源二进制不可修面→取证上交转「黑名单+替代路径」纯策略方案裁定。

## 领取/回执/验收
- 领取（取证腿·零栈先行）：2026-11-08 / origin/main=8c68c186f86f32c9e9c97c0af135a19c1bc31393 / owner=ZCode GLM-5.3 L2-worker PC 子代理（309 取证腿） / 分支 port/waveshell-offrender-fix-1 / worktree D:/Vit_DAW_wt_309recon（领取基线：HEAD=8c68c186，status 全净，diff --stat 空；worktree 内 tracktion_engine 子模块未初始化=b4397498 gitlink，取证输入用主仓 D:/Vit_DAW/tracktion_engine @ 60ab1a49 只读——301 插桩已由主管处置①落为 engine repo 独立提交，主仓子模块指针 M 属预期态，不触碰）；领取提交=本笔 coord/runs/WAVESHELL-OFFRENDER-FIX-1/（PROGRESS.md）+卡面回填，推 port 分支（派发指令明示不合入 main，协调面处置交主管）
- 回执：（执行侧回填）
- 验收：（主管回填）

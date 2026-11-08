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
- 回执（**取证腿·零栈先行完成 2026-11-08**）：工件=coord/runs/WAVESHELL-OFFRENDER-FIX-1/（recon-design.md+PROGRESS.md）@ port/waveshell-offrender-fix-1 已推；**零栈零 LLM 零源码改动零计费，无 run ID**（VitApp/tracktion_engine 全程只读）。摘要：
  1. **取证方法清单**（recon §0）：引擎渲染循环 non-realtime 全链源码亲读（Renderer/NodeRenderContext/PluginNode/Plugin/ExternalPlugin/vendored JUCE AudioProcessor+VST3Impl+VST3_SDK 规范）+VitApp 渲染参数面亲读+301 工件观测对账（M2 日志实况设备 44100/**bs=480**、WaveShell 共享容器证据）+实时电平观测面勘察（LevelMeasurer API/ZMQ 命令面）。
  2. **根因定位（主嫌疑 S1 全链钉死）**：离线渲染对 VST3 的 processMode 信令不一致——JUCE 基类 setNonRealtime 仅置标志（juce_AudioProcessor.cpp:410-413），setupProcessing(kOffline) 因时序永不发送（re-prepare 发生在 node build=置标志之前→再发一遍 kRealtime；同 sr/bs 场景被 Impl:2489-2493 早退守卫吞掉），而每个 process 调用 ProcessData.processMode=kOffline（Impl:2647-2648）——**插件 setup 级 kRealtime vs per-call kOffline 不一致贯穿全程**，违反 VST3 SDK 切换规范（ivstaudioprocessor.h:138-143：切 kOffline 必须 setupProcessing）。与 301 三 run 全观测逐项吻合（GR 两挂载静音/Surge XT 容错出声/守卫零命中/无错误日志）。次要嫌疑：S2 WaveShell kOffline 真缺陷（黑箱）/S3 reset-setActive 循环状态清洗/S4 flush 预热早断（弱）。**停止条件未触发**（S1 干预面全在 host 侧可修；S2 闭源但修法 B 绕开）。
  3. **M1 实时对照腿设计**（只设计）：零插桩不可行（无既有实时电平读出面）→VitApp-only VIT309_RTLEVEL 插桩（LevelMeasurer::getLevelCache 定时 tap）+301 M2 直挂装置复用+--realtime-probe 驱动模式；判定矩阵=RT-SOUND（→non-realtime 特异，B 案可用）vs RT-SILENT（→装载面升格，B 降级纯警告）；单 run 零 LLM。
  4. **插桩加密观测腿**：VIT309 mode 遥测三点（initialise prepareToPlay/setupProcessing 发送/首个 processAudio）+可选判别实验（Impl:2648 一致化=修法 C 原型）；与 M1 同栈同内核一次占用。
  5. **修法三案评估**（最小干预排序）：**B 黑名单+realTimeRender=true 回退（VitApp-only ~5-15 行，对 S1/S2 皆免疫，排序 1）> C 包装器信令一致性化（vendored JUCE ~3-5 行，待判别实验证 S1，排序 2）> A 预热（证据弱不推荐单用，排序 3）**。
  6. **自检断言形态**：推荐形态 3「渲染后静音断言+自动降级重渲」（resultMagnitude 信号已在 NodeRenderContext.cpp:156/Renderer.h:92）+shell 类提示日志；探针式纯警告/门禁两形态待命不默认。
  7. **候栈排队**：M1/插桩/验证腿=真栈，排 **296-R2/310 之后**（§2.2 登记协议照旧）；裁定请求 5 项见 recon-design.md §7。
  8. 端测边界：本腿零栈零改动无端测义务；后续腿端测=ZMQ 面真栈+内核日志+wav 物理计量，不覆盖 webui/Godot 旅程（无该域改动）。
- 验收：（主管回填）

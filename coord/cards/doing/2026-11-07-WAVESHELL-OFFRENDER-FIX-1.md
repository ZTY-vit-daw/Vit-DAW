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

- **取证腿（零栈先行）**：领取/回执两节已回填于 port 分支 `port/waveshell-offrender-fix-1`（领取 8113a09a / 回执 0644ae89，均已推远端）；主交付件=`D:/Vit_DAW_wt_309recon/coord/runs/WAVESHELL-OFFRENDER-FIX-1/recon-design.md`+PROGRESS.md；零栈零 LLM 零源码改动（telemetry 未创建亲证）；主仓 tracktion_engine 子模块指针 M=301 插桩在位预期态，全程只读。

### 验收（取证腿，主管）

- **pass（2026-11-08 晚窗）**——六承重锚点主管亲读逐字吻合：①早退守卫 `juce_VST3PluginFormatImpl.h:2489-2493`（isActive+同 sr/bs 即 return）②per-call mode `:2647-2648`（`isNonRealtime()?kOffline:kRealtime`）③基类 `juce_AudioProcessor.cpp:409-413` setNonRealtime 仅置标志零 VST3 调用④SDK 规范 `ivstaudioprocessor.h:138-143`（切 kOffline 须 setupProcessing，RT↔Prefetch 才可 per-call）⑤渲染初始化序 `tracktion_NodeRenderContext.cpp:135-139`（setAllPluginsRealtime→prepareToPlay→flush；析构恢复 true）⑥`Renderer.h:61/:80` 默认 bs=512/realTimeRender=false 且 `VitProductionCoordinator.cpp:857-871` 两参数均不设置（301 实测 480→512 走 S1b 路径）。**S1 机理链成立=301 定性精化**：现象学上 WaveShell 特异维持（Surge XT 容错），可行动缺陷面=**host 侧信令不一致**（setup 级 kRealtime vs per-call kOffline），待插桩腿运行时确证；嫌疑分列/301 三 run 吻合度/停止条件评估合格。
- **五项裁定（§7 裁定请求全数处置）**：
  ① **M1+插桩+判别实验合并一次栈占用授权**（同一插桩内核=301 CC9FDACC+VIT309 遥测；候栈排 296-R2/310 之后）；
  ② **修法 B 先行**（VitApp-only 黑名单+realTimeRender=true，M1=RT-SOUND 即落地）+**C 授权为判别实验原型**（§4.2 临时一致化，一次实验同时判别 S1/S2+验证 C；S1 证实且渲染限速不可接受时转正修法）+A 不单用；
  ③ **自检断言=形态 3**（渲染后静音断言+自动降级 realTimeRender 重渲+`degraded_to_realtime_render` 披露+shell 类 hint 日志）；形态 1/2 待命不默认；
  ④ **插桩落点许可**：VIT309 mode 遥测落 vendored JUCE Impl（engine 子模块独立提交，301 处置规则同款）；VIT309_RTLEVEL 落 VitApp（零 engine 改动优先）；
  ⑤ **M1=RT-SILENT 应急预案采纳**：B 降级纯警告+装载取证另立卡。
- 后续腿=M1+插桩+判别（栈队列第 3 位），实现按本卡原目标面（M1 对照/根因运行时确证/修法/自检断言/验证）。

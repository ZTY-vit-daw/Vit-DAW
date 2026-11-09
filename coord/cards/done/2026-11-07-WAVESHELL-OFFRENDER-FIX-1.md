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

- **栈腿领取（M1+插桩+判别+修法 B+形态 3+验证矩阵，2026-11-09）**：续用 worktree D:/Vit_DAW_wt_309recon / 分支 port/waveshell-offrender-fix-1（取证腿 0644ae89 之上）；origin/main=31d0b9d1。领取基线：HEAD=0644ae895bd5212af286c73cf4a8baaa7f33dacb，status 全净，diff --stat 空（worktree 子模块未初始化=b4397498 gitlink；栈腿 engine 改动在本 worktree 子模块内做独立提交，主仓 D:/Vit_DAW/tracktion_engine @ 60ab1a49 仍只读）。形态=五项裁定锁定的合并栈窗口；§8 跑前口径在 RUNBOOK 成文后方可起栈。本腿领取先推 port 分支（实现门），卡面状态同步推 main（coord/cards 允许路径）。

- **回执（栈腿完成 2026-11-09，自验 done 待主管验收）**：工件=`coord/runs/WAVESHELL-OFFRENDER-FIX-1/stackleg_20261109_1906/`（8 run 子目录+evidence.md）+`RUNBOOK-STACKLEG.md` @ port/waveshell-offrender-fix-1（领取 896a18c8→3280352d→61e29e1e→3cee3f42→8bc39aa6→e1873478→eb6cb3d2→回执提交）；栈两条登记=占用 main 5915bb9c（19:06）/释放 main 4711dfd3（19:29，TEARDOWN_ALL_ZERO 19:28:47 亲核，killed VitApp=23644/VitAgent=24724 在案；窗口内 K1a→K1b→K2→K3→K4 owner 亲停亲启）。**零 LLM 兑现**（8 run telemetry 文件全 absent 亲证）。摘要：
  1. **M1=RT-SILENT（判别性结果，裁定⑤激活）**：E1v2 GR 实时播放全窗 -100dB；E1ctrl Surge XT 同 fixture 同装置 **-10.97dB 出声**（对照证实时路径/MIDI 馈入/测量面全健康）→ GR（WaveShell1-VST3 容器）本机四 harness（chat-rack M0a2/M2 直挂/离线渲染/实时播放）一致全零静音——「non-realtime 特异」假说被推翻，静音归属=装载/license/壳层（S2/S3 族，闭源面）。E1v1 无效轮如实记录（engine 快照 levelCache 死代码[setLevelCache 零调用点亲证]致测量假阴性；tap 改 LevelMeasurer::Client 直挂 8bc39aa6 后 v2 有效）。
  2. **S1 信令链运行时确证（卡目标 2，裁定④插桩落地）**：E2a 遥测逐字钉死 S1b 时序——装载期 setupProcessing kRealtime(44100/480)→建图 bs 480→512 变化触发 re-prepare（isNonRealtime 仍 0）→**第二次 setupProcessing kRealtime(44100/512)**→渲染期 firstProcessAudio **perCallMode=kOffline/setupMode=kRealtime** 不一致签名。VIT309 mode 遥测落 vendored JUCE Impl+ExternalPlugin（engine 子模块 cd957f4，独立提交）。
  3. **判别实验完成（裁定②C 面）**：E2b C 原型生效亲证（cPrototype=1/reportedMode=kRealtime 一致化）渲染**仍静音**+M1 实时也静音 → **S1 真实存在但非 GR 静音成因**；C 在本环境对 GR 无效，转正失去该插件面依据——留主管裁定。
  4. **修法 B=⑤形态落地（e1873478，内核 9241f345）**：默认 `warn_only`（检出+WARN 日志+reply `shell_class_plugin_hint`/`realtime_fallback:false`/`blacklist_action` 三披露，渲染照走离线不翻转）；`VIT309_BLACKLIST_REALTIME_FALLBACK` opt-in 保留原 B；`VIT309_DISABLE_BLACKLIST` 全旁路（判别实验）。谓词 fileOrIdentifier containsIgnoreCase(waveshell) 轨+rack 双遍历（E5 rack 检出亲证；E4 Surge 零误伤亲证）。**装载取证另立卡建议交主管**（Waves 授权面/壳层初始化探针，超本卡文件域不扩修）。
  5. **形态 3 全链验证（裁定③）**：E2a/E2b/E3v2/E5 四次触发（VIT309_DEGRADED→同 job realtime 重渲→VIT309_RT_STILL_SILENT→render_done `degraded_to_realtime_render`+`render_silence_observed` 双披露）；Surge（E4）零触发不误伤。
  6. **验证矩阵（卡目标 5）**：M0a2 场景复跑（E5 rack=seg3a_r2 同形）仍静音——RT-SILENT 下「复跑=出声」在本机不可达（非渲染路径缺陷；E4 Surge 同路径 -12.6dBFS 出声=渲染路径健康），⑤预案内预期如实记录，「出声」前置=装载取证另立卡；**M3 Surge 不回退 PASS**（hint/fallback/degraded 全空）；全包回归绿（`go build ./...`+`go test ./...` 全量 exit 0 零 FAIL[§6 五健康包全 ok]+webui vitest 39 文件 471 测试+AB 烟测 exit 0）。
  7. 装置缺陷两枚如实分账（非产品面）：levelCache 死代码（E1v1）/--expect-env 空集传参（E3 首跑 exit1，eb6cb3d2 修复）；§8 对账=单 run 无重跑未破（无效轮+装置修复后新 run 依规记录），止损线未触发，环境中断零。
  8. engine 子模块 `vit309-stackleg`（60ab1a49+cd957f4+9b8cc8a，本地分支未推 engine 远端=301 同款处置，landing/推送交主管）；主仓子模块 gitlink 保持 b4397498 不动；主仓 D:/Vit_DAW/tracktion_engine 全程只读。
  9. **端测边界（AGENTS §5）**：agent ZMQ 面真栈驱动（直挂/rack/实时 transport）+内核日志留痕（VIT301_GUARD/VIT309_*）+wav 物理计量+render_done 披露捕获；不覆盖 webui 渲染面（无该域改动，单测全绿）/Godot 旅程（无该域改动）/chat 全链复跑（与驱动同一 start_render 入口，未另跑付费轮，如需可另派）。

# VST3-SIGNAL-HYGIENE-1：host 侧 setupProcessing/per-call mode 信令一致性（309 S1 卫生缺陷，P3）

- 发卡：GLM 主管决策流 / 2026-11-09（依据=[309 终验](../done/2026-11-07-WAVESHELL-OFFRENDER-FIX-1.md) 裁定③——E2a 运行时确证 host 信令不一致[两次 setupProcessing kRealtime+渲染期 perCallMode=kOffline]真实存在；已证非 GR 静音成因，C 一致化原型[vit309-stackleg 分支 9b8cc8a]失转正依据；属 VST3 规范卫生缺陷，影响面=其他对模式敏感的插件）
- 派发确认：已确认（309 终验裁定③；**P3 低优先，不占展演前窗口**）
- 验收负责人：GLM 主管决策流
- 池序 331；P3 / 0.5-1 天 / 依赖=无（C 原型可参考不强制）；L1-L2 / PC；C++ 取证+小修

## 目标

1. 规范对照：VST3 SDK ivstaudioprocessor.h:138-143（RT↔Prefetch 才可 per-call；切 kOffline 须 setupProcessing）与 tracktion 渲染初始化序（NodeRenderContext setAllPluginsRealtime→prepareToPlay→flush）的偏差钉死成文档锚点；
2. 修法评估：离线渲染路径对全部插件一致信令（setup 级 kOffline 或渲染窗内一致 per-call）——评估 C 原型（9b8cc8a）能否直接转正或需重做；**回归红线=Surge XT/全包渲染不回退（309 E4 同款断言）**；
3. 实施+真栈验证腿候栈（复用 309 装置）。

## 文件域

tracktion_engine 渲染初始化面（vendored JUCE/NodeRenderContext 邻域）+scripts；engine 子模块独立提交（301/309 处置同款）。

## 验收标准

规范锚点文档+修法（或纯登记负结论）+E4 同款回归 PASS+栈协议两条。

## 停止条件

修法引发非 WaveShell 插件回退 → 回退修法转纯登记（缺陷记录在案即可）。

## 领取/回执/验收

- **领取（执行侧 sighyg331，2026-11-20）**：主仓 worktree `D:/Vit_DAW_wt_pc331` + 分支 `pc331-sighyg`（基线=领取时 main HEAD `0e659626`，worktree 全净）；engine 子模块分支 `pc331-sighyg` @ `60ab1a49`（放主模块仓库 `.git/modules/tracktion_engine`——309 的 `vit309-stackleg` 因放 wt_309recon 独立子模块克隆、worktree 删除即失，本卡改放主模块仓库防复发）；主仓 gitlink 保持 `b4397498` 不动（301/309 处置同款）。领取勘察两结论：①**C 原型 9b8cc8a/遥测 cd957f4 本机不可达**（vit309-stackleg 未推远端且 wt_309recon 已删）→目标 2 评估=**直接转正不可行，需重做**，重做方向取 setup 级 kOffline（规范正方向；C 的 per-call 强制 kRealtime 语义不适 offline 且 E2b 已证对 GR 无效）；②**机理精化（前提成立，修法面更准）**：setup 行 `juce_VST3PluginFormatImpl.h:2505` 已是 `isNonRealtime()?kOffline:kRealtime`，缺陷面=守卫 `:2490-2493` 不含模式条件——标志翻转后渲染窗 prepareToPlay 早退，模式感知 setup 永不可达（E2a 签名吻合：装载期 setup kRealtime×2→渲染窗早退→perCall kOffline）。修法定案=守卫加 `lastSetupProcessMode` 模式感知+`setupIO`/`prepareToPlay` 双调用点维护跟踪+`VIT331_SETUP`/`VIT331_SIGNAL` 一次性 log-only 断言面+env 门 `VIT331_CONSISTENT_SETUP`（默认关=生产行为不变，真栈 E4 腿 A/B PASS 后由决策侧翻默认）。**真栈腿=lead 留决策侧**（本卡禁启栈；复用 309 装置与 RUNBOOK-STACKLEG 模式，E4 断言=Surge XT 不回退+VIT331_SIGNAL consistent=1）。与并行卡 440（genapi）文件域不相交。

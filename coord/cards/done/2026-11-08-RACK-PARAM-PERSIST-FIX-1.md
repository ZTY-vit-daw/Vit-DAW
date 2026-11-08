# RACK-PARAM-PERSIST-FIX-1：rack 内插件参数持久化修复（VitApp C++——320 揭出的产品缺陷）

- 发卡：GLM 主管决策流 / 2026-11-08 深夜（依据=[320 续批取证](../blocked/2026-11-08-PLUGIN-CONTROL-EVAL-PROQ3-FREE-1.md)——A/B 判别铁证：rack 内 Pro-Q 3 Band1 Gain 0.7 写入在靶→save/open→回默认 0.5×三次；track 级 Volume&Pan -18.08dB 过重载原样保留=缺陷限定 rack 内插件；[forensic_persistence_20261108 双报告](../runs/PLUGIN-CONTROL-EVAL-PROQ3-FREE-1/forensic_persistence_20261108/)）
- 派发确认：已确认（主管发卡）；验收负责人：GLM 主管决策流
- 池序 321；目标仓库=VitApp C++（保存/写入 flush 面）；P1 **产品缺陷**（不只影响实验：一切 rack 内插件参数调整重载即丢）/ 0.5 天；L2 / PC；零栈单测先行+真栈 probe 验收

## 目标

1. **修复**（主管裁定方向：**写时直 flush 为主+保存路径 rack 遍历兜底，两腿都做**——最小干预与全覆盖兼得）：
   - 写时：`PluginRackControlService.cpp handleSetPluginParam` ~2051-2054 写后 flush 目标 rack 插件（`RackType::flushStateToValueTree()` 在位可用）；
   - 保存路径：`VitHeadlessService.cpp ipcSaveEncryptedProjectToPath` ~1316-1320 / `saveProjectToDefaultXml` ~671-675 保存前 rack 遍历 flush（覆盖非写路径 mutation）；
   - 锚点背景（320 亲读）：`Track::flushStateToValueTree` ~49-53 只级联本轨 pluginList（~277-280），rack 内插件挂 RackType 自身轨表不被触达——修复不改动该语义，在两腿显式补 rack 面；
2. **验收**：320 的 forensic probe 复跑=**rack 内参数过 save/open 存活**（0.7 重载后仍 0.7）+track 级回归维持+全量构建 exit 0+既有持久化/工程测试零回退；
3. **解锁 320**：本卡合入后 320 续批（装置全就绪，一次栈窗口完成 T7×3+对账+用户 GUI 终审交付物）。

## 约束

- 文件域=VitApp 保存/写入 flush 面±测试；不触 tracktion_engine（`RackType::flushStateToValueTree` 在位用）；C++ 构建走 301 同款 cmake 配置；真栈 probe 复用 320 取证脚本（只读复用）；
- §8 跑前成文；插桩留痕保留。

## 停止条件

修复发现 rack 状态树结构不允许外部 flush（需 engine 改动）→取证上交转裁定（扩域须授权）。

## 领取/回执/验收

- 领取：2026-11-08 深夜（承 320 回执二取证窗口） / origin/main=c15d3723d29af3964b79dcc87dc2dcf0a2998efb（fetch 亲证，=发卡要求基线≥c15d3723）/ owner=ZCode GLM-5.3 L2-worker PC 执行子代理（会话 321 rack-param-persist fix，派发已确认）/ 分支=port/rack-param-persist-fix-1 / worktree=D:/Vit_DAW_wt_321persist / 领取提交=coord/321-claim→main。领取基线：fresh worktree 自 origin/main c15d3723，`git status --short`=0 行、`git diff --stat`=空（全净）；tracktion_engine 子模块按 main 指针 b4397498 初始化（本卡零 engine 改动）。在飞卡域核对：doing 八卡——282（catalog/policy/capability_pack agent Go 域）、273（harness agent Go 域）、271（chat agent Go 域）、309（tracktion_engine ExternalPlugin+VitApp 渲染服务面——同 C++ 但与保存/写入 flush 面无文件重叠）、316 LEGAME-DIR（chat 域）、LEGAME-AUTOINTRO（chat/scripts 域）、LEGAME-FRESH（scripts 域）、S1B（工件/编排域）——与本卡（VitApp/Source/Service/PluginRackControlService.cpp+VitHeadlessService.cpp±VitApp 测试）均不相交；320 已 blocked（工件域只读复用）。真栈腿排队：登记前重读 coord/resources/PC-RUNTIME-STACK.md 顶部（任务书示当前空闲，仍按协议净空亲核）。
- 回执（2026-11-09 00:30 收口）：实现 commit 7af6371e+工件收缩 2ffd3b87 @ port/rack-param-persist-fix-1（已推远端）；状态=**执行侧自验完成（doing→done 待验收）**
  - **锚点核实记录（320 机理读数 vs 本卡亲读，出入以代码为准）**：①保存两入口只 Track flush（ipcSaveEncryptedProjectToPath ~1316-1318/saveProjectToDefaultXml ~671-673）——**吻合**；②Track::flushStateToValueTree 只级联本轨 pluginList（tracktion_Track.cpp ~49-53+getAllPlugins=pluginList.getPlugins() ~277-280）——**吻合**；③Edit::flushState 的 getAllPlugins 走 visitAllTracksRecursive（track 树）漏 rack 内——**吻合**（EditUtilities.cpp 亲读）；④RackType::flushStateToValueTree 在位（tracktion_RackType.cpp:566）且 rack_add_node 等 handler 已用同款 rack+track 双 flush 模式——**吻合**；⑤**一处精化**：写时 ~2051-2054 320 记"owner-track flush only"——rack 内插件 getOwnerTrack() 实际返回 **nullptr**（Track::containsPlugin 只查本轨 pluginList），走 **else 分支 plugin->flushPluginStateToValueTree()（本体直 flush 已执行）**；即写时并非"从未 flush"，真正缺口=flush 拉取的 VST3 chunk 对 host 侧刚写参数的吸收依插件而定（juce wrapper getState 前 parameterDispatcher.flush+IComponent/IEditController 双 getState 亲读，仍不保证插件实现吸收），且 rack 面无补偿通道——修复据此加双保险；
  - **修复（卡面两腿+β 对冲，零 engine 改动）**：腿 1 写时（handleSetPluginParam）——新增 findRackTypeContainingPlugin（edit.getRackList().getTypes() 遍历），rack 内插件补 rack->flushStateToValueTree()（原 if/else 分支保留=track 级行为零变化）+外挂显式参数 override 写入（新 VitPluginParamPersist.h：engine "parameters" 属性线格式逐字兼容[writeString+writeFloat 对]，upsert 替换或追加；重载链 ExternalPlugin::completePluginInstanceCreation 先 chunk 后 restoreChangedParametersFromState——**重放必胜 chunk** 时序亲读）；腿 2 保存路径——两入口 Track flush 后加 rack 遍历 flush；
  - **diff 文件清单**：VitApp/Source/Service/PluginRackControlService.cpp（+33）/VitHeadlessService.cpp（+14）/VitPluginParamPersist.h（新 133）/Tests/CMakeLists.txt（+18）/Tests/RackParamPersistOverrideTests.cpp（新 183）+coord/runs/RACK-PARAM-PERSIST-FIX-1/（RUNBOOK §8+probe 三件逐字复用 sha256 钉死+四报告+placement+stack_receipt+日志工件[gzip 全量+窄口径相关段]）；tracktion_engine 零改动（b4397498 原版树 export，构建用）；
  - **测试命令与退出码**：全量 Release 构建（cmake -S VitApp -B VitApp/build_release -G "Visual Studio 18 2026"+cmake --build --config Release）**exit 0**；VitRackParamPersistOverrideTests（engine 线格式兼容/upsert 语义/撕裂流防御/ValueTree 往返）构建+运行 **exit 0 all checks passed**；
  - **真栈 probe 验收（run=stack_persist_probe；§8 冻结于 RUNBOOK，probe 零 LLM 零计费，计费上界 ¥0）**：栈协议全链（净空亲核 00:15 全零→占用登记 0e2eba76→≤60s 复核全零→起栈→拆栈→**TEARDOWN_ALL_ZERO 亲核 00:22:18**→释放登记 8ae87693 远端亲证）；布位 r1（320 driver 复用，fixture digest=5080116bec1afcfa 与 320 run3 初始态全等）；probe×4=**run1 无效轮**（F5 装置环境类：起栈 agent 未带 BAREFREE 缝 env，EQ 治理守卫拦裸写[typed_eq_control_required]，A 臂写入即拒未进持久化测量；非同型断点，处置=带 env 重启 agent[进程 env psutil 亲证]后继续）+**run2/3/4 三次有效测量全 PASS：A 臂 rack 内 Pro-Q 3 Band1 Gain 0.7→save→open→回读 0.699999988079071（persisted=true×3=三次重载稳定；对照 320 基线同 probe 未修复三次全回 0.5）+B 臂 track 级 Volume&Pan -18.08dB 回归维持（persisted=true×3）+每轮 restore 复原 digest 全等**；probe discriminator.shape 落 "unexplained"=两者都过（脚本缺陷形态分类语义，验收以 persisted 布尔为准——RUNBOOK §二 预声明）；布位副本已清除（projects 域=ledger+手测·小星星乐局.vit 卡前基态亲核）；内核 VitApp.exe SHA256=950cf07b7b735749c2def5e9447b7260f1272694dc5b83a865790b8e24cd30f5；
  - **域外记录（不扩修）**：dev_agent_smoke.ps1 起栈不传递调用方 PS 会话 env 至 agent 进程（-RestartAgent 链），缝类实验须从带 env 的 shell 直接启动 agent 或修脚本——scripts 域，交主管裁量；保存路径其余 5 处 Track flush 调用点（~186/~232/~1389/~1445/~1641，load 前快照/其他路径）rack 面同型缺口未在本卡修（卡面只锚两保存入口）——如主管认为需全覆盖可另开小卡；
  - **端测边界声明**：本卡验收=agent HTTP/ZMQ 面持久化 probe（零 LLM），无渲染面/用户旅程主张；「**320 解锁条件已备**」——本卡合入后 320 可续批（装置全就绪+selftest 25/25 在分支，布位→device→T7×3 一次栈窗口可完成）。
- 验收：（主管回填）

### 终验（主管，2026-11-09 凌晨）

- **pass 整卡收口+合入 main**——probe 报告主管直读亲证（arm_a rack 内 Pro-Q 3 value_after_reopen=0.699999988 persisted:true ×3 有效测量；对照 320 未修基线三次全回 0.5；arm_b track 级回归维持）；锚点核实的一处精化（写时 else 分支本体直 flush 已执行、真缺口=VST3 chunk 吸收依插件而定+rack 面无补偿通道）采信并记录——修复设计（override 重放必胜时序）与之自洽；全量 Release 构建 exit 0+新单测全过；栈协议全链（0e2eba76→8ae87693）零 LLM。
- **域外两记录处置**：①-RestartAgent env 不透传=scripts 域记录候卡（缝类实验影响面小，暂不开卡挂 PENDING）；②其余 5 处 Track flush 同型缺口=后续同型修复小卡候排（非本卡扩域）。
- **320 解锁**：装置全就绪，续批跑 T7×3 终章。

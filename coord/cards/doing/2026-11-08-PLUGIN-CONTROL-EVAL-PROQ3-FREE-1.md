# PLUGIN-CONTROL-EVAL-PROQ3-FREE-1：T7 自由调控 Pro-Q 3（早期首碰壁目标的终审——自选+自报+GUI 实物对账）

- 发卡：GLM 主管决策流 / 2026-11-08 深夜（依据=[用户裁定 2026-11-08 深夜](../../decisions/2026-11-08-product-style-handtest-ruling.md)：加调控 Pro-Q 3 目标——**早期开发第一个碰壁处**；自行选择调控但必须说清频段/频率/Q 值/增益量/滤波器类型；**保留调控后插件状态供用户 GUI 亲验**——"如果对得上那就厉害了"）
- 派发确认：已确认（用户裁定+主管发卡）；验收负责人：GLM 主管决策流+**用户 GUI 终审**
- 池序 320；目标仓库=主仓 scripts/工件；P1 / 0.5 天 / 依赖=318 缝已合入；L2 / PC；真栈一次（**排 319 后**）

## 目标

1. **fixture**：工程副本布位（M2-PREP 同模式，用户可从启动页打开）：一轨真实音频素材+挂载 **FabFilter Pro-Q 3**（plugin registry 亲证可用；§10 副本制零权威污染）；工程名「实验·ProQ3 裸控」；
2. **任务话术（开放自选+强制自报）**：「请用这条轨上的 Pro-Q 3 做你认为有必要的均衡处理。调整完必须说明：动了哪个/哪些频段，每段的频率、Q 值、增益量、滤波器类型（钟形/搁架/高通/低通等），以及为什么。」——模型自行决定调什么；
3. **run 配置**：BAREFREE 面（SEAM=1+守卫许可缝全开）×3 轮（3 重复）；每轮独立工程副本、终态保留（三轮=三个可开工程副本，或同工程三轮快照——以「用户至少能打开一个看 GUI」为底线）；
4. **机器侧证据**：每轮 post-run readback（get_plugin_parameters 全量）+模型自报告全文落工件；主管对账（自报 vs readback vs 初始态 diff——哪些参数真动了）；
5. **用户终审交付**：工程路径+三轮自报告+readback 对照表——用户从 Godot 启动页打开工程→机架→Pro-Q 3 GUI→**肉眼比对频段/频率/Q/增益/类型是否与模型所述一致**；
6. 判定三态：GUI 与自报一致（含近似取整）=**裸控终审通过**；不一致=取证（哪层错：识别/编码/自报诚实度）；环境中断按 §9。

## 约束

- 文件域=工件+卡面+布位脚本扩展+驱动（T7 单案驱动，六案例体系外新 case 文件）；agent 源码零改动（缝全在 main）；§8 跑前成文（3 轮 ≈¥1；失败分类含「自报与实际不符」新类——诚实度观察项）；
- 真栈协议全链；栈排队 319 后。

## 停止条件

Pro-Q 3 不在 registry/无法装载→取证上交（换 EQ 插件须用户裁定）；缝泄露停段。

## 领取/回执/验收

- 领取：2026-11-08 22:56 +0800 / origin/main=1883021b4ee0383c6b00e0c2223546be230f969d（fetch 亲证，=发卡要求基线；含 318 缝与全套工件）/ owner=ZCode GLM-5.3 L2-worker PC 执行子代理（会话 320 proq3 free runner，派发已确认）/ 分支=port/plugin-control-eval-proq3-free-1 / worktree=D:/Vit_DAW_wt_320proq3 / 领取提交=coord/320-claim→main。领取基线：fresh worktree 自 origin/main 1883021b，`git status --short`=0 行、`git diff --stat`=空（全净）。在飞卡域核对：doing 九卡（319 BAREFREE-T1=harness.go 守卫区+T1 工件域、EDU-ASK-ANCHOR-1、ARRLOOP-AUTHBRIDGE-1、CLOSURE-ROUTE-ADAPT-1、WAVESHELL-OFFRENDER-FIX-1=C++ 域、LEGAME 三卡、S1B=工件目录+编排+tools 缝域）与本卡（coord/runs/PLUGIN-CONTROL-EVAL-PROQ3-FREE-1/ 工件目录 + 本卡面 + %APPDATA%\Vit-DAW\projects\ 三工程布位[用户数据域持久位，311 先例] + 冻结 runner 只读 import）不相交；agent 源码零改动（卡面约束，缝全在 main 亲证：318 双缝在 origin/main 1883021b）。Pro-Q 3 registry 在案预核：identifier=VST3-Pro-Q 3-a1aba170-ba5f7ab2、path=C:\Program Files\Common Files\VST3\FabFilter\FabFilter Pro-Q 3.vst3（FE-LANE-VIEW-TUNING-1 BridgeReplies plugin_list 亲读；活栈装载亲证留装置门）。栈排队 319 之后：登记前重读 coord/resources/PC-RUNTIME-STACK.md 顶部，等待不抢。
- 回执：commit 3266fb8e @ port/plugin-control-eval-proq3-free-1（装置预冻结 7fd83995+修复#1 fe825a5b+修复#2 3266fb8e+run1/run2 取证工件全量在分支）；状态=**批止损上交（doing→blocked）——候主管授权续批**
  - **装置就绪**：T7 单案四件（cases/driver/orchestrator/RUNBOOK §8 冻结）——话术卡面原文逐字；Pro-Q 3 registry 预核行；driver selftest 25/25+ps1 PARSE_OK；**agent 源码零改动亲证**（分支 diff 仅 coord/runs 工件+卡面）；
  - **run1/run2 均装置腿 exit 4 停段（未进付费轮）**：registry 门两跑 PASS=**Pro-Q 3 装载亲证在卷**（plugin_search→冻结 identifier→.vst3 路径+rack_add_node 接受+save/open 往返）——卡面停止条件（无法装载）未触发；两次失败均 harness_invalid=runner 状态形状假设错（#1 plugins[] 含内建 volume/level 行；#2 外挂实例在 track.rack.nodes[]，kernel 源码锚点 CommandDispatcher.cpp ~1797/~3754 亲读）；**零 LLM/零计费**（失败点均在零 LLM 装置腿）；
  - **止损依据**：RUNBOOK §四 预冻结「环境重建上限 1 次/批（含装置脚本缺陷类，312/318 先例）」——run2 即重建#1 已耗尽；§8 纪律禁止批内自改额度，故 run3 不自行启动；
  - **上交点（候主管授权）**：修复#2 已提交分支待用；授权内容=新一批 §8 冻结（新 run 目录+device 门+三轮 T7；装置与预算无需改，预计仍 ≈¥1/批）；半布位 r1.vit 已清除（%APPDATA% projects 域回归卡前状态亲核）；
  - **栈协议全链**：排队 319 后（319 占用 23:06→释放 23:15 亲证在卷）→净空亲核 23:16:35 全零→占用登记 bb952a03→≤60s 复核 23:17:57 全零→run×2→owner 亲拆 **TEARDOWN_ALL_ZERO 23:26:49 亲核**→释放登记（见栈记录本日 23:27 条）；
  - **端测边界声明**：两 run 均未达付费轮——无任务轮数据、无渲染面/用户旅程主张；用户 GUI 终审物（三工程+自报告+对账表）未产生（候授权续批后交付）。
- 回执二（续批 b2，2026-11-08 23:41 收口）：取证工件随 port/plugin-control-eval-proq3-free-1 最新提交+释放登记 4591eabf；状态=**卡面前提被产品缺陷推翻→blocked 上交（doing→blocked）**
  - **续批结果**：run3 proq3_free_20261108_233122 device 五门前四门全 PASS（registry 亲证+rack.nodes 布位 r1/r2/r3 全成功[修复#2 生效]+启动页 recent 亲证+初始态等价 digest 全等+known-op 裸写 0.7 落地回读在靶=BAREWRITE 缝在 Pro-Q 3 面活体亲证）；**败在持久化门（第三型，非装置脚本缺陷）**：Pro-Q 3 Band 1 Gain 活体 0.7→save_project→open_project→回 0.5；
  - **零 LLM 判别 probe（占用窗口内，forensic_persistence_20261108/ 两跑）**：A 臂 rack 内 Pro-Q 3 三次重载全回默认 vs **B 臂 track 级 Volume&Pan -18.08 dB 过重载保留——A/B 分离铁证**；两跑均复原 r1 至初始 digest 亲证；
  - **机理（源码锚点，C++ 域）**：ipcSaveEncryptedProjectToPath（VitApp/Source/Service/VitHeadlessService.cpp ~1316-1320）+saveProjectToDefaultXml（~671-675）保存前只 track 级 flush；tracktion_Track.cpp ~49-53 flush 只级联本轨 pluginList（~277-280 getAllPlugins=pluginList.getPlugins()），rack 内插件挂 RackType 自身轨表不触达；PluginRackControlService.cpp ~2051-2054 写时同只 owner track flush；**Edit::flushState() 的 getAllPlugins 走 edit trackList 亦漏 rack 内——修复须 rack 感知（RackType::flushStateToValueTree 在位可用）**；修复候选=写时直 flush 目标插件 和/或 保存路径 rack 遍历 flush——**归主管裁定另开 C++ 卡**（本卡文件域不含 VitApp，不扩修）；
  - **为何付费轮不启动**：卡面核心交付=「终态保留供用户 GUI 亲验」——模型调完参用户打开工程只会看到默认值（缺陷实证），付费轮的钱会花在不可交付的终态上；持久化门的设计目的即在付费前拦住此情形；
  - **栈协议（续批）**：占用 a3a4af22（净空亲核 23:30:36+≤60s 复核 23:31:17）→run3+取证 probe 腿→owner 亲拆 **TEARDOWN_ALL_ZERO 23:39:51**→释放 4591eabf（栈记录 b2 占用/释放两条在卷）；三布位工程已清除（projects 域=ledger+手测·小星星乐局.vit 亲核）；
  - **预算**：三 run+probe 全程零 LLM 零计费（失败均在零 LLM 腿）；≈¥1 批预算未动；
  - **端测边界**：无任务轮数据/无渲染面/无用户旅程主张；GUI 终审物未产生；
  - **解锁条件（候主管）**：C++ 持久化缺陷修复合入后，本卡装置全部就绪可直接续（cases/driver/orchestrator/RUNBOOK 在分支，selftest 25/25；布位→device→T7×3→对账表全链一次栈窗口可完成）。
- 回执三（终章批，2026-11-09 00:42 收口）：修复#3 已提交分支+释放登记 2c7ef6db；状态=**终章批两 run 后再停（doing→blocked 第三停）——候第四批授权**
  - **run4 proq3_free_20261109_003004**：持久化门假阴性=**本侧装置门跨工程测量缺陷**（布位循环尾 active=r3，known-op/持久化持 r1 id 写——两副本命令序列相同→id 同名巧合命中 r3 实例——随后 reopen r1「验证」=写 r3 验 r1；321 修复本身与此无关）；修复=门先 open 目标工程再写（321 probe 正确流对齐）；
  - **run5 proq3_free_20261109_003454（终章批重建#1）**：**装置六门全 TRUE**——registry+布位 r1/r2/r3+启动页 recent+初始态等价 digest 全等+known-op 裸写 0.7 落地+**持久化门 PASS（Band1 Gain 0.7→save→open→0.699999988079071 persisted:true，restore 后 digest 回初始 5080116bec1afcfa）=321 修复经修正装置门端到端亲证**；随后 T7 r1 崩于付费会话启动前——open 后参数面异步重建竞态（agent invoke status=ok 但 0 参数行→runner RuntimeError 于 try 外→PS EAP=Stop 把 python stderr 升级 NativeCommandError 终止整批；agent 日志止于 00:35:03 get_plugin_parameters ok+rounds/r1 空，取证在卷）；
  - **零 LLM 亲证**：telemetry 文件不存在（崩点在首次 chat 前）——DeepSeek 零调用零计费，≈¥1 预算未动；
  - **停批依据**：终章指令第 5 条（付费段装置失败=取证上交）+终章批重建额度耗尽（run5=重建#1，§四冻结）；
  - **修复#3（已提交分支候用）**：runner 5 处读面改 wait_agent_parameters 轮询（布位门同款，消除 open 后异步竞态）+编排 3 个 python 调用点 EAP=Continue 守卫（traceback 入 Tee 日志不再杀批）；selftest 25/25+PARSE_OK；
  - **栈协议（终章批）**：占用 4dce706f（净空 00:29:24+≤60s 复核 00:30:00+psutil env 亲证两键=1）→run4/run5→owner 亲拆 **TEARDOWN_ALL_ZERO 00:41:33**→释放 2c7ef6db；三布位工程已清除（projects 域亲核）；
  - **第四批距离（候主管授权）**：唯一剩余=两 runner 侧竞态修复（已在分支）后的重跑——321 修复+装置六门+T7 话术/判据/预算全就绪；一次栈窗口即可完成 device→T7×3→对账表→用户 GUI 终审交付物。
- 验收：（主管回填+用户 GUI 终审）

### 续批授权（主管，2026-11-08 深夜）

- **新批授权**：批止损处置合规（预冻结上限 1 次耗尽即停+不自改额度+环境重建先例 312/318 同模式）；两缺陷=装置脚本形状误读（plugins[] 内建行/rack.nodes 真实位置），修复#2 已提交分支 3266fb8e+selftest 25/25 维持+kernel 源码锚点在卷——**非同型断点原样重跑**（run1≠run2 缺陷相异且各有修复），新批成立。
- 新批范围：新 run 目录+device 门复跑+T7×3 付费轮+对账表+用户终审交付物；预算 ≈¥1 沿用；若新批再遇装置腿同型失败→再止损上交（不再续批）。
- 卡回 doing 续跑。

### 第四批授权（主管，2026-11-09 凌晨 00:50）

- **授权成立**：三停三缺陷相异且各有修复（#1 plugins[] 误读/#2 rack.nodes 定位/#3 参数面异步竞态轮询化+EAP 守卫），run5 装置六门全绿=321 修复端到端亲证已落袋；付费轮零计费保护性停批全对。修复#3（d67bed9a）就绪，第四批=终章收官批。
- **夜停线（新设）**：第四批若再遇任何装置腿失败→当夜收工不再续批，缺陷取证入卷明晨处理（连续四批装置层失败=夜间疲劳信号）。

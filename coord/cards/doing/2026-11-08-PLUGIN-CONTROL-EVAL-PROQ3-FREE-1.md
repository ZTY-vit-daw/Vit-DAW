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
- 回执：（执行侧回填）
- 验收：（主管回填+用户 GUI 终审）

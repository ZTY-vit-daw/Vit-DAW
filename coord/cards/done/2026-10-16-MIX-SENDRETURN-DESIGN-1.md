# MIX-SENDRETURN-DESIGN-1：send/return 轨间路由能力设计——内核暴露+agent 语义+观察面+机架 GUI

- 发卡：GLM 主管决策侧 / 2026-10-16 晚窗（依据=用户质询「混响和延迟很多时候通过 bus 轨道发送，有没有考虑到？打算怎么解决？」+ 同晚四项裁定）
- 派发确认：**已确认（设计腿——用户 2026-10-16 晚窗「我们现在先设计，设计好再开动」）**；实现卡须待设计件评审后另确认
- 验收负责人：GLM 主管决策流
- 池序 124；目标仓库=D:\Vit_DAW（设计件）+前端仓（GUI 交互规格）；P1（用户直接需求）/ 取证+设计 1-2 天
- 模型分级：L2 / GLM 执行会话（跨内核+agent+前端三面设计）

## 用户核心设计思想（2026-10-16 晚窗口述，设计前提）

**混合轨道模型：不设专门 bus 轨类型。** 我们的轨道本来就是音频/MIDI/bus 三合一——用户想挂 bus 混响发送时，操作逻辑=**新建一条轨、加载混响、发送回目标轨道**；这串接线在**机架视窗有列表标识，用户可手动接回**。

## 四项裁定（用户 2026-10-16 晚窗 AskUserQuestion 亲答，设计硬输入）

1. **湿信号回流=回目标轨**：干湿在目标轨内混合（目标轨 solo=干+湿一起，避开传统 DAW solo 掉混响痛点）；多轨共享时湿声=全体发送源的混合（共享空间本质）；**回线不得再参与该轨 send（防环）**；
2. **发送量控制=目标轨侧为主**（传统通道条 send 槽位+量；用户亲认传统习惯如此）；FX 轨侧留只读列表；
3. **必须多轨共享**：一条 FX 轨同时服务多条目标轨、各轨独立发送量——共享空间话术与 MOM 轨间关系观察依赖它；
4. **GUI=拖拽画线，FX 轨 icon 与效果器插件同级**：新建的 FX 轨在 GUI 中成为**与效果器插件同级的 icon**，与插件同构地**被接线在 rack 间**——即 send/return 连线是机架图里的一等连线（不是隐藏在轨道设置里的参数）。

## 现状实锤（2026-10-16 晚窗决策侧勘察）

1. **agent 语义层零 send/return 词汇**：`agent/internal` 全域无 send_amount/send_level/aux_bus/fx_bus 命中；
2. **内核命令面零 send 命令**：CommandDispatcher 406 命令字符串中路由类仅 `routing_bus`/`routing_bus_enabled`/`set_folder_routing_bus`（文件夹轨子混音，TrackService.cpp:669-703）+ `output_routing_name`；无源轨→轨发送量命令；
3. **现状行为=insert 直挂**：M2 第四轮实况，混响直接装载 vocals(1032) 机架 Z3，无 send/return 路径可选；
4. **底座原语在场**：tracktion AuxSendPlugin/AuxReturnPlugin——`plugins/internal/tracktion_AuxSend.cpp:17-26`（busNum+`send level` 0..1）、`AudioTrack::getAuxSendPlugin`（tracktion_AudioTrack.h:62）、`edit.getAuxBusName`；**但其语义=编号总线+回主输出，与裁定 1「回目标轨」+裁定 3「多目标」的匹配度待实证**（轨到轨输出路由是否单目标是关键取证点）；
5. 用户领域事实：insert 直挂每轨一实例、干湿比只能在插件内调、无法多轨共享空间；发送量本质是**轨间关系**，与 MOM 混音关系模型同域。

## 设计范围（四腿，一件设计件交付）

1. **内核暴露面**：send 连接命令（源轨→FX 轨+发送量+回流目标）、FX 轨即普通混合轨（零新轨类型）、**实现 seam 裁定：tracktion AuxSend/AuxReturn 原语 vs VitApp graph 层自实现轨间连线**（裁定 4「连线=rack 图一等公民」暗示后者与 GUI 更同构）；轨到轨多目标回流能力实证；防环约束；send 拓扑读回命令；VSP/CAS/幂等沿用既有体系；
2. **agent daw 适配+编排器拓扑语义**：send/return 命令封装；话术「给某轨加混响/延迟」的拓扑决策（多轨共享空间→send 结构+向用户呈现回路、独占色彩→insert）、意图确认面；
3. **观察投影面**：TOM/MOM 看 send 拓扑（轨→FX 轨→回流+发送量）作为轨间关系证据；与 FXM 同路由对比、TIM 路由完整性断言衔接；
4. **机架 GUI（Godot 前端）**：FX 轨 icon 与插件 icon 同级的机架图渲染、rack 间拖拽画线交互、连接列表标识（目标轨侧 send 槽位+量、FX 轨侧只读）、手动接回（断开/重连/调量）；与 FE-GUI-RACK 系列既有机架面板的整合。

## 文件域

本卡=取证+设计件（只读+coord/ 工件+前端仓交互规格文档）；实现卡另立（届时按设计件定域领取）。

## 验收标准

- 设计件（coord/runs/MIX-SENDRETURN-DESIGN-1/）：四腿逐项方案+内核 seam 锚点+命令 schema+拓扑决策表+投影字段+GUI 交互规格+烟测场景+实现拆卡建议（含内核/agent/前端三线排期）；
- 底座可达性实证：worktree 内核最小实验（建 send 连接→设量→回流目标轨→读回/渲染验证）或等效源码级证明，二选一并声明边界；**轨到轨多目标回流必须实证**（裁定 3 的硬前提）；
- 设计件过决策侧评审+用户过目 GUI 交互规格后，方可拆实现卡。

## 停止条件

- 取证发现轨到轨多目标回流在引擎与 graph 封装层均不可达 → 上交裁定替代路径（多 FX 轨折衷/自实现 send 节点成本评估），不擅自定架构；
- 涉及渲染路径（audio graph）行为不确定 → 渲染取证拆卡先行，设计件如实登记边界。

- 领取：2026-10-16 18:52 CST / origin/main=5428366213b5e857ab0b1d81302d0fb5b5ef2715 / owner=GLM-5.3 L2 PC 执行会话（ZCode，本会话）/ 分支 coord/mixsend-design-claim（设计卡零生产代码：设计件+回执同分支，无实现分支）/ worktree D:/Vit_DAW_wt_mixsend_design / 领取提交=本提交（卡面入池+领取同批；入池提交为发卡侧留盘原样）
- 领取时核对：卡在 todo（发卡侧留盘未推，本批一并入池，内容零改动）、派发已确认（用户 2026-10-16 晚窗「先设计」）；在飞 doing 两卡——LOUDNESS-B-TRIGGER（agent 内部 port/loudness-watch-trigger-1 + D:/Vit_DAW_wt_loudness_watch_b）、FE-CEF-PASTE-SHIM-2（前端仓），与本卡文件域（只读取证 + coord/runs/MIX-SENDRETURN-DESIGN-1/ + 前端仓交互规格文档）零交集；真栈不占用（headless 取证优先，如需占栈先核对 PC-RUNTIME-STACK 并登记）
- 回执：设计件=coord/runs/MIX-SENDRETURN-DESIGN-1/README.md（四腿方案+seam 裁定=tracktion 原语双 bus 三明治+命令 schema 5 条+话术拓扑决策表+MOM/TOM/FXM/TIM 投影+GUI 交互规格+烟测场景+K1/K2/K3 拆卡排期）；底座实证=experiment/RESULTS.md（AuxMultiReturnProbe **VERDICT PASS exit 0**：轨到轨多目标回流 2.0001× 精确实证——全接线双目标回授 vs 单回授退化 1.5001× vs 无接线 1.0× 三态可分；静音联动抑制归零；乱序防环有界不发散）；边界声明=探针直接驱动 tracktion_engine 原语（与 VitApp 同源同建图路径，不经 VitApp 命令面——命令面正是 K1 要补的），端到端能力归实现卡真栈烟测；零生产代码改动，真栈零占用（独立 console 进程零端口）；开放问题 4 项（solo 语义/FX 轨双出/旧内核反灌/bus 号段）上交 README §7；[等待用户:GUI 交互规格过目（README §4）+决策侧评审后方可拆实现卡]
- 验收：（裁定文件 / 验收 commit）

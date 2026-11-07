# KERNEL-OFFRENDER-FORENSICS-1：内核离线渲染数字静音取证（277 seg3a_r2 转卡）

- 发卡：GLM 主管决策流 / 2026-11-07 早窗（依据=[277 段 3a-r2 取证](../done/2026-11-05-ARR-SRCLOAD-1.md)+工件 coord/runs/ARR-SRCLOAD-1/seg3a_r2_20261107_1020/——装载链全实证后 start_render→render_done 产物 8.0s **数字静音**（1,411,200 数据字节全零，python wave 独立核验 peak/rms=0/-120dBFS）；活栈只读探针已排除：轨 mute/solo/fader/输出解析全干净、rack 图模型 MIDI/音频针脚全接（edges 含 RACK_INPUT|0→1013|0+L/R→RACK_OUTPUT、audio_reachable_from_rack_input=true）——「接线完整」与「渲染全零」并存）
- 派发确认：已确认（主管发卡）；验收负责人：GLM 主管决策流
- 池序 297；目标仓库=VitApp 内核（C++ 离线渲染执行路径）；P1 / 1 天 / 依赖=seg3a_r2 工件在库；L2 / PC；零栈先行（源码级取证），复现腿需真栈一次（可复用 arr_srcload 驱动器，候栈登记）

## 三候选（取证起点，锚点在卷）

1. **te::EditRenderer 离线图 MIDI 馈入**（TransportAudioService.cpp:1185-1223）：离线渲染时 MIDI clip 是否馈入 VSTi（活栈探针只证图连通，未证离线路径取数）；
2. **VSTi 离线处理**（VitProductionCoordinator.cpp:811/62-89）：离线渲染模式下插件 process 调用链（prepared/active 状态、block 馈送）；
3. **合路面**（PluginRackControlService.cpp:3530-3655）：rack 输出→总线的离线混合阶段静音丢弃。

## 目标（取证卡）

1. 源码级定位三候选中哪个（或组合）导致全零；给出确定性锚点+机理链（对照 seg3a_r2 工件：journal revision=4 的 rack/clip 状态、wav、活栈探针输出）；
2. 最小复现设计（可在真栈上用 arr_srcload 驱动器复现的场景变体——如仅 MIDI 轨+VSTi 渲染最小集）；
3. 产出最小修建议（不实现）或「设计内限制」结论（如离线渲染本就不支持 VSTi MIDI 馈入=能力边界，需如实声明）——三选一钉死，交主管裁定；
4. 顺带查证 PD-7（journal→.vit 主文件回写时机：rack/clip 仅落 journal 未回写主文件是否 by-design）。

## 约束

- 文件域=coord/runs/KERNEL-OFFRENDER-FORENSICS-1/ 工件+报告；**源码零改动**（取证卡）；
- 真栈复现腿如需：登记协议照旧，候栈排队（296 亦在队列）；
- seg3a_r2 工件只读。

## 停止条件

证据不足以区分三候选→并陈上交；发现渲染路径在案裁定约束（如既有离线渲染设计文档/裁定）→引用上交不越裁。

## 领取/回执/验收

- 领取：2026-11-07 / origin/main=a93dd97cefda1ac16ead33b6d7e9645ee8947605（worktree 领取基线全净，status 空、无 diff）/ owner=GLM L2-worker 子代理(ZCode/PC，会话 KERNEL-OFFRENDER-FORENSICS-1) / 分支 port/kernel-offrender-forensics-1 + 协调分支 coord/297-claim / worktree D:/Vit_DAW_wt_297forensic / 零栈零 LLM（源码级取证，无栈占用需求）
- 回执：（**执行侧自验完成 2026-11-07 晚窗**；工件 commit=`port/kernel-offrender-forensics-1`（报告 coord/runs/KERNEL-OFFRENDER-FORENSICS-1/report.md，已推 origin）；零栈零 LLM 零源码改动零计费，无 run ID。**结论=③证据不足并陈上交**：候选1（MIDI 馈入）源码语义层结构排除度最高——离线图与实时图同一 EditNodeBuilder（forRendering 只跳 live-input/freeze 旁路），MIDI 透传九环节贯通（TrackMutingNode:156/RackInstanceNode:105 "Always copy MIDI"/SendNode:517-523/ReturnNode:651/ConnectedNode:247-248/SummingNode:188），rack 连接（RackType.cpp:528-529 MIDI pin0↔插件）与活栈探针 edges 同源同形，渲染 player prepareToPlay 内跑 transform（ReturnNode 总线连接必建）；「离线不馈 VSTi MIDI=设计边界」表述不成立。候选2（VSTi 离线处理）=**唯一不可排除**：ExternalPlugin::applyToBuffer 两道 fail-silent 守卫（tracktion_ExternalPlugin.cpp:1338/1341，无日志 return）在「渲染时实例未加载/未 prepare」下产生与 seg3a_r2 全部观测（图完整+无错误+render_done+8.0s 全零）逐项吻合的结果；渲染循环非 realtime（setNonRealtime(true)）+独立 prepareToPlay 的插件特异行为为次要保留。候选3（合路面）排除（轨筛选两门探针实证过+顶层链无静默丢轨分支）。引擎树定位=wt_228 CMakeCache VIT_TRACKTION_ENGINE_DIR→D:/Vit_DAW/tracktion_engine（te 快照 b4397498 零改动）。上游佐证=Renderer.test.cpp 仅音频轨渲染测试，MIDI+rack+VSTi 离线渲染无上游覆盖，Vit-DAW 先例（RENDERGAIN 八腿/GEN 域）全音频源——本路径首次触达。**最小复现设计 M0-M3**（报告 §3，只设计未执行）：M0 复现基线（复刻 chat）→M2 rack 二分（instantiate_plugin 直挂轨，CommandDispatcher.cpp:3365 在场）→M3 插件特异性（换 Surge XT）→M1 实时对照（277 未验证的盲区）；M1-M3 免 LLM。**PD-7 钉死=by-design**：B15 产品裁定 2026-09-11（VitWorkingCopyPersistPolicy.h:9-44「agent 治理变更只进版本仓库不推进工作副本」）+runAutoPersist defer 语义（CommandDispatcher.cpp:2312-2329 注释明言 intended semantics）——rack/clip 经 VSP envelope→defer→只落 journal revision=4，主 .vit 停在 save_as_project 显式直写，观测与设计逐项一致；重开丢 rack/clip 的恢复入口=Project History（叠加 FIX-START-STATE-RECOVERY-1「不恢复」裁定自洽），用户可感知面是否增强交主管裁量。**若复现腿证实守卫命中→缺陷定性建议=物化缺陷**（fail-silent 违反 §5.1 留痕口径，最小修=守卫加一次性留痕+渲染路径实例状态断言，修复卡另立）。交主管裁定）
- 验收：（主管回填）

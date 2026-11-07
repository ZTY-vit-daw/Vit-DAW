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

- （执行侧回填）/（执行侧回填）/（主管回填）

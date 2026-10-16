# 底座实证记录：AuxMultiReturnProbe（卡 MIX-SENDRETURN-DESIGN-1 硬验收腿）

- 运行时间：2026-10-16 19:5x CST（晚窗）
- verdict：**PASS（exit 0，failures=0）**，完整日志见 [probe_run.log](probe_run.log)
- 探针源码：[AuxMultiReturnProbe.cpp](AuxMultiReturnProbe.cpp)；构建脚本：[CMakeLists.txt](CMakeLists.txt)；构建日志：[build.log](build.log)

## 实验目的

实证 tracktion `AuxSendPlugin`/`AuxReturnPlugin` 原语在「轨到轨**多目标回流**」拓扑下的真实音频行为
（卡面验收标准：worktree 内核最小实验；裁定 3 的硬前提）。

## 拓扑

```
T0 (targetA, sin 440Hz): [AuxSend(bus0, 0dB)] -> [AuxReturn(bus1)] -> vol(-12dB) -> master
T1 (targetB, sin 880Hz): 同上
FX (无 clip):            [AuxReturn(bus0)] -> [AuxSend(bus1, -6.02dB≈0.5x)] -> vol(-96dB) -> master
```

湿信号 = 全体发送源之和（bus0 求和）× 0.5，广播回**每一条**目标轨（bus1 广播）。

## 五腿结果（Goertzel 幅度，定标 R=0.25119≈decibelsToGain(-12dB)，容差 5%）

| 腿 | 配置 | amp440 | amp880 | 比率 | 断言 |
|---|---|---|---|---|---|
| L1 control | 无 aux 接线 | 0.25119 | 0.25119 | 1.0× | 定标 |
| **L2 full** | 全接线（双目标回授） | **0.50239** | **0.50239** | **2.0001×** | **PASS（核心）** |
| L3 single_return | 仅 T0 回授 bus1 | 0.37680 | 0.37680 | 1.5001× | PASS |
| L4 mute_T0 | 全接线 + T0 静音 | 0.00000 | 0.37680 | 440≈0 / 880=1.5001× | PASS |
| L5 loop_order | T0 链序颠倒（return 在 send 前） | 0.25119 | 0.50239 | 1.0× / 2.0× | PASS（有界断言） |

判读：
- **L2=2.0× 只可能来自「湿信号同时注入两条目标轨」**：每个频率分量 = 自身干声(1×) + 在两条轨上各注入
  一次的 0.5× 湿 → 1+0.5+0.5=2。单目标回流退化拓扑（L3）= 1+0.5=1.5×，无回流 = 1.0×，三者精确可分。
- L3（1.5×）证明 L2 的 2.0× 不是测量假象，是多目标回流的特征值。
- L4：T0 静音后其干发送被引擎抑制（`shouldProcessAuxSendWhenTrackIsMuted` 默认行为），bus0 只剩 T1，
  湿=0.5×T1 → 880=1.5×；440 归零。静音/solo 联动语义有引擎默认行为可依赖。
- L5：乱序（send 在 return 之后）配置下引擎图级防环（`ReturnNode::findSendNodes` 的上游环检测+剔除）
  重整图后渲染**有限且有界**（440=1.0×、880=2.0×），无发散。精确幅度取决于图 transform 次序
  （剔除是传播式的：乱序轨的干发送也被剥离），**不构成可依赖契约**——设计结论：VitApp 封装层必须
  强制链序（send 恒在 return 前），乱序配置在命令面上做成不可达，引擎剔除仅作兜底。

## 边界声明

1. 探针直接驱动 tracktion_engine 原语（`Edit::createSingleTrackEdit` + `createNode` + `TestProcess` 渲染，
   与 VitApp 内核链接**同一套引擎源**、同一 `EditNodeBuilder` 建图路径），**不经过 VitApp 命令面**
   （VitApp 现无 aux 命令——正是实现卡要补的面）。本实验证明的是**底座可达性**，
   不是 VitApp 端到端能力；端到端归实现卡的真栈烟测。
2. 单线程渲染（`setNumThreads(0)`），3s/腿、2 通道、44100Hz、block 256；确定性输入（正弦 WAV），
   非概率性运行（§8 口径：无需重复样本，一轮即证据）。
3. 实验未覆盖：真实 VST 效果器（用 0.5× send 电平作湿路径标记代替）、FX 轨效果器机架（RackInstance）
   与 aux 插件的链序共存（设计上 send 插 RackInstance 之后即可，见设计件腿 1）、solo 行为（引擎
   mute 行为已证，solo=mute 系同族 mute-state 机制）、离线 `start_render` 通路（实验走节点图直渲，
   与内核渲染共用 createNode）。
4. 环境与可复现：
   - worktree `D:/Vit_DAW_wt_mixsend_design`（基线 origin/main=54283662，协调分支 coord/mixsend-design-claim）
   - tracktion_engine submodule `b43974980c`（主树只读编译源，与 worktree 基线记录指针一致且干净）
   - MSVC 19.50.35717（VS 18 2026）/ Win SDK 10.0.26100 / cmake Release / `/utf-8`
   - 构建目录 `D:/Vit_DAW_wt_mixsend_build`（worktree 外临时目录，不入库）
   - 复现：`cmake -G "Visual Studio 18 2026" -A x64 -S <experiment目录> -B <build目录> && cmake --build <build目录> --config Release --target VitAuxMultiReturnProbe && <build目录>/VitAuxMultiReturnProbe_artefacts/Release/VitAuxMultiReturnProbe.exe`
5. 未占用共享真实栈：探针为独立 console 进程，零端口、零 ZMQ、零 workspace 写入（正弦文件走
   juce TemporaryFile）；PC-RUNTIME-STACK 无需登记。

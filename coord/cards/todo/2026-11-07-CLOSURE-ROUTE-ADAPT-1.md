# CLOSURE-ROUTE-ADAPT-1：分析类请求误入 closure 治理路径的适配（276 rerun1 转卡）

- 发卡：GLM 主管决策流 / 2026-11-07 凌晨（依据=[276 rerun1 取证](../runs/EDU-SYMBOL-CORE-1/smokerun1/rerun1/evidence.md)+[276 终验裁定](../rulings/2026-11-07-EDU-SYMBOL-CORE-1-pass.md) §二——三因素：semantic_entry LLM 非确定性分类（同话术两轮 201/168 tok）×closure 治理路径武装入口×candidate-selection 轮 MaxTurns=1（agent/internal/chat/audio_closure_controller.go:529））
- 派发确认：已确认（主管发卡，自主批次）；验收负责人：GLM 主管决策流
- 池序 271；目标仓库=主仓 agent（路由/闭包预算面）；P2 / 0.5-1 天 / 依赖=276 工件在库；L1-L2 / PC；验证腿可搭后续真栈 run 便车（优先零栈构造）

## 背景

276 rerun1：分析类请求（「分析这段 MIDI 的变换关系」）被 semantic_entry LLM 分类**随机**路由进 minimal_audio_closure 治理路径（run1 同形消息未进），closure candidate-selection 轮 MaxTurns=1×模型单轮 86tok 无工具调用→limit_reached 停轮、executed 零、durable continuation 落库未终局——命令面根本未触达。

## 目标

1. **路由面取证+适配设计**（先行零栈）：closure 武装入口对「分析类请求」（无音频编辑意图的只读分析）的判据——理想=分析类不进 closure 治理路径（或进了也能走分析命令）；语义面属判断权归属裁定域（2026-10-30），**方案须先交主管裁定再动手**；
2. **预算面**：candidate-selection 轮 MaxTurns=1 的合理性取证（1-turn 是否足以选 candidate；若为有意设计记录边界，若欠账则最小修）；
3. **驱动面（可选腿）**：烟测驱动对 waiting_continue 停泊的有界续跑腿（发「继续」走完 continuation）——若路由/预算面修复后仍需；
4. 验证=零栈构造（路由判据单测+预算面单测）优先；真栈可搭后续 run 便车（276 chat exit 0 尾巴随本卡收口）。

## 约束

文件域=chat 路由/closure 预算面+测试+（可选）冒测驱动；禁动判断权归属裁定边界（方案先行交裁定）；治理链红线不变。

## 停止条件

取证发现路由为有意设计（在案裁定）→只记录边界转裁定；构造验证失败取证不重跑。

## 领取/回执/验收

- （执行侧回填）/（执行侧回填）/（主管回填）

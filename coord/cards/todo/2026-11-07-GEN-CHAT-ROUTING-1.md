# GEN-CHAT-ROUTING-1：chat 能力路由对生成线的双缺陷修（278 域外取证①②转卡）

- 发卡：GLM 主管决策流 / 2026-11-07 晚窗（依据=[278 回执域外双缺陷](../doing/2026-11-05-GEN-YUE-DIALECT-1.md)——①capability_owner.go:225 全消息小写子串匹配：ABC 谱文本 `b32|Z|` 含 "b3"→pan_layout canary **误劫持**（run2 工件；b4/d1/c2/c3 标记同暴露面）②goalrunner_chat.go:4361/4544/4605/4608/4437-4492/5001 capabilities 词典收窄工具集：vocal/drums/bpm 话术天然命中既有能力包而包内无 generation 工具→LLM 诚实申报无 gen.generate 可调（run3 工件；276「第四处注册面」同型，裁定 A 先例=一行扩表+配对单测））
- 派发确认：已确认（主管发卡）；验收负责人：GLM 主管决策流
- 池序 303；目标仓库=主仓 agent（chat 能力路由面）；P1 / 0.5 天 / 依赖=278 工件在库（run2/run3 为负锚蓝本）；L2 / PC；零栈先行，chat 腿真栈验证搭 278 末腿

## 目标

1. **缺陷①修（误劫持）**：marker 匹配对结构化文本（ABC 谱/代码块/工具参数）的边界处理——方案择一须证据：A=词边界匹配（substring→token 边界）B=消息分区豁免（谱面/参数区不参与 canary 词法）C=其他；**b4/d1/c2/c3 同暴露面一并修**；配对单测（ABC 含 b3/b4/d1 字样不劫持+正常话术仍命中两方向）。
2. **缺陷②修（词典缺生成）**：capabilities 词典扩 generation 域（vocal/drums/bpm/生成/唱/翻唱等话术→含 gen.generate 的工具集；276 裁定 A 同型一行扩表+配对单测）；既有能力包行为零变化。
3. 回归：`go test ./internal/chat -count=1` 全绿+build 0；278 run2/run3 形态离线复放（修复前必败/修复后通过双形状钉）。

## 约束

- 文件域=capability_owner.go marker 面+goalrunner_chat.go capabilities 词典面+测试；不触 genapi（278 已收口）/PCA 门/装载链；
- 禁止 git reset --hard / git clean；不动主工作树；零真栈（本腿——chat 腿搭 278 末腿顺带）。

## 停止条件

发现 marker 匹配语义存在在案裁定约束（词法后继/入口识别薄层裁定族）→锚点取证上交转裁定，不擅改路由哲学。

## 领取/回执/验收

- （执行侧回填）/（执行侧回填）/（主管回填）

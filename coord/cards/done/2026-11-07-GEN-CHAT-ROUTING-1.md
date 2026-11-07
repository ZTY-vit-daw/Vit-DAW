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

- 领取：2026-11-07 晚窗 +0800 ｜ origin/main=`5c5ef23ec76727970ebca73830214783293c3c18` ｜ owner=ZCode GLM-5.3 L2-worker PC 执行子代理（会话 303runner）｜ 分支=`port/gen-chat-routing-1` ｜ worktree=`D:/Vit_DAW_wt_303grt` ｜ 领取提交=coord/303-claim（hash 见 git log）。领取基线：fresh worktree 自 origin/main，`git status --short` 与 `git diff --stat` 均空（clean）。前置核对：卡在 todo/ ✓ 派发已确认 ✓ 278 工件在库（run2=coord/runs/GEN-YUE-DIALECT-1/run_20261107_193711、run3=run_20261107_194419，只读取证）✓；doing 五卡文件域核对——ARRLOOP-AUTHBRIDGE-1 同文件 goalrunner_chat.go 但不同段（执行器桥 vs 本卡词典段）且其改动仅在未合入 port 分支，本卡基线 origin/main 无重叠文本 ✓；B3-ABLATION-1（275）=ps1/py 面（其 Go 钩在 port/b3-ablation-1 未合入，非本卡基线）✓；PLUGIN-CONTROL-EVAL-RUNNER-1（300）=scripts eval 面 ✓。
- 回执：2026-11-07 晚窗 +0800 ｜ 实现 commit=**28252e19**（分支 `port/gen-chat-routing-1` 已推；领取提交 657761e0 已入 main）｜ 零真栈（卡面约束），无 run 工件、无计费。
  - **diff 清单**（基线 5c5ef23e，4 文件 +398/-14）：`agent/internal/chat/capability_owner.go` +74（新增 `capabilityMarkerWordByte`/`capabilityShortCodeMarker`/`capabilityMarkerPresent` 三 helper+注释；`inferProjectAwareCapability` 六个 marker 循环 `strings.Contains`→`capabilityMarkerPresent`；`explicitB4InvocationMarker`/`explicitC1InvocationMarker` 裸码同面）；`agent/internal/chat/goalrunner_chat.go` +57（`agentLoopGenerationIntent` 新增+词典挂接 `add("generation")`；switch 增 `case "generation"`；`agentLoopGenerationTools()` 新增 gen.generate/gen.providers；static-mix 三意图函数 b1/b2/b3 裸码边界化——复合变体 "b 1"/"b1.1" 族维持 Contains）；`gen_chat_routing_fixture_test.go` 新增（278 run2 消息逐字 fixture，工件锚注）+`gen_chat_routing_test.go` 新增（配对单测 6 个）。
  - **方案择一论证（缺陷①选 A=词边界）**：B 消息分区豁免需 ABC/代码块/参数区结构化解析器——run2 消息里 ABC 谱是散文内嵌无可靠分区信号，且新增解析器本身即路由哲学变更（贴停止条件族）；A 只把裸短码（b2/b3/b4/b5/c1/c2/c3/d1/d2/d3，goalrunner 侧 b1/b2/b3）从 substring 收紧为 token 边界（字母/数字/下划线为连续字节，CJK 与标点为边界），点分 ID 与自然短语维持 Contains——词表成员零增删，与 M1-3/M1-4「入口词表收敛、显式标记才认领」裁定族方向一致（该族管成员准入不管 token 形状，卡面停止条件未触发，判定论证已写入 helper 注释）。**勘察新发现（已修，属卡面『同暴露面一并修』语义）**：goalrunner 词典面 `agentLoopStaticMixPanLayoutIntent` 的 "b3" 同为全消息 Contains——run2 形态在 goalrunner 侧同样误触发（离线 dump：run2 消息 caps=[midi plugin **static_mix_pan_layout** transport]，pan_layout 规则还连带删掉 mix 包）；b1/b2 同族一并边界化，复合变体原语义不动。
  - **双形状复放（离线）**：pristine 基线树（origin/main=5c5ef23e）跑本卡新测试 → 3 个形态测试 FAIL 于缺陷点（`abc score tail with b32` hijacked into "static_mix.pan_layout.v0"；run2 消息 canary 接管；run3 形态 caps=[midi plugin static_mix_pan_layout transport] 无 generation，exit 1）；修复树（28252e19）同测试 6/6 PASS。fixture=run2 original_intent 逐字（b32/a4b4×3/vocal/drums/BPM/gen.generate/唱出来 全在场，含金量核对过）。
  - **既有包零变化钉**：`TestGenChatRoutingExistingPacksUnchangedWithoutGenerationIntent` 9 条话术 capability 集逐字等于 pristine 树 dump 值（[mix]/[midi]/[transport]/[track]/[mix plugin]/[clip]）+工具集无 gen.*；裸 vocal/drums/bpm 非生成触发词。275 B3 钩/264 B4 钩：merge 验证（临时分支 tmp/303-b3merge=本卡 28252e19+origin/port/b3-ablation-1，一处 B3 循环冲突按「275 包裹+本卡内层条件」解）后 `TestPanLayoutAblationHookPair`+`TestLowEndRelationAblationHookPair`+本卡 6 测试+chat 全包 128s 全绿（temp 已删，冲突解法留给主管合 275 时用）。
  - **测试**（worktree D:/Vit_DAW_wt_303grt @ 28252e19）：① `go build ./...` exit 0；② `go test ./internal/chat -count=1` ok 129.7s exit 0；③ `go test ./internal/tools -count=1` ok 0.4s exit 0；④ 新测试 -v 6/6 PASS；⑤ pristine 树同测试 FAIL exit 1（修复前必败钉）。
  - **端测边界声明（零栈）**：本卡零真栈——全部验证为单测/离线复放；真实栈 chat 驱动腿（canary 不劫持+gen.generate 真实披露→YuE2 生成链）按卡面搭 278 末腿（278 卡 doing 待主管调度）。gen.generate 披露不改变治理链：RiskConfirm 确认流与 PCA 门未触碰。
  - **域外观测（不扩修，锚点备查）**：`agentLoopTextHasAny` 全词典仍为 Contains——本卡只边界化了 B 码裸码族；其余词典词为多字词/短语，误命中面低，如后续取证到同类误触发再转卡。
- 验收：（主管回填）

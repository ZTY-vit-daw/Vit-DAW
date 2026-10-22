# FS-NAMED-ALIAS-COVER-1：指名轨闭集别名表覆盖扩展（159 判读缺口①继卡）

- 发卡：GLM 主管决策流 / 2026-10-22 深夜（依据=159 判读：A2「钢琴」/A4「吉他」不在 157 闭集别名表（现仅人声/伴奏/鼓组三族，free_state_reasoning_loop.go:754-758）→ prepare 解析 fail-open → 模型边界自报或域漂移）
- 派发确认：已确认（159 RESULTS §2 判读落定）
- 验收负责人：GLM 主管决策流
- 池序 169；目标仓库=D:\Vit_DAW；P1（意图保真链直接缺口）/ 0.25-0.5 天 / 依赖=无；模型分级：L2 / GLM 执行会话

## 范围

①闭集别名表扩展常见轨族：钢琴（piano/钢琴/keyboard/键盘）、吉他（guitar/吉他/木吉他/电吉他）、贝斯（bass/贝斯）、弦乐（strings/弦乐）、合成器（synth/合成器）、鼓组既有（核对别名完备）；②解析优先级与歧义处理语义保持 157 既有（精确名>别名>歧义 fail-open）；③解析失败时的回复披露可选面：指名未识别时回复告知「指名轨『X』未在轨集中识别，按自动选择处理」（防用户以为指名已生效——159 A2/A4 形态）；④别名词表单测（各族命中/跨族歧义/空输入）。

## 文件域

`agent/internal/chat/free_state_reasoning_loop.go`（闭集别名表面）+同文件族单测。越域即停。

## 验收标准

单测（各族别名命中+歧义 fail-open 保持+既有三族零回退）+真栈 chat 腿最小面：A2 话术（钢琴指名）unt 解析命中非空（159 复刻对照）；全量 0 FAIL+gofmt 净；端测边界声明。真栈腿登记 PC-RUNTIME-STACK。

## 停止条件

别名扩展牵动解析器结构（非纯表扩展）→ 锚点上交。

- 领取：（2026-10-22 20:17 +0800 / origin/main=2bc46406 / owner=GLM-5.3 L2 PC ZCode 执行会话（169 卡会话）/ 分支 port/fs-named-alias-cover-1（实现）+coord/fsnac1-claim（领取提交，本提交）/ worktree D:/Vit_DAW_wt_fsnac1（实现）+D:/Vit_DAW_wt_fsnac1_coord（协调）/ 领取提交=本提交；卡面自主管本地 main ee0050de 摘取入共享 main（只同步 coord 面，同 2b3a9b05/2ccc0b07 先例）；实现基线=origin/main 2bc46406（157/158 在场；主管本地未推线含 FS-NAMED-DYNAMIC-WIRE-1 同文件改动，本卡 ③ 面插入点与之无文本重叠，合并时主管知悉）；串行约定=170 实现等本卡合入后领取，本卡先改先合）
- 回执：（实现 **ab67d9a1**@port/fs-named-alias-cover-1（已推 origin，含工件/artifacts 提交；基线 origin/main 2bc46406）／真栈 run=**fs_alias_chat_20261022_2125**（占用 4a2b49d3→释放 3bff8a62，序=171 释放后）／端测边界=agent HTTP chat 链真栈（真实内核+真 deepseek 链+工程副本隔离），**不含** Godot GUI/webui 渲染面断言（本卡零渲染面文件）；A2/A4 轮停泊确认卡=157 A1 同族形态，提案/执行面非本卡判定面）
  - ①表扩展落地：钢琴（钢琴/键盘/piano/keyboard｜轨名面 piano/keys/keyboard/钢琴/键盘）、吉他（吉他/木吉他/电吉他/guitar/acoustic guitar/electric guitar｜guitar/吉他）、贝斯（贝斯/贝司/bass｜bass/贝斯/贝司）、弦乐（弦乐/strings｜string/弦乐，**不含单字弦**——防「十二弦吉他」误入）、合成器（合成器/synth/synthesizer｜synth/合成器）；鼓组族核对=157 原样完备（别名面多字规则保持，精确名路径自兜底具体乐器名）。**解析器结构零改动**（纯表扩展，停止条件未触发）。
  - ②语义保持：精确名>别名>歧义 fail-open 原逻辑零改动；既有 fs_named_target_test.go/fs_named_dynamic_wire_test.go 零 diff 通过=三族零回退证据。
  - ③可选面已实现（同文件内）：prepare=marker（user_named_target_unresolved）**唯一写入者**——含闭集别名但解析 fail-open 且无粘滞指名→置 marker，其余 prepare 一律**清除**陈旧副本（防客户端回显/持久快照跨轮复燃，写屏障 contextWithUserNamedUnresolvedMarker）；响应包装面追加机器常数行「指名轨『X』未在轨集中识别，按自动选择处理。」（containment 去重+空回复降级）。边界如实声明：粘滞指名在场时抑制披露（此时指名仍绑定，「按自动选择」不真）；walk/confirm 交互面 RequestContext 不带 marker。
  - 单测：新文件 fs_named_alias_cover_test.go 6 例（159 六 stem 镜像 A2/A4 话术复刻命中+弦乐/合成器正命中+同族/跨族歧义 fail-open+单字琴/鼓不触发+三族零回退+③披露/写屏障/粘滞抑制全链+prepare 捕获面）；go build OK+**全量 go test ./... -count=1 exit 0（96 包 0 FAIL）**+gofmt 净（worktree CRLF 检出为既有形态，剥 EOL 后亲核两文件净）。
  - 真栈判定面（RESULTS.md 6 项）：A2 unt=1027/piano、A4 unt=1017/guitar **双命中非空**（159 同话术 unt 缺失→F-gate #1/域漂移形态消除）；断言器 assert_leg.py exit 0（unt 非空+轨名交叉核对）；持久面双源（A2 conversation_graph=1027+A4 runtime_state loops=1017）；③附带验证（弦乐指名 unt=None+marker+回复行）；prepare 解析链 ×16；LLM=17 次全 deepseek-flash 0 error，一次过零重跑。
  - **主管合并注意**：实现基线=origin/main（157/158 在场）；主管本地未推 39 笔含 FS-NAMED-DYNAMIC-WIRE-1 同文件改动（wire marker 块在 bindFreeStateContextToResponse 内 WorkflowData init 后）。本卡 ③ 插入点与其相邻不重叠（别名表区两版本同文+prepare else 分支+resolve 后 helper 区+bind 内独立块），合并时请亲核该函数区；cherry-pick ab67d9a1 预期仅小冲突或零冲突。
  - 串行约定执行：本卡先改先合已达成（done 待验收）；170 实现面（beb75ae3，生成侧抑制条件，自述与本卡文件域不相交）按约定等本卡合入后领取实现/合入。
- 验收：**通过**（GLM 主管决策流 / 2026-10-22 深夜）。核对：diff 亲读（五族表扩展纯表零解析器改动+单字规则保持+③单写者 marker 语义[qualified 置 fresh/其余清除防快照复燃/粘滞抑制]）；与 162 同文件相邻披露块冲突手解（两块并存=改写披露[解析成功]+未识别披露[fail-open]互斥场景全覆盖）；真栈 run fs_alias_chat_20261022_2125 复刻对照成立（A2=1027/A4=1017 双命中，159 unt 缺失形态消除）。验收 commit=主仓 **e5e53b31**（merge，含工件）。

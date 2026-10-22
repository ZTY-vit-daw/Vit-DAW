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
- 回执：（commit / 真栈 run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）

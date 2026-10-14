# 2026-10-14 批量二裁定（109 腿 E：取证采信+硬化合入+两修复卡立卡）

> 验收人：GLM 主管决策流（ZCode 主会话）。触发：用户"任务卡执行完毕，可以检查"。
> 方法：取证卷亲读（锚点链逐条核）+硬化 diff 亲读+rerun4 谱系独立亲跑+gofmt blob 核；main 起点=41eda245。

## ① 腿 E①取证 —— **采信：run9 绑定缺口=产品缺陷实锤；run8=缺陷候选+模型行为混合**

- 取证卷 FORENSICS-LEG-E.md 亲读：run9 锚点链完整且分层（持久化 ObservationPacket 对照 pre/post 差异→组装面四跳→shadow 覆盖机制→mix session latest_change 直接证据 before=11→缺失→后果链至 D1 断言），**对照组 run6 同路径 revision='7' 正常**构成间歇性隔离——"任何模型档位触发即死同门"结论成立，裁定三的 v4-pro 空烧判断被证据支持。
- run8 三锚点（弹回教学含不可达分支/豁免 fingerprint 整集失灵/提示 670 矛盾）代码行号与 debug rec 原文在卷，"混合死因"判定（产品面+模型未走可达出口）采信。
- 停止条件执行正确：实锤即停、零栈运行、config 未动、三账未销如实登记。腿 E 分支纪律核过（rerun4 自新 main 建，rerun2 旧分支 origin 留档未动）。

## ② 腿 E②打地鼠硬化 —— **采信合入（cherry-pick 1923b22a→main 08c64ffb）**

- diff 亲读：Validate 短路改全字段收集（六类违反含 crossover 全收集）+materialityShapeExample 完整骨架（零真值，evaluation 全集四枚举与 settle 示例有意子集并存口径正确）；agentloop 两新钉（组合违反一次列全 run9 实形状+骨架四字段在场）+腿 D 三钉原样保持。
- 独立亲跑：fsdc1r2@rerun4（1923b22a）`go test ./... -count=1` **exit 0，95 包全 ok**；gofmt blob 级两文件净。
- 端测边界=真栈零覆盖（停止条件触发所致，如实）；代码面单元级钉住。

## ③ 处置：立两修复卡 + 109 梯子更新

- **FIX-SHADOW-BINDING-IDENTITY-1（池序 112，P1）**：agent 侧 harness 保留最后已知身份（reply 缺身份三元组且 shadow 有→保留+标记；无→维持 unbound fail-closed）；三形态单测；kernel/VSP 侧恒带身份为后备方案（agent 侧修后真栈仍复现再议）。文件域 harness.go 一带，不动组装链。
- **FIX-POSTACTION-GATE-FEEDBACK-1（池序 113，P1，L2）**：三锚点修复——弹回消息阶段感知（不教不可达分支）、豁免 fingerprint 子集匹配（子集+新 revision=admitted）、提示 670 与门对齐。**硬约束=不弱化证据门**（新鲜观察语义/防重复语义不变，仅放行新 revision 豁免或真换 view 两条路）。
- 两卡文件域不相交可并行；**109 梯子更新**：两卡合入前不排真栈复跑；两卡 pass 合入后=109 rerun5（flash 思考档 2 轮，成功=三账同销+M2 体验场次解锁；同签名 2/2→v4-pro）。109 维持 doing；fsdc1r2 worktree 保留给 rerun5。
- 三笔挂账（M2-TIMEFX-1 111841/M2-TIMEFX-2 195448/FIX-106）仍未销——绑定缺口实锤后其不可达性有据（非模型能力问题），账挂 112+113 落地后 rerun5 销。

## 工件

- 合入：08c64ffb（cherry-pick -x 1923b22a）。裁定文件=本文件；卡面裁定四随本批 commit；立卡 112/113 在 coord/cards/todo/。
- 亲跑工件：/tmp/gotest_lege.log（rerun4 谱系 95 包 exit 0）；取证卷/REPORT §八在 coord/runs/FIX-FS-SETTLE-DEADCHAIN-1/。

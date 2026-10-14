# FIX-POSTACTION-GATE-FEEDBACK-1：post-action 观察门反馈面三锚点修复（弹回教学/豁免粒度/提示矛盾）

- 发卡：GLM 主管决策侧 / 2026-10-14 晚窗（依据=[FORENSICS-LEG-E.md](../../runs/FIX-FS-SETTLE-DEADCHAIN-1/FORENSICS-LEG-E.md) §一 run8 子形态三锚点+腿 E 上交裁定；109 收口腿预算效率前置）
- 派发确认：已确认（主管依已核实取证证据发卡）
- 验收负责人：GLM 主管决策流
- 池序 113；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P1 / 0.5 天 / 无（与 112 文件域不相交可并行）
- 模型分级：L2 / GLM 执行会话（涉门语义，L2）

## 缺陷三锚点（已核实，FORENSICS-LEG-E §一）

1. **弹回教学含不可达分支**：per-view 弹回消息教 "decide from the recorded evidence"，但 requires_post_action_observation=true 时 settle（free_state_reasoning.go:1085）/blocked（:1214）/needs_action（:1026）/satisfied（:1188）四门全部要求本周期成功观察——教的路径被结构性封死。
2. **revision 豁免粒度错位**：freeStateReObservationAdmitted（free_state_gate.go:1055）按**整集 fingerprint** 找 prior（:1061-1076），模型收窄 view 集（合法请求）后 fingerprint 不匹配→豁免够不着→落入 per-view 弹回。
3. **提示矛盾**：ccb_model_prompt.go:670 教 post_action 段首动作="experiment's view set on the applied target"重取，与门的整集豁免/弹回行为未对齐（run8 模型收窄两视图即落入弹回陷阱）。

## 任务（硬约束：**不弱化证据门**——post-action 必须有本周期新鲜观察的语义不变；防重复防浪费语义保持；仅放行"新 revision 豁免成立"或"真正换 view/target"两条路）

1. 弹回消息阶段感知：requires_post_action_observation=true 时不再教"decide from the recorded evidence"，只教可达出口（按豁免形状重取整集 / 换 view/target）。
2. 豁免 fingerprint 子集匹配：请求 view 集为某 receipt view 集的**子集**且 revision 新于该 receipt 时可走 AdmitReObservation 新 revision 路径（收窄不再失灵）；非子集/revision 不新→维持弹回。
3. ccb_model_prompt.go:670 与门行为对齐：教学形状必须是豁免可收的形状（整集重取或子集+新 revision），并明说收窄是允许的。
4. 单测钉：三锚点各一钉+组合形态（post-action 阶段弹回消息不含不可达分支；子集+新 revision=admitted；子集+旧 revision=弹回；提示文本与豁免形状一致）。
5. `go test ./... -count=1` 0 FAIL+gofmt 净（LF blob）。

## 文件域

agent/internal/agentloop/free_state_reasoning.go+free_state_gate.go+chat/ccb_model_prompt.go+同包测试；**不得动** settle/final gate 证据断言与 MaterialityEvaluation（109 腿 D/E 域）；不占真栈（复跑归 109 rerun5）。

## 验收标准

diff 亲读+证据门语义不弱化论证（逐断言核）+四钉亲跑+95 包 0 FAIL。

## 停止条件

子集匹配与防重复语义冲突（子集放行会造成真实重复观察浪费）→ 带冲突场景分析上交。

- 领取：2026-10-14 晚窗 / origin/main=55c4d871（todo 在位、派发已确认；doing 两卡域=experiment_runtime+goalrunner / webui+前端仓，均不相交；112 未领取） / GLM-5.3 L2 执行会话（PC，ZCode） / port/fix-postaction-gate-feedback-1 / D:/Vit_DAW_wt_113 / 领取提交=8291d42a
- 回执：commit=51e5ca9a（port 分支，已推 origin）。四钉全绿（free_state_postaction_feedback_test.go：钉1 弹回阶段感知/钉2 子集豁免三形态+防浪费/钉3 prompt 形状/钉4 run8 组合）；`go test ./... -count=1` 95 包 0 FAIL；gofmt 净（LF blob——本卡四文件全 LF，`gofmt -l` 的全仓大面积报红=autocrlf 检出 CRLF 既有工件，i/lf 未变，非本卡引入）。**证据门语义不弱化逐断言**：①锚点1 纯弹回文案变更、门逻辑零改动——删除的"decide from the recorded evidence"教学恰是被 settle（free_state_reasoning.go:1085）/blocked（:1214）/needs_action（:1026）/satisfied（:1188）四门结构性封死的不可达出口，四门断言本体与 messageLoopHasSuccessfulCCBObservationRequest 未动；阶段外（flag=false）文案字节不变（钉1 后半钉）；新文案仍明示本周期必须新鲜观察。②锚点2 仅新增一条放行形状=请求集⊆**最新覆盖** receipt 集且当前 revision **严格新于**（尾整数数值比较）该 receipt：(a) 子集永不以 new_view_set 放行（PriorViews==RequestedViews 强制仅走 revision 豁免通道）；(b) 旧覆盖 receipt 不能解锁已被更新 receipt 在当前 revision 录过的视图（防浪费钉）；(c) revision 不可排序→fail-closed 弹回；(d) "严格新于"比精确集通道的不等式更严；(e) 精确集通道行为不变（meta/revision 解析仅上移，纯读）。豁免只放行观察请求**执行**，fresh 证据仍须本周期真实返回，豁免≠记账。③锚点3 纯教学：FIRST-action 必须观察+settle/needs_action 禁令两句原样（钉3 断言），替换的仅是被弹形状教学→豁免可收形状（整集/子集+新 revision），明说收窄允许。未动面：settle/final gate 证据断言与 MaterialityEvaluation（109 腿D/E 域）零改动；audioclosure.AdmitReObservation 零改动。**文件域说明**：除卡面三文件+测试外，同步改了 agent/rules/ruleset/resources/sections/neutral_family.discipline.rules.txt——它是 ccb_model_prompt.go 模板的逐字节权威镜像（ruleset_parity_test.go 双向字节锁，生产 prompt 从 embed 渲染），单改 .go 侧 parity 即红且锚点3 生产面不生效；先例=aee05996（C 腿同款双源同步），属锚点3唯一完整实现路径，非越域新缺陷修补，请验收侧裁定采认。**端测边界声明**：本卡零真栈（卡面约定，复跑归 109 rerun5）；单元面覆盖=豁免/弹回/文案/prompt 渲染四钉；post-action 实链（弹回消息实投到模型→收窄请求放行→观察返回→D1 记账）在真实栈的行为由 109 rerun5 验证。停止条件未触发（子集匹配与防重复无冲突：最新覆盖锚+严格新于保证放行的必是"世界已变"的重取）。
- 验收：（裁定文件 / 验收 commit）

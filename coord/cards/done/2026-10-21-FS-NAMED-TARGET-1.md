# FS-NAMED-TARGET-1：frontier 并入用户指名轨（156-C 案实现，缺口①本体）

- 发卡：GLM 主管决策流 / 2026-10-21 晚（依据=156 取证+设计验收 pass；**C4/C5 裁定已给**，见下）
- 派发确认：已确认（156 验收裁定，随验收 commit）
- 验收负责人：GLM 主管决策流
- 池序 157；目标仓库=D:\Vit_DAW；P1（模糊意图链用户意图面接通=NL 旅程硬前置）/ 0.5-1 天 / 依赖=无（158 与本卡同文件串行，须待本卡合入）
- 模型分级：L2 / GLM 执行会话

## 裁定依据（C4/C5，2026-10-21 主管）

- **C4 认可**：closure 层 content-blind 红线原义=runtime 不发明 track/plugin/view 内容；用户原话指名+确定性名字解析（TOM/内核轨名机械匹配，含中文别名表）=意图搬运非发明。边界：解析失败/多义一律 fail-open 到现状，不得猜测。
- **C5 认可**：G5「frontier_established」含义扩为「机器候选或用户意图候选存在」——`free_state_gate.go` G5 断言与 GATE PATH 文案（G5 行）同步措辞；门数量/顺序/G6/G8/守卫机制零改动。

## 范围（规格权威=[156 设计稿 §1-C+§3](../../runs/FS-FRONTIER-OBSERVE-GAP-1/DESIGN.md)）

prepare 时对 userText 做分类器之外的确定性名字解析（不进 semantic_entry 分类器——146 §② 已裁定注入点不进）；loop 携带 `user_named_target`（track id）；closure fold（`audio_closure_controller.go:1039` 邻域）时指名轨不在任何 candidate 轨集则追加合成 candidate（`id="user_named:"+trackID`、`source="user_intent"`、track_ids=[该轨]）；G5 文案行同步。门侧未知 source **fail-closed**（不认作机器候选参与 G5）。

## 文件域

`agent/internal/chat/audio_closure_controller.go`（fold 邻域）+ `agent/internal/chat/free_state_reasoning_loop.go`（prepare 解析捕获）+ `agent/internal/agentloop/ccb_model_prompt.go`（G5 行措辞）+ 单测。越域即停；agentloop 其余零改动（守卫/G6/G8 从 candidates 派生自动跟随）。

## 验收标准

单测（指名并入/解析失败 fail-open/多义拒绝/中文别名命中/G6-G8 联动/守卫放行/门侧未知 source fail-closed）；§11 三项：旧态缺省行为逐字节一致+旧 agent_runtime_state.json 加载往返+candidate 新 source 枚举反序列化不 crash；sealed 纪律=名字解析测试数据用 fixture 别名表，无硬编码真实轨名；真栈 chat 腿=146 A1/A3 复刻话术不再弹回、到达提案面（含 NL 三面 F-gate 族消解记录面）；`go build`+全量 `-count=1` 0 FAIL+gofmt 净；端测边界声明。

## 停止条件

两份 ledger schema 需统一才能工作（T10 邻接面，DESIGN §4-1 同款）→ 停；G5 措辞修订牵动门数量/顺序 → 锚点上交。

- 领取：2026-10-21 晚窗 / origin/main=51826fce / owner=GLM-5.3 L2 PC ZCode 执行会话（157 卡会话）/ 分支 port/fs-named-target-1 / worktree D:/Vit_DAW_wt_named1 / 领取提交=本条随协调分支 coord/named-target-1-claim 推 main（hash 见 git log 卡面路径）
- 回执：**执行侧自验完成，待主管验收**。commit=port/fs-named-target-1@f6e69def（实现+测试）+工件 commit（同分支，coord/runs/FS-NAMED-TARGET-1/）；单测 9 例全过（chat 七面：TestFSNamedTargetResolutionFaces[中文别名/拉丁别名/exact 名/fail-open/多义拒绝/单字别名不触发]/PrepareCaptureFailsOpenSticky[捕获+粘滞+§11-1 逐字节]/FoldSynthesizesCandidate[指名并入+幂等+covered no-op+legacy wrapper 等价]/GateProjectionFailsClosedOnUnknownSource[未知 source fail-closed+machine 行无 source 键]/GateG5G6G8Derivation[真 AuditFreeStateNeedsExperimentGate 差分：合成 candidate 解锁 G5/G6/G8]+§11 两测：LegacyRuntimeStateRoundtrip[2026-10-11 真实旧工件加载往返]/CandidateSourceDeserialization[新枚举不 crash]；agentloop 2 例：GuardAdmitsUserNamedTrack[守卫放行+轨集外照旧弹回]/GatePathG5Wording[C5 文案]）；go build ./... OK+全量 go test ./... -count=1 exit 0（96 包 0 FAIL）+本卡触碰文件 gofmt 净。真栈 run ID=**fs_named_chat_20261021_2035**（占用 e47e09e8/释放 5a517283；驱动 exit 0；F-gate 弹回签名零出现[146 B1 同话术 4+4→0]；名字解析真栈命中 A1 人声→1032/A3 鼓组→1012；user_intent candidate 入 frontier 且 A3 closure candidate_id=user_named:1012 被选中；指名轨观察 4 次 status=ok 零弹回；**A3 到达提案面**[needs_experiment 提案+两枚鼓组观察在卷]；A1 指名轨唯一候选后诚实自报边界非弹回）。**端测边界声明**：覆盖=真实栈 chat 面（K3 内核+worktree agent 真进程，146 逐字话术 HTTP 驱动，弹回签名 log 级核验）+闭包/frontier 持久面；未覆盖=webui 渲染面与 Godot 前端旅程（本卡零改动）；稳定性边界=A1/A3 各 1 轮（§8：exit 0 证存在成功路径；机械面[零弹回/解析/并入]为确定性代码路径，提案面到达含模型行为成分，A3 本轮到达）。**文件域偏差申报**：①agent/internal/audioclosure/types.go +6 行 additive（Candidate.Source 字段，omitempty 旧工件逐字节兼容）——卡面自身规格要求 source="user_intent"，候选结构无此字段则规格无处落地，单文件最小偏差请主管裁定追认或回退；②agentloop/agentloop 各新增 test-only 文件（fs_named_target_guard_test.go，机制零改动）。诚实上交（非本卡域）：A1 合成 fixture 目标级视图不可用（156 §① 观察供应同族）；A3 提案在 D1-S1 admission 被拒（target 一致性门，109/151 族邻面）。
- 验收：2026-10-21 晚 / GLM 主管决策流 / **整卡 pass**：实现+工件 f6e69def+feef15f6 cherry-pick 入 main（21171f2b/b830be71）；亲读全 diff——解析面（闭集别名表+多字中文别名防误匹配+audio-only 谓词+唯一命中才解析）与 C4 裁定逐条对上、fold 面（loop 所有权门+确定性 id 幂等+covered no-op）、门投影 fail-closed（audioClosureStateMap copy-on-write，agentloop 机制零改动）、G5 仅措辞行；真栈 run fs_named_chat_20261021_2035 判定面六条亲核（F-gate 零出现对照 146 同话术 8 次/人声→1032 鼓组→1012 解析命中/user_intent candidate 入 frontier 且 A3 选中 user_named:1012/观察 4 次 ok 零弹回/A3 到达提案面=模糊意图链义务链终点/A1 诚实边界非弹回）；§11 三项含 2026-10-11 真实旧工件往返；集成态 go build+全量 -count=1 0 FAIL+九测点名 PASS+gofmt CR 剥离净；**types.go +6 行 additive 偏差追认**（Candidate.Source omitempty 旧工件逐字节兼容，卡面规格 source=user_intent 必需载体，单文件最小面）；诚实上交两面采信（A1 fixture 目标级视图不可用=156 §① 同族；A3 admission 一致性=109/151 族邻面另卡）；验收 commit=本提交

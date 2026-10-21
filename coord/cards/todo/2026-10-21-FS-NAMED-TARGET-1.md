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

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / 单测名单 / 真栈 run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）

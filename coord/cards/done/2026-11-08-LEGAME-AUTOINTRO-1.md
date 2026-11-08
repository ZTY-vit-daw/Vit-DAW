# LEGAME-AUTOINTRO-1：进入工程自动开场+重进恢复播报（L1-b）

- 发卡：GLM 主管决策流 / 2026-11-08 深夜（依据=[313 设计稿 §2.2]+用户首验核心诉求「进入即自动开场」）
- 派发确认：已确认（主管发卡，L1 三件组）；验收负责人：GLM 主管决策流
- 池序 315；目标仓库=主仓 agent+scripts；P1 / 1 天 / **依赖=314（布位段同改）**；L2 / PC；零栈先行+真栈腿候队列

## 目标（照设计稿 §2.2 执行）

1. 工程↔level 绑定元数据（binding sidecar 形态照设计稿）+activation/restore 完成后自动开场注入；**幂等语义=开场工程级一次性/恢复会话级播报**；只在「binding 在案∧无活跃局」触发（不触挂起语义）；
2. chat/edu_jam_chat.go ~120 行+server.go handleChat ~15 行插点+布位脚本 binding 段；
3. 单测=binding×会话四象限+幂等+F5 竞态回归（281-B2 教训）；真栈腿=白话触发开场→重进恢复播报→exit 0；GUI 最后一米边界声明。

## 约束
文件域=chat/edu_jam_chat.go+chat/server.go 插点±测试+scripts 布位段；与 316 布位段协调（同 owner 顺序执行）；禁动 312 域。

## 领取/回执/验收

### 领取

- 2026-11-08 21:11 +0800 / origin/main `684d8367`（≥发卡基线 `699d486d`；含本组 314 领取+段1 回执两笔在卷）/ owner=ZCode GLM-5.3 L2-worker PC 执行子代理（会话 314-316 legame L1 组，同 owner 顺序 314→315→316）/ 分支 `port/legame-l1-1` / worktree `D:/Vit_DAW_wt_314l1`
- 领取基线：HEAD=`bdf2e0fc`（=314 实现提交，依赖兑现起点）；`git status --short` 空；`git diff --stat` 空（净树续卡）
- 段划分：段 1=实现+单测零栈先行；真栈腿候队列（315 真栈腿含 316 重铺后再开场验收，照设计稿 §6 顺序）

### 回执

#### 段 1：实现+单测（2026-11-08 21:20，commit `cd5aa5b0` @ `port/legame-l1-1`，零栈）

- **diff 清单**（4 文件，+704/−2）：
  - `agent/internal/chat/edu_jam_chat.go`（+250）：`legameBinding`（schema `vit.legame_binding.v1`，工程旁 `.vit_agent/legame_binding.json`；agent 只读绑定本体、唯一写面=回写 opened_at/opened_conversation_id 两个开场标记，temp+rename 原子落盘；缺文件/坏 schema/坏 level id=视同无 binding 普通工程零差异）+`eduJamProjectActiveSession`（工程级活跃局查找，active/awaiting_closure 均算活跃，最新 EnteredAt 定序）+`eduJamRecoveryBroadcast`（确定性模板零 LLM，照设计稿 §2.2-3 字面：「接着上局：{节点标题}（{n}/{total}）——继续或问我都行」；全关已过形态给收尾引导分支）+`eduJamAutoIntroCheckpoint`（五条件全查；命令词形与 `/` 斜杠放行给既有命令面=同一 enter 内核语义单源；enter 失败 fail-soft 不拦轮不写 opened_at 下轮重试；`eduJamIntroMu` 串行化整个决策+动作防并发双开场）
  - `agent/internal/chat/server.go`（+17）：Server 增 `eduJamIntroMu`/`eduJamIntroServed` 两字段（播报=会话级一次性进程内标记，跨进程重启后重播一次=「重进工程」设计形态）+handleChat 检查点插点 12 行。**插点位置说明**：设计稿写「2584-2586 附近（activation 尾部）」；实际落点=activation+restore 完成之后、`takePendingImprovementProposalForChat` 确认面之后（确认轮不得被开场劫持——乐局不偷会话轮的既有红线）、语义入口分类之前——满足 F5 时序要求（只观察最终态），writeChat 生命周期（turn.started/completed 事件+history 记录）全保；note 会话在前导段早分叉天然排除
  - `agent/internal/chat/edu_jam_intro_test.go`（新，349 行，5 测例 13 子例）：binding×会话态矩阵 8 态（无 binding×无局/有局=零差异[播报同受 binding 门控，普通工程 100% 零行为差异]/坏 schema 视同无/fresh×无局=开场+binding 回写断言/fresh×活跃局=播报/fresh×终态局=再开场[设计稿字面：终态不挡]/opened×活跃局=播报/opened×终态=静默）+幂等（同会话二轮不注入；新会话=播报非重开场；opened_conversation_id 锚定开场会话）+播报内容（n1「读谱纠错（2/7）」+workflow_data node_index/node_total/owner_conversation_id）+命令词旁路（开始乐局/乐局状态/写好了//reset 均不经检查点，手动 enter 照常）+**F5 竞态回归两方向**（方向 1：内核已切目标工程而工作区绑旧草稿且旧草稿旁有 fresh binding→不注入+会话表空+binding 不被写+空态整表覆盖无会话可丢；方向 2：激活追上+空态 restore→白话开场绑定激活 uuid+binding 落盘+新会话播报）
  - `scripts/legame_placement.py`（+90）：`write-binding` 命令——fresh 写入+回读核对；幂等（未 opened 重写恒等）；已 opened 拒绝 reset（exit 2，防误清工程级开场标记）；`--force` 显式放开。本地亲验三路径（0/2/0）
- **测试命令与退出码**（全 0）：`go build ./...` ok；`go test ./internal/chat -run 'TestEduJamAutoIntro' -count=1`（5 测例 PASS）；`go test ./internal/chat -count=1` ok 145.8s 全包零回退
- **端测边界声明（AGENTS §5）**：段 1 零栈（单测+build+脚本域）；Godot 聊天面板视觉呈现不在断言面（API 级验证，GUI 最后一米=用户手测本体）；真栈腿（布位带 binding 工程→白话触发开场[含 edu_jam_entered，无命令词]→新会话白话→恢复播报→exit 0；含 316 重铺后再开场）候栈——21:15 起 317 占用登记在卷，候其释放
- **设计偏差记录**：①插点从「activation 尾部」后移至 pending 确认面之后（理由如上，F5 时序不破坏）；②恢复播报同受「binding 在案」门控（设计稿五条件针对注入整体；保证无 binding 工程 100% 零差异的 §11 义务）；③「fresh binding×终态局→再开场」按设计稿字面语义实现（终态不属「活跃（非终态）」），供验收复核

#### 段 2：真栈腿（2026-11-09 00:0x，占用 `421f1665`/释放 `b2cfad93`，工件随 `port/legame-l1-1` `37f47c7b`）

- **runB（intro 腿）PS1/py exit 0**：place-copy 隔离副本（binding fresh）→ 启动页面进入 → conv-A **白话「在吗」（无命令词）→ `edu_jam_entered`**（完整开场叙事；含 314 三级解析+binding 五条件全链真栈实证）→ binding 磁盘回写 opened_at=2026-11-08T16:05:15Z+opened_conversation_id=conv-A → conv-B「嗨」→ **`edu_jam_recovery_broadcast`「接着上局：拍谱开局（1/7）——继续或问我都行。」**（模板确定性零 LLM）→ %APPDATA% 用户目标工程 SHA256==e0e95338…2441e3（预演副本制零接触）；两会话 telemetry 零行。
- **runC（含 316 重铺后再开场验收）PS1/py exit 0**：真 `relay` 命令清 runB 会话污染态（.vit_agent 仅 binding/opened_at=null/bound_at 溯源保留；.vit_history 消失；工程哈希==模板——重铺三问全过）→ **新 agent 进程白话 → 自动开场再次触发**（opened_at 重写为新会话 16:06:03Z）→ 新会话恢复播报照常 → 目标哈希不动 → 零 LLM。**重铺=「新工程」语义自证**。
- 取证在卷：两次启动失败均环境类不计 run（同 314 段 2；驱动入场面修正 `9b87531e`）；TEARDOWN_ALL_ZERO 00:06:13 亲核。详见 `coord/runs/LEGAME-L1-STACK-1/evidence.md`。
- 整卡自验完成（段 1 零栈四象限+F5 竞态回归+段 2 真栈白话开场/播报/重铺再开场 exit 0），mv done/ 待主管终验。

### 验收

- （主管回填）

### 终验（主管，2026-11-09 凌晨）

- **pass 整卡收口+合入 main（L1 三件组批）**——runB summary 主管直读亲证：白话会话 auto_intro stop_reason=edu_jam_entered+binding writeback opened_at+恢复播报「接着上局：拍谱开局（1/7）——继续或问我都行」+用户工程 sha256 零触碰（place-copy 副本制兑现）；runA=零 env 非 repo cwd 启动页面进入 enter_attempts=1（产品级装载实证）；runC=重铺后再开场（新工程语义）；三 run exit 0 零 LLM+TEARDOWN_ALL_ZERO。域扩展批准：coord/fixtures/legame/（布位模板面）+server.go +2 字段（binding 态）。合入后主仓定向 EduJam/Legame 测试复跑 ok。


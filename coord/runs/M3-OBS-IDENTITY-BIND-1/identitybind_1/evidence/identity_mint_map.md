# 四源 project identity 铸造点图谱与断裂机制（M3-OBS-IDENTITY-BIND-1 诊断定案）

依据：230 run `coord/runs/M3-OBS-EVIDENCE-FIXTURE-1/obsfixture_20261031_1/evidence/identity_break_and_stale_l1.json` + 本卡代码勘察（worktree @ ab2bcde6）。锚点 id：bake 导入 `03a0`、观察链解析 `ce9e`、落盘工程 `8093`、boot 隔离 default `43f8`。

## 一、铸造点图谱（全部内核铸造；agent 侧零 vitproj_ 铸造点——grep 亲证）

| # | id（230 run） | 铸造点（代码锚点） | 语义 |
|---|---|---|---|
| M1 | `ce9e` | `VitApp/Workspace/default_project.xml` 的 `vit_project_uuid`（6fda77e5 git 内容亲核=ce9e；boot 装载走 `applyLoadedEdit`→`ensureVitProjectUUID(edit)` 不 forceNew，保留文件 id） | 内核 boot 装载的 default 工程 id |
| M2 | `43f8` | 内核 boot 期初始 edit（装载 default 前的空白 edit，`ensureVitProjectUUID` 空值铸造分支）；agent boot 激活窗口抓到的瞬态 id | boot 瞬态 |
| M3 | `03a0` | `VitHeadlessService.cpp` new_project 处理 `ensureVitProjectUUID(*edit, true)`（forceNew）；随后 `saveProjectToDefaultXml()` 把新 id 写回 default_project.xml（driver 注释"new_project rewrites the workspace default project"，217 事故家族） | 合法新工程铸造（fork 语义：无 parent） |
| M4 | `8093` | `ipcSaveEncryptedProjectToPath(target, createNewProjectIdentity=true)`→`ensureVitProjectUUID(*activeEdit, true)`（save_as 路径 forceNew）+ `vit_project_parent_uuid`=source | 合法 fork 铸造（save_as 语义：parent=source） |

其余 forceNew 站点：非 .vit saveAs（`ipcSaveAsProjectAt`）、folder export（:1611）——同 fork 家族。`open_project`/`save`（当前路径）保留 id 不铸造。

**结论：内核侧 new_project/save_as 每次合法铸新 id（fork 语义，内核设计如此）；edit state id 随铸造漂移（ce9e→03a0→8093）。这不是内核缺陷，是设计语义。**

## 二、agent 消费面（三个 id 输入源，三种滞后形态，无对账层）

| 消费面 | 代码锚点 | id 来源 | 230 run 实测 |
|---|---|---|---|
| store 绑定 | `ActivateProjectStore`（harness.go:580）← boot 激活 `activateCurrentProjectWorkspace`（chat/server.go:7415，多处 face 调用）/ version_project_* 外部通知 / agent 生命周期钩子 | `projectstore.Current()`（进程级 active roots） | `03a0`（bake 时点）→ `.vit_derived/vitproj_03a0/` |
| bake 持久化盖章 | `ingestKernelL3AcousticTelemetry`（harness.go:4722-4737）：事件 id 空/"current"/==roots 才落盘，盖章=roots.UUID | Current() | L3 行+acoustic store 落在 03a0 名下（包 #0/#1，l3 ready） |
| 观察读取 identity | `mixObservationCommandWithLiveProjectIdentity`（harness.go:7556，state id 盖进 cmd）→ `IdentityFromMaps`（acousticpackage/status.go:279 五级回落） | shadow state（kernel 快照/delta，可滞后） | `ce9e`（boot 快照滞后，包 #2）→ chat 期刷新后 `8093` |
| 判定面 | `rowIdentityMismatchReason` project_mismatch（status.go:1033）+ `MergeStatus`（status.go:394） | — | 03a0 事实 vs ce9e/8093 读取 → stale → levels null |

**路径基准同源**：`acousticpackage.DefaultStorePath`、`mixboard.DefaultRoot`、`projectworkspace.DerivedDir` 全部 keying 于 `projectstore.Current()`——对账 Current() 即可对齐全路径。

## 三、断裂机制定案

1. 内核合法铸新 id 后（raw ZMQ `new_project`+`save_as_project` 直连，**不产生任何 agent 通知**——version_project_new/saved/opened 由 Godot 前端发，内核 C++ 源 grep 零命中），agent 无跟随触发。
2. bake 事实落在铸造序列**中间态** id（03a0）名下；观察读取用**另一时点**的 state id（ce9e 滞后快照→8093 刷新后）。
3. `MergeStatus`/`rowIdentityMismatchReason` 按 id 严格判 stale——**判定本身正确**（防御语义，卡面安全面：不弱化）；错在上游多源无对账。
4. 已有传播件只覆盖**agent 生命周期**：agent-invoked save_as（harness.go:1522-1554 ForkDerived+ForkAgentStore+Activate）与 version_project_saved 通知（harness.go:3237+）；**缺的是"外部生命周期变化"的跟随**——即对账层。

## 四、真实旅程暴露面判定（代码层预判，真栈腿验证见后）

- **GUI save_as 旅程**：前端发 version_project_save_prepare/version_project_saved → fork+再激活+shadow 刷新——传播件在位，**预判不暴露**。
- **agent 重启旅程**（226 形态）：boot 激活绑定已存盘工程文件 id（open 保留 id，store 在工程旁）——**预判不暴露**。
- **暴露旅程**：任何不经 agent 通知面的生命周期变化——raw ZMQ 脚本流（229/230 烟测形态）、无前端参与的头less自动化。
- **判定**：分裂=合法 fork 语义 + 缺"外部变化"跟随层的**生产缺陷**（非纯烟测工件）；但修复面=agent 对账层（生产），**不动 scripts**（条件 scripts 腿先决条件不满足：不是"烟测流自身诱导合法分裂且真实旅程不暴露"——真实旅程依赖通知完备性，对账层补的是通知之外的全部路径）。

## 五、单一真相源设计裁定（目标 1 交付）

- **权威 id = 内核当前 loaded project 的 edit state UUID**（内核铸造、每个遥测事件自带、GUI 用户所见）。落盘/观察/导入/读取层全部跟随。
- **跟随机制（对账层，单一挂点）**：观察链入口（`requestMixObservation` 顶部，覆盖 mix_observe+ccb.observation_request 全观察面）：
  1. liveID=state id；generic/空 → 刷新一次重读；仍空 → 原样返回。
  2. A=Current().UUID；无 active store → 绑定 liveID（switch 跟随）。
  3. A==liveID → 无操作（含 shadow 滞后自愈：先刷新再比）。
  4. A≠liveID → 刷新 state 重比；仍异 → 按内核 parent 判跟随形态：
     - **fork 跟随**（state.parent_project_uuid==A）：`ForkDerived`+`ForkAgentStore`（A→liveID，既有传播件复用）+`Activate(liveID)`——事实再绑定到权威 id；
     - **switch 跟随**（parent≠A：new_project 空白/外部 open 他工程）：`Ensure`+`Activate(liveID)`，不搬运旧事实。
- **ForkDerived 补强**：现只 rebind L3 log+manifest；补全 copied derived 树内全部 JSON/JSONL 再绑定（复用 `projectstore.RebindProjectTree`）——acoustic store/feature snapshot 一并跟随权威 id（§11 兼容：缺文件容忍，旧 derived 无 acoustic store 照常 fork）。
- **ingest 门不放松**（事件自带外域具体 id≠roots 仍不落盘——非本进程工程的事实不抢注）；观察触发的新 bake 在对账后落 liveID 名下，事实链闭合。

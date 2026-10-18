# FE-START-EMPTY-DRAFT-1：启动页双 draft 创建+kernel↔shadow 轨道不同步（原「空工程」定性已勘误）

- 发卡：GLM 主管决策侧 / 2026-10-15 早窗（依据=M2-EXPERIENCE-1 首跑取证，[README](../../runs/M2-EXPERIENCE-1/README.md)）
- 派发确认：**已确认（用户 2026-10-18 早窗 /morning 四线确认）**；验收负责人：GLM 主管决策流
- 池序 118；目标仓库=前端仓（Godot）+ agent 启动面；**P1（2026-10-18 早窗主管裁定升级，依据=双 draft 四轮 100% 复现+「新建工程+导入」为 M2 旅程用户入口面+track.list 工具面失真自愈掩盖；归因腿先行）** / ≤0.5 天
- 模型分级：L1 / Mac 线或 PC 执行流
- 领取（2026-10-18 午窗）：2026-10-18 13:41 +0800 / origin/main=0c433634 / owner=**DSH PC 执行流 exec-start-draft**（本机 DSH 团队会话，12:00-18:00 决策流派发；用户 2026-10-18 午窗选择 118+132 并行）/ 分支 **port/fe-start-empty-draft-1**（自 origin/main 0c433634 建）/ worktree **D:/Vit_DAW_wt_fedraft1** / 领取提交=（本提交）。执行口径=**归因腿先行**（四轮样本证据链+代码锚点，不占栈）→ 最小修复（agent 侧可直接做；**前端侧写修复前须报 Lead 定基线**——前端 A/B/C 三分支+main 归一挂账，不得自行选线/合 main）→ 验证（agent Go 全量测试；前端侧按 91/92 探针+回归模式）。真栈/UI 腿与用户 92 手测/132 内核栈腿串行排队（PC-RUNTIME-STACK 排队条），Lead 放行前不得启栈。协作模式（DSH 团队）：协调面由 12-18 决策流单写，实现面由执行流在独立 worktree 写。

## 缺陷实锤

1. 启动页入场连建**两个空壳 draft**（09:52:32 与 09:52:33 各一 Unsaved.vit 75B；12:37:31/32 二次复现同签名）——75B=从未保存的占位文件；
2. **【晚窗勘误】「用户误入空工程」定性撤回**：用户明确新建工程并加载音轨，两轮 draft 的 l3_feature_log 各 18 条真实音频分析（1.76M 采样/条）在卷——**内核有真实音轨；异常=agent shadow 报 0 轨**（kernel↔shadow 轨道不同步）。「新建工程+导入音轨」路径的 shadow 同步缺口是本卡与 117 卡的共同根因候选（烟测只走「打开既有工程」路径故从未暴露）；
3. 新线索（领取时归因）：第二个 draft 的 `.vit_history/` 树内出现**首轮工程 uuid（vitproj_69a775…）的 state/worktrees 登记**——跨工程历史状态泄漏候选；另有 VitAgent 启动期（09:52:36）写 2026-09 旧 draft 的 agent_runtime_state.json（42MB）——启动加载未区分会话/工程归属或未清理；
4. 环境登记：用户前端=@port/fe-gui-shrink-1 c56e38f（10-11 Phase A 开发分支，非 10-14 合入 91 后主线）——双 draft 创建与前端工程进入握手的因果关系待归因（前端侧 or agent 侧建了第二个 draft）。
5. **第三轮样本（2026-10-15 21:00，晚窗）**：双 draft **第三次复现**（21:00:48/21:00:51，3 秒间隔）；**但 shadow 正常同步（track_count=6）**——shadow 失同步未复现。归因更新：**双 draft 与 shadow 失同步解耦**（双 draft 稳定复现≠必致失同步；今早失同步另有条件——疑与对话绑定到哪一个 draft/时序竞态有关，归因腿验证）；另本轮 12 个 draft 目录晚窗新建（20:00 后）待一并清点归因。
6. **第四轮样本（2026-10-16 18:16，晚窗，M2 体验第四轮）**：双 draft **第四次复现**（18:16:33/34，draft_20261016T101633_4494fa5a + draft_20261016T101634_b87bbe28，1 秒间隔；shadow 正常 6/6）。新关联观察（决策侧入场取证，[README 第四轮](../../runs/M2-EXPERIENCE-1/)）：本场 `track.list` 工具返回 0 轨（events seq7 body 自述）而 shadow/engine 均为 6 轨，agent 以 get_project_state 自愈——**列表工具面失真**与首轮 shadow 失同步是否同根（「新建+导入」路径的轨道枚举缺口），归因腿一并核。

## 修复目标

- 双 draft 创建归因（谁建的两个、为何两个）并消除重复创建；
- 「进入目标工程」入口语义明确：工程打开失败必须显形，不得静默落入新空 draft；
- （附带）旧工程 42MB 运行时状态的启动加载归因：是否按最近工程恢复、是否应加载、加载是否拖慢启动。

## 文件域

前端仓启动页/工程打开路径（Mac 线域）+ agent 启动状态恢复面（PC 侧，领取时以归因锚点为准）；领取前与 114（Mac 在飞）核对前端仓文件域。

## 验收标准

- 归因件：双 draft 与空工程落入的因果链（前端/agent/内核各自角色）；
- 修复后手测：启动页进入既有工程 → 打开的是目标工程本身（非新 draft）；打开失败显形；
- 前端仓回归+探针（按 91/92 卡模式）。

## 停止条件

归因发现属 114（CEF 焦点）同根 → 上交 Mac 决策侧并卡处理。
- 回执（2026-10-18 午窗，exec-start-draft 执行，Lead 转写）：分支 **port/fe-start-empty-draft-1@922e6305**（=生产修复 b24aae8f + docs 1dcb20f8 + 真栈复验工件 922e6305；已推远端亲核；worktree clean；未 push main）。**生产修复**（唯一生产代码 commit b24aae8f，仅 `agent/internal/history` +66/−1 + tests +110）：`ProjectNew` 先判定缓存默认 draft 是否原始空壳（目录仅 Unsaved.vit、无 .vit_history/.vit_agent）→ 是则复用为新工程 draft 并清启动期旧 uuid 身份绑定，否则维持原「清缓存建新 draft」语义；仅新增 `reused_default_draft` 返回键；2 回归单测（复用=仅 1 个 draft 且旧 uuid 不泄漏 / 非空壳仍新建）+既有测试保持。**归因件**（[ATTRIBUTION.md](../../runs/FE-START-EMPTY-DRAFT-1/ATTRIBUTION.md)，4 轮样本+13:45 live5）：双 draft=**两个 agent 动作**（启动读路径物化空壳 A：CurrentProjectIdentity→BindProjectIdentity("",uuid)→defaultDraftProjectPath；前端 version.project_new→ProjectNew 清缓存新建 B），非前端/内核建；shadow 0 轨=权威快照不刷新+delta 仅入 nodes/orphan（与双 draft 解耦）；42MB=按内核启动 uuid 找旧 draft 整读≥2 次+写 1 次≈4s（随后丢弃）；跨工程泄漏=ProjectNew 后并发读路径用旧 uuid 物化（round2 1/5 竞态）；track.list 0 轨=候选面待探针。**验证**：go build exit 0；隔离 draft 根 `go test ./...` 95 包 ok/0 FAIL（evidence/gotest.txt）；gofmt 净。**真栈复验=PASS 8/8**（realstack_d_clean_20261018_1421/probe_final：pristine_precondition/new_project_reuse/no_old_uuid_leak 全 true、failures=[]；用户等价场景 kernel new_project+agent version.project_new 后 **draft 根新增=0**（旧行为 +1）、隐式 draft 被复用、只含新 uuid、无旧 uuid 泄漏；**用户真实 draft 根零写入 10177→10177**）；被测 agent SHA256 69B8D77C…/kernel B6565DCF…；LLM=0；拆栈 14:18:52 六端口=0+本腿进程=0（只杀自起 pid）。**适用边界（重要，如实登记）**：①空隔离根（首装）仍双 draft（A 轮实证）——本修复覆盖用户真实时序（有上一会话 draft 且 A 保持 75B）；②启动期自主历史 reader（version.list/version.worktree_list，activation 后 1.5–2s）会物化隐藏 draft 破坏 pristine（D/E 轮实证）；③track.list 0 轨未在空工程栈验证；④测试隔离既有违规（4 污染 draft 已隔离未删）；⑤13:45 live5=用户 92 手测尝试（用户 session）；⑥真栈最终轮以并发探针在 reader 落盘前完成通知（自然 pristine、无取消动作，非放宽断言）。
- 验收：**pass（2026-10-18 午窗）**——[rulings/2026-10-18-BATCH4-rulings.md](../../rulings/2026-10-18-BATCH4-rulings.md) §一；**决策侧独立复验**=生产 commit diff 亲读（最小面/无越域/旧 uuid 清理合理/返回键仅新增）+**独立全量复跑**（隔离 draft 根：go build exit 0；go test ./... **exit 0、95 包 ok/0 FAIL、实测 100.9s**；真实 draft 根 10177→10177 零新增）+真栈工件亲读（probe_final 3/3 true/failures=[]/teardown 六端口=0 只杀自起 pid/LLM=0）+远端 hash 亲核；归因件采信；**6 项边界/偏差如实登记，不阻塞本卡最小修复口径**；三笔 cherry-pick 入 main；卡归 done。根因面（读路径物化/reader 抢跑/泄漏守卫）已拆卡 **137 FIX-START-DRAFT-SIDE-EFFECT-1（P1，待用户确认）**；42MB=138、track.list=139、测试隔离=140；用户 UI 手测 4 条保留排期。

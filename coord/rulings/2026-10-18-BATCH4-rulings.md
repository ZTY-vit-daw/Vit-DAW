# 2026-10-18 午窗 批量四裁定（118 FE-START-EMPTY-DRAFT 终验 pass + 拆卡四张）

- 验收会话：12:00-18:00 决策流（DSH PC，本机）；裁定来源=用户 2026-10-18 午窗派发（118+132 并行）。
- 基线：origin/main=**e5fc38fa**（验收前）；本批合入后 main=__MAIN_AFTER__。
- 执行侧：DSH exec-start-draft（task-4）；协调面由 12:00-18:00 决策流单写。

## 一、FE-START-EMPTY-DRAFT-1（118）——**终验 pass**（最小修复口径；适用边界如实登记）

- 交付：分支 **port/fe-start-empty-draft-1@922e6305**（=生产修复 b24aae8f + docs 1dcb20f8 + 真栈复验工件 922e6305；已推、远端亲核；worktree clean；未 push main）。
- 生产修复（唯一生产代码 commit b24aae8f，仅 `agent/internal/history` +66/−1 + tests +110）：`ProjectNew` 先判定缓存默认 draft 是否为原始空壳（目录仅 `Unsaved.vit`，无 `.vit_history/.vit_agent`）→ 是则**复用**为新工程 draft 并清启动期旧 uuid 身份绑定，否则维持原「清缓存建新 draft」语义；返回 map 仅新增 `reused_default_draft`；新增 2 回归单测（复用=仅 1 个 draft 且无旧 uuid 泄漏 / 非空壳仍新建），既有测试保持。
- 决策侧独立复验（亲做）：
  1. diff 亲读：最小面、无越域、复用语义与旧 uuid 清理合理、新增返回键仅新增；
  2. **独立全量复跑**（隔离 draft 根）：`go build ./...` exit 0；`go test ./... -count=1` **exit 0（95 包 ok/0 FAIL，实测 100.9s）**；真实 draft 根运行后仍 **10177 目录（零新增）**；
  3. 真栈工件亲读：`realstack_d_clean_20261018_1421/probe_final/probe_final_result.json` assertions 3/3 true（pristine_precondition/new_project_reuse/no_old_uuid_leak）、failures=[]；teardown 只杀自起 pid（VitApp 10100/VitAgent 3620/17352）、六端口=0；14:18:52 在场 `VitApp pid=20128` 属 134 未登记 run（未触碰，见纪律记录）；LLM=0；
  4. 远端 hash 亲核 + 归因件（4 轮证据+13:45 live5）采信。
- **适用边界/偏差采信（7 项，不阻塞本卡最小修复口径）**：
  1. 空隔离根（首装用户）字面场景仍双 draft（A 轮实证）——最小修复覆盖用户真实时序（有上一会话 draft 且 A 保持 75B）；
  2. 启动期自主历史 reader（`version.list`/`version.worktree_list`，activation 后 1.5–2s）会物化隐藏 draft 破坏 pristine 前置（D/E 轮实证）——**根因归拆卡 137**；
  3. 42MB 旧工程启动恢复（按内核启动 uuid、整读≥2 次+写 1 次≈4s、随后丢弃）——归拆卡 138；
  4. `track.list` 0 轨（round4 seq7）未证——归拆卡 139（同帧对照探针）；
  5. 测试隔离违规（既有测试写真实 draft 根 4 个，已隔离未删）——归拆卡 140；
  6. 13:45 live5 双 draft/42MB 归因=**用户 92 手测尝试**（用户 session，非执行流 owner）；
  7. 真栈最终轮以并发探针在 reader 落盘前完成通知（自然 pristine、无取消动作）——为边界证据，非放宽断言。
- **合入**：三笔 cherry-pick 入 main（b24aae8f/1dcb20f8/922e6305）；卡 118 归 done。用户 UI 手测 4 条（新建工程 draft 根只新增 1 个 / 打开既有工程不落空壳 / 导入后问轨数 / track.list 轨数一致）保留排期。

## 二、拆卡记录（118 归因拆出，池序 137–140）

- **137 FIX-START-DRAFT-SIDE-EFFECT-1**（P1，0.5-1d）：启动读路径零落盘/零 draft 物化 + 自主 reader 不得破坏 pristine + 跨工程 uuid 泄漏守卫；118 最小修复保留为第二道防线。→ `todo/2026-10-18-FIX-START-DRAFT-SIDE-EFFECT-1.md`
- **138 FIX-START-STATE-RECOVERY-1**（P1 候选，0.5-1d）：启动状态恢复语义（不恢复/最近工程/显式）与 42MB 成本优化，先上交裁定后落代码。→ `todo/2026-10-18-FIX-START-STATE-RECOVERY-1.md`
- **139 TRACK-LIST-GAP-1**（P2 取证，0.5d）：track.list 0 轨同帧对照探针（三候选面），零生产代码。→ `todo/2026-10-18-TRACK-LIST-GAP-1.md`
- **140 FIX-TEST-DRAFT-ISOLATION-1**（P2 hygiene，0.5d）：既有测试隔离（全量测试前后真实 draft 根零新增）。→ `todo/2026-10-18-FIX-TEST-DRAFT-ISOLATION-1.md`
- shadow 刷新兜底（118 机制②的 agent 侧兜底）登记为 **P2 候选**（与 117 FIX-ASK-SILENT-GAP-1 同域），后续裁定并入或另卡，本批不立卡。
- 四卡派发确认=**待用户确认**；池序按发卡顺序排定。

## 三、栈与纪律

- 118 真栈腿释放：14:10–14:18（实际执行窗口；PC-RUNTIME-STACK 已登记释放条；六端口/本腿进程=0；用户真实 draft 根零写入）。
- **134 未登记未 GO 启动（纪律记录）**：exec-mastervol 14:18:49–14:19:07 以 dry-run 为由启动 kernel-only berth 并跑完 L0–L6；自报、隔离满足、拆栈干净，但未登记/未 GO，且与 118 拆栈有 2s 重叠（无功能踩踏）。裁定=该 run 仅作工程参考、不作验收证据；134 正式验收腿须 GO+登记后重跑（已实现 `-PreflightOnly` 整改 commit d7e11ff9）。
- 当前栈空闲；**用户 92 手测复测优先**；134 正式腿待用户测完+Lead GO。

## 四、待办面

- 用户：92-R1 复测（抽屉层级/F2 定位/过渡动效）；118 UI 手测 4 条可同场补测。
- 执行侧：134 正式 GO 重跑（~1min）；137-140 派发待用户确认。
- 池面：136 CAP-BOUNDARY-1（须排池序 54 后串行）；135 MEM-SCHEMA done/pass；132 done/pass。

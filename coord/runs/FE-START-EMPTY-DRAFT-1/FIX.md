# FE-START-EMPTY-DRAFT-1 修复记录：消除启动页双 draft（agent 侧最小修复）

- 卡：`coord/cards/doing/2026-10-15-FE-START-EMPTY-DRAFT-1.md`（池序 118）
- 执行 owner：DSH PC 执行流 `exec-start-draft`；分支 `port/fe-start-empty-draft-1`，worktree `D:/Vit_DAW_wt_fedraft1`，基线 origin/main `c33a5cfe`
- 归因依据：本目录 `ATTRIBUTION.md`（缺陷 A：双 draft 因果链）；**前端侧零改动**（不需要前端基线裁定）
- 修复范围：`agent/internal/history/history.go`（`ProjectNew` 复用原始空壳默认 draft）+ `agent/internal/history/history_test.go`（2 个回归单测）
- 文件域核对：`agent/internal/history` 在 task-4 writeScopes 内；未触碰 `agent/internal/projectstore`、`agent/internal/chat`、前端仓、共享主工作树、main

## 1. 修复内容

- 缺陷：agent 启动期任何读路径（`CurrentProjectIdentity` → `BindProjectIdentity("", uuid)` → `defaultDraftProjectPath`，见 `ATTRIBUTION.md` §1）会在 ProjectHistory draft 根物化一个只有 75B `Unsaved.vit` 的默认 draft（draft A）。用户从启动页点「新建工程」→ 前端 `version.project_new` → `history.ProjectNew` 原先**无条件清空进程内 draft 缓存再新建**（`history.go:777-793` 旧实现），于是每次进入都留下 draft A 弃件 + draft B 实件（四轮 100% + live5 旁证复现）。
- 修复：`ProjectNew` 在建新 draft 前先判定缓存中的默认 draft 是否仍是**原始空壳**（目录内仅 `Unsaved.vit`，无 `.vit_history`/`.vit_agent`/`.vit_derived` 等任何历史/会话/agent store 材料化痕迹）。是则**复用**该 draft 作为新工程 draft（清掉启动期旧 uuid 的身份绑定），否则维持原「清缓存建新 draft」语义。
- 语义保持：复用的 draft 与全新 draft 在后续初始化路径上完全等价（`ProjectNew` 之后 `BindProjectIdentity`→`EnsureWorkingSession`→`ActivateProjectStore` 的现有序列不变）；`ProjectNew` 的「新建工程=空历史」语义不变（有历史的默认 draft 不会被复用）。
- 可观测性：复用分支在 `ProjectNew` 返回 map 中新增 `"reused_default_draft": true`（仅新增键，不改既有键）。
- 不做的事：不修 `CurrentProjectIdentity` 的落盘副作用（根因，面大，建议另卡）；不修 42MB 启动恢复（设计裁定）；不修跨工程 `.vit_history` 泄漏（竞态，另卡/与根因同卡）；不修 shadow 刷新（117 域）；不修 `track.list`（内核/传输域，待栈内探针）。

## 2. diff 摘要

```
agent/internal/history/history.go      | 66 +++++++++++++++++-
agent/internal/history/history_test.go | 110 +++++++++++++++++++++++++++++
2 files changed, 175 insertions(+), 1 deletion(-)
```

关键新增：
- `defaultDraftProjectRoot()`（从 `defaultDraftProjectPath` 抽出的 root 解析，行为不变）
- `pristineDefaultDraftPathLocked()`（空壳判定：仅 `Unsaved.vit` 一个条目 + 缓存 root 匹配 + 文件存在）
- `clearProjectIdentityBinding(path)`（清掉启动期身份探测留下的绑定，避免旧 uuid 在复用 draft 上决定历史材料化位置）
- `ProjectNew` 复用分支 + `reused_default_draft` 返回键

## 3. 回归单测（新增）

- `TestProjectNewReusesPristineDefaultDraft`：复刻 agent 启动路径（`defaultDraftProjectPath()` → `BindProjectIdentity("", "vitproj_previous")`），随后 `ProjectNew()` 必须**复用同一路径**、`reused_default_draft=true`、draft 根**只有 1 个** draft 目录（原缺陷=2）；再绑定新 uuid 并 `EnsureWorkingSession` 后，断言新工程历史只材料化在 `.vit_history/vitproj_new`，**不得**出现旧 uuid `vitproj_previous` 目录（防复用路径把启动期旧身份带进新工程）。
- `TestProjectNewCreatesFreshDraftWhenDefaultDraftIsInitialized`：默认 draft 一旦已有 `.vit_history`（非原始空壳），`ProjectNew` 必须新建独立 draft（draft 根 2 个目录、无 `reused_default_draft`），保证既有「新建工程=全新历史」语义。
- 既有回归 `TestProjectNewStartsFreshDraftHistory`（新工程之间历史隔离）在修复后仍通过。

## 4. 验证（命令 + 退出码）

见本目录 `evidence/build.log`、`evidence/gotest.log`（复跑命令与退出码记录如下）：

| 验证 | 命令 | 结果 |
|---|---|---|
| 定向单测 | `cd agent && go test ./internal/history -run 'TestProjectNew\|TestDefaultDraftProjectPath' -count=1 -v` | PASS（4/4） |
| 相关包回归 | `go test ./internal/history -count=1` / `go test ./internal/harness -run 'TestProjectHistory\|TestOrdinarySave\|TestSaveAsProject' -count=1` | PASS |
| 全量编译 | `cd agent && go build ./...` | exit 0 |
| 全量单测（隔离 draft 根） | `VIT_HISTORY_DRAFT_ROOT=<temp>; go test ./... -count=1` | exit 0，95 包 ok / 12 包 no test files / **0 FAIL** |
| 测试隔离性核对 | 运行前后真实 draft 根目录数 | 本测试流净新增=0（+2 目录经核为同时段 **132 真栈腿** live 产物，非本测试流，保留不动；见 §5） |
| gofmt | `gofmt -l agent/internal/history/history.go agent/internal/history/history_test.go` | 两文件干净（包内其它文件为基线既有未格式化，未动） |

**未做真栈/UI 验证**（遵守：用户 92 手测优先、栈未放行不得启栈）：本修复无前端改动、无运行栈依赖；UI 类验证条款见 `ATTRIBUTION.md` §9 用户手测清单。真栈复验（启动页→新建工程→核对 draft 根只新增 1 个）需 Lead 排窗口后进行。

## 5. 测试隔离性偏差（如实登记，重要）

- 首次全量 `go test ./... -count=1` 时，仓库内**既有**测试存在 draft 根隔离缺陷：部分 history/chat/harness 测试未设置 `VIT_HISTORY_DRAFT_ROOT`，直接向真实 draft 根 `C:\Users\timoz\AppData\Local\Vit\ProjectHistory\drafts\` 写入了 4 个 `draft_20261018T0559*/*T0600*` 测试工件（内容为 `project-capacity`/`project-targeting-coverage` 等测试工程历史，无用户数据；见 `evidence/test_isolation_finding.md`）。
- 处置：4 个目录已**移出**真实 draft 根到隔离区 `C:\Users\timoz\AppData\Local\Temp\fedraft1\test_pollution_quarantine\`（未删除，可复核），真实 draft 根复核=0 遗留；随后全量复跑改为 `VIT_HISTORY_DRAFT_ROOT=<临时目录>` 环境隔离，真实 draft 根本测试流净新增=0。
  - 复跑窗口（14:02:30–14:05:15）真实 draft 根另新增 `draft_20261018T060416_4bcdde3a` / `draft_20261018T060418_5450cd7c` 两个目录，经只读核对为**同时段另一真实栈腿产物**（journal `source=pcactl.certify_compressor`、VitAgent pid 11452/VitApp pid 21624/VspHub 7844 在场、7878/5555/5556/8787 监听），**非本测试流写入，保留不动**；不计入测试污染净新增。
- 这与本修复无关（基线同样存在），但违反 AGENTS §10「测试不得写源码树/固定历史目录」精神，建议另开小卡处理测试隔离。**本卡未在本分支顺手修改其它测试**（越域）。

## 6. 回滚与边界

- 回滚：单 commit，`git revert <commit>` 即可；无持久化 schema/格式变更（`reused_default_draft` 仅为响应 map 新增键，读方无强校验）。
- 边界：修复只消除「原始空壳默认 draft 被弃用」这一双建直接成因；启动期读路径仍会物化 draft（现在被复用，不再产生弃件）。若未来出现「启动期已把默认 draft 用于旧 uuid 历史」（非空壳），新工程仍会新建 draft（保持安全但可能留下已使用 draft —— 那是旧 uuid 工程的历史载体，不应删除）。
- 未覆盖：`track.list` 0 轨、shadow 失同步、42MB 启动恢复、跨工程历史泄漏——均见 `ATTRIBUTION.md`，按 Lead 裁定另卡/另腿。

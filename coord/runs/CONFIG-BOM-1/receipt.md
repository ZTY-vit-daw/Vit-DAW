# CONFIG-BOM-1 回执（PC 执行侧）

- 卡片：coord/cards/doing/2026-10-09-CONFIG-BOM-1.md（领取提交 main=4b0d6d18）
- 分支 / worktree：port/config-bom-1 / D:/Vit_DAW_wt_bom（主工作树既有改动未触碰）
- 领取时 origin/main：60893c74
- 领取时 worktree git status --short：空（clean，基于 origin/main=60893c74 建树）
- 实现提交：见下方「commit hash」（领取行与回执行回填同一 hash）

## 缺陷与修法

- 缺陷：`agent/internal/chat/audition_events.go` `auditionBlindSettingsFor()` 中 `json.Unmarshal(data, &config)` 不接受 UTF-8 BOM（Go encoding/json 行为）；Windows PowerShell 5.1 `Set-Content`/旧记事本默认写 BOM，用户写 `{"audition_blind": true}` 即落 `config_invalid` 面，blind 意图静默失效。
- 修法：`os.ReadFile` 之后、`json.Unmarshal` 之前 `data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})`——仅此一处容错；其余解析失败语义不变（`config_invalid` fail-visible 保持，有新测试锚定 BOM+malformed 仍报 invalid）。
- 停止条件核查：BOM 容错与既有 config_invalid 语义测试两全，无冲突，未触发上交条件。

## Diff（vs 60893c74）

文件域严格两文件：

- `agent/internal/chat/audition_events.go`（+8/−1）：
  - import 增加 `bytes`；
  - Unmarshal 前剥 BOM 前缀（含 5 行注记 CONFIG-BOM-1）；
  - 另含 1 行 whitespace 修复：`"goal_id":      loop.GoalID` → `"goal_id": loop.GoalID`（~1148 行，`history.Checkpoint` 字面量）。**注记：该 misalignment 为 HEAD 既有（`git show HEAD:... | gofmt -d` 复核确认），非本卡引入；因本卡验收门要求 gofmt 净且该文件在卡面文件域内，随卡一并修复，零行为变化。**
- `agent/internal/chat/audition_blind_config_test.go`（+39）：三个新用例（进既有测试文件，未新建文件）。

## 新增测试名

1. `TestAuditionBlindConfigFileWithBOMEnablesBlindTier` — BOM+`{"audition_blind": true}` → enabled=true 且 source=`config:`+path（验收标准①）
2. `TestAuditionBlindEnvironmentWinsOverBOMConfigFile` — BOM 文件存在时 env 仍赢（验收标准③）
3. `TestAuditionBlindBOMDoesNotRescueMalformedConfig` — BOM+`{not json` 仍 `config_invalid`（守停止条件边界：仅剥前缀，不救其余解析失败）

既有 6 个 config 面 Blind 用例（无 BOM 路径）回归全绿，行为零变化（验收标准②）。

## 验收门结果（全部在最终代码状态执行）

| 门 | 命令 | 结果 |
|---|---|---|
| 编译 | `cd agent && go build ./...` | 退出码 0 |
| Blind 测试 | `go test ./internal/chat -run 'Test.*Blind' -count=1` | ok（含新 3 用例 + 既有用例全 PASS） |
| 全量 | `go test ./... -count=1` | **GO_TEST_EXIT=0，FAIL_LINES=0，PKG_OK=90** |
| gofmt | `gofmt -l`（两文件 LF 归一化副本） | 无输出（净） |

## gofmt 口径注记

worktree 为 Windows `core.autocrlf=true` 检出，`gofmt -l` 对 chat 包全部 ~230 文件报 CRLF 伪影（含未触碰文件，`gofmt -d` 显示整文件逐行差异即换行符形态）。故 gofmt 净按内容口径验证：两文件 CR 剥离后在临时目录（源码树外）跑 `gofmt -l` 无输出、`gofmt -e` 解析通过。真实内容缺陷仅上述 1148 行既有 misalignment，已随卡修复。

## 覆盖边界声明（AGENTS.md §5）

本卡未跑端侧三件套烟测：改动为单文件解析容错单点（读配置路径），卡面验收标准即为单测+全量+gofmt 四条，全部达成。渲染面与用户旅程无涉。端侧烟测豁免请决策侧裁定。

## 工件

- 实现提交：（port/config-bom-1 commit hash，见卡片回执行）
- 全量测试原始输出：临时文件已清理（退出码 0 / 0 FAIL / 90 ok 已记录于本回执）

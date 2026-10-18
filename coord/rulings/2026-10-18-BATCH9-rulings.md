# 2026-10-18 晚窗 批量九裁定（140 测试隔离 pass + 139 真栈残留隔离 + 栈脚本隔离卫生卡）

- 验收会话：12:00-18:00 决策流（DSH PC，本机）；裁定来源=用户 2026-10-18 确认派发（140）+ 137/139 真栈窗口收口。
- 基线：origin/main=**96cd1fec**（140 验收前）；本批合入后 main=dc62aeec。
- 执行侧：DSH exec-start-draft（task-14）；协调面由 12:00-18:00 决策流单写。

## 一、FIX-TEST-DRAFT-ISOLATION-1（140）——**pass**

- 交付：分支 **port/fix-test-draft-isolation-1@ead01a47**（已推，worktree 干净；实际基线 origin/main=3e60df15；卡面领取行 3abe0cfb 为发卡时点）。
- 定位（真实根、未设隔离 env）：107 包逐包串行扫描 → **仅** `internal/chat`（+2 目录）与 `internal/harness`（+1 目录）写真实 draft 根，其余 105 包 0 新增 0 失败；逐测试/时间映射命中 chat **9 个**测试（free_state_owed_round_guidance_test.go:126、fs_adopt_closure_test.go:28、server_test.go:2397/:3675、traj_auto_settle_test.go:441/:547/:729、free_state_arm_slot_restore_test.go:87/:136）+ harness **1 个**（loudness_watch_hook_test.go:101）；根因=`defaultDraftProjectPath` 进程内缓存——每包首个无 env 写路径物化一次真实 draft（LOCALIZATION.md + 108 逐测试日志 + pollution maps 在卷）。
- 修复（**零产品代码**）：`agent/internal/chat/server_test.go` 与 `agent/internal/harness/testmain_test.go` 的既有 TestMain——未外部提供 `VIT_HISTORY_DRAFT_ROOT` 时创建 per-run 临时根并注入、`m.Run()` 后删除；外部 CI env 优先；测试内既有 `t.Setenv` 继续生效。
- 决策侧独立复验（亲做）：diff 亲读（仅两处 TestMain 测试辅助）；**独立全量复跑（未设隔离 env）**：`go build ./...` exit 0；`go test ./... -count=1` **exit 0（95 包 ok/12 no test/0 FAIL，118.8s）**；真实 draft 根 **10185→10185（delta=0）**；changed files gofmt 净（`cmd/*` 既有未格式化文件非本卡改动）；远端 hash 亲核。
- 边界：定位窗口（17:56–18:10）与 139 真栈腿重叠，真实根出现 4 个**运行时** draft（非测试污染，见 §二）；本卡定位过程自身 11 个测试污染目录已移入 `%TEMP%\fedraft1\task14_test_quarantine\`（未删）；118 隔离区 4 个 draft 未删。
- 合入：cherry-pick ead01a47 入 main；卡 140 归 done。

## 二、139 真栈腿真实 draft 根残留隔离（Lead 执行）

- 定位：真实根 4 个运行时 draft（18:04:25/18:04:26/18:05:47/18:07:58，均含 `.vit_agent`+`.vit_history`）来自 139 真栈腿 agent（该 berth 未设 `VIT_HISTORY_DRAFT_ROOT`）。
- 处置：**未删除**，整体移入 `%TEMP%\fedraft1\stack_leg_orphan_drafts\`（含 MOVED_LIST manifest）；真实 draft 根 **10185→10181**（恢复至 139 窗口前水平）。
- 根因面：真栈/烟测脚本隔离不一致（118/134/137 有隔离、139 无）→ 新立卫生卡 **FIX-STACK-DRAFT-ROOT-ISOLATION-1（池序 144，P2，待用户确认）**：审计全部真栈/烟测脚本→默认在 run 目录隔离 draft 根+启动前断言+代表腿前后真实根零新增。

## 三、当前交付面

- **R2 启动页**（116）：R2.1 内部验收通过（186 钉/三尺寸 delta=0/标签安全区/列表页网格/回归全绿/红线零），**待用户 R2 worktree 复测 5 项**；通过后 ff 合 frontend main。
- **141 MIXER**：修复已在 frontend main（3ab4dfd），待用户真栈复测（新建轨道→mixer 刷新）。
- **143 FIX-TRACKLIST-SHADOW-FRESHNESS-1**（P1 候选）、**144**（P2 卫生）、**138**（启动恢复语义）→ 待用户确认/裁定。
- 其它：126 KERNEL-L3-TIMEENERGY（已确认待派）、136 CAP-BOUNDARY（须排池序 54 后）、129 宏设计→130。

## 四、纪律与边界

- 139 / 134 未登记未 GO 启动事故均已自报并登记；整改 `-PreflightOnly` 落地（d7e11ff9 / 75f3caf2）。
- 真实 draft 根全程有清单/时间戳证据链；隔离区证据一律未删（由决策侧后续处置）。

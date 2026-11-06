# CHAT-TEST-ISOLATION-1（池序 259）工件

- owner：GLM-5.3 · PC · ZCode 执行会话；worktree `D:/Vit_DAW_wt_259impl`（port/chat-test-isolation-1）
- 领取：2026-11-06 午后；领取基线 origin/main=929d6cd9；领取协调提交 964c8026
- 实现提交：见回执（本目录工件随 port 分支同批入库）

## 修改面（+43 / -0，全部 chat 测试文件，零生产改动）

| 文件 | 内容 |
|---|---|
| server_test_helpers_test.go | 新增 `isolateUnboundMixboardRootForTest(t)`：`t.Setenv("VIT_MIXBOARD_ROOT", t.TempDir())`——`mixboard.DefaultRoot()` 中该 env 优先于 projectstore 全局绑定与 `%TEMP%/vit-daw-unbound/<pid>` 回退，把 loudness watch verdict/snapshot 的 unbound 解析钉到 per-test 目录（G1 注入缝走测试 env 注入，生产面 0 行，卡面 ≤5 行上限内） |
| loudness_watch_trigger_test.go | 3 个无 Activate 测试（ProjectChangeHook×2 + ConfigureRoutes）接入隔离（G1） |
| loudness_watch_state_test.go | 5 个裸 `New(nil,nil,nil)` server 测试接入隔离（G1） |
| loudness_watch_persist_race_test.go | 2 个测试（含子测试）接入隔离（G1） |
| server_test.go | TestMain 收尾清理本进程 pid 域 `%TEMP%/vit-daw-unbound/<pid>`（残留清扫，G1「t.Cleanup 清残留」的进程级兜底；晚于 t.Setenv 还原的异步 goroutine 落盘也扫掉） |
| free_state_conclusion_bridge_test.go | G2 测试写后加 projectstore 绑定漂移检测断言（该测试本已 per-test Deactivate+Activate 自建 store；残留顺序敏感向量=外来异步 ActivateProjectStore 翻全局态，漂移时以明确归因失败替代「conclusion dir 不落盘」式模糊失败） |

## G4 处置

- `TestV2ProjectStoreSurvivesRestartAndContinues`：隔离复跑 PASS 0.15s（g4_harness_isolated.txt）；全仓跑中 harness 包整体 ok（fullrepo_go_test.txt）——不复现，按卡面记录。

## 验收命令与结果

| 命令 | 结果 | 工件 |
|---|---|---|
| `go test ./internal/chat/ -count=1`（第 1 轮） | ok 141.126s，EXIT=0 | chat_round1.txt |
| `go test ./internal/chat/ -count=1`（第 2 轮） | ok 141.332s，EXIT=0 | chat_round2.txt |
| `go test ./... -count=1`（全仓） | 96 包 ok / 0 FAIL，EXIT=0 | fullrepo_go_test.txt |
| 修复后 unbound 残留核查 | 两轮后无新增 pid 目录（对照：修复前基线轮 pid 13384 留有 acoustic_package_status.json+mixboard 会话） | 本 README |

## 附带观察（不扩卡面，供主管参考）

- 修复前基线轮实证：chat 包仍有非 watch 面测试（agentloop/chat 事件与 D1 会话持久化路径）向 `%TEMP%/vit-daw-unbound/<pid>` 写 mixboard 会话与 acoustic_package_status.json（本次 pid 13384 目录）。本卡未逐一收纳（超出 G1 verdict-store 路径族与卡面文件域），TestMain 清理兜底防跨轮致敏；若后续出现同族 must-not-exist 断言失败，建议另开卡按同一 `isolateUnboundMixboardRootForTest` 模式收纳。
- baseline_chat_round0.txt 为修复前基线（ok 173.589s），作 before/after 对照。

# FIX-CHAT-TMPDIR-FLAKE-1：chat 包 TempDir 清理竞态族根治（Windows）

- 池序 9；来源=AGENTS §11 重复出现规则触发——2026-09-30 同日两卡全量 run 命中：VITNOTE-IMPL-4（早窗，首见）+ CLIPSCOPE-AGENT-1（晚窗，再现）；裁定见 [rulings/2026-09-30-CLIPSCOPE-AGENT-1-pass.md](../../rulings/2026-09-30-CLIPSCOPE-AGENT-1-pass.md)
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / 无；agent chat 测试域
- 模型分级：L1 / flash 可接
- 已核实事实（两次失败原始输出均在两卡回执留档）：
  - 失败形态：`TempDir RemoveAll cleanup: unlinkat ...TestWorkspaceSwitchSettlesInFlightChainExplicitly<rand>\001\.vit_agent\vitproj_b5_switch_b: The directory is not empty`——Windows 目录清理竞态，非功能断言失败。
  - 两次均隔离复跑 PASS+整包复跑 PASS。
  - 同族已知项：`TestProcessorCertificationStart*`（CURRENT-STATE 已知不稳定项在案，Windows TempDir 清理竞态同族）。
- 目标：
  1. 族根治：`.vit_agent` 工作区目录的测试句柄释放与清理时序修复——方向（领取后实锚定案）：测试收尾显式关闭/释放内核句柄后再交还 TempDir 清理；或状态目录改用测试专用 TempDir 子目录+清理重试；或 t.Cleanup 顺序修正。要求对两测试名同源生效，不逐测试打补丁。
  2. 验证：`TestWorkspaceSwitchSettlesInFlightChainExplicitly` 与 `TestProcessorCertificationStart*` 两名隔离各 3 轮+整包 2 轮全绿。
- 文件域：agent/internal/chat/ 测试文件（若需生产侧目录参数化小改，越域申报）。
- 验收标准：上述验证轮次全绿+gofmt+全量 0 FAIL；回执附各轮退出码。
- 停止条件：取证发现竞态源于生产代码句柄泄漏（非测试清理路径）→ 上交转生产缺陷卡。
- 领取：2026-09-30 19:32 / origin/main=c4a10d54398ed3b12a7596d0e68d58892116c824 / port/fix-chat-tmpdir-flake-1（独立 worktree D:/Vit_DAW_worktrees/fix-chat-tmpdir-flake-1）；领取时工作树预存改动=VitApp/Workspace/{Settings.xml,default_project.xml}（运行时工程态，非本卡，不动）+暂存区并行卡 FIX-BUCKET-SAVE-RACE-1 领取 rename（他卡会话，不纳入本卡提交）
- 回执：（commit hash / 根治方案一句话 / 验证轮次记录）
- 验收：（裁定文件 / 验收 commit）

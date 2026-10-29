# FE-BRIDGE-SPILL-NS-COLLIDE-1：agent spill 文件名纳秒位恒 0——同秒同 cmd+rid 碰撞覆盖（209 勘察上交瑕疵）

- 发卡：GLM 主管决策流 / 2026-10-28 晚（依据=209 勘察上交：bridge.go:213-216 格式串字面零+用户会话 4 次 spill 仅留 2 文件亲证）
- 派发确认：待用户确认（P2 低危，闲时/量产档可做，不阻主线）
- 验收负责人：GLM 主管决策流
- 池序 210；目标仓库=**主仓 agent/（Go）**；P2 / 0.25 天 / 依赖=无
- 模型分级：L1 / 量产引擎可做（DeepSeek/flash 窗口）

## 根因（209 勘察亲核）

`agent/internal/bridge/bridge.go:213-216` 文件名时间戳 `time.Now().Format("20060102_150405_000000000")`——Go 格式串 `0` 系为字面零，纳秒位恒 `000000000` → 同秒同 cmd+rid 的 spill 文件名相同互相覆盖（209 用户会话 4 次 env_lane spill 仅留 2 文件实证；同 rid 内容相同无读损坏=低危留痕面，但不同路径并发下存在覆盖异容风险）。

## 目标

1. 文件名纳秒位取真实值（格式串改 `999999999` 或 `Nanosecond()` 拼接，形态从简）；
2. 同秒碰撞防护兜底（如已存在则追加序号/O_EXCL 重试——最小面）；
3. Go 单测：同秒两次 spill 文件名不同+内容各自完整；既有 bridge 测试零回退。

## 文件域

`agent/internal/bridge/bridge.go`+`agent/internal/bridge/*_test.go`。越域即停。

## 停止条件

- 文件名格式另有消费方依赖（外部工具按恒零格式匹配）→上交锚点。

## 领取

- 领取：2026-10-29 18:47 晚窗 / 主仓 origin/main=5d1d5eb786b078dced9f7ad0b2ee57227e9f276c / owner=GLM-5.3-flash ZCode PC 量产执行会话（L1 档）/ 分支=port/fe-bridge-spill-ns-1 / worktree=D:/Vit_DAW_worktrees/fe-bridge-spill-ns
- 派发确认依据：用户 2026-10-29 晚窗会话直接指派领取本卡（池序 210），授权依据记录于领取会话
- 领取基线记录（AGENTS §12）：worktree `git rev-parse HEAD`=5d1d5eb786b078dced9f7ad0b2ee57227e9f276c（=origin/main，含 213 领取提交）；`git status --short`=零改动；`git diff --stat`=空。主检出 D:/Vit_DAW 既有未提交改动（Settings.xml/default_project.xml/coord 工件）为领取前已存在，本卡不触碰
- 领取前核对：fetch 后重读，卡仍在 todo；doing=GEN-SUNO（Mac 设计卡）+213（前端仓 GDScript 域），与本卡 `agent/internal/bridge/` 文件域零交集；本卡离线可验，不占真栈（PC-RUNTIME-STACK 无需登记）
- 环境注记：worktree 首建因 `coord/runs/FS-LARGEPROJECT-SMOKE-1` 长证据路径超 Windows 260 限制失败一次；已设仓库本地 `core.longpaths=true` 后重建成功（共享仓库配置变更，随本卡回执披露）

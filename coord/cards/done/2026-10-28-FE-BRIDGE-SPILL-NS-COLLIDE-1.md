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

## 回执

- 实现 commit：de8fa121 @ `port/fe-bridge-spill-ns-1`（已推 origin；**不自行 merge，留分支待主管验收**）；改动=bridge.go+新增 spill_filename_test.go，2 文件 +215/−6，未越域
- 修复面：`spillControlReply` 与 `spillTelemetryPacket` 两处 `Format("20060102_150405_000000000")` 字面零（Go 版式小数秒 token 须为秒后 `.000` 形态，下划线后 0 系全字面）→ 新增 `spillFileStamp`（`Format("20060102_150405")`+`Nanosecond()` `%09d` 拼接）+ `writeSpillFileExclusive`（O_EXCL 原子建文，同名 `_1.._N` 序号重试，100 次上限后报错不覆盖）。卡面只点名 control reply 侧，telemetry 侧同款同根因、同文件域内一并修
- 停止条件核对：全仓 grep `000000000`——harness/macrocontrols/runtime 等其他用法均带点（`.000000000`=真实取值），缺陷面恰为 bridge 两处；既有测试仅断言文件名含 cmd/label+无路径分隔符，未发现恒零格式消费方，零回退
- 验收命令与退出码（worktree agent/ 下）：`go build ./...` exit 0；`go test ./... -count=1` 三轮均 96 包全 ok、零 FAIL；新增 5 测试 `-v` 逐条 PASS：TestSpillFileStampHasRealNanosecondSegment / TestSpillControlReplySameSecondRequestsGetDistinctFiles / TestSpillTelemetryPacketSameSecondLabelsGetDistinctFiles / TestWriteSpillFileExclusiveSuffixesOnCollision（`_1` 序号+首文件不覆写）/ TestWriteSpillFileExclusiveGivesUpAfterBoundedCollisions（耗尽报错）
- 端测覆盖边界声明（AGENTS §5）：本卡卡面明示「离线可验、不占真栈」——以上为离线证据（单测+行为断言），未跑真实栈烟测；spill 属 agent UDP 溢出降级留痕链，无 GUI/渲染/用户旅程面
- 领取提交：158dc22a（claim todo→doing，仅 coord/ 变更直推 main）

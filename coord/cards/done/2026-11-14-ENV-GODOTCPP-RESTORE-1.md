# ENV-GODOTCPP-RESTORE-1：godot-cpp 双位置异常取证与恢复

- 发卡：GLM 主管决策流 / 2026-11-14 晚窗（依据=[UI-BRIDGE-1 终验环境登记](../../rulings/2026-11-14-UI-BRIDGE-1-pass.md)）
- 派发确认：已确认（主管发卡）；P3（extension cpp 改动卡的前置；现卡不受阻——dll 为 git 工件）
- 验收负责人：GLM 主管决策流
- 池序 353；P3 / 0.5 天 / 依赖=零；PC；零栈

## 事实

- 343（~19:0x）：worktree 编译借 junction 用 godot-cpp（当时前端主检出存在）；
- 349（~19:2x）：前端主检出 godot-cpp 已空，真实依赖 `D:/Vit_DAW/godot-cpp`；
- 348（~20:4x）：两位置均报「被清空」；但主管 22:3x 查 `D:/Vit_DAW/godot-cpp` 顶层文件在（CMakeLists/SConstruct 等）——**实态与报告出入待核**（可能仅 obj/bin 缺失）；
- 今晚多次 worktree 创建/强制移除（内含 junction 借用，主管亦两次 `git worktree remove --force` 含 junction 的 worktree）——**根因未定，junction 穿透删除为候选假设之一**；
- 恢复源候选：`D:/备份/vit-daw-frontend/godot-cpp`（488MB，**版本相符性未核对**——extension 按 api_version=4.6 编译）。

## 目标

1. 取证：两位置现状清单（顶层/obj/bin 各缺什么）；从今晚各卡 PROGRESS/时间线还原消失时点；给出根因结论（junction 穿删除/其他）+证据。
2. 恢复：核对备份版本（godot-cpp 版本文件/头与 extension 编译期望 api_version=4.6 对照）→ 恢复 `D:/Vit_DAW/godot-cpp` 至可编译态（前端主检出是否恢复候裁定——junction 借用模式已可满足）→ 试编译 extension 一轮 exit 0 亲证。
3. 预防：验证 PROTOCOL §3 新 junction 拆除纪律的可执行性（给出拆除命令样例）。

## 文件域

`D:/Vit_DAW/godot-cpp`（恢复目标，仓外目录）、`D:/Godot/project/vit-daw-frontend/godot-cpp`（如裁定恢复）、coord/runs/ENV-GODOTCPP-RESTORE-1/ 工件、卡面。零仓内源码改动。

## 验收标准

现状清单+时间线根因结论+恢复后 extension 编译 exit 0 亲证+工件在卷。

## 停止条件

备份版本不符且无其他恢复源 → 上交候裁定（重下 godot-cpp 4.6 需网络与用户确认）。

## 领取

- 时间：2026-11-14 21:49
- 领取时 origin/main：2110e5cf（派发基线 bfe0356e 的直接后代——fetch 时主管已推 352 状态同步 coord-only 提交，ancestor 关系已核；worktree 建于最新 origin/main 之上）
- owner：freeflash（GLM-5.3 Flash）· PC · ZCode 免费车道执行子代理（主管会话派发，会话标识见派发提示词）
- 分支：port/env-godotcpp-restore-1（基于 origin/main 2110e5cf）
- worktree：D:/Vit_DAW_wt_gcpp1（领取时工作树干净，status 0 项）
- 并行核对：doing 卡 ARRLOOP-AUTHBRIDGE-1 / CLOSURE-ROUTE-ADAPT-1 / WAVESHELL-LOADING-FORENSICS-1 / LAUNCHER-ZEROFRICTION-1 / UI-LOADING-1 文件域均与本卡（仓外 godot-cpp 目录 + 本卡 coord/runs 工件）不相交

## 回执

- 时间：2026-11-14 22:08；执行分支 port/env-godotcpp-restore-1（已推远端，tip ed821ec2）；领取提交 5bb10b0a（coord-only → main，远端 owner 已核对）
- 工件（均在分支 port/env-godotcpp-restore-1）：coord/runs/ENV-GODOTCPP-RESTORE-1/{FORENSICS.md, restore-version-check.md, VERIFY.md, PROGRESS.md, robocopy_restore.log, scons_debug.log, scons_release.log}
- 结果对照验收标准：
  1. **现状清单**：A（D:/Vit_DAW/godot-cpp）与 B（前端主检出 godot-cpp）均为完全空目录（0 文件），mtime A=11-14 19:46:43 / B=11-14 18:57:14，均非 reparse point。卡面"主管 22:3x 见顶层文件在"与 A 的 NTFS mtime 不相容（19:46:43 后 A 顶层无变动）——最可能为时间标注误差或路径混淆（备份目录顶层恰为 CMakeLists/SConstruct 等，与描述吻合），详见 FORENSICS.md §3，如实上交不影响恢复。
  2. **时间线+根因**：根因（高置信，机制层）=worktree 收尾拆除时删除动作穿透 godot-cpp junction 递归删除目标内容，链接随后被摘，留下空实体目录。执行者归因 open：候选 1=HOST/OS 会话收尾清理（mtime 18:57/19:46 直接吻合，但两卡 PROGRESS 文面写"保留待处置"）；候选 2=主管 20:45/21:0x 两次 force remove（晚于 mtime，若时刻准确则其 remove 时目标已空）。两候选共享同一根因层=拆除前未摘链接（§3 针对层）。USN 日志需管理员未读（证据局限）。
  3. **恢复+编译亲证**：备份核对=4.2 分支 4bc6e67（godot-4.2.2-stable-74）、tracked 内容与 HEAD 零差异、fsck 过、哈希抽样在卷；**卡面 api_version=4.6 前提被证据修正**——4.2 SConstruct 对未知变量警告后忽略（明文），OS 卡 19:3x 实际按 4.2 API 编译成功，.gdextension compatibility_minimum="4.2" 自洽，今晚 HOST/OS 真栈全 PASS 实证 4.2 系 dll 被 4.6.1 加载。版本相符放行（未触发停止条件）；robocopy /MIR RC=1 恢复 A（3832 文件+哈希一致）；临时 worktree（origin/port/ui-os-1=128477b）junction 借用，scons 双变体 **template_debug exit 0 + template_release exit 0**（log 在卷），产物经 junction 落 A（bin 重链/gen 重生成/1848 obj，git status 0）。勘误：备份含 git-ignored 预编译 obj 缓存（restore-version-check.md 补记）。
  4. **预防（§3 可执行性）**：Git Bash 实测——`cmd //c rmdir '<反斜杠路径>'`（外层单引号无内层引号）rc=0 仅删链接，含空格路径样例亦过；内层双引号形式失败（"文件名、目录名或卷标语法不正确"）；mklink /J 的 /J 会被 MSYS 路径转换吞掉，建链建议 PowerShell New-Item -ItemType Junction。建议补进 §3 条文（VERIFY.md 有对照表）。
- 端测边界声明：本卡零栈（卡面注明），无真栈腿；验收标准=恢复后 extension 编译 exit 0 已亲证；真栈加载级验证不在本卡域（dll 为 git 工件，今晚 HOST/OS 真栈已证同配置 dll 被 4.6.1 加载并全 PASS）。
- 遗留：①前端主检出 B 是否恢复=候裁定（junction 借用模式已满足编译链）；②根因执行者归因 open（无主管侧命令日志工件，不作定论）；③"22:3x"观察矛盾上交；④USN 日志取证需管理员，如需可由主管在有权限窗口补读。

## 验收

- 待回填
<<<<<<< HEAD

## 验收（主管 2026-11-14 深夜）

- 裁定：**pass**——恢复体主管亲核（include/godot_cpp 全在）；scons 双变体 exit 0 日志亲证；§3 rmdir 拆链验证接受并已补进 PROTOCOL 条文（Git Bash `cmd //c rmdir <反斜杠路径>` 无内层引号形式）。
- 根因结论采纳（高置信/open 归因）：worktree 收尾删除穿透 junction 递归删空目标——mtime 证明早于主管 force remove，非主管操作；预防=§3 新纪律（已落地）。
- 前端主检出 B 不恢复（junction 借用已满足，裁定维持）；「22:3x 观察矛盾」按报告记录为 open（不阻塞）；USN 日志取证局限接受。
- 工件自主干分支合入 main；worktree 已清理；port/env-godotcpp-restore-1 留远端存档。
=======
>>>>>>> 3ce55cacfe8e120f31af24151c382000b545904e

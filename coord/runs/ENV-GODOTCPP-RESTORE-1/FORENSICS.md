# ENV-GODOTCPP-RESTORE-1 取证报告（2026-11-14 晚，freeflash 执行腿）

## 1. 两位置现状清单（21:49–21:53 现场核查）

### 位置 A：D:/Vit_DAW/godot-cpp（主仓 git-ignore 真实依赖）
- **完全空目录：0 文件、0 子目录**（`find -type f | wc -l` = 0；`ls -la` 仅 . ..）
- 目录本身存在：CreationTime 2026-09-19 10:07:33，**LastWriteTime 2026-11-14 19:46:43**
- 属性：普通 Directory，**非 reparse point**（无 junction 残留）
- 与卡面「主管 22:3x 查顶层文件在（CMakeLists/SConstruct 等）」**不符**：现无任何顶层文件（详见 §4 时间矛盾分析）

### 位置 B：D:/Godot/project/vit-daw-frontend/godot-cpp（前端主检出，git-ignore）
- **完全空目录：0 文件**（`find | wc -l` = 1，仅目录本身）
- 目录本身存在：CreationTime 2026-03-23 18:15:40，**LastWriteTime 2026-11-14 18:57:14**
- 属性：普通 Directory，非 reparse point；`dir /AL` 父目录无 junction 条目

### 备份：D:/备份/vit-daw-frontend/godot-cpp（488MB，恢复源候选）
- 顶层完整：CMakeLists.txt / LICENSE.md / Makefile / README.md / SConstruct / __pycache__ / bin / binding_generator.py / build_profile.py / cmake / doc / gdextension / gen / include / misc / msvc_capture.log / pyproject.toml / src / test / tools
- **注意：备份顶层正是 CMakeLists/SConstruct 等——与卡面「主管见顶层文件在」的描述吻合，存在路径混淆可能（见 §4）**
- 版本核对见 restore-version-check.md（下一步）

### 其他现场
- 主仓 .git/worktrees：无 wt_uihost1 / wt_uios1 / wt_uibridge1 注册（已移除）
- 前端仓 git worktree list：无 wt_uihost1 / wt_uios1 / wt_uibridge1（已移除）；D:/Godot 根下三个 wt 目录均已不存在
- 抽查现存前端 worktree（sendext1 / wt-rackdel1 / lchat1）：无 godot-cpp junction 残留
- USN 日志读取：`fsutil usn readjournal D:` → 错误 5 拒绝访问（需管理员；按边界不提权，记为证据局限）

## 2. 今晚时间线（NTFS mtime + 三卡 PROGRESS 交叉锚定）

| 时刻（11-14） | 事件 | 证据 |
|---|---|---|
| 18:38–18:44 | 343/UI-HOST-1 真栈 4 轮 run（junction 借用「主检出」godot-cpp 编译 extension，183806→184412） | UI-HOST-1 PROGRESS S5/runs 目录名 |
| ~18:5x | HOST 收尾（回执/done 推送 b81438ae） | UI-HOST-1 PROGRESS S6 |
| **18:57:14** | **B（前端主检出 godot-cpp）内容被清空**（此后顶层再无变动） | B 目录 LastWriteTime（NTFS 硬证据） |
| 19:2x | 349/UI-OS-1 领取勘察：发现「主检出 frontend/godot-cpp 为空目录，真实依赖在主仓 git-ignore 目录（A）」 | UI-OS-1 PROGRESS S5 |
| 19:3x | OS 建 junction wt_uios1/godot-cpp → **A**，scons api_version=4.6 双变体编译过（A 此时仍有内容） | UI-OS-1 PROGRESS S5/runs 193333–193656 |
| 19:36–19:40 | OS 烟测 PASS（run_193656 exit 0）、拆栈、回执 | UI-OS-1 PROGRESS S8–S10 |
| **19:46:43** | **A（D:/Vit_DAW/godot-cpp）内容被清空**（此后顶层再无变动） | A 目录 LastWriteTime（NTFS 硬证据） |
| 20:31 | 348/UI-BRIDGE-1 勘察：「D:/Vit_DAW/godot-cpp 与前端主检出 godot-cpp 均为空实体目录（UI-OS-1/HOST-1 worktree 已被清理，其 junction 消失）」 | UI-BRIDGE-1 PROGRESS 环境备注（20:3x 发现） |
| 20:31–21:07 | BRIDGE 真栈 8 attempts（extension cpp 零改动，免重编） | UI-BRIDGE-1 PROGRESS §8 |
| ~20:45 / ~21:0x | 主管两次 `git worktree remove --force`（派发提示词称含 junction 的前端 worktree；**本腿无工件可直接核此两次操作的时刻**） | 派发提示词（间接证据） |
| 21:07 | BRIDGE PASS run_210725 exit 0 | UI-BRIDGE-1 PROGRESS |
| 21:21:47 | 主管写 BRIDGE pass 裁定（环境登记：godot-cpp 双位置异常 → 开卡 353） | 裁定文件 mtime |
| 21:49 | 本腿领取并核查：A/B 均完全空 | 本报告 §1 |

## 3. 根因结论（含证据强度）

**结论：高置信度——B/A 的内容清空是「worktree 收尾拆除时删除动作穿透 godot-cpp junction、递归删除了链接目标的内容」所致；junction 本体随后被一并摘除，留下两个空实体目录。**

推理链：
1. 两卡 worktree 内各有一条 junction：HOST wt 的 godot-cpp 借用「主检出」（=B），OS wt 的 godot-cpp → A（各卡 PROGRESS 明文）。
2. B 最后内容变动 18:57:14 = HOST 收尾窗口；A 最后内容变动 19:46:43 = OS 收尾窗口。两个时刻均恰在各该会话收尾、且在对方会话还依赖另一位置之前（OS 19:2x 已证 B 空，故 OS 改借 A；BRIDGE 20:31 证两者皆空）。时序闭环，无第三方删除窗口。
3. 终态形态（目标目录存活+内容全空+链接消失+worktree 消失）与「递归删除穿过 reparse point」的已知 Windows 语义一致（`git worktree remove --force` 的递归删除或会话侧 `rm -rf` 穿链接，两者终态相同）。

**具体执行者归因（无法从现有证据唯一确定，如实分列）：**
- 候选 1：HOST/OS 会话自身收尾时的 worktree 清理（18:57/19:46）——mtime 直接吻合；但两卡 PROGRESS 文面均写「worktree 保留待决策侧处置」，与自行清理矛盾（可能文面写于清理决定之前，或仅指主仓 worktree）。
- 候选 2：主管 20:45/21:0x 的两次 `git worktree remove --force`——派发提示词明言；但其时刻晚于 A/B 清空 mtime（18:57/19:46）。若 20:45/21:0x 准确，则主管 remove 时目标已空、junction 已消失，穿透删除的执行者应为其前的会话收尾；若派发时刻为约数，主管 remove 即为穿透执行者。**本腿无主管会话的命令日志工件，无法裁决。**
- 两个候选共享同一根因层：**拆除含 junction 的 worktree 时未先摘链接**（§3 新纪律针对的正是这一层）。机制层面置信度高；进程/命令级归因证据不足，标注为 open。

**「主管 22:3x 顶层文件在」矛盾分析**：A 的 LastWriteTime=19:46:43，若 19:46:43 之后（含 21:3x/22:3x）有人删除 A 顶层文件，mtime 必然更新——故「22:3x 见 A 顶层有文件」与 NTFS 硬证据不相容（除非该观察发生在 19:46:43 之前，或「22:3x」为昨晚 11-13 的观察、或时间标注误差）。另一可能：观察对象实为 **D:/备份/vit-daw-frontend/godot-cpp**（其顶层恰为 CMakeLists/SConstruct 等，与描述吻合，路径仅差「备份/」一段）。此矛盾不影响恢复决策（A 现态为空已亲证），但如实上交。

**证据强度分级**：
- 硬证据：A/B 现态全空（亲证）；A/B mtime 19:46:43 / 18:57:14；三卡 PROGRESS 的 junction 指向与发现时点；备份顶层清单（亲证）。
- 中等：根因机制（穿透删除）——终态形态吻合但无删除过程的直接日志（USN 需管理员，未读）。
- 弱/间接：执行者归因（候选 1 vs 2）；主管 22:3x 观察与 20:45/21:0x remove 时刻（仅有派发提示词转述，无原始工件）。

## 4. 证据局限

- USN 日志（可给出逐文件删除记录）：需管理员，本腿按边界不提权。
- 主管会话两次 worktree remove 的准确时刻与命令：无工件，仅派发提示词转述。
- HOST junction 的精确指向（B）为其 PROGRESS「借用主检出」的合理解读，无命令行工件。

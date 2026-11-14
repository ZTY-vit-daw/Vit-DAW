# S8 编译验证与 §3 junction 拆除纪律验证（VERIFY）—— 2026-11-14 22:0x freeflash

## 编译验证（恢复后 godot-cpp 可编译态亲证）

- 载体：临时前端 worktree `D:/Godot/project/vit-daw-frontend-gcppverify1` @ origin/port/ui-os-1（128477b，今晚最新 extension cpp 状态，OS 卡真栈验证过的代码基）
- 借用方式：junction `wt/godot-cpp → D:/Vit_DAW/godot-cpp`（PowerShell `New-Item -ItemType Junction` 创建，与今晚各卡同模式）
- 命令与退出码（在 wt/extension 下）：
  1. `scons platform=windows target=template_debug arch=x86_64` → **exit 0**（log: scons_debug.log）
  2. `scons platform=windows target=template_release arch=x86_64` → **exit 0**（log: scons_release.log）
- 产物：extension/bin/ 下双变体 dll（template_debug 493056B / template_release 450048B）+ .lib/.exp；scons 日志含实际编译（gen/src/classes/*.cpp）与两段 Linking（godot-cpp 静态库 + extension dll）
- 产物落点实证（junction 穿透写入 = A 为真实构建缓存位）：A/bin 两静态库 LastWriteTime 22:03:49 / 22:04，gen 头重生成，1848 个新 .obj 在 src/、gen/src/；`git -C A status --short` 仍 0 行（全部被 gitignore 吸收，tracked 内容零改动）
- 验证后临时 worktree 处置：先摘链接（见下）→ 确认无 reparse point → `git worktree remove --force`（force 仅针对未跟踪构建产物）→ 注销核对 0 残留

## §3 junction 拆除纪律可执行性验证（卡面目标 3）

**Git Bash 环境下 `cmd /c rmdir` 的可用形式（实测）：**

| 调用形式 | 结果 |
|---|---|
| `cmd //c 'rmdir "D:\path\link"'`（内层双引号） | **失败**——"文件名、目录名或卷标语法不正确" |
| `cmd //c rmdir 'D:\path\link'`（外层单引号+反斜杠路径，无内层引号） | **成功 rc=0**，仅删链接本身 |
| 含空格路径样例 `cmd //c rmdir 'D:\Vit_DAW_wt_gcpp1\sectest_link dir'` | **成功 rc=0**，目标目录内容未伤（0 文件） |

- 拆除后核验：worktree 内无 godot-cpp 条目、`dir /AL` 无 reparse point；A 位置完好（extension_api.json/SConstruct/gdextension_interface.h 哈希与恢复时一致，tracked 0 改动）
- **对 §3 条文的补充建议（供主管采择）**：Git Bash 下内层双引号形式会失败，样例宜写为 `cmd //c rmdir '<反斜杠路径>'`（路径含空格也可用，勿加内层引号）；`mklink /J` 同理（`/J` 会被 MSYS 路径转换吞掉，建链用 PowerShell `New-Item -ItemType Junction` 更稳）

## 残留与边界

- A 现为"4.2.2 tracked 内容 + 22:0x 新鲜构建缓存（obj/bin/gen）"的可编译态，即被清空前的角色形态
- 前端主检出 B（D:/Godot/project/vit-daw-frontend/godot-cpp）仍为空——按卡面留候裁定，本腿未动
- 编译验证用 dll 未入库（临时 worktree 已随拆链拆除）；入库 dll 属后续 cpp 改动卡的工作域
- 真栈加载级验证（4.6.1 运行时加载新 dll）不在本卡域：dll 为 git 工件，今晚 HOST/OS 真栈已证 4.2 系 dll 可被 4.6.1 加载并全 PASS

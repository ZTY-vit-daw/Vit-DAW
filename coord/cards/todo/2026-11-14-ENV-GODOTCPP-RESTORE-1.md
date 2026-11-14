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

- 待回填

## 回执

- 待回填

## 验收

- 待回填

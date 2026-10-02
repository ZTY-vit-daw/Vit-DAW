# VITNOTE-V2-IMPL-D 工件留档（结论可回指原始工件）

- 日期：2026-10-02；工作仓：D:\Godot\project\vit-daw-frontend，分支 port/vitnote-v2-impl-d（自 port/vitnote-v2-impl-c@4e0b8625967c6927cc62151680b281b3262de2a3 切出，实现 commit=fd7e6af，已推 origin）
- Godot：D:\Godot\Godot_v4.6.1-stable_win64_console.exe（v4.6.1.stable.official.14d19694e，与 IMPL-A/B/C 同二进制）
- **运行栈声明（§9）**：执行侧全程未启动真实三件套（无内核/前端/agent 进程，headless 探针与 --import 不占端口栈）；手测栈将由用户按 AGENTS §5 途径自行拉起并独占。

## 实现侧自验（headless 面）

| 工件 | 命令 | 退出码 | 结论 |
|---|---|---|---|
| probe_panel_summary_run1.txt | `"...console.exe" --headless --path . -s tools/probe_vitnote_panel_summary.gd` | 1（16 项 1 红） | **红**：`empty label and kind falls to generic`——face_kind 键存在但值为空串时 `get("face_kind", "面")` 默认值不生效，label 落空串。实现边界缺陷（非探针搭建缺陷），首红留档 |
| probe_panel_summary_run2.txt | 同上（实现修复后） | 0 | 16/16 PASS |
| probe_panel_summary_run3_rerun.txt | 同上（确定性复跑） | 0 | 16/16 PASS |
| probe_face_resolve_regression.txt | 同形命令 -s tools/probe_vitnote_face_resolve.gd | 0 | 127/127 PASS（IMPL-B/C 套件零回归） |
| probe_circle_state_regression.txt | 同形命令 -s tools/probe_vitnote_circle_state.gd | 0 | 52/52 PASS（IMPL-A 套件零回归） |
| probe_input_regression.txt | 同形命令 -s tools/probe_vitnote_input.gd | 0 | 全套 PASS（INPUT-FIX 套件零回归） |
| import_zero_script_error.txt | `... --headless --path . --import` | 0 | SCRIPT/PARSE ERROR 计数=0 |

本卡探针断言面（16 项）：多面降序拼接（设计 §5.3 原文示例+§10.2 第 3 步三面形态）/单面 100%/机架带轨名 label/roundi 取整边界（.284→28%、.286→29%）/载荷原序信任（乱序输入按数组原样拼接——降序是 resolve_circle 载荷契约，呈现层不重排）/faces 空·缺键·非数组三形态皆空辖区/非字典元素跳过/label 两级兜底（face_kind→「面」）/share 缺省 0%/tooltip 同步。边界（如实申报）：真实渲染视觉（clip 截断/hover）/多 note 并存/跨面真实占比目检归手测第 3/8 步。

## 实现内容（commit fd7e6af，2 文件=卡面预计上限）

- `app/tracks/vitnote/vit_note_panel.gd`：`_rebuild_summary` 读 faces[]——「label 整数%」按载荷原序以「 · 」拼接，前缀 note_id；faces 空/缺键/非数组走「空辖区」文案（§5.3 合法态，退化路径不变）；v1 顶层 track_ids/clip_ids 读取移除（IMPL-B 起上移入 timeline 面 domain，lane 直连入口已随 IMPL-A 退役，无活生产者）；防御：非字典元素跳过+label→face_kind→「面」兜底+share 缺省 0%；tooltip 与摘要行同步（clip_text 截断时 hover 可见全貌）。
- `tools/probe_vitnote_panel_summary.gd`：新建 headless 探针（沿 face_resolve 先例的帧驱动模式；§10.1 辅助验收最小形态）。

## 手测工件

- `MANUAL_TEST_9STEPS.md`：九步编排清单（用户执行场；判据逐条=设计 §10.2 全表+本卡多面摘要呈现判据注入第 3/4/5 步）。
- `MANUAL_TEST_9STEPS_RESULT.md`：结果记录（手测后回填）。

## 三卡同场销（裁定 8，手测第 9 步后落簿）

- VITNOTE-IMPL-1 条件项：待手测（步骤 1/2/8 复核）
- VITNOTE-DOCK-MOUNT-1 条件项：待手测（全程 dock 面）
- V2 五卡（IMPL-1→DOCK-MOUNT→INPUT-FIX→A→B→C→D 链）：本卡=收官张，待手测 1–9 全过

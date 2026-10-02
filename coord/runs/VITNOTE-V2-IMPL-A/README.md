# VITNOTE-V2-IMPL-A 验收工件留档（INPUT-FIX-1 ruling 注记要求：结论可回指原始工件）

- 日期：2026-10-02；工作仓：D:\Godot\project\vit-daw-frontend，分支 port/vitnote-v2-impl-a（自 port/vitnote-input-fix-1@d97293c 切出，实现 commit=091cf80，已推 origin）
- Godot：D:\Godot\Godot_v4.6.1-stable_win64_console.exe（v4.6.1.stable.official.14d19694e）

## 命令与退出码（仓库根目录执行）

| 工件 | 命令 | 退出码 |
|---|---|---|
| probe_circle_state_run1.txt | `"D:\Godot\Godot_v4.6.1-stable_win64_console.exe" --headless --path . -s tools/probe_vitnote_circle_state.gd` | 0 |
| probe_circle_state_run2_rerun.txt | 同上（确定性复跑） | 0 |
| import_zero_script_error.txt | `... --headless --path . --import` | 0，SCRIPT/PARSE ERROR 计数=0 |

（原始输出为控制台重定向文本；仓库 .gitignore 排除 `*.log`，按 FIX-BROADBAND-SHARED-1/go_full_after.txt 先例以 .txt 落档，内容字节不变。）

## 探针结果（两轮一致）

- checks=52 failed=0，PROBE PASS（状态机套件 41 项 + lane 通知静态退役 1 项 + 两面挂载烟测各 5 项）
- 覆盖：§4.1 状态/转移表逐条（S1–S8/S18）、四类取消路径零残留（S5 Q+单击、S9 ESC、S10 松 Q、S11 RMB、S12 pending 态三式）、透传边界（S13 滚轮、S14 Q 字符、S15 重复 press）、宿主兜底（S16 cancel、S17 observe_position）、矩形规范化+viewport 钳制（S8d/S8e）
- 两面烟测：legacy+dock 的 VitNoteLayer 均持 VitCircleSelectLayer 子节点、present_marquee_hint 已退役、_on_circle_select_finished 在位、层脚本绑定正确
- legacy 树转储顺带证实挂载顺序不变量现状合规：VitNoteLayer 之后仅 GlobalFrequencyAxisController（无输入回调）等节点

## 过程记录（诚实申报）

- import_run0_first_with_parse_error_fixed_in_worktree.txt：首轮 import 抓到实现真实缺陷（observe_position 返回 Dictionary 直传 _apply_actions 缺 .get("actions")），当场修复后零错误——保留首轮日志作修复前证据。
- 探针开发中另有两处探针侧修正：S2c 状态污染（复用机器）与 S8e 测试数据（未拖出视口）；S8a 失败暴露实现缺陷（release 位置未回写 current_global，终值矩形漂移）→ 已修（release 事件位置回写）。
- 退出时 ObjectDB leaks 警告为探针环境产物（headless 场景 teardown），退出码 0 为门槛判据。

## 未覆盖边界（归用户手测 §10.2 第 1/2/6/7 步）

渲染视觉（暗幕/亮框/淡出）、真实输入链（Q 轮询、焦点让位、router/lane 先序）、胶囊双出口交互、Alt 克隆与普通框选共存——headless 探针不覆盖，见卡面自验清单。

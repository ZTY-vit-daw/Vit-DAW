# Project Prep Capability Layer v0

状态：命名补正稿。本文把原 A 阶段重新表述为能力族，而不是线性工作流。

## 核心结论

A 不再只被理解为“工程准备阶段的一串顺序步骤”。它是 `project_prep` 能力族：每个 A 编号都是可独立触发、可观察、可确认、可记录的能力单元。

阶段编号继续保留，因为它方便对照旧路线图和工程黑板；但编号不表示门禁。用户可以按需要调用任意能力，Agent 不应要求先完成 A1 才能使用 A4 或 A5。

## 命名约定

- 产品编号：继续使用 A1、A2、A3、A4、A5。
- 能力族前缀：`project_prep`.
- 能力 ID：使用点分命名，例如 `project_prep.import_intake.v0`。
- 能力 ID 属于 Agent workflow/capability layer，不等于后端 typed command。
- 后端 typed command 仍保持通用对象命令，例如 `project.state`、`clip.gain.set`、`clip.strip_silence.apply_batch`。

## A 能力族

| 产品编号 | Capability ID | 中文名 | 当前职责 |
| --- | --- | --- | --- |
| A1 | `project_prep.import_intake.v0` | 工程接收与多轨导入 | 接收素材、批量导入、建立轨道和 clip、刷新 project state、输出导入摘要。 |
| A2 | `project_prep.technical_integrity.v0` | TIM 技术完整性检查 | 读取格式、采样率、bit depth、声道、source/path/playback、静音、削波、异常 clip、DAD acoustic readiness；未知事实标记为 missing/limited。 |
| A3 | `project_prep.track_organization.v0` | TOM 智能轨道整理 | 基于 TIM/DAD/project facts 聚类并生成文件夹、轨道整理和路由建议；确认后执行。 |
| A4 | `project_prep.clip_edit_cleanup.v0` | Clip 编辑与片段清理 | 覆盖切割、fade/gain、范围选择、Strip Silence、片段清理建议与确认执行。 |
| A5 | `project_prep.section_marker_map.v0` | Marker / 段落地图 | 生成段落地图和 marker 建议；确认后写入工程。 |

## 跨阶段能力

`project.blackboard.status_report.v0` 不再编号为 A6。它是跨阶段当前工程状态汇报能力：

- 汇总 A-F 的已观察、已建议、待确认、已执行、阻塞、跳过、延期、需要复查状态。
- 在导入后整理 TIM/TOM/EPM/Marker/片段清理结果。
- 在用户询问阶段性状态时，用工程黑板格式回复。
- 不把 A-F 解释成线性门禁。

## 原 A4 的处理

旧规划中的 A4 “整理方案确认 + 初听与目标确认”拆分为两部分：

- TOM 整理方案确认归入 `project_prep.track_organization.v0` 的 confirmation/apply 子流程。
- 初听与目标确认不再作为必须线性节点；它进入 `project.blackboard.status_report.v0` 的“已知用户目标”字段，并可在任意阶段由用户补充。

## Capability 结构

每个 A 能力单元按同一结构理解：

```text
Capability =
  意图识别
  + 上下文要求
  + 观察/分析
  + 建议或待确认动作
  + typed command 执行映射
  + 结果摘要
  + 工程黑板记录
```

这层不是 skill，也不是工具层。skill 提供方法论；工具层读写 DAW 对象；capability layer 把一个真实工程任务组织成可验证、可确认、可执行、可记录的闭环。

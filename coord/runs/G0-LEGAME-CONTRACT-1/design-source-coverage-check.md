# 设计源覆盖性核验 — vit.legame_design.v0 契约草案 vs design.v1.yaml (revision 3)

- 卡：G0-LEGAME-CONTRACT-1；日期：2026-11-15
- 方法：机械核验（python -I + PyYAML），非人工目检——路径提取脚本对设计源 YAML 做全树遍历，产出的每条字段路径逐段对照 schema 草案（`legame-design-v0.schema-draft.yaml`）登记键；枚举域从设计源实例实际取值归纳后与契约闭集比对；引用完整性规则按契约 §5.2 编号在实例上执行。

## 1. 结果汇总

| 核验项 | 结果 |
|---|---|
| 顶层段数 | 17（schema_version, id, revision, title, brief, objectives, materials, workspace, capability_requests, checks, flow, guidance, evidence_recording, submission, completion, budget, pending_questions）——契约 §4.1 表 17 行全覆盖 |
| 全树字段路径 | 589 条实例路径 → 去列表索引后 119 条结构路径 → **schema 草案键覆盖 119/119，未覆盖 0** |
| 枚举闭集比对 | 14 个枚举面（objective.kind / material.kind / track.role / check.purpose / check.method / check.on_unavailable / check.target / flow.progression / node.lead / unknown_source / on_return / export / on_limit / locator.scope）设计源实际取值 ⊆ 契约闭集，**无溢出** |
| 引用完整性 | REF-OBJ / REF-CHK / REF-NODE-* / REF-COMP / REF-REGION / REF-MAT / ID-UNIQ / FLOW-CW / REG-METER / REG-ALIAS 在实例上执行：**违例 0** |
| region/alias 清单 | 6 regions（r-reference, r-rhythm-lab, r-q1, r-q2, r-theme, r-results）；8 个全局唯一 clip alias（rhythm-lab, q1-free, q1-flat, q1-smooth, q1-wave, q2-prototype, q2-variant, eight-bar-theme） |
| checks.target 特例 | 唯一非 region 目标=submission_snapshot（c-reports），契约 §4.8 已收编 |

## 2. 枚举域实测值（设计源 revision 3）

```
objectives[].kind          = learning | music_task
materials[].kind           = text | image | symbolic_reference | midi_or_audio | midi | material_group
workspace.tracks[].role    = reference | user_work | assist_generated | generated_result
checks[].purpose           = task_progress | learning_evidence
checks[].method            = user_judgment | rubric | deterministic
checks[].on_unavailable    = request_review | pause_and_report
checks[].target            = 6 个 region id + submission_snapshot
flow.progression           = ordered
flow.nodes[].lead          = human（全部 8 节点一致 → T-4 lead_default=human 可确定性推导）
evidence_recording.unknown_source = mark_unverified
submission.on_return       = preserve_history_and_continue
completion.export          = optional
budget.on_limit            = preserve_and_pause
locator.scope              = selected_reference_excerpt | four_clips | prototype_and_variant | generated_audio_versions
```

## 3. 边界与诚实声明

1. 本核验证明**契约草案覆盖现行设计源全字段且实例全绿**；不证明解析器/编译器存在（实施卡草案 `impl-cards-draft.md`，未实现）。
2. 枚举闭集以设计源实例实测值为下界收编；未来设计源新增枚举值 → 按契约"扩值须扩门"流程先修订契约。
3. 完整路径清单留档 `design-source-paths.txt`（589 行，本目录），供实施卡 W01-A 写固定 fixture 测试时对账。
4. 设计源自身语义问题（如 c-q1-structure criterion 写"4/4谱稿"而工程为 2/4 的表述张力、r-rhythm-lab"写法待导师确认"）属内容面待决，已在设计源 pending_questions 与裁定文档 §14 挂账，本契约不裁、schema 不加字段。

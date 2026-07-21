# Static Mix Capability Contract v0

状态：B 能力族契约稿。本文把 B 粗混 / Static Mix 正式建模为能力层，而不是线性工作流。

## 核心结论

B 是 `static_mix` capability family。B1-B5 是并列能力单元，不互相阻塞，也不表示必须按顺序完成。

用户可以直接请求任一能力，例如先做 B3 声像布局、只查看 B4 低频关系，或直接讨论 B5 核心元素定位。Agent 不能要求“必须先完成 B1 才能做 B2/B3/B4/B5”。

## 能力命名

| 产品编号 | Capability ID | 中文名 | 主要目标 |
| --- | --- | --- | --- |
| B1 | `static_mix.gain_staging.v0` | Gain Staging | 建立安全电平和 headroom。 |
| B2 | `static_mix.static_balance.v0` | 静态音量平衡 | 不依赖插件，先建立主次关系。 |
| B3 | `static_mix.pan_layout.v0` | 声像布局 | 中心元素、左右展开、宽度、mono 风险。 |
| B4 | `static_mix.low_end_relation.v0` | 低频关系 | kick、bass、low synth、低频堆积和遮蔽。 |
| B5 | `static_mix.focus_position.v0` | 核心元素定位 | lead vocal / lead instrument / snare / bass 的前后关系。 |

## 状态枚举

Capability 状态使用统一枚举：

- `not_started`：尚未记录为已执行或已观察。
- `observed`：已有只读观察结果。
- `suggested`：已有建议，但尚未形成待确认动作。
- `pending_confirmation`：已有待用户确认动作。
- `applied`：已确认并执行。
- `blocked`：存在阻塞，无法继续。
- `skipped`：用户明确跳过。
- `deferred`：延期处理。
- `needs_review`：需要复查或 AB review。

## 统一结果结构

每个 B capability 的结果应能整理为：

```json
{
  "capability_id": "static_mix.gain_staging.v0",
  "capability_name": "B1 Gain Staging",
  "status": "observed",
  "scope": {
    "kind": "project|selected_tracks|selected_clip|time_range|user_named_target",
    "ids": []
  },
  "evidence_refs": [
    "project.state",
    "mix.observe",
    "mix.read",
    "mix.derive"
  ],
  "observations": [],
  "candidate_actions": [],
  "pending_action_refs": [],
  "executor_result_refs": [],
  "risk": [],
  "limitation": [],
  "recommended_next_step": ""
}
```

## 工具映射

Capability layer 不新增后端命令。它编排现有通用 typed commands：

- 观察：`project.state`、`mix.observe`、`mix.read`、`mix.derive`
- B1 fader unity：`track.group.apply_control`，`mode:absolute`，`db:0`，必须待确认并回读验证。
- B1.2 source/clip 校准：读取同类可用电平证据（优先 LUFS/近似 LUFS，其次 RMS，最后 peak 替代），用项目内技术参考生成待确认 `clip.gain.set`；后续 trim/input gain 工具可替代 clip gain。不得使用 `track.volume`、`track.group.apply_control` 或 `mix.propose_tick`/`mix.apply_tick` 做 B1.2 源素材校准。
- B2 能力入口：先执行确定性的 Readiness Check。能力依赖当前工程状态，不依赖“是否曾执行 B1”的历史；B1 只是修复不健康技术基准的一种路径。
- B2 分析：从 TOM full assignment manifest、MOM full multitrack relation 和 `project.state` 构建全轨 `static_balance.model.v1`，再结合 Mix Style（`vit.mix_style.v1`，文件后缀 `.vms`）生成确定性候选方案。
- B2 CCB：装配 `static_mix.static_balance.context_pack.v1`，只披露 readiness、覆盖率、功能摘要、候选 ID 和有限示例。`analyzed_track_count` 与 `disclosed_track_count` 必须分离，披露预算不得限制分析或 pending 动作。
- B2 LLM 决策：LLM 只能选择已有 `candidate_plan_id`、解释、澄清或拒绝，不能生成轨道 ID、dB 值或动作列表；Action Compiler/Safety Validator 从完整 solver 结果解析待确认方案。
- B2 执行：确认后逐项调用 `mix.propose_tick` -> `mix.apply_tick`；不得直接调用 `track.volume`，不得修改 clip gain、插件、声像或自动化。
- B2 验证：完整计划执行后回读 `project.state` 核对每轨目标推子，并运行一次全工程 `mix.observe` 刷新 MOM 关系证据。
- B3 待确认/执行：继续使用单个或明确耦合的小步 `mix.propose_tick` -> `mix.apply_tick` 声像动作。
- 回滚：`mix.rollback_tick`
- 只读 clip 状态：`clip.gain.read`

## 执行边界

- B1-B3 可以在 v0 中形成待确认动作；B1 的异常推子复位属于工程状态校准，可以使用绝对 0 dB，不受 B2/B3 小步混音限制。
- B1.2 必须建立在 B1.1 fader unity 之后；如果仍有非 0 dB track fader，应先回到 B1.1，而不是用源素材校准掩盖 fader 状态。
- B1.2 执行后必须重新读取 `project.state`，并在工具可用时重新运行 `mix.observe` 验证/刷新证据；LUFS/RMS/active RMS 缺失时必须明确 limitations。
- B2 使用功能关系规则（foreground、rhythm anchor、low-end anchor、harmonic bed、support、effects），不内置“人声一定比鼓大”一类跨风格音量铁律。
- B2 的 VMS 只能在固定维度和边界内改变权重，不能绕过 Readiness、MOM 证据门、单动作 `+/-2 dB`、pending confirmation 和执行后回读验证。B2 不设由 CCB disclosure 派生的分析轨道数或动作数上限。
- B2 readiness 不把 L3 频谱、声像或 LUFS 设为静态音量平衡的普遍硬门；当全轨可比较 RMS/active RMS、角色关系、推子状态和 MOM action-preflight 证据充分时即可进入 solver。
- B4-B5 在 v0 中优先输出观察和建议；证据不足时不能伪造 EQ、压缩或空间处理结果。
- 所有会修改工程的动作必须进入 pending confirmation。
- 大工程不能每轨触发一次 LLM；能力层必须先完成全轨机械计算，再向 LLM 提供 project-level compact CCB disclosure。
- 工程黑板记录 B1-B5 独立状态，不把它们折叠成单个“B 粗混已完成”。

## 非目标

- 不实现 trim/input gain 后端命令；B1.2 v1 先使用 `clip.gain.set` 完成最小可用闭环。
- 不新增 GUI。
- 不新增 kernel typed command。
- 不做插件 EQ、压缩、空间、自动化 lane。
- 不把 B1-B5 写成线性 workflow gate。

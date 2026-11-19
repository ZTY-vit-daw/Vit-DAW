# TUTOR-SIM-1 实验工程说明

本目录是池序 435 / TUTOR-SIM-1 的服务端导师循环实验装置。目标是验证最小闭环：
打开实验工程 → 服务端创建导师会话 → LLM 按人设开场 → 用户按当前步骤输入 →
**服务端 trim 后精确匹配** → 错不推进、对推进 → 三步完成后停止。

本实验**不依赖**生产 design.v1 / 乐局卡 schema，也不改冻结生产设计源；服务端不执行
控件、谱面或音频操作，不做 UI 自动开场（留 TUTOR-SIM-2）。

## 1. 实验 schema

`tutor_sim.yaml`（schema_version = `vit.tutor_sim.v0`）：

```yaml
schema_version: vit.tutor_sim.v0
id: tutor-sim-3step
persona: |
  导师人设文本
steps:
  - id: step-1
    ask: "第 1 步任务"
    expect: "7"       # 服务端判定值，绝不发给 LLM
    retry: "失败时的重试引导素材"
    pass: "通过时的反馈素材"
```

字段约束（fail-closed，未知字段/多文档/重复 step id 都会被拒绝）：

| 字段 | 必填 | 说明 |
|---|---|---|
| schema_version | 是 | 只能是 `vit.tutor_sim.v0` |
| id | 是 | 实验 id |
| persona | 是 | 导师人设，进入 system prompt |
| steps[] | 是（≥1） | 每一步含 id/ask/expect/retry/pass，均非空 |
| expect | 是 | 用户输入 trim 后与 expect 完全相等才算通过；大小写敏感 |

## 2. 准备一个空白实验工程（不导入设计源）

1. 用既有真实产品入口新建并保存一个空白工程（例如 `D:/Vit-DAW-experiments/tutor-blank.vit`）。
   本实验不需要设计源导入、legame 编译、轨道或音频。
2. 在**工程文件所在目录**创建 sidecar：

   ```powershell
   New-Item -ItemType Directory -Force "<工程目录>/.vit_agent" | Out-Null
   Copy-Item "D:/Vit_DAW/coord/experiments/tutor-sim-1/tutor_sim.yaml" "<工程目录>/.vit_agent/tutor_sim.yaml" -Force
   ```

   sidecar 的精确位置是 `<工程目录>/.vit_agent/tutor_sim.yaml`（与 `.vit` 文件同级目录）。
3. 通过正常入口打开该工程，让 agent 的当前工程指向它；之后 smoke 脚本用
   `project_path` 显式传入工程路径（不要求工程已打开，但本卡的真实栈腿按“已打开”执行）。

> 本实验只读 sidecar；不会写入工程、设计源、sealed fixture 或 recents。

## 3. 服务端 API 契约

### POST /agent/tutor/start

请求：`{"conversation_id": "...", "project_path": "..."}`（两字段均可选；空 project_path
时回落到 agent 当前工程；空 conversation_id 时服务端生成）。

- 工程旁无 `.vit_agent/tutor_sim.yaml`：HTTP 200 `{"status":"unavailable","reason":"sidecar_missing"}`
  （无工程路径时 `reason":"no_project"`）。不创建会话。
- sidecar 非法：HTTP 422 `{"status":"error",...}`，不创建会话。
- LLM 配置不完整：HTTP 503，不创建会话。
- 成功：HTTP 200

```json
{
  "status": "ok",
  "conversation_id": "tutor_...",
  "experiment_id": "tutor-sim-3step",
  "schema_version": "vit.tutor_sim.v0",
  "step_index": 1,
  "step_total": 3,
  "step_id": "step-1",
  "reply": "<flash LLM 按 persona+第 1 步 ask 生成的开场>"
}
```

### GET /agent/tutor/status?conversation_id=...

```json
{
  "status": "ok",
  "conversation_id": "tutor_...",
  "experiment_id": "tutor-sim-3step",
  "step_index": 2,
  "step_total": 3,
  "step_id": "step-2",
  "session_status": "active",
  "completed": false,
  "attempts": 1,
  "failures": 0
}
```

未知会话：HTTP 404 `{"status":"error","error":"unknown_tutor_session"}`。

### POST /agent/chat（命中导师会话时）

若 `conversation_id` 命中内存导师会话，请求在进入普通 agent loop（goal / pending /
语义入口 / harness 落盘）**之前**由导师循环接管：

- 取用户文本 → trim 后与当前 step.expect 精确比较；
- 失败：不推进，LLM 按当前 step 的 retry 引导重试；
- 通过：推进并把下一 step 的 ask 交给 LLM；第 3 步通过后 completed；
- 完成后继续输入：不再调用 LLM，返回 completed 状态；
- 返回 `workflow="tutor_sim"`，`workflow_data` 至少含
  `{step_index, step_total, step_id, passed, completed}`，另有
  `schema_version/experiment_id/attempts/failures`；
- 服务端判定不来自 LLM；LLM 失败时不伪造回复，返回错误分类
  （`failure_class=transient|permanent|empty_reply|config`）并保留服务端已判定的步骤状态。

`stop_reason` 取值（稳定契约）：

| stop_reason | 含义 |
|---|---|
| `tutor_sim_retry` | 本次输入未通过，停在原步 |
| `tutor_sim_step_advanced` | 本次输入通过，已推进到下一步 |
| `tutor_sim_completed` | 第 3 步通过，实验完成 |
| `tutor_sim_already_completed` | 已完成后再次输入，未推进、未调用 LLM |
| `tutor_sim_llm_transient` | LLM/网络临时失败（已判定步骤状态保留） |
| `tutor_sim_llm_error` | LLM 永久性失败（已判定步骤状态保留） |
| `tutor_sim_llm_empty_reply` | LLM 返回空文本，不伪造回复 |
| `tutor_sim_llm_unavailable` | LLM 配置不可用 |

start 失败时不创建会话：sidecar 缺失返回 `unavailable`；sidecar 非法 422；配置不完整 503；
LLM 调用失败 502（带 `failure_class`）。

未命中导师会话的 `conversation_id`：完全走原有普通 chat 路径，行为不变。

## 4. Headless smoke（lead 执行）

```powershell
# 默认仅打印 runbook，不接触栈：exit 3
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_tutor_sim_smoke.ps1

# 无栈 dry-run：只解析参数/工程与 sidecar，不发 HTTP：exit 0
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_tutor_sim_smoke.ps1 -DryRun -ProjectPath "<工程.vit>"

# 真实栈腿（需主管登记栈位并授权；脚本不启停栈）
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_tutor_sim_smoke.ps1 `
  -StackAuthorized -RuntimeOwner "<登记 owner>" -ProjectPath "<工程.vit>"
```

真实栈腿断言：start 返回第 1 步与 LLM 开场；错误输入不推进；7 → 第 2 步；
42 → 第 3 步；123 → completed；status 与 chat 回执一致；回复不含
「开始乐局 / 写好了 / 乐局状态 / 乐局事件」命令词；回执 workflow 恒为 tutor_sim。
工件写入新的 `VitApp/Workspace/Artifacts/smoke/tutor_sim_<stamp>/`。

## 5. 边界与已知限制

- 会话只存内存：agent 进程重启后全部丢失，需要重新 start。
- 每步无尝试次数上限；错误输入可无限重试，第 3 步通过后即 completed。
- 本轮无 UI 控件、无自动开场、无谱面/音频/设计源依赖；不写入工程。
- 真实 LLM 是概率性的：smoke 只断言服务端状态机与契约字段，不对开场措辞做语义断言。

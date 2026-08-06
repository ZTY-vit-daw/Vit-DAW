# VSP Phase 1 Schema and Channels

日期：2026-07-02

状态：Phase 1 冻结草案。本文定义 VSP v1 的 message envelope、session/capability、Command/State/Realtime/Asset/Event 通道骨架，以及 legacy IPC adapter 设计。它不要求立刻替换旧 UDP/ZMQ 链路，但后续实现必须以这里的语义为准。

配套机器可读草案：

- `schema/vsp_v1_envelope.schema.json`

## 1. Phase 1 完成标准映射

| Phase 1 要求 | 本文落点 |
| --- | --- |
| VSP message envelope | 第 2 节 |
| `request_id`、`client_id`、`session_id`、`role`、`capability`、`revision`、`transaction_id` | 第 2、3、4、6 节 |
| Command / State / Realtime / Asset / Event / Session 通道定义 | 第 6 节 |
| 错误码、回执、feature flags | 第 4、5 节 |
| 旧 IPC command adapter 设计 | 第 7 节 |
| schema 能表达现有关键动作 | 第 6.2、7.3 节 |
| schema 不暴露 Tracktion Engine 私有结构 | 第 8 节 |
| 最小 SDK 类型定义或代码生成计划 | 第 9 节 |

## 2. Message Envelope

所有 VSP 消息都必须有统一 envelope。payload 可以按通道扩展，但 envelope 字段名、含义、idempotency 和 tracing 规则必须保持一致。

### 2.1 Envelope 字段

| 字段 | 必填 | 类型 | 说明 |
| --- | --- | --- | --- |
| `vsp_version` | 是 | string | VSP 主版本。本草案固定为 `1.0`。 |
| `schema` | 是 | string | 消息 schema id，例如 `vsp.command.request.v1`。 |
| `message_id` | 是 | string | 每条消息唯一 ID，建议 `msg_` 前缀。 |
| `session_id` | 是 | string | hello 成功后分配；hello request 可用 `session_pending`。 |
| `client_id` | 是 | string | 客户端稳定 ID，例如 `gui.main`、`agent.main`。 |
| `role` | 是 | string | `kernel`、`gui`、`agent`、`controller`、`analyzer`、`gateway`、`extension`、`legacy_adapter`、`test`。 |
| `channel` | 是 | string | `session`、`command`、`state`、`realtime`、`asset`、`event`、`log`。 |
| `type` | 是 | string | 通道内消息类型，例如 `command.request`、`state.delta`。 |
| `created_at` | 是 | string | UTC ISO-8601 时间。 |
| `request_id` | 条件 | string | request/response/error 必须携带；event/realtime 可省略。 |
| `correlation_id` | 条件 | string | response/error 指向原 `message_id`。 |
| `transaction_id` | 条件 | string | 写命令或批量命令必须携带；无事务读命令可省略。 |
| `trace_id` | 建议 | string | 跨进程 trace。未提供时 Kernel 可生成。 |
| `project_epoch` | 条件 | string | 与工程状态相关的消息应携带。重开工程/重启内核后变化。 |
| `base_revision` | 条件 | integer | delta、patch、conditional write 使用。 |
| `revision` | 条件 | integer | state snapshot/delta 或写命令成功回执使用。 |
| `capabilities` | 条件 | array[string] | hello/capability 消息使用。 |
| `feature_flags` | 条件 | object | hello/capability 消息使用，值为 boolean。 |
| `payload` | 是 | object | 通道 payload。无内容时为 `{}`。 |
| `ack` | 条件 | object | accepted/rejected/progress/completed 回执元信息。 |
| `error` | 条件 | object | 失败时使用统一 error object。 |

### 2.2 ID 与幂等规则

- `message_id`：每条传输消息唯一，只用于 tracing 和 correlation。
- `request_id`：客户端生成，标识一次逻辑请求。Kernel 对同一 `session_id + request_id` 的危险写命令必须幂等，不能重复执行。
- `transaction_id`：Kernel 或客户端生成均可。客户端提供时表示希望多命令归并到同一事务；Kernel 可拒绝不支持的事务形态。
- `revision`：Kernel 状态版本，单调递增。只描述 Project Model 层状态，不描述 realtime frame。
- `project_epoch`：工程重新打开、内核重启、状态基线失效后更新。客户端发现 epoch 改变必须丢弃旧 revision。

### 2.3 基础示例

```json
{
  "vsp_version": "1.0",
  "schema": "vsp.command.request.v1",
  "message_id": "msg_01JZ000001",
  "session_id": "sess_01JZ000000",
  "client_id": "gui.main",
  "role": "gui",
  "channel": "command",
  "type": "command.request",
  "created_at": "2026-07-02T04:30:00Z",
  "request_id": "req_01JZ000002",
  "transaction_id": "tx_01JZ000003",
  "trace_id": "trace_01JZ000004",
  "payload": {
    "command": "clip.move",
    "args": {
      "clip_id": "clip_9f4a",
      "track_id": "track_31b8",
      "start_seconds": 12.5
    }
  }
}
```

## 3. Session、Capability 与 Feature Flags

Session 是 VSP 的第一条可靠通道。客户端必须先 hello，再发命令、订阅状态或订阅 realtime。

### 3.1 `session.hello`

```json
{
  "vsp_version": "1.0",
  "schema": "vsp.session.hello.v1",
  "message_id": "msg_hello_1",
  "session_id": "session_pending",
  "client_id": "agent.main",
  "role": "agent",
  "channel": "session",
  "type": "session.hello",
  "created_at": "2026-07-02T04:30:00Z",
  "payload": {
    "client_name": "Ask Vit Agent",
    "client_version": "0.1.0",
    "protocol_min": "1.0",
    "protocol_max": "1.0",
    "wants": [
      "project.read",
      "project.write",
      "plugin.read",
      "plugin.control",
      "asset.read",
      "realtime.subscribe"
    ],
    "transport_bindings": ["tcp.local", "legacy.zmq_udp"],
    "metadata": {}
  }
}
```

### 3.2 `session.hello_ack`

```json
{
  "vsp_version": "1.0",
  "schema": "vsp.session.hello_ack.v1",
  "message_id": "msg_hello_ack_1",
  "session_id": "sess_01JZ000000",
  "client_id": "kernel.main",
  "role": "kernel",
  "channel": "session",
  "type": "session.hello_ack",
  "created_at": "2026-07-02T04:30:00Z",
  "correlation_id": "msg_hello_1",
  "capabilities": [
    "project.read",
    "project.write",
    "transport.control",
    "plugin.read",
    "plugin.control",
    "asset.read",
    "realtime.subscribe",
    "legacy.command"
  ],
  "feature_flags": {
    "state.delta": true,
    "state.snapshot.scoped": true,
    "realtime.visible_tracks": true,
    "asset.reference": true,
    "legacy.ipc_adapter": true
  },
  "payload": {
    "accepted_protocol": "1.0",
    "project_epoch": "epoch_01JZ000000",
    "server_name": "Vit Kernel",
    "server_version": "0.1.0"
  }
}
```

### 3.3 Capability 命名

Capability 使用点分命名：

- `project.read`
- `project.write`
- `transport.control`
- `track.edit`
- `clip.edit`
- `plugin.read`
- `plugin.control`
- `plugin.scan`
- `asset.read`
- `asset.write`
- `realtime.subscribe`
- `state.subscribe`
- `render.offline`
- `filesystem.import`
- `filesystem.export`
- `legacy.command`

### 3.4 Feature Flag 命名

Feature flag 描述服务端实际启用的细粒度能力：

- `command.batch`
- `command.idempotency`
- `state.snapshot.scoped`
- `state.delta`
- `state.resync`
- `realtime.visible_tracks`
- `realtime.latest_only`
- `asset.reference`
- `asset.range_read`
- `event.job_progress`
- `legacy.ipc_adapter`

Capability 是权限/能力授予，feature flag 是具体实现开关。客户端必须同时检查二者。

## 4. 回执与错误模型

### 4.1 回执阶段

| `ack.stage` | 含义 | 使用场景 |
| --- | --- | --- |
| `accepted` | Kernel 已接收并排队 | 长任务、异步 import/render |
| `rejected` | 请求未执行 | validation、permission、capability 失败 |
| `in_progress` | 任务进度 | event progress 或 command ack |
| `completed` | 请求已完成 | command response、event terminal |
| `no_op` | 请求有效但没有状态变化 | 已在目标状态、重复幂等请求 |

### 4.2 Error Object

```json
{
  "code": "validation_error",
  "message": "clip_id is required",
  "retryable": false,
  "details": {
    "field": "clip_id"
  }
}
```

### 4.3 错误码

| code | retryable | 说明 |
| --- | --- | --- |
| `validation_error` | false | 参数不合法 |
| `not_found` | false | 对象不存在 |
| `conflict` | false | revision、epoch、事务冲突 |
| `busy` | true | Kernel 忙，例如 render/import 独占 |
| `permission_denied` | false | capability 不足 |
| `capability_not_supported` | false | 服务端不支持 |
| `timeout` | true | 请求超时 |
| `transport_error` | true | 传输层错误 |
| `asset_unavailable` | true | 资产尚未准备好或缓存失效 |
| `partial_failure` | false | batch 部分失败 |
| `internal_error` | true | 未分类内部错误 |
| `legacy_adapter_error` | false | 旧 IPC adapter 无法映射或旧命令失败 |

## 5. 通用对象地址

VSP 对外使用稳定对象地址，不使用 UI index 或 Tracktion Engine 私有结构。

| 对象 | ID 示例 | 说明 |
| --- | --- | --- |
| Project | `project_current` | 当前工程，也可用持久工程 ID |
| Track | `track_31b8` | Kernel 稳定轨道 ID |
| Clip | `clip_9f4a` | Kernel 稳定 clip ID |
| Plugin | `plugin_7a1c` | 插件实例 ID，不是链路 index |
| Macro | `macro_gain_balance` | 宏控 ID |
| Asset | `asset_peak_01JZ` | 资产 ID |
| Job | `job_import_01JZ` | import/render/bake job ID |

## 6. 通道骨架

### 6.1 Session Channel

用途：hello、capability、heartbeat、permission refresh。

可靠性：

- 必须可靠。
- 必须与 Command 通道同源或同等可靠。
- heartbeat 丢失超过实现阈值后，服务端可以暂停该 session 的订阅并要求重新 hello。

消息类型：

- `session.hello`
- `session.hello_ack`
- `session.heartbeat`
- `session.capability_update`
- `session.close`

### 6.2 Command Channel

用途：所有可验证工程动作。Command 不直接承载实时帧或大资产。

消息类型：

- `command.request`
- `command.ack`
- `command.response`
- `command.error`
- `command.cancel`

规则：

- 所有 request 必须有 `request_id`。
- 写命令必须有 `transaction_id`，或由 Kernel 在 `command.ack` 中补发。
- 写命令成功应返回 `revision` 或明确 `ack.stage = no_op` / `payload.stateless = true`。
- 读命令可不产生 revision，但必须有可验证 response。
- batch 命令必须保持子命令顺序，并返回每个子命令的 result。

关键命令命名：

| VSP command | 旧命令/现状能力 | 说明 |
| --- | --- | --- |
| `transport.play` | `play` | 播放 |
| `transport.stop` | `stop` | 停止 |
| `project.snapshot.get` | `get_project_state` | 兼容全量快照 |
| `project.import_audio_files` | `project.import_audio_files`、`import_audio` | 导入音频 |
| `track.create` | `add_track` | 新建轨道 |
| `track.delete` | `delete_track` | 删除轨道 |
| `clip.move` | `move_clip` | 移动 clip |
| `project.undo` | `undo` | 撤销 |
| `project.redo` | `redo` | 重做 |
| `plugin.parameters.get` | `get_plugin_parameters` | 插件参数读取 |
| `plugin.apply_eq_edits` | `plugin_grabber_apply_eq_edits` | 确定性 EQ 编辑事务 |
| `plugin.set_params_batch` | 新 VSP 优先 | 多参数批量写入 |

`transport.play` 示例：

```json
{
  "vsp_version": "1.0",
  "schema": "vsp.command.request.v1",
  "message_id": "msg_play_1",
  "session_id": "sess_gui",
  "client_id": "gui.main",
  "role": "gui",
  "channel": "command",
  "type": "command.request",
  "created_at": "2026-07-02T04:30:00Z",
  "request_id": "req_play_1",
  "payload": {
    "command": "transport.play",
    "args": {}
  }
}
```

`plugin.set_params_batch` 示例：

```json
{
  "vsp_version": "1.0",
  "schema": "vsp.command.request.v1",
  "message_id": "msg_plugin_batch_1",
  "session_id": "sess_agent",
  "client_id": "agent.main",
  "role": "agent",
  "channel": "command",
  "type": "command.request",
  "created_at": "2026-07-02T04:30:00Z",
  "request_id": "req_plugin_batch_1",
  "transaction_id": "tx_plugin_batch_1",
  "payload": {
    "command": "plugin.set_params_batch",
    "args": {
      "plugin_id": "plugin_7a1c",
      "parameters": [
        {"parameter_id": "gain", "normalized_value": 0.62},
        {"parameter_id": "mix", "normalized_value": 0.48}
      ],
      "readback": true
    }
  }
}
```

### 6.3 State Channel

用途：project snapshot、scoped snapshot、delta、selection、resync。

消息类型：

- `state.snapshot_request`
- `state.snapshot`
- `state.subscribe`
- `state.delta`
- `state.resync_request`
- `state.resync_required`

规则：

- snapshot 必须携带 `project_epoch` 与 `revision`。
- delta 必须携带 `project_epoch`、`base_revision`、`revision`。
- 客户端发现 `base_revision != local_revision` 时必须发 `state.resync_request`。
- State 可以按 scope 分片订阅，避免 GUI 每次重建完整工程。

建议 scope：

- `project.core`
- `project.timeline`
- `project.tracks`
- `project.track:<track_id>`
- `project.clip:<clip_id>`
- `project.rack:<track_id>`
- `project.plugins`
- `project.selection`
- `project.health`

`state.delta` 示例：

```json
{
  "vsp_version": "1.0",
  "schema": "vsp.state.delta.v1",
  "message_id": "msg_delta_101",
  "session_id": "sess_gui",
  "client_id": "kernel.main",
  "role": "kernel",
  "channel": "state",
  "type": "state.delta",
  "created_at": "2026-07-02T04:30:01Z",
  "project_epoch": "epoch_01JZ000000",
  "base_revision": 100,
  "revision": 101,
  "transaction_id": "tx_move_1",
  "payload": {
    "scope": "project.timeline",
    "ops": [
      {
        "op": "replace",
        "path": "/tracks/track_31b8/clips/clip_9f4a/start_seconds",
        "value": 12.5
      }
    ],
    "changed": ["clip_9f4a"]
  }
}
```

### 6.4 Realtime Channel

用途：播放指针、电平、可见轨实时频谱、录音状态等 latest-only 数据。

消息类型：

- `realtime.subscribe`
- `realtime.unsubscribe`
- `realtime.frame`
- `realtime.stream_status`

规则：

- 默认 latest-only，旧帧可丢。
- 必须有 `stream_id` 与单调 `frame_index` 或 `timestamp_samples`。
- 不携带大资产内容。
- GUI 必须能声明 visible track/window/rate limit。
- Realtime 不等待 State/Asset/Event。

stream 建议：

- `transport.playhead`
- `meters.visible_tracks`
- `spectrum.visible_tracks`
- `recording.status`

`realtime.subscribe` 示例：

```json
{
  "vsp_version": "1.0",
  "schema": "vsp.realtime.subscribe.v1",
  "message_id": "msg_rt_sub_1",
  "session_id": "sess_gui",
  "client_id": "gui.main",
  "role": "gui",
  "channel": "realtime",
  "type": "realtime.subscribe",
  "created_at": "2026-07-02T04:30:00Z",
  "request_id": "req_rt_sub_1",
  "payload": {
    "streams": [
      {
        "stream": "meters.visible_tracks",
        "track_ids": ["track_31b8", "track_91aa"],
        "max_hz": 20,
        "mode": "latest_only"
      },
      {
        "stream": "transport.playhead",
        "max_hz": 30,
        "mode": "latest_only"
      }
    ]
  }
}
```

### 6.5 Asset Channel

用途：waveform peak、spectral tile、render output、analysis package、import staging 等资产引用与读取。

消息类型：

- `asset.request`
- `asset.reference`
- `asset.manifest_request`
- `asset.manifest`
- `asset.changed`
- `asset.invalidated`
- `asset.error`

规则：

- VSP 核心只传资产引用、hash、revision、range、format、lifetime。
- 大数据通过 transport binding 获取，例如 cache file、mmap、shared memory、HTTP range。
- 资产 URI 必须平台中立，优先使用 `vit-cache://`、`vit-project://`、`vit-render://`。
- Windows/macOS 句柄或路径只能出现在 binding-specific metadata 中，不能作为唯一资产身份。

`asset.reference` 示例：

```json
{
  "vsp_version": "1.0",
  "schema": "vsp.asset.reference.v1",
  "message_id": "msg_asset_peak_1",
  "session_id": "sess_gui",
  "client_id": "kernel.main",
  "role": "kernel",
  "channel": "asset",
  "type": "asset.reference",
  "created_at": "2026-07-02T04:30:02Z",
  "project_epoch": "epoch_01JZ000000",
  "revision": 101,
  "payload": {
    "asset": {
      "asset_id": "asset_peak_01JZ",
      "kind": "waveform_peak",
      "owner": "clip_9f4a",
      "uri": "vit-cache://project_current/clips/clip_9f4a/peaks/v1.bin",
      "revision": 4,
      "byte_range": [0, 4096],
      "format": "f32_peak_minmax",
      "sample_rate": 48000,
      "channels": 2,
      "duration_seconds": 183.2,
      "hash": "sha256:..."
    },
    "bindings": [
      {
        "binding": "cache_file",
        "platform": "any",
        "ttl_ms": 60000
      }
    ]
  }
}
```

`asset.manifest_request` is the batched visible-priority form for waveform and
spectrum references. It must return references only, never large waveform or
spectrum arrays. Clients should use it for visible track/time-window warmup and
fall back to `asset.request` for a single missing clip.

```json
{
  "vsp_version": "1.0",
  "schema": "vsp.asset.manifest_request.v1",
  "message_id": "msg_asset_manifest_req_1",
  "session_id": "sess_gui",
  "client_id": "gui.main",
  "role": "gui",
  "channel": "asset",
  "type": "asset.manifest_request",
  "created_at": "2026-07-03T10:00:00Z",
  "payload": {
    "kind": "waveform_peak",
    "track_ids": ["1007", "1008"],
    "priority": "visible_manifest",
    "max_assets": 128
  }
}
```

```json
{
  "vsp_version": "1.0",
  "schema": "vsp.asset.manifest.v1",
  "message_id": "msg_asset_manifest_1",
  "session_id": "sess_gui",
  "client_id": "kernel.main",
  "role": "kernel",
  "channel": "asset",
  "type": "asset.manifest",
  "created_at": "2026-07-03T10:00:00Z",
  "payload": {
    "status": "ok",
    "manifest_id": "asset_manifest_01JZ",
    "kind": "waveform_peak",
    "references": [
      {
        "clip_id": "clip_9f4a",
        "kernel_track_id": "1007",
        "kind": "waveform_peak",
        "asset": {
          "asset_id": "asset_peak_01JZ",
          "kind": "waveform_peak",
          "owner": "clip_9f4a",
          "track_id": "1007",
          "uri": "vit-cache://project_current/clips/clip_9f4a/waveform/peaks/v1",
          "format": "f32_peak_minmax",
          "platform_neutral": true
        },
        "bindings": [
          {"binding": "cache_file", "platform": "any", "availability": "reference_only"}
        ]
      }
    ],
    "inline_payload": false,
    "no_big_json_payload": true
  }
}
```

### 6.6 Event Channel

用途：import/render/bake/plugin scan/recording/job progress 等事件。

消息类型：

- `event.subscribe`
- `event.notification`
- `event.progress`
- `event.completed`
- `event.error`

规则：

- event 必须携带 `event_id`。
- job 型 event 必须携带 `job_id` 和 job 内 `sequence`。
- event 可限频，但 terminal event 不可丢。
- event 不代替 state delta；完成后如改变工程状态，仍应有 state revision 或 resync hint。

`event.progress` 示例：

```json
{
  "vsp_version": "1.0",
  "schema": "vsp.event.progress.v1",
  "message_id": "msg_import_progress_12",
  "session_id": "sess_gui",
  "client_id": "kernel.main",
  "role": "kernel",
  "channel": "event",
  "type": "event.progress",
  "created_at": "2026-07-02T04:30:03Z",
  "payload": {
    "event_id": "evt_import_12",
    "job_id": "job_import_01JZ",
    "sequence": 12,
    "topic": "import.audio",
    "progress": 0.42,
    "message": "Decoding audio files",
    "terminal": false
  }
}
```

## 7. Legacy IPC Adapter 设计

Legacy adapter 的任务是让旧 Godot/Agent 调用继续工作，同时把旧命令纳入 VSP tracing、session、capability 和错误模型。

### 7.1 Adapter 位置

Phase 2 reference implementation 可采用两种兼容路径：

1. Kernel 内 VSP Server 包含 `legacy.command` adapter，直接调用现有 `CommandDispatcher`。
2. Agent/bridge 侧先引入 VSP envelope wrapper，再转旧 ZMQ/UDP 命令。

推荐顺序：

- Phase 2 优先在 Kernel 侧建 adapter skeleton，保证 VSP command 到旧 handler 的路径最短。
- Bridge 侧保留旧 UDP/ZMQ，作为未迁移 GUI 的兼容入口。
- Godot/Agent 新客户端只面对 VSP SDK，不直接拼旧 JSON。

### 7.2 包装规则

旧 request：

```json
{"cmd": "move_clip", "clip_id": "clip_9f4a", "track_id": "track_31b8", "start_seconds": 12.5}
```

VSP wrapper：

```json
{
  "vsp_version": "1.0",
  "schema": "vsp.command.request.v1",
  "message_id": "msg_legacy_move_1",
  "session_id": "sess_gui",
  "client_id": "gui.main",
  "role": "gui",
  "channel": "command",
  "type": "command.request",
  "created_at": "2026-07-02T04:30:00Z",
  "request_id": "req_legacy_move_1",
  "transaction_id": "tx_legacy_move_1",
  "payload": {
    "command": "legacy.command",
    "legacy": {
      "cmd": "move_clip",
      "args": {
        "clip_id": "clip_9f4a",
        "track_id": "track_31b8",
        "start_seconds": 12.5
      }
    }
  }
}
```

Adapter response rules：

- 旧 `{"status":"ok"}` 归一化为 `command.response`。
- 旧 `{"status":"error","message":"..."}` 归一化为 `command.error`。
- 旧 handler 返回 `revision` 时透出到 envelope。
- 旧 handler 没有 revision 的写命令，adapter 必须标记 `payload.revision_status = "legacy_unknown"`，并可附 `resync_hint`。
- 旧 `file_reply` 在 VSP 中只能作为 transport binding 细节，不能暴露为核心 payload 语义。

### 7.3 旧命令映射表

| 旧 cmd | VSP canonical command | Adapter 策略 | 状态结果 |
| --- | --- | --- | --- |
| `ping` | `session.heartbeat` 或 `kernel.ping` | 可直接兼容 | 无状态 |
| `play` | `transport.play` | 转 command | realtime transport 更新 |
| `stop` | `transport.stop` | 转 command | realtime transport 更新 |
| `get_project_state` | `project.snapshot.get` | 返回 legacy full snapshot，同时标记 `scope=legacy.full_project` | state snapshot |
| `list_tracks` | `project.tracks.list` | 读命令 | 无 revision 或当前 revision |
| `import_audio` | `project.import_audio_files` | 转 job command | event progress + state delta |
| `project.import_audio_files` | `project.import_audio_files` | canonical 优先 | event progress + state delta |
| `add_track` | `track.create` | 写命令 | state revision |
| `delete_track` | `track.delete` | 写命令 | state revision |
| `move_clip` | `clip.move` | 写命令 | state revision |
| `undo` | `project.undo` | 写命令 | state revision 或 resync hint |
| `redo` | `project.redo` | 写命令 | state revision 或 resync hint |
| `get_midi_clip_notes` | `clip.midi_notes.get` | 读命令 | scoped state payload |
| `get_plugin_parameters` | `plugin.parameters.get` | 读命令 | plugin payload |
| `plugin_grabber_apply_eq_edits` | `plugin.apply_eq_edits` | Agent 能力要求 `plugin.control` | transaction readback + rollback evidence |
| render commands | `render.start` / `render.cancel` | job command | event progress + asset reference |

### 7.4 Adapter 禁止事项

- 不把 TE ValueTree、Tracktion plugin index、Windows path、SHM handle 作为 canonical 字段。
- 不要求旧 Godot 一次性改完所有调用点。
- 不让 legacy full snapshot 成为新 State 通道的唯一实现。
- 不把 Agent 插件抓手智能搬进 Kernel；adapter 只做协议转换和执行路由。

## 8. 非 TE 泄漏边界

VSP canonical payload 只能表达 Vit 应用对象：

- track、clip、plugin、macro、asset、job、project、selection、transport。

禁止表达：

- Tracktion ValueTree path。
- Tracktion internal plugin list index。
- JUCE object pointer、engine object address。
- Windows-only path/handle as identity。
- Godot scene path as Kernel object identity。

内部实现可以继续使用 TE/JUCE/Godot/Windows 细节，但必须被 adapter 或 binding 层隔离。

## 9. 最小 SDK 类型与代码生成计划

### 9.1 Source of Truth

Phase 1 的 schema source 放在：

- `docs/vsp/schema/vsp_v1_envelope.schema.json`

后续可拆出：

- `vsp_v1_command.schema.json`
- `vsp_v1_state.schema.json`
- `vsp_v1_realtime.schema.json`
- `vsp_v1_asset.schema.json`
- `vsp_v1_event.schema.json`

### 9.2 最小类型

第一批 SDK 类型应覆盖：

- `VspEnvelope`
- `VspError`
- `VspAck`
- `VspCapabilitySet`
- `VspCommandRequest`
- `VspCommandResponse`
- `VspStateSnapshot`
- `VspStateDelta`
- `VspRealtimeSubscription`
- `VspRealtimeFrame`
- `VspAssetReference`
- `VspEvent`
- `VspLegacyCommand`

### 9.3 生成目标

| 目标 | 用途 |
| --- | --- |
| C++ | Kernel VSP Server、legacy adapter、conformance fixtures |
| Go | Agent client、bridge adapter、harness |
| GDScript | Godot client wrapper、typed validation helper |
| TypeScript | Web UI、debug tools、schema fixture generation |

### 9.4 生成策略

- JSON Schema 是跨语言校验基线。
- C++/Go/TypeScript 优先生成强类型结构。
- GDScript 先生成常量、field validator 和 fixture，不强求完整静态类型。
- 所有生成结果必须保留未知字段透传能力，方便 minor version 扩展。
- Conformance tests 以 schema fixture + 旧命令映射表为输入。

## 10. Phase 1 冻结结论

Phase 1 的设计边界是：先冻结 VSP 应用语义，不急着替换底层传输。Command/Session 走可靠通道，State 走 snapshot + delta，Realtime 走 latest-only 订阅，Asset 只传引用，Event 负责 job/progress，旧 IPC 通过 legacy adapter 进入新 envelope。

这套定义已经覆盖现有 `play`、`stop`、`get_project_state`、`import_audio`、`project.import_audio_files`、`add_track`、`move_clip`、`plugin_grabber_apply_eq_edits` 等关键动作，并为后续 Kernel reference implementation、Godot/Agent SDK 和 conformance test 留出明确落点。

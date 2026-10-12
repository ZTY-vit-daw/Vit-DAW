# 设计件二：异步作业模型（生命周期 / 状态参数枚举 / 取消 / 失败分类 / 缓冲播放）

- 归属：GEN-PLUGIN-SHELL-DESIGN-1 / 2026-10-12
- 卡面对应：设计五件之 2；设计前提 2（异步交互模型——"不锁 UI/不锁音频引擎/不占对话流"、"明确不做立刻生成出现"）与前提 3 尾句（"真正难点=异步（作业+状态参数+缓冲）非协议"）。
- 交叉引用：件一（通道分层/状态参数）；`../../runs/GEN-UNIFY-CTX-DESIGN-1/04-scene-boundary.md`（scene 义务）。

## 1. 作业生命周期与状态承载

```
idle ──submit──▶ queued ──claim──▶ running ──▶ downloading ──▶ landing ──▶ done
                   │                  │           │              │
                   │                  │(任何阶段失败)             │
                   ▼                  ▼           ▼              ▼
              cancelled ◀──cancel──(cancel_requested)         failed(error_class)
```

**状态承载=内核既有 `VitAIGCJobRuntime` 零结构改动**（复用裁定）：

- 作业登记/推进全走既有 `aigc_register_job` 命令的 `job_state`/`status_message` 字符串 upsert（JobEventService.cpp:32-61 亲勘，值域自由字符串；缺省 "idle"/"bypass"）。
- 新增值域（仅字符串扩展，Record 结构不动——VitAIGCJobRuntime.h:8-19 亲验字段集）：`queued / running / downloading / landing / done / failed / cancel_requested / cancelled / interrupted`（claim 动作=queued→running 的 upsert）。
- §11 兼容声明：旧读方对未知 job_state 字符串=字符串比较不崩；快照面（snapshotProjectJobs）原样携带。壳 GUI 只识别已知值，未知值显示原串（不静默）。
- `source` 字段固定 `"gen_shell:<rack节点id>"`——作业与壳实例绑定；`request_hash`=提交请求 JSON 的 sha256（审计锚定，载荷本体在壳 chunk，件一 §5）。
- ghost 集成：提交时宿主服务经既有 `set_async_ghost_state` 在目标区域置 ghost 占位；落轨时 ingest 链把 clip 置 present（GeneratedAssetService 既有语义，GEN-MAC-BASELINE-1 assets_ghost_state_ok 门实证）；终态 failed/cancelled 时 ghost 清除（set_async_ghost_state 既有命令）。

**异步性落实（前提 2 逐条）**：提交=T2 消息（内核消息线程处理，微秒级，不锁音频线程——T2/controller 消息走 VST3 主线程面）；执行=agent 进程后台 runner（不占对话流——runner 是独立轮询服务，与 LLM 会话循环无交集，件四 §3）；等待期 UI=壳 GUI 定时器轮询宿主服务（进程内，500ms）刷新；音频引擎=零占用（渲染互斥只在 print 路径短暂出现，§6）。

## 2. 提交请求结构（submit_request 载荷定稿）

```json
{ "schema_version": "vit.genshell.request.v1",
  "job_id": "<uuid，壳侧生成>",
  "shell_node_id": "<rack 节点 id，宿主回填>",
  "track_id": "<壳所在轨>", "start_time": <秒>, "length_beats": <拍，区域建议>,
  "scene": "in_project",
  "provider_id": "<genapi provider 条目 id>", "model": "<可选覆盖>",
  "template_id": "<模板 id>", "form": { "<field_id>": "<value>" }, "free_text": "<用户自由文本原文>",
  "gen_params": { "duration_seconds": 0, "count": 1, "style_tags": "", "instrumental": false, "title": "" } }
```

- `free_text`=USER 块的**结构化字段编排结果+用户原文**合成体（模板面产物，件三 §2）——agent 侧把它作为 `userFreeText` 整体交给渲染器；壳不再拆分（用户块契约：渲染器只认一个自由文本串）。
- `gen_params` 直接映射 `genapi.GenerationRequest` 字段（genapi.go:43-53 亲验）；`prompt/lyrics/tempo_bpm/musical_key` **不由壳携带**——prompt 由 agent 渲染终文填充、tempo/key 从快照注入（统一化纪律：工程特征不经壳旁路）。
- `count>1` = 多候选：同 take_stack 落轨（gen_generate.go:110-116 先例），壳 GUI 候选列表+`switch_asset_take` 切换。

## 3. 失败分类（对齐 genapi 既有分类+新增 3 类）

| 类别 | 源 | Retryable 语义（GUI 重试按钮显隐） |
|---|---|---|
| config_error / auth_error / quota_error / rate_limit / not_found / provider_error / moderation_error / timeout / network_error / response_parse_error / unsupported_format | genapi `ErrorClass` 11 枚举**原样透传**（errors.go:12-23 亲验；`Retryable` 标志随行——timeout/network/rate_limit 类为真） | 按 Retryable |
| `landing_error` | ingest 失败（track 不存在/文件不可读/渲染期拒令） | 是（产物在盘上，修复后可重试落轨） |
| `context_error` | 快照组装失败（get_project_state 失败/genctx 未就绪） | 是（工程态问题修复后重试） |
| `interrupted` | agent 重启清扫孤儿作业（§7） | 是（重新提交等价重试） |

- 分类入 job record 的 `status_message`（结构化前缀 `class=<x>` +人类可读详情）；违例/失败**不静默**：每次终态失败在壳 GUI 状态区+内核日志双留痕（AGENTS §8 失败分类纪律）。
- **音频产物无机检面**（统一化件三 §3 C 形态边界如实继承）：MUST 声明照发、产物不分析；壳 v1 不落轨 MIDI 产物（`mid` 格式 artifact 到达=标记 `unsupported_format_for_landing` 类+工件路径保留——MIDI 生成归编曲线，AIGC-DRUM-GEN-1 后置裁定口径）。

## 4. 提交可靠性：队列+轮询（不依赖 PUB 事件）

**v1 裁定=轮询契约，零新 PUB topic**：

```
壳 submit(T2) → 宿主服务入队（内存注册表）+ job upsert(queued) + ghost 置位
agent runner 每 2s 轮询 gen_shell.pending（新读命令）→ 取 queued 作业+请求载荷（服务从壳 chunk 拉）
  → claim（upsert running；单 agent 单栈所有权=AGENTS §9，无竞争claimer）→ 执行
终态：upsert(done/failed/cancelled) + 落轨（或工件路径回执）
```

- 轮询而非事件：PUB 是 at-most-once（agent 重启窗口丢事件=作业悬挂）；轮询 at-least-once 且自愈（队列在内核+壳 chunk 持久，agent 回来即领）。2s 轮询在分钟级生成作业上延迟可忽略。PUB 事件唤醒=v2 优化（非契约）。
- `gen_shell.pending` 应答（**唯一新命令**）：`{ jobs: [ {job_id, state, cancel_requested, request} ], providers: [ secret-free 投影 ] }`——providers 目录顺带刷新（省一条推送命令；agent 启动+每次轮询即同步）。
- 取消：壳 cancel(T2) → 服务 upsert `cancel_requested` → runner 轮询发现 → `ctx.Cancel()`（genapi.Generate 全链 ctx 感知——genapi.go:94-101 亲验）→ 终态 cancelled。provider 侧任务**不可真取消**（外部 API 无取消面）：取消=放弃下载/不落轨，已产生费用如实告知（GUI 文案固定）。
- **落轨后的取消**=删除 clip：破坏性操作，仅用户在 GUI 显式确认后走既有 clip 删除命令，不由 runner 自动执行。

## 5. 缓冲播放（图内输出，VST3 本职）

- 壳=instrument：宿主把产物 WAV 路径经 `audition_ready` 推给壳 → 壳后台线程解码进 `AudioBuffer` → 走带播放至目标区域时按进程块时间戳同步流播（标准采样器行为）。
- 采样率不匹配（artifact≠工程设备采样率）→ 壳内线性重采样（有界工作量，v1 自实现）。
- **落轨后 clip=权威形态**：壳缓冲仅服务于即时试听/AB 对照（take 切换由 `switch_asset_take` 既有面管，重指 clip 源文件——GeneratedAssetService.cpp:348-349 亲勘）；工程重开后缓冲按 `audition_buffer_ref` 惰性重载（chunk 记 job_id/take_id，artifact 路径经 job_history 回查）。
- `Bypass` 参数=图内播放静音（不影响已落轨 clip）。

## 6. 落轨分工（fast/print 双路径）

| 路径 | 触发 | 链 | 先例 |
|---|---|---|---|
| **fast（v1 默认）** | WAV 产物+用户未要求"打印所听" | artifact 直 `bridge_ingest_generated_asset`（track/start_time/take_stack/job_id 从请求） | GEN-API-ADAPTER-1 17/17（gen_generate.go:94-148 同模式） |
| **print** | 非 WAV 产物经壳解码可播 / 用户显式选"打印所听"（含链上处理） | 壳图内输出就绪 → `start_render(file_path=工件目录, range=目标区域)` → render_done → WAV → ingest | GEN-MAC-BASELINE-1 render 腿 A 三门（job_started/file_completed/surface_alive）+ RLM-PROFILE-2 renderResults 终态缓存消费（harness.go:3985-4057 亲勘） |

- 非 WAV 且壳不可解码 → `unsupported_format_for_landing`（不静默转码——转码腿 v2 另议）。
- print 路径时序纪律：渲染期内核命令白名单拒写（CommandDispatcher.cpp:2242-2276 亲勘）——runner 必须等 `render_done` 遥测（WaitRender 既有面）后再发 ingest，不得并发。
- take 栈跨会话边界（既有事实如实声明）：`VitTakeHistoryStack` 为内核进程内存态（VitTakeHistoryStack.cpp:24-33 亲勘），重启后栈记录丢失、clip 上 vit_* 属性残留——壳 GUI 的候选切换在重启后经 job_history+clip 属性降级为"文件重指提示"，不冒充栈仍在。

## 7. 孤儿清扫与崩溃恢复

- **agent 重启**：runner 启动时把 job record 中 source=gen_shell 且 state∈{running,downloading,landing} 的作业 upsert `interrupted`（环境中断语义，AGENTS §8/§9 口径；不把中断写成失败归因模型）。用户可一键重提交（请求载荷在壳 chunk，仍在）。
- **内核重启**（工程重开）：壳 chunk 恢复 pending_requests → 服务重建队列 → agent 轮询续领；in-flight 作业同 `interrupted` 清扫（job record 随工程持久，重启后 runner 首轮轮询处理）。
- **壳崩溃**：壳在内核进程内（无进程外隔离——与 pluginprobe 的 crash-isolated worker 不同，vst3host/adapter.go:24-30 亲勘其只读隔离面）——风险表 06 §6 在案：接受风险+缓解（壳音频线程零分配零锁、解码后台线程 try/catch 包络、代码面小+自有二进制）。
- **artifact 缓存目录**：genapi 既有 `~/.vit/genapi-cache/<provider>/`（audio.go:66-73 亲勘）——壳工件零新增目录纪律；print WAV 落 run 工件目录（烟测）或 genapi-cache（产品），不入源码树（AGENTS §10）。

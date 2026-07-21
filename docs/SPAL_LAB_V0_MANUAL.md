# SPAL Lab v0 手测手册

`SPAL Lab` 是 SPAL 的开发者实验入口。它验证的是：

```text
已学习并确认的 Plugin Skill
  -> 显式 TDR Nova conformance / 实例登记
  -> 语义静态 Bell Proposal
  -> 人工确认
  -> VSP 批量参数写入、回读、Receipt
  -> 自动同 tap、同范围的 L2 前后信号方向检查
  -> 独立 Proposal 的回滚
```

它不是 B4 用户功能、不是聊天命令，也不会自动加载 TDR Nova 或任何其它插件。

## 前置条件

1. 使用隔离的测试工程，目标轨中由操作者手动加载 TDR Nova VST3 `2.2.2`。
2. 重新构建并启动当前 VitApp。SPAL Lab 会协商 VSP 的
   `plugin.set_params_batch` 和 `audio.l2_render_probe` feature flags；后者缺失
   不会阻断参数执行，但会把信号验证明确记为 `inconclusive`。
3. 对该**当前实例**完成 Plugin Learning。`get_plugin_parameters` 必须返回
   v2 `plugin_skill`，其身份必须是：
   - `profile_key = plugin_f9788adda4203df8`
   - `name = TDR Nova`
   - `format = VST3`
   - `version = 2.2.2`
4. 选定一个 Band（首次建议 `band1`），由操作者确认它已经是静态 Bell，且
   dynamic mode 关闭。Lab 不猜测 TDR Nova 的 filter-type 枚举值。
5. 该 Band 的 `enable`、`dyn_enable`、`frequency`、`gain`、`q` 映射都必须
   是已确认且有 min/max/unit display domain 的 Plugin Skill 映射；并且要有
   `type` 映射。Conform 会记录当前 type 的 raw/normalized/display 值作为
   static-Bell invariant。

如果任一步不满足，应该回到 Plugin Learning/人工确认；不要手填或绕过
Provider Store。

## 运行入口

在 `D:\Vit_DAW\agent` 下执行：

```powershell
go run ./cmd/spallab help
```

默认持久化文件位于用户配置目录 `Vit\Agent` 下。可用以下环境变量或同名 flag
改写位置：

- `VIT_SPAL_LAB_PROVIDER_STORE`
- `VIT_SPAL_LAB_SESSION_STORE`
- `VIT_SPAL_LAB_JOURNAL`

## 1. Conform：登记一个实验实例

此命令只读当前插件参数，然后把成功的 conformance 写入独立的 lab Provider
Store；不会加载、写入或替换插件。

```powershell
go run ./cmd/spallab conform `
  -track-id "<轨道 ID>" `
  -plugin-id "<TDR Nova 实例 ID>" `
  -target-ref "track:bass" `
  -band-slot "band1" `
  -static-bell-confirmed `
  -evidence "manual:tdr-nova-band1-static-bell"
```

成功输出的 `instance` 必须包含：

- `status: verified`
- `spal_lab_only: true`
- `static_bell_ready: true`

没有这个实例时，后续 `plan` 必须阻塞为 `no_verified_provider`，不会自动选择或
加载 TDR Nova。

## 2. Plan：只生成 Proposal

以下例子表达“在 bass 轨 92 Hz 以 Q 1.2 做 -2.5 dB 的静态 Bell cut”，而非
“使用 TDR Nova 的某个参数”。

```powershell
go run ./cmd/spallab plan `
  -session-id "spal-lab-bass-01" `
  -goal "SPAL Lab: check a bounded bass low-end cut" `
  -target-ref "track:bass" `
  -frequency-hz 92 `
  -gain-db -2.5 `
  -q 1.2 `
  -max-abs-gain-db 3 `
  -evidence "lab:low-end-observation-01" `
  -expected-band-low-hz 80 `
  -expected-band-high-hz 110 `
  -expected-direction decrease `
  -signal-probe-start-seconds 0 `
  -signal-probe-end-seconds 12
```

若希望自动信号验证，Plan 必须同时冻结一个验证范围。二选一：

- `-signal-probe-clip-id`：对指定 clip 的编辑时间范围渲染；
- `-signal-probe-start-seconds 0 -signal-probe-end-seconds 12`：对明确的
  轨道时间范围渲染。

两者不能混用。Lab 不会因为未给范围而悄悄选择轨道上的第一个 clip。可选
`-signal-probe-tail-seconds 0.25` 会一并冻结进 Proposal。

此步骤会先重新读取当前实例，核对参数签名、Plugin Skill、Band type 的
static-Bell invariant，然后才捕获物理 preimage、冻结 Provider binding 和
Project Cut。任一项不一致都会拒绝生成 Proposal；该步骤不会改变工程。保存输出中的
精确 `proposal.id`。

## 3. Approve 与 Execute

只有显示过 Proposal 后，才用其精确 ID 进行人工批准：

```powershell
go run ./cmd/spallab approve `
  -session-id "spal-lab-bass-01" `
  -proposal-id "<plan 输出的 proposal ID>" `
  -source "manual-spal-lab-confirmation"

go run ./cmd/spallab execute -session-id "spal-lab-bass-01"
```

`execute` 会执行：Project Cut preflight、冻结 preimage 比较、VSP
`plugin.set_params_batch`、参数回读，以及失败时的补偿回滚。若 Plan 冻结了验证
范围且内核支持 L2，它还会在**写入前**和**写入后**各请求一次同一
`track_post_fader` / `offline_probe` 范围的离线渲染，并要求两个 render revision
不同。目标频带会作为精确 L2 analysis band 一并请求，而不是拿宽泛的 bass 区间
替代。前后 probe、质量证据和 evidence refs 会写进 Action Receipt。

这些 probe 是只读观察，不绕过 Proposal、Project Cut、参数回读或 Receipt；它们
也不会因暂时不可用而暗中重试或改写参数。

最终状态通常是 `needs_review`，即使方向检查通过也一样：

- Structural Verification：应为 `pass`；
- Signal Verification：可以是 `pass`、`mismatch`、`unsupported` 或
  `inconclusive`。`mismatch` 表示参数已正确写入但目标频带的实测方向未符合预期，
  需要复核；它不是参数执行失败；
- Musical/User Verification：始终是 `unknown`，等待人工试听。

因此 `signal=pass` 仅表示“同 tap 的目标频带朝预期方向变化”，并不表示“混音已
成功”或“更好听”。

查看结果：

```powershell
go run ./cmd/spallab status -session-id "spal-lab-bass-01"
```

## 4. 手工 Signal Direction 诊断（兼容入口）

正常 Lab 路径会自动采样并把证据写入 Receipt。本命令保留给外部 L2 数据或定位
传输问题；它只输出诊断结果，不会回写已结束 Session。

先对同一 tap、相同 render mode 生成前后 L2 render probe JSON。最小格式：

```json
{
  "tap_point": "track:bass:post_plugin",
  "render_mode": "offline",
  "render_revision": "before-001",
  "evidence_ref": "l2:before-001",
  "bands": [{ "min_hz": 20, "max_hz": 200, "energy_db": -18.4 }]
}
```

`after` 文件必须保持相同 `tap_point` 和 `render_mode`，但使用不同
`render_revision`。然后执行：

```powershell
go run ./cmd/spallab signal-check `
  -session-id "spal-lab-bass-01" `
  -before ".\before_probe.json" `
  -after ".\after_probe.json"
```

结果仅说明目标频段能量是否朝预期方向变化；它不宣称“混音更好听”。

## 5. 回滚也是一条 Proposal

不要使用 raw parameter write 覆盖原状。创建独立 rollback Proposal：

```powershell
go run ./cmd/spallab rollback-plan `
  -session-id "spal-lab-bass-01" `
  -rollback-session-id "spal-lab-bass-01-rollback"

go run ./cmd/spallab approve `
  -session-id "spal-lab-bass-01-rollback" `
  -proposal-id "<rollback proposal ID>"

go run ./cmd/spallab execute -session-id "spal-lab-bass-01-rollback"
```

Rollback planning 会先回读当前参数，并且只有当前值仍然等于原操作的冻结 postimage
时才会生成 Proposal。任何手动或并发改值都会 fail-closed，避免覆盖用户修改。

## 结果解释

- `no_verified_provider`：尚未为这个 target 注册可用的已验证实例。
- Plugin Skill / current readback mismatch：重新学习或确认当前插件版本/参数布局。
- `stale_project_cut`：在 plan 与 execute 之间工程已变化；重新 plan。
- rollback unsafe：原操作后有人或别的流程改变过参数；不要自动覆盖。
- `needs_review`：参数执行成功，但尚无 Signal 或人工试听结论；不是失败，也不是
  “混音成功”。

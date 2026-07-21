Status: superseded by ADR-AGENT-CLEANUP-0001

# VPS v3 Foundation

本文记录 ADR-SPAL-001 的第一阶段实现边界。它建立用户级 VPS Library、
Credential Core、`spectral.static_eq.v0` conformance profile、由 Credential
派生的 Provider Catalog，以及旧数据的安全迁移入口。

## 权威数据与所有权

`agent/internal/vps` 定义四类不同对象：

| 对象 | 归属 | 是否可调度 |
| --- | --- | --- |
| `PluginInventoryEntry` | 用户级 VPS Library | 否；仅表示已发现的安装插件 |
| `VPSDocument` | 用户级 VPS Library | 否；它是学习、映射和证据的规范档案 |
| `ProviderCredential` | VPSDocument 内 | 仅 `verified` 状态且 conformance 完整时可调度 |
| `ProviderCatalog` | 从所有 VPS 的 verified Credential 即时派生 | 是；但不落盘为第二个事实源 |
| Project Provider Instance | 项目执行层 | 当前 TDR Reference EQ 竖切会从已验证 Credential 和 fresh live surface 构造并验证运行时实例；不写回 VPS Library |
| Resource Lease | 项目执行层 | 后续 SPAL 资源化执行阶段实现 |

Credential 不含 project、track、plugin instance 或 Band lease 字段。当前
Reference EQ 竖切的 `Project Provider Instance` 由执行前 fresh readback 临时构造，
不会写回 VPS Library；持久化 `Resource Lease` 与多动作所有权仍是后续阶段。

`ControlSurface` 同时持久化 parameter/display mappings 和 `macro_relations`；
从 Plugin Skill v2 operation 或旧 ProfilePatch virtual control 导入的宏关系不再
另存为无关联的配置。

## 持久化

默认 Library 路径为：

```text
%APPDATA%\Vit\Agent\vps_library_v3.json
```

可通过 `VIT_VPS_LIBRARY_V3_PATH` 覆盖。文件使用原子临时文件替换，并且只
保存 `VPSDocument`；Catalog 每次从 Library 读取结果派生。

## `spectral.static_eq.v0`

第一份 Capability Conformance Profile 要求：

- `allocate_band`、`patch_band`、`read_band`、`release_band` 与 `bell_cut`；
- `filter_type`、`frequency_hz`、`gain_db`、`q`、`enabled`；
- 至少 `bell` filter type；
- 写入/回读、边界和回滚测试均通过，且有证据引用；
- 插件 installation、parameter-surface、display-surface 三种 fingerprint
  完整且与 Credential 绑定时完全一致。

每份 VPS 会持久化其 `capability_conformance_profiles`。Verified Credential
必须引用同一 VPS 中的 profile；代码只负责校验该 profile 是否满足正式的
`spectral.static_eq.v0` 标准，Catalog 不复制或篡改这份规范。

`BuildPluginFingerprint` 使用当前 host observation 的 parameter ID、type、
min/max range、enum values 生成 parameter-surface signature，并使用每个参数的
display domain、unit、scale 生成 display-surface signature；它不是旧 v2 的
“只对参数 ID 排序哈希”。

`BuildPluginFingerprintFromDigest` 是现有 Plugin Grabber `ParameterDigest`
到该模型的 fail-closed bridge：缺少完整 range 或 display domain 时会拒绝生成
可用于 Credential 的 fingerprint，而不是猜测或沿用旧 hash。

任一 fingerprint 改变会把已验证 Credential 置为 `stale`。它不会自动恢复，
也不会再出现在 Catalog。`revoked` 同样不可恢复为可调度状态。

Credential 状态机禁止 `pending_requalification -> verified` 和
`stale -> verified` 的直接跳转。后续学习/conformance 流程必须产生新的
candidate/credential revision，并通过显式 `Requalify` 写入完整 fingerprint 与
全部 conformance 证据。

## 兼容迁移

下列入口全部是显式、幂等且非破坏性的：

- `ImportPluginSkillDocumentV2`
- `ImportProfilePatch`
- `ImportLegacyVPS`
- `ImportSPALV0ProviderRecord`
- `ImportSPALV0ProviderStore`
- `ImportDefaultSPALV0ReferenceEQProviderStore`

PluginSkillDocument v2、ProfilePatch 和旧 `.vps` 都导入为 `draft`，其既有
参数签名只保留在 `legacy_parameter_signature` 证据字段中，不满足 v3 的完整
fingerprint 条件。

现有 TDR Nova SPAL v0 `ProviderRecord` 被导入为
`pending_requalification` Credential。它不会自动进入 v3 Catalog，也不会改变
或覆盖原来的 v0 Provider Store；因此既有 SPAL Reference EQ v0 路径保持独立。

## 本阶段刻意未做的事

本 Foundation 之外，TDR Reference EQ 已获得一个使用 VPS v3 Catalog 的受限产品
竖切（见下文）。它仍不实现：

- Generic Plugin Learning capabilities beyond the first confirmed
  `spectral.static_eq.v0` Credential path;
- Band lease、所有权和多动作资源化执行；
- B4 到 SPAL v3 的正式竖切。

这些阶段必须使用本 Library 的 verified Credential 和派生 Catalog，不能重新
引入 raw parameter 或 v0 ProviderRecord 的绕过路径。

## 验证

```powershell
cd D:\Vit_DAW\agent
go test .\internal\vps
go test .\internal\spal .\internal\spallab .\internal\workflows\plugingrabber
```

## Plugin Learning to Credential (implemented)

The previous Foundation-only boundary is now superseded for the first
`spectral.static_eq.v0` path:

1. A confirmed Plugin Learning save retains the validated Plugin Skill and
   performs a fresh host parameter-surface read.
2. Vit writes or updates the user-level VPS Library. The record contains no
   project track, loaded-plugin-instance, or lease identifiers.
3. The agent hashes the local plug-in package, then builds the v3 installation,
   parameter-surface, and display-surface fingerprints. Any unreadable package
   or incomplete display/range evidence leaves the VPS mapped but
   non-dispatchable.
4. For a fully confirmed EQ-band mapping, it verifies that the currently read
   filter type is Bell, performs temporary write/readback probes, checks
   parameter boundaries, and restores the exact pre-test values. The probe is
   run only inside the already user-confirmed Plugin Learning transaction.
5. Only a passing structural conformance run issues a `verified` Credential.
   The Provider Catalog is immediately derived from that Credential; failed,
   stale, revoked, candidate, and imported records remain absent.

The read-only catalog is available from `GET /agent/vps/catalog`. It is a
derived view only and does not create a VPS, issue a Credential, or modify the
legacy SPAL v0 Reference EQ Provider Store.

## TDR Nova Reference EQ VPS v3 竖切（已实现）

`spal.reference_eq_test.v0` 现在把 VPS v3 Catalog 作为唯一的 Provider 来源：

1. Godot 选择上下文指定现有的 target track、plugin instance 和可选 clip。
2. 自然语言 Plugin Learning 在用户确认的事务内生成/更新 VPS、执行静态 Bell
   conformance，并在通过后自动签发 `verified` Credential。
3. Reference EQ Resolver 只读取 Catalog 中的 `spectral.static_eq.v0`
   Credential；它按当前实例做 fresh parameter-surface 读取、fingerprint 比对和
   Bell 映射校验。失败时会 fail closed，且没有 SPAL v0 ProviderRecord fallback。
4. 通过校验后，运行时构造 Project Provider Instance、Credential 派生的 SPAL
   adapter、强 Project Cut 与 Frozen Proposal；执行继续保留 VSP readback、Receipt、
   分层 verification 和单独确认的 rollback。

这一竖切验证“已重新资格验证的 TDR VPS 能驱动已有 Reference EQ 任务”，不宣称
已完成多动作 Band lease/所有权或 B4 自动处理。真实 Godot 验收入口为：

```powershell
powershell -ExecutionPolicy Bypass -File D:\Vit_DAW\scripts\run_vit_product_path_smoke.ps1 `
  -RepoRoot D:\Vit_DAW -SkipBuild -SPALReferenceEQAgentOnly -TimeoutSeconds 90
```

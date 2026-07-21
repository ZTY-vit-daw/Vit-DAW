# SPAL Reference EQ Product Path v0 — VPS v3 Provider Bridge

状态：实验性产品竖切；手测门禁。

`spal.reference_eq_test.v0` 仍是一个刻意狭窄的显式静态 Bell EQ
测试能力，**不是** B4 诊断或自动低频处理。自 VPS v3 切换后，它的唯一
可执行 Provider 来源是由已验证 VPS Credential 派生的 Catalog；旧
SPAL v0 `ProviderRecord` 仅作为迁移/兼容证据，不会成为此路径的 fallback。

```text
Godot 启动 Kernel / VSP Hub / VitAgent
  -> 用户选中已加载的插件实例
  -> Ask Vit 自然语言 Plugin Learning
  -> 受控 conformance（写入 / 回读 / 边界 / 恢复）
  -> VPS v3 中签发 verified Credential
  -> 从 VPS Library 派生 Provider Catalog
  -> 显式静态 Bell 语义请求
  -> 已验证的当前项目实例 + 强 Project Cut + Frozen Proposal
  -> 用户确认
  -> SPAL / VSP 写入、回读、Receipt
  -> 分开报告 structural / signal / musical-user 结果
  -> 单独确认的 rollback
```

## 范围和非目标

- Capability ID：`spal.reference_eq_test.v0`。
- VPS Capability：`spectral.static_eq.v0`；当前只调度已经验证的 Bell
  子集。
- 语义 schema：`spectral.static_bell.v1`；所需值为 target、frequency、gain、
  Q，以及同一目标轨上的 selected clip/显式 clip 或显式时间范围。
- Capability 请求、Proposal 和用户回复不暴露 TDR Nova 名称、Band 编号或原始
  parameter ID。
- 当前竖切复用用户已明确放在目标轨上的已验证实例；它不把 TDR Nova 或任意
  插件当作 fallback 自动加载。
- 它不实现 B4 诊断、自动低频清理、任意 VST 控制、持久化 Band lease 或多动作
  资源调度。后两项属于 ADR-SPAL-001 的后续 SPAL 资源化执行阶段。

## 前置条件

1. 从 Godot 产品路径启动当前 `agent\bin\VitAgent.exe`，并连接声明
   `command.base_revision_cas` 的 Kernel。没有它，不会生成可执行 Proposal。
2. 在隔离测试工程中，用户明确把待学习的 TDR Nova（或其他匹配插件）放在目标轨。
   当前竖切不会替用户猜测、选择或加载 fallback 实例。
3. 选中该 track、plugin 和一个同轨 clip，然后在 Ask Vit 中请求：

   ```text
   学习当前选中的 TDR Nova，并建立 VPS v3 静态 Bell EQ Credential。
   ```

   完成学习会话中的必要确认。成功结果必须包含
   `credential_status=verified`、`catalog_visible=true` 和 `credential_id`。
4. `GET /agent/vps/catalog` 必须能看到这枚 Credential；Catalog 是 VPS Library
   的派生视图，不能通过该 API 创建 VPS 或手工注册 Provider。
5. 若需要 signal check，选择同一目标轨的 clip 或提供明确时间范围。
   `audio.l2_render_probe` 优先；不可用时参数执行仍可完成，但 signal 结果必须是
   `inconclusive`。

## Ask Vit 请求

完成上面的自然语言学习后，在 Godot 中保持目标 track、plugin 和 clip 选中，输入：

```text
SPAL Reference EQ 测试：频率: 92Hz，增益: -2.5dB，Q: 1.2
```

Godot 的 selected `track_id`、`plugin_id` 与 `clip_id` 是显式 UI 上下文；Vit
不会自行挑选实例。没有选中上下文时，需提供完整语义 target 和 scope：

```text
SPAL Reference EQ 测试：目标: track:bass，频率: 92Hz，增益: -2.5dB，Q: 1.2，片段: clip-001
```

或使用显式范围：

```text
SPAL Reference EQ 测试：目标: track:bass，频率: 92Hz，增益: -2.5dB，Q: 1.2，范围: 0s-12s，尾音: 0.25s
```

当前测试安全边界是 ±6 dB；零增益因没有可验证的 signal direction 而被拒绝。

## 预期产品行为

确认前，Ask Vit 必须显示 Proposal，且其中包含：

- 语义静态 Bell 请求，而非 TDR Nova 原始参数；
- 冻结的 target、Project Cut、受限 scope、预期 signal direction 和可逆风险；
- `provider_source=vps_v3_catalog`、Credential ID 与当前 Project Provider
  Instance 的绑定证据；
- 明确说明 structural 成功不等同于音乐结果。

确认后：

1. Resolver 只从 VPS v3 Catalog 选择已验证 Credential，并再次检查安装、参数面和
   显示面 fingerprint 以及 live Bell 映射；不匹配会使 Credential stale。
2. VSP 检查 Project Cut 和冻结的 physical preimage。
3. SPAL 编译 Credential 中已经确认的 control surface，写入并回读参数。
4. Receipt 记录 manifest/binding、preimage/postimage、structural 结果，以及任何
   L2 before/after signal 证据。
5. Structural `pass` 仅表示预期参数已写入且回读一致；signal 的 `pass`、
   `mismatch` 或 `inconclusive` 单独报告，不能自动重试或改变 mutation 结果。
6. 自动结果中的 musical/user acceptance 保持 `unknown`；系统不得因参数或 signal
   成功而声称混音已经改善。

## 回滚

在已应用的 Reference EQ 测试后，输入：

```text
回滚 SPAL Reference EQ 测试
```

Vit 必须创建新的 rollback Proposal。它先验证当前 physical 值仍等于原 postimage；
任何人工或并发修改都 fail closed。rollback 自己拥有确认、Project Cut、回读、
Receipt 和反向 signal expectation。

## 预期 blocker

| 结果 | 含义 |
| --- | --- |
| `no_verified_vps_provider` | 当前 VPS Catalog 中没有可用的 verified `spectral.static_eq.v0` Credential；重新执行 Plugin Learning。 |
| `no_observed_vps_provider_instance` | Credential 存在，但当前工程没有与它指纹和 target 匹配的已加载实例。 |
| `provider_fingerprint_stale` | 安装、参数面或显示面已变化；Credential 已被置为 stale，必须重新资格验证。 |
| `stale_project_cut` | 工程在规划后改变；创建新 Proposal。 |
| `needs_review` | 参数应用已持久化，但 signal 证据不充分或用户试听仍未知。 |

## 自动化验收

```powershell
powershell -ExecutionPolicy Bypass -File D:\Vit_DAW\scripts\run_vit_product_path_smoke.ps1 `
  -RepoRoot D:\Vit_DAW -SkipBuild -SPALReferenceEQAgentOnly -TimeoutSeconds 90
```

此烟测从 Godot 拉起 Kernel、VSP Hub 和新构建的 VitAgent，删除旧 TDR 实验
Skill 文档，通过自然语言学习重新建立 VPS v3 Credential，并验证该 Credential
实际驱动现有 Reference EQ 任务及其 rollback。

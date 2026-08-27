# Phase D2-1.5-S1 设计记录：实验通道执行层表驱动 + 真实 EQ 插件路径（2026-08-27 晚窗）

- 卡：`queue/done/2026-08-27-D2-1-5-S1-table-driven-execution-layer.md`（本卡完成时移入 done）
- 分支：`codex/g1-g7-runtime-remediation`；引擎 GLM-5.3 flash 执行、GLM diff 首审待做
- 上游定标：`docs/D2_1_5_S1_EQ_PLUGIN_ANCHORS_2026-08-27.md`（§3.2 gain param_id 表、§4.3 值语义、§4.4 注入序列、§5.1 用户 PCA 裁定）与 `docs/D2_1_5_S1_EXECUTION_LAYER_INVENTORY_2026-08-27.md`（逐文件代码点清单）
- 直接复用：D2-1.5-S1a 冻结包 `agent/internal/experimentplugins/`（Load / NearestStaticEQBand / ValidateStaticEQAdmission，v1 库谓词）

## 1. 表驱动落点

### 1.1 `experiment.D1S1DomainSpec` 执行描述扩展（d1s1_domains.go）

在既有三个准入字段之外**只加数据字段**，validator 体逐字节未动：

| 字段 | track_gain | static_eq |
|---|---|---|
| `ActionIDSuffix` | `_gain` | `_eq` |
| `CapabilityID` | `static_mix.static_balance.v0` | `static_mix.static_eq.v0` |
| `ContractVersions` | `{free_state:d1_s1, action:track_gain_adjust}` | `{free_state:d1_s1, action:static_eq_band_adjust}` |
| Target/Before fingerprint 模板 | `track:{track}:fader(:_db):{db}` | `track:{track}:eq:{param}:pending` |
| `ObservationViewIDs` | `mix.multitrack_relationship` | 同左 |
| `Journal`（Summary/Tool/CommandLabel/Fields） | set_volume 形状 | set_plugin_param 形状（plugin_id ← plugin_id‖plugin_identifier、param_id、value←target_value） |
| `WriteBinding` | Channels=1 | PluginBound、StubParamIDFormat=`band_%d_gain`、Channels=2 |

消费方：chat 侧新增 `d1AssembleFrozenPlan`（两个 plan 函数共用 set/proposal/observation 尾装）、
`d1JournalRecordForAction`（journal Command map 组装）、`d1SubstituteFingerprint`（模板标记替换）、
`projectD1Execution` 观测 view 由表给出（未知域回落历史字面量）。经验证旧输出逐字节等价：
既有测试 `TestD1S1PlanContainsOneBoundedAction`、`TestD1S1StaticEQJournalRecordsSetPluginParam`
不经修改仍绿，另加回归锁 `TestD1S1TableDrivenBuildersReproduceLegacyPlans`。

偏差说明：卡内清单把"param_id 组装规则"列为 spec 字段；实现拆成两半——stub 纪
（`band_%d_gain` 写死保历史形状）进表的 `StubParamIDFormat` 字段留档，真实绑定必须经白名单，
在 chat 的 `d1StaticEQActionArgs` 组装。第三域若无需真实插件路径即可纯表接入。

### 1.2 `executionports.StaticEQVSPPort` 参数驱动泛化

- 动作名校验从硬编码改为构造注入 `CommandName`（空值保留 `static_eq_band_adjust` 默认）。
- `VSPClient` 接口增补 `SendVSPCommandWithIDs`：内核 `plugin.set_params_batch` 只按 typed 命令
  分派（`PluginRackControlService.cpp` 经 `VspKernelReference.cpp:2965`），legacy 信封不认。
  kernel.Client 本就实现该签名；唯一受影响实现是测试 fake。
- 每 action Args 带 `write_mode: normalized_batch_v1` 时切换真实插件管线，否则原管线逐字节保留：

```
preflight 校验按模式分流：NB 要求 plugin_path+ch2、不再要求 identifier/plugin_id
apply     instantiate_plugin {plugin_path}（identifier 键缺失 = 内核走路径直载 VST3 内省）
          get_plugin_parameters(include_parameters) → BuildParameterDigest → display domain 发现
            · display_domain_candidate(unit=dB,scale=linear,min/max) → n=(dB-min)/(max-min)
            · 否则 five-point value_to_string 样本曲线反解 eqNormalizedFromCurve（自 chat 移植逐字节同版）
          plugin.set_params_batch 单批写 ch1+ch2 两通道 normalized_value
            base_revision CAS 参与批命令（C++ validateCommandBaseRevision）——仍是一个幂等键一次 revision 前进
          回执前 fresh surface 复读：normalized 逐通道 ≤1e-4 为权威判据；value_text 物理解析（ParseEQLocalizedNumber）
            仅作 receipt 的 actual_readback_value 参考值（真实插件显示量化不做 dB 比对判据）
receipt   细节键在历史键族上追加 write_mode / plugin_path / param_id_ch2 / normalized_channels；
          历史 legacy 回执形状完全不变
reconcile 不变：无持久 plugin_id 一律 not_applied（路径实例化产生的实例 id 不可从持久 action 复核——现状语义）
```

`StaticBalanceVSPPort`（track_gain）零改动。

### 1.3 执行入口 PCA 门（chat.executeD1StaticEQ 顶部）

`resolveD1StaticEQWhitelistBinding(TypedAction)` 在依赖守卫**之前**运行（新建与会话恢复同一入口）：

1. Load(`~/.vit/free_state_experiment_plugins.json`)：文件不存在 → blocked 文案含 "not configured"
   与机器本地路径；损坏/未知字段 → "invalid"/"whitelist is invalid"。两类可区分。
2. TypedAction 若钉了 `plugin_identifier` 必须等于白名单条目（模型不能指定名单外插件）。
3. `processorattestation.NewStore("").Read()`（缺文件=合法空库）→
   `ValidateStaticEQAdmission(lib)`（V1 库谓词：BuildSubjectKey+FingerprintPath+QueryLibraryAdmission
   FamilyStaticEQ）。不 Eligible → "refused by the PCA admission check"+"not PCA-promoted"。
   store 双文件都坏 → 第四类 "could not evaluate its PCA admission"。
4. 通过后 NearestStaticEQBand(frequency_hz) 选 band，嵌入冻结 plan。

loader/reader 是包级函数变量，测试以 t.TempDir fixture 覆盖，绝不触真实 ~/.vit。

## 2. 已知边界

- 真实路径动作的 durable Reconcile 维持 not_applied（实例 id 运行时才知道）。与 stub 时代一致；
  后续如需跨重启复核，需把 runtime 解析出的 plugin_id 升格为持久会话事实——超出本卡范围。
- juce_eq 占位符 fallback 从生产路径消失，但保留为表内 stub 数据 + nil-binding 计划形状，
  用途限于回归锁定与离线工具链；executor 无法到达它（gate 先行硬失败）。
- eqNormalizedFromCurve/port 侧换算属"复制共享数学而非搬移"，chat 原件与其测试未动，避免本轮触碰
  生链路；两份的语义一致性由移植即逐字拷贝保证。
- 内核侧 display_probe 五点采样、descriptor 的 value_text/normalized_value 输出，
  与 anchor §3.3 描述一致，已在 VitApp/VitPluginGrabber.cpp 实地核对。

## 3. 验收（Go 级，flash 会话执行）

```
cd D:\Vit_DAW\agent
go build ./...                                    # PASS
go test ./internal/experiment ./internal/chat ./internal/executionports ./internal/executionverifiers -count=1   # PASS (ok×4)
go test ./... -count=1                            # PASS（全部 ok，无 FAIL）
```

真实栈烟测（run_free_state_d1_smoke.ps1 -PublicCaseId spv1_p01，含 frequency flavor 二连）
归 GLM 会话 diff 首审后执行；预期内核不再报 identifier not found、回执出现 value_text 物理读数。

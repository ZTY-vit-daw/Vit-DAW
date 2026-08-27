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

## 晚窗执行与 L2 修复链（2026-08-27，GLM）

S1（flash，abc0250）合入后实栈验证暴露四层执行缺口，GLM L2 连修（adc0f1b / b506ce3 / b8c1eb6 / f31c3c0 / bb45075，逐层根因与证据见 `docs/FREE_STATE_D1_SMOKE_HISTORY_STATS.md` §5）：

1. **闭包 revision 记账**：治理变异的回执 revision 必须在落地时确定性记入 audioclosure（`RecordGovernedMutation`，专用事件不清证据），被取代 revision 的观察重放跳过、shadow 滞后追及容忍——外部漂移的 stale 结算守卫不变。
2. **内核 CAS 时序**：instantiate 是工程状态变更（revision +1），随后的 `plugin.set_params_batch` 必须以 instantiate 后的快照为 `base_revision`，否则内核 stale_project_cut 拒绝（且错误文本在 ack/error 子对象，Go 侧已深挖透出）。
3. **持久 pending 生命周期**：mix.tick 执行成功后 durable pendingmanager 记录必须转终态，否则 continuation 投影复活已消费确认、驱动重复批准、预算空转。

**终态**：spv1_p01 frequency exit 0（static_eq 真实插件端到端，bx_hybrid V2 实例 1042 / param 827092295，rev 3→4）；track_gain 零回退（194037 exit 0）。

**开放项（S2 前排查）**：p02 × static_eq 2/2 缺 post-action CCB 观察（回执 applied、模型完成评估话术、实验轮无 post-action 观察记录；p01 同链路 PASS）——疑回合内声学验证在 p02 工程未产出 fresh 观察，需对照两 fixture 的验证器执行差异。

## D2-1.5-S2：第三域准入（broadband_compression，2026-08-27 晚窗 flash 执行）

**结论：泛化成立。** 执行层（executionports / executionverifiers / orchestration / executor）零改动；第三域准入成本收敛为"experiment 表行 + 白名单 v2 节 + chat 表驱动覆盖 + 烟测映射"。

### 锚点定标（探针证据，本会话 pluginprobe 离核 dump `temp/d2_1_5_s2_pluginprobe/`）

- 主锚点 A=**Vertigo VSC-2** 一次命中判据，未降级 B/C：PCA `broadband_compressor` 家族 **promoted**（`processor_control_attestations.v1.json`，subject 五字段与本机路径/指纹逐项一致，`sha256:a4cacf25…` 与磁盘二进制匹配）；threshold 双通道参数 `Threshold A=1416131121` / `Threshold B=1416131122`（stable vst3_hosted id，unit=dB，normalized 线性域，display +11.8 满量程）。
- PCA 家族谓词：v1 `QueryLibraryAdmission` 原生收 `FamilyBroadbandCompressor`（store.go 不需要新家族），白名单按节选家族查询即完成"PCA family 泛化"。

### 落地面（S2 定稿六条 → 实现）

| 裁定 | 实现 |
|---|---|
| 域规格 | `broadband_compression` / `broadband_threshold_adjust`，threshold_db 非零 ±2（与 static_eq gain_db 同级收紧），单实例单参数对（ch A/B 单批写），budget/attempts 不在表内照旧 1；无 stub 形态（生产先过白名单门） |
| 锚点 | 上节 VSC-2；`threshold_param_id_ch1/ch2` 入白名单节 |
| schema v2 | `SchemaVersion="…v2"`；新增 `broadband_compression` 节（7 字段校验 + ch1≠ch2）；`static_eq` 校验/错误语义逐字节不变（S1a 冻结接口保持，新增 `ErrCompressionNotConfigured`/`ValidateCompressionAdmission` 为平行增量）；真实 `~/.vit/free_state_experiment_plugins.json` 已升 v2，static_eq 节逐字保留 |
| PCA 门 | 白名单节→家族查询参数化（私有 `validateSectionAdmission(label,…,family)`），eq 前缀文案逐字节不变，压缩节用同构 `"broadband_compression …not PCA-promoted"` 前缀 |
| 视图绑定 | 表行 `ObservationViewIDs=["track.time_dynamics"]`（COM source_dynamics，非 static_level）；post-action CCB 观察的 Requested/ExecutedViewIDs 经既有 `d1ObservationViewIDsFor` 自动生效 |
| 回执键映射 | Journal 表行沿用 set_plugin_param 三键形状（plugin_id‖identifier / param_id / value←target_value）；端口用构造注入 `CommandName: spec.ActionKind`，回执/幂等键族与 static_eq 同族不变 |

### chat 表驱动自动覆盖（从硬编码分支改消费域表）

`D1S1DomainSpec` 新增三个纯数据字段：`AdmissionValueKey`（delta_db/gain_db/threshold_db）、`AdmissionPassthroughKeys`、`AppliedReplyText`。三行回填；行为零变化由既有测试保证（不经修改全绿）。覆盖点：准入 TypedAction/dose 组装（free_state_experiment_runtime）、pending 分发（mix_tick_confirmation，按 `WriteBinding.PluginBound` 路由而非域名字符串）、候选 Operation 白名单/摘要、proposal 路由门+case、journal 形状（port 新增 `journalSpec`，旧 bool 签名保留）、回执人类文案、计划构造（新增 `d1PluginParamPlanWithBinding`/`resolveD1PluginParamWhitelistBinding` 通用路径；旧 `d1StaticEQPlan*` 与 bool 版 journal 构造器逐字节保留为回归锁，泛型路径与 legacy 真实绑定路径以 ActionSetHash/Actions 深比较锁定等价）。

### 边际成本实测（泛化结论证据）

- 生产 diff：9 文件 +436/−77。其中"纯压缩域"成本 = 表行 ~55 行 + 白名单节 ~45 行 + runner 10 行 + chat 候选/文案枚举 ~15 行；其余为一次性的 v2 schema 机制与三字段表驱动改造（后续家族增量域复用，不再付）。
- 测试：3 个新文件 583 行（whitelist_v2×4 用例、chat S2×5 用例、experiment 行×1）。
- 烟测映射：`ADMITTED_DOMAIN_KINDS` 一行 + `compression` flavor + ps1 两个 ValidateSet 扩充（共 10 行）。
- prompt 自动更新（S3 成果）零改动生效：`freeStateD1AdmittedDomainRule` 从表枚举第三域。

### 验收

```
cd D:\Vit_DAW\agent
go build ./...                                    # PASS
go test ./internal/experimentplugins ./internal/experiment ./internal/chat ./internal/executionports ./internal/executionverifiers -count=1   # PASS×5
go test ./... -count=1                            # PASS（83 包 ok；首跑出现 1 例 processor-certification 测试
                                                  # 环境性抖动（HOME/USERPROFILE 重定向 + .vst3 fixture 实时扫描），
                                                  # 与本 diff 无交集，隔离/复跑均 PASS）
python -m py_compile scripts\free_state_d1_smoke.py   # PASS
```

真实栈 `-ExpectDomain broadband_compression` + compression flavor 烟测由 GLM 会话 diff 首审后执行；白名单/PCA/指纹已在离线层面预验一致（探针快照 + attestation 库比对）。p02 链式 tick 批准问题已另开 S2b 卡（py 驱动守卫，依赖本卡合入）。

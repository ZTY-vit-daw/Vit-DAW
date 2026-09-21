# FIX-F5-SNAPSHOT-FRESHNESS：声学桥快照新鲜度标注（前台铃机制）+ 原子发布

- 优先级 / 预估 / 依赖：P1 / 1 天 / 取证定责 FORENSIC-45-FAMILIES（两次实证+自收敛证据在案）
- 模型分级：L2 / GLM-5.3
- **背景**：⑤ 声学桥断言两次撞上 band_energy_summary.request_id=mixboard_时间戳 vs latest_request=kernel_prepared_* 的分叉（统计 ⑤R4 + 修复会话 ⑤R2）；事后快照自收敛（kernel_prepared/ready）证明是瞬态竞态窗。两条写路径（kernel L3 备谱 / mixboard 戳记）非原子更新同一快照。用户裁定方向：**双层各自在写入时自写新鲜度标注（前台铃机制——按铃即刷新，不按即未刷新）**；永久单侧缺口有先例（投影清单 §9 masking unavailable / mixed-tap withheld，按显式 missing+理由处理）。
- 目标：
  1. 代码定位：mixboard 戳记与 kernel_prepared 两条写路径及快照装配点（预期 agent/internal/mixboard + bridge/内核装配面，定位后收窄文件域）
  2. 写侧①（标注）：每个特征行携带所属 request_id + 新鲜度标注——当新 latest_request 发布时旧行必须被标 stale/superseded（标注由写入层自己写，不做读取侧推断）；贯彻 CCB"stale 原样保留不升级 readiness"纪律
  3. 写侧②（原子）：快照整体 copy-on-write 发布（表头+行同瞬替换），消除瞬态分叉窗
  4. 读/断言侧：分叉时一次有界重读区分瞬态/永久；断言对象从"不存在分叉"改为"不存在**未标注**的分叉"；永久分叉以"行 X 属旧请求 Y 且未标注"披露
  5. 红测试：构造双写竞态序列，修前断言红（未标注分叉）/修后绿（标注或收敛）
- §8：断言语义变更=本卡目标之一（非削弱：从"无分叉"到"无未披露分叉"，更诚实不是更宽松）；真栈 ⑤ 1 轮该段验证
- 文件域：定位后收窄（预期 agent/internal/mixboard、装配层，可能涉 VitApp 桥面——跨层则拆子卡）；scripts/run_vit_product_path_smoke.ps1 断言段
- 验收：①双写路径定位报告；②红测试；③回归绿；④⑤ 真栈该段过
- 停止条件：装配点在内核 C++ 深处且改动影响渲染主链 → 上交拆卡
- 领取：2026-09-21 PC 执行侧（GLM-5.3 / L2）。领取时 HEAD=origin/main=8e6e253d；工作树已有 diff：VitApp/Workspace/default_project.xml（运行时工程状态，不动）+ F2 卡 mv（另一会话）；untracked：coord/runs/、extension 构建产物、godot-cpp/。分支 fix/f5-snapshot-freshness。
- 定位报告（①）：双写路径与装配点全在 Go agent 侧，**不涉内核 C++，停止条件不触发**。
  - 路径 A（kernel_prepared，内核 telemetry 材化）：`harness.go` `ingestKernelL3AcousticTelemetry`/`ingestKernelSpectralTelemetry`/`ingestKernelWaveformTelemetry`（3955+）→ `kernelFeatureMaterializerRequestID`（4104，`kernel_prepared_<feature>_<clip>`）→ `writeMixboardReadyL3SummarySnapshot`（6034）。
  - 路径 B（mixboard 戳记，请求侧）：`requestMixboardObservationFeatures` 等（4754/4121）→ `newMixboardFeatureRequestPacket`（6763，`mixboard_<UTC纳秒>`）→ `writeMixboardReady*Snapshot` 家族（5969/6002/6034/6084）。
  - 装配/落盘单点：四个 `writeMixboardReady*Snapshot` 统一模式 Lock→ReadFile→`existing["latest_request"]=packet` 无条件覆盖→merge 行→`writeMixboardFeatureSnapshotFile`（7070，promote+normalize+os.WriteFile 直写）；masking（ccb_masking_observation.go:122）与 l2_probe_batch 也走该单点。所有写者在 `mixboardFeatureSnapshotWriteMu`（126）下。
  - 分叉根因：(1) 每次写无条件覆盖 latest_request、其余桥行按条件替换，两次写之间文件呈"latest=新/行=旧"逻辑分叉窗（进程内有锁，非物理竞态；断言在窗口内读即红）；(2) `normalizeMixboardBridgeRowForPacket`（7101+）写时把 kernel 家族 fresh 旧行 request_id 洗成新 packet id（过继），mixboard↔kernel 跨家族行不洗、无任何标注；(3) `writeMixboardFeatureSnapshotFile` 用 `os.WriteFile` 非原子替换。
  - 文件域（收窄）：`agent/internal/harness/harness.go`、`agent/internal/harness/ccb_masking_observation.go`（如需）、`agent/internal/mixboard/mixboard.go`（fresh 判定接受标注，最小化）、`scripts/run_vit_product_path_smoke.ps1`（Assert-BridgeSnapshotRow 段）、新测试 `agent/internal/harness/bridge_freshness_annotation_test.go`。
- 回执（2026-09-21，PC 执行侧）：
  - ②写侧标注：`writeMixboardFeatureSnapshotFile`（harness.go 落盘单点）新增 `annotateMixboardSnapshotFreshness`——每次落盘以 latest_request 为铃，同笔写入为全部桥行/数组行自写标注：本铃行 `freshness=current`；旧铃行 `freshness=stale`+`superseded_by_request`+`superseded_at`（保留行原 request_id 不洗写）；无铃归属有材料身份行 `material_reuse`+`reused_for_request`；missing 占位行跳过（status+reason 披露）。normalize 的 kernel 家族过继保留，但过继时加 `origin_request_id` 透明化。读侧 fresh 判定（`mixboardBridgeFeatureRowFreshness`）零改动：stale 行天然落入不 fresh 路径，CCB"stale 原样保留不升级 readiness"纪律自动满足。
  - ③原子发布：tmp+rename 整体替换（同目录同卷，Windows MoveFileEx REPLACE_EXISTING），tmp 写失败或 rename 失败退回直写；修前为 os.WriteFile 非原子直写。
  - ④断言语义：`Assert-BridgeSnapshotRow` mismatch Fail 改为"unannotated fork"（有 freshness 标注的分叉行 Write-WarnLine 披露不失败）；`Assert-FeatureSnapshotAcousticBridgeReadiness` 新增 `-SnapshotPath` 有界重读（600ms×2 区分瞬态/永久），authoritative snapshot 调用点已传路径；响应投影场景（无路径）未标注分叉即 Fail（投影无重读源，视为永久披露）。
  - ⑤红测试：`agent/internal/harness/bridge_freshness_annotation_test.go` 三测——确定性红（路径 B mixboard 戳记落 band 行→路径 A kernel_prepared 换铃→断言行必须带标注且保留原 id+superseded_by_request）修前红实证 `[band_energy_summary:mixboard_20260921T120000.000000001]`（与 ⑤R4/修复会话 ⑤R2 撞的形态一致）；并发双写轮询（40 轮交替+读者断言无未标注分叉）修前红 `read #4 [spectrogram_tiles:kernel_prepared_spectral_field_1011]`；原子替换契约（无 .tmp 残留+落盘完整）修后绿防线。修后三测全绿。
  - 回归：`go build ./...` 过；`go test ./...` 全量 exit 0（含 harness 全量 13.1s）；webui `npm run test` 28 文件 324 测试全绿。
  - 真栈 ⑤ 1 轮：`run_vit_product_path_smoke.ps1`（自建三件套，agent sha256=49C7D604...，HEAD=166d313a 工作树）**exit 0 PASS**——product observe / Chinese multitrack observe 声学桥断言过，无 unannotated fork 触发；工件 `VitApp/Workspace/Artifacts/smoke/product_path_20260921_220450`，日志 `coord/runs/FIX-F5-SNAPSHOT-FRESHNESS-1/run1_product_path_f5.log`（首启一次 bash 转义调用失败 exit 127 未跑，重启用 PowerShell 重定向成功，非烟测轮次）。
  - **新发现（超出本卡文件域，上交决策）**：真栈取证发现第三写者——Godot 前端 `app/kernel/autoloads/telemetry_manager.gd` 是 `kernel_prepared_waveform_envelope_<clip>` 铃的原始发明者：内存持有整份快照、750ms debounce、自身已是 tmp+rename 原子写、flush 前 `_merge_existing_mixboard_feature_request` preserve 磁盘行（原样保留 agent 标注）。三层写者协议闭环：agent 写必标注、Godot 换铃重置行为新铃占位（missing+reason）、Godot 聚合行 id==铃。残余风险：Godot `_mixboard_should_keep_prepared_feature_row` 的 preserve 分支（磁盘 prepared 行胜过内存弱状态行）理论上可落"行 id≠铃且无标注"形态（agent 未写过的行），本轮真栈未触发；Godot 工程在仓库外（D:\Godot\project），按卡面文件域纪律不擅自跨仓改——建议决策侧裁定：开子卡在 Godot flush 落盘前对 preserve 行补写同款标注，或接受烟测有界重读兜底。
  - 领取后工作树变化：F2 会话并行提交 main（166d313a）并在工作树改 chat 测试文件（-1 行），与本卡文件域（harness/mixboard/scripts）无重叠，未处置；F3 会话其后切分支 fix/f3-g4-semantics 到共享工作树（本卡 commit 经 stash 隔离完成，未触碰其改动）。
- 验收：待决策侧（回执见上；代码 commit fix/f5-snapshot-freshness 4e4d850d 已推 origin；真栈 run 工件 product_path_20260921_220450；Godot 第三写者发现上交裁定）。

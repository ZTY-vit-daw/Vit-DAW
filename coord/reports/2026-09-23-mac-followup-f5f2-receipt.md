# mac 跟随回执：转交包 §二 三步完成（脚本移植 F5+F2 + ④⑤ 复测对照）（2026-09-23）

> mac 执行侧回执。实现与回执同分支：`port/mac-followup-f5f2`（bbd0dbf 脚本移植 + 本回执）；复测跑在该树。工件根 `~/Documents/vit-mac-followup-artifacts/`（SELF_CHECK_bbd0dbf.md + retest_20260922/RUN_LEDGER.md）。裁定请求见 §四。

## 一、步 1：拉 main + 并树确认

- 树 = main 5678a79（五补丁+FOLD+六卡验收全在）；`go test ./...` 完整跑（真实退出码复核，首轮管道 tail 掩盖退出码已作废重跑）：**全部包 ok，唯一 FAIL = `internal/contextruntime` `TestDefaultSnapshotPathFindsVitAppFromNestedWorkdir`**——macOS `/var`→`/private/var` 符号链接致路径字面比较失败（context_test.go:537）；确定性平台既有问题（该包近史无代码变更、五补丁域不含它、PC 84 包绿一致）。按 §11 上交：建议并入 mac 环境已知项或开修复卡（证据 gotest_full_5678a79.log）。

## 二、步 2：脚本移植（本分支 bbd0dbf，2 文件 +80/-6）

- **F5（必需）**：⑤ `run_vit_product_path_smoke_mac.sh` `bridge_snapshot_check` 对齐 PC `Test-BridgeSnapshotRowFork` 分叉谓词——request_id 分叉且有 `freshness` 标注→warn 披露；无标注→fail（PC 措辞同款）；旧严格 `request_id mismatch` 语义移除。可选 `-SnapshotPath` 有界重读未做：mac 该断言在内存内调用点（observe 响应内嵌 snapshot）无文件路径可重读，PC 同位调用亦单次；mac 权威快照文件侧维持 schema-only 既有形态（未扩面）。
- **F2（升级必做）**：④⑤ `SETTLE_PY` 补 FIX-F2-SURFACE-REPLY 兜底——needs_clarification 且合成 reply 无问句特征（U+003F/U+FF1F）→ GET `/agent/runtime/status` 读 pending_interaction.reply（conversation 匹配+问句校验）替换并标 `clarify_reply_source=runtime_status_pending_interaction`；异常静默（双保险不加失败面）；AGENT_HTTP 经 argv 传入。
- 自检：bash -n 双绿；24 个 python heredoc 块全编译；真实代码+合成输入功能红绿 7/7（F5 标注披露/未标注 fail/匹配静默、④⑤兜底命中、无问句不动、agent 不可达静默）。首跑 ⑤ 用例一度红=测试脚本自身 argv 序错误（⑤ 无 placeholder 参数位），修正测试参数后全绿，被测代码未动。

## 三、步 3：④⑤ 复测 ×5 对照 WL-1 基线（树 bbd0dbf，flash，白名单 1971a39c…，驱动镜像 WL-1）

| 指标 | WL-1 基线（296ad46+白名单） | 复测 | 判读 |
|---|---|---|---|
| ④ 功能通过 | 2/5（F2×2、F4×1） | **4/4**（R4 环境批另计：起栈首条 chat 传输失败） | 红轮族全吸收 |
| ⑤ 通过 | 3/5（F3×1、F2×1、F5复合×1） | 3/5（clarify 链失败×1、F4 形态×1） | F3/F5复合消失；族迁移 |
| ⑤ vocal 停车率 | 4/5 | 3/5 | -1 |

- **补丁实效直接证据**：三个停车轮 reply 均天然含完整问句（Go 层 F2 修复直供，兜底零替换=双保险按预期）；④ F2/F4 清零；⑤ F5复合（白名单缺失）清零。
- **F5 断言实跑**：5 轮零误报零漏报；本批未遇 freshness 分叉（双写窗未出现，如实披露）——语义差分证据以合成红绿为准。
- **F2 兜底实跑**：触发条件命中 1 次（④ R3 vocal_guard needs_clarification+无问句）→ 查 runtime status，pending reply 同文无问句（模型 parked 内容本为"证据+建议"叙事）→ 按设计保持合成 reply，轮次经三分支容忍通过。

## 四、裁定请求（上交四项）

1. **⑤ R3 clarify 链失败（新形态，建议开取证/修复卡）**：run_id `vit_product_path_mac_20260922-235030`——vocal 轮 turn=3 LLM 调用 **42.2s err=false 成功返回**（23:52:03），11 秒后 `agent_loop_chat status=failed stop=failed completed_steps=0`（total 52.6s）；turn.failed 详情引用模型澄清问句「需要先确认哪条是主唱轨」。非 LLM 超时环境项（调用成功）、非 settle 超时（即时终止于 failed）；失败点在 LLM 返回后的 turn 处理/clarify-park 路径——FIX-F2-SURFACE-REPLY 触碰面邻域（goalHasLiveContinuationOwner / clarify-park 终态事件）。10 轮 1 例（§8 未到确定性断点连续两次）。
2. **⑤ 残差存在（R3+R5）**：按 2026-09-22 WL-1 验收裁定 b)「复测有残差再升一线」——**pro 对照升线资格条件成立**，是否升线请裁定。R5=F4 形态（done 不停车：observe×2 propose=0，reply「处理完成…建议 presence EQ lift…」无明示边界词，run_id …235453）。
3. ④ R4 chat transport 环境失败（driver_logs/mix_tick_r4.log:27）→ 并入已登记 mac 环境已知项。
4. contextruntime /var 符号链接测试失败（§一）→ 登记或开卡处置。

## 五、边界声明

- 本轮全部 10 轮为存在性+对照口径（非稳定性证明）；flash 引擎、白名单机器本地、key 零入工件（preflight 仅存在性）。
- 端测覆盖边界：go test+脚本合成红绿+真栈 ④⑤ 十轮；渲染面与用户旅程门槛未涉及（本变更域=脚本断言+settle 合成，非 webui/旅程）。
- todo/ 待领卡未动（PORT-WL-EQ-1、FIX-GD-TELEMETRY-BELL-1）。

# FIXTURE-WE-HYGIENE-1 执行报告：smoke fixture 波形包络 9/5 遗留卫生（294）

- 执行：GLM-5.3 ZCode 执行子代理（PC）/ 2026-11-07 / 卡 294 / 分支 `port/fixture-we-hygiene-1` / 基线 `66313d86`（领取提交 `8fb5e0e4` 已推 main 亲核）
- 输入：293 取证报告（`port/we-mismatch-forensics-1` @ `dd1eff6c`）+ 265 run 工件（只读，工程态捕获复用）+ fixture 实体（temp/，gitignore 域）
- 工件：`coord/runs/FIXTURE-WE-HYGIENE-1/`（rig 脚本×3+overlay、hash_ledger、removed_elements×4、verification_go_test_log、census_and_inventory）
- 零栈零源码改动：无 VitApp/Godot/agent 进程、无端口占用；源码树经 `go test -overlay` 注入测试文件验证（worktree `git status` 全程零改动亲核）

## 1. fixture 定位与哈希账

权威 fixture 实际存放：`D:\Vit_DAW\temp\semantic-processor-agent-project-smoke-v1\fixtures\semantic_processor_project_smoke_v1_80085263a651cf20`（`temp/` 在 `.gitignore:22`，不在版本控制；265/266/AF-D1 run 的 `public_source_project` 均指向此路径——run 工件亲证）。`fixture_manifest.json`（2026-08-30 建）只覆盖 `cases/*/stems` 材质哈希，不含 `.vit_agent`/`.vit_derived` 运行态——卫生编辑不触 manifest 面。

| 文件（spv1_p01 下） | 换入前 sha256 | 换入后 sha256 |
|---|---|---|
| `.vit_agent/vitproj_b3f2838183d64add9ca7824b8b3ac557/mixboard/mixboard_feature_snapshot.json` | `5c0c97d5…3527b` | `be05aafd…96634` |
| `.vit_derived/vitproj_b3f2838183d64add9ca7824b8b3ac557/acoustic_package_status.json` | `e43d159c…ce43` | `22416df0…b8d2` |

换入三重门：①复制时哈希=换入前重核哈希（期间无人改动）；②换入后权威位置字节 `cmp` 与副本工作区卫生版逐位一致；③manifest 54 stems sha256+size 复验全过（sealed 材质面零接触）。前态完整备份+被剔元素 JSON 全文在工件 `removed_elements/`（可逆）。

## 2. 剔除清单（卡面三处，外科最小口径）

副本工作区 `D:\Vit_DAW_ws_294hyg`（仓库外）操作，文本级外科剔除（Go MarshalIndent 制式保留，diff 仅删行+2 处逗号收口），每步经「解析+与 json 级删除预期 deep-equal」双验证：

1. **WE 槽行** `waveform_envelope` = `kernel_prepared_waveform_envelope_1055`（t=1007/c=1055，kernel_prepared_telemetry，first_received 2026-09-05T07:59:32.7597918Z，材质=strip_silence_agent_smoke_8764 wav）——整槽剔除；
2. **twe 末行** `track_waveform_envelopes[18]` (1007,1055)——剔除（19→18 行，三代×6 规范行完整）；
3. **声学包** `packages[6]` (1007,1055)（updated 2026-09-05T15:59:32.767+08:00，三层包）——剔除（7→6 包）。

**同族发现（只记录不扩剔，交验收裁定）**：`band_energy_summaries[18]`/`loudness_summaries[18]`/`stereo_relation_summaries[18]` 三行同为 (1007,1055)、`source=kernel_l3_offline_analyzer`、材质/时间戳（15:59:32.766/.767+08）与已剔声学包同源——同属 9/5 strip_silence L3 offline 遗留族，但**不在卡面枚举内且无读面暴露**（归一化 `normalizeProjectFeatureMaterialFreshness` 对数组行静默过滤，与 WE 槽的裸槽 stub 路径不同），按外科最小纪律保留，全文在 `removed_elements/siblings_record_only_1007_1055.json`。

一致性检查全过：JSON 合法；twe 18 行全为规范 6 对 (track,clip)；声学包 6 条全规范（1011/1016/1021/1026/1031/1036）且 (1007,1011) 桥包 orig/hyg 逐字节一致；顶层 summary 槽（loudness/band/stereo=(1007,1011)@08-10、spectrogram=probe_1007@09-05）未被 9/5 事件覆写、原样保留；无引用断裂（停止条件未触发）。

## 3. 离线读路径验证（293 同型，四情景）

Rig：`mixboard.BuildObservation`（读侧全路径：桥判定→`normalizeProjectFeatureMaterialFreshness`→`currentProjectFeatureRowOrMissing`→`promoteBestL3FeatureRows`）× 权威 fixture 快照 + (1007,1011) 桥包 + **265 run 工件捕获的 kernel 工程态**（6 音轨 1007-1032 各 1 规范 clip + 1 服务轨，只读复用）。快照变体：lr 原位（probe_1007，265 型）vs lr 换写为 266 run 真实身份 `mixboard_l2_render_probe_1027_20261106T171511.867643400`（266 型桥脱落）。

命令（agent/ 下，`-count=1`，确定性单跑）：
`go test -overlay D:/Vit_DAW_ws_294hyg/rig/overlay.json ./internal/mixboard -run TestWEHygieneReadPathScenarios -count=1 -v` → **exit 0**（日志+复跑记录 `verification_go_test_log.txt`）

| 情景 | 输入 | WE 视图结果 | 判定 |
|---|---|---|---|
| v1 控制 | 卫生前+lr 原位 | **ready `_1011`**（桥开） | 复现 265 ground truth ✓ |
| v2 控制 | 卫生前+lr 换写 | **missing+`project_material_identity_mismatch` 于 `_1055`** | 复现 266 obs_171506 翻转 ✓（rig 保真度成立） |
| v3 | **卫生后**+lr 原位 | **ready `_1011`**（与 v1 全同） | 稳定 ready 路径保持 ✓ |
| v4 | **卫生后**+lr 换写 | missing 且**无 request/track/clip 身份**（非 mismatch stub） | `_1055` 裸槽浮出路径消灭 ✓ |

v3/v4 在**权威位置换入后的现行字节**上复跑同过（post-swap 变体重建+复测，exit 0）。结论：卫生后 fixture 在 293 同型读路径下 WE 视图稳定——桥开时 ready（v3），桥脱时诚实缺失**不携带失效身份**（v4，272「诚实缺失不升级」语义延续）；裸槽浮出→D1 settle 相位诚实翻转的通路已消除。

## 4. 盘点结论（只记录）

- **本 fixture 其余 1055 引用**：全部位于历史层——observations×4（9/5 当日观察记录）+ `l3_feature_log.jsonl`（18）+ `.vit_history` 会话态×2（1170+4）；`spv1_p01.vit`（9/6 工程态）零 1055；两份 `.bak`（bak_20260905 / bak_20260906_preclip1055）各含 `id="1055"`×1（合法历史边界证物）。历史层按只读纪律未动。
- **同型 fixture `…_02be6193fbc17bed`**：状态层文件（mixboard 快照/声学包/.vit）clip id 全集均在规范 1001-1036 内——**无同型日期遗留**。

## 5. 端测边界声明（零栈）

本卡为零栈离线卡：验证边界=读侧函数离线复放（含双控制情景对 265/266 ground truth 的精确复现）+结构/哈希/manifest 静态核验；**未跑真实栈回归**。266 D1 门 A 真栈回归门维持 274/293 backlog 记法，随后续真栈 run（如 296 D1D3-REGRESS-STACK-1）在卫生后 fixture 上验证。265/266/293 run 工件只读未写回；源码零改动（overlay 注入，worktree 全程 clean）。

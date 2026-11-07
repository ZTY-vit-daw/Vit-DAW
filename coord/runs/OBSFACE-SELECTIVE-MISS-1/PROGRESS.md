# PROGRESS — OBSFACE-SELECTIVE-MISS-1

**终态：DONE（2026-11-07）**——全五步骤完成，取证工件 forensics.md 落盘，卡面回执+提交推送完成（commit fe2c1ed1 领取 / 5dd3aaf2 取证+回执，分支 port/obsface-selective-miss-1 已推 origin）。

定性结论一句话：**缺陷成立但位置改写——主缺陷=agent 侧 d1d3 观察消费面接线错位（顶层读 digest 键实际嵌套 out["digest"]，M1-4 e0e2f4ac 先天，单测形状掩盖），投影结构性恒 missing；建议修（最小修=改读 digest 子图+单测反钉，kernel 面零改动）；「kernel 观察面选择性缺失非确定性」不成立（铸造层 stereo 13/13 ready），waveform 面 D1 settle 相位翻转设计内疑似两案并陈。**

池序 272 / kernel 观察面选择性缺失取证（纯读档零 LLM）/ 工件目录=本目录。

## 步骤清单（对应卡面任务步骤）

1. [x] git fetch origin；建 worktree D:/Vit_DAW_wt_272impl @ port/obsface-selective-miss-1（origin/main）；领取记录回填卡面 todo→doing；逐文件 add 提交推送成功（fe2c1ed1）
2. [x] 三组 digest 对照表（实为 5 采样点矩阵，见 forensics.md §2；另核 run 侧持久化观察包 13+ 时点）
3. [x] 铸造路径勘察（前提修正：铸造在 agent 侧 mixboard 非 VitApp C++，VitApp 零命中；主缺陷=d1d3 投影消费错位，forensics.md §1/§3）
4. [x] 定性（主缺陷成立建议修；waveform 翻转设计内疑似两案并陈；kernel 非确定缺失候选不成立，forensics.md §4）
5. [x] 产出 forensics.md + 卡面回执（回执在 worktree 卡面 doing/2026-11-07-OBSFACE-SELECTIVE-MISS-1.md，随 5dd3aaf2 推送）

## 执行日志

- 2026-11-07 开工：工件目录与 PROGRESS.md 创建；卡面已读（D:/Vit_DAW/coord/cards/todo/2026-11-07-OBSFACE-SELECTIVE-MISS-1.md）。
- 步骤 1 完成：git fetch origin（exit 0，origin/main=37fd7c97）；worktree D:/Vit_DAW_wt_272impl @ port/obsface-selective-miss-1（新建，基线 37fd7c97，status/diff 空）；卡面 todo→doing+领取记录回填；commit fe2c1ed1 推送 origin（exit 0）。
- 步骤 2 进行中（关键中间发现，对照表成型中）：
  - 三组 run 实际含**四个 kernel digest 采样点**：265 reverify1 仅 check 回合（贫）；266v1 LW 门 1（富，17:16:22）+settle 回合（贫，17:17:21）；266v2 probe 回合（富，17:36:32）+check 回合（贫，17:36:50）。
  - 富 digest（v1/v2 两点形态全同）：stereo_status=ready+balance/correlation 在场+三组 rankings 各 6 行+waveform.status=missing（peak/rms/crest null）+available_detail.stereo_relation=ready+missing_metrics=[waveform_envelope,before_after_delta,ab_result]。
  - 贫 digest（三个 check/settle 回合）：无 stereo 三键（stereo_status/correlation/balance 全无）、无 waveform、无 rankings、无 available_detail.stereo_relation（agent 侧 StereoSourceStatus omitempty 缺席反推）。
  - **D1 全局面（waveform peak/rms/crest）在全部四个采样点均 missing**——恒定缺失非漂移；漂移的是 stereo+rankings 面。
  - 参数差异：富点= LW mix.request_observation（v1）/ probe mix.observe 无 disclosure 参数（v2）；贫点= d1d3ObserveArgs 的 `disclosure:"digest_catalog"`+`observation_only:true`（三处一致）。
  - 同 run 内 18 秒一富一贫（v2）⟹ 排除纯缓存时效单因，主嫌疑=kernel 对 disclosure 参数的分支或 session/state 差异。
  - 证据件锚点：265 reverify1/d2_check_chat.json（obs_20261106T152825）；266v1 loudness_watch_observe1.json（obs_20261106T171622）+d1_settle_chat.json（obs_20261106T171721）；266v2 d3_probe_chat.json executed_kernel_reply[0]（obs_20261106T173632）+d3_check_chat.json（obs_20261106T173650）。
- 下一步：步骤 3（kernel C++ mix_observe/digest 组装路径勘察，验证 disclosure 分支假设）。
- 步骤 3 完成（含颠覆性修正）：
  - **「kernel 铸造」前提修正**：VitApp C++ 内无 mix_observe/mix_observation_digest 任何命中（grep 零命中，Source/Bridge/Tests 全查）——digest 铸造全程在 **agent 侧 Go**：harness.requestMixObservation（harness.go:3706）→ mixboard.Store.RequestObservation（mixboard.go:1077）→ buildObservation（mixboard.go:1164）→ BuildDigest（catalog.go:93）。
  - **主缺陷定位（决定性）**：d1d3AssemblePreMasterReport（chat/d1d3_premaster_observation_runtime.go:315-398）从 harness 响应**顶层**读 stereo_status/peak_dbfs/waveform/rankings（:339-369），而 requestMixObservation 的 out map 把 digest 嵌套在 `out["digest"]`（harness.go:3903，M1-4 e0e2f4ac 时已如此=先天错位非回归）；拍平面存在但叫 `out["acoustic_digest"]` 且 stereo 键名不同（stereo_relation_status，harness.go:7305/7377 区段）。⟹ d1d3 投影**结构性恒 missing**，与 digest 实际健康无关。
  - **持久化观察实盘对照（三次 run 五采样点）**：265 reverify1 全 5 点 stereo=ready+waveform=ready（峰值实值在场）含 check 回合（obs_20261106T152825，session cap_...d2_check）；266v1 6 点 stereo 全 ready、waveform 于 17:15:06 项目域翻转 ready→missing（D1 settle 1-mutation 相位内，逐轨域仍 ready）；266v2 9 点 stereo 全 ready、waveform 17:34:00 起项目域 missing。**stereo 面在铸造层 0/13 缺失**——三 run 卡面所称 stereo 缺失全部是 d1d3 投影错位读造成。
  - 相邻发现（不扩查，记锚点）：265 逐轨 stereo mix.read（track.<id>.slow.stereo.summary）7/7 全 missing，而 feature snapshot 内 19 条逐轨 stereo_relation_summary 行全 ready（.vit_agent/.../mixboard/mixboard_feature_snapshot.json）——mix.read 消费路径第二处错位候选。
  - 单测掩盖面：TestD1D3AssembleReportPresentSurfaces（_test.go:234-249）直接喂 digest 形状 map，钉死错误形状假设。
- 下一步：步骤 4 定性收口 + 步骤 5 forensics.md。

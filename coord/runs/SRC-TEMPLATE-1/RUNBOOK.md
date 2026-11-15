# SRC-TEMPLATE-1 真栈腿 RUNBOOK（DG1-B；跑前口径冻结）

- 卡：`coord/cards/doing/2026-11-15-SRC-TEMPLATE-1.md`（池序 373）；关卡契约=[STACKGATE DG1-B](../../docs/STACKGATE_DG1_DG3_V1.md) §3.2（B1-B7）+§1 口径总则。
- 本文件在起栈前成文（AGENTS §8）；执行中不得回改验收标准，只可补记观测。

## 1. 腿形态与装置

- **berth 形态**（STACKGATE §1.1）：`dev_agent_smoke.ps1 -StartKernel -RestartAgent -SrcTemplateSmoke -RunArtifactsDir <run 根>`（不启 UI；B1 打开链走 agent `open_project` tool face=journey1 已验证缝隙，与 Godot 宿主通知同一 agent 侧会话激活链）。
- 驱动体=`scripts/src_template_smoke.py`（B1-B7 断言全在驱动内，summary JSON 逐断言判定）。
- 被测模板=`coord/lesson-designs/music-creation-unit2/templates/melody-drum-v1/template.v1.xml`（place-copy 进 run 目录；sha256 对账 manifest；仓库资产只读）。

## 2. 断言面（B1-B7，逐项与 STACKGATE §3.2 对齐）

| 断言 | 判据（机械） |
|---|---|
| B1 正常装载 | agent `open_project` reply status=ok；`/agent/ui/state` 有 project_uuid 且 5 模板轨名齐（会话激活证据，journey1 同缝） |
| B2 旋律非静音 | `l2_render_probe`(melody, track_post_fader, offline_probe, 0–6.25s) ready + `quality_evidence.nonzero=true` + `max_abs≥0.01` + `nan_inf_count=0` |
| B3 鼓非静音 | 同上（辅助律动轨） |
| B4 改音差分 | 内核 `mutate_midi_notes`（占位 60→79，beat 0，by vit_note_id）readback 命中；`start_render` 全编 WAV 前后对：双双非静音（int16 peak>500 且 RMS>20，309 契约）+ 逐样本 max diff≥500；辅助：旋律轨探针 sum_abs 相对变化记录在案 |
| B5 保存重开 | `save_project`（clear-XML 分支）→ agent open 重开 → 音符回读逐键相等 → 重开探针仍非静音 |
| B6 配置持久 | 重开后 `get_project_state` 5 轨名齐+三轨 Surge XT 插件行在场；保存 XML 含 TIMESIG 2/4+TEMPO 80+`vit_music_key_signature="C"`+channelNumber=10+改后音高 |
| B7 无越权 | agent 日志窗口：admission 相关行摘录在案且零 bypass/forged/unauthorized；零 agent 侧插件控制命令（instantiate_plugin/rack_add_node/set_plugin_param）；模板插件=内核原生装载（不触 agent PCA 门）；Waves 静音族零挂载 |

阈值随 run 工件记录（summary `thresholds` 节），不硬编码凑过。

## 3. §8 跑前口径（概率运行声明）

- **运行次数**：正式 1 次（单 run 无重跑）。装置缺陷轮（语法/断言设计错误）按 309 先例分记为无效轮，修复后新 run；产品断点**禁止**原样重跑。
- **成功条件**：脚本 exit 0=B1-B7 全绿+工件齐（§1.4 工件契约）；一次 exit 0 只证存在成功路径，不宣称稳定率。
- **LLM/生成预算=0**：确定性腿；telemetry 断言 stayed-zero（文件缺省或窗口内 0 行=零计费亲证，309 同款）。
- **失败分类**：exit 2=environment（内核/agent 不可达、Surge 不在装载清单=健康源前置移交）；exit 4=face_defect（B1-B6 断言失败=产品缺陷取证上交）；**B7 违例=立即停+取证上交决策侧，不入重试**（安全边界）。
- **止损线**：同确定性断点连续两败→停手上交；静音且非配置可修=卡面停止条件触发（对齐 309/329 取证链，不硬凑）。

## 4. 占用与隔离（STACKGATE §1.5 / AGENTS §9 §10）

- 栈位：372（GEN-YUE-WIRE-1）真栈腿之后（本卡候第 3）；按 `coord/resources/PC-RUNTIME-STACK.md` 六步：净空亲核→登记+推 main→fetch 重读+≤60s 即时复核→启动→测试→TEARDOWN_ALL_ZERO 亲证释放。
- 隔离：VIT_PROJECT_XML=run 目录隔离副本（脚本共享隔离块）；模板 place-copy 进 run 目录（源哈希 before/after 对账）；recents（`%APPDATA%\Vit-DAW\global_project_config.json`）启动前快照→teardown 哈希亲证还原（RECENTS-HYGIENE-1 b 案）。
- 工件：run 根=`coord/runs/SRC-TEMPLATE-1/run_<时间戳>/`（run_id、UTC 起止、HEAD+status、二进制 sha256、summary JSON、console、探针事件、WAV×2、B7 摘录、telemetry）。

## 5. 零栈段已验证项（真栈前）

- `go build ./...` exit 0；`go test ./... -count=1` exit 0（含 coursetemplate 6 测：装载/校验/state 漂移检出/媒体引用检出/place-copy 字节同/W01 seam 字段面）。
- `scripts/src_template_build.py` 确定性再生（同 sha256）；`scripts/src_template_smoke.py` py_compile+纯函数离线测（wav_diff/日志窗口过滤）过；`dev_agent_smoke.ps1` Parser PARSE_OK（既有腿字节不变）。
- webui 零改动（vitest/构建面不适用，回执声明端测边界）。

## 6. 真栈腿执行记录（跑后回填）

- 待回填：run_id、起止、HEAD、占用/释放登记链接、B1-B7 逐判定、工件路径、计费亲证、异常分记。

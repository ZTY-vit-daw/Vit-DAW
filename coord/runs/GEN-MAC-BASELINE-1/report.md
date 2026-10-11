# GEN-MAC-BASELINE-1：Mac 生成线基线对齐 + 生成链首勘报告

- 执行：GLM-5.3 Mac 执行流（ZCode 会话）/ 2026-10-11
- worktree：`/Users/timozty/Documents/Vit-DAW-genbase1` / 分支 `port/gen-mac-baseline-1` / 领取提交 67a904c9（origin/main=32fcab06）
- 权威 run：`run4-pass-20261011-122948`（脚本 exit 0，30/30 门绿，summary schema `gen.chain.survey.mac.v1`）
- 零生产源码改动：本卡全部提交=`scripts/`（勘测脚本+SMOKE_TESTS 条目）+ `coord/` 工件

---

## 一、四件事交付结论

### ① 同步对齐 ✅

- Mac 仓 fetch 直连（代理 7890 未起，`-c http.https://github.com.proxy=` 覆盖，实测可用）→ origin/main=**32fcab06**（≥卡面要求 e3af3c4e；领取时当日最新）。
- **在飞卡清点（以远端现状为准，卡面"RLM-PROFILE-2 待验收"已过时）**：origin/main `doing/` 仅 **MIXLAYER-M1-1**（PC 侧执行，worktree D:/Vit_DAW_wt_mixlayer_m1_1，文件域=agent/internal/agentloop——与本卡 scripts/+coord 不相交）；`done/` 待验收仅 VIS-FILM-DESIGN-1（零代码设计卡）。本地主工作树（port/rlm-profile-2 分支，陈旧视图）显示的 doing 三卡（CCB-PARAM/L1-3-IMPL-B/MACRO-RECON-1）在远端均已 done，非在飞。
- Mac 遗留 9 个历史 worktree（smoke-mac1/2、journey1、fss1、b2p、d1settle、rlm2-coord、b7b-coord、settle-accept）均挂旧 port 分支，未触碰；本卡独立 worktree 开工（PROTOCOL §3），无 HEAD 踩踏。
- 注意事实：PC 侧 PC-RUNTIME-STACK 显示 MIXLAYER-M1-1 占用 PC 栈（7878/5555/5556）——PC 机器栈与 Mac 栈互不相干，本卡 Mac 端口开工前核对=空闲。

### ② 内核重建 ✅（GEN-CAP-RECON-1 缺口②闭合）

| 产物 | 值 |
|---|---|
| VitApp 内核（Debug, arm64） | `/tmp/vit-genbase1-build/app/VitApp_artefacts/Debug/VitApp`，106,803,416 B，mtime 2026-10-11 11:41:39 |
| 内核 sha256 | `9a70f064d2a20f023f35389c28fa87956595ce46dad4eb35bda571d243f54057` |
| vitagent（go1.24.1 darwin/arm64） | sha256 `d3dc1662c5b9bb1fd3adeb7b6dbe2c078e25c30957f630e059e8ab7bb7d08d17` |
| 构建命令 | `cmake -S VitApp -G "Unix Makefiles" -DCMAKE_BUILD_TYPE=Debug -B /tmp/vit-genbase1-build/app -DVIT_TRACKTION_ENGINE_DIR=<主仓 tracktion checkout>` → `make -j8 VitApp`（exit 0，`error:` 计 0） |
| 先例对齐 | PORT-A4 构建链原样可用；worktree 子模块空 → `--tracktion-dir`/`VIT_TRACKTION_ENGINE_DIR` 指主仓同 pin checkout（b439749，与 gitlink 一致） |
| 代理坑（环境注记） | FetchContent（libzmq/cppzmq）会被 git 全局代理配置打死（7890 未起时 Empty reply）；解法=`GIT_CONFIG_COUNT/KEY/VALUE` 环境变量空覆盖直连，configure 423s 过 |

**缺口①闭合证据**：`git merge-base --is-ancestor` 79b8ecdc（K1 渲染冻结修复）与 9f53bd4e（K2 midi 注册）均为本构建基线祖先；且 79b8ecdc..HEAD 对 `VitApp/Source` 零改动（源码与 PC 绿基线一致）。**Mac 从此有包含 K1/K2 的内核**。

### ③ 生成链五命令真栈首勘 ✅（本卡核心产出）

脚本：`scripts/gen_chain_survey_smoke_mac.sh`（`*_smoke_mac.sh` 体系模式；两件套=内核+agent，状态全隔离 run workdir；`--kernel-bin` 复用重建内核并记 sha256）。逐命令经 `POST /agent/invoke` 发令（needs_confirmation→confirmed:true 双形态），回执 JSON 逐探针落盘。

**五命令（GEN-CAP-RECON §3.2 原单）+ 卡面扩展面的勘测结果**：

| 命令族 | 结果 | 关键证据（run4） |
|---|---|---|
| `project.new` / `project.save_as` / `save` / `clear_project` | ✅ | save_as 落盘 6,810B `.vit` + `_Media/` 目录（媒体池随工程） |
| `midi.import_file`（import_midi_to_track） | ✅ | 3 音符 fixture 导入 note_count=3（clip 1010），回读逐项一致 |
| midi 写族：insert_clip / add_notes / add_notes_bulk / write_clip_notes（混合 op patch）/ mutate_notes / delete_notes | ✅ | 最终回读数学一致：clip1={65@0 v90, 74@2 v55}（transpose+5/区域删/定点 mutate/按 id 删全对）；clip2=5 音符（add 2 + bulk 3） |
| midi 读族：read_clip_notes / read_clip_data | ✅ | 回读含 note id、channel；clip_data 9 键（notes/start_seconds/length_seconds/offset_in_source_seconds/time_unit…） |
| `bridge_ingest_generated_asset`（assets.ingest_generated_asset） | ✅ | 双 ingest（同 take_stack_id）：clip 1016/1018、asset_ref `asset:732810a4…`、take 栈 2 takes 快照、job 关联（register_job 先行） |
| `switch_asset_take` | ✅ | clip2→take1 切换 ok，active_take 回显切换后值 |
| `set_async_ghost_state` | ✅ | clip 级 ghost_state=mute ok |
| `start_render` 腿 B（MIDI-only） | ✅ | **同步拒绝**：status=error reason=no_renderable_audio_content、无 job_id（K1 修复回复语义 Mac 首证）；命令面存活、idle cancel ok |
| `start_render` 腿 A（含真实音频） | ✅ | job_id `79d90c78…`，产物 3,175,984B RIFF wav；完成后台面无冻结（K1 回归核对）；`render.profile.bind`（RLM-PROFILE-2 新面）Mac 首勘 **ok** |
| fail-closed 负探针（ingest 坏路径） | ✅ | status=error 回执（不静默） |
| agent 权威形态 | 记录 | Mac 默认 agent 起动下 **17 个确认卡**（needs_confirmation→confirmed 直过全兼容）；受限权威用户侧确认流不在本卡 |

### ④ MAC-RUNTIME-STACK ✅

`coord/resources/MAC-RUNTIME-STACK.md` 建立（PROTOCOL 资源登记语义，对齐 PC-RUNTIME-STACK 形态）；占用登记（de3a3b6d，登记前核对空闲）→ 释放记录随本回执回填。

---

## 二、Mac 生成线缺口清单（本卡产出）

**已闭合（本卡内）**：
1. ~~缺口① Mac main 缺 K1/K2~~ → 基线 32fcab06 + 重建内核（见上）。
2. ~~缺口② Mac 内核二进制陈旧（9-18 构建）~~ → 新内核 sha256 `9a70f064…`（§9 纪律：哈希+mtime 入 run_meta/summary）。
3. ~~PORT 系列零覆盖面~~（GeneratedAssetService/媒体池/take 栈/AIGC job/midi 写命令/render）→ 全部首勘绿（上表）。

**仍开放的缺口（如实记录，不在本卡修）**：
1. **A1 instrument 装载通道**：结构性缺失（PC 侧 2026-10-06 刹车裁定维持：待用户定形态）。Mac 形态输入（插件库 instrument 占比/候选实列）仍未取证——归建议卡 A1-MAC-FORM-1（GEN-CAP-RECON §4.1 ③）。
2. **argspec 与内核校验漂移（小）**：`mutate_midi_notes` 内核要求每 note 必带非空 id（MidiService.cpp:1125-1144 实测 "each note requires a non-empty id" 拒区域形式），而 catalog.go:540 的 argspec 写 `notes:{id|note_id:string optional …}[]` 暗示 id 可选——LLM 按别名字面发区域形式会被拒。同理 `delete_midi_notes` 内核只收 note_ids（catalog 一致，无漂移）。建议后续小卡对齐 argspec（或内核接受区域形式，二选一，决策侧定）。
3. **note start 的坐标系注记**：add_midi_notes 的 start 为 **clip 相对 beats**（clip2 插在 beat 8，回读 start=0.0/1.0/…），非时间线绝对拍。生成线编排（LLM 产音符序列）时需按此约定。行为本身自洽，仅作消费方约定记录。
4. **SMF chunk id 静默容忍（行为注记）**：JUCE 对坏 chunk id（如 "MTrK" 大写 K）静默跳过该 chunk 且 readFrom 仍 true → 内核回 "MIDI file contains no importable notes"，报错与真因距离远（本卡 run1 排障实录：与 PC 绿基线归档 fixture 逐字节比对才定位）。非缺陷，但生成线若接外部 MIDI 生成器，错误信息有误导性，记录备查。

**探针迭代记录（AGENTS §8 失败分类）**：run1（5 红门）=探针 fixture 缺陷（SMF chunk id 大写 K 笔误→级联 clip_id 空）；run2（2 红门）=mutate 探针参数形态（须 by id）；run3（exit 1 但 28/28 功能门绿）=汇总管道 export 漏。三轮均**脚本缺陷，非栈功能失败**；栈本体自 run1 起除上述探针因素外全绿。迭代证据归档 `probe-iteration-history/`。

## 三、可跑面结论

**Mac 生成线数据级地基（写入+回读+落轨+多候选+渲染）已可跑**：project 全周期、midi 九命令数据级闭环、generated assets 入库/入轨/take 切换/ghost、离线渲染（含 MIDI-only 同步拒绝语义）在 Mac 真栈 30/30 门 exit 0。**AIGC-DRUM-GEN-1 的"写入+回读"腿在 Mac 具备开跑条件**；出声/试听腿仍挂 A1（instrument 装载，两端同缺）。文件级多候选（take 栈）与音符数据级候选两形态在 Mac 均已实证可用。

## 四、工件索引

- 权威 run：`coord/runs/GEN-MAC-BASELINE-1/run4-pass-20261011-122948/`（summary.json 30 门、http/ 31 份逐探针回执含 17 确认卡、logs/ 内核+agent 全量日志、fixtures/ 三份 fixture 与渲染产物、project_state/ save_as 工程与 _Media、run_meta + 双二进制 sha256/mtime、lsof 端口属主）
- 探针迭代：`coord/runs/GEN-MAC-BASELINE-1/probe-iteration-history/`（run1/run2/run3 的 summary+gate_failures）
- 内核构建日志：`/tmp/vit-genbase1-build/{cmake-configure.log,make-vitapp.log}`（临时目录，run_meta 记录了二进制哈希可复验）
- 占用记录：`coord/resources/MAC-RUNTIME-STACK.md`
- 端测覆盖边界声明（AGENTS §5）：本卡验证面=Mac 两件套真栈（内核+agent）命令/回执面；**未含** Godot 前端、webui 渲染与用户旅程面（本卡无相关改动）；插件扫描/Waves 面不在本卡（PORT-A5/C3 已覆盖）。

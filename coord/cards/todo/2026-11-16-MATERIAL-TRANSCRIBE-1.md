# MATERIAL-TRANSCRIBE-1：教材谱例材料转写流（约 20 条 planned_transcription）

- 发卡：主管 2026-11-16 午（承接 gate-morning 候派发项；免费车道/量产分批）
- 池序 380；P2；依赖=无（材料图已缓存于 source/pages/）；零栈（纯资产生产，不启真实栈）
- 范围权威：`coord/lesson-designs/music-creation-unit2/assets/catalog.yaml`（22 项中 `status: planned_transcription` 且 `file: null` 的全部条目）+ [TEXTBOOK-ALIGNMENT §材料](../../lesson-designs/music-creation-unit2/TEXTBOOK-ALIGNMENT-AND-MATERIALS.md)。

## 目标（逐条流水，每条独立交付）

对每条 planned_transcription 材料（如 pitch-repeat「我向党来唱支歌」pdf-022）：

1. **核小节**：读 `source_image` 谱面图，核定转写小节范围（excerpt 短句对应段），回填 catalog 行 `excerpt`（从"小节范围待核"改为实际范围，如"第 1-8 小节（含同音反复标注段）"）。
2. **转写**：对照原谱转写音高、节奏、休止、调号/拍号 → MIDI 文件落 `assets/midi/<材料id>.mid`；catalog 回填 `file: midi/<材料id>.mid`。
   - 未给 BPM/调号处：演示设定单独记录（新列或 verification 记录，**不改原谱事实**）；
   - **诚实边界**：图中辨不清的音符标 `uncertain` 清单入 verification，不猜不编。
3. **回读核对**：用现成工具（仓库内 MIDI 解析/373 血统工具；无则写最小脚本）回读 MIDI，核对音高序列/时值/拍号与转写意图一致；每条留回读记录。
4. **status 更新**：转写+回读完成 → `status: transcribed_unverified`。**听核未做不得写 checked**（catalog 头注"不可假装加载完成"）；用户/主管听核后另行升级。
5. 批次工件：`coord/runs/MATERIAL-TRANSCRIBE-1/batch<N>/` 留每条核对记录 + catalog diff。

## 边界

- 只改 `coord/lesson-designs/music-creation-unit2/assets/`（catalog.yaml、midi/ 新增）与 run 工件目录；不动生产代码、不动 source/pages 缓存图。
- 不把 status=generated_symbolic_checked 的既有两项改状态；不新增材料条目（替代/对照曲目录另卡）。
- 分批执行：建议每批 5 条，批间可中断续跑（catalog 行级幂等：已 transcribed_unverified 的不重做）。

## 验收

全部条目 status 到 transcribed_unverified + MIDI 文件在 + 回读记录在卷；主管抽样 3 条亲证（看谱面对照 + 回读记录 + catalog diff 一致）；uncertain 清单如实（有则接受、无则确认零）。

## 领取/回执/验收：待回填

## 领取/回执/验收

### 领取（闲时任务通道，2026-11-16 深夜）

- 派发：主管决策流（freeflash 供应商当晚不可用，改走 OffPeak 闲时通道执行批 1）；零栈零 LLM。
- 范围：批 1 = catalog 前 5 条 planned_transcription（pitch-repeat/step-up/step-down/small-leap/large-leap）。

### 回执（批 1 完成，执行侧自验）

- **5 条全部 transcribed_unverified + 回读核对 ALL PASS**：SMF format 0/单轨/480 PPQ（随仓库 seed-midi-manifest 惯例，卡面工作分派所述 96 PPQ 按仓库惯例改为 480——如实记录）。
- 逐条（谱面=source/pages/ 用户教材页缓存，转写范围=教材印刷摘录小节，不超出图面所示）：
  1. pitch-repeat 我向党来唱支歌（1=F 2/4，P17 同音反复标注句 4 小节，96bpm）→ `assets/midi/pitch-repeat_annotated-phrase_F_96bpm.mid`（15 音）
  2. pitch-step-up 哆来咪（1=C 2/4，P18 三段递进句 12 小节，96bpm）→ `assets/midi/pitch-step-up_textbook-excerpt_C_96bpm.mid`（24 音）
  3. pitch-step-down 小白菜（1=G 5/4，P19 例 4 小节，60bpm）→ `assets/midi/pitch-step-down_first4_G_60bpm.mid`（18 音）
  4. pitch-small-leap 共产儿童团歌（1=F 4/4，P19 例 4 小节，100bpm）→ `assets/midi/pitch-small-leap_first4_F_100bpm.mid`（20 音）
  5. pitch-large-leap 东方之珠（1=D 4/4，P20 摘引 弱起+4 小节，72bpm）→ `assets/midi/pitch-large-leap_excerpt_D_72bpm.mid`（24 音）
- 回读核对：音符数/音高序列/时值序列/拍号/速度五面逐条 PASS（机制在卷：首跑即抓出 SMF 拍号 meta 分母指数写法 bug——回读 ts=2/16 暴露，修复后 5/5 PASS，非重跑凑过）。
- uncertain 清单：逐条 2-4 项记录于 `coord/runs/MATERIAL-TRANSCRIBE-1/batch1/verification_batch1.json`（诚实边界重点：pitch-step-up 三段句低八度点按平行结构推断；pitch-large-leap 整条扫描分辨率最低、八度点/三连音标记候高分辨率图像复核）。
- 工件：`coord/runs/MATERIAL-TRANSCRIBE-1/batch1/`（transcribe_batch1.py 生成器+verification_batch1.json+catalog_before 快照+catalog_diff patch）；catalog 5 行已回填（excerpt 实际范围+file+status）。
- 边界遵守：零栈零 LLM；只改 assets/ 与 run 工件；source/pages 缓存图零触碰；未动生产代码/POOL-INDEX；歌词不入 MIDI（教学概念演示用途，音高/时值数据面）。
- **剩余批次**：批 2-4（15 条 rhythm-*/contour-* 等）候续派（catalog 行级幂等，已完成行自动跳过）。

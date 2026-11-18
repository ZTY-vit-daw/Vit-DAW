# MATERIAL-TRANSCRIBE-2 / batch4 工件说明

- 卡：coord/cards/doing/2026-11-18-MATERIAL-TRANSCRIBE-2.md，批次 4（5 条）
- 来源页：P29 pdf-034（编创实践：即兴一~四）、P30 pdf-035（三十里铺·陕西民歌，前两句 8 小节）
- 生成器：transcribe_batch4.py（SMF format 0/480PPQ + 独立回读 + verification_batch4.json）
- 读谱证据：crops_p34/（dev_proto / dev_ornament / dev_rhythm / dev_augment / dev_shift 单例横向放大；
  dev_shift_b34_hi 3.4x）、crops_p35/（sslp_S1/S2/S3 系统条、sslp_S1a_hi / sslp_S1b_hi / sslp_bars23_hi 放大）
- 命令与退出码：python transcribe_batch4.py → exit 0，5/5 PASS（batch4_run_log.txt）
- 目录：catalog_before_batch4.yaml / catalog_after_batch4.yaml / catalog_diff_batch4.patch

## 逐条

| id | 文件 | 拍号/演示BPM | 音符数 | 回读 | 摘录范围 |
|---|---|---|---|---|---|
| develop-ornament | assets/midi/develop-ornament_jiaxing1_C_80bpm.mid | 4/4, 80, ch1 | 11 | PASS | P29 即兴一 2 小节 |
| develop-rhythm | assets/midi/develop-rhythm_change1_C_80bpm.mid | 4/4, 80, ch1 | 9 | PASS | P29 即兴二 2 小节 |
| develop-augment-meter | assets/midi/develop-augment-meter_3-4_C_80bpm.mid | 3/4, 80, ch1 | 9 | PASS | P29 即兴三 7 小节（3/4） |
| develop-shift-rhythm | assets/midi/develop-shift-rhythm_yiwei1_C_80bpm.mid | 4/4, 80, ch1 | 15 | PASS | P29 即兴四 4 小节 |
| reference-sanshilipu-melody | assets/midi/reference-sanshilipu-melody_first8_C_80bpm.mid | 2/4, 80, ch1 | 22 | PASS | P30 前两句 8 小节 |

- 全部 status=transcribed_unverified（听核未做）；uncertain 逐条见 verification_batch4.json。
- 与既有 checked 素材的关系：即兴一~四以 develop-prototype（C 调/80BPM）为原型配对；三十里铺旋律与
  既有 checked「sanshilipu-first8_rhythm-only」的 22 音节奏模板逐音对应（第一句 11 音 ×2）。
- 边界：未改 source/pages、生产代码、既有 checked 两项、POOL-INDEX；零真栈零 LLM。

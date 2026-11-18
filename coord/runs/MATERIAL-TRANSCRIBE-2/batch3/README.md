# MATERIAL-TRANSCRIBE-2 / batch3 工件说明

- 卡：coord/cards/doing/2026-11-18-MATERIAL-TRANSCRIBE-2.md，批次 3（5 条）
- 来源页：P22 pdf-027（平直线·乘胜追击）、P25 pdf-030（平缓进行·早晨的问候）、
  P26 pdf-031（跌宕起伏·海港之夜，取第一行）、P27 pdf-032（创作示例·怀念战友，取首句 4 小节）、
  P28 pdf-033（小草，取第一行 8 小节）
- 生成器：transcribe_batch3.py（SMF format 0/480PPQ + 独立回读 + verification_batch3.json）
- 读谱证据：crops_p25/（早晨的问候单小节/分段 4.6x-8x 放大）、crops_p26/（海港之夜第一行两半）、
  crops_p27/、crops_p32/（怀念战友 4.7x/4.3x 放大）、crops_p28/（小草单小节 9x 放大）；
  symbols_p25_smooth.json / symbols_p28_grass.json / symbols_p32_huai.json（连通域+减时线/点符像素分析）
- 命令与退出码：python transcribe_batch3.py → exit 0，5/5 PASS（batch3_run_log.txt）
- 目录：catalog_before_batch3.yaml / catalog_after_batch3.yaml / catalog_diff_batch3.patch（快照间 diff）

## 逐条

| id | 文件 | 拍号/演示BPM | 音符数 | 回读 | 摘录范围 |
|---|---|---|---|---|---|
| contour-flat | assets/midi/contour-flat_chengsheng-first8_Bb_120bpm.mid | 2/4, 120, ch1 | 24 | PASS | P22 图面 8 小节 |
| contour-smooth | assets/midi/contour-smooth_morning-first4_C_72bpm.mid | 3/4, 72, ch1 | 18 | PASS | P25 弱起 2 十六分 + 4 小节 |
| contour-wave | assets/midi/contour-wave_seaport-first4_Bb_60bpm.mid | 4/4, 60, ch1 | 22 | PASS | P26 弱起 + 第一行 4 小节 |
| analysis-huainian | assets/midi/analysis-huainian_first4_Bb_72bpm.mid | 2/4, 72, ch1 | 15 | PASS | P27 首句 4 小节 |
| analysis-xiaocao | assets/midi/analysis-xiaocao_first8_G_80bpm.mid | 2/4, 80, ch1 | 22 | PASS | P28 第一行 8 小节 |

- 全部 status=transcribed_unverified（听核未做）；uncertain 逐条见 verification_batch3.json。
- 边界：未改 source/pages、生产代码、既有 checked 两项、POOL-INDEX；零真栈零 LLM。

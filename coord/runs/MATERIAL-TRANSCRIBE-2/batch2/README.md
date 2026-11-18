# MATERIAL-TRANSCRIBE-2 / batch2 工件说明

- 卡：coord/cards/doing/2026-11-18-MATERIAL-TRANSCRIBE-2.md
- 批次：2（5 条，卡面每批 ≤5 条）
- 来源页：P21/p4-026（节奏四例）、P22/pdf-027（自由曲线·二月里来）
- 生成器：transcribe_batch2.py（写 SMF format 0/480PPQ + 独立回读核对 + verification_batch2.json）
- 读谱证据：crops_p26/（节奏行像素级条带、单小节 6x 放大）、crops_p27/（二月里来单小节 6x 放大）、
  rhythm_symbols_p26.json（连通域+减时线/点符像素分析）、melody_symbols_p27b.json（数字/减时线/八度点/装饰音连通域分析）
- 运行命令与退出码：python transcribe_batch2.py → exit 0，5/5 PASS（见 batch2_run_log.txt）
- 目录：catalog_before_batch2.yaml（批前快照）、catalog_after_batch2.yaml（批后快照）、catalog_diff_batch2.patch（git diff）

## 逐条（readback 五面：音符数/音高序列/时值序列/拍号/速度 + 小节总长）

| id | 文件 | 拍号/演示BPM/通道 | 音符数 | 回读 | uncertain |
|---|---|---|---|---|---|
| rhythm-even | assets/midi/rhythm-even_uniform-4bars_GM37_80bpm.mid | 4/4, 80, GM37 ch10 | 15 | PASS | 第5小节三连音例未选入；X谱无音高→GM37 演示设定 |
| rhythm-long-short | assets/midi/rhythm-long-short_4bars_GM37_80bpm.mid | 4/4, 80, GM37 ch10 | 25 | PASS | X谱无音高；弧线按圆滑线；密集十六分候复核 |
| rhythm-short-long | assets/midi/rhythm-short-long_4bars_GM37_80bpm.mid | 4/4, 80, GM37 ch10 | 23 | PASS | X谱无音高；附点四分/附点八分判定候复核 |
| rhythm-syncopation | assets/midi/rhythm-syncopation_4bars_GM37_80bpm.mid | 4/4, 80, GM37 ch10 | 29 | PASS | X谱无音高；跨小节弧线按圆滑线未合并（若延音线则不同） |
| contour-free | assets/midi/contour-free_february-first4_Ab_72bpm.mid | 4/4, 72, ch1 | 30 | PASS | 装饰音按32分实现；低八度点按批1惯例；2.3 附点八分读法候复核 |

- 完整 uncertain 见 verification_batch2.json 各项 uncertain 数组。
- 转写范围与图面一致：P21 四例取图面全部小节（rhythm-even 取第 1-4 小节，避开第 5 小节三连音例）；
  P22 二月里来取图面 4 小节（“二月里来呀好春光，家家户户种田忙”）。
- 诚实边界：全部 status=transcribed_unverified（听核未做）；不冒充 checked/可加载；未改 source/pages、生产代码、既有 checked 两项、POOL-INDEX；零真栈零 LLM。

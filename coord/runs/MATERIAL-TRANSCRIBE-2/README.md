# MATERIAL-TRANSCRIBE-2 运行工件总览

- 卡：coord/cards/doing/2026-11-18-MATERIAL-TRANSCRIBE-2.md（批 2-4，剩余 15 条 planned_transcription）
- owner：lead/materials420（DeepSeek 执行流）；worktree C:/Users/timoz/.codex/worktrees/pc420-materials/Vit_DAW；
  分支 codex/material-transcribe-420；基线 origin/main；零真栈零 LLM。
- 结果：15 条全部推进到 status=transcribed_unverified，MIDI 落 coord/lesson-designs/music-creation-unit2/assets/midi/，
  每条独立回读 PASS；catalog 中 planned_transcription 归零（22 条 = 20 transcribed_unverified + 2 既有 checked）。
- 分批：
  - batch2（5 条）：rhythm-even / rhythm-long-short / rhythm-short-long / rhythm-syncopation / contour-free
  - batch3（5 条）：contour-flat / contour-smooth / contour-wave / analysis-huainian / analysis-xiaocao
  - batch4（5 条）：develop-ornament / develop-rhythm / develop-augment-meter / develop-shift-rhythm / reference-sanshilipu-melody
- 每批目录内容：transcribe_batch<N>.py（生成器+回读核对）、verification_batch<N>.json（回读记录+uncertain）、
  catalog_before/after 快照、catalog_diff patch、batch<N>_run_log.txt、README.md、crops_*（读谱放大证据）、
  symbols_*.json（连通域/减时线/点符像素分析）、tools/（裁剪与像素分析脚本）。
- 验证：python transcribe_batch2.py / 3 / 4 全部 exit 0、15/15 PASS；tools/validate_catalog.py 检查 22 条、
  0 planned、file 全在场；sha256 在 verification JSON 与 MIDI 实件已复核一致。
- 诚实边界：全部 transcribed_unverified（未做听核/对谱正式复核）；uncertain 逐条在 verification JSON；
  未改 source/pages、生产代码、既有 checked 两项、POOL-INDEX、其他卡片。

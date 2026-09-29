# PILOT p01 真栈轮（确定性失败取证）
- 命令：powershell -NoProfile -ExecutionPolicy Bypass -File scripts/run_free_state_d1_smoke.ps1 -RepoRoot "D:/Vit_DAW" -PublicManifest "C:/Users/timoz/Documents/毕业设计/experiments/out/suite_v1_lm_runner/fixture_manifest.json" -PublicCaseId pv1_p07 -PromptFlavor neutral
- 退出码：1（ps1 throw "D1-S1 smoke failed"）
- 失败点：qualify_material 材料合格门（LLM 前，responses=[]，零概率性成分）
- 错误：public material failed the compression-fixture qualification gates: {"weak_rms_tracks": ["guitar"], "weak_crest_tracks": [], "best_crest_db": 27.192}
- 原始工件目录：D:\Vit_DAW\artifacts\free_state_d1_s1\20260929_194244\（未覆盖）
- HEAD=e8955ed7（代码=tag experiment-baseline-2026-09-29）；无 -SkipBuild（ps1 内置增量重建）；无特殊 env（VIT_AGENT_DOMAIN_ROUTES 未设）
- VitApp.exe SHA256=78DC4889921204472DDCA365FFA1915287EF8C5D69BD7BA8C91C68463AD0631F（19:37:42 构建）

# YUE2-PC-DEPLOY-1 run1 回执（2026-10-19 20:16–20:38，用户在场直接执行）

## 结论：本机冒烟全部通过，部署成功

引擎：yue-server.exe（yue2.cpp c8516db，2026-10-09 构建），来源 YuE2-Studio v3.6.0 portable（zip 354,234,123 B）。
模型：YuE2-3B-Q8_0.gguf（SHA256 41121ce9…，与方案附录 A 一致）+ YuE2-Vae-F32.gguf（93e49dfb…，一致）。
环境：RTX 4070 12GB / 驱动 617.42 / 基线显存 1250 MiB / HEAD 81ef05c5 / 工作树含既有改动（Settings.xml、default_project.xml，与本部署无关）。
引擎 SHA256：6db1bd3ee9f8849f698f5636bf9e89a62d6d3ee1d145e20e819221ed0ed78a3a。

## 与方案的偏差（四项，已核实）

1. **`--device CUDA0` 参数不存在**于 c8516db 构建（引擎打印 usage 退出）。设备选择机制 = ggml 扫描 exe 同目录后端 DLL；已将 `cuda13\ggml-cuda.dll` 拷至 engine 根目录，引擎自动加载 CUDA 后端（自检 94ms，对比 Vulkan 4108ms）。cudart64_13 疑点解除：cuda13 构建静态链接，仅需 cublas64_13/cublasLt64_13（已就位）。
2. dsh 指令两处路径错误已在执行前修正（studio zip 无版本名中间层；cublas 实际在 libcublas-…-archive\bin\x64\）。
3. job id 为十六进制字符串（如 e215a29a16796d8f），非方案假设的数字；multipart boundary=yue2-batch-boundary，含 application/json replay + audio/wav。
4. 中文歌词必须走 UTF-8 文件 + `curl --data-binary @file`，命令行内联 JSON 会被引擎报 invalid JSON。

## 冒烟结果（5 腿全过）

| 腿 | 请求 | 实际音频 | 墙钟 | WAV SHA256（前8） |
|---|---|---|---|---|
| smoke1 | 60s seed1234 | 55.68s / 48kHz / 2ch | ~40s | 0a8ec847 |
| smoke2 | 60s seed1234 重跑 | 29.72s | 16s | 9745a301 |
| smoke3 | 60s seed5678 | 60.00s | 34s | ae92cba0 |
| smoke4 | 180s seed777 富歌词 | 99.64s | 40s | d81b221f |
| smoke5a/b | smoke3 的 semantic_tokens 重放 ×2 | 60.00s | 12s | 842161f5（两次相同）|

ffprobe 全部 pcm_s16le/48000/2ch ✓。生成速度 ≈ 0.55× 实时（60s 歌 34s）。重放渲染 12s（跳过 AR 阶段）。

## 关键行为声明（如实）

- **duration 是上限不是保证**：模型自行收尾（60→29.7s、180→99.6s 两例）。replay JSON 无 truncated 字段。
- **同 seed 重跑生成不逐字节确定**（CUDA 数值非确定性 + 采样混沌，smoke1≠smoke2）。**semantic_tokens 重放路径逐字节确定**（smoke5a==smoke5b）——A/B 与回归实验用重放，不用重跑。
- **显存**：峰值 4629 MiB（含桌面基线 1250，引擎自身 ≈3.4GB）；任务间默认卸载模型，用后回落 ~1430 MiB（--keep-loaded 才常驻）。与 Godot 开发共存无压力。
- 引擎绑 127.0.0.1:8087，无鉴权，未暴露局域网。

## 工件

- 音频+replay：E:\AI\YuE2\outputs\（smoke1–5 wav/replay；不入仓库）
- 本目录：env.txt、vram_samples.log、smoke*_req.json / headers / replay.json、engine_hash.txt
- 启停脚本：E:\AI\YuE2\start_engine.ps1 / stop_engine.ps1（仓库 scripts/ 版留给 S6 立卡）
- 引擎日志：E:\AI\YuE2\logs\engine.log

## 未做与边界

- 人耳听感未评估（wav 已就位待用户试听，WebUI http://127.0.0.1:8087/）。
- Mac 端到端（S4 shim + genapi）未开始，防火墙未动。
- 权重 CC BY-NC 4.0：仅非商用研发实验，不入仓不入交付物。
- 引擎为社区第三方实现（MIT），非官方 PyTorch 栈。

## 事件追加（2026-10-19 20:40–20:44）：WebUI 长时长请求失败 → 已修复

- 现象：用户 WebUI 连续生成多首成功后，一次请求显示生成失败。引擎未崩溃（/health ok）。
- 根因：该请求 duration=360s（WebUI 默认值即 360），超出 `--max-seq 8192` 的预算上限——引擎日志 `Pipeline FATAL: prompt+score 1703/8192 tokens, a song can last 260 s and 360 s were asked for`。属参数约束下的正确拒绝，非故障。
- 修复：实测 KV 仅 476MB@8192（4D batched），外推满上下文 ~1.4GB，远低于方案文档预估；重启引擎改 `--max-seq 24576`（模型满上下文）。start_engine.ps1 已同步更新，且改用 Start-Process 拉起（进程脱离会话存活）。
- 验证：同 style（acoustic piano, lofi pop, jazz, trap）+ duration=360 重发 → done，127.7s/48kHz/2ch WAV，49s 墙钟，全程峰值显存 6340 MiB（含桌面基线）。360s 请求不再被拒。
- 行为注记：放宽后模型仍倾向提前收尾（实测最长 127.7s）。song 长度上限由模型 EOS 决定，duration 仍是天花板不是保证；更长歌曲需调采样参数探索，属实验问题非部署问题。
- 新增工件：verify360_req.json / headers / replay.json、vram_360s.log、logs/engine_8192_run1.log（8192 时期完整日志）。

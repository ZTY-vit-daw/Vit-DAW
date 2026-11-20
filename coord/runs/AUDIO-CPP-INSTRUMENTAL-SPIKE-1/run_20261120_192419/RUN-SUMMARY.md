# RUN-SUMMARY — AUDIO-CPP-INSTRUMENTAL-SPIKE-1 步①（audiocpp439，2026-11-20）

- 引擎：audio.cpp v0.9.5（git 61c7838b 2026-11-14，msvc Release，backends cpu,cuda），`E:/AI/audio-cpp/engine/audiocpp_cli.exe`。
- 模型：`E:/AI/audio-cpp/models`（yue2-3b-q8_0.gguf + yue2-vae-f32.gguf + sidecars/4 + lora/ar_lora_inst_v3abc.safetensors）。
- 生成：恰 1 次，**exit 0**；seed=1234，cot=full，semantic_max_tokens=1500（min 默认 200），AR LoRA 挂载引擎亲证 `projections=196 scale=1.000000`。
- 产物：`yue2_instrumental_seed1234.wav` 11,519,788 字节，**audio_duration_ms=59998.7（60.0s）**，48kHz 双声道，pcm16；引擎另报内部 ABC score 1229B（未单独落盘）。
- 性能：wall 77.0s，RTF 1.28345；**VRAM 采样峰值 6404 MiB**（56 样本 @2s；峰值时刻 19:25:55，AR→NAR 段）；引擎自报 peak_rss 7312MB（宿主）。
- 完整性：三件 HF 文件本地 SHA256 与官方 LFS 指针逐一匹配（见 `sha256_local_computed.txt`；对照值：q8 `52be345b…591` / f32 `cf3b71c6…546` / lora `2252d707…a96`）；两个引擎 zip 字节数与 GitHub 官方资产精确一致（469,569,407 / 607,273,675），官方未发布 SHA，本地计算值留档。
- 来源（用户终端手动下载，lead 派发变更 2026-11-20 19:0x）：
  - https://github.com/0xShug0/audio.cpp/releases/download/v0.9.5/audio-v0.9.5-bin-windows-x64-cuda12.4.zip
  - https://github.com/0xShug0/audio.cpp/releases/download/v0.9.5/audio-v0.9.5-cudart-windows-x64-cuda12.4.zip
  - https://huggingface.co/audio-cpp/Yue2-3B-GGUF（q8_0 主模型 + f32 VAE + sidecars）
  - https://huggingface.co/Mothersuperior/YuE2-instrumental-cot-full-loras（ar_lora_inst_v3abc.safetensors，CC BY-NC 4.0）
- 栈纪律：跑前 GPU 空闲（基线 1648MiB）；Vit 系进程 before/after 均 0（零变化亲证）；引擎进程 teardown 后 0；VRAM 回落 1754MiB。
- 许可注意：LoRA 与 base 均为 CC BY-NC 4.0（非商用；毕设/展演可用）。
- 边界：本轮未挂 NAR LoRA（stock NAR decoder，按 lead 派发口径）；人声判定不在执行侧——lead 组织用户人耳终审。
- 工件：`command.txt`、`engine_stdout.log`（完整 TRACE/TIMING/metrics）、`engine_stderr.log`、`vram_before/after.txt`、`vram_samples.csv`、`vit_processes_before/after.txt`、`sha256_local_computed.txt`、`refs_yue2_docs_v095.md`。WAV 留工作树工件目录未纳入 git 提交（按 §12 工件不自动入版本）。

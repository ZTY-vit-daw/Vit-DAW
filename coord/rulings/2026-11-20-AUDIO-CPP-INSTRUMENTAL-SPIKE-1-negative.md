# 裁定：AUDIO-CPP-INSTRUMENTAL-SPIKE-1(439) 负结果终局收口

- 日期：2026-11-20 20:20 +0800
- 卡：439 AUDIO-CPP-INSTRUMENTAL-SPIKE-1
- 裁定：**整卡负结果收口（步②不触发，不接 genapi）**；执行面零缺陷（两轮生成均 exit 0、栈纪律全合规）。

## 终局事实（用户人耳终审×2）

- Q8+AR LoRA（run_20261120_192419）：**成段人声**。
- BF16+AR LoRA（run_20261120_195455，单变量隔离——命令唯一差异=基座文件，同 seed/style/LoRA/时长，双 WAV SHA 相异证基座差异真实进入渲染）：**成段人声**。

## 四路证伪汇总（YuE2 本地 instrumental 线全谱）

| 路径 | 卡 | 结果 |
|---|---|---|
| style 英文词 `instrumental` | 381 | 成段人声 |
| 官方配方 conditioning（负条件 style+cot+空歌词+Vocal 休止） | 390 | 成段人声（合唱） |
| audio.cpp AR LoRA @ Q8 基座 | 439① | 成段人声 |
| audio.cpp AR LoRA @ BF16 基座（量化假设排除） | 439①-B | 成段人声 |

结论：**YuE2 家族在本地可部署引擎（yue2.cpp/audio.cpp）上无法产出可靠纯器乐内容**；概率性偶然成功不可承诺（§8 口径），此线关闭。

## 残余未证面（如实登记，不再投入）

- Mothersuperior 仓 `scripts/ar_generate.py` 参考脚本与其 README 细则（lora scale/lyrics 格式变体）未逐项对拍——但 audio.cpp 官方文档口径已按其执行，且四路一致证伪，边际收益低。
- 「joint_v1 NAR LoRA」存在性未落实（HF 仓文件列表仅 AR 件）；NAR 系音色层，AR 仍在写人声时 NAR LoRA 难以翻盘。
- 换 seed 抽奖式搜索违反概率运行纪律，不做。

## 处置

- 步②（genapi `audiocpp_local` 接入）不触发；audio.cpp 部署保留（`E:\AI\audio-cpp\`，含 BF16/VAE/LoRA 全套，磁盘 ~11GB——保留候未来上游演进复用，不占栈）。
- 产品 instrumental 口径维持 390 fallback 终态（带人声+披露）；确定性素材=MIDI clip 路线（438 v2 已落地）。
- 439 done（负结果收口）；上游若出现新的 instrumental 机制（新 adapter/新引擎/官方栈门槛变化）另开卡，须新证据解锁。

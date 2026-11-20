# 裁定：YUE-INSTRUMENTAL-PATH-1(390) 真栈验证=未接通，fallback 口径收口

- 日期：2026-11-20 19:05 +0800
- 卡：390 YUE-INSTRUMENTAL-PATH-1
- 裁定：**整卡收口（技术接入面 pass + 声学面负结果 fallback）**，用户听核 2026-11-20。

## 听核事实（用户人耳终审）

- v1 WAV（60.0s 完整渲染，新配方全条件）`run_20261120_1832/downloads/yue-local-1526daa82c0c8478.wav`：**成段人声（用户判定：直接是合唱）** → 卡面预设三档口径判「未接通」。
- v2（14.9s 短输出）未听核（预算 2 次已尽，v1 已足以定性）。

## 技术面结论（两腿 pass，非执行失败）

- 勘察腿+接入腿（官方 instrumental 配方条件化）零栈 PASS 在卷；真栈 dispatched 四件套证成（style 负条件/cot=melody/空歌词等价/Vocal 休止+W3 逐音对账）。
- **声学结论：官方 style 负条件与 cot 推导在本地 yue2.cpp 量化引擎上无人声抑制效果**——与 381「英文词 instrumental 无抑制」同一结论在新配方下的复现与强化。归属=引擎侧渲染先验（不可经 /synth 配置面干预），非封装层缺陷。

## 产品口径（回退即终态）

- 381 的「能力边界披露」口径**维持不升级**：instrumental 请求=带人声渲染+Limitations 披露（S6 文案 `engine-side muting not guaranteed` 回归断言在卷，无需改动）。
- 展演生成线如需纯器乐：教学素材按 2026-11-20 用户裁定走 **MIDI clip 装载**（438 v2 方向）；YuE 不用于确定性素材。

## 边界

- 单次听核样本（预算 1-2 次内 1 次有效定性）；未穷尽 seed/采样面（引擎内置不可配，RECON 差距#6 在卷）。
- 装置债两笔亲证入卷：save_as prepend recents（RECENTS-HYGIENE-1 面）+default_project 内核改写；vram 采样器峰值 0 缺陷。

## 后续

- 390 done（负结果定性收口）；引擎侧若未来有 instrumental 支持（上游更新/新引擎）另开卡，不占当前池。

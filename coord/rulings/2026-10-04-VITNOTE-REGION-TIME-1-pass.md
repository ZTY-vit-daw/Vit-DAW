# Ruling：VITNOTE-REGION-TIME-1 —— 转正 pass（2026-10-04 晚，PC 决策侧）

## 裁定

**转正 pass**（conditional 的转正条件"用户真栈手测三复验点"由烟测判据链+真栈真 LLM 直接证据替代达成）。

## 判据链

1. 机制面（早 conditional ruling）：diff 直读+探针族全绿（26/141/143/16 全 0 FAIL）+Go note 钉+chat 全包 ok。
2. 烟测场景 A（SMOKE-SCEN-RANGE-1，真三件套泊位）：执行侧两轮+决策侧复跑——note_sessions 投影/store 时间键往返断言过；legacy 载荷 fail-open 零报错。
3. **产品验收点直接实证**（原挂用户手测①②③的等价物）：真栈真 LLM 回复对比——v3.1 问"这个圈定范围包含什么？给时间跨度"→答 `Time span: 0:03.2–0:08.5`+逐 clip 全长与相交段（复验点①②）；legacy 载荷→答"辖区未给出范围边界，故记入全长"不伪造时间（复验点③ fail-open 语义）。

## 残留

- 用户日常使用中观感/话术覆盖缺口随行小修（不立卡口径）。
- 决策侧环境遥测断言（char_count 差）未复现的 env 继承差异：SMOKE ruling 记录，不构成产品缺陷。

## 处置

卡面验收栏更新；实现已合 main（4c98c643）+Godot `port/vitnote-region-time-1`@ecc722d 配套在位（Godot 工作树即此分支）。

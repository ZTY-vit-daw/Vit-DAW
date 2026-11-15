# 谱面工作台记谱推导规则（v1）——内核平音符 → 记谱符号的确定性转换

- 状态：现行（SCORE-WORKBENCH-1 M1 产出，369 选型定案承接；W18 契约基础）。
- 依据：G0-SCORE-SPIKE-1 规则 v0（27 断言亲证）+ 教材语汇精化（拍内附点归并/弱起延音链/休止填充表）。
- 范围：节奏/旋律模式编辑（16 分网格）；单位约定见 §0。本文件是可审查契约（风险 R7）：实现
  `derive.ts` 与本文件一一对应，改规则先改本文件再改代码，测试锚定本文件编号。

## 0. 单位与模型形状

- 网格分辨率：`GRID_PER_BEAT = 4`（16 分音符网格；最小可编辑时值=1/4 拍）。tick = 16 分音符数。
- 内核形状音符（与 `get_midi_clip_notes` 回读对齐）：`{ id, pitch, startBeats, lengthBeats }`
  —— start/length 单位=拍（相对 clip 起点，time_unit:"beats"）；id 为内核稳定音符 ID（一等公民）。
- 记谱模型仅接受落在网格上的音符（start/length 均为 1/4 拍的整数倍）；网格外值 fail-honest 报错，
  不静默取整（编辑面自身只产生网格对齐值；内核回读出现网格外值=数据异常，显形给调用方）。
- 拍号：v1 支持 denominator=4（2/4、3/4、4/4——课程硬需求全集）；denominator≠4 fail-honest
  （beatsPerMeasure = numerator；拍线即 4 tick 间隔）。

## 1. R1 切分点规则（cut points）

每个音符跨度与空档跨度先在**小节线**与**拍线**处切开，再进入 R2：

- R1.1 小节线：`m * beatsPerMeasure * GRID_PER_BEAT`（m=0..measures）。
- R1.2 拍线：每小节内每 `GRID_PER_BEAT` tick 一条。
- R1.3 切分后每段长度 ∈ {1,2,3,4} tick（≤一拍），段边界即记谱符号边界。
- R1.4 **弱起延音链**：音符起点在拍内、跨拍线 → R1 切出多段 → R3 连成延音链（切分记谱，
  教材「弱起切分」的正确写法；spike e4 亲证：1.5 拍起、0.75 拍长 → 八分+十六分延音对）。
- R1.5（v1 边界，如实声明）**不做整小节音符归并**：填满整个 2/4 小节的音符按 R1 切成
  四分+四分延音链，不合并为二分音符。是否引入「整小节单符号」归并属 W18 待裁定项，v1
  保持切分链写法（与「延音建立/解除」编辑原语一致）。

## 2. R2 段内符号贪心（符号表+填充表）

段内用允许符号**从大到小**贪心填满。v1 允许符号集（16 分数 → VexFlow 时值/附点）：

| 段长(tick) | 符号 | VexFlow duration | dots |
|---|---|---|---|
| 4 | 四分 | `q` | 0 |
| 3 | 附点八分 | `8` | 1 |
| 2 | 八分 | `8` | 0 |
| 1 | 十六分 | `16` | 0 |

由于 R1 保证段长 ≤4，每段恰好一个符号；「拍内附点归并」即段长 3 → 附点八分单符号
（教材「前长后短」首半拍=附点八分+十六分两符号分属两段，各归各段）。

**休止填充表**（空档跨度同样经 R1 切分后逐段映射，符号同上表但 kind=rest）：

| 空档段长(tick) | 休止符号 |
|---|---|
| 4 | 四分休止 |
| 3 | 附点八分休止 |
| 2 | 八分休止 |
| 1 | 十六分休止 |

音符段与休止段共用同一确定性分解；不存在「先凑四分再补」与「先附点」的歧义（段长唯一决定符号）。

## 3. R3 延音链规则（tie chains）

- R3.1 同一内核音符（同 `id`）被 R1 切出的相邻符号互为延音：前符号 `tieNext`、后符号 `tiePrev`。
- R3.2 延音组渲染为跨符号 tie（VexFlow StaveTie，VF5 camelCase 参数 firstNote/lastNote）。
- R3.3 休止符永不参与延音；不同内核音符（同音高相邻）不自动合并延音（学生显式建立/解除是
  M2 编辑原语，作用于内核音符粒度，不造「记谱层假延音」）。
- R3.4 跨小节线延音：延音组两端分属相邻小节（spike k5 亲证），渲染跨 stave tie。

## 4. 推导输出形状（与渲染/游标的契约）

```
deriveNotation(model) → {
  events: Array<{
    measure: number,        // 0-based 小节号
    tick: number,           // 全局 16 分 tick
    sizeSixteenths: 1|2|3|4,
    kind: "note" | "rest",
    kernelId: string | null,// rest 为 null
    pitch: number | null,
    tieNext?: true, tiePrev?: true,
  }>,
  tieGroups: Array<number[]>,  // 每组=同 kernelId 各符号的 tick 序列
  measures: number,
}
```

- 确定性：同输入同输出（无随机、无浮点累积——全部整数 tick 运算；beats↔tick 换算只用乘除 4）。
- measures = max(ceil(最后音符末端 / 小节长), 1)；完全空模型=1 小节全休止。

## 5. 教材四要素映射（验证矩阵，测试锚定）

| 教材要素 | 规则路径 | 断言来源 |
|---|---|---|
| 附点（前长后短 3+1） | R2 表（两段各一符号） | spike L1-dotted |
| 切分（弱起跨拍） | R1.4 + R3.1 | spike L1-ties |
| 休止 | R2 休止填充表 | spike L1-rest-1/2 |
| 延音（含跨小节线） | R3.1-R3.4 | spike L1-tie-barline |

## 6. 边界与已知不做（v1）

- denominator≠4、网格外 start/length、负值/重叠音符（同 pitch 同 tick 双写）→ fail-honest 错误
  （重叠音符的内核写入由内核侧校验拒绝，见 MidiService 校验；模型层不静默合并）。
- 整小节单符号归并：见 R1.5，候 W18 裁定。
- 多声部/和弦（同 tick 多 pitch）：v1 单声部；内核回读同 tick 多音符时报「非单声部」fail-honest
  （课程内容单声部；和弦编辑属后续卡）。

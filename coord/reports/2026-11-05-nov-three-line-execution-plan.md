# 11 月三线执行排序（乐局 / 编曲 / 生成）——依赖逻辑版

- 编制：GLM 主管决策流 / 2026-11-05（用户裁定：三线均为必须能力层、11 月内完成；与混音能力层并行执行）
- 依据：四勘全 pass（256/257/258/259）+ [rulings §五-21/22](../rulings/2026-11-05-EDU-DEMO-SCOPE-RULINGS-1.md) + [generation-mcp-pivot](../decisions/2026-11-05-generation-mcp-pivot.md) §四-5 + [capability-domain-taxonomy](../decisions/2026-11-05-capability-domain-taxonomy.md)
- 本文件=执行侧领卡顺序权威；日期不预设，按完成与提交推进
- **池序改号注记（2026-11-06 主线整理）**：本计划原用池序 260-275，与主线混音/仪器线同日分配的 258-266 撞号（主线已入库 7 张不可改写）；按 [FE-MOJIBAKE 卡撞号先例](../cards/todo/2026-10-13-FE-MOJIBAKE-CLEANUP-1.md)（2026-10-13 协调=后发者改号）本线整体改号 **260-275 → 276-291**，本文档与 16 张卡面已同步（四勘历史编号 256-259 系本线勘察记录编号，保留不改）。**领取凭证=卡文件名（卡 ID）**，数字池序仅为排序辅助。

## 池序映射（原 → 新）

260→276 SYMBOL-CORE ｜ 261→277 SRCLOAD ｜ 262→278 YUE-DIALECT ｜ 263→279 LEVEL-CONTENT ｜ 264→280 LEVEL-RUNTIME-A ｜ 265→281 LEVEL-RUNTIME-B ｜ 266→282 ASK-ANCHOR ｜ 267→283 SYMLINK ｜ 268→284 VARMIN ｜ 269→285 MCP-DIALECT ｜ 270→286 PHOTO-SCORE ｜ 271→287 SMOKE-REGRESS ｜ 272→288 SUNO-MCP-VERIFY ｜ 273→289 YUE-RENDER ｜ 274→290 VAR-GEN-1b ｜ 275→291 PRESET


## 全局两条串行规则

1. **注册链**（动 `catalog.go`/`policy.go`/`capability_pack.go` 的卡）：**1a(276) → SRCLOAD(277) → YUE-DIALECT(278) → 乐局4(282) → SYMLINK(283)**——前卡验收提交后卡方可领取；验收当天必须 commit。
2. **真栈窗口**（§9）：一次一个 owner。我方占栈腿=SRCLOAD 烟测 / YUE-DIALECT 烟测 / 3b / 5 / 6 / VARMIN 试听（+SUNO-VERIFY 轻）；与混音能力层窗口按日协调登记。

## 依赖图

```
【注册链】 276 1a 🔗 277 SRCLOAD 🔗 278 YUE-DIALECT 🔗 282 乐局4 🔗 283 SYMLINK
            │           │              │                │             │
            │           │              └→ 285 MCP-DIALECT ─→ 288 SUNO-VERIFY(+用户key)
            │           └→ 284 VARMIN ▣ ─────────┐       └→(286 拍谱需要它)
            │                                     ↓
            ↓                              ⭐检查点：VARMIN 形态对口？
   279 乐局2(设计先行,定稿在1a后)               │是→乐局接编曲后端；│否→290 1b 兜底
            ↓
   280 3a 状态机 → 281 3b 挂载+烟测 ▣ ★全链通电
            ↓（+282+用户工具）        289 YUE-RENDER(12月素材腿,依乐局定稿)
   286 5 拍谱 ▣ → 287 6 回归试跑 ▣ → 12月：素材采集/deck

（机动不阻塞：291 ARR-PRESET 内核卡，SRCLOAD 后任意空档）
```

## 卡序表

| 池序 | 卡 | 依赖 | 注册链 | 真栈 |
|---|---|---|---|---|
| 276 | EDU-SYMBOL-CORE-1 | 无 | 🔗① | ▣ 烟测腿 |
| 277 | ARR-SRCLOAD-1 | 276 提交 | 🔗② | ▣ |
| 278 | GEN-YUE-DIALECT-1 | 277 提交 | 🔗③ | ▣ |
| 279 | EDU-LEVEL-CONTENT-1 | 设计先行；定稿在 276 验收后 | — | — |
| 280 | EDU-LEVEL-RUNTIME-A | 279 定稿 | — | — |
| 281 | EDU-LEVEL-RUNTIME-B | 280 | — | ▣ ★ |
| 282 | EDU-ASK-ANCHOR-1 | 278 提交；裁定 15 定落点 | 🔗④ | 搭车 |
| 283 | ARR-SYMLINK-1 | 276+282 提交 | 🔗⑤ | — |
| 284 | ARR-VARMIN-1 | 283 | — | ▣ |
| 285 | GEN-MCP-DIALECT-1 | 278 | — | — |
| 286 | EDU-PHOTO-SCORE-1 | 281+282+用户工具 | — | ▣ |
| 287 | EDU-SMOKE-REGRESS-1 | 281（282/286 齐更佳） | — | ▣ |
| 288 | GEN-SUNO-MCP-VERIFY-1 | 285+用户 key（后置） | — | ▣轻 |
| 289 | GEN-YUE-RENDER-1 | 乐局最终变奏定稿（12 月素材腿） | — | — |
| 290 | EDU-VAR-GEN-1b | **条件待命：检查点判"否"才领取** | — | 搭车 |
| 291 | ARR-PRESET-1 | 277（机动，不阻塞） | — | ▣ |

## 外部依赖（用户动作）

- 裁定 18（终局）+写入降级 → **279 定稿前**；裁定 15（问询通用化）→ **282 前**；
- Audiveris+music21 部署 → **286 前**；Suno key（可选）→ 288 前；12 月手测 → 素材采集。

## 首波派发（明日）

**276（1a）+ 279（乐局 2 设计先行稿）**同刻可领；277 在 276 验收提交后领取。与混音能力层并行，栈窗口登记协调。

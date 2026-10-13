# FE-MOJIBAKE-CLEANUP-1：webui 残留 GBK→UTF8 双重编码文案清理（3 处卡前存量）

- 发卡：Mac 决策侧 / 2026-10-13（依据=FE-CONFIG-EDIT-1 复验广谱乱码复核上报；[ruling](../../rulings/2026-10-13-FE-CONFIG-EDIT-1-pass.md) 域外裁量）
- 派发确认：待用户
- 验收负责人：Mac 决策侧先行裁定+PC 主管复核留档
- 池序 108（原 107 与 PC 侧 FIX-C2-TIMEFX-VIEWS-1 撞号，后发者改号，2026-10-13 协调）
- 归属：Mac 线（webui 渲染面卫生）；P3 / 10 分钟 / 无依赖
- 模型分级：L1 / 任意执行会话

## 背景

FE-CONFIG-EDIT-1 复验以广谱 hallmark 字符类全 src/ 扫描，发现 3 处卡前存量乱码（origin/main 与 ea8df384~1 均在场=非该卡引入；设置页文件域外，故不阻塞该卡）：

- `agent/webui/src/App.tsx` 3014 行（origin/main 行号）：`鍐呮牳閿欒: ${kernelError}` → 应为「内核错误」（顶栏运行时渲染）
- 4187 行：`return "鏄惧紡步进";` → 应为「显式步进」
- 9918 行：`return kind || "鑺傜偣";` → 应为「节点」

## 目标

1. 三处文案按上述还原为正确 UTF-8 中文（只改字符串字面量，不动任何逻辑）。
2. 顺带全 src/ 复扫广谱双重编码 hallmark 类（排除 *.test.* 断言 fixture），如有新发现一并修复并逐条记录。

## 文件域

agent/webui/src/App.tsx 文案字符串（仅字面量）；领取时与在飞卡核对 webui 文件域互斥。

## 验收标准

- `npm run test` exit 0 + `npm run build` exit 0；
- 广谱 hallmark 类 grep（排除 *.test.*）0 命中，命令与口径入回执。

## 停止条件

还原原文存疑（无法确证某乱码串对应中文原文）→ 保留原串并在回执上交，不猜。

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / 测试与 grep 口径 / 修复清单）
- 验收：（裁定文件 / 验收 commit）

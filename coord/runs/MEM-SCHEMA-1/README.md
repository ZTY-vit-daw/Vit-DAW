# MEM-SCHEMA-1 运行目录（设计件工件集）

- 卡：`coord/cards/doing/2026-10-18-MEM-SCHEMA-1.md`（池序 135，P2，L2 设计件；纯文档域，零真栈、零生产代码）
- owner：DSH PC 执行流 `exec-memschema`（task-7）
- 工作区：`D:/Vit_DAW_wt_memschema1`（分支 `port/mem-schema-1`，基线 `origin/main = a99117a0`）
- 唯一生产文档：**`docs/PLUGIN_MEMORY_SCHEMA_V1.md`**（本目录只放样例/证据/自查工件，不产生第二份规范）

## 文件清单

| 文件 | 作用 |
|---|---|
| `SAMPLE_ENTRIES.jsonl` | 13 条样例条目的机器可读版本（Pro-Q 3 ×5 / Pro-C 2 ×5 / ValhallaSupermassive ×3；与文档 §6.2 内联内容一致） |
| `SAMPLES_FOR_DOC.md` | 文档 §6.2.1 的生成源（逐条目索引表 + 完整 JSON）；重生成后需同步文档内联块 |
| `VALIDATION.txt` | schema 自校验报告（13/13 PASS；含 sample_mode 警告说明） |
| `make_samples.py` | **非生产代码**：设计期样例生成器 + schema 校验器（含参数面摘要重算、env_key 派生、entry_ref 内容摘要自洽断言） |
| `EVIDENCE_INDEX.md` | 证据索引：tracked/workspace 证据、SHA256、样例→证据映射、已核查缺失、`[待用户确认]` 汇总 |
| `SELF_CHECK_8POINTS.md` | 卡面 8 点覆盖自查 + 验收标准自查 + 停止条件核查 + 端测边界声明 |
| `README.md` | 本文件 |

## 复现 / 自验命令

```powershell
# 在 D:/Vit_DAW_wt_memschema1 下执行
python coord\runs\MEM-SCHEMA-1\make_samples.py `
  --main-repo D:\Vit_DAW `
  --worktree D:\Vit_DAW_wt_memschema1
# 期望输出：entries=13 errors=0 warnings=13（warnings 全部为 sample_mode 下 null 身份字段已标 [待用户确认]）
# 并重写 SAMPLE_ENTRIES.jsonl / SAMPLES_FOR_DOC.md / VALIDATION.txt
```

- `make_samples.py` 的输入依赖：主检出 `D:/Vit_DAW/artifacts/eq_compat_matrix/20260728_011728/capture/pro_q_3.json`（gitignored 工作区工件）与本 worktree `coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/pgtdc_20261016_205639/surfaces_compact.json`；缺失时脚本 fail-fast（不静默编造）。
- 样例环境键中的 `machine_key=m_local` 为**样例占位**；生产条目使用本机不透明指纹（文档 §3.6）。

## 与卡面目标 8 点对应（快速导航）

| 卡面目标 | 文档位置 |
|---|---|
| 1 定位与键（D14 客观经验类；环境聚合键；失配降权；不跨设备） | `docs/PLUGIN_MEMORY_SCHEMA_V1.md` §1 |
| 2 条目类型（六类逐类允许/禁止） | §2 |
| 3 字段（16 字段、ref 对齐、trust 五态、时间/证据文法） | §3 |
| 4 写入策略（capture 触发/写者/矛盾保留/过期合并） | §4 |
| 5 消费契约非权威（红线/executor 义务/live 优先/A1–A7 断言） | §5 |
| 6 样例（13 条、写序/Mix 0–200%/非有限端点/通道联动） | §6 |
| 7 与既有资产关系（不复活 VPS/Profile/SPAL；reference 区别；无映射/alias/credential） | §7 |
| 8 D14 接入点（raw vs distilled；查询/巩固/画像归 D14；开放项 O1–O8） | §8 |

## 边界声明

- 本目录**不是** D14 memory store，不含持久化/查询/consolidation/画像实现；`make_samples.py` 是设计期校验工具，不进入生产链路。
- 未修改 `agent/`、`VitApp/`、`scripts/`；未写共享主工作树；未 push main。
- 证据缺口与 `[待用户确认]` 项见 `EVIDENCE_INDEX.md` §6。

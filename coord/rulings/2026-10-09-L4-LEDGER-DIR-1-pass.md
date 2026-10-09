# Ruling：L4-LEDGER-DIR-1 pass（2026-10-09 决策侧）

- 裁定：**pass**。cherry-pick 092f4197 → main 05d74b2f。**L1-4 retain 链真栈闭环达成**（IMPL-D 端测边界②根因闭合）。
- 亲核四层：
  1. diff 亲读（全部 hunk）：三处卡面点名切换+两同族 hook 面（message_loop.go:4088/server.go:5778，RunTurnBoundaryHook→WriteRetains 写入面，确属同缺陷族，目录路径行为零变化）+挂点侧 genesisLedgerDir 消重复+G-3 两入口回落提炼（neutralFamilySystemSections/chatSystemSections，与被删内联逻辑逐字节同构）。
  2. **空路径语义独立复核**：helper `""→""` 与原 genesisLedgerDir 隐式 `filepath.Dir("")="."` 在消费面等价——`ledgerPath` 经 `filepath.Join` 使 `"."` 与 `""` 产出同一相对路径（Join 清洗）；exit_retain 空值守卫（"no project dir" advisory）行为保持。回执已如实申报 guard 为新增语义，采信。
  3. 我方复跑：carriers+harness+agentloop/chat 定向（G-3/G-4/装配面）全 ok；真栈工件亲读（下）。
  4. 真栈工件亲读（PASS 轮 20261009_102924）：ledger_path 落 `.vit` 文件父目录（修复生效实锤）；present=43031/absent=42324/delta=707（遥测 jsonl 双轮亲核：present 5 段含 session 账本段、absent 4 段）；真 LLM 轮（deepseek-v4-flash 真实 HTTP）；SCRIPT-LASTEXITCODE=0。首轮 FAIL（20261009_102500）为验证腿取点偏差（goalrunner 面本就不含 carriers）——如实留档非实现缺陷，分类正确。
- R-4 余缺口闭合核验：G-1（corrupt 读失败支全量保留直测）/G-3（两入口纯函数直测）/G-4（runner 终态漏斗：终态触发+暂停零触发+trace 恰一条）测试全在且复跑绿——**REVIEW-1 R-4 至此全闭合**（G-2 已随 GENESIS）。
- 边界声明采信：retain 真栈样本无结论轮不可得（卡面明示不算失败，单测覆盖在位）；真栈读面取点=chat 直连（点名②），点名①以单测锁定（同一 helper 同一语义）——边界如实。
- 附注：l4_genesis_summary.json 本身带 UTF-8 BOM（PS 写盘形态）——与 CONFIG-BOM-1 同根现象，脚本面无害（本轮 exit 0 实证），记录不立卡。

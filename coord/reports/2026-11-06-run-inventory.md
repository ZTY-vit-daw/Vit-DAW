# 2026-11-06 当日真栈 run 工件盘点（run-inventory）

用途：为夜间 gate 报告准备素材。纯只读盘点，唯一写入=本文件。

## PROGRESS 草稿区

- [S1] 检查进度文件：不存在，新建本文件。
- [S4] D1-DISCLOSURE verdict 拿到（runner exit 0 / PS1 exit 1 LW throw；branch=zero_mutation_boundary 亲证）；D1GATE attempt1/attempt2 关系确认（RESULTS.md：attempt1 22:07 6/8 留档，attempt2 8/8 ALL PASS）；MIXTICK=卡在 doing、工件在 commit ab798299（git ls-tree 亲证）工作树缺席；git 分类计数完成（89 条全对账）。终态区已写入，DONE。

---

## 终态区（盘点完成 2026-11-06）

### 一、当日 run 工件盘点表

| 卡 | run 目录（coord/runs/ 下） | stamp | 关键文件在场（summary/gates/console/RECEIPT/RUNBOOK） | exit / verdict | 一句话形态 |
|---|---|---|---|---|---|
| B4-ABLATION-1 | `B4-ABLATION-1/abrun1/` | 20261106_192330（summary 内 conversation b4_ablation_20261106_192348_r1…） | summary ✓（b4_ablation_summary.json）、RUNBOOK ✓、console ✓×2；gates/RECEIPT absent（RECEIPT 走 coord 提交 runs(264)） | 5/5 rounds `tool_chain_ok`，group_pass=true，read_only_verified=true；smoke console 尾 "ok: dev agent smoke completed"；脚本退出码 absent（工件未记） | 低频关系话术×5 轮 LLM 判定消融，全过；分支①成立判 A 态候选（真栈） |
| D2-STEREO-THIN-1 | `D2-STEREO-THIN-1/d2run1/` | run4 standing stack 20261106_203639 | RUNBOOK ✓、console ✓×4（run1–run4）、d2_manual_gates.json ✓、d2_verdict_store.json ✓；smoke report absent（在 worktree artifacts 路径） | 脚本四轮均未 exit 0：run1/run3 PropertyNotFoundStrict 崩、run2/run4 D1-S1 smoke failed throw；run4 后手动 gate A–D 全绿（gate_A pass=true，verdict=`unmeasurable` no_stereo_evidence，disclosure_only_v1，零 mutation） | stereo 门组真栈取证卡：排干族×2+门组管线 bug×2 四轮未过脚本线，修复后残存栈手动补门全绿，exit 0 轮交主管裁定 |
| D1GATE-SCRIPT-HYGIENE-1 | `D1GATE-SCRIPT-HYGIENE-1/construct/`（attempt2 顶层；attempt1 留档于 `construct/attempt1_20261106_2207/`） | attempt1=20261106_2207 / attempt2=22:10 | RESULTS.md ✓、probe_268_summary.json ✓、8 腿 console+smoke report（全部 status=pass）+gates JSON ✓ | attempt2 probe exit 0，8/8 ALL PASS（leg2/5/7 exit 1=pre-fix 诚实败证据腿）；attempt1 6/8 留档（败因=桩忠实性/断言锚点，非被测脚本） | D1 settle 门组三处卫生修补+第四处同族的构造验证，零真栈零 LLM（TEMP 桩仓 pre/post 对照） |
| SMOKE-SCRIPT-HYGIENE-1 | `SMOKE-SCRIPT-HYGIENE-1/construct_20261106/` | 20261106（目录名戳） | RESULTS.md ✓、probe1/2/3_summary.json ✓、leg1–4+legA–D console ✓、LW/settle gates JSON ✓ | probe1/2/3 ALL PASS exit 0（共 11 腿）；probe2 legB/legC exit 1=kernel 桩听门超时预期形态；leg1–4 期望 exit 形态逐腿吻合 | 烟测脚本四处修改的桩层验证（exit code/RunDir/前置文档/相位分支化），零 LLM 零真栈 |
| CHAT-TEST-ISOLATION-1 | `CHAT-TEST-ISOLATION-1/`（证据平铺，无 run 子目录） | 无 run stamp（验收轮次 round0–2） | README.md ✓（含验收命令表+回执）、chat_round1/2.txt ✓、fullrepo_go_test.txt ✓、g4_harness_isolated.txt ✓、impl_diff.patch ✓、baseline_chat_round0.txt ✓；console/summary/gates 形式不适用 | chat 包 ×2 轮 ok EXIT=0（141.1s/141.3s）；全仓 `go test ./...` 96 包 ok 0 FAIL EXIT=0；G4 不复现记录在案 | chat 测试隔离修补（+43/-0 零生产面）：mixboard unbound 根 per-test 隔离+TestMain 残留清扫，closeout doing→done |
| D1-DISCLOSURE-FORENSICS-1 | `D1-DISCLOSURE-FORENSICS-1/verifyrun1/` | run_stamp 20261106_113023（LW gates）；runner 开跑 2026-11-06T03:31Z | RUNBOOK ✓、console.log ✓、d1_smoke_report.json ✓（~785KB）、loudness_watch_gates.json ✓ | runner exit 0（D1-S1 PASS，validation.branch=`zero_mutation_boundary`，admitted 全门过，target 1032）+ PS1 exit 1（LW gates failures=2：watch hook 未注册/verdicts_path 空，throw 拦断 settle 相位）；material_qualification passed | D1 披露两形态根因取证+验收腿真栈复跑：两形态上游修复亲证，LW 相位分支结构性三选项交主管 |
| MIXTICK-CONFIRM-CONTRACT-1 | 当前工作树 `coord/runs/` **缺席** | 无（取证腿零栈零 LLM） | 工作树 absent；git 内工件亲证：commit `ab798299`（port/mixtick-confirm-contract-1）含 `coord/runs/MIXTICK-CONFIRM-CONTRACT-1/forensics.md` + `excerpts.md`；卡面 `coord/cards/doing/2026-11-05-MIXTICK-CONFIRM-CONTRACT-1.md` 回执在场 | 回执=取证腿完成（卡前提被证据推翻：模型始终输出结构化 needs_observation，真实失败链=去重弹回→反馈未入重建请求→settle 只读 goal.status）；实现/验证腿 HELD | mix_tick 确认卡契约取证：前提推翻上交，修补二选一（chat 措辞 :292 / settle 重建 walk）交主管裁定 |

注 1：`coord/runs/` 顶层与全深度均无以「2026-11-06」（含横线）命名的 run 目录；带 20261106 日期戳的目录为上表 D1GATE `attempt1_20261106_2207` 与 SMOKE `construct_20261106` 两处嵌套。
注 2：当日 `coord/reports/` 另有 5 份非 runs 范围文件（b4/c4 ablation verdict、d3 recon、gate-1-early、gate-2），供 gate 引用。

### 二、当日 git 提交分类计数（`git log --oneline --since="2026-11-06T00:00:00"`，合计 89）

| 类型 | 计数 |
|---|---|
| review | 10 |
| merge | 15（merge(...) 13 + 裸 Merge 2，其中 1 条合入 port/d1-disclosure-forensics-1） |
| dispatch | 1 |
| stack | 9 |
| claim | 13 |
| impl | 9 |
| done | 2 |
| gate | 1 |
| pool | 1 |
| 其它前缀小计 | 28（receipt 5、closeout 5、artifacts 4、fix 3、runbook 2、coord 2、test/runs/run/release/draft/decision/blocked 各 1） |
| **合计** | **89** |

计数口径：按提交消息首个 `type(` 前缀归类；无括号前缀的裸 `Merge branch …` 计入 merge；「其它前缀」为实际存在但任务卡类型清单未列的前缀，如实列出。

DONE

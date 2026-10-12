# G3-REOPEN-COSTCMP-1：probe 计量真实成本对比轮（G3 重开条件①取证）

- 发卡：GLM 主管决策侧 / 2026-10-11 晚窗（依据=[G3 终裁](../../rulings/2026-10-09-G3-FINAL-ruling.md)重开条件①"probe 计量源接入后真实成本对比"+PULL-PROBE-METER-1 已合入（层 a 工具墙充分层））
- 派发确认：已确认（主管依终裁既定条件发卡）
- 验收负责人：GLM 主管决策流
- 池序 73；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / PULL-PROBE-TIER-EXT-1（池序 52，0.25h）建议先收
- 模型分级：L1 / GLM 执行会话（PC，真栈对比轮）

## 目标

G3 终裁不切换的唯一实质依据=成本门槛（轮级 LLM 调用 17 vs 6 + probe 计量 unknown）。计量源已接入后，跑**真实成本对比轮**补齐物理口径证据：

1. 同话术族×双模式（push 生产缺省 vs pull `VIT_DAW_HARNESS=pull`）真栈对比，probe 物理计量（probeSpent/probeCostKnown 账本位）入账——对照 G3-ATTRIB-1 三话术族模式（run 工件在 coord/runs/G3-ATTRIB-1/）。
2. 产出成本口径对比报告：轮级调用数+逐 turn 注入+**probe 物理成本（墙钟，含调度噪音声明）**三口径并列——G3 重开判定输入。
3. **判定归主管**：本卡只交证据与建议，不自行切换 harness 缺省。

## 随轮附加断言（主管补注 2026-10-11 晚夜，M1-5 验收挂账）

任一 run 收尾后调 `/agent/invoke` `mix_report`（M1-5 三视图面）：断言**最近 settle 结论行为有结论形态**（latest_conclusion_id 非空+结论内容与盘上 free_state_conclusions 记录一致）——补 M1-5「有结论」分支的真栈行使（其两轮烟测 round 均无 settle 落盘只走了诚实缺省分支，单测已覆盖，真栈正面证据由本卡对比轮顺带产出，不另开卡）。

## 概率面纪律（AGENTS §8）

对比轮≥2 run（双模式各≥2）；失败分类分记（话术随机分支/环境中断/断言失败）；模型随机分支触发时同话术重跑不换族。

## 文件域

零生产代码或最小脚本扩展（现有 harness_ab 体系模式）；报告落 coord/runs/G3-REOPEN-COSTCMP-1/。

## 验收标准

双模式各≥2 run 退出码记录+三口径对比表+重开建议；工件可回指原始 run。

## 停止条件

计量账本位与 PULL-PROBE-METER-1 合入形态不符 → 上交对齐。

- 领取：2026-10-12 晚窗 / origin/main 3bcaa39a（前置 PULL-PROBE-TIER-EXT-1=e6cacb88 已在 main，merge-base --is-ancestor 实核）/ owner=GLM-5.3 PC 执行流（ZCode 会话，用户指派）/ 分支 port/g3-reopen-costcmp-1 / worktree D:/Vit_DAW_wt_g3rc1 / 领取提交=coord-claim-20261012-g3rc1（卡 mv 协调提交，随本回填推 main）
- 回执：**run=coord/runs/G3-REOPEN-COSTCMP-1/{20261012_192737_run1, 20261012_193106_run2}（双模式各 2 run，均 SCRIPT-LASTEXITCODE=0）+ 三轮启栈取证腿（185828/191706/192223）**；报告=[G3-REOPEN-COSTCMP-1-REPORT.md](../../runs/G3-REOPEN-COSTCMP-1/G3-REOPEN-COSTCMP-1-REPORT.md)。实现=port/g3-reopen-costcmp-1（eba745df 脚本扩展：原始 chat 响应捕获+mix_report 断言腿；6e66761d 相对路径硬化；3acfd0ff 工件精简件；已推 origin）。
  - **三口径**：③ probe 物理成本**完整有效**——push 族轮 5316ms/6 次（均 886）vs pull 5700ms/6 次（均 950），同机同 fixture 配对、调度噪音带内对称（墙钟含调度噪音已声明）；①轮级调用数②逐 turn 注入**被 API 额度耗尽差异性截断**（push 大上下文预扣 0.0249 全败/pull 小额入口偶过——任何双模差异均为额度墙伪象），健康链证据仍为 G3-ATTRIB-1 的 17 vs 6。
  - **失败分类分记**：脚本调用形态缺陷×3（相对 -RunArtifactsDir→内核双段路径必死 exit 0@1.6s；包装器捕获退出码+部署目录内核日志钉死；G3-ATTRIB-1 当轮=偶然躲过；已修待主管追认）＋额度环境中断（与 M2-TIMEFX-1 同源）＋断言失败 0；止损线未触发。
  - **mix_report 随轮断言**：两轮 invoke ok+mix_report.v1；盘上无结论板（判定链被额度截断从未 settle）→走诚实无记录分支（0=0 折叠一致）；**有结论分支未行使**（额度阻断面非断言面缺陷），M1-5 正面真栈证据仍缺——随额度恢复补。
  - **账本位核对**：execRecord.elapsed_ms+分级表+逐笔累加与 PULL-PROBE-METER-1/TIER-EXT 合入形态一致（停止条件未触发）；账本披露窗（budget_state 行/checkpoint probe_spent）本轮未开（额度截断所致），物理计量=账本同源逐笔和已入账。
  - 端测边界声明：真栈双轮 exit 0（fixture/open/authority/DAD/telemetry/分类/metrics 门全过）；渲染面零改动；重工件（raw 捕获 7MB/工程存储 9.2MB 每 run）留盘不入库可回指。
  - **给主管**：①条件①物理成本腿可收口（无 pull 物理反转空间）；②LLM 面健康链对比需额度恢复后复跑（建议小卡 ~0.25h 或并入 M2-TIMEFX-2 额度挂账腿并窗）；③脚本修复请追认。
- 验收：（裁定文件）

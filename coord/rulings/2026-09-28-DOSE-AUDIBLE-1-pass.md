# Ruling: DOSE-AUDIBLE-1 — pass（附端测门回补条款）（2026-09-28 决策侧）

## 裁定

**pass**。剂量上限全轴对齐可听阈值（dB 族 ±2→±10、pan ±0.15→±0.5）验收通过，实现合入 main（cherry-pick 34595601 → f748bd1a + gofmt 修复 baa6bf9a）。**端测门（真栈 exit 0）被 main 既有断裂阻断，实证已核为非本卡回归；exit-0 回补条款挂 SETTLE-CHAIN-1 卡**（该卡修复后烟测 exit 0 即视为本卡端测门闭合）。

## 决策侧复跑证据（独立于执行侧自验）

- 隔离 worktree（origin/port/dose-audible-1）全量门：build exit 0、**87 包 0 FAIL**；vet 告警仅 `semantic_treatment_strategy.go` unreachable ×2——该文件零 diff，main 既有（亲核）。
- **gofmt：决策侧 blob 级复验抓到回执不实**——`d1s1_domains.go` 两处结构体注释缩进违规（pan 注释块为本卡 diff 编辑文本时引入；de_esser 块为 main 既有顺手修复）。回执"触碰包 gofmt 干净"不成立，定性与修复记档于 baa6bf9a。教训：autocrlf 环境下 gofmt 门必须以 blob 级为准。
- 常量亲核：`experiment/dose_bounds.go` `D1S1MaxAbsDeltaDB=10.0`/`D1S1MaxAbsDeltaPan=0.5`+文案 helper；残留 grep 亲核：生产文件唯一残留=capabilitycontext:222（B2 域上交对，正确未越域），数值锚点零残留。
- 边界测试族亲核在案：端点/文案随常量/累计界跟随/disclosure 文案互锁/clampPanDelta 五族。

## 真栈证据链（exit 0 未达的定性——阻断均为 main 既有）

1. **剂量路径本身真栈绿**：主树 run 20260928_120230/120725（决策侧直读 d1_smoke_report.json）——`parameter_applied/readback_verified/evaluation_ready/human_audition_ready` 四旗标全 true；static_eq 剂量 −1.5/−1.0 经新校验路径接受。
2. **阻断一（disclosure×断言）**：基线 worktree（干净 HEAD@8ab2c8f6 旧 ±2 代码）run 115926 同签名失败——**先于剂量改动存在**（c8325305 09-23 引入的结构性冲突）。脚本域内修复（`values_for_key_outside_disclosure` 排除机器附挂 disclosure 子树、保留模型产文断言）定性为**结构假阳性修复而非弱化断言**，docstring 带取证 run 引用，采信。
3. **阻断二（settle 链）**：主树同断点两次（§8 止损遵守）；基线 run 121206 另一形态失败（before/after revision 无区分，applied=false）；本卡 diff 零触 settle 文件（文件清单亲核）。→ 上交 SETTLE-CHAIN-1。
4. 09-12 后 main 无该烟测 pass 记录（执行侧申报，与 G2 期间烟测走 materialize 脚本无冲突）。

## 遗留与上交

- **SETTLE-CHAIN-1**（新卡，todo/）：settle 链两形态取证+修复，验收=free_state_d1 冒测真栈 exit 0（回补本卡端测门）。
- **B2 剂量合约对**（staticbalance/validate.go:59 + capabilitycontext/context_manifest.go:222）：执行侧正确未越域。是否按"全轴可听"原则同步放宽 B2 求解器候选界，**待用户裁定**（牵动 solver 准入语义与 LLM manifest 互锁，须同卡同步）。
- 多轮累计界同源移动（=D1S1MaxAbsDeltaDB）：sealed 相等测试锁定的是相等关系非数字，同源是唯一一致落法，采信。
- `paper/experiment-baseline` 未触（红线遵守）；实验基线容纳方式待 K 拍板（决策记录第 3 条）。

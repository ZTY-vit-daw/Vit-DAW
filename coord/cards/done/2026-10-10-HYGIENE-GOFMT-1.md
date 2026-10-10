# HYGIENE-GOFMT-1：全仓 blob 级 gofmt 机械清债（51 文件，纯 gofmt -w）

- 发卡：GLM 主管决策侧（/morning 会话）/ 2026-10-10
- 派发确认：已确认（用户 2026-10-10 /morning 裁定标准日+机会面清理（gate 建议序②）；gate 记录"gofmt 既有债两处"经发卡侧全仓 blob 级扫描证实为 **51 文件**——新信息已修正卡面）
- 验收负责人：GLM 主管决策流
- 池序 39；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P3 / 0.5-1h（机械）/ 无（main a28eb1b0）
- 模型分级：L0 / **任意引擎**（含闲时、codex 无人时段）
- **超粒度豁免声明**：51 文件超"≤1h/≤5 文件"默认粒度——豁免依据=纯 `gofmt -w` 机械产物、禁止任何手工编辑、验收含 `git diff -w` 为空（零非空白变更的机械证明）。发卡侧明示，执行侧不得借粒度扩任何逻辑改动。

## 基线与口径

- 发卡侧基线扫描：[coord/runs/HYGIENE-GOFMT-1/baseline_scan_2026-10-10.txt](../../runs/HYGIENE-GOFMT-1/baseline_scan_2026-10-10.txt)（HEAD=a28eb1b0，共 51 文件，扫描命令内嵌可复跑；包分布=chat 36 / executionports 3 / harness 3 / epm 2 / audioclosure·capabilitycontext·history·materialize·mixboard·processorattestation·tim 各 1）。
- 口径：**blob 级**（`git show HEAD:agent/<file>` 输出 + CRLF 归一后与 `gofmt` 产物比对）。工作树 `gofmt -l .` 直跑受 autocrlf 行尾干扰（本机实测 333 文件假阳性），**不得**以工作树直跑清单为准。
- 领取时先复跑基线扫描命令，以复跑结果为准（HEAD 前进可能小幅增减清单）；复跑清单存入回执工件目录。

## 目标

1. 对复跑清单逐文件执行 `gofmt -w`（在 agent/ 工作树内；建议独立 worktree 或确认主树无他人占用）。
2. 单 commit 提交全部格式化产物（提交消息含本卡 ID+文件数）；**禁止任何手工整理**——若某文件 `gofmt -w` 后仍不净（理论上仅语法错误可能）→ 停止上报。
3. **CRLF 防线**：提交前抽验 per-file diff——若见整文件行尾翻转（每行都变）而非局部空白/对齐变化，停止并上报（环境中断/方案需裁定），不得提交混合行尾噪音。

## 验收标准（全部满足方 pass）

1. `git diff -w HEAD~1 HEAD`（忽略全部空白）**输出为空**——零非空白变更的机械证明。
2. 基线扫描命令复跑（对新 HEAD）输出 **0 文件**。
3. 全量 `go test ./... -count=1` 0 FAIL（gofmt 不改语义，此为防意外回归门）+ `go build ./...` exit 0。
4. 回执工件目录 [coord/runs/HYGIENE-GOFMT-1/](../../runs/HYGIENE-GOFMT-1/)：复跑前后两份清单+`git diff --stat` 汇总。

## 停止条件

- 任何文件 diff 呈整文件行尾翻转（见 CRLF 防线）。
- 全量测试出现任何 FAIL（gofmt 后不应发生；发生即环境或基线问题，保留原始输出上报，不得归因于"格式化副作用"了事）。

## 并行与资源

- 文件域=51 债文件清单本身（与 HYGIENE-FASTPATH-1 / REFSCHEMA-M4 / REFSCHEMA-M5 文件域**零重叠**，发卡侧已逐一核对：message_loop.go、gain_staging_ref.go、refschema.go、free_state_observation.go、pack.go、semantic_eq.go、c1_frequency_cleanup_runtime.go、b4_eq_runtime.go、c2_dynamic_batch.go、mixboard_decision_projection.go 均不在债清单）。可与他卡并行；不需要真栈；无资源占用。

- 领取：2026-10-10 晚窗 / origin/main 5558a0ce（blob 侦察=债文件全 LF、本机 autocrlf=true，CRLF 防线按逐文件 diff 核）/ owner=GLM-5.3-Flash（ZCode flash 执行会话）+ PC win32 / 分支 port/hygiene-gofmt-1 / worktree D:/Vit_DAW-worktrees/HYGIENE-GOFMT-1 / 领取提交=0bec8de4（已核远端卡面 owner）
- 回执：实现 commit=30bbc871（port/hygiene-gofmt-1，51 文件 +263/-262 单 commit，已推远待决策验收） / 复跑清单=pre 51 文件与基线逐行同序（[rerun_scan_pre_20261010.txt](../../runs/HYGIENE-GOFMT-1/rerun_scan_pre_20261010.txt)）/ post **0 文件**（[rerun_scan_post_20261010.txt](../../runs/HYGIENE-GOFMT-1/rerun_scan_post_20261010.txt)）；字节级 gofmt 可复现 51/51+CR=0（[gofmt_reproducibility_20261010.txt](../../runs/HYGIENE-GOFMT-1/gofmt_reproducibility_20261010.txt)）；build exit 0+全量 test 92 包 0 FAIL（终版提交态复测 [buildtest_final_20261010.txt](../../runs/HYGIENE-GOFMT-1/buildtest_final_20261010.txt)）；**验收标准1按字面不满足=diff -w 非空（13 文件 +16/-15，全部 gofmt 规范行为，工件 [diff_w_nonempty_20261010.txt](../../runs/HYGIENE-GOFMT-1/diff_w_nonempty_20261010.txt)）——该债集含结构性规范化使「diff -w 为空」与「复扫 0」不可同时满足，卡片缺陷上交裁定**；端测边界声明=纯格式化零语义面（51/51 字节级唯一变换=gofmt 的机械证明），不涉运行链路行为，端侧烟测不适用；执行异常两起（continuation_scheduler stat 伪象漏提交×1、traj CRLF 输入路径 doc 注释空格残留×1）均以 reset --soft 重组单 commit 处置+复现排除，详见 [receipt.md](../../runs/HYGIENE-GOFMT-1/receipt.md)
- 验收：（裁定文件 / 验收 commit）
